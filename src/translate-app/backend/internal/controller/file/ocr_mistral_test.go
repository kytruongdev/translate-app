package file

import (
	"strings"
	"testing"
)

// ── truncateMistral ───────────────────────────────────────────────────────────

func TestTruncateMistral(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{"no truncation needed", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"truncate ASCII", "hello world", 5, "hello…"},
		{"truncate Vietnamese runes", "Xin chào thế giới", 8, "Xin chào…"},
		{"empty string", "", 5, ""},
		{"n=0 truncates all", "hi", 0, "…"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateMistral(tc.input, tc.n)
			if got != tc.want {
				t.Errorf("truncateMistral(%q, %d) = %q, want %q", tc.input, tc.n, got, tc.want)
			}
		})
	}
}

// ── mistralIsRetryable ────────────────────────────────────────────────────────

func TestMistralIsRetryable(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		want  bool
	}{
		{"nil error", nil, false},
		{"5xx HTTP error", &mistralHTTPError{StatusCode: 500, Body: "internal error"}, true},
		{"503 HTTP error", &mistralHTTPError{StatusCode: 503, Body: "service unavailable"}, true},
		{"4xx is not retryable", &mistralHTTPError{StatusCode: 400, Body: "bad request"}, false},
		{"429 is not retryable", &mistralHTTPError{StatusCode: 429, Body: "rate limit"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mistralIsRetryable(tc.err)
			if got != tc.want {
				t.Errorf("mistralIsRetryable(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// ── mistralSplitBlocks ────────────────────────────────────────────────────────

func TestMistralSplitBlocks(t *testing.T) {
	md := "# Heading\n\nFirst paragraph.\n\nSecond paragraph."
	blocks := mistralSplitBlocks(md)
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d: %v", len(blocks), blocks)
	}
	if !strings.Contains(blocks[0], "Heading") {
		t.Error("first block should contain heading")
	}

	// # flushes the preceding block — "intro" becomes its own block;
	// "# Heading\nafter" becomes the next block (continuation lines stay together).
	md2 := "intro\n# Heading\nafter"
	blocks2 := mistralSplitBlocks(md2)
	if len(blocks2) != 2 {
		t.Fatalf("# should flush preceding block, got %d blocks: %v", len(blocks2), blocks2)
	}
	if !strings.Contains(blocks2[0], "intro") {
		t.Error("first block should be 'intro'")
	}
	if !strings.Contains(blocks2[1], "# Heading") {
		t.Error("second block should contain # Heading")
	}

	// Empty lines are skipped
	md3 := "\n\n\nhello\n\n\nworld\n\n"
	blocks3 := mistralSplitBlocks(md3)
	if len(blocks3) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(blocks3))
	}
}

// ── mistralIsMeaninglessBlock ─────────────────────────────────────────────────

func TestMistralIsMeaninglessBlock(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"single uppercase letter", "A", true},
		{"single uppercase letter Z", "Z", true},
		{"single lowercase", "a", false}, // not uppercase
		{"CJK garbage 4 runes", "中文垃圾", true},
		{"CJK 5 runes — too long", "中文垃圾长", false},
		{"Vietnamese word", "Tên", false}, // has latin extension, not garbage
		{"empty", "", false},
		{"normal word", "hello", false},
		{"two uppercase letters", "AB", false}, // len > 1, not single uppercase
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mistralIsMeaninglessBlock(tc.input)
			if got != tc.want {
				t.Errorf("mistralIsMeaninglessBlock(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ── mistralIsStampNoise ───────────────────────────────────────────────────────

func TestMistralIsStampNoise(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"stamp with TM/UBND", "TM/UBND PHƯỜNG", true},
		{"stamp with TM/", "TM/ CHỦ TỊCH", true},
		{"stamp with UBHC", "UBHC XÃ", true},
		{"long block not stamp", strings.Repeat("A", 61), false},
		{"has too many lowercase", "TM/UBND some lowercase text here", false},
		{"valid heading CỘNG HÒA", "CỘNG HÒA XÃ HỘI", false}, // no stamp keyword
		{"empty", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mistralIsStampNoise(tc.input)
			if got != tc.want {
				t.Errorf("mistralIsStampNoise(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ── mistralIsImpliedHeadingBlock ──────────────────────────────────────────────

func TestMistralIsImpliedHeadingBlock(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"ALL CAPS heading", "CỘNG HÒA XÃ HỘI CHỦ NGHĨA", true},
		{"starts with #", "# heading", false},
		{"starts with |", "| table |", false},
		{"starts with *", "* list", false},
		{"mixed case", "Cộng hòa", false},
		{"too short (< 4 runes)", "AB", false},
		{"too long (> 100 runes)", strings.ToUpper(strings.Repeat("A", 101)), false},
		{"exactly 4 runes ALL CAPS", "ABCD", true},
		{"exactly 100 runes ALL CAPS", strings.ToUpper(strings.Repeat("A", 100)), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mistralIsImpliedHeadingBlock(tc.input)
			if got != tc.want {
				t.Errorf("mistralIsImpliedHeadingBlock(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ── mistralIsBoldHeading ──────────────────────────────────────────────────────

func TestMistralIsBoldHeading(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid bold heading", "**Nơi nhận:**", true},
		{"no bold markers", "plain text", false},
		{"too short (< 5)", "****", false},
		{"nested bold", "**a **b** c**", false},
		{"multiline bold", "**line1\nline2**", false},
		{"only open marker", "**hello", false},
		{"only close marker", "hello**", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mistralIsBoldHeading(tc.input)
			if got != tc.want {
				t.Errorf("mistralIsBoldHeading(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ── mistralHeadingAlignment ───────────────────────────────────────────────────

func TestMistralHeadingAlignment(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"roman numeral I", "I. Introduction", "left"},
		{"roman numeral IV", "IV. Payment Terms", "left"},
		{"BÊN prefix", "BÊN A:", "left"},
		{"Độc lập keyword", "Độc lập - Tự do - Hạnh phúc", "center"},
		{"Hạnh phúc keyword", "Hạnh phúc", "center"},
		{"ALL CAPS > 3 runes", "CONTRACT", "center"},
		{"ALL CAPS exactly 3 runes", "ABC", "left"}, // ≤ 3 → left
		{"mixed case", "Some Title", "left"},
		{"empty", "", "left"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mistralHeadingAlignment(tc.input)
			if got != tc.want {
				t.Errorf("mistralHeadingAlignment(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ── mistralIsSeparatorRow ─────────────────────────────────────────────────────

func TestMistralIsSeparatorRow(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"standard separator", "|---|---|---|", true},
		{"with colons", "|:---|:---:|---:|", true},
		{"with spaces", "| --- | --- |", true},
		{"data row", "| Name | Value |", false},
		{"mixed", "| --- | data |", false},
		{"empty cells", "| | |", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mistralIsSeparatorRow(tc.input)
			if got != tc.want {
				t.Errorf("mistralIsSeparatorRow(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ── mistralSplitTableRow ──────────────────────────────────────────────────────

func TestMistralSplitTableRow(t *testing.T) {
	cells := mistralSplitTableRow("| A | B | C |")
	if len(cells) != 3 {
		t.Fatalf("expected 3 cells, got %d: %v", len(cells), cells)
	}
	if cells[0] != "A" || cells[1] != "B" || cells[2] != "C" {
		t.Errorf("unexpected cells: %v", cells)
	}

	// Leading/trailing spaces trimmed
	cells2 := mistralSplitTableRow("|  spaces  |  trimmed  |")
	if cells2[0] != "spaces" || cells2[1] != "trimmed" {
		t.Errorf("spaces not trimmed: %v", cells2)
	}
}

// ── mistralLooksLikeTable ─────────────────────────────────────────────────────

func TestMistralLooksLikeTable(t *testing.T) {
	table := "| A | B |\n|---|---|\n| 1 | 2 |"
	if !mistralLooksLikeTable(table) {
		t.Error("should detect markdown table")
	}

	notTable := "Just some text\nwith multiple lines"
	if mistralLooksLikeTable(notTable) {
		t.Error("should not detect plain text as table")
	}

	// Only one pipe line — not a table
	oneRow := "| A | B |"
	if mistralLooksLikeTable(oneRow) {
		t.Error("single row should not be detected as table")
	}
}

// ── mistralIsLabelValueTable ──────────────────────────────────────────────────

func TestMistralIsLabelValueTable(t *testing.T) {
	// 2-column label:value form
	labelValue := "| Họ và tên | : Nguyễn Văn A |\n|---|---|\n| Ngày sinh | : 01/01/1990 |"
	if !mistralIsLabelValueTable(labelValue) {
		t.Error("should detect label:value table")
	}

	// Regular data table
	dataTable := "| Name | Age |\n|---|---|\n| Alice | 30 |\n| Bob | 25 |"
	if mistralIsLabelValueTable(dataTable) {
		t.Error("should not detect regular data table as label:value")
	}

	// 3 columns — not label:value
	threeCol := "| A | : B | C |\n|---|---|---|\n| D | : E | F |"
	if mistralIsLabelValueTable(threeCol) {
		t.Error("3-column table should not be label:value")
	}
}

// ── mistralLabelValueTableToText ──────────────────────────────────────────────

func TestMistralLabelValueTableToText(t *testing.T) {
	table := "| Họ và tên | : Nguyễn Văn A |\n|---|---|\n| Ngày sinh | : 01/01/1990 |"
	got := mistralLabelValueTableToText(table)

	if !strings.Contains(got, "Họ và tên") {
		t.Error("missing label")
	}
	if !strings.Contains(got, ": Nguyễn Văn A") {
		t.Error("missing value")
	}
	if !strings.Contains(got, "Ngày sinh") {
		t.Error("missing second label")
	}
}

// ── mistralRemoveEmptyColumns ─────────────────────────────────────────────────

func TestMistralRemoveEmptyColumns(t *testing.T) {
	rows := [][]string{
		{"A", "", "C"},
		{"D", "", "F"},
	}
	result := mistralRemoveEmptyColumns(rows)
	if len(result[0]) != 2 {
		t.Fatalf("expected 2 columns after removing empty col, got %d", len(result[0]))
	}
	if result[0][0] != "A" || result[0][1] != "C" {
		t.Errorf("unexpected result: %v", result[0])
	}

	// No empty columns — unchanged
	full := [][]string{{"A", "B"}, {"C", "D"}}
	result2 := mistralRemoveEmptyColumns(full)
	if len(result2[0]) != 2 {
		t.Error("should not remove non-empty columns")
	}
}

// ── mistralIsLikelyCrossPageTable ─────────────────────────────────────────────

func TestMistralIsLikelyCrossPageTable(t *testing.T) {
	crossPage := `<table><thead><tr><th>Col</th></tr></thead><tbody><tr><td>[2.1]</td></tr></tbody></table>`
	if !mistralIsLikelyCrossPageTable(crossPage) {
		t.Error("should detect cross-page table with bracket placeholders + 1 data row")
	}

	normal := `<table><thead><tr><th>Col</th></tr></thead><tbody><tr><td>Data</td></tr><tr><td>More</td></tr></tbody></table>`
	if mistralIsLikelyCrossPageTable(normal) {
		t.Error("multi-row table should not be detected as cross-page")
	}

	noBrackets := `<table><thead><tr><th>Col</th></tr></thead><tbody><tr><td>data</td></tr></tbody></table>`
	if mistralIsLikelyCrossPageTable(noBrackets) {
		t.Error("table without bracket placeholders should not be cross-page")
	}
}

// ── mistralTableColCount / mistralTableDataColCount ───────────────────────────

func TestMistralTableColCount(t *testing.T) {
	html := `<table><thead><tr><th>A</th><th>B</th><th>C</th></tr></thead><tbody><tr><td>1</td><td>2</td><td>3</td></tr></tbody></table>`
	if got := mistralTableColCount(html); got != 3 {
		t.Errorf("mistralTableColCount = %d, want 3", got)
	}

	noThead := `<table><tbody><tr><td>1</td><td>2</td></tr></tbody></table>`
	if got := mistralTableColCount(noThead); got != 0 {
		t.Errorf("no thead should return 0, got %d", got)
	}
}

func TestMistralTableDataColCount(t *testing.T) {
	html := `<table><tbody><tr><td>A</td><td>B</td></tr></tbody></table>`
	if got := mistralTableDataColCount(html); got != 2 {
		t.Errorf("mistralTableDataColCount = %d, want 2", got)
	}

	noTbody := `<table><tr><td>A</td></tr></table>`
	if got := mistralTableDataColCount(noTbody); got != 0 {
		t.Errorf("no tbody should return 0, got %d", got)
	}
}

// ── mistralMergeCompanionRows ─────────────────────────────────────────────────

func TestMistralMergeCompanionRows(t *testing.T) {
	original := `<table><thead><tr><th>H</th></tr></thead><tbody><tr><td>R1</td></tr></tbody></table>`
	companion := `<table><tbody><tr><td>R2</td></tr><tr><th>R3</th></tr></tbody></table>`

	result := mistralMergeCompanionRows(original, companion)

	if !strings.Contains(result, "R1") {
		t.Error("should contain original row")
	}
	if !strings.Contains(result, "R2") {
		t.Error("should contain companion row R2")
	}
	if !strings.Contains(result, "R3") {
		t.Error("should contain companion row R3")
	}
	// companion <th> should be converted to <td>
	if strings.Count(result, "<th>") > 1 { // only original header remains
		t.Error("companion th should be converted to td")
	}
}

// ── mistralDetectSectionBox ───────────────────────────────────────────────────

func TestMistralDetectSectionBox(t *testing.T) {
	rows := [][]string{
		{"Nợ TK: 111", "some data"},
	}
	if !mistralDetectSectionBox(rows) {
		t.Error("should detect section box with Nợ TK: prefix")
	}

	normal := [][]string{
		{"Normal content", "value"},
	}
	if mistralDetectSectionBox(normal) {
		t.Error("should not detect normal table as section box")
	}
}

// ── mistralIsRightColumnLine ──────────────────────────────────────────────────

func TestMistralIsRightColumnLine(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"Nợ TK: 111", true},
		{"Có TK: 222", true},
		{"Phí: 100", true},
		{"VAT: 10", true},
		{"Normal text", false},
		{"", false},
	}
	for _, tc := range tests {
		got := mistralIsRightColumnLine(tc.input)
		if got != tc.want {
			t.Errorf("mistralIsRightColumnLine(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// ── mistralMarkdownToRegions (integration of pure logic) ─────────────────────

func TestMistralMarkdownToRegions_BasicText(t *testing.T) {
	md := "Some plain text paragraph."
	regions := mistralMarkdownToRegions(md)
	if len(regions) == 0 {
		t.Fatal("expected at least 1 region")
	}
	if regions[0].typ != "text" {
		t.Errorf("expected text region, got %q", regions[0].typ)
	}
}

func TestMistralMarkdownToRegions_HeadingDetection(t *testing.T) {
	md := "# CONTRACT TITLE\n\nSome text."
	regions := mistralMarkdownToRegions(md)
	if regions[0].typ != "title" {
		t.Errorf("# heading should be title region, got %q", regions[0].typ)
	}
	if regions[0].content != "CONTRACT TITLE" {
		t.Errorf("unexpected title content: %q", regions[0].content)
	}
}

func TestMistralMarkdownToRegions_SkipsCamScanner(t *testing.T) {
	md := "Scanned with CamScanner\n\nReal content."
	regions := mistralMarkdownToRegions(md)
	for _, r := range regions {
		if strings.Contains(r.content, "CamScanner") {
			t.Error("CamScanner watermark should be filtered out")
		}
	}
}

func TestMistralMarkdownToRegions_SkipsStampNoise(t *testing.T) {
	md := "TM/UBND PHƯỜNG\n\nReal document content."
	regions := mistralMarkdownToRegions(md)
	for _, r := range regions {
		if strings.Contains(r.content, "TM/UBND") {
			t.Error("stamp noise should be filtered out")
		}
	}
}

func TestMistralMarkdownToRegions_TableDetection(t *testing.T) {
	md := "| Header A | Header B |\n|---|---|\n| Data 1 | Data 2 |\n| Data 3 | Data 4 |"
	regions := mistralMarkdownToRegions(md)
	found := false
	for _, r := range regions {
		if r.typ == "table" {
			found = true
			if !strings.Contains(r.html, "<table") {
				t.Error("table region should contain HTML table")
			}
		}
	}
	if !found {
		t.Error("should detect table region")
	}
}

func TestMistralMarkdownToRegions_LabelValueAsText(t *testing.T) {
	md := "| Họ và tên | : Nguyễn Văn A |\n|---|---|\n| Ngày sinh | : 01/01/1990 |"
	regions := mistralMarkdownToRegions(md)
	for _, r := range regions {
		if r.typ == "table" {
			t.Error("label:value table should be converted to text, not table region")
		}
	}
	if len(regions) == 0 {
		t.Error("should produce text regions from label:value table")
	}
}

func TestMistralMarkdownToRegions_FigureDetection(t *testing.T) {
	md := "![stamp](data:image/png;base64,abc123)"
	regions := mistralMarkdownToRegions(md)
	if len(regions) != 1 || regions[0].typ != "figure" {
		t.Errorf("image reference should produce figure region, got %v", regions)
	}
}
