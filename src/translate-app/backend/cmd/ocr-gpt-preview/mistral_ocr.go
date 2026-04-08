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
	"image"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gen2brain/go-fitz"
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
	ImageURL    string `json:"image_url,omitempty"`
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
// After the first pass, it runs a cross-page table fix:
// any page whose last region is a table gets re-OCR'd together with the next page
// (as a stitched PNG image) so Mistral sees the full cross-page table in one context.
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

	// Pass 2: fix cross-page tables
	results = fixCrossPageTables(results, pdfPath, apiKey)

	return results, nil
}

// ── Cross-page table fix (Pass 2) ────────────────────────────────────────────

// tableColCount counts the number of <th> cells in the first header row of a table HTML.
// Used to match tables with the same structure across stitched OCR results.
func tableColCount(html string) int {
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

func tableDataColCount(html string) int {
	tbodyIdx := strings.Index(html, "<tbody>")
	if tbodyIdx == -1 {
		return 0
	}
	trStart := strings.Index(html[tbodyIdx:], "<tr>")
	if trStart == -1 {
		return 0
	}
	region := html[tbodyIdx+trStart:]
	trEnd := strings.Index(region, "</tr>")
	if trEnd == -1 {
		return 0
	}
	firstRow := region[:trEnd]
	return strings.Count(firstRow, "<td>") + strings.Count(firstRow, "<th>")
}

// reHasBracket detects bracket placeholders like [2.1], [4.1.2] anywhere in a string.
var reHasBracket = regexp.MustCompile(`\[\d[\d.]*\]`)

// isLikelyCrossPageTable returns true when a table is missing its actual data rows —
// specifically: the tbody has exactly 1 row AND that row contains bracket placeholders.
// This catches templates like "| [2.1] | [2.2] | [2.3] |" which are header templates
// whose real data rows are on the next page.
func isLikelyCrossPageTable(html string) bool {
	// Must contain at least one bracket placeholder somewhere in the HTML
	if !reHasBracket.MatchString(html) {
		return false
	}
	// Count <tr> inside <tbody> (data rows only, not header)
	tbodyIdx := strings.Index(html, "<tbody>")
	if tbodyIdx == -1 {
		return false
	}
	dataRows := strings.Count(html[tbodyIdx:], "<tr>")
	// Exactly 1 data row = template row only, real data is on next page
	return dataRows == 1
}

// fixCrossPageTables scans all table regions on each page. If a table has only
// placeholder data rows (e.g. [2.1], [2.2] — indicating the real data is on the
// next page), it stitches that page + the next page into one PNG, re-OCRs with
// Mistral, and replaces the incomplete table with the complete one.
func fixCrossPageTables(results []pageResult, pdfPath string, apiKey string) []pageResult {
	for i := 0; i < len(results)-1; i++ {
		// Find any table on this page that has only placeholder rows
		tableIdx := -1
		for j, r := range results[i].regions {
			if r.Type == "table" && isLikelyCrossPageTable(r.HTML) {
				tableIdx = j
				break
			}
		}
		if tableIdx == -1 {
			continue
		}

		pageNo := results[i].pageNo
		nextPageNo := results[i+1].pageNo
		fmt.Fprintf(os.Stderr, "\n[cross-page] Page %d has placeholder-only table (region %d) → stitching with page %d\n",
			pageNo, tableIdx, nextPageNo)

		// Stitch the 2 pages into one tall PNG
		stitchedPNG, err := renderAndStitchPages(pdfPath, pageNo, nextPageNo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[cross-page] stitch error: %v\n", err)
			continue
		}

		// Re-OCR the stitched image with Mistral OCR
		markdown, err := callMistralOCRImage(stitchedPNG, apiKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[cross-page] OCR image error: %v\n", err)
			continue
		}
		fmt.Fprintf(os.Stderr, "[cross-page] stitched markdown (%d chars):\n%s\n", len(markdown), markdown)

		// Find the companion table in the stitched result:
		// same column count, not a placeholder-only table → merge its data rows
		// into the original placeholder table (append after existing rows).
		stitchedRegions := markdownToRegions(markdown)

		for _, r := range stitchedRegions {
			if r.Type != "table" || isLikelyCrossPageTable(r.HTML) {
				continue
			}
			// First non-placeholder table in the stitched boundary image is the companion.
			// (Column count matching is unreliable due to colspan/removeEmptyColumns skew.)
			// Merge companion data rows into the original placeholder table
			merged := mergeCompanionRows(results[i].regions[tableIdx].HTML, r.HTML)
			results[i].regions[tableIdx].HTML = merged
			origAfter := strings.Count(merged, "<tr>")
			fmt.Fprintf(os.Stderr, "[cross-page] ✅ Table merged: +%d rows (total=%d)\n",
				strings.Count(r.HTML, "<tr>"), origAfter)
			break
		}
	}
	return results
}

var (
	reThOpen       = regexp.MustCompile(`<th(\b[^>]*)?>`)
	reThClose      = regexp.MustCompile(`</th>`)
	reColspanAttr  = regexp.MustCompile(`\s+colspan="\d+"`)
)

// mergeCompanionRows takes the original placeholder table HTML and appends
// all <tr> rows from the companion table (converting any <th> to <td> so they
// become proper data rows rather than a second header).
// This reconstructs the complete cross-page table from its two halves.
func mergeCompanionRows(originalHTML, companionHTML string) string {
	reTR := regexp.MustCompile(`(?s)<tr>.*?</tr>`)
	companionRows := reTR.FindAllString(companionHTML, -1)

	var extra strings.Builder
	for _, row := range companionRows {
		// Convert <th ...> → <td ...> (companion rows are data rows, not headers)
		row = reThOpen.ReplaceAllString(row, "<td$1>")
		row = reThClose.ReplaceAllString(row, "</td>")
		// Strip colspan from all cells — colspan was applied because the row appeared
		// as a "header" in the stitched OCR, but it's actually a data row
		row = reColspanAttr.ReplaceAllString(row, "")
		extra.WriteString(row)
	}

	// Inject before the closing </tbody></table>
	return strings.Replace(originalHTML, "</tbody></table>", extra.String()+"</tbody></table>", 1)
}

// renderAndStitchPages renders the BOUNDARY REGION between two pages:
// bottom 50% of page1 + top 35% of page2, stitched vertically.
// This focuses Mistral OCR on the cross-page area rather than full pages,
// which avoids truncation and keeps the image compact.
func renderAndStitchPages(pdfPath string, page1No, page2No int) ([]byte, error) {
	doc, err := fitz.New(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("open PDF: %w", err)
	}
	defer doc.Close()

	img1, err := doc.ImageDPI(page1No-1, 200)
	if err != nil {
		return nil, fmt.Errorf("render page %d: %w", page1No, err)
	}
	img2, err := doc.ImageDPI(page2No-1, 200)
	if err != nil {
		return nil, fmt.Errorf("render page %d: %w", page2No, err)
	}

	// Crop tight around the page boundary:
	// bottom 25% of page1 (where the cross-page table header sits)
	// top 20% of page2 (where the orphaned data rows are)
	// Keeping the crop tight removes visual inter-page gaps and context noise.
	h1 := img1.Bounds().Dy()
	h2 := img2.Bounds().Dy()
	crop1Y := h1 * 75 / 100 // start at 75% of page1 → bottom 25%
	crop2H := h2 * 40 / 100 // top 40% of page2

	// *image.RGBA has SubImage directly
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
		return nil, fmt.Errorf("encode PNG: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[cross-page] stitched boundary image: %dx%d px\n", w, h)
	return buf.Bytes(), nil
}

// callMistralOCRImage sends a PNG image to the Mistral OCR API and returns
// the markdown of the first (and only) page in the response.
func callMistralOCRImage(imgData []byte, apiKey string) (string, error) {
	b64 := base64.StdEncoding.EncodeToString(imgData)
	dataURL := "data:image/png;base64," + b64

	req := mistralOCRRequest{
		Model:    mistralOCRModel,
		Document: mistralDocument{Type: "image_url", ImageURL: dataURL},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequest("POST", mistralOCREndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("HTTP: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Mistral API error %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}

	var ocrResp mistralOCRResponse
	if err := json.Unmarshal(respBody, &ocrResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(ocrResp.Pages) == 0 {
		return "", fmt.Errorf("no pages in response")
	}

	// Concatenate all pages (stitched image might return 1 or 2 pages)
	var parts []string
	for _, p := range ocrResp.Pages {
		parts = append(parts, p.Markdown)
	}
	return strings.Join(parts, "\n\n"), nil
}

// ── Markdown → regions converter ─────────────────────────────────────────────

var (
	reMDTable      = regexp.MustCompile(`(?m)^\|.+\|[ \t]*$`)
	reHeading      = regexp.MustCompile(`^(#{1,4})\s+(.+)`)
	reImgTag       = regexp.MustCompile(`!\[.*?\]\(.*?\)`)
	// reRomanPrefix matches headings that start with a Roman numeral section number,
	// e.g. "I.", "II.", "III.", "IV.", "V.", "VI.", "VII.", "VIII.", "IX.", "X."
	// These are section headings (→ left), not the main document title (→ center).
	reRomanPrefix  = regexp.MustCompile(`(?i)^(X{0,3})(IX|IV|V?I{0,3})\.`)
)

// isMeaninglessBlock returns true for very short blocks that are OCR noise:
//  - 1-char blocks that are a single uppercase Latin letter (e.g. "U") or CJK char
//  - 2-4 char blocks containing ONLY non-Latin / non-Vietnamese characters
func isMeaninglessBlock(block string) bool {
	runes := []rune(strings.TrimSpace(block))
	if len(runes) == 0 || len(runes) > 4 {
		return false
	}
	// Single uppercase ASCII letter (A-Z) → OCR artifact from logo/stamp
	if len(runes) == 1 && runes[0] >= 'A' && runes[0] <= 'Z' {
		return true
	}
	for _, r := range runes {
		// Allow ASCII printable, Latin extended, Vietnamese diacritics
		if r < 0x80 || (r >= 0x00C0 && r <= 0x024F) || (r >= 0x1E00 && r <= 0x1EFF) {
			return false // has Latin / Vietnamese → not garbage
		}
	}
	return true // all chars are non-Latin (CJK, symbols, etc.) → garbage
}

// isImpliedHeadingBlock returns true when a text block (no markdown markers)
// starts with an ALL-CAPS line — Mistral sometimes omits # prefixes for headings
// (e.g. page 12 of CIPUTRA where every heading is bare ALL-CAPS text).
func isImpliedHeadingBlock(block string) bool {
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

// isBoldHeading returns true when an entire block is wrapped in ** markers
// (e.g. "**Nơi nhận:**") — Mistral sometimes uses bold instead of ## for headings.
func isBoldHeading(block string) bool {
	b := strings.TrimSpace(block)
	if !strings.HasPrefix(b, "**") || !strings.HasSuffix(b, "**") || len(b) < 5 {
		return false
	}
	inner := b[2 : len(b)-2]
	// Must not contain another ** pair inside (would be a bold phrase, not a heading)
	return !strings.Contains(inner, "**") && !strings.Contains(inner, "\n")
}

// headingAlignment returns "center" or "left" for a heading's content.
// Rules:
//   - Roman numeral prefix (I., II., III. …) → always left (section heading)
//   - ALL-CAPS content at any level → center (document titles, state headers)
//   - "Độc lập" / "Hạnh phúc" → center (Vietnamese state header subtitle)
//   - Everything else → left
func headingAlignment(content string) string {
	c := strings.TrimSpace(content)
	// Roman numeral section heading → always left
	if reRomanPrefix.MatchString(c) {
		return "left"
	}
	// "BÊN ..." party labels in contracts → always left (section labels, not titles)
	if strings.HasPrefix(strings.ToUpper(c), "BÊN ") {
		return "left"
	}
	// Vietnamese state header subtitle → always center
	if strings.Contains(content, "Độc lập") || strings.Contains(content, "Hạnh phúc") {
		return "center"
	}
	// ALL-CAPS content → center (document/section titles)
	if c == strings.ToUpper(c) && len([]rune(c)) > 3 {
		return "center"
	}
	return "left"
}

// markdownToRegions converts Mistral markdown output to []region for HTML rendering.
func markdownToRegions(md string) []region {
	var regions []region
	seenTable := false // once a table is encountered, subsequent ALL-CAPS blocks are labels/names not headings

	// Split into blocks: blank-line separated
	blocks := splitBlocks(md)
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		// Skip scanner watermarks (CamScanner, etc.)
		lowerBlock := strings.ToLower(block)
		if strings.Contains(lowerBlock, "scanned with") || strings.Contains(lowerBlock, "camscanner") {
			continue
		}

		// Skip garbage/noise blocks: very short content (≤3 runes) that contains
		// CJK or other non-Latin characters — typically OCR artifacts from stamps/logos.
		if isMeaninglessBlock(block) {
			continue
		}

		// Image reference → figure
		if reImgTag.MatchString(block) {
			regions = append(regions, region{Type: "figure", FigureType: "decorative"})
			continue
		}

		// Markdown table block
		if looksLikeTable(block) {
			seenTable = true
			html := mdTableToHTML(block)
			regions = append(regions, region{Type: "table", HTML: html})
			continue
		}

		// Bold-only paragraph used as heading by Mistral (e.g. "**Nơi nhận:**")
		// Treat as a title region so it gets heading styling instead of plain <p>.
		if isBoldHeading(block) {
			content := strings.TrimSpace(block[2 : len(block)-2])
			regions = append(regions, region{Type: "title", Content: content, Alignment: headingAlignment(content)})
			continue
		}

		// Implied heading block: Mistral outputs ALL-CAPS text without # markers.
		// Treat ALL-CAPS lines as titles, non-ALL-CAPS follow-up lines as subtitles
		// (inheriting parent alignment when parent was centered) or plain text.
		//
		// Exception: once a table has been seen on this page, all subsequent ALL-CAPS
		// blocks are signature labels / names (e.g. "CHI NHƯƠNG", "LÊ THỊ THU HƯỞNG"),
		// not section headings — skip implied heading detection for them.
		if !seenTable && isImpliedHeadingBlock(block) {
			currentAlign := "left"
			for _, line := range strings.Split(block, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if line == strings.ToUpper(line) && len([]rune(line)) > 3 {
					currentAlign = headingAlignment(line)
					regions = append(regions, region{Type: "title", Content: line, Alignment: currentAlign})
				} else if currentAlign == "center" {
					regions = append(regions, region{Type: "title", Content: line, Alignment: "center"})
				} else {
					regions = append(regions, region{Type: "text", Content: line, Alignment: "left"})
				}
			}
			continue
		}

		// Heading
		if m := reHeading.FindStringSubmatch(block); m != nil {
			content := strings.TrimSpace(m[2])
			regions = append(regions, region{Type: "title", Content: content, Alignment: headingAlignment(content)})

			// Process continuation lines in the same block (Mistral sometimes puts a
			// subtitle on the very next line with no blank line, e.g.:
			//   # THÔNG BÁO NỘP TIỀN
			//   Về thuế thu nhập cá nhân...   ← lost without this
			//
			//   # CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM
			//   Độc lập - Tự do - Hạnh phúc  ← lost without this)
			lines := strings.Split(block, "\n")
			parentAlign := headingAlignment(content)
			for _, line := range lines[1:] {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if mm := reHeading.FindStringSubmatch(line); mm != nil {
					// Another heading on the next line (e.g. ## HỢP ĐỒNG MUA BÁN NHÀ Ở)
					c2 := strings.TrimSpace(mm[2])
					regions = append(regions, region{Type: "title", Content: c2, Alignment: headingAlignment(c2)})
				} else {
					// Subtitle / continuation text — inherit parent heading alignment
					regions = append(regions, region{Type: "title", Content: line, Alignment: parentAlign})
				}
			}
			continue
		}

		// Plain text / paragraph
		regions = append(regions, region{Type: "text", Content: block, Alignment: "left"})
	}
	return regions
}

// splitBlocks splits markdown into logical blocks.
// Blank lines are the primary delimiter.
// Each line starting with '#' is also forced into its own block, so that
// consecutive heading lines (e.g. "# Title\n## Subtitle" with no blank line)
// are processed independently rather than the second heading being lost.
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
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flush()
			continue
		}
		// Each heading line starts its own block
		if strings.HasPrefix(trimmed, "#") {
			flush()
		}
		cur = append(cur, line)
	}
	flush()
	return blocks
}

// joinTableContinuations handles Mistral's multi-line cell content.
// Mistral sometimes emits table rows with line breaks inside cells:
//
//	|  STT | Số tờ khai/
//	Số quyết định/
//	Mã định danh | ... |
//
// This joins each non-| continuation line onto the preceding | line with <br>.
func joinTableContinuations(block string) string {
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

// looksLikeTable returns true if the block contains markdown table rows.
// Joins continuations first so multi-line cells are counted correctly.
func looksLikeTable(block string) bool {
	joined := joinTableContinuations(block)
	lines := strings.Split(joined, "\n")
	tableLines := 0
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "|") {
			tableLines++
		}
	}
	return tableLines >= 2
}

// rightColumnPrefixes are line prefixes that belong in the RIGHT column of
// Vietnamese government form bordered boxes (KBNN/bank sections).
// Mistral collapses all content into cell 0; we redistribute these lines back.
var rightColumnPrefixes = []string{
	"Nợ TK:", "Có TK:", "No TK:", "Co TK:",
	"Phí:", "Phi:", "VAT:", "Vat:",
}

func isRightColumnLine(line string) bool {
	t := strings.TrimSpace(line)
	for _, p := range rightColumnPrefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

// redistributeColumns restores the 2-column layout for bordered boxes where
// Mistral collapsed everything into column 0. It detects when:
//  1. All non-first cells are empty across all rows, AND
//  2. Some lines in cell 0 match rightColumnPrefixes
//
// When detected, those lines are moved to the last cell of each row.
// Returns the (possibly modified) rows and whether redistribution was applied.
// When redistribution is applied the table is a section box (not a data table)
// and should be rendered entirely with <td> rather than <th>.
func redistributeColumns(rows [][]string) ([][]string, bool) {
	if len(rows) == 0 || len(rows[0]) < 2 {
		return rows, false
	}
	// Check all non-first cells are empty
	for _, row := range rows {
		for _, cell := range row[1:] {
			if strings.TrimSpace(cell) != "" {
				return rows, false // already has right-column content
			}
		}
	}
	// Check at least one row has right-column lines in cell 0
	hasRight := false
	for _, row := range rows {
		for _, line := range strings.Split(row[0], "<br>") {
			if isRightColumnLine(line) {
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
			if isRightColumnLine(line) {
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

// detectSectionBox returns true when any cell in the table contains
// right-column keywords (Nợ TK, Có TK, Phí, VAT, etc.).
// Such tables are government form section boxes (KBNN / bank sections),
// not data tables — all rows should render as <td>, not <th>.
func detectSectionBox(rows [][]string) bool {
	for _, row := range rows {
		for _, cell := range row {
			for _, line := range strings.Split(cell, "<br>") {
				if isRightColumnLine(line) {
					return true
				}
			}
		}
	}
	return false
}

// removeEmptyColumns strips columns where every cell across all rows is empty.
// This fixes tables where Mistral leaves a middle column completely blank.
// isHeader is kept in sync with rows (same slice length).
func removeEmptyColumns(rows [][]string, isHeader []bool) [][]string {
	if len(rows) == 0 {
		return rows
	}
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	if maxCols == 0 {
		return rows
	}
	// Determine which column indices have at least one non-empty cell
	keep := make([]bool, maxCols)
	for _, row := range rows {
		for ci, cell := range row {
			if strings.TrimSpace(cell) != "" {
				keep[ci] = true
			}
		}
	}
	// Count kept columns; if all are kept, return unchanged
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

// mdTableToHTML converts a markdown table to an HTML table.
func mdTableToHTML(block string) string {
	block = joinTableContinuations(block)
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

	// Remove entirely empty columns (e.g. middle column that Mistral left blank).
	rows = removeEmptyColumns(rows, isHeader)

	// Section box detection (KBNN / bank sections):
	// 1. Check all cells for right-column keywords (handles case where Mistral
	//    already put them in the correct column, e.g. CIPUTRA page 15).
	// 2. Also try redistributeColumns for the collapsed-to-col-0 case (BDS Kim Chung).
	// Either way → render all rows as <td> (no bold header styling).
	sectionBox := detectSectionBox(rows)
	var wasRedistributed bool
	rows, wasRedistributed = redistributeColumns(rows)
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
		if isHeader[i] {
			b.WriteString("<thead><tr>")
			writeRowCells(row, "th", true, &b)
			b.WriteString("</tr></thead><tbody>")
		} else {
			b.WriteString("<tr>")
			writeRowCells(row, "td", false, &b)
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

// writeRowCells writes table cells for one row.
// colspan is only applied for header rows (isHeader=true): a non-empty header cell
// followed by empty cells is treated as a merged group header.
// For data rows, all cells are rendered individually — empty cells are genuinely
// empty data cells, not merged cells, and applying colspan there causes incorrect rendering.
func writeRowCells(cells []string, tag string, isHeader bool, b *strings.Builder) {
	i := 0
	for i < len(cells) {
		content := cells[i]
		span := 1
		if isHeader && strings.TrimSpace(content) != "" {
			// Only merge empty cells into colspan for header rows
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
