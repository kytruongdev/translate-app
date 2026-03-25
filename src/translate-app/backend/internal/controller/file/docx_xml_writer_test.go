package file

import (
	"archive/zip"
	"os"
	"strings"
	"testing"
)

func TestBuildWTNode(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Hello", `<w:t xml:space="preserve">Hello</w:t>`},
		{"Xin chào", `<w:t xml:space="preserve">Xin chào</w:t>`},
		{"a & b", `<w:t xml:space="preserve">a &amp; b</w:t>`},
		{"<tag>", `<w:t xml:space="preserve">&lt;tag&gt;</w:t>`},
		{"", `<w:t xml:space="preserve"></w:t>`},
	}
	for _, c := range cases {
		got := string(buildWTNode(c.input))
		if got != c.want {
			t.Errorf("buildWTNode(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestApplyDocxPatches_SingleNode(t *testing.T) {
	raw := []byte(`<w:p><w:r><w:t>Hello world</w:t></w:r></w:p>`)
	s := string(raw)
	start := strings.Index(s, "<w:t>")
	end := strings.Index(s, "</w:t>") + len("</w:t>")

	pps := []docxParagraphPatch{{
		nodes:      []DocxTextNode{{Start: start, End: end, Text: "Hello world"}},
		translated: "Xin chào thế giới",
	}}

	got := string(applyDocxPatches(raw, pps))
	if !strings.Contains(got, "Xin chào thế giới") {
		t.Errorf("expected translated text in output, got: %s", got)
	}
	if strings.Contains(got, "Hello world") {
		t.Errorf("original text should be replaced, got: %s", got)
	}
}

func TestApplyDocxPatches_MultipleNodes(t *testing.T) {
	raw := []byte(`<w:p><w:r><w:t>Hello </w:t></w:r><w:r><w:t>world</w:t></w:r></w:p>`)
	s := string(raw)

	t1Start := strings.Index(s, "<w:t>")
	t1End := t1Start + strings.Index(s[t1Start:], "</w:t>") + len("</w:t>")

	rest := s[t1End:]
	t2Start := t1End + strings.Index(rest, "<w:t>")
	t2End := t2Start + strings.Index(s[t2Start:], "</w:t>") + len("</w:t>")

	pps := []docxParagraphPatch{{
		nodes: []DocxTextNode{
			{Start: t1Start, End: t1End, Text: "Hello "},
			{Start: t2Start, End: t2End, Text: "world"},
		},
		translated: "Xin chào thế giới",
	}}

	got := string(applyDocxPatches(raw, pps))
	if !strings.Contains(got, "Xin chào thế giới") {
		t.Errorf("expected translated text, got: %s", got)
	}
	if !strings.Contains(got, "<w:t/>") {
		t.Errorf("expected emptied second node <w:t/>, got: %s", got)
	}
}

func TestApplyDocxPatches_XmlEscaping(t *testing.T) {
	raw := []byte(`<w:p><w:r><w:t>original</w:t></w:r></w:p>`)
	s := string(raw)
	start := strings.Index(s, "<w:t>")
	end := strings.Index(s, "</w:t>") + len("</w:t>")

	pps := []docxParagraphPatch{{
		nodes:      []DocxTextNode{{Start: start, End: end, Text: "original"}},
		translated: "a & b < c > d",
	}}

	got := string(applyDocxPatches(raw, pps))
	if !strings.Contains(got, "&amp;") {
		t.Errorf("& should be escaped, got: %s", got)
	}
	if !strings.Contains(got, "&lt;") {
		t.Errorf("< should be escaped, got: %s", got)
	}
}

func TestApplyDocxPatches_Empty(t *testing.T) {
	raw := []byte(`<w:p><w:r><w:t>unchanged</w:t></w:r></w:p>`)
	result := applyDocxPatches(raw, nil)
	if string(result) != string(raw) {
		t.Error("empty patches should return raw unchanged")
	}
}

func TestWriteTranslatedDocx_LengthMismatch(t *testing.T) {
	df := &DocxFile{
		Paragraphs: []DocxParagraph{{Text: "hello"}, {Text: "world"}},
		ZipPath:    "dummy.docx",
	}
	err := WriteTranslatedDocx(df, []string{"xin chào"}, "/tmp/out.docx")
	if err == nil {
		t.Error("expected error for length mismatch, got nil")
	}
}

func TestWriteTranslatedDocx_RoundTrip(t *testing.T) {
	docXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:body><w:p><w:r><w:t>Hello world</w:t></w:r></w:p></w:body>` +
		`</w:document>`

	// Write minimal DOCX (just document.xml inside a ZIP).
	tmpIn := t.TempDir() + "/input.docx"
	if err := writeMinimalDocx(tmpIn, docXML); err != nil {
		t.Fatalf("creating test DOCX: %v", err)
	}

	// Parse.
	df, err := ParseDocx(tmpIn)
	if err != nil {
		t.Fatalf("ParseDocx: %v", err)
	}
	if len(df.Paragraphs) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(df.Paragraphs))
	}

	// Write translated.
	tmpOut := t.TempDir() + "/output.docx"
	if err := WriteTranslatedDocx(df, []string{"Xin chào thế giới"}, tmpOut); err != nil {
		t.Fatalf("WriteTranslatedDocx: %v", err)
	}

	// Verify output ZIP contains patched content.
	zr, err := zip.OpenReader(tmpOut)
	if err != nil {
		t.Fatalf("output is not valid ZIP: %v", err)
	}
	defer zr.Close()

	found := false
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, _ := f.Open()
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := rc.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		rc.Close()
		content := sb.String()
		if strings.Contains(content, "Xin chào thế giới") {
			found = true
		}
		if strings.Contains(content, "Hello world") {
			t.Error("original text should be replaced in output")
		}
	}
	if !found {
		t.Error("translated text not found in output DOCX")
	}
}

// writeMinimalDocx creates a ZIP file at path containing only word/document.xml.
func writeMinimalDocx(path, docXML string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()
	w, err := zw.Create("word/document.xml")
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(docXML))
	return err
}
