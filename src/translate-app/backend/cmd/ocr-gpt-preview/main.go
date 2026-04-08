// ocr-gpt-preview: chạy GPT-4o-mini vision OCR trên PDF và render HTML để inspect.
//
// Usage:
//
//	go run ./cmd/ocr-gpt-preview <pdf_file> [output.html]
//	go run ./cmd/ocr-gpt-preview <pdf_file> --pages 8,10,13
//
// Output HTML hiển thị từng page với regions được detect:
//   - TITLE: tiêu đề / heading
//   - TEXT:  đoạn văn bản
//   - TABLE: bảng HTML
//   - FIGURE: stamp / seal / hình ảnh
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gen2brain/go-fitz"
	openai "github.com/sashabaranov/go-openai"
	xdraw "golang.org/x/image/draw"

	"translate-app/config"
)

// ── GPT prompts ───────────────────────────────────────────────────────────────

const gptModel = "o4-mini"

const systemPrompt = `You are a precise OCR engine for Vietnamese legal documents.
Extract all visible content from the page image in reading order.
Return ONLY valid JSON — no markdown fences, no explanation.`

// pass1UserPrompt: single-pass layout extraction with improved table/form distinction.
const pass1UserPrompt = `Extract the page content and return this exact JSON:
{
  "regions": [
    {"type": "text",   "content": "exact Vietnamese text", "alignment": "left|center|right"},
    {"type": "title",  "content": "heading or section label", "alignment": "left|center|right"},
    {"type": "table",  "html": "<table>...</table>"},
    {"type": "figure", "figure_type": "decorative"},
    {"type": "figure", "figure_type": "informational", "text_lines": ["line1", "line2"]}
  ]
}

GENERAL RULES:
- Copy Vietnamese text EXACTLY — preserve all diacritics, do NOT translate
- alignment: "center" if visually centered, "right" if right-aligned, else "left"
- Use "title" for headings, article numbers (Điều X), section labels
- Stamps, seals, signatures, logos → "figure" figure_type "decorative"
- Return regions top-to-bottom, left-to-right reading order

TEXT PARAGRAPH RULES:
- Each visually distinct paragraph or numbered item → its own "text" region
- Do NOT merge multiple paragraphs into one region
- If a paragraph has multiple lines, keep them joined with a space (they are one sentence)
- Use \n to separate clearly distinct lines within one region (e.g. address lines)
- Example: "- Bên A có nghĩa vụ thanh toán.\n- Bên B có quyền nhận tiền." → two items in ONE region separated by \n

TABLE vs FORM — read carefully:
- TABLE: a grid where multiple rows contain comparable data across the same columns.
  Example: a list of taxpayers with columns STT / Họ tên / Mã số thuế / Số tiền
  → use type "table" with full HTML

- FORM / NOTICE: a document section with labeled fields, declarations, or free-form paragraphs
  that happen to be arranged in boxes or indented lines. No repeating data rows.
  Example: "Tên người nộp thuế: ___", "Địa chỉ: ___", "Số điện thoại: ___"
  → use type "text", write each line as "Label: Value" separated by newlines

TABLE EXTRACTION RULES (when type IS table):
- Before writing HTML: mentally count the number of columns in the widest row
- Every row in the output must have exactly that many cells (use <td></td> for empty cells)
- Merged cells: colspan="N" for horizontal span, rowspan="N" for vertical span
- Copy ACTUAL cell text — never use placeholders like [2.1] or [row][col]
- Use <th> for header row cells only

TEXT FORMATTING:
- Bold → <strong>text</strong>, Italic → <em>text</em>, Underline → <u>text</u>
- Only apply when formatting is clearly visible`

