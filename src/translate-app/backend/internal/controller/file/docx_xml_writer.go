package file

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// docxParagraphPatch holds the node positions and translated text for one paragraph.
type docxParagraphPatch struct {
	nodes      []DocxTextNode
	translated string
}

// WriteTranslatedDocx produces a translated DOCX at outPath.
//
// translations must have the same length as df.Paragraphs:
// translations[i] is the translated text for df.Paragraphs[i].
//
// Strategy:
//   - For each translatable XML part (document.xml, header*.xml, footer*.xml):
//     patch the raw XML bytes in-place using the recorded <w:t> node offsets.
//     The first <w:t> node of each paragraph receives the full translated text;
//     all subsequent nodes are emptied (<w:t/>).
//   - Every other ZIP entry is copied byte-for-byte.
func WriteTranslatedDocx(df *DocxFile, translations []string, outPath string) error {
	if len(translations) != len(df.Paragraphs) {
		return fmt.Errorf("translations length %d != paragraphs length %d",
			len(translations), len(df.Paragraphs))
	}

	// Build per-file patch lists: sourceName → []docxParagraphPatch
	patches := make(map[string][]docxParagraphPatch)
	for i, para := range df.Paragraphs {
		if len(para.Nodes) == 0 {
			continue
		}
		patches[para.SourceFile] = append(patches[para.SourceFile], docxParagraphPatch{
			nodes:      para.Nodes,
			translated: translations[i],
		})
	}

	// Open source ZIP.
	zr, err := zip.OpenReader(df.ZipPath)
	if err != nil {
		return fmt.Errorf("mở DOCX gốc: %w", err)
	}
	defer zr.Close()

	// Create output file.
	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("tạo file output: %w", err)
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer zw.Close()

	for _, f := range zr.File {
		if err := copyOrPatchEntry(zw, f, patches); err != nil {
			return fmt.Errorf("xử lý %s: %w", f.Name, err)
		}
	}

	return nil
}

// copyOrPatchEntry copies one ZIP entry into zw, patching XML content if needed.
func copyOrPatchEntry(zw *zip.Writer, f *zip.File, patches map[string][]docxParagraphPatch) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// Preserve the original file header (compression method, timestamps, etc.)
	fh := f.FileHeader
	w, err := zw.CreateHeader(&fh)
	if err != nil {
		return err
	}

	pp, needsPatch := patches[f.Name]
	if !needsPatch {
		_, err = io.Copy(w, rc)
		return err
	}

	// Read full raw XML.
	raw, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	patched := applyDocxPatches(raw, pp)
	_, err = w.Write(patched)
	return err
}

// applyDocxPatches applies all paragraph patches to raw XML bytes.
// Patches are applied from the highest byte offset downwards so that
// earlier offsets remain valid after each substitution.
func applyDocxPatches(raw []byte, pps []docxParagraphPatch) []byte {
	type replacement struct {
		start   int
		end     int
		content []byte
	}
	var replacements []replacement

	for _, pp := range pps {
		if len(pp.nodes) == 0 {
			continue
		}
		// First node gets the full translated text.
		replacements = append(replacements, replacement{
			start:   pp.nodes[0].Start,
			end:     pp.nodes[0].End,
			content: buildWTNode(pp.translated),
		})
		// Remaining nodes are emptied.
		for _, node := range pp.nodes[1:] {
			replacements = append(replacements, replacement{
				start:   node.Start,
				end:     node.End,
				content: []byte(`<w:t/>`),
			})
		}
	}

	if len(replacements) == 0 {
		return raw
	}

	// Sort descending by start offset so replacements don't shift earlier positions.
	sort.Slice(replacements, func(i, j int) bool {
		return replacements[i].start > replacements[j].start
	})

	buf := make([]byte, len(raw))
	copy(buf, raw)

	for _, r := range replacements {
		if r.start < 0 || r.end > len(buf) || r.start >= r.end {
			continue
		}
		buf = spliceBuf(buf, r.start, r.end, r.content)
	}

	return buf
}

// spliceBuf replaces buf[start:end] with replacement and returns the new slice.
func spliceBuf(buf []byte, start, end int, replacement []byte) []byte {
	var b bytes.Buffer
	b.Grow(len(buf) - (end - start) + len(replacement))
	b.Write(buf[:start])
	b.Write(replacement)
	b.Write(buf[end:])
	return b.Bytes()
}

// buildWTNode constructs a <w:t xml:space="preserve">text</w:t> element.
// xml:space="preserve" ensures leading/trailing spaces in the translation
// are preserved by Word when rendering.
func buildWTNode(text string) []byte {
	var b strings.Builder
	b.WriteString(`<w:t xml:space="preserve">`)
	b.WriteString(xmlEscapeText(text))
	b.WriteString(`</w:t>`)
	return []byte(b.String())
}

// xmlEscapeText escapes XML special characters for use in text content.
func xmlEscapeText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// writePlainDocx writes plain translated text as a minimal valid DOCX file.
// Each double-newline-separated paragraph becomes a <w:p> element.
func writePlainDocx(text, outPath string) error {
	paragraphs := strings.Split(text, "\n\n")

	var body strings.Builder
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		// Handle single newlines within a paragraph as line breaks.
		lines := strings.Split(para, "\n")
		body.WriteString(`<w:p><w:r>`)
		for i, line := range lines {
			if i > 0 {
				body.WriteString(`<w:br/>`)
			}
			body.WriteString(`<w:t xml:space="preserve">`)
			body.WriteString(xmlEscapeText(line))
			body.WriteString(`</w:t>`)
		}
		body.WriteString(`</w:r></w:p>`)
	}

	docXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:wpc="http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas"` +
		` xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:body>` + body.String() + `<w:sectPr/></w:body></w:document>`

	relsXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
		`</Relationships>`

	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
		`</Types>`

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("không tạo được file DOCX: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	entries := []struct {
		name    string
		content string
	}{
		{"[Content_Types].xml", contentTypes},
		{"_rels/.rels", relsXML},
		{"word/document.xml", docXML},
	}
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			return fmt.Errorf("không tạo được entry %s: %w", e.name, err)
		}
		if _, err := io.WriteString(w, e.content); err != nil {
			return fmt.Errorf("không ghi được entry %s: %w", e.name, err)
		}
	}
	return nil
}
