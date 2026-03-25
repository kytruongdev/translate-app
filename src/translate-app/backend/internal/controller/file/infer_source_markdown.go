package file

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	reNumberedHeading = regexp.MustCompile(`^\d+\.\s+\S`)
	reSectionKeyword  = regexp.MustCompile(`(?i)^(PHẦN|CHƯƠNG|CHAPTER|PHỤ\s+LỤC|MỤC|PART)\s+`)
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

// inferMarkdownFromPlain thêm heading Markdown (##) cho dòng “giống tiêu đề” — chủ yếu plain text.
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
		if shouldPromoteToHeading(line) {
			return "## " + line
		}
		return line
	}
	first := nonEmpty[0]
	if shouldPromoteToHeading(first) && utf8.RuneCountInString(first) <= 220 {
		rest := strings.Join(nonEmpty[1:], "\n\n")
		if strings.TrimSpace(rest) == "" {
			return "## " + first
		}
		return "## " + first + "\n\n" + rest
	}
	return p
}

func shouldPromoteToHeading(line string) bool {
	line = strings.TrimSpace(line)
	if utf8.RuneCountInString(line) < 4 {
		return false
	}
	if reNumberedHeading.MatchString(line) {
		return true
	}
	if reSectionKeyword.MatchString(line) {
		return true
	}
	if isMostlyUppercaseHeadingLine(line) {
		n := utf8.RuneCountInString(line)
		return n >= 12 && n <= 360
	}
	return false
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
