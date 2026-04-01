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

	"github.com/pdfcpu/pdfcpu/pkg/api"

	"translate-app/internal/bridge"
)

const (
	maxPDFPeek       = 4 << 20  // 4 MiB for text-operator heuristic
	maxPDFSize       = 50 << 20 // 50 MiB file size limit
	maxPDFPages      = 200
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

	switch ext {
	case ".pdf":
		return readPDFInfo(clean, name, size)
	case ".docx":
		return readDocxInfo(clean, name, size)
	default:
		return nil, errors.New("chỉ hỗ trợ DOCX và PDF")
	}
}

func readPDFInfo(path, name string, size int64) (*bridge.FileInfo, error) {
	if size > maxPDFSize {
		return nil, errors.New("tệp PDF quá lớn (tối đa 50MB)")
	}

	ctxPDF, err := api.ReadContextFile(path)
	if err != nil {
		return nil, fmt.Errorf("không mở được PDF (có thể được bảo vệ bằng mật khẩu): %w", err)
	}
	if err := ctxPDF.EnsurePageCount(); err != nil {
		return nil, fmt.Errorf("không xác định được số trang: %w", err)
	}
	pages := ctxPDF.PageCount
	if pages < 1 {
		return nil, errors.New("PDF không có trang hợp lệ")
	}
	if pages > maxPDFPages {
		return nil, fmt.Errorf("tệp PDF quá dài (tối đa %d trang)", maxPDFPages)
	}

	// Scan detection + char count estimation.
	// Prefer pdftotext (accurate). Fall back to raw Tj/TJ operator count — pure Go,
	// no external binary, never panics. Never use rsc.io/pdf here (panics on some PDFs).
	const samplePages = 10
	sampleCount := min(pages, samplePages)
	var charCount int
	if p := findPDFToText(); p != "" {
		sampleText, _ := extractPDFSample(p, path, sampleCount)
		sampleChars := utf8.RuneCountInString(sampleText)
		avgCharsPerPage := 0
		if sampleCount > 0 {
			avgCharsPerPage = sampleChars / sampleCount
		}
		if avgCharsPerPage < 50 {
			return nil, errors.New("Ứng dụng chưa hỗ trợ dịch thuật từ văn bản scan")
		}
		charCount = sampleChars
		if pages > sampleCount {
			charCount = avgCharsPerPage * pages
		}
	} else {
		// pdftotext unavailable: count Tj/TJ operators in raw bytes — fast, no deps.
		// Scanned PDFs have 0 operators; text PDFs typically have hundreds per page.
		if pdfTextOperatorCount(path) < 10 {
			return nil, errors.New("Ứng dụng chưa hỗ trợ dịch thuật từ văn bản scan")
		}
		charCount = pages * 2000 // rough estimate for UI preview
	}

	return buildFileInfo(name, "pdf", size, pages, charCount, false), nil
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
	return buildFileInfo(name, "docx", size, pages, charCount, false), nil
}

func buildFileInfo(name, typ string, size int64, pageCount, charCount int, isScanned bool) *bridge.FileInfo {
	chunks := (charCount + charsPerChunk - 1) / charsPerChunk
	if chunks < 1 {
		chunks = 1
	}
	minutes := (chunks + 1) / 2
	if minutes < 1 {
		minutes = 1
	}

	return &bridge.FileInfo{
		Name:              name,
		Type:              typ,
		FileSize:          size,
		PageCount:         pageCount,
		CharCount:         charCount,
		IsScanned:         isScanned,
		EstimatedChunks:   chunks,
		EstimatedMinutes:  minutes,
	}
}

func pdfTextOperatorCount(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxPDFPeek))
	if err != nil || len(b) == 0 {
		return 0
	}
	return bytes.Count(b, []byte(" Tj")) + bytes.Count(b, []byte(" TJ"))
}

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
