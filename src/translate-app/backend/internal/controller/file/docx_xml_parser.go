package file

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// DocxTextNode represents a single <w:t> node inside a paragraph run.
// Offset is its index among all w:t nodes in the paragraph (0-based),
// used by the writer to place translated text back precisely.
type DocxTextNode struct {
	// ByteOffset is the byte position of the opening <w:t...> tag in the raw XML.
	// Used by the writer for in-place replacement.
	Start int // byte index of '<' in '<w:t'
	End   int // byte index just after '</w:t>'
	Text  string
}

// DocxParagraph is one logical paragraph extracted from a DOCX XML part.
// A paragraph maps to one <w:p> element.
type DocxParagraph struct {
	// SourceFile is the ZIP entry name, e.g. "word/document.xml".
	SourceFile string
	// Index is the paragraph's order within SourceFile (0-based).
	Index int
	// Text is the concatenation of all <w:t> node values in this paragraph.
	Text string
	// Nodes holds positions of every <w:t> node so the writer can patch them.
	Nodes []DocxTextNode
}

// DocxFile holds the parsed content of a DOCX archive ready for translation.
type DocxFile struct {
	// Paragraphs across all processed XML parts, in order.
	Paragraphs []DocxParagraph
	// ZipPath is the original file path, kept for the writer.
	ZipPath string
}

// ParseDocx opens a DOCX file and extracts all translatable paragraphs.
// It processes: word/document.xml, word/header*.xml, word/footer*.xml.
// Footnotes and endnotes are intentionally skipped.
func ParseDocx(path string) (*DocxFile, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("không mở được DOCX: %w", err)
	}
	defer zr.Close()

	df := &DocxFile{ZipPath: path}

	for _, f := range zr.File {
		if !isTranslatablePart(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("đọc %s: %w", f.Name, err)
		}
		raw, err := io.ReadAll(io.LimitReader(rc, maxDocxXML))
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("đọc nội dung %s: %w", f.Name, err)
		}
		paras, err := extractParagraphs(f.Name, string(raw))
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", f.Name, err)
		}
		df.Paragraphs = append(df.Paragraphs, paras...)
	}

	return df, nil
}

// isTranslatablePart returns true for DOCX XML parts that contain body text.
// Excludes footnotes, endnotes, comments, and non-text parts.
func isTranslatablePart(name string) bool {
	base := filepath.Base(name)
	dir := filepath.Dir(name)
	if dir != "word" {
		return false
	}
	if base == "document.xml" {
		return true
	}
	if strings.HasPrefix(base, "header") && strings.HasSuffix(base, ".xml") {
		return true
	}
	if strings.HasPrefix(base, "footer") && strings.HasSuffix(base, ".xml") {
		return true
	}
	return false
}

