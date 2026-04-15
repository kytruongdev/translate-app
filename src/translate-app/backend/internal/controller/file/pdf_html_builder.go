package file

import (
	"fmt"
	"regexp"
	"strings"
)

const pdfHTMLTemplate = `<!DOCTYPE html>
<html lang="vi">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Bản dịch tài liệu - TranslateApp</title>
    <style>
        body {
            font-family: 'Times New Roman', Times, serif;
            line-height: 1.5;
            color: #333;
            max-width: 900px;
            margin: 40px auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .page {
            background: white;
            padding: 60px;
            margin-bottom: 30px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
            position: relative;
            min-height: 1100px;
        }
        h1 { font-size: 1.5em; font-weight: bold; text-align: center; margin-bottom: 8px; margin-top: 16px; }
        h2 { font-size: 1.15em; font-weight: bold; margin-bottom: 8px; margin-top: 16px; }
        p {
            margin-bottom: 12px;
            text-align: justify;
        }
        table {
            width: 100%%;
            border-collapse: collapse;
            margin: 20px 0;
        }
        table, th, td {
            border: 1px solid #444;
        }
        td, th {
            padding: 6px 8px;
            vertical-align: top;
            font-size: 0.9em;
        }
        th {
            background: #f5f5f5;
            font-weight: bold;
            text-align: center;
        }
        figure {
            margin: 15px 0;
            display: inline-block;
        }
        figure img {
            max-width: 100%%;
            height: auto;
            border: 1px solid #eee;
            display: block;
        }
        figcaption {
            font-size: 0.9em;
            color: #555;
            margin-top: 6px;
            padding: 6px 8px;
            background: #f9f9f9;
            border-left: 3px solid #aaa;
        }
        .label-meta {
            font-size: 0.75em;
            color: #aaa;
            font-style: italic;
            display: block;
            margin-bottom: 4px;
        }
        @media print {
            body { background: white; margin: 0; padding: 0; }
            .page { box-shadow: none; margin: 0; page-break-after: always; }
        }
    </style>
</head>
<body>
    %s
</body>
</html>`

// assembleStructuredHTML builds the final translated HTML from the OCR result.
//
// translated maps regionKey(pageNo, regionIdx) → translated content:
//   - text/title regions  → translated paragraph text
//   - table regions       → translated table HTML (cells replaced)
//   - informational figure → translated annotation text
//
// Figure images are taken directly from OCRRegion.ImageData (base64 data URL
// embedded in the Mistral OCR response) — no external figureCrops map needed.
func assembleStructuredHTML(result *StructuredOCRResult, translated map[string]string) (string, error) {
	var body strings.Builder

	for _, page := range result.Pages {
		body.WriteString(fmt.Sprintf("<div class=\"page\" id=\"page-%d\">\n", page.PageNo))

		// consumedTitles tracks region indices already merged into a preceding title block.
		consumedTitles := map[int]bool{}

		for ri, region := range page.Regions {
			if consumedTitles[ri] {
				continue
			}
			key := regionKey(page.PageNo, ri)

			switch region.Type {
			case "text":
				content := translated[key]
				if strings.TrimSpace(content) != "" {
					align := region.Alignment
					if align == "center" || align == "right" {
						body.WriteString(fmt.Sprintf("<div style=\"text-align:%s\">\n", align))
						body.WriteString(renderTextBlocks(content))
						body.WriteString("</div>\n")
					} else {
						body.WriteString(renderTextBlocks(content))
					}
				}

			case "title":
				// Merge consecutive title regions that look like continuation lines of a
				// multi-line document title (e.g. "CONTRACT FOR THE TRANSFER OF RIGHTS" /
				// "SALE AND PURCHASE AGREEMENT"). A region is eligible for merging only if
				// it does NOT look like a standalone section header:
				//   - does not start with a Roman numeral label  (I., II., III. …)
				//   - does not end with a colon  (e.g. "CERTIFICATE:", "Based on:")
				//   - is short  (≤ 80 runes — section headers tend to be short but distinctive)
				content := translated[key]
				for next := ri + 1; next < len(page.Regions); next++ {
					if page.Regions[next].Type != "title" {
						break
					}
					nextContent := strings.TrimSpace(translated[regionKey(page.PageNo, next)])
					if nextContent == "" || !isMergableTitle(nextContent) {
						break
					}
					content += "\n" + nextContent
					consumedTitles[next] = true
				}
				if strings.TrimSpace(content) != "" {
					align := region.Alignment
					alignStyle := ""
					tag := "h2"
					if align == "center" {
						alignStyle = " style=\"text-align:center\""
						// ALL-CAPS centered → <h1> (main title / state header)
						// Mixed-case centered → <h2> centered (subtitle)
						c := strings.TrimSpace(content)
						if c == strings.ToUpper(c) && len([]rune(c)) > 3 {
							tag = "h1"
						}
					} else if align == "right" {
						alignStyle = " style=\"text-align:right\""
					}
					lines := strings.Split(strings.TrimSpace(content), "\n")
					var parts []string
					for _, l := range lines {
						l = strings.TrimSpace(l)
						if l != "" {
							parts = append(parts, convertMarkdownInline(l))
						}
					}
					if len(parts) > 0 {
						body.WriteString(fmt.Sprintf("<%s%s>%s</%s>\n", tag, alignStyle, strings.Join(parts, "<br>\n"), tag))
					}
				}

			case "table":
				html := translated[key]
				if strings.TrimSpace(html) != "" {
					body.WriteString(html)
					body.WriteString("\n")
				}

			case "figure":
				// ImageData is populated from Mistral's embedded base64 response —
				// no extra PNG rendering needed.
				imgData := region.ImageData
				if region.FigureType == "decorative" {
					if imgData != "" {
						body.WriteString("<figure>\n")
						body.WriteString(fmt.Sprintf("<img src=\"%s\" alt=\"Hình ảnh\">\n", imgData))
						body.WriteString("</figure>\n")
					}
				} else {
					// Informational: embed image + translated annotation below
					annotation := translated[key]
					if imgData != "" || strings.TrimSpace(annotation) != "" {
						body.WriteString("<figure>\n")
						if imgData != "" {
							body.WriteString(fmt.Sprintf("<img src=\"%s\" alt=\"Hình ảnh có văn bản\">\n", imgData))
						}
						if strings.TrimSpace(annotation) != "" {
							body.WriteString("<figcaption>\n")
							body.WriteString("<span class=\"label-meta\">[Nội dung hình ảnh - đã dịch]</span>\n")
							body.WriteString(escapeHTML(annotation))
							body.WriteString("\n</figcaption>\n")
						}
						body.WriteString("</figure>\n")
					}
				}
			}
		}

		body.WriteString("</div>\n")
	}

	return fmt.Sprintf(pdfHTMLTemplate, body.String()), nil
}

