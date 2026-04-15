package file

import (
	"strings"
	"testing"
)

// ── regionKey ─────────────────────────────────────────────────────────────────

func TestRegionKey(t *testing.T) {
	tests := []struct {
		pageNo    int
		regionIdx int
		want      string
	}{
		{1, 0, "1_0"},
		{3, 7, "3_7"},
		{0, 0, "0_0"},
		{10, 100, "10_100"},
	}
	for _, tc := range tests {
		got := regionKey(tc.pageNo, tc.regionIdx)
		if got != tc.want {
			t.Errorf("regionKey(%d, %d) = %q, want %q", tc.pageNo, tc.regionIdx, got, tc.want)
		}
	}
}

// ── isAIRefusal ───────────────────────────────────────────────────────────────

func TestIsAIRefusal(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"refusal i'm sorry but", "I'm sorry, but I cannot translate this.", true},
		{"refusal i'm unable to", "I'm unable to assist with that request.", true},
		{"refusal i cannot assist", "I cannot assist with this.", true},
		{"refusal as an ai", "As an AI, I cannot...", true},
		{"refusal sorry but i cannot", "Sorry, but I cannot help.", true},
		{"refusal i cannot provide", "I cannot provide a translation.", true},
		{"refusal i'm not able to", "I'm not able to do this.", true},
		{"refusal case insensitive", "I'M SORRY, BUT I CANNOT do that.", true},
		{"refusal leading whitespace", "   I'm unable to help.", true},
		{"normal translation", "The contract is signed.", false},
		{"empty", "", false},
		{"partial match mid sentence", "He said I cannot assist him.", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isAIRefusal(tc.input)
			if got != tc.want {
				t.Errorf("isAIRefusal(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ── parseContextResult ────────────────────────────────────────────────────────

func TestParseContextResult(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantDocType string
		wantSummary string
	}{
		{
			name:        "valid JSON",
			input:       `{"doc_type":"contract","is_new_doc_type":false,"summary":"Property transfer contract"}`,
			wantDocType: "contract",
			wantSummary: "Property transfer contract",
		},
		{
			name:        "JSON with preamble text",
			input:       "Here is the result:\n{\"doc_type\":\"notary\",\"is_new_doc_type\":false,\"summary\":\"Notarized document\"}",
			wantDocType: "notary",
			wantSummary: "Notarized document",
		},
		{
			name:        "JSON with trailing text",
			input:       "{\"doc_type\":\"tax\",\"is_new_doc_type\":false,\"summary\":\"Tax notice\"} end",
			wantDocType: "tax",
			wantSummary: "Tax notice",
		},
		{
			name:        "invalid JSON returns empty",
			input:       "not json at all",
			wantDocType: "",
			wantSummary: "",
		},
		{
			name:        "empty string returns empty",
			input:       "",
			wantDocType: "",
			wantSummary: "",
		},
		{
			name:        "missing fields returns empty strings",
			input:       `{}`,
			wantDocType: "",
			wantSummary: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotDocType, gotSummary := parseContextResult(tc.input)
			if gotDocType != tc.wantDocType {
				t.Errorf("parseContextResult docType = %q, want %q", gotDocType, tc.wantDocType)
			}
			if gotSummary != tc.wantSummary {
				t.Errorf("parseContextResult summary = %q, want %q", gotSummary, tc.wantSummary)
			}
		})
	}
}

// ── extractFirstPagesMarkdown ─────────────────────────────────────────────────

func makeOCRResult(pages []OCRPage) *StructuredOCRResult {
	return &StructuredOCRResult{Pages: pages}
}

func TestExtractFirstPagesMarkdown(t *testing.T) {
	result := makeOCRResult([]OCRPage{
		{PageNo: 1, Regions: []OCRRegion{
			{Type: "title", Content: "DOCUMENT TITLE"},
			{Type: "text", Content: "Intro text."},
			{Type: "table", HTML: "<table><tr><td>ignored</td></tr></table>"},
		}},
		{PageNo: 2, Regions: []OCRRegion{
			{Type: "text", Content: "Page two text."},
		}},
		{PageNo: 3, Regions: []OCRRegion{
			{Type: "text", Content: "Page three text."},
		}},
		{PageNo: 4, Regions: []OCRRegion{
			{Type: "text", Content: "Page four — should be excluded."},
		}},
	})

	got := extractFirstPagesMarkdown(result, 3)

	if !strings.Contains(got, "## DOCUMENT TITLE") {
		t.Error("missing title with ## prefix")
	}
	if !strings.Contains(got, "Intro text.") {
		t.Error("missing text from page 1")
	}
	if strings.Contains(got, "ignored") {
		t.Error("table HTML should not be included")
	}
	if !strings.Contains(got, "Page two text.") {
		t.Error("missing text from page 2")
	}
	if !strings.Contains(got, "Page three text.") {
		t.Error("missing text from page 3")
	}
	if strings.Contains(got, "Page four") {
		t.Error("page 4 should be excluded (n=3)")
	}
}

func TestExtractFirstPagesMarkdown_NGreaterThanPages(t *testing.T) {
	result := makeOCRResult([]OCRPage{
		{PageNo: 1, Regions: []OCRRegion{{Type: "text", Content: "Only page."}}},
	})
	got := extractFirstPagesMarkdown(result, 10)
	if !strings.Contains(got, "Only page.") {
		t.Error("should include the single page even when n > total pages")
	}
}

func TestExtractFirstPagesMarkdown_EmptyRegions(t *testing.T) {
	result := makeOCRResult([]OCRPage{
		{PageNo: 1, Regions: []OCRRegion{
			{Type: "text", Content: "   "},
			{Type: "figure", FigureType: "decorative"},
		}},
	})
	got := extractFirstPagesMarkdown(result, 3)
	if strings.TrimSpace(got) != "" {
		t.Errorf("expected empty result, got %q", got)
	}
}

// ── collectSegments ───────────────────────────────────────────────────────────

func TestCollectSegments(t *testing.T) {
	result := makeOCRResult([]OCRPage{
		{PageNo: 1, Regions: []OCRRegion{
			{Type: "title", Content: "Title here"},
			{Type: "text", Content: "Body text"},
			{Type: "text", Content: "   "}, // empty — skipped
			{Type: "table", HTML: "<table></table>"},
			{Type: "figure", FigureType: "informational", TextLines: []string{"fig text"}},
			{Type: "figure", FigureType: "decorative"}, // skipped
		}},
	})

	segs := collectSegments(result)

	if len(segs) != 4 {
		t.Fatalf("expected 4 segments, got %d", len(segs))
	}
	// title → text segment, not HTML
	if segs[0].isHTML {
		t.Error("title segment should not be HTML")
	}
	// table → isHTML
	if !segs[2].isHTML {
		t.Error("table segment should be isHTML")
	}
	// figure informational → text joined with |
	if !strings.Contains(segs[3].text, "fig text") {
		t.Errorf("figure segment text = %q, want to contain 'fig text'", segs[3].text)
	}
}

func TestCollectSegments_HTMLTagsInTextMarkAsHTML(t *testing.T) {
	result := makeOCRResult([]OCRPage{
		{PageNo: 1, Regions: []OCRRegion{
			{Type: "text", Content: "Hello <strong>world</strong>"},
			{Type: "text", Content: "Plain text"},
		}},
	})
	segs := collectSegments(result)
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segs))
	}
	if !segs[0].isHTML {
		t.Error("segment with <strong> should be isHTML")
	}
	if segs[1].isHTML {
		t.Error("plain segment should not be isHTML")
	}
}

