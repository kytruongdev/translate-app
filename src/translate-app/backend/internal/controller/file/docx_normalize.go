package file

import (
	"html"
	"regexp"
	"strings"
)

var (
	// INCLUDEPICTURE "https://..." * MERGEFORMATINET — thường gặp khi dán bài báo vào Word.
	reIncludePicture = regexp.MustCompile(`(?i)INCLUDEPICTURE\s+"([^"]+)"`)
	// Phần dư sau INCLUDEPICTURE / field Word.
	reWordFieldSuffix = regexp.MustCompile(`(?i)\s*\*?\s*MERGEFORMAT\w*`)
	// HYPERLINK "url" — chỉ bắt dạng có dấu ngoặc kép (đơn giản).
	reHyperlinkField = regexp.MustCompile(`(?i)HYPERLINK\s+"([^"]+)"`)
)

// reDeletedText matches <w:del>…</w:del> blocks (tracked deletions).
// These must be stripped before text extraction so deleted Vietnamese/foreign
// text in revision history does not pollute language detection.
var reDeletedText = regexp.MustCompile(`(?s)<w:del\b[^>]*>.*?</w:del>`)

// docxXMLToMarkdown đọc document.xml (chuỗi), giữ đoạn văn cơ bản và chuyển field Word sang Markdown.
func docxXMLToMarkdown(xml string) string {
	s := xml
	// Remove tracked-deletion blocks before any text extraction.
	s = reDeletedText.ReplaceAllString(s, "")
	// Giữ ranh giới đoạn / xuống dòng từ OOXML trước khi bỏ thẻ.
	s = strings.ReplaceAll(s, "</w:p>", "\n\n")
	s = strings.ReplaceAll(s, "<w:br/>", "\n")
	s = strings.ReplaceAll(s, "<w:br />", "\n")
	s = strings.ReplaceAll(s, "<w:cr/>", "\n")
	s = strings.ReplaceAll(s, "<w:cr />", "\n")
	s = strings.ReplaceAll(s, "<w:tab/>", "\t")
	s = strings.ReplaceAll(s, "<w:tab />", "\t")

	s = strings.TrimSpace(reXMLTags.ReplaceAllString(s, " "))
	s = html.UnescapeString(s)

	// Gom khoảng trắng thừa trong từng “dòng logic” nhưng giữ \n\n giữa đoạn.
	s = normalizeDocxWhitespace(s)

	s = reIncludePicture.ReplaceAllString(s, "\n\n![]($1)\n\n")
	s = reHyperlinkField.ReplaceAllString(s, "\n[$1]($1)\n")
	s = reWordFieldSuffix.ReplaceAllString(s, "")

	s = normalizeDocxWhitespace(s)
	return strings.TrimSpace(s)
}

func normalizeDocxWhitespace(s string) string {
	paras := strings.Split(s, "\n\n")
	var out []string
	for _, p := range paras {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		lines := strings.Split(p, "\n")
		for i := range lines {
			lines[i] = strings.TrimSpace(strings.Join(strings.Fields(lines[i]), " "))
		}
		p = strings.Join(lines, "\n")
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "\n\n")
}