// extractParagraphs parses the raw XML of one DOCX part and returns all
// non-empty paragraphs with their <w:t> node positions for later patching.
func extractParagraphs(sourceName, raw string) ([]DocxParagraph, error) {
	var paras []DocxParagraph
	paraIndex := 0

	// We use a byte-level scanner paired with xml.Decoder so we can record
	// exact byte offsets of every <w:t> node for the writer.
	dec := xml.NewDecoder(strings.NewReader(raw))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity

	type frame struct {
		name  xml.Name
		depth int
	}

	var (
		depth      int
		inPara     bool
		paraDepth  int
		inRun      bool
		inT        bool
		skipNode   bool // true inside nodes we want to ignore
		skipDepth  int
		nodes      []DocxTextNode
		tStart     int
		tEnd       int
	)

	// rawBytes lets us compute byte offsets from the decoder's InputOffset.
	rawBytes := []byte(raw)

	for {
		offset := int(dec.InputOffset())
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Non-fatal: skip malformed tokens
			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			local := t.Name.Local
			ns := t.Name.Space

			// Detect nodes to skip entirely (footnote refs, bookmarks, proofing).
			if isSkippableElement(local, ns) {
				skipNode = true
				skipDepth = depth
			}

			if skipNode {
				continue
			}

			switch local {
			case "txbxContent": // <w:txbxContent> — treat as independent paragraph scope
				// Text boxes embed <w:p> elements inside an outer <w:p> (drawing wrapper).
				// Finalize the outer paragraph (usually empty) and reset so each inner
				// <w:p> is treated as its own independent translation unit.
				if inPara {
					para := buildParagraph(sourceName, paraIndex, rawBytes, nodes)
					if para.Text != "" {
						paras = append(paras, para)
						paraIndex++
					}
					inPara = false
					inRun = false
					inT = false
					nodes = nil
				}
			case "p": // <w:p>
				if !inPara {
					inPara = true
					paraDepth = depth
					nodes = nil
				}
			case "r": // <w:r> run
				if inPara {
					inRun = true
				}
			case "t": // <w:t>
				if inPara && inRun {
					inT = true
					tStart = offset // byte offset of the '<' of <w:t
				}
			}

		case xml.EndElement:
			local := t.Name.Local

			if skipNode {
				if depth == skipDepth {
					skipNode = false
				}
				depth--
				continue
			}

			switch local {
			case "t": // </w:t>
				if inT {
					tEnd = int(dec.InputOffset())
					// Append node with its byte range and collected text.
					// Text was accumulated via CharData below.
					inT = false
					// tStart points to '<w:t', find the actual start of the tag.
					tagStart := findTagStart(rawBytes, tStart)
					nodes = append(nodes, DocxTextNode{
						Start: tagStart,
						End:   tEnd,
					})
				}
			case "r":
				inRun = false
			case "p":
				if inPara && depth == paraDepth {
					inPara = false
					inRun = false
					inT = false
					// Reconstruct text and record offsets from raw XML.
					para := buildParagraph(sourceName, paraIndex, rawBytes, nodes)
					if para.Text != "" {
						paras = append(paras, para)
						paraIndex++
					}
					nodes = nil
				}
			case "txbxContent": // </w:txbxContent> — exit text box scope
				// Finalize any trailing paragraph inside the text box.
				if inPara {
					para := buildParagraph(sourceName, paraIndex, rawBytes, nodes)
					if para.Text != "" {
						paras = append(paras, para)
						paraIndex++
					}
				}
				// Reset state: outer <w:p> (drawing wrapper) resumes, but its
				// </w:p> is a no-op since inPara=false.
				inPara = false
				inRun = false
				inT = false
				nodes = nil
			}
			depth--

		case xml.CharData:
			if inT && !skipNode {
				// Record actual text into the last pending node (its text field
				// will be set when we close </w:t>).
				text := string(t)
				if len(nodes) > 0 {
					// Update the text of the current (last) node being built.
					// We may receive multiple CharData tokens for one <w:t>.
					// Nothing to do here: we'll re-read from raw in buildParagraph.
					_ = text
				}
			}
		}
	}

	return paras, nil
}

// buildParagraph re-reads the raw byte ranges of each <w:t> node to extract
// their text and returns a fully populated DocxParagraph.
func buildParagraph(sourceName string, index int, raw []byte, nodeRanges []DocxTextNode) DocxParagraph {
	var sb strings.Builder
	nodes := make([]DocxTextNode, 0, len(nodeRanges))

	for _, nr := range nodeRanges {
		if nr.Start < 0 || nr.End > len(raw) || nr.Start >= nr.End {
			continue
		}
		snippet := string(raw[nr.Start:nr.End])
		text := extractWTText(snippet)
		nodes = append(nodes, DocxTextNode{
			Start: nr.Start,
			End:   nr.End,
			Text:  text,
		})
		sb.WriteString(text)
	}

	return DocxParagraph{
		SourceFile: sourceName,
		Index:      index,
		Text:       strings.TrimSpace(sb.String()),
		Nodes:      nodes,
	}
}

// extractWTText parses the text content from a raw <w:t ...>text</w:t> snippet.
// It handles xml:space="preserve" implicitly since we preserve the raw text.
func extractWTText(snippet string) string {
	dec := xml.NewDecoder(strings.NewReader(snippet))
	dec.Strict = false
	var text strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if cd, ok := tok.(xml.CharData); ok {
			text.Write(cd)
		}
	}
	return text.String()
}

// findTagStart walks backwards from offset to find the '<' that opens the tag.
func findTagStart(raw []byte, hint int) int {
	for i := hint; i >= 0; i-- {
		if raw[i] == '<' {
			return i
		}
	}
	return hint
}

// isSkippableElement returns true for XML elements whose subtree should be
// ignored for translation purposes.
func isSkippableElement(local, _ string) bool {
	switch local {
	case
		"footnoteReference", // <w:footnoteReference>
		"footnoteRef",       // <w:footnoteRef>
		"endnoteReference",  // <w:endnoteReference>
		"endnoteRef",        // <w:endnoteRef>
		"instrText",         // field instructions (HYPERLINK, INCLUDEPICTURE…)
		"del",               // tracked deletions
		"rPrChange",         // revision markup
		"pPrChange",         // revision markup
		"bookmarkStart",     // bookmarks (no text)
		"bookmarkEnd",
		"commentReference", // comment anchors
		// NOTE: <w:drawing> and <w:pict> are intentionally NOT skipped — they may
		// contain <w:txbxContent> (text boxes) with translatable text.
		// Image-only drawings have no <w:t> children so no harm is done.
		"Fallback": // <mc:Fallback> — skip VML duplicate of modern text boxes
		return true
	}
	return false
}