// tableExtractionPrompt: focused prompt for pass 2 — single table crop.
const tableExtractionPrompt = `This image contains a single table from a Vietnamese legal document.
Extract it as an HTML table. Return ONLY a JSON object: {"html": "<table>...</table>"}

Rules:
- Copy ALL rows and columns — do not skip any row
- Copy EXACT Vietnamese text from each cell — do NOT translate
- Empty cell → <td></td>
- Header cells → <th>text</th>
- Horizontally merged cells → <td colspan="N">text</td>
- Vertically merged cells → <td rowspan="N">text</td>
- If this is a form (labeled fields, not a data grid) → still output as table with label in first column, value in second
- No placeholder like [row][col] — always copy actual text`

// ── Structs ───────────────────────────────────────────────────────────────────

type region struct {
	Type       string   `json:"type"`
	FigureType string   `json:"figure_type,omitempty"`
	Content    string   `json:"content,omitempty"`
	Alignment  string   `json:"alignment,omitempty"`
	HTML       string   `json:"html,omitempty"`
	TextLines  []string `json:"text_lines,omitempty"`
	BBox       []int    `json:"bbox,omitempty"` // [x1,y1,x2,y2] in resized image coords
}

type gptResponse struct {
	Regions []region `json:"regions"`
}

type pageResult struct {
	pageNo  int
	width   int
	height  int
	regions []region
	err     error
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: go run ./cmd/ocr-gpt-preview <pdf_file> [output.html] [--pages 1,2,3]")
		os.Exit(1)
	}

	pdfPath := args[0]
	outPath := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath)) + "_gpt_preview_v4.html"
	var pageFilter map[int]bool
	engine := "gpt" // default

	for i := 1; i < len(args); i++ {
		switch {
		case args[i] == "--pages" && i+1 < len(args):
			pageFilter = parsePageList(args[i+1])
			i++
		case args[i] == "--engine" && i+1 < len(args):
			engine = args[i+1]
			i++
		case !strings.HasPrefix(args[i], "--"):
			outPath = args[i]
		}
	}

	// 1. Render PDF pages to PNGs (needed for GPT; Mistral uses raw PDF)
	fmt.Printf("PDF: %s\n", filepath.Base(pdfPath))

	var pages []pageResult

	if engine == "mistral" {
		// ── Mistral OCR path ──────────────────────────────────────────────
		mistralKey := config.Keys.MistralKey
		if mistralKey == "" {
			fmt.Fprintln(os.Stderr, "ERROR: MistralKey not set in config/keys.go")
			os.Exit(1)
		}
		if pageFilter != nil {
			fmt.Printf("  Testing pages: %s\n", formatPageFilter(pageFilter))
		}
		fmt.Println("Running Mistral OCR…")
		var err error
		pages, err = runMistralOCR(pdfPath, pageFilter, mistralKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		for _, p := range pages {
			fmt.Printf("  Page %d… %d regions\n", p.pageNo, len(p.regions))
		}
	} else if engine == "pixtral" {
		// ── Mistral Vision (Pixtral) path ─────────────────────────────────
		mistralKey := config.Keys.MistralKey
		if mistralKey == "" {
			fmt.Fprintln(os.Stderr, "ERROR: MistralKey not set in config/keys.go")
			os.Exit(1)
		}
		if pageFilter != nil {
			fmt.Printf("  Testing pages: %s\n", formatPageFilter(pageFilter))
		}
		fmt.Println("Running Pixtral vision OCR…")
		var err error
		pages, err = runMistralVision(pdfPath, pageFilter, mistralKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
	} else {
		// ── GPT vision path ───────────────────────────────────────────────
		apiKey := config.Keys.OpenAIKey
		if apiKey == "" {
			fmt.Fprintln(os.Stderr, "ERROR: OpenAI API key not set in config/keys.go")
			os.Exit(1)
		}

		fmt.Println("Rendering pages…")
		imagePaths, tempDir, err := renderPDF(pdfPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		defer os.RemoveAll(tempDir)
		fmt.Printf("  %d pages rendered\n", len(imagePaths))

		if pageFilter != nil {
			var filtered []string
			for i, p := range imagePaths {
				if pageFilter[i+1] {
					filtered = append(filtered, p)
				}
			}
			fmt.Printf("  Testing pages: %s\n", formatPageFilter(pageFilter))
			imagePaths = filtered
		}

		fmt.Printf("Running GPT vision OCR (%s, %d pages)…\n", gptModel, len(imagePaths))
		client := openai.NewClient(apiKey)
		ctx := context.Background()

		for i, imgPath := range imagePaths {
			pageNo := i + 1
			if pageFilter != nil {
				pageNo = pageNoFromPath(imgPath)
			}
			fmt.Printf("  Page %d/%d… ", pageNo, len(imagePaths))

			pr := ocrPage(ctx, client, imgPath, pageNo)
			pages = append(pages, pr)

			if pr.err != nil {
				fmt.Printf("ERROR: %v\n", pr.err)
			} else {
				fmt.Printf("%d regions\n", len(pr.regions))
			}
		}
	}

	// 4. Render HTML
	html := renderHTML(filepath.Base(pdfPath), pages)
	if err := os.WriteFile(outPath, []byte(html), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR writing HTML: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nDone → %s\n", outPath)
	fmt.Println("  Open in browser to inspect.")
}

// ── OCR ───────────────────────────────────────────────────────────────────────

func ocrPage(ctx context.Context, client *openai.Client, imgPath string, pageNo int) pageResult {
	imgData, w, h, err := resizeForAPI(imgPath)
	if err != nil {
		return pageResult{pageNo: pageNo, err: err}
	}
	regions := callGPT(ctx, client, imgData, pass1UserPrompt, 16000)
	if regions == nil {
		return pageResult{pageNo: pageNo, width: w, height: h, err: fmt.Errorf("OCR failed")}
	}
	return pageResult{pageNo: pageNo, width: w, height: h, regions: regions}
}

// ocrPageTwoPass runs a 2-pass OCR:
//   - Pass 1: full page → detect all regions; tables return bbox only (no HTML)
//   - Pass 2: for each table bbox → crop from original image → extract table HTML
func ocrPageTwoPass(ctx context.Context, client *openai.Client, imgPath string, pageNo int) pageResult {
	// Pass 1: layout detection with resized image
	imgData, resizedW, resizedH, err := resizeForAPI(imgPath)
	if err != nil {
		return pageResult{pageNo: pageNo, err: err}
	}

	regions := callGPT(ctx, client, imgData, pass1UserPrompt, 16000)
	if regions == nil {
		return pageResult{pageNo: pageNo, width: resizedW, height: resizedH,
			err: fmt.Errorf("pass 1 failed")}
	}

	// Pass 2: extract each table from a cropped region of the original image
	tableCount := 0
	for i, r := range regions {
		if r.Type != "table" || len(r.BBox) != 4 {
			continue
		}
		tableCount++
		html, err := extractTableCrop(ctx, client, imgPath, r.BBox, resizedW, resizedH)
		if err != nil || html == "" {
			// fallback: keep as table type but empty HTML — visible as error in preview
			regions[i].HTML = ""
			continue
		}
		regions[i].HTML = html
	}

	if tableCount > 0 {
		fmt.Printf("(+%d table crops) ", tableCount)
	}

	return pageResult{pageNo: pageNo, width: resizedW, height: resizedH, regions: regions}
}

// callGPT sends an image to GPT and returns parsed regions.
func callGPT(ctx context.Context, client *openai.Client, imgData []byte, prompt string, maxTokens int) []region {
	b64 := base64.StdEncoding.EncodeToString(imgData)
	dataURL := "data:image/png;base64," + b64

	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: gptModel,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{URL: dataURL, Detail: openai.ImageURLDetailHigh}},
					{Type: openai.ChatMessagePartTypeText, Text: prompt},
				},
			},
		},
		MaxCompletionTokens: maxTokens,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "  [callGPT error] %v\n", err)
		return nil
	}
	if len(resp.Choices) == 0 {
		fmt.Fprintf(os.Stderr, "  [callGPT error] no choices returned\n")
		return nil
	}
	raw := stripFences(strings.TrimSpace(resp.Choices[0].Message.Content))
	if raw == "" {
		fmt.Fprintf(os.Stderr, "  [callGPT] empty content, finish_reason=%s\n", resp.Choices[0].FinishReason)
		return nil
	}
	regions := parseRegions(raw)
	if regions == nil {
		fmt.Fprintf(os.Stderr, "  [callGPT] parseRegions failed, raw[:200]=%q\n", truncate(raw, 200))
	}
	return regions
}

