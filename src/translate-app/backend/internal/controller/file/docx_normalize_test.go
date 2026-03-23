package file

import (
	"strings"
	"testing"
)

func TestDocxXMLToMarkdown_IncludePicture(t *testing.T) {
	xml := `<?xml version="1.0"?><w:document><w:body><w:p><w:r><w:t>Đoạn trước</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>INCLUDEPICTURE "https://example.com/a.jpg?x=1&amp;y=2" \* MERGEFORMATINET</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>Sau ảnh</w:t></w:r></w:p></w:body></w:document>`

	got := docxXMLToMarkdown(xml)
	if !strings.Contains(got, "![](https://example.com/a.jpg?x=1&y=2)") {
		t.Fatalf("expected markdown image, got:\n%s", got)
	}
	if strings.Contains(strings.ToUpper(got), "INCLUDEPICTURE") {
		t.Fatalf("field code should be replaced, got:\n%s", got)
	}
	if !strings.Contains(got, "Đoạn trước") || !strings.Contains(got, "Sau ảnh") {
		t.Fatalf("expected paragraph text preserved, got:\n%s", got)
	}
}

func TestDocxXMLToMarkdown_ParagraphBreaks(t *testing.T) {
	xml := `<w:p><w:r><w:t>Một</w:t></w:r></w:p><w:p><w:r><w:t>Hai</w:t></w:r></w:p>`
	got := docxXMLToMarkdown(xml)
	if !strings.Contains(got, "Một") || !strings.Contains(got, "Hai") {
		t.Fatal(got)
	}
	// Hai đoạn → có xuống dòng kép giữa (Markdown)
	if !strings.Contains(got, "\n\n") {
		t.Fatalf("expected paragraph separation, got %q", got)
	}
}
