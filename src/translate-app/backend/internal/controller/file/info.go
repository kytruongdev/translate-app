package file

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"translate-app/internal/bridge"
)

const (
	charsPerChunk    = 2500
	docxCharsPerPage = 2800
)

var reXMLTags = regexp.MustCompile(`<[^>]+>`)

func (c *controller) ReadFileInfo(ctx context.Context, path string) (*bridge.FileInfo, error) {
	_ = ctx
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("đường dẫn tệp trống")
	}

	clean := filepath.Clean(path)
	fi, err := os.Stat(clean)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("không tìm thấy tệp")
		}
		return nil, fmt.Errorf("không đọc được tệp: %w", err)
	}
	if fi.IsDir() {
		return nil, errors.New("đường dẫn là thư mục, không phải tệp")
	}

	ext := strings.ToLower(filepath.Ext(clean))
	name := filepath.Base(clean)
	size := fi.Size()

	if ext == ".docx" {
		return readDocxInfo(clean, name, size)
	}
	return nil, errors.New("chỉ hỗ trợ DOCX")
}

func readDocxInfo(path, name string, size int64) (*bridge.FileInfo, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("không mở được DOCX: %w", err)
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
		return nil, errors.New("DOCX thiếu word/document.xml")
	}
	rc, err := docXML.Open()
	if err != nil {
		return nil, fmt.Errorf("không đọc được nội dung DOCX: %w", err)
	}
	defer rc.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(rc, 32<<20)); err != nil {
		return nil, fmt.Errorf("đọc DOCX: %w", err)
	}

	md := docxXMLToMarkdown(buf.String())
	charCount := utf8.RuneCountInString(md)
	if charCount < 1 {
		charCount = 1
	}

	pages := max(1, (charCount+docxCharsPerPage-1)/docxCharsPerPage)
	return buildFileInfo(name, "docx", size, pages, charCount), nil
}

func buildFileInfo(name, typ string, size int64, pageCount, charCount int) *bridge.FileInfo {
	chunks := (charCount + charsPerChunk - 1) / charsPerChunk
	if chunks < 1 {
		chunks = 1
	}
	minutes := (chunks + 1) / 2
	if minutes < 1 {
		minutes = 1
	}

	return &bridge.FileInfo{
		Name:             name,
		Type:             typ,
		FileSize:         size,
		PageCount:        pageCount,
		CharCount:        charCount,
		EstimatedChunks:  chunks,
		EstimatedMinutes: minutes,
	}
}
