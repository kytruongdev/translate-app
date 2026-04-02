package file

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const ocrPageTimeout = 2 * time.Minute // per-page OCR timeout

func tesseractBinaryName() string {
	if runtime.GOOS == "windows" {
		return "tesseract.exe"
	}
	return "tesseract"
}

// findTesseract returns the path to the tesseract binary.
// Search order: bundled next to executable → bin/ relative to cwd (dev mode) → system PATH
// → common Homebrew paths (macOS GUI apps don't inherit shell PATH).
func findTesseract() string {
	name := tesseractBinaryName()
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(cwd, "bin", name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if p, err := exec.LookPath("tesseract"); err == nil {
		return p
	}
	for _, dir := range []string{"/opt/homebrew/bin", "/usr/local/bin"} {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// pdfRendererInfo holds info about which PDF-to-PNG renderer is available.
type pdfRendererInfo struct {
	path       string
	isPdftopng bool // true = pdftopng (XPDF), false = pdftoppm (Poppler)
}

// findPDFRenderer locates a PDF-to-PNG renderer.
// Preference: pdftopng (XPDF, bundled in production) → pdftoppm (Poppler, common on dev machines).
// Both produce PNG images of each page; the difference is in flag syntax and output naming.
func findPDFRenderer() (pdfRendererInfo, bool) {
	// pdftopng search: bundled next to exe → bin/ → PATH → Homebrew
	for _, name := range pdfRendererNames(true) {
		if p := findBinary(name, "pdftopng"); p != "" {
			return pdfRendererInfo{path: p, isPdftopng: true}, true
		}
	}
	// pdftoppm fallback: PATH → Homebrew (usually installed via poppler)
	for _, name := range pdfRendererNames(false) {
		if p := findBinary(name, "pdftoppm"); p != "" {
			return pdfRendererInfo{path: p, isPdftopng: false}, true
		}
	}
	return pdfRendererInfo{}, false
}

func pdfRendererNames(wantPdftopng bool) []string {
	if runtime.GOOS == "windows" {
		if wantPdftopng {
			return []string{"pdftopng.exe"}
		}
		return []string{"pdftoppm.exe"}
	}
	if wantPdftopng {
		return []string{"pdftopng"}
	}
	return []string{"pdftoppm"}
}

// findBinary searches for a binary in: next to executable → bin/ relative to cwd → PATH → Homebrew.
func findBinary(name, lookpathName string) string {
	if exe, err := os.Executable(); err == nil {
		c := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		c := filepath.Join(cwd, "bin", name)
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if p, err := exec.LookPath(lookpathName); err == nil {
		return p
	}
	for _, dir := range []string{"/opt/homebrew/bin", "/usr/local/bin"} {
		c := filepath.Join(dir, name)
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// bundledTessdataDir returns the tessdata/ directory adjacent to the tesseract binary,
// used only when the binary is bundled with the app. Returns "" when absent (system
// Homebrew install keeps tessdata elsewhere — tesseract finds it on its own).
func bundledTessdataDir(tesseractPath string) string {
	dir := filepath.Join(filepath.Dir(tesseractPath), "tessdata")
	if _, err := os.Stat(dir); err != nil {
		return ""
	}
	return dir
}

// ocrAvailable returns true if tesseract and a PDF-to-PNG renderer are both reachable.
func ocrAvailable() bool {
	if findTesseract() == "" {
		return false
	}
	_, ok := findPDFRenderer()
	return ok
}

// ocrPDFText extracts text from a scanned PDF using pdftopng/pdftoppm + Tesseract OCR.
//
// Pipeline:
//  1. pdftopng (XPDF) or pdftoppm (Poppler) renders each PDF page to PNG at 200 DPI.
//  2. Tesseract OCRs each PNG (vie+eng LSTM engine).
//  3. Page texts are concatenated with double newlines.
//
// onProgress is called after each page with (pagesOCRed, totalPages).
// Individual page failures are non-fatal: the page is skipped silently.
func ocrPDFText(ctx context.Context, pdfPath string, onProgress func(done, total int)) (string, error) {
	tesseractPath := findTesseract()
	if tesseractPath == "" {
		return "", fmt.Errorf("tesseract không tìm thấy")
	}
	renderer, ok := findPDFRenderer()
	if !ok {
		return "", fmt.Errorf("pdftopng/pdftoppm không tìm thấy")
	}

	// Create temp directory for rendered page images.
	tmpDir, err := os.MkdirTemp("", "gnj-ocr-*")
	if err != nil {
		return "", fmt.Errorf("không tạo được thư mục tạm OCR: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	pagePrefix := filepath.Join(tmpDir, "page")

	// Render all PDF pages to PNG at 200 DPI.
	// pdftopng (XPDF): pdftopng -r 200 input.pdf prefix → prefix-000001.png …
	// pdftoppm (Poppler): pdftoppm -r 200 -png input.pdf prefix → prefix-1.png … (zero-padded by page count)
	// Both output naming schemes sort correctly with sort.Strings.
	var renderArgs []string
	if renderer.isPdftopng {
		renderArgs = []string{"-r", "200", pdfPath, pagePrefix}
	} else {
		renderArgs = []string{"-r", "200", "-png", pdfPath, pagePrefix}
	}
	renderCtx, renderCancel := context.WithTimeout(ctx, 5*time.Minute)
	renderOut, renderErr := exec.CommandContext(renderCtx, renderer.path, renderArgs...).CombinedOutput()
	renderCancel()
	if renderErr != nil {
		return "", fmt.Errorf("%s: %w — %s", filepath.Base(renderer.path), renderErr, strings.TrimSpace(string(renderOut)))
	}

	// Collect rendered PNG files sorted by name (= page order).
	pages, err := filepath.Glob(filepath.Join(tmpDir, "page-*.png"))
	if err != nil || len(pages) == 0 {
		return "", fmt.Errorf("%s không tạo được ảnh trang", filepath.Base(renderer.path))
	}
	sort.Strings(pages)

	// Optional bundled tessdata directory (empty when tesseract is a system install).
	td := bundledTessdataDir(tesseractPath)

	total := len(pages)
	var sb strings.Builder

	for i, pagePath := range pages {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		pageCtx, pageCancel := context.WithTimeout(ctx, ocrPageTimeout)
		args := []string{pagePath, "stdout"}
		if td != "" {
			args = append(args, "--tessdata-dir", td)
		}
		args = append(args,
			"-l", "vie+eng",
			"--oem", "1", // LSTM engine only — best quality
			"--psm", "1", // Automatic page segmentation with OSD
		)
		out, ocrErr := exec.CommandContext(pageCtx, tesseractPath, args...).Output()
		pageCancel()

		if ocrErr == nil {
			if text := strings.TrimSpace(string(out)); text != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n\n")
				}
				sb.WriteString(text)
			}
		}
		// Page failure is non-fatal: skip and continue.

		if onProgress != nil {
			onProgress(i+1, total)
		}
	}

	result := strings.TrimSpace(sb.String())
	if result == "" {
		return "", fmt.Errorf("OCR không trích xuất được văn bản từ PDF")
	}
	return result, nil
}