// extractTableCrop crops the table region from the original image and calls GPT
// with a focused table extraction prompt. Returns the HTML table string.
func extractTableCrop(ctx context.Context, client *openai.Client, imgPath string, bbox []int, resizedW, resizedH int) (string, error) {
	// Load original image (full resolution, no resize)
	f, err := os.Open(imgPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	origImg, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}
	origW := origImg.Bounds().Dx()
	origH := origImg.Bounds().Dy()

	// Scale bbox from resized image coords to original image coords
	// Add 2% padding around the table for context
	scaleX := float64(origW) / float64(resizedW)
	scaleY := float64(origH) / float64(resizedH)
	padX := int(float64(origW) * 0.01)
	padY := int(float64(origH) * 0.01)

	x1 := max(0, int(float64(bbox[0])*scaleX)-padX)
	y1 := max(0, int(float64(bbox[1])*scaleY)-padY)
	x2 := min(origW, int(float64(bbox[2])*scaleX)+padX)
	y2 := min(origH, int(float64(bbox[3])*scaleY)+padY)

	if x2-x1 < 10 || y2-y1 < 10 {
		return "", fmt.Errorf("bbox too small")
	}

	// Crop
	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	si, ok := origImg.(subImager)
	if !ok {
		return "", fmt.Errorf("image does not support SubImage")
	}
	crop := si.SubImage(image.Rect(x1, y1, x2, y2))

	// Encode crop to PNG bytes
	var buf bytes.Buffer
	if err := png.Encode(&buf, crop); err != nil {
		return "", err
	}

	// Call GPT with focused table extraction prompt
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	dataURL := "data:image/png;base64," + b64

	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: gptModel,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{URL: dataURL, Detail: openai.ImageURLDetailHigh}},
					{Type: openai.ChatMessagePartTypeText, Text: tableExtractionPrompt},
				},
			},
		},
		MaxCompletionTokens: 4096,
	})
	if err != nil || len(resp.Choices) == 0 {
		return "", fmt.Errorf("table extraction API error: %w", err)
	}

	raw := stripFences(strings.TrimSpace(resp.Choices[0].Message.Content))

	// Parse {"html": "..."}
	var result struct {
		HTML string `json:"html"`
	}
	if json.Unmarshal([]byte(raw), &result) == nil && result.HTML != "" {
		return result.HTML, nil
	}
	// Fallback: raw output might be bare HTML
	if strings.Contains(raw, "<table") {
		return raw, nil
	}
	return "", fmt.Errorf("no table HTML in response")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ── PDF rendering ─────────────────────────────────────────────────────────────

