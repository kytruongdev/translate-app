package file

import (
	"strings"
	"testing"
)

// ── escapeHTML ────────────────────────────────────────────────────────────────

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"ampersand", "a & b", "a &amp; b"},
		{"less than", "a < b", "a &lt; b"},
		{"greater than", "a > b", "a &gt; b"},
		{"double quote", `say "hello"`, "say &quot;hello&quot;"},
		{"single quote", "it's", "it&#39;s"},
		{"all five", `<a href="x">'&'</a>`, "&lt;a href=&quot;x&quot;&gt;&#39;&amp;&#39;&lt;/a&gt;"},
		{"empty", "", ""},
		{"no special chars", "hello world", "hello world"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := escapeHTML(tc.input)
			if got != tc.want {
				t.Errorf("escapeHTML(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ── isMergableTitle ───────────────────────────────────────────────────────────

func TestIsMergableTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"short plain title", "CONTRACT FOR SALE", true},
		{"ends with colon", "CERTIFICATE:", false},
		{"ends with colon lowercase", "Based on:", false},
		{"roman I", "I. Introduction", false},
		{"roman II", "II. Terms", false},
		{"roman III", "III. Conditions", false},
		{"roman IV", "IV. Payment", false},
		{"roman alone", "I.", false},
		{"exceeds 80 runes", strings.Repeat("A", 81), false},
		{"exactly 80 runes", strings.Repeat("A", 80), true},
		{"short word", "Title", true},
		{"whitespace only treated as short", "   ", true}, // TrimSpace → empty → rune count 0 ≤ 80
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isMergableTitle(tc.input)
			if got != tc.want {
				t.Errorf("isMergableTitle(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ── convertMarkdownInline ─────────────────────────────────────────────────────

func TestConvertMarkdownInline(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"bold", "**hello**", "<strong>hello</strong>"},
		{"italic", "*hello*", "<em>hello</em>"},
		{"bold before italic", "**bold** and *italic*", "<strong>bold</strong> and <em>italic</em>"},
		{"no markdown", "plain text", "plain text"},
		{"already has strong — returned as-is", "<strong>x</strong>", "<strong>x</strong>"},
		{"already has em — returned as-is", "<em>x</em>", "<em>x</em>"},
		{"html escape plain text", "a < b", "a &lt; b"},
		{"bold with special chars escaped", "**a & b**", "<strong>a &amp; b</strong>"},
		{"empty string", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := convertMarkdownInline(tc.input)
			if got != tc.want {
				t.Errorf("convertMarkdownInline(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ── renderTextBlocks ──────────────────────────────────────────────────────────

func TestRenderTextBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantSubs []string // substrings that must be present
		wantNot  []string // substrings that must NOT be present
	}{
		{
			name:     "single paragraph",
			input:    "Hello world",
			wantSubs: []string{"<p>", "Hello world", "</p>"},
		},
		{
			name:     "double newline splits paragraphs",
			input:    "Para one\n\nPara two",
			wantSubs: []string{"<p>Para one</p>", "<p>Para two</p>"},
		},
		{
			name:     "single newline becomes br",
			input:    "Line one\nLine two",
			wantSubs: []string{"<p>", "Line one<br>", "Line two", "</p>"},
		},
		{
			name:     "br tags normalized to newline",
			input:    "a<br>b<br/>c<br />d",
			wantSubs: []string{"a<br>", "b<br>", "c<br>", "d"},
		},
		{
			name:     "empty input",
			input:    "",
			wantSubs: []string{},
			wantNot:  []string{"<p>"},
		},
		{
			name:     "only whitespace",
			input:    "   \n\n   ",
			wantNot:  []string{"<p>"},
		},
		{
			name:     "markdown bold preserved",
			input:    "**important** note",
			wantSubs: []string{"<strong>important</strong>"},
		},
		{
			name:     "trailing spaces before newline trimmed",
			input:    "Line one   \nLine two",
			wantNot:  []string{"   \n"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := renderTextBlocks(tc.input)
			for _, sub := range tc.wantSubs {
				if !strings.Contains(got, sub) {
					t.Errorf("renderTextBlocks(%q): missing %q\ngot: %q", tc.input, sub, got)
				}
			}
			for _, sub := range tc.wantNot {
				if strings.Contains(got, sub) {
					t.Errorf("renderTextBlocks(%q): unexpected %q\ngot: %q", tc.input, sub, got)
				}
			}
		})
	}
}

// ── assembleStructuredHTML ────────────────────────────────────────────────────

func TestAssembleStructuredHTML_BasicRegions(t *testing.T) {
	result := &StructuredOCRResult{
		Pages: []OCRPage{
			{
				PageNo: 1,
				Regions: []OCRRegion{
					{Type: "title", Content: "MY TITLE", Alignment: "center"},
					{Type: "text", Content: "Some body text."},
					{Type: "table", HTML: "<table><tr><td>A</td></tr></table>"},
				},
			},
		},
	}
	translated := map[string]string{
		"1_0": "MY TITLE",
		"1_1": "Some body text.",
		"1_2": "<table><tr><td>A</td></tr></table>",
	}

	html, err := assembleStructuredHTML(result, translated, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(html, "page-1") {
		t.Error("missing page div")
	}
	if !strings.Contains(html, "<h1") {
		t.Error("ALL-CAPS centered title should be h1")
	}
	if !strings.Contains(html, "Some body text") {
		t.Error("missing body text")
	}
	if !strings.Contains(html, "<table>") {
		t.Error("missing table HTML")
	}
}

func TestAssembleStructuredHTML_EmptyTranslation(t *testing.T) {
	result := &StructuredOCRResult{
		Pages: []OCRPage{
			{
				PageNo: 1,
				Regions: []OCRRegion{
					{Type: "text", Content: "hello"},
				},
			},
		},
	}
	// translated map empty → segment skipped
	html, err := assembleStructuredHTML(result, map[string]string{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(html, "hello") {
		t.Error("empty translation should not appear in output")
	}
}

func TestAssembleStructuredHTML_TitleMerging(t *testing.T) {
	result := &StructuredOCRResult{
		Pages: []OCRPage{
			{
				PageNo: 1,
				Regions: []OCRRegion{
					{Type: "title", Content: "PART ONE", Alignment: "center"},
					{Type: "title", Content: "SUBTITLE", Alignment: "center"},
					{Type: "title", Content: "I. Section Header", Alignment: "left"}, // not mergable
				},
			},
		},
	}
	translated := map[string]string{
		"1_0": "PART ONE",
		"1_1": "SUBTITLE",
		"1_2": "I. Section Header",
	}
	html, err := assembleStructuredHTML(result, translated, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// SUBTITLE should be merged into PART ONE heading
	if !strings.Contains(html, "PART ONE") {
		t.Error("missing PART ONE")
	}
	if !strings.Contains(html, "SUBTITLE") {
		t.Error("missing SUBTITLE")
	}
	// I. Section Header is not mergable, should appear separately
	if !strings.Contains(html, "I. Section Header") {
		t.Error("missing section header")
	}
}

func TestAssembleStructuredHTML_MultiplePages(t *testing.T) {
	result := &StructuredOCRResult{
		Pages: []OCRPage{
			{PageNo: 1, Regions: []OCRRegion{{Type: "text", Content: "Page one"}}},
			{PageNo: 2, Regions: []OCRRegion{{Type: "text", Content: "Page two"}}},
		},
	}
	translated := map[string]string{"1_0": "Page one", "2_0": "Page two"}
	html, err := assembleStructuredHTML(result, translated, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(html, "page-1") || !strings.Contains(html, "page-2") {
		t.Error("missing page divs")
	}
}
