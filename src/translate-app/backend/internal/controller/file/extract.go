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
	if strings.ToLower(ext) == ".docx" {
		return extractDocxPlain(path)
	}
	return "", errors.New("chỉ hỗ trợ DOCX")
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
// Prefers pandoc (accurate table/heading extraction); falls back to XML extraction.
func extractTranslationText(path, ext string) (string, error) {
	if strings.ToLower(ext) == ".docx" {
		if pandocPath := findPandoc(); pandocPath != "" {
			text, err := extractDocxPlainText(pandocPath, path)
			if err == nil && text != "" {
				return text, nil
			}
		}
	}
	return extractSourceMarkdown(path, ext)
}

// sourceMarkdownFromPlain chuẩn hoá nguồn để hiển thị song ngữ: với plain (DOCX chưa có MD),
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

func charAndPageCount(plain, ext string, _ int) (charCount, pageCount int) {
	charCount = utf8.RuneCountInString(plain)
	if charCount < 1 {
		charCount = 1
	}
	pages := max(1, (charCount+docxCharsPerPage-1)/docxCharsPerPage)
	return charCount, pages
}

func fileExt(path string) string {
	return strings.ToLower(filepath.Ext(path))
}
