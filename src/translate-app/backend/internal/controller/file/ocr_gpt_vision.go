package file

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/image/draw"
)

const gptVisionModel = openai.GPT4o

const gptVisionSystemPrompt = `You are a precise OCR engine for Vietnamese legal documents.
Extract all visible content from the page image in reading order.
Return ONLY valid JSON — no markdown fences, no explanation.`

const gptVisionUserPrompt = `Extract the page content and return this exact JSON:
{
  "regions": [
    {"type": "text",   "content": "exact Vietnamese text", "alignment": "left|center|right"},
    {"type": "title",  "content": "heading or section label", "alignment": "left|center|right"},
    {"type": "table",  "html": "<table><tr><th>col</th></tr><tr><td>val</td></tr></table>"},
    {"type": "figure", "figure_type": "decorative"},
    {"type": "figure", "figure_type": "informational", "text_lines": ["line1", "line2"]}
  ]
}

Rules:
- Copy Vietnamese text EXACTLY — preserve all diacritics, do NOT translate
- alignment: "center" if text is visually centered, "right" if right-aligned, else "left"
- Use "title" for headings, section numbers, bold labels; "text" for body paragraphs
- Stamps, seals, signatures, logos, photos → "figure" figure_type "decorative"
- Diagrams or images containing important text → "figure" figure_type "informational" with text_lines
- Return regions top-to-bottom, left-to-right reading order

TABLE RULES (critical):
- Any grid of rows and columns → ALWAYS use "table" type, never break into individual "text" regions
- This includes: forms with boxes, tax declaration tables, numbered lists in a grid, any bordered cells
- In the HTML: copy the ACTUAL text from each cell — never use placeholders like [2.1] or [row][col]
- If a cell is empty, use <td></td>
- Use <th> only for header row cells
- MERGED CELLS: use colspan="N" for horizontal merges, rowspan="N" for vertical merges — match the original layout exactly
- Count rows and columns carefully — every visible row in the original must appear in the HTML
- Example with merged cell: {"type":"table","html":"<table><tr><th colspan=\"2\">Thông tin</th></tr><tr><td>Họ tên</td><td>Nguyễn Văn A</td></tr></table>"}

TEXT FORMATTING RULES:
- Preserve bold, italic, underline from the original using inline HTML tags in the "content" field
- Bold text → wrap with <strong>text</strong>
- Italic text → wrap with <em>text</em>
- Underlined text → wrap with <u>text</u>
- Plain text stays as plain text — only wrap when formatting is clearly visible
- Example: {"type":"text","content":"Bên A có nghĩa vụ <strong>thanh toán đầy đủ</strong> theo hợp đồng."}`

type gptOCRResponse struct {
	Regions []OCRRegion `json:"regions"`
}

// runGPTVisionOCR calls GPT-4o-mini vision API for each page image and returns
// a StructuredOCRResult with the same schema as the Python sidecar.
//
// Each page is a separate API call — pages are processed sequentially to avoid
// hitting rate limits on the vision endpoint.
func runGPTVisionOCR(ctx context.Context, imagePaths []string, apiKey string, onPage func(done, total int)) (*StructuredOCRResult, error) {
	client := openai.NewClient(apiKey)
	total := len(imagePaths)
	result := &StructuredOCRResult{}

	for i, imgPath := range imagePaths {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		pageNo := i + 1

		// Read + optionally resize image for API
		imgData, width, height, err := readImageForAPI(imgPath)
		if err != nil {
			return nil, fmt.Errorf("page %d: đọc ảnh thất bại: %w", pageNo, err)
		}

		// Base64-encode for GPT vision inline image
		b64 := base64.StdEncoding.EncodeToString(imgData)
		dataURL := "data:image/png;base64," + b64

		resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model: gptVisionModel,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: gptVisionSystemPrompt,
				},
				{
					Role: openai.ChatMessageRoleUser,
					MultiContent: []openai.ChatMessagePart{
						{
							Type: openai.ChatMessagePartTypeImageURL,
							ImageURL: &openai.ChatMessageImageURL{
								URL:    dataURL,
								Detail: openai.ImageURLDetailHigh,
							},
						},
						{
							Type: openai.ChatMessagePartTypeText,
							Text: gptVisionUserPrompt,
						},
					},
				},
			},
			MaxTokens:   4096,
			Temperature: 0,
		})
		if err != nil {
			return nil, fmt.Errorf("page %d: GPT vision API error: %w", pageNo, err)
		}

		if len(resp.Choices) == 0 {
			return nil, fmt.Errorf("page %d: GPT returned no choices", pageNo)
		}

		raw := strings.TrimSpace(resp.Choices[0].Message.Content)
		raw = stripJSONFences(raw)

		page := OCRPage{PageNo: pageNo, Width: width, Height: height}
		page.Regions = parseGPTRegions(raw) // tolerates both {"regions":[...]} and [...]
		result.Pages = append(result.Pages, page)

		if onPage != nil {
			onPage(pageNo, total)
		}
	}

	return result, nil
}

// maxAPIImageWidth is the maximum pixel width sent to the OpenAI vision API.
// Images wider than this are resized proportionally to avoid request body size
// limits that cause "could not parse JSON body" 400 errors.
const maxAPIImageWidth = 1600

// readImageForAPI reads the image at path, resizes it if wider than maxAPIImageWidth,
// and returns the PNG bytes along with the (possibly resized) dimensions.
func readImageForAPI(path string) ([]byte, int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		// Fallback: return raw bytes without resize
		raw, rerr := os.ReadFile(path)
		return raw, 0, 0, rerr
	}

	origW := img.Bounds().Dx()
	origH := img.Bounds().Dy()

	if origW <= maxAPIImageWidth {
		// No resize needed — re-encode as PNG
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			raw, _ := os.ReadFile(path)
			return raw, origW, origH, nil
		}
		return buf.Bytes(), origW, origH, nil
	}

	// Resize proportionally
	newW := maxAPIImageWidth
	newH := origH * newW / origW
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		raw, _ := os.ReadFile(path)
		return raw, origW, origH, nil
	}
	return buf.Bytes(), newW, newH, nil
}

// imageDimensions returns width and height of the image at path.
// Returns (0, 0) on error — callers treat zero dims as "unknown".
func imageDimensions(path string) (int, int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// parseGPTRegions parses regions from GPT JSON output, tolerating two formats:
//  1. {"regions": [...]}  — the requested wrapper format
//  2. [...]               — direct array (GPT-4o sometimes skips the wrapper)
func parseGPTRegions(raw string) []OCRRegion {
	// Try wrapped format first
	var wrapped gptOCRResponse
	if json.Unmarshal([]byte(raw), &wrapped) == nil && len(wrapped.Regions) > 0 {
		return wrapped.Regions
	}
	// Try bare array
	var regions []OCRRegion
	if json.Unmarshal([]byte(raw), &regions) == nil {
		return regions
	}
	return nil
}

// stripJSONFences removes ```json ... ``` or ``` ... ``` wrappers that GPT sometimes adds.
func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	// Remove opening fence line (e.g. "```json\n")
	if idx := strings.Index(s, "\n"); idx >= 0 {
		s = s[idx+1:]
	}
	// Remove closing fence
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
