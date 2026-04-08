package file

// ocr_mistral.go — Gọi Mistral OCR API (mistral-ocr-latest) để extract structured
// content từ PDF và trả về *StructuredOCRResult (cùng type với GPT-4o vision path).
//
// Mistral nhận toàn bộ PDF dạng base64 data URL, trả về per-page markdown.
// Markdown được parse thành []OCRRegion (text/title/table/figure) giống như
// sidecar Python — pipeline_pdf_structured.go không cần biết engine nào được dùng.
//
// Không cần render PNG trước → nhanh hơn và không cần go-fitz ở bước OCR.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"os"

	"github.com/gen2brain/go-fitz"
	"regexp"
	"strings"
)

const (
	mistralInternalOCREndpoint = "https://api.mistral.ai/v1/ocr"
	mistralInternalOCRModel    = "mistral-ocr-latest"
)

// ── Mistral API types ─────────────────────────────────────────────────────────

type mistralInternalOCRRequest struct {
	Model    string                  `json:"model"`
	Document mistralInternalDocument `json:"document"`
}

type mistralInternalDocument struct {
	Type        string `json:"type"`
	DocumentURL string `json:"document_url,omitempty"`
}

type mistralInternalOCRResponse struct {
	Pages []mistralInternalPage `json:"pages"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type mistralInternalPage struct {
	Index    int    `json:"index"`
	Markdown string `json:"markdown"`
}

// ── Entry point ───────────────────────────────────────────────────────────────

// runMistralOCR sends the PDF to Mistral OCR and returns a *StructuredOCRResult
// compatible with the rest of the PDF translation pipeline.
//
// onPage (optional) is called after each page is parsed: onPage(done, total).
func runMistralOCR(ctx context.Context, pdfPath string, apiKey string, onPage func(done, total int)) (*StructuredOCRResult, error) {
	pdfData, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("đọc PDF: %w", err)
	}
	b64 := base64.StdEncoding.EncodeToString(pdfData)
	dataURL := "data:application/pdf;base64," + b64

	reqBody := mistralInternalOCRRequest{
		Model:    mistralInternalOCRModel,
		Document: mistralInternalDocument{Type: "document_url", DocumentURL: dataURL},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", mistralInternalOCREndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("đọc response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Mistral API lỗi %d: %s", resp.StatusCode, truncateMistral(string(respBody), 300))
	}

	var ocrResp mistralInternalOCRResponse
	if err := json.Unmarshal(respBody, &ocrResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if ocrResp.Error != nil {
		return nil, fmt.Errorf("Mistral API error: %s", ocrResp.Error.Message)
	}

	total := len(ocrResp.Pages)
	var result StructuredOCRResult
	for _, p := range ocrResp.Pages {
		pageNo := p.Index + 1
		regions := mistralMarkdownToRegions(p.Markdown)
		var ocrRegions []OCRRegion
		for _, r := range regions {
			ocrRegions = append(ocrRegions, OCRRegion{
				Type:       r.typ,
				Content:    r.content,
				Alignment:  r.alignment,
				HTML:       r.html,
				FigureType: r.figureType,
			})
		}
		result.Pages = append(result.Pages, OCRPage{
			PageNo:  pageNo,
			Regions: ocrRegions,
		})
		if onPage != nil {
			onPage(len(result.Pages), total)
		}
	}

	// Pass 2: fix cross-page tables (table header on page N, data rows on page N+1)
	fixMistralCrossPageTables(&result, pdfPath, apiKey)

	return &result, nil
}

// ── Cross-page table fix ──────────────────────────────────────────────────────

// fixMistralCrossPageTables scans for tables with placeholder-only data rows
// (e.g. [2.1], [2.2]) — a sign the real data is on the next page.
// It stitches the boundary region of consecutive pages into one PNG and
// re-OCRs with Mistral so the full cross-page table is captured in one context.
func fixMistralCrossPageTables(result *StructuredOCRResult, pdfPath, apiKey string) {
	for i := 0; i < len(result.Pages)-1; i++ {
		tableIdx := -1
		for j, r := range result.Pages[i].Regions {
			if r.Type == "table" && mistralIsLikelyCrossPageTable(r.HTML) {
				tableIdx = j
				break
			}
		}
		if tableIdx == -1 {
			continue
		}

		pageNo := result.Pages[i].PageNo
		nextPageNo := result.Pages[i+1].PageNo

		stitchedPNG, err := mistralStitchPages(pdfPath, pageNo, nextPageNo)
		if err != nil {
			continue
		}

		markdown, err := mistralOCRImage(stitchedPNG, apiKey)
		if err != nil {
			continue
		}

		stitchedRegions := mistralMarkdownToRegions(markdown)
		origCols := mistralTableColCount(result.Pages[i].Regions[tableIdx].HTML)

		for _, r := range stitchedRegions {
			if r.typ != "table" || mistralIsLikelyCrossPageTable(r.html) {
				continue
			}
			if mistralTableColCount(r.html) != origCols {
				continue
			}
			merged := mistralMergeCompanionRows(result.Pages[i].Regions[tableIdx].HTML, r.html)
			result.Pages[i].Regions[tableIdx].HTML = merged
			break
		}
	}
}

var mReHasBracket = regexp.MustCompile(`\[\d[\d.]*\]`)

func mistralIsLikelyCrossPageTable(html string) bool {
	if !mReHasBracket.MatchString(html) {
		return false
	}
	tbodyIdx := strings.Index(html, "<tbody>")
	if tbodyIdx == -1 {
		return false
	}
	return strings.Count(html[tbodyIdx:], "<tr>") == 1
}

func mistralTableColCount(html string) int {
	theadIdx := strings.Index(html, "<thead>")
	if theadIdx == -1 {
		return 0
	}
	trEnd := strings.Index(html[theadIdx:], "</tr>")
	if trEnd == -1 {
		return 0
	}
	return strings.Count(html[theadIdx:theadIdx+trEnd], "<th>")
}

var (
	mReThOpen      = regexp.MustCompile(`<th(\b[^>]*)?>`)
	mReThClose     = regexp.MustCompile(`</th>`)
	mReColspanAttr = regexp.MustCompile(`\s+colspan="\d+"`)
)

func mistralMergeCompanionRows(originalHTML, companionHTML string) string {
	reTR := regexp.MustCompile(`(?s)<tr>.*?</tr>`)
	companionRows := reTR.FindAllString(companionHTML, -1)

	var extra strings.Builder
	for _, row := range companionRows {
		row = mReThOpen.ReplaceAllString(row, "<td$1>")
		row = mReThClose.ReplaceAllString(row, "</td>")
		row = mReColspanAttr.ReplaceAllString(row, "")
		extra.WriteString(row)
	}
	return strings.Replace(originalHTML, "</tbody></table>", extra.String()+"</tbody></table>", 1)
}

// mistralStitchPages renders the bottom 25% of page1 + top 40% of page2
// and stitches them into one PNG for cross-page OCR.
func mistralStitchPages(pdfPath string, page1No, page2No int) ([]byte, error) {
	doc, err := fitz.New(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("open PDF: %w", err)
	}
	defer doc.Close()

	img1, err := doc.ImageDPI(page1No-1, 200)
	if err != nil {
		return nil, err
	}
	img2, err := doc.ImageDPI(page2No-1, 200)
	if err != nil {
		return nil, err
	}

	h1 := img1.Bounds().Dy()
	h2 := img2.Bounds().Dy()
	crop1Y := h1 * 75 / 100
	crop2H := h2 * 40 / 100

	bottom1 := img1.SubImage(image.Rect(0, crop1Y, img1.Bounds().Dx(), h1))
	top2 := img2.SubImage(image.Rect(0, 0, img2.Bounds().Dx(), crop2H))

	w := bottom1.Bounds().Dx()
	if top2.Bounds().Dx() > w {
		w = top2.Bounds().Dx()
	}
	h := bottom1.Bounds().Dy() + top2.Bounds().Dy()

	combined := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(combined, image.Rect(0, 0, bottom1.Bounds().Dx(), bottom1.Bounds().Dy()),
		bottom1, bottom1.Bounds().Min, draw.Over)
	draw.Draw(combined, image.Rect(0, bottom1.Bounds().Dy(), top2.Bounds().Dx(), h),
		top2, top2.Bounds().Min, draw.Over)

	var buf bytes.Buffer
	if err := png.Encode(&buf, combined); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// mistralOCRImage sends a PNG to Mistral OCR and returns the markdown text.
func mistralOCRImage(imgData []byte, apiKey string) (string, error) {
	b64 := base64.StdEncoding.EncodeToString(imgData)
	dataURL := "data:image/png;base64," + b64

	reqBody := mistralInternalOCRRequest{
		Model:    mistralInternalOCRModel,
		Document: mistralInternalDocument{Type: "image_url", DocumentURL: dataURL},
	}
	// image_url uses image_url field not document_url — override via raw map
	type imgReq struct {
		Model    string `json:"model"`
		Document struct {
			Type     string `json:"type"`
			ImageURL string `json:"image_url"`
		} `json:"document"`
	}
	req := imgReq{Model: mistralInternalOCRModel}
	req.Document.Type = "image_url"
	req.Document.ImageURL = dataURL
	_ = reqBody

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequest("POST", mistralInternalOCREndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var ocrResp mistralInternalOCRResponse
	if err := json.Unmarshal(respBody, &ocrResp); err != nil {
		return "", err
	}
	if ocrResp.Error != nil {
		return "", fmt.Errorf("Mistral OCR image error: %s", ocrResp.Error.Message)
	}
	var parts []string
	for _, p := range ocrResp.Pages {
		parts = append(parts, p.Markdown)
	}
	return strings.Join(parts, "\n\n"), nil
}

// ── Internal region struct (local to this file) ────────────────────────────────

type mistralRegion struct {
	typ        string // "text", "title", "table", "figure"
	content    string
	alignment  string
	html       string
	figureType string
}

// ── Markdown → regions ────────────────────────────────────────────────────────

var (
	mReMDTable     = regexp.MustCompile(`(?m)^\|.+\|[ \t]*$`)
	mReHeading     = regexp.MustCompile(`^(#{1,4})\s+(.+)`)
	mReImgTag      = regexp.MustCompile(`!\[.*?\]\(.*?\)`)
	mReRomanPrefix = regexp.MustCompile(`(?i)^(X{0,3})(IX|IV|V?I{0,3})\.`)
)

func mistralMarkdownToRegions(md string) []mistralRegion {
	var regions []mistralRegion
	seenTable := false

	blocks := mistralSplitBlocks(md)
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		// Skip CamScanner watermarks
		lower := strings.ToLower(block)
		if strings.Contains(lower, "scanned with") || strings.Contains(lower, "camscanner") {
			continue
		}

		// Skip OCR noise (CJK garbage, single uppercase letters)
		if mistralIsMeaninglessBlock(block) {
			continue
		}

		// Figure (image reference)
		if mReImgTag.MatchString(block) {
			regions = append(regions, mistralRegion{typ: "figure", figureType: "decorative"})
			continue
		}

		// Table
		if mistralLooksLikeTable(block) {
			seenTable = true
			html := mistralMDTableToHTML(block)
			regions = append(regions, mistralRegion{typ: "table", html: html})
			continue
		}

		// Bold-only paragraph → heading (e.g. "**Nơi nhận:**")
		if mistralIsBoldHeading(block) {
			content := strings.TrimSpace(block[2 : len(block)-2])
			regions = append(regions, mistralRegion{typ: "title", content: content, alignment: mistralHeadingAlignment(content)})
			continue
		}

		// Implied heading (ALL-CAPS without # markers) — only before first table.
		// After a table, ALL-CAPS blocks are signature labels/names, not headings.
		if !seenTable && mistralIsImpliedHeadingBlock(block) {
			currentAlign := "left"
			for _, line := range strings.Split(block, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if line == strings.ToUpper(line) && len([]rune(line)) > 3 {
					currentAlign = mistralHeadingAlignment(line)
					regions = append(regions, mistralRegion{typ: "title", content: line, alignment: currentAlign})
				} else if currentAlign == "center" {
					regions = append(regions, mistralRegion{typ: "title", content: line, alignment: "center"})
				} else {
					regions = append(regions, mistralRegion{typ: "text", content: line, alignment: "left"})
				}
			}
			continue
		}

		// Explicit heading (# markers)
		if m := mReHeading.FindStringSubmatch(block); m != nil {
			content := strings.TrimSpace(m[2])
			align := mistralHeadingAlignment(content)
			regions = append(regions, mistralRegion{typ: "title", content: content, alignment: align})

			// Process continuation lines (subtitle on next line without blank)
			lines := strings.Split(block, "\n")
			for _, line := range lines[1:] {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if mm := mReHeading.FindStringSubmatch(line); mm != nil {
					c2 := strings.TrimSpace(mm[2])
					regions = append(regions, mistralRegion{typ: "title", content: c2, alignment: mistralHeadingAlignment(c2)})
				} else {
					regions = append(regions, mistralRegion{typ: "title", content: line, alignment: align})
				}
			}
			continue
		}

		// Plain text — split multi-line blocks into separate paragraphs
		for _, line := range strings.Split(block, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				regions = append(regions, mistralRegion{typ: "text", content: line, alignment: "left"})
			}
		}
	}
	return regions
}

