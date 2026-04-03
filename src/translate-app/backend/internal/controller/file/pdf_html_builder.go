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
        p {
            margin-bottom: 12px;
            text-align: justify;
        }
        table {
            width: 100%;
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
        .region-seal, .region-signature {
            display: block;
            margin: 15px 0;
        }
        .region-seal img, .region-signature img {
            max-width: 200px;
            height: auto;
            border: 1px solid #eee;
        }
        .label-meta {
            font-size: 0.8em;
            color: #888;
            font-style: italic;
            margin-bottom: 4px;
            display: block;
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

// assembleStructuredHTML converts OCR results into a complete HTML string.
// If images is not nil, it embeds seal/signature regions as base64 images.
func assembleStructuredHTML(result *StructuredOCRResult, imagePaths []string) (string, error) {
	var body strings.Builder

	for i, page := range result.Pages {
		body.WriteString(fmt.Sprintf("<div class=\"page\" id=\"page-%d\">\n", page.PageNo))
		
		// Use corresponding image path for cropping seals/signatures
		var pageImagePath string
		if i < len(imagePaths) {
			pageImagePath = imagePaths[i]
		}

		for _, region := range page.Regions {
			switch region.Type {
			case "text":
				if strings.TrimSpace(region.Content) != "" {
					body.WriteString(fmt.Sprintf("<p>%s</p>\n", escapeHTML(region.Content)))
				}
			case "table":
				// PaddleOCR already provides raw HTML for tables
				body.WriteString(region.HTML)
				body.WriteString("\n")
			case "seal", "signature":
				label := "Con dấu"
				if region.Type == "signature" {
					label = "Chữ ký"
				}
				body.WriteString(fmt.Sprintf("<div class=\"region-%s\">\n", region.Type))
				body.WriteString(fmt.Sprintf("<span class=\"label-meta\">[%s gốc]</span>\n", label))
				
				// Crop and embed image if available
				if pageImagePath != "" && len(region.BBox) == 4 {
					b64, err := cropImageToBase64(pageImagePath, region.BBox)
					if err == nil {
						body.WriteString(fmt.Sprintf("<img src=\"data:image/png;base64,%s\" alt=\"%s\">\n", b64, label))
					} else {
						body.WriteString(fmt.Sprintf("<div class=\"placeholder\">[%s hiện diện ở đây]</div>\n", label))
					}
				}
				body.WriteString("</div>\n")
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
