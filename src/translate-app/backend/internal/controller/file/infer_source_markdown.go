package file

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	reNumberedHeading    = regexp.MustCompile(`^\d+\.\s+\S`)
	reSubHeading         = regexp.MustCompile(`^\d+\.\d+\.?\s+\S`)    // 1.1 or 1.1.
	reDeepHeading        = regexp.MustCompile(`^\d+\.\d+\.\d+\.?\s+\S`) // 1.1.1 or 1.1.1.
	reRomanHeading       = regexp.MustCompile(`^(I{1,3}|IV|VI{0,3}|IX|XI{0,3}|XIV|XV|XVI{0,3}|XIX|XX)\.\s+\S`)
	reBulletLine         = regexp.MustCompile(`^[•+]\s+`)              // • or + bullet → normalize to -
	reSectionKeyword     = regexp.MustCompile(`(?i)^(PHẦN|CHƯƠNG|CHAPTER|PHỤ\s+LỤC|MỤC|PART)\s+`)
)

// plainTextLooksLikeMarkdown tránh chèn thêm ## khi nguồn đã có cú pháp Markdown.
func plainTextLooksLikeMarkdown(s string) bool {
	if strings.Contains(s, "##") || strings.Contains(s, "###") {
		return true
	}
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "# ") || strings.HasPrefix(t, "## ") {
			return true
		}
	}
	if strings.Count(s, "**") >= 2 {
		return true
	}
	if strings.Contains(s, "![") && strings.Contains(s, "](") {
		return true
	}
	return false
}

// inferMarkdownFromPlain thêm heading Markdown (##) cho dòng “giống tiêu đề” — chủ yếu PDF/plain.
func inferMarkdownFromPlain(plain string) string {
	paras := splitBlankParagraphs(plain)
	if len(paras) == 0 {
		return strings.TrimSpace(plain)
	}
	var b strings.Builder
	for i, p := range paras {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(enrichPlainParagraph(p))
	}
	return b.String()
}

func splitBlankParagraphs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	parts := strings.Split(s, "\n\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func enrichPlainParagraph(p string) string {
	lines := strings.Split(p, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	nonEmpty := lines[:0]
	for _, ln := range lines {
		if ln != "" {
			nonEmpty = append(nonEmpty, ln)
		}
	}
	if len(nonEmpty) == 0 {
		return p
	}
	if len(nonEmpty) == 1 {
		line := nonEmpty[0]
		line = normalizeBullet(line)
		if lvl := headingLevel(line); lvl > 0 {
			return strings.Repeat("#", lvl) + " " + line
		}
		return line
	}
	first := nonEmpty[0]
	first = normalizeBullet(first)
	if lvl := headingLevel(first); lvl > 0 && utf8.RuneCountInString(first) <= 220 {
		// Merge continuation lines that are part of the same heading.
		// pdftotext word-wraps long headings into multiple single-\n lines.
		// A line is a heading continuation if it's also uppercase or a short parenthetical.
		headingParts := []string{first}
		bodyLines := nonEmpty[1:]
		for i, ln := range nonEmpty[1:] {
			if isHeadingContinuation(ln) {
				headingParts = append(headingParts, ln)
				bodyLines = nonEmpty[i+2:]
			} else {
				break
			}
		}
		merged := strings.Join(headingParts, " ")
		if len(bodyLines) == 0 {
			return strings.Repeat("#", lvl) + " " + merged
		}
		return strings.Repeat("#", lvl) + " " + merged + "\n\n" + strings.Join(bodyLines, "\n")
	}
	// Normalize bullet lines within the paragraph.
	for i, ln := range nonEmpty {
		nonEmpty[i] = normalizeBullet(ln)
	}
	return strings.Join(nonEmpty, "\n")
}

// headingLevel returns 1-4 if line looks like a heading, 0 otherwise.
// ## (2) = top section, ### (3) = sub-section, #### (4) = deep sub-section.
func headingLevel(line string) int {
	line = strings.TrimSpace(line)
	n := utf8.RuneCountInString(line)
	if n < 4 {
		return 0
	}
	if reDeepHeading.MatchString(line) {
		return 4
	}
	if reSubHeading.MatchString(line) {
		return 3
	}
	if reNumberedHeading.MatchString(line) || reRomanHeading.MatchString(line) || reSectionKeyword.MatchString(line) {
		return 2
	}
	if isMostlyUppercaseHeadingLine(line) && n >= 12 && n <= 360 {
		return 2
	}
	return 0
}

// isHeadingContinuation returns true if a line is a continuation of a wrapped heading:
// all-uppercase text or a short parenthetical like "(HÀ TÂY CŨ)".
func isHeadingContinuation(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	// Short parenthetical like "(HÀ TÂY CŨ)" or "(Old Province)"
	if strings.HasPrefix(line, "(") && strings.HasSuffix(line, ")") && utf8.RuneCountInString(line) <= 80 {
		return true
	}
	return isMostlyUppercaseHeadingLine(line)
}

// normalizeBullet converts • and + bullets to markdown - format.
func normalizeBullet(line string) string {
	if reBulletLine.MatchString(line) {
		return "- " + strings.TrimSpace(reBulletLine.ReplaceAllString(line, ""))
	}
	return line
}

// shouldPromoteToHeading is kept for backwards compatibility with sourceMarkdownFromPlain.
func shouldPromoteToHeading(line string) bool {
	return headingLevel(line) > 0
}

func isMostlyUppercaseHeadingLine(s string) bool {
	letters := 0
	upper := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				upper++
			}
		}
	}
	if letters < 10 {
		return false
	}
	return float64(upper)/float64(letters) >= 0.82
}