// renderTextBlocks converts OCR-extracted text into proper HTML paragraphs.
//
// Rules:
//   - Double newline (\n\n) → separate <p> block
//   - Single newline (\n)   → <br> within a <p> (Mistral OCR uses \n for semantic line breaks)
//   - Trailing spaces before \n are trimmed
//
// isMergableTitle returns true when a translated title line looks like a
// continuation of a multi-line document title rather than a standalone section
// header. We refuse to merge if the line:
//   - starts with a Roman numeral label  (I., II., III., IV. …)
//   - ends with a colon  ("CERTIFICATE:", "Based on:")
//   - exceeds 80 runes  (section headers are usually short but unambiguous)
func isMergableTitle(s string) bool {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) > 80 {
		return false
	}
	if strings.HasSuffix(s, ":") {
		return false
	}
	// Reject Roman-numeral section labels: "I.", "II.", "III.", "IV.", "V.", …
	romanPrefixes := []string{"I.", "II.", "III.", "IV.", "V.", "VI.", "VII.", "VIII.", "IX.", "X."}
	for _, p := range romanPrefixes {
		if strings.HasPrefix(s, p+" ") || s == p {
			return false
		}
	}
	return true
}

// Mistral OCR produces semantic \n — e.g. each label:value field in a form is a
// separate line. We preserve those as <br> so the layout matches the source document.
func renderTextBlocks(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	// Normalize <br> tags (from OCR or translation output) to \n so the
	// existing newline-splitting logic renders them correctly as <br> in HTML.
	content = strings.ReplaceAll(content, "<br/>", "\n")
	content = strings.ReplaceAll(content, "<br />", "\n")
	content = strings.ReplaceAll(content, "<br>", "\n")
	// Convert markdown bold/italic to HTML before escaping
	content = convertMarkdownInline(content)

	// Split into paragraph blocks on double newlines
	blocks := strings.Split(content, "\n\n")
	var out strings.Builder
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		lines := strings.Split(block, "\n")
		var parts []string
		for _, line := range lines {
			line = strings.TrimRight(line, " \t")
			if line != "" {
				parts = append(parts, line)
			}
		}
		if len(parts) > 0 {
			out.WriteString("<p>")
			out.WriteString(strings.Join(parts, "<br>\n"))
			out.WriteString("</p>\n")
		}
	}
	return out.String()
}

// reBold matches **bold** spans; reItalicOnly matches *italic* (not preceded by *).
var (
	reBold       = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reItalicOnly = regexp.MustCompile(`\*([^*]+)\*`)
)

// convertMarkdownInline converts **bold** and *italic* markdown to HTML,
// HTML-escaping all plain-text segments in between.
// Works entirely in byte space to avoid rune/byte offset mismatch on Unicode.
// If the string already contains HTML tags it is returned unchanged.
func convertMarkdownInline(s string) string {
	if strings.Contains(s, "<strong>") || strings.Contains(s, "<em>") || strings.Contains(s, "<u>") {
		return s
	}

	var out strings.Builder
	rem := s

	for len(rem) > 0 {
		boldLoc := reBold.FindStringIndex(rem)
		italicLoc := reItalicOnly.FindStringIndex(rem)

		// Pick whichever match starts earlier; bold wins on tie
		var loc []int
		isBold := false
		if boldLoc != nil {
			loc = boldLoc
			isBold = true
		}
		if italicLoc != nil && (loc == nil || italicLoc[0] < loc[0]) {
			loc = italicLoc
			isBold = false
		}

		if loc == nil {
			// No more markdown — escape the rest as plain text
			out.WriteString(escapeHTML(rem))
			break
		}

		// Escape plain text before the match
		out.WriteString(escapeHTML(rem[:loc[0]]))

		inner := rem[loc[0]+2 : loc[1]-2]
		if isBold {
			out.WriteString("<strong>")
			out.WriteString(escapeHTML(inner))
			out.WriteString("</strong>")
		} else {
			inner = rem[loc[0]+1 : loc[1]-1]
			out.WriteString("<em>")
			out.WriteString(escapeHTML(inner))
			out.WriteString("</em>")
		}
		rem = rem[loc[1]:]
	}
	return out.String()
}

func escapeHTML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
	)
	return r.Replace(s)
}
