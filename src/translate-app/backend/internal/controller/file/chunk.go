package file

import (
	"strings"
	"unicode/utf8"
)

const defaultChunkRunes = 2500

// chunkDocxParagraphs groups DocxParagraph slices into batches for translation.
// Each batch contains as many paragraphs as fit within maxChars (rune count).
// A single paragraph that exceeds maxChars is placed in its own batch.
// Paragraph boundaries are never split.
func chunkDocxParagraphs(paras []DocxParagraph, maxChars int) [][]DocxParagraph {
	if maxChars < 1 {
		maxChars = defaultChunkRunes
	}
	var batches [][]DocxParagraph
	var cur []DocxParagraph
	curRunes := 0

	for i := range paras {
		p := paras[i]
		r := utf8.RuneCountInString(p.Text)
		if r == 0 {
			// Empty paragraph: include in current batch as a no-op placeholder
			// so index alignment with translations slice is preserved.
			cur = append(cur, p)
			continue
		}
		add := r
		if len(cur) > 0 {
			add++ // account for separator between paragraphs
		}
		if curRunes+add <= maxChars {
			cur = append(cur, p)
			curRunes += add
			continue
		}
		if len(cur) > 0 {
			batches = append(batches, cur)
			cur = nil
			curRunes = 0
		}
		cur = append(cur, p)
		curRunes = r
	}
	if len(cur) > 0 {
		batches = append(batches, cur)
	}
	return batches
}

func chunkRunes(s string, maxRunes int) []string {
	if maxRunes < 1 {
		maxRunes = defaultChunkRunes
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	for len(s) > 0 {
		if utf8.RuneCountInString(s) <= maxRunes {
			out = append(out, s)
			break
		}
		cut := runeCutIndex(s, maxRunes)
		out = append(out, strings.TrimSpace(s[:cut]))
		s = strings.TrimSpace(s[cut:])
	}
	return out
}

func runeCutIndex(s string, n int) int {
	if n <= 0 {
		return 0
	}
	var i, count int
	for count < n && i < len(s) {
		_, sz := utf8.DecodeRuneInString(s[i:])
		if sz == 0 {
			break
		}
		i += sz
		count++
	}
	return i
}

// chunkMarkdownByParagraphs gom theo đoạn (\n\n) để tránh cắt giữa ![](url) và giữ layout gần với tài liệu.
func chunkMarkdownByParagraphs(s string, maxRunes int) []string {
	if maxRunes < 1 {
		maxRunes = defaultChunkRunes
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	blocks := strings.Split(s, "\n\n")
	var chunks []string
	var cur []string
	curRunes := 0
	sep := 2 // "\n\n" giữa các block trong chunk

	for _, b := range blocks {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		r := utf8.RuneCountInString(b)
		add := r
		if len(cur) > 0 {
			add += sep
		}
		if curRunes+add <= maxRunes {
			cur = append(cur, b)
			curRunes += add
			continue
		}
		if len(cur) > 0 {
			chunks = append(chunks, strings.Join(cur, "\n\n"))
			cur = nil
			curRunes = 0
		}
		if r <= maxRunes {
			cur = append(cur, b)
			curRunes = r
			continue
		}
		for _, sub := range chunkRunes(b, maxRunes) {
			if strings.TrimSpace(sub) != "" {
				chunks = append(chunks, sub)
			}
		}
	}
	if len(cur) > 0 {
		chunks = append(chunks, strings.Join(cur, "\n\n"))
	}
	return chunks
}
