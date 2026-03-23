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

	"rsc.io/pdf"
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

func extractDocxPlain(path string) (string, error) {
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