func renderPDF(pdfPath string) ([]string, string, error) {
	doc, err := fitz.New(pdfPath)
	if err != nil {
		return nil, "", err
	}
	defer doc.Close()

	tempDir, err := os.MkdirTemp("", "ocr-gpt-preview-*")
	if err != nil {
		return nil, "", err
	}

	var paths []string
	for i := 0; i < doc.NumPage(); i++ {
		img, err := doc.ImageDPI(i, 200)
		if err != nil {
			os.RemoveAll(tempDir)
			return nil, "", fmt.Errorf("render page %d: %w", i+1, err)
		}
		p := filepath.Join(tempDir, fmt.Sprintf("page-%04d.png", i+1))
		f, _ := os.Create(p)
		png.Encode(f, img)
		f.Close()
		paths = append(paths, p)
	}
	return paths, tempDir, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

const maxAPIWidth = 1600

// resizeForAPI reads an image and resizes it if wider than maxAPIWidth
// to avoid OpenAI 400 errors from oversized request bodies.
func resizeForAPI(path string) ([]byte, int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		raw, rerr := os.ReadFile(path)
		return raw, 0, 0, rerr
	}

	origW := img.Bounds().Dx()
	origH := img.Bounds().Dy()

	var buf bytes.Buffer
	if origW <= maxAPIWidth {
		if err := png.Encode(&buf, img); err != nil {
			raw, _ := os.ReadFile(path)
			return raw, origW, origH, nil
		}
		return buf.Bytes(), origW, origH, nil
	}

	newW := maxAPIWidth
	newH := origH * newW / origW
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Over, nil)

	if err := png.Encode(&buf, dst); err != nil {
		raw, _ := os.ReadFile(path)
		return raw, origW, origH, nil
	}
	return buf.Bytes(), newW, newH, nil
}

