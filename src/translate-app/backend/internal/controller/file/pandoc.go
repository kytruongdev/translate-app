package file

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var (
	rePandocImg        = regexp.MustCompile(`(?i)<img\b[^>]*/?>`)
	rePandocSup        = regexp.MustCompile(`(?i)<sup>[^<]*</sup>`)
	rePandocBlockquote = regexp.MustCompile(`(?m)^> ?`)
	rePandocHTMLTag    = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	rePandocEmptyLines = regexp.MustCompile(`\n{3,}`)
)

// pandocBinaryName returns "pandoc.exe" on Windows, "pandoc" elsewhere.
func pandocBinaryName() string {
	if runtime.GOOS == "windows" {
		return "pandoc.exe"
	}
	return "pandoc"
}

// findPandoc returns the path to the pandoc binary.
// Search order: bundled next to executable → bin/ relative to cwd (dev mode) → system PATH.
// Returns "" if pandoc is not found.
func findPandoc() string {
	name := pandocBinaryName()
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
	if p, err := exec.LookPath("pandoc"); err == nil {
		return p
	}
	return ""
}

// extractDocxWithPandoc converts a DOCX file to GFM Markdown for display.
func extractDocxWithPandoc(pandocPath, docxPath string) (string, error) {
	out, err := exec.Command(pandocPath,
		"--from=docx",
		"--to=gfm",
		"--wrap=none",
		docxPath,
	).Output()
	if err != nil {
		return "", fmt.Errorf("pandoc: %w", err)
	}
	md := cleanPandocOutput(string(out))
	return strings.TrimSpace(md), nil
}

// extractDocxPlainText converts a DOCX file to plain text for AI translation.
// Plain text is consistent across all document types — no HTML leakage,
// no table/blockquote formatting issues — just readable content in order.
func extractDocxPlainText(pandocPath, docxPath string) (string, error) {
	out, err := exec.Command(pandocPath,
		"--from=docx",
		"--to=plain",
		"--wrap=none",
		docxPath,
	).Output()
	if err != nil {
		return "", fmt.Errorf("pandoc plain: %w", err)
	}
	text := rePandocEmptyLines.ReplaceAllString(string(out), "\n\n")
	return strings.TrimSpace(text), nil
}

// --- pdftotext (poppler) ---

// pdftotextBinaryName returns "pdftotext.exe" on Windows, "pdftotext" elsewhere.
func pdftotextBinaryName() string {
	if runtime.GOOS == "windows" {
		return "pdftotext.exe"
	}
	return "pdftotext"
}

// findPDFToText returns the path to the pdftotext binary.
// Search order: bundled next to executable → bin/ relative to cwd (dev mode) → system PATH.
// Returns "" if pdftotext is not found.
func findPDFToText() string {
	name := pdftotextBinaryName()
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
	if p, err := exec.LookPath("pdftotext"); err == nil {
		return p
	}
	return ""
}

// extractPDFWithPDFToText uses pdftotext (poppler) to extract plain text from a PDF.
// pdftotext correctly handles ToUnicode CMap tables and complex font encodings.
func extractPDFWithPDFToText(pdftotextPath, pdfPath string) (string, error) {
	out, err := exec.Command(pdftotextPath,
		"-enc", "UTF-8",
		"-nopgbrk",
		pdfPath,
		"-",
	).Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// cleanPandocOutput removes noise from pandoc GFM output that would
// confuse the AI translation prompt.
func cleanPandocOutput(md string) string {
	// Strip <img> — not translatable, break sentence flow
	md = rePandocImg.ReplaceAllString(md, "")
	// Strip <sup>N</sup> footnote reference markers — noise for translation
	md = rePandocSup.ReplaceAllString(md, "")
	// Strip blockquote markers from text boxes — pandoc renders text boxes as
	// blockquotes ("> text") but they are regular body content for translation
	md = rePandocBlockquote.ReplaceAllString(md, "")
	// Strip remaining HTML tags — pandoc emits raw HTML for complex tables
	// (colspan/rowspan). Keep text content, discard tag structure.
	md = rePandocHTMLTag.ReplaceAllString(md, "")
	// Collapse 3+ blank lines down to 2
	md = rePandocEmptyLines.ReplaceAllString(md, "\n\n")
	return md
}