// mistralSplitBlocks splits markdown into logical blocks (blank-line delimited).
// Each line starting with '#' is forced into its own block.
func mistralSplitBlocks(md string) []string {
	lines := strings.Split(md, "\n")
	var blocks []string
	var cur []string

	flush := func() {
		if len(cur) > 0 {
			blocks = append(blocks, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flush()
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			flush()
		}
		cur = append(cur, line)
	}
	flush()
	return blocks
}

func mistralIsMeaninglessBlock(block string) bool {
	runes := []rune(strings.TrimSpace(block))
	if len(runes) == 0 || len(runes) > 4 {
		return false
	}
	if len(runes) == 1 && runes[0] >= 'A' && runes[0] <= 'Z' {
		return true
	}
	for _, r := range runes {
		if r < 0x80 || (r >= 0x00C0 && r <= 0x024F) || (r >= 0x1E00 && r <= 0x1EFF) {
			return false
		}
	}
	return true
}

func mistralIsImpliedHeadingBlock(block string) bool {
	if strings.HasPrefix(block, "#") || strings.HasPrefix(block, "|") ||
		strings.HasPrefix(block, "*") || strings.HasPrefix(block, "-") ||
		strings.HasPrefix(block, "!") || strings.HasPrefix(block, ">") {
		return false
	}
	firstLine := strings.TrimSpace(strings.SplitN(block, "\n", 2)[0])
	runes := []rune(firstLine)
	if len(runes) < 4 || len(runes) > 100 {
		return false
	}
	return firstLine == strings.ToUpper(firstLine)
}

func mistralIsBoldHeading(block string) bool {
	b := strings.TrimSpace(block)
	if !strings.HasPrefix(b, "**") || !strings.HasSuffix(b, "**") || len(b) < 5 {
		return false
	}
	inner := b[2 : len(b)-2]
	return !strings.Contains(inner, "**") && !strings.Contains(inner, "\n")
}

func mistralHeadingAlignment(content string) string {
	c := strings.TrimSpace(content)
	if mReRomanPrefix.MatchString(c) {
		return "left"
	}
	if strings.HasPrefix(strings.ToUpper(c), "BÊN ") {
		return "left"
	}
	if strings.Contains(content, "Độc lập") || strings.Contains(content, "Hạnh phúc") {
		return "center"
	}
	if c == strings.ToUpper(c) && len([]rune(c)) > 3 {
		return "center"
	}
	return "left"
}

// ── Table conversion ──────────────────────────────────────────────────────────

var mRightColumnPrefixes = []string{
	"Nợ TK:", "Có TK:", "No TK:", "Co TK:",
	"Phí:", "Phi:", "VAT:", "Vat:",
}

func mistralIsRightColumnLine(line string) bool {
	t := strings.TrimSpace(line)
	for _, p := range mRightColumnPrefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

func mistralLooksLikeTable(block string) bool {
	joined := mistralJoinTableContinuations(block)
	lines := strings.Split(joined, "\n")
	count := 0
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "|") {
			count++
		}
	}
	return count >= 2
}

