package file

import (
	"strings"
	"testing"
)

func TestIsTranslatablePart(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"word/document.xml", true},
		{"word/header1.xml", true},
		{"word/header2.xml", true},
		{"word/footer1.xml", true},
		{"word/footer2.xml", true},
		{"word/footnotes.xml", false},
		{"word/endnotes.xml", false},
		{"word/comments.xml", false},
		{"word/styles.xml", false},
		{"word/settings.xml", false},
		{"docProps/app.xml", false},
		{"[Content_Types].xml", false},
	}
	for _, c := range cases {
		got := isTranslatablePart(c.name)
		if got != c.want {
			t.Errorf("isTranslatablePart(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestExtractParagraphs_Basic(t *testing.T) {
	raw := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r><w:t>Hello world</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:t>Second paragraph</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`

	paras, err := extractParagraphs("word/document.xml", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paras) != 2 {
		t.Fatalf("expected 2 paragraphs, got %d", len(paras))
	}
	if paras[0].Text != "Hello world" {
		t.Errorf("para[0].Text = %q, want %q", paras[0].Text, "Hello world")
	}
	if paras[1].Text != "Second paragraph" {
		t.Errorf("para[1].Text = %q, want %q", paras[1].Text, "Second paragraph")
	}
}

func TestExtractParagraphs_MultipleRuns(t *testing.T) {
	// A paragraph with multiple runs (e.g. partial bold) — text should be concatenated.
	raw := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r><w:t xml:space="preserve">Hello </w:t></w:r>
      <w:r><w:rPr><w:b/></w:rPr><w:t>world</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`

	paras, err := extractParagraphs("word/document.xml", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paras) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(paras))
	}
	if paras[0].Text != "Hello world" {
		t.Errorf("para[0].Text = %q, want %q", paras[0].Text, "Hello world")
	}
	if len(paras[0].Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(paras[0].Nodes))
	}
}

func TestExtractParagraphs_SkipsFootnoteRef(t *testing.T) {
	// Footnote reference should be skipped; surrounding text preserved.
	raw := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r><w:t xml:space="preserve">Some text</w:t></w:r>
      <w:r><w:footnoteReference w:id="1"/></w:r>
      <w:r><w:t xml:space="preserve"> more text</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`

	paras, err := extractParagraphs("word/document.xml", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paras) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(paras))
	}
	if !strings.Contains(paras[0].Text, "Some text") {
		t.Errorf("expected 'Some text' in para, got %q", paras[0].Text)
	}
	if !strings.Contains(paras[0].Text, "more text") {
		t.Errorf("expected 'more text' in para, got %q", paras[0].Text)
	}
}

func TestExtractParagraphs_EmptyParasSkipped(t *testing.T) {
	// Paragraphs with no text (e.g. image-only) should not appear in output.
	raw := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r><w:t>Real content</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:drawing/></w:r>
    </w:p>
    <w:p></w:p>
  </w:body>
</w:document>`

	paras, err := extractParagraphs("word/document.xml", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paras) != 1 {
		t.Fatalf("expected 1 paragraph (empty ones skipped), got %d", len(paras))
	}
}

func TestExtractParagraphs_NodeOffsets(t *testing.T) {
	// Verify that node byte offsets actually point to <w:t> tags in the raw XML.
	raw := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r><w:t>Test</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`

	paras, err := extractParagraphs("word/document.xml", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paras) == 0 {
		t.Fatal("no paragraphs extracted")
	}
	for _, node := range paras[0].Nodes {
		snippet := raw[node.Start:node.End]
		if !strings.HasPrefix(snippet, "<w:t") {
			t.Errorf("node.Start does not point to <w:t>, got: %q", snippet[:min(20, len(snippet))])
		}
		if !strings.HasSuffix(snippet, "</w:t>") {
			t.Errorf("node.End does not end at </w:t>, got: %q", snippet[max(0, len(snippet)-20):])
		}
	}
}

func TestExtractWTText(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{`<w:t>Hello</w:t>`, "Hello"},
		{`<w:t xml:space="preserve"> spaced </w:t>`, " spaced "},
		{`<w:t></w:t>`, ""},
		{`<w:t>Tiếng Việt</w:t>`, "Tiếng Việt"},
	}
	for _, c := range cases {
		got := extractWTText(c.input)
		if got != c.want {
			t.Errorf("extractWTText(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
