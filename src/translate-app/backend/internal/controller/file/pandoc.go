package file

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	rePandocImg        = regexp.MustCompile(`(?i)<img\b[^>]*/?>`)
	rePandocSup        = regexp.MustCompile(`(?i)<sup>[^<]*</sup>`)
	rePandocBlockquote = regexp.MustCompile(`(?m)^> ?`)
	rePandocHTMLTag    = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	rePandocEmptyLines = regexp.MustCompile(`\n{3,}`)
)

// findPandoc returns the path to the pandoc binary.
// Search order: bundled next to executable → system PATH.
// Returns "" if pandoc is not found.
func findPandoc() string {
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "pandoc")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if p, err := exec.LookPath("pandoc"); err == nil {
		return p
	}
	return ""
}

// extractDocxWithPandoc converts a DOCX file to GFM Markdown using pandoc.
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
