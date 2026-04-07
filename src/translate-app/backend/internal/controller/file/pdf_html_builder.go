package file

import (
	"fmt"
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
        td {
            padding: 8px;
            vertical-align: top;
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
// figureCrops maps regionKey(pageNo, regionIdx) → base64-encoded PNG for figure regions.
func assembleStructuredHTML(result *StructuredOCRResult, translated map[string]string, figureCrops map[string]string) (string, error) {
	var body strings.Builder

	for _, page := range result.Pages {
		body.WriteString(fmt.Sprintf("<div class=\"page\" id=\"page-%d\">\n", page.PageNo))

		for ri, region := range page.Regions {
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
				content := translated[key]
				if strings.TrimSpace(content) != "" {
					align := region.Alignment
					alignStyle := ""
					if align == "center" || align == "right" {
						alignStyle = fmt.Sprintf(" style=\"text-align:%s\"", align)
					}
					// Render each newline-separated line — preserves multi-line titles
					lines := strings.Split(strings.TrimSpace(content), "\n")
					var parts []string
					for _, l := range lines {
						l = strings.TrimSpace(l)
						if l != "" {
							parts = append(parts, escapeHTML(l))
						}
					}
					if len(parts) > 0 {
						body.WriteString(fmt.Sprintf("<h2%s>%s</h2>\n", alignStyle, strings.Join(parts, "<br>\n")))
					}
				}

			case "table":
				html := translated[key]
				if strings.TrimSpace(html) != "" {
					body.WriteString(html)
					body.WriteString("\n")
				}

			case "figure":
				b64 := figureCrops[key]
				if region.FigureType == "decorative" {
					// Embed image as-is — no translation
					if b64 != "" {
						body.WriteString("<figure>\n")
						body.WriteString(fmt.Sprintf("<img src=\"data:image/png;base64,%s\" alt=\"Hình ảnh\">\n", b64))
						body.WriteString("</figure>\n")
					}
				} else {
					// Informational: embed image + translated annotation below
					annotation := translated[key]
					body.WriteString("<figure>\n")
					if b64 != "" {
						body.WriteString(fmt.Sprintf("<img src=\"data:image/png;base64,%s\" alt=\"Hình ảnh có văn bản\">\n", b64))
					}
					if strings.TrimSpace(annotation) != "" {
						body.WriteString("<figcaption>\n")
						body.WriteString(fmt.Sprintf("<span class=\"label-meta\">[Nội dung hình ảnh - đã dịch]</span>\n"))
						body.WriteString(escapeHTML(annotation))
						body.WriteString("\n</figcaption>\n")
					}
					body.WriteString("</figure>\n")
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
//   - Single newline (\n)   → space within a <p> (OCR scan lines, not semantic breaks)
//   - Trailing spaces before \n are trimmed
//
// Single newlines come from physical scan-line boundaries detected by EasyOCR.
// They should NOT produce <br> tags — doing so creates an artificial right margin
// because text stops at each scan line instead of flowing to the container edge.
func renderTextBlocks(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
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
				parts = append(parts, escapeHTML(line))
			}
		}
		if len(parts) > 0 {
			out.WriteString("<p>")
			out.WriteString(strings.Join(parts, " "))
			out.WriteString("</p>\n")
		}
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
