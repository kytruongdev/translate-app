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

// writePlainDocx writes markdown-formatted translated text as a valid DOCX file.
// Supports: ## Heading2, ### Heading3, #### Heading4, - bullet lists, normal paragraphs.
func writePlainDocx(text, outPath string) error {
	lines := splitDocxLines(text)
	hasBullets := false
	for _, ln := range lines {
		if strings.HasPrefix(ln, "- ") {
			hasBullets = true
			break
		}
	}

	var body strings.Builder
	for _, ln := range lines {
		body.WriteString(lineToDocxParagraph(ln))
	}

	docXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:body>` + body.String() + `<w:sectPr/></w:body></w:document>`

	rootRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
		`</Relationships>`

	var numberingXML string
	if hasBullets {
		numberingXML = buildNumberingXML()
	}

	stylesXML := buildStylesXML()
	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
		`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>`
	if hasBullets {
		contentTypes += `<Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>`
	}
	contentTypes += `</Types>`

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("không tạo được file DOCX: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	// document.xml.rels: always points to styles, optionally to numbering.
	docRelsEntries := `<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`
	if hasBullets {
		docRelsEntries += `<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>`
	}
	docRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		docRelsEntries + `</Relationships>`

	entries := []struct{ name, content string }{
		{"[Content_Types].xml", contentTypes},
		{"_rels/.rels", rootRels},
		{"word/document.xml", docXML},
		{"word/styles.xml", stylesXML},
		{"word/_rels/document.xml.rels", docRels},
	}
	if hasBullets {
		entries = append(entries, struct{ name, content string }{"word/numbering.xml", numberingXML})
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

// splitDocxLines splits text into individual lines for DOCX paragraph conversion.
// Double newlines become paragraph breaks; single newlines are preserved as separate lines.
func splitDocxLines(text string) []string {
	var out []string
	for _, para := range strings.Split(text, "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		for _, line := range strings.Split(para, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				out = append(out, line)
			}
		}
		out = append(out, "") // blank separator between paragraphs
	}
	// Remove trailing blank
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// lineToDocxParagraph converts one markdown line to a DOCX <w:p> element.
func lineToDocxParagraph(line string) string {
	if line == "" {
		return `<w:p/>`
	}
	// Headings: ## / ### / ####
	if strings.HasPrefix(line, "#### ") {
		return wParagraphStyled("Heading4", strings.TrimPrefix(line, "#### "))
	}
	if strings.HasPrefix(line, "### ") {
		return wParagraphStyled("Heading3", strings.TrimPrefix(line, "### "))
	}
	if strings.HasPrefix(line, "## ") {
		return wParagraphStyled("Heading2", strings.TrimPrefix(line, "## "))
	}
	if strings.HasPrefix(line, "# ") {
		return wParagraphStyled("Heading1", strings.TrimPrefix(line, "# "))
	}
	// Bullet list
	if strings.HasPrefix(line, "- ") {
		return wBulletParagraph(strings.TrimPrefix(line, "- "))
	}
	// Normal paragraph
	return wParagraphNormal(line)
}

func wParagraphStyled(style, text string) string {
	return `<w:p><w:pPr><w:pStyle w:val="` + style + `"/></w:pPr>` +
		`<w:r><w:t xml:space="preserve">` + xmlEscapeText(text) + `</w:t></w:r></w:p>`
}

func wBulletParagraph(text string) string {
	return `<w:p><w:pPr><w:pStyle w:val="ListParagraph"/>` +
		`<w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr>` +
		`<w:r><w:t xml:space="preserve">` + xmlEscapeText(text) + `</w:t></w:r></w:p>`
}

func wParagraphNormal(text string) string {
	return `<w:p><w:r><w:t xml:space="preserve">` + xmlEscapeText(text) + `</w:t></w:r></w:p>`
}

func buildStylesXML() string {
	const ns = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`
	const calibri = `<w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:cs="Calibri"/>`
	style := func(id, name string, sz, lvl int) string {
		return `<w:style w:type="paragraph" w:styleId="` + id + `">` +
			`<w:name w:val="` + name + `"/>` +
			`<w:pPr><w:outlineLvl w:val="` + fmt.Sprintf("%d", lvl) + `"/></w:pPr>` +
			`<w:rPr>` + calibri + `<w:b/><w:sz w:val="` + fmt.Sprintf("%d", sz) + `"/>` +
			`<w:szCs w:val="` + fmt.Sprintf("%d", sz) + `"/></w:rPr>` +
			`</w:style>`
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:styles ` + ns + `>` +
		`<w:docDefaults><w:rPrDefault><w:rPr>` + calibri +
		`<w:sz w:val="22"/><w:szCs w:val="22"/></w:rPr></w:rPrDefault></w:docDefaults>` +
		`<w:style w:type="paragraph" w:default="1" w:styleId="Normal">` +
		`<w:name w:val="Normal"/>` +
		`<w:rPr>` + calibri + `</w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="ListParagraph">` +
		`<w:name w:val="List Paragraph"/>` +
		`<w:pPr><w:ind w:left="720"/></w:pPr>` +
		`<w:rPr>` + calibri + `</w:rPr></w:style>` +
		style("Heading1", "heading 1", 32, 0) +
		style("Heading2", "heading 2", 28, 1) +
		style("Heading3", "heading 3", 24, 2) +
		style("Heading4", "heading 4", 22, 3) +
		`</w:styles>`
}

func buildNumberingXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:abstractNum w:abstractNumId="0">` +
		`<w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/>` +
		`<w:lvlJc w:val="left"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr>` +
		`<w:rPr><w:rFonts w:ascii="Symbol" w:hAnsi="Symbol"/></w:rPr></w:lvl>` +
		`</w:abstractNum>` +
		`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>` +
		`</w:numbering>`
}
