package file

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// structuredOCRSidecarName returns the platform-specific sidecar binary name.
func structuredOCRSidecarName() string {
	if runtime.GOOS == "windows" {
		return "paddleocr-windows-amd64.exe"
	}
	return "paddleocr-darwin-arm64"
}

// findStructuredOCRSidecar locates the OCR sidecar binary. Search order:
//  1. Next to the running executable (production: bundled in .app/Contents/MacOS)
//  2. bin/ relative to the current working directory (dev: make sidecar-mac)
//  3. PATH
func findStructuredOCRSidecar() string {
	name := structuredOCRSidecarName()

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
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

// ocrPythonRunner holds the python executable + script path for the dev-mode
// Python fallback when the compiled sidecar binary is unavailable or crashes.
type ocrPythonRunner struct {
	python string // path to python3 interpreter
	script string // path to ocr_sidecar.py
}

// findPythonRunner looks for a Python interpreter + ocr_sidecar.py, searching:
//  1. ../.venv/bin/python3 relative to cwd (dev venv)
//  2. system python3 on PATH
//
// The script is searched relative to the current working directory as ../ocr_sidecar.py.
// Returns nil if not usable (script not found, python not found).
func findPythonRunner() *ocrPythonRunner {
	// Locate ocr_sidecar.py — one level up from cwd (backend/../ocr_sidecar.py)
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	scriptCandidates := []string{
		filepath.Join(cwd, "..", "ocr_sidecar.py"),         // backend/../ocr_sidecar.py (dev)
		filepath.Join(filepath.Dir(cwd), "ocr_sidecar.py"), // same as above, alternative
	}
	script := ""
	for _, s := range scriptCandidates {
		if _, err := os.Stat(s); err == nil {
			script = s
			break
		}
	}
	if script == "" {
		return nil
	}

	// Locate python3 interpreter — prefer .venv adjacent to script
	scriptDir := filepath.Dir(script)
	pythonCandidates := []string{
		filepath.Join(scriptDir, ".venv", "bin", "python3"),
		filepath.Join(cwd, "..", ".venv", "bin", "python3"),
	}
	python := ""
	for _, p := range pythonCandidates {
		if _, err := os.Stat(p); err == nil {
			python = p
			break
		}
	}
	if python == "" {
		// Fall back to system python3
		if p, err := exec.LookPath("python3"); err == nil {
			python = p
		}
	}
	if python == "" {
		return nil
	}

	return &ocrPythonRunner{python: python, script: script}
}

// runStructuredOCR invokes the OCR sidecar with the provided page image paths
// and returns the parsed structured result.
//
// onPage (optional) is called after each page completes: onPage(pagesDone, totalPages).
// Use this to emit progress events during the OCR phase.
//
// Execution order:
//  1. Try the compiled sidecar binary (production path)
//  2. Fall back to Python interpreter + ocr_sidecar.py (dev path)
//
// All pages are processed in a single subprocess invocation so models load once.
func runStructuredOCR(ctx context.Context, imagePaths []string, onPage func(done, total int)) (*StructuredOCRResult, error) {
	sidecar := findStructuredOCRSidecar()
	if sidecar != "" {
		result, err := invokeOCRProcessStreaming(ctx, sidecar, nil, imagePaths, onPage)
		if err == nil {
			return result, nil
		}
		// Binary failed — fall through to Python fallback
		_ = err
	}

	// Python fallback (dev mode / binary unavailable)
	runner := findPythonRunner()
	if runner == nil {
		if sidecar == "" {
			return nil, fmt.Errorf(
				"OCR sidecar không tìm thấy (%s) — chạy `make sidecar-mac` hoặc cài .venv",
				structuredOCRSidecarName(),
			)
		}
		return nil, fmt.Errorf(
			"OCR sidecar binary thất bại và không tìm thấy Python fallback (.venv/bin/python3 + ocr_sidecar.py)",
		)
	}
	return invokeOCRProcessStreaming(ctx, runner.python, []string{runner.script}, imagePaths, onPage)
}

// invokeOCRProcessStreaming spawns an OCR subprocess (either binary or python+script)
// and reads streaming NDJSON output — one JSON object per page line, then {"done":true}.
//
// onPage is called after each page is parsed; it may be nil.
func invokeOCRProcessStreaming(ctx context.Context, executable string, prefixArgs []string, imagePaths []string, onPage func(done, total int)) (*StructuredOCRResult, error) {
	args := append(append([]string{}, prefixArgs...), imagePaths...)
	cmd := exec.CommandContext(ctx, executable, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start OCR sidecar: %w", err)
	}

	total := len(imagePaths)
	var result StructuredOCRResult

	// 16 MB per line — large tables can produce sizeable HTML in a single JSON line
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 16<<20), 16<<20)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Check for sentinel {"done":true} or fatal {"error":"..."} from sidecar
		var meta struct {
			Done  bool   `json:"done"`
			Error string `json:"error"`
		}
		if json.Unmarshal(line, &meta) == nil {
			if meta.Error != "" {
				// Drain stdout so Wait doesn't deadlock
				_, _ = io.Copy(io.Discard, stdout)
				_ = cmd.Wait()
				return nil, fmt.Errorf("OCR sidecar error: %s", meta.Error)
			}
			if meta.Done {
				break
			}
		}

		// Parse page result
		var page OCRPage
		if json.Unmarshal(line, &page) == nil && page.PageNo > 0 {
			result.Pages = append(result.Pages, page)
			if onPage != nil {
				onPage(len(result.Pages), total)
			}
		}
	}

	// Drain any remaining output before Wait to avoid broken pipe
	_, _ = io.Copy(io.Discard, stdout)

	waitErr := cmd.Wait()

	if scanErr := scanner.Err(); scanErr != nil {
		return nil, fmt.Errorf("reading OCR output: %w", scanErr)
	}
	if waitErr != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = waitErr.Error()
		}
		errMsg = stripANSI(errMsg)
		errMsg = filterLogLines(errMsg)
		return nil, fmt.Errorf("OCR sidecar failed: %s", errMsg)
	}

	return &result, nil
}

// stripANSI removes ANSI escape codes (e.g. color codes from Python loggers).
func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// skip until 'm'
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			i = j + 1
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

// filterLogLines removes INFO/WARNING lines from Python log output,
// keeping only actual error lines for cleaner error messages.
func filterLogLines(s string) string {
	lines := strings.Split(s, "\n")
	var kept []string
	for _, l := range lines {
		upper := strings.ToUpper(l)
		if strings.Contains(upper, "[INFO]") || strings.Contains(upper, "[WARNING]") {
			continue
		}
		if strings.TrimSpace(l) != "" {
			kept = append(kept, l)
		}
	}
	if len(kept) == 0 {
		return s // return original if everything was filtered
	}
	return strings.Join(kept, "\n")
}
