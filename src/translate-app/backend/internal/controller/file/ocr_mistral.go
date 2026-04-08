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
	"io"
	"net/http"
	"os"
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

	return &result, nil
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
