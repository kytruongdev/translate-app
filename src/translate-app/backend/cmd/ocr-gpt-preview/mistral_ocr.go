package main

// Mistral OCR integration.
// API docs: https://docs.mistral.ai/capabilities/document/
// Sends the full PDF in one request, gets back per-page markdown.
// Markdown tables are preserved and converted to HTML for the preview.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
)

const mistralOCREndpoint = "https://api.mistral.ai/v1/ocr"
const mistralOCRModel = "mistral-ocr-latest"

// ── Mistral API types ─────────────────────────────────────────────────────────

type mistralOCRRequest struct {
	Model    string          `json:"model"`
	Document mistralDocument `json:"document"`
	Pages    []int           `json:"pages,omitempty"` // 0-indexed, optional
}

type mistralDocument struct {
	Type        string `json:"type"`
	DocumentURL string `json:"document_url,omitempty"`
}

type mistralOCRResponse struct {
	Pages []mistralPage `json:"pages"`
}

type mistralPage struct {
	Index    int    `json:"index"`
	Markdown string `json:"markdown"`
}

// ── Main entry point ──────────────────────────────────────────────────────────

// runMistralOCR sends the PDF to Mistral OCR and returns pageResults.
// pageFilter is 1-indexed (same convention as CLI --pages flag); nil = all pages.
func runMistralOCR(pdfPath string, pageFilter map[int]bool, apiKey string) ([]pageResult, error) {
	// Read PDF
	pdfData, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("read PDF: %w", err)
	}
	b64 := base64.StdEncoding.EncodeToString(pdfData)
	dataURL := "data:application/pdf;base64," + b64

	// Build page list (Mistral uses 0-indexed)
	var pages []int
	if pageFilter != nil {
		for p := range pageFilter {
			pages = append(pages, p-1) // convert to 0-indexed
		}
	}

	req := mistralOCRRequest{
		Model:    mistralOCRModel,
		Document: mistralDocument{Type: "document_url", DocumentURL: dataURL},
		Pages:    pages,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", mistralOCREndpoint, bytes.NewReader(body))
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
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Mistral API error %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}

	var ocrResp mistralOCRResponse
	if err := json.Unmarshal(respBody, &ocrResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	var results []pageResult
	for _, p := range ocrResp.Pages {
		pageNo := p.Index + 1 // convert to 1-indexed
		fmt.Fprintf(os.Stderr, "\n=== RAW MISTRAL PAGE %d ===\n%s\n=== END PAGE %d ===\n",
			pageNo, p.Markdown, pageNo)
		regions := markdownToRegions(p.Markdown)
		results = append(results, pageResult{
			pageNo:  pageNo,
			regions: regions,
		})
	}
	return results, nil
}

// ── Markdown → regions converter ─────────────────────────────────────────────

var (
	reMDTable = regexp.MustCompile(`(?m)^\|.+\|[ \t]*$`)
	reHeading = regexp.MustCompile(`^(#{1,4})\s+(.+)`)
	reImgTag  = regexp.MustCompile(`!\[.*?\]\(.*?\)`)
)

// markdownToRegions converts Mistral markdown output to []region for HTML rendering.
func markdownToRegions(md string) []region {
	var regions []region

	// Split into blocks: blank-line separated
	blocks := splitBlocks(md)
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		// Image reference → figure
		if reImgTag.MatchString(block) {
			regions = append(regions, region{Type: "figure", FigureType: "decorative"})
			continue
		}

		// Markdown table block
		if looksLikeTable(block) {
			html := mdTableToHTML(block)
			regions = append(regions, region{Type: "table", HTML: html})
			continue
		}

		// Heading
		if m := reHeading.FindStringSubmatch(block); m != nil {
			regions = append(regions, region{Type: "title", Content: m[2], Alignment: "left"})
			continue
		}

		// Plain text / paragraph
		regions = append(regions, region{Type: "text", Content: block, Alignment: "left"})
	}
	return regions
}

// splitBlocks splits markdown by blank lines, keeping table blocks together.
func splitBlocks(md string) []string {
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
		if strings.TrimSpace(line) == "" {
			flush()
		} else {
			cur = append(cur, line)
		}
	}
	flush()
	return blocks
}

// looksLikeTable returns true if the block contains markdown table rows.
func looksLikeTable(block string) bool {
	lines := strings.Split(block, "\n")
	tableLines := 0
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "|") && strings.HasSuffix(l, "|") {
			tableLines++
		}
	}
	return tableLines >= 2
}

// mdTableToHTML converts a markdown table to an HTML table.
func mdTableToHTML(block string) string {
	lines := strings.Split(block, "\n")
	var rows [][]string
	var isHeader []bool

	headerDone := false
	separatorSeen := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "|") {
			continue
		}

		// Separator row: |---|---|
		if isSeparatorRow(line) {
			separatorSeen = true
			headerDone = true
			continue
		}

		cells := splitTableRow(line)
		rows = append(rows, cells)
		isHeader = append(isHeader, !headerDone && !separatorSeen)
		_ = separatorSeen
	}

	var b strings.Builder
	b.WriteString(`<table border="1" style="border-collapse:collapse;width:100%">`)
	for i, row := range rows {
		if isHeader[i] {
			b.WriteString("<thead><tr>")
			for _, cell := range row {
				b.WriteString("<th>")
				b.WriteString(cell)
				b.WriteString("</th>")
			}
			b.WriteString("</tr></thead><tbody>")
		} else {
			b.WriteString("<tr>")
			for _, cell := range row {
				b.WriteString("<td>")
				b.WriteString(cell)
				b.WriteString("</td>")
			}
			b.WriteString("</tr>")
		}
	}
	b.WriteString("</tbody></table>")
	return b.String()
}

func isSeparatorRow(line string) bool {
	inner := strings.Trim(line, "|")
	cells := strings.Split(inner, "|")
	for _, c := range cells {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		// must be like ---, :---, ---:, :---:
		stripped := strings.Trim(c, ":-")
		if stripped != "" {
			return false
		}
	}
	return true
}

func splitTableRow(line string) []string {
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	var cells []string
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}