// parseRegions tolerates {"regions":[...]} and bare [...] from GPT.
func parseRegions(raw string) []region {
	var wrapped gptResponse
	if json.Unmarshal([]byte(raw), &wrapped) == nil && len(wrapped.Regions) > 0 {
		return wrapped.Regions
	}
	var regions []region
	if json.Unmarshal([]byte(raw), &regions) == nil {
		return regions
	}
	return nil
}

func stripFences(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if idx := strings.Index(s, "\n"); idx >= 0 {
		s = s[idx+1:]
	}
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func parsePageList(s string) map[int]bool {
	m := map[int]bool{}
	for _, part := range strings.Split(s, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil {
			m[n] = true
		}
	}
	return m
}

func formatPageFilter(m map[int]bool) string {
	var ns []string
	for n := range m {
		ns = append(ns, strconv.Itoa(n))
	}
	return strings.Join(ns, ",")
}

func pageNoFromPath(p string) int {
	base := strings.TrimSuffix(filepath.Base(p), ".png")
	// filename: page-0008.png → 8
	parts := strings.Split(base, "-")
	if len(parts) >= 2 {
		n, err := strconv.Atoi(parts[len(parts)-1])
		if err == nil {
			return n
		}
	}
	return 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── HTML rendering ────────────────────────────────────────────────────────────

const css = `
body {
    font-family: 'Times New Roman', Times, serif;
    line-height: 1.6;
    color: #333;
    max-width: 900px;
    margin: 0 auto;
    padding: 20px;
    background: #f0f0f0;
}
.toolbar {
    background: #fff;
    border-radius: 6px;
    padding: 10px 16px;
    margin-bottom: 20px;
    font-size: .85em;
    color: #555;
    font-family: 'Segoe UI', sans-serif;
    display: flex;
    align-items: center;
    gap: 16px;
    box-shadow: 0 1px 3px rgba(0,0,0,.1);
}
.toolbar strong { color: #222; }
.toolbar label { display: flex; align-items: center; gap: 6px; cursor: pointer; }
.page {
    background: white;
    padding: 60px;
    margin-bottom: 30px;
    box-shadow: 0 4px 6px rgba(0,0,0,.1);
    position: relative;
    min-height: 400px;
}
.page-label {
    font-family: 'Segoe UI', sans-serif;
    font-size: .7em;
    color: #aaa;
    position: absolute;
    top: 10px;
    right: 16px;
}
h2 { font-size: 1.1em; font-weight: bold; margin: 16px 0 8px; }
p { margin: 0 0 10px; text-align: justify; }
table { width: 100%%; border-collapse: collapse; margin: 16px 0; }
td, th { border: 1px solid #555; padding: 6px 8px; vertical-align: top; font-size: .95em; }
th { background: #f5f5f5; font-weight: bold; }
.figure-placeholder {
    font-family: 'Segoe UI', sans-serif;
    font-size: .8em;
    color: #999;
    border: 1px dashed #ccc;
    padding: 8px 12px;
    margin: 8px 0;
    border-radius: 3px;
    display: none;  /* hidden by default */
}
.show-figures .figure-placeholder { display: block; }
.error-box {
    font-family: 'Segoe UI', sans-serif;
    background: #fff0f0;
    border: 1px solid #e74c3c;
    padding: 8px 12px;
    border-radius: 4px;
    font-size: .85em;
    color: #c0392b;
    margin: 8px 0;
}
@media print {
    body { background: white; margin: 0; }
    .page { box-shadow: none; margin: 0; page-break-after: always; }
    .toolbar { display: none; }
}
`

var (
	reMDBold   = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reMDItalic = regexp.MustCompile(`\*([^*\n]+?)\*`)
)

func esc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// contentHTML renders region content as HTML:
// - Converts markdown **bold** and *italic* to <strong>/<em>
// - Preserves inline tags (<strong>, <em>, <u>) from GPT
// - Converts \n to <br> so line breaks are visible
// - Escapes plain text
func contentHTML(s string) string {
	// Convert markdown inline syntax first (before escaping)
	s = reMDBold.ReplaceAllString(s, "<strong>$1</strong>")
	s = reMDItalic.ReplaceAllString(s, "<em>$1</em>")

	hasInline := strings.Contains(s, "<strong>") || strings.Contains(s, "<em>") || strings.Contains(s, "<u>")
	if hasInline {
		return strings.ReplaceAll(s, "\n", "<br>")
	}
	return strings.ReplaceAll(esc(s), "\n", "<br>")
}

func renderHTML(pdfName string, pages []pageResult) string {
	var b strings.Builder

	totalRegions := 0
	for _, p := range pages {
		totalRegions += len(p.regions)
	}

	b.WriteString("<!DOCTYPE html><html lang='vi'><head><meta charset='UTF-8'>")
	b.WriteString(fmt.Sprintf("<title>GPT OCR Preview — %s</title>", esc(pdfName)))
	b.WriteString(fmt.Sprintf("<style>%s</style></head><body>", css))

	// Toolbar
	b.WriteString("<div class='toolbar' id='toolbar'>")
	b.WriteString(fmt.Sprintf("<strong>%s</strong> &nbsp;·&nbsp; %d pages &nbsp;·&nbsp; %d regions",
		esc(pdfName), len(pages), totalRegions))
	b.WriteString(`<label><input type="checkbox" onchange="document.body.classList.toggle('show-figures',this.checked)"> Show figure placeholders</label>`)
	b.WriteString("</div>")

	for _, pg := range pages {
		b.WriteString(fmt.Sprintf("<div class='page' id='page-%d'>", pg.pageNo))
		b.WriteString(fmt.Sprintf("<div class='page-label'>Page %d</div>", pg.pageNo))

		if pg.err != nil {
			b.WriteString(fmt.Sprintf("<div class='error-box'>OCR error: %s</div>", esc(pg.err.Error())))
		}

		for _, r := range pg.regions {
			alignStyle := ""
			if r.Alignment == "center" || r.Alignment == "right" {
				alignStyle = fmt.Sprintf(" style='text-align:%s'", r.Alignment)
			}

			switch r.Type {
			case "title":
				b.WriteString(fmt.Sprintf("<h2%s>%s</h2>", alignStyle, contentHTML(r.Content)))

			case "text":
				b.WriteString(fmt.Sprintf("<p%s>%s</p>", alignStyle, contentHTML(r.Content)))

			case "table":
				b.WriteString(r.HTML)

			case "figure":
				label := "🖼 [Stamp / Seal / Signature]"
				if r.FigureType == "informational" && len(r.TextLines) > 0 {
					label = "🖼 [Figure: " + esc(strings.Join(r.TextLines, " · ")) + "]"
				}
				b.WriteString(fmt.Sprintf("<div class='figure-placeholder'>%s</div>", label))
			}
		}

		b.WriteString("</div>")
	}

	b.WriteString("</body></html>")
	return b.String()
}
