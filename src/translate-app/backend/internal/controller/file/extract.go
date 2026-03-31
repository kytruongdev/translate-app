package file

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"rsc.io/pdf"
)

const maxDocxXML = 32 << 20 // 32 MiB

var (
	rePageNumber  = regexp.MustCompile(`(?m)^\s*\d{1,4}\s*$`)           // dòng chỉ có số trang
	reShortNoise  = regexp.MustCompile(`(?m)^.{1,2}\s*$`)               // dòng < 3 ký tự
	reHyphenBreak = regexp.MustCompile(`(\p{L})-\n(\p{L})`)             // hyphen line-break
	reSoftHyphen  = regexp.MustCompile("\u00AD")                        // soft hyphen U+00AD
	reMultiNL     = regexp.MustCompile(`\n{3,}`)                        // 3+ newlines
	religature    = strings.NewReplacer("ﬁ", "fi", "ﬂ", "fl", "ﬀ", "ff", "ﬃ", "ffi", "ﬄ", "ffl")
)

// extractSourceMarkdown returns normalized plain text suitable for markdown-ish display.
func extractSourceMarkdown(path, ext string) (string, error) {
	switch strings.ToLower(ext) {
	case ".pdf":
		return extractPDFPlain(path)
	case ".docx":
		return extractDocxPlain(path)
	default:
		return "", errors.New("chỉ hỗ trợ PDF và DOCX")
	}
}

func extractPDFPlain(path string) (string, error) {
	r, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("không đọc được PDF: %w", err)
	}
	n := r.NumPage()
	if n < 1 {
		return "", errors.New("PDF không có trang hợp lệ")
	}
	var b strings.Builder
	for i := 1; i <= n; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		content := p.Content()
		for _, t := range content.Text {
			b.WriteString(t.S)
		}
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String()), nil
}

// extractPDFWithClean extracts text from a PDF page by page, applies rule-based
// cleaning, and merges cross-page paragraph fragments.
func extractPDFWithClean(path string) (string, error) {
	r, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("không đọc được PDF: %w", err)
	}
	n := r.NumPage()
	if n < 1 {
		return "", errors.New("PDF không có trang hợp lệ")
	}

	var acc strings.Builder   // accumulated clean text
	var fragment string       // unfinished paragraph fragment from previous page

	for i := 1; i <= n; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}

		// Extract raw text from page.
		var raw strings.Builder
		for _, t := range p.Content().Text {
			raw.WriteString(t.S)
		}
		pageText := raw.String()
		if strings.TrimSpace(pageText) == "" {
			continue // skip empty/image-only pages
		}

		// Merge fragment from previous page.
		if fragment != "" {
			pageText = fragment + pageText
			fragment = ""
		}

		// Rule-based clean.
		pageText = cleanPDFPageText(pageText)
		if strings.TrimSpace(pageText) == "" {
			continue
		}

		// Detect if last line is an unfinished paragraph (no sentence-ending punctuation).
		lines := strings.Split(strings.TrimRight(pageText, "\n"), "\n")
		lastLine := strings.TrimSpace(lines[len(lines)-1])
		if lastLine != "" && !strings.ContainsAny(lastLine, ".!?:") {
			// Keep as fragment to merge with next page.
			fragment = lastLine + " "
			pageText = strings.Join(lines[:len(lines)-1], "\n")
		}

		if strings.TrimSpace(pageText) != "" {
			acc.WriteString(pageText)
			acc.WriteString("\n\n")
		}
	}

	// Flush remaining fragment.
	if fragment != "" {
		acc.WriteString(strings.TrimSpace(fragment))
		acc.WriteString("\n")
	}

	result := strings.TrimSpace(acc.String())
	if result == "" {
		return "", errors.New("không trích xuất được văn bản từ PDF")
	}
	return result, nil
}

// cleanPDFPageText applies rule-based cleaning to raw extracted PDF page text.
func cleanPDFPageText(s string) string {
	// Replace ligatures.
	s = religature.Replace(s)
	// Remove soft hyphens.
	s = reSoftHyphen.ReplaceAllString(s, "")
	// Fix hyphen line-break: "transla-\ntion" → "translation".
	s = reHyphenBreak.ReplaceAllString(s, "$1$2")
	// Remove lines that are only page numbers.
	s = rePageNumber.ReplaceAllString(s, "")
	// Remove lines shorter than 3 chars (noise/artifacts).
	s = reShortNoise.ReplaceAllString(s, "")
	// Collapse 3+ newlines to 2.
	s = reMultiNL.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func extractDocxPlain(path string) (string, error) {
	// Prefer pandoc when available — handles tables, headings, field codes correctly.
	if pandocPath := findPandoc(); pandocPath != "" {
		md, err := extractDocxWithPandoc(pandocPath, path)
		if err == nil && md != "" {
			return md, nil
		}
	}

	// Fallback: regex-based extraction (no pandoc installed).
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("không mở được DOCX: %w", err)
	}
	defer zr.Close()

	var docXML *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docXML = f
			break
		}
	}
	if docXML == nil {
		return "", errors.New("DOCX thiếu word/document.xml")
	}
	rc, err := docXML.Open()
	if err != nil {
		return "", fmt.Errorf("không đọc được nội dung DOCX: %w", err)
	}
	defer rc.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(rc, maxDocxXML)); err != nil {
		return "", fmt.Errorf("đọc DOCX: %w", err)
	}

	md := docxXMLToMarkdown(buf.String())
	if md == "" {
		return "", errors.New("DOCX không có văn bản trích được")
	}
	return md, nil
}

// extractTranslationText returns clean plain text for AI translation.
// Separate from extractSourceMarkdown (used for display) — plain text is
// consistent across all document types regardless of table/layout complexity.
func extractTranslationText(path, ext string) (string, error) {
	if strings.ToLower(ext) == ".docx" {
		if pandocPath := findPandoc(); pandocPath != "" {
			text, err := extractDocxPlainText(pandocPath, path)
			if err == nil && text != "" {
				return text, nil
			}
		}
	}
	// PDF or no pandoc: fall back to same extraction as display
	return extractSourceMarkdown(path, ext)
}

// sourceMarkdownFromPlain chuẩn hoá nguồn để hiển thị song ngữ: với plain (PDF / DOCX chưa có MD),
// suy luận nhẹ ## / tiêu đề số để cột trái render được format gần bản dịch.
func sourceMarkdownFromPlain(plain string) string {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return ""
	}
	if !plainTextLooksLikeMarkdown(plain) {
		plain = inferMarkdownFromPlain(plain)
	}
	return plain
}

func charAndPageCount(plain, ext string, pdfPages int) (charCount, pageCount int) {
	charCount = utf8.RuneCountInString(plain)
	if charCount < 1 {
		charCount = 1
	}
	switch strings.ToLower(ext) {
	case ".pdf":
		if pdfPages > 0 {
			return charCount, pdfPages
		}
		return charCount, max(1, (charCount+1999)/2000)
	default:
		pages := max(1, (charCount+docxCharsPerPage-1)/docxCharsPerPage)
		return charCount, pages
	}
}

func pdfPageCount(path string) (int, error) {
	r, err := pdf.Open(path)
	if err != nil {
		return 0, err
	}
	n := r.NumPage()
	if n < 1 {
		return 0, errors.New("PDF không có trang")
	}
	return n, nil
}

func fileExt(path string) string {
	return strings.ToLower(filepath.Ext(path))
}