func mistralJoinTableContinuations(block string) string {
	lines := strings.Split(block, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			result = append(result, line)
			continue
		}
		if len(result) > 0 && !strings.HasPrefix(trimmed, "|") {
			result[len(result)-1] += "<br>" + trimmed
		} else {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

func mistralMDTableToHTML(block string) string {
	block = mistralJoinTableContinuations(block)
	lines := strings.Split(block, "\n")
	var rows [][]string
	var isHeader []bool
	headerDone := false
	separatorSeen := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "|") {
			continue
		}
		if mistralIsSeparatorRow(line) {
			separatorSeen = true
			headerDone = true
			continue
		}
		cells := mistralSplitTableRow(line)
		rows = append(rows, cells)
		isHeader = append(isHeader, !headerDone && !separatorSeen)
		_ = separatorSeen
	}

	rows = mistralRemoveEmptyColumns(rows)

	// Section box detection
	sectionBox := mistralDetectSectionBox(rows)
	var wasRedistributed bool
	rows, wasRedistributed = mistralRedistributeColumns(rows)
	if wasRedistributed {
		sectionBox = true
	}
	if sectionBox {
		for i := range isHeader {
			isHeader[i] = false
		}
	}

	var b strings.Builder
	b.WriteString(`<table border="1" style="border-collapse:collapse;width:100%">`)
	for i, row := range rows {
		if i < len(isHeader) && isHeader[i] {
			b.WriteString("<thead><tr>")
			mistralWriteRowCells(row, "th", true, &b)
			b.WriteString("</tr></thead><tbody>")
		} else {
			b.WriteString("<tr>")
			mistralWriteRowCells(row, "td", false, &b)
			b.WriteString("</tr>")
		}
	}
	b.WriteString("</tbody></table>")
	return b.String()
}

