package file

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const sparsePageThreshold = 50 // chars per page below which OCR is attempted

// isOCRGarbage returns true when OCR output looks like noise rather than real text.
//
// Faded or overexposed scans produce scattered punctuation and single characters
// that Tesseract "reads" at very low accuracy. Two signals are combined:
//
//  1. Alpha ratio: letters+digits / all non-space chars.
//     Real text ≥ 0.70; severely noisy pages can be < 0.45.
//
//  2. Average meaningful-token length: strip leading/trailing punctuation from
//     each whitespace-split token, measure average rune length of what remains.
//     Real words average ≥ 3 runes; garbage tokens ("ee", "i", "L", "F") average < 2.5.
//
// A page is garbage when its alpha ratio is below 0.45 OR when it is below 0.75
// AND its average token length is below 2.5. This catches faded scans while
// keeping tables (low alpha ratio but long cell values) and OCR with punctuation.
func isOCRGarbage(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return true
	}

	// Alpha ratio.
	var alpha, total int
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		total++
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			alpha++
		}
	}
	if total == 0 {
		return true
	}
	alphaRatio := float64(alpha) / float64(total)
	if alphaRatio < 0.45 {
		return true
	}
	if alphaRatio >= 0.75 {
		return false // clearly good text
	}

	// Borderline alpha ratio (0.45–0.75): also check average token length.
	tokens := strings.Fields(text)
	var tokenCount, tokenLenSum int
	for _, tok := range tokens {
		// Strip leading/trailing non-alphanumeric runes (punctuation, symbols).
		clean := strings.TrimFunc(tok, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		if n := utf8.RuneCountInString(clean); n > 0 {
			tokenCount++
			tokenLenSum += n
		}
	}
	if tokenCount == 0 {
		return true
	}
	avgTokenLen := float64(tokenLenSum) / float64(tokenCount)
	return avgTokenLen < 2.5
}

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
		// Use basename + cmd.Dir to work around a Leptonica bug on macOS where
		// absolute paths are ignored and only the CWD is searched.
		args := []string{filepath.Base(pagePath), "stdout"}
		if td != "" {
			args = append(args, "--tessdata-dir", td)
		}
		args = append(args,
			"-l", "vie+eng",
			"--oem", "1", // LSTM engine only — best quality
			"--psm", "1", // Automatic page segmentation with OSD
		)
		cmd := exec.CommandContext(pageCtx, tesseractPath, args...)
		cmd.Dir = filepath.Dir(pagePath)
		out, ocrErr := cmd.Output()
		pageCancel()

		if ocrErr == nil {
			if text := strings.TrimSpace(string(out)); !isOCRGarbage(text) {
				if sb.Len() > 0 {
					sb.WriteString("\n\n")
				}
				sb.WriteString(text)
			}
		}
		// Page failure or garbage result is non-fatal: skip and continue.

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

// ocrSinglePage renders one PDF page to a temporary PNG and runs Tesseract on it.
// Returns the extracted text, or an error if rendering or OCR fails.
func ocrSinglePage(ctx context.Context, tesseractPath string, renderer pdfRendererInfo, tessdataDir, pdfPath string, pageNum int) (string, error) {
	tmpDir, err := os.MkdirTemp("", "gnj-ocr-p-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	pagePrefix := filepath.Join(tmpDir, "page")
	pageStr := strconv.Itoa(pageNum)
	var renderArgs []string
	if renderer.isPdftopng {
		renderArgs = []string{"-r", "200", "-f", pageStr, "-l", pageStr, pdfPath, pagePrefix}
	} else {
		renderArgs = []string{"-r", "200", "-png", "-f", pageStr, "-l", pageStr, pdfPath, pagePrefix}
	}
	renderCtx, renderCancel := context.WithTimeout(ctx, 30*time.Second)
	_, renderErr := exec.CommandContext(renderCtx, renderer.path, renderArgs...).Output()
	renderCancel()
	if renderErr != nil {
		return "", fmt.Errorf("render page %d: %w", pageNum, renderErr)
	}

	pngFiles, _ := filepath.Glob(filepath.Join(tmpDir, "page-*.png"))
	if len(pngFiles) == 0 {
		return "", fmt.Errorf("page %d: PNG không được tạo", pageNum)
	}
	sort.Strings(pngFiles)
	pngPath := pngFiles[0]

	pageCtx, pageCancel := context.WithTimeout(ctx, ocrPageTimeout)
	defer pageCancel()
	args := []string{filepath.Base(pngPath), "stdout"}
	if tessdataDir != "" {
		args = append(args, "--tessdata-dir", tessdataDir)
	}
	args = append(args, "-l", "vie+eng", "--oem", "1", "--psm", "1")
	cmd := exec.CommandContext(pageCtx, tesseractPath, args...)
	cmd.Dir = filepath.Dir(pngPath)
	out, ocrErr := cmd.Output()
	if ocrErr != nil {
		return "", ocrErr
	}
	text := strings.TrimSpace(string(out))
	if isOCRGarbage(text) {
		return "", nil // caller treats empty string as "nothing usable on this page"
	}
	return text, nil
}

// extractPDFHybrid extracts text from a potentially mixed PDF (some pages digital,
// some scanned). It runs pdftotext with page-break separators to get per-page text,
// then OCRs only the pages that are sparse (< sparsePageThreshold chars).
//
// Returns the combined text, whether any OCR was performed, and any fatal error.
// onProgress is called as (sparsePagesDone, totalSparsePages) while OCR runs;
// it is never called if no sparse pages are found (fast path).
func extractPDFHybrid(ctx context.Context, pdfPath string, onProgress func(done, total int)) (string, bool, error) {
	pdftotextPath := findPDFToText()
	if pdftotextPath == "" {
		return "", false, fmt.Errorf("pdftotext không khả dụng")
	}

	pageTexts, err := extractPDFPageTexts(pdftotextPath, pdfPath)
	if err != nil || len(pageTexts) == 0 {
		return "", false, err
	}

	// Identify pages with too little text (likely scanned).
	var sparseIdxs []int
	for i, pt := range pageTexts {
		if utf8.RuneCountInString(pt) < sparsePageThreshold {
			sparseIdxs = append(sparseIdxs, i)
		}
	}

	if len(sparseIdxs) == 0 {
		// Fully digital: return joined text directly (no OCR needed).
		return strings.TrimSpace(strings.Join(pageTexts, "\n\n")), false, nil
	}

	// Has sparse pages: OCR them.
	tesseractPath := findTesseract()
	renderer, ok := findPDFRenderer()
	if !ok || tesseractPath == "" {
		// OCR unavailable: return what pdftotext gave us.
		return strings.TrimSpace(strings.Join(pageTexts, "\n\n")), false, nil
	}

	td := bundledTessdataDir(tesseractPath)
	totalSparse := len(sparseIdxs)
	if onProgress != nil {
		onProgress(0, totalSparse)
	}
	didOCR := false
	for done, i := range sparseIdxs {
		select {
		case <-ctx.Done():
			return "", didOCR, ctx.Err()
		default:
		}
		text, ocrErr := ocrSinglePage(ctx, tesseractPath, renderer, td, pdfPath, i+1)
		if ocrErr == nil && strings.TrimSpace(text) != "" {
			pageTexts[i] = text
			didOCR = true
		}
		if onProgress != nil {
			onProgress(done+1, totalSparse)
		}
	}

	return strings.TrimSpace(strings.Join(pageTexts, "\n\n")), didOCR, nil
}
