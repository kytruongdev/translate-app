package file

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

// runStructuredOCR invokes the OCR sidecar with the provided page image paths
// and returns the parsed structured result.
//
// All pages are processed in a single subprocess invocation so the sidecar
// loads its ONNX models only once.
func runStructuredOCR(ctx context.Context, imagePaths []string) (*StructuredOCRResult, error) {
	sidecar := findStructuredOCRSidecar()
	if sidecar == "" {
		return nil, fmt.Errorf("structured OCR sidecar not found (%s) — run `make sidecar-mac` or `make sidecar-win`", structuredOCRSidecarName())
	}

	args := append([]string{}, imagePaths...)
	cmd := exec.CommandContext(ctx, sidecar, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, fmt.Errorf("OCR sidecar failed: %s", errMsg)
	}

	var result StructuredOCRResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("parse OCR sidecar output: %w", err)
	}

	return &result, nil
}