// ── buildPDFBatchInput ────────────────────────────────────────────────────────

func TestBuildPDFBatchInput(t *testing.T) {
	segs := []pdfSegment{
		{key: "1_0", text: "First segment"},
		{key: "1_1", text: "Second segment"},
		{key: "1_2", text: "Third segment"},
	}
	got := buildPDFBatchInput(segs)

	if !strings.Contains(got, "<<<1>>>") {
		t.Error("missing <<<1>>> marker")
	}
	if !strings.Contains(got, "<<<2>>>") {
		t.Error("missing <<<2>>> marker")
	}
	if !strings.Contains(got, "<<<3>>>") {
		t.Error("missing <<<3>>> marker")
	}
	if !strings.Contains(got, "First segment") {
		t.Error("missing first segment text")
	}
	if !strings.Contains(got, "Third segment") {
		t.Error("missing third segment text")
	}
}

func TestBuildPDFBatchInput_SingleSegment(t *testing.T) {
	segs := []pdfSegment{{key: "1_0", text: "Only"}}
	got := buildPDFBatchInput(segs)
	if !strings.Contains(got, "<<<1>>>") {
		t.Error("missing <<<1>>> marker")
	}
	if strings.Contains(got, "<<<2>>>") {
		t.Error("unexpected <<<2>>> marker for single segment")
	}
}