func mistralIsSeparatorRow(line string) bool {
	inner := strings.Trim(line, "|")
	for _, c := range strings.Split(inner, "|") {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if strings.Trim(c, ":-") != "" {
			return false
		}
	}
	return true
}

func mistralSplitTableRow(line string) []string {
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	var cells []string
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

func mistralRemoveEmptyColumns(rows [][]string) [][]string {
	if len(rows) == 0 {
		return rows
	}
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	keep := make([]bool, maxCols)
	for _, row := range rows {
		for ci, cell := range row {
			if strings.TrimSpace(cell) != "" {
				keep[ci] = true
			}
		}
	}
	keptCount := 0
	for _, k := range keep {
		if k {
			keptCount++
		}
	}
	if keptCount == maxCols {
		return rows
	}
	result := make([][]string, len(rows))
	for ri, row := range rows {
		var newRow []string
		for ci, cell := range row {
			if ci < maxCols && keep[ci] {
				newRow = append(newRow, cell)
			}
		}
		result[ri] = newRow
	}
	return result
}

func mistralDetectSectionBox(rows [][]string) bool {
	for _, row := range rows {
		for _, cell := range row {
			for _, line := range strings.Split(cell, "<br>") {
				if mistralIsRightColumnLine(line) {
					return true
				}
			}
		}
	}
	return false
}

func mistralRedistributeColumns(rows [][]string) ([][]string, bool) {
	if len(rows) == 0 || len(rows[0]) < 2 {
		return rows, false
	}
	for _, row := range rows {
		for _, cell := range row[1:] {
			if strings.TrimSpace(cell) != "" {
				return rows, false
			}
		}
	}
	hasRight := false
	for _, row := range rows {
		for _, line := range strings.Split(row[0], "<br>") {
			if mistralIsRightColumnLine(line) {
				hasRight = true
				break
			}
		}
		if hasRight {
			break
		}
	}
	if !hasRight {
		return rows, false
	}
	result := make([][]string, len(rows))
	for i, row := range rows {
		newRow := make([]string, len(row))
		copy(newRow, row)
		var left, right []string
		for _, line := range strings.Split(row[0], "<br>") {
			if mistralIsRightColumnLine(line) {
				right = append(right, line)
			} else {
				left = append(left, line)
			}
		}
		newRow[0] = strings.Join(left, "<br>")
		newRow[len(newRow)-1] = strings.Join(right, "<br>")
		result[i] = newRow
	}
	return result, true
}

func mistralWriteRowCells(cells []string, tag string, isHeader bool, b *strings.Builder) {
	i := 0
	for i < len(cells) {
		content := cells[i]
		span := 1
		if isHeader && strings.TrimSpace(content) != "" {
			for j := i + 1; j < len(cells) && strings.TrimSpace(cells[j]) == ""; j++ {
				span++
			}
		}
		if span > 1 {
			fmt.Fprintf(b, `<%s colspan="%d">`, tag, span)
		} else {
			b.WriteString("<" + tag + ">")
		}
		b.WriteString(content)
		b.WriteString("</" + tag + ">")
		i += span
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func truncateMistral(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
