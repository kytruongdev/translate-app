package file

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const maxDocxXML = 32 << 20 // 32 MiB

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
	p := findPDFToText()
	if p == "" {
		return "", errors.New("pdftotext không khả dụng — vui lòng dùng bản app mới nhất")
	}
	return extractPDFWithPDFToText(p, path)
}

// extractPDFWithClean extracts plain text from a PDF for AI translation.
func extractPDFWithClean(path string) (string, error) {
	return extractPDFPlain(path)
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

func fileExt(path string) string {
	return strings.ToLower(filepath.Ext(path))
}
