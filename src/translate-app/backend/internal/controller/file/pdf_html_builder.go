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
        h2 { font-size: 1.2em; margin-bottom: 10px; }
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
					body.WriteString(fmt.Sprintf("<p>%s</p>\n", escapeHTML(content)))
				}

			case "title":
				content := translated[key]
				if strings.TrimSpace(content) != "" {
					body.WriteString(fmt.Sprintf("<h2>%s</h2>\n", escapeHTML(content)))
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