// ── batchPDFSegments ──────────────────────────────────────────────────────────

func TestBatchPDFSegments_HTMLIsolated(t *testing.T) {
	segs := []pdfSegment{
		{key: "1_0", text: "text one", isHTML: false},
		{key: "1_1", text: "<table></table>", isHTML: true},
		{key: "1_2", text: "text two", isHTML: false},
	}
	batches := batchPDFSegments(segs)
	// HTML segment must be in its own batch
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches (text | html | text), got %d", len(batches))
	}
	if !batches[1][0].isHTML {
		t.Error("middle batch should be the HTML segment")
	}
}

func TestBatchPDFSegments_RuneLimit(t *testing.T) {
	// Create segments that together exceed pdfStructuredChunkSize (5000)
	longText := strings.Repeat("あ", 3000) // 3000 runes each
	segs := []pdfSegment{
		{key: "1_0", text: longText},
		{key: "1_1", text: longText},
	}
	batches := batchPDFSegments(segs)
	if len(batches) != 2 {
		t.Errorf("expected 2 batches due to rune limit, got %d", len(batches))
	}
}

func TestBatchPDFSegments_SegmentCountLimit(t *testing.T) {
	// 31 short segments should be split at pdfMaxSegmentsPerBatch (30)
	segs := make([]pdfSegment, 31)
	for i := range segs {
		segs[i] = pdfSegment{key: regionKey(1, i), text: "short"}
	}
	batches := batchPDFSegments(segs)
	if len(batches) != 2 {
		t.Errorf("expected 2 batches at segment count limit=30, got %d", len(batches))
	}
	if len(batches[0]) != 30 {
		t.Errorf("first batch should have 30 segments, got %d", len(batches[0]))
	}
}

func TestBatchPDFSegments_Empty(t *testing.T) {
	batches := batchPDFSegments(nil)
	if len(batches) != 0 {
		t.Errorf("expected 0 batches for nil input, got %d", len(batches))
	}
}

// ── extractSourceTextFromOCR ──────────────────────────────────────────────────

func TestExtractSourceTextFromOCR(t *testing.T) {
	result := makeOCRResult([]OCRPage{
		{PageNo: 1, Regions: []OCRRegion{
			{Type: "title", Content: "My Title"},
			{Type: "text", Content: "Body text here."},
			{Type: "table", HTML: "<table></table>"},
			{Type: "figure", FigureType: "informational", TextLines: []string{"chart data"}},
			{Type: "figure", FigureType: "decorative"},
		}},
	})

	got := extractSourceTextFromOCR(result)

	if !strings.Contains(got, "## My Title") {
		t.Error("missing title with ## prefix")
	}
	if !strings.Contains(got, "Body text here.") {
		t.Error("missing body text")
	}
	if !strings.Contains(got, "[Bảng]") {
		t.Error("table should be summarized as [Bảng]")
	}
	if !strings.Contains(got, "[Hình:") {
		t.Error("informational figure should produce [Hình:] placeholder")
	}
	if !strings.Contains(got, "chart data") {
		t.Error("figure text lines should appear in placeholder")
	}
	// decorative figure should produce nothing
	count := strings.Count(got, "[Hình:")
	if count != 1 {
		t.Errorf("expected 1 figure placeholder, got %d", count)
	}
}

func TestExtractSourceTextFromOCR_EmptyContent(t *testing.T) {
	result := makeOCRResult([]OCRPage{
		{PageNo: 1, Regions: []OCRRegion{
			{Type: "text", Content: "   "},
		}},
	})
	got := extractSourceTextFromOCR(result)
	if strings.TrimSpace(got) != "" {
		t.Errorf("expected empty result for whitespace-only content, got %q", got)
	}
}
