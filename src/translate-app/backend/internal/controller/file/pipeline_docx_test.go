package file

import (
	"strings"
	"testing"
)

func TestBuildBatchInput_Basic(t *testing.T) {
	batch := []DocxParagraph{
		{Text: "Hello world"},
		{Text: "Second paragraph"},
	}
	got := buildBatchInput(batch, []int{0, 1})
	if !strings.Contains(got, "<<<1>>>") || !strings.Contains(got, "<<<2>>>") {
		t.Errorf("missing <<<N>>> markers, got: %s", got)
	}
	if !strings.Contains(got, "Hello world") {
		t.Errorf("missing paragraph text, got: %s", got)
	}
	// Should not use old [N] format
	if strings.Contains(got, "[1]\n") {
		t.Errorf("should not use [N] format, got: %s", got)
	}
}

func TestBuildBatchInput_SkipsEmpty(t *testing.T) {
	batch := []DocxParagraph{
		{Text: "Hello"},
		{Text: ""},
		{Text: "World"},
	}
	// localIndices only contains non-empty: 0 and 2
	got := buildBatchInput(batch, []int{0, 2})
	if strings.Contains(got, "<<<3>>>") {
		t.Errorf("should only have <<<1>>> and <<<2>>>, got: %s", got)
	}
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "World") {
		t.Errorf("missing text, got: %s", got)
	}
}

func TestBuildBatchInput_NoCollisionWithListMarkers(t *testing.T) {
	// Ensure <<<N>>> doesn't appear in normal document content
	batch := []DocxParagraph{
		{Text: "[1] TS. Pham Thi Minh Chau - textbook reference"},
		{Text: "[2] Another reference"},
	}
	got := buildBatchInput(batch, []int{0, 1})
	// Batch markers should be <<<N>>> not [N]
	if !strings.Contains(got, "<<<1>>>") || !strings.Contains(got, "<<<2>>>") {
		t.Errorf("missing <<<N>>> markers, got: %s", got)
	}
}

func TestParseBatchOutput_Basic(t *testing.T) {
	raw := "<<<1>>>\nXin chào thế giới\n\n<<<2>>>\nĐoạn thứ hai"
	got := parseBatchOutput(raw, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0] != "Xin chào thế giới" {
		t.Errorf("got[0] = %q, want %q", got[0], "Xin chào thế giới")
	}
	if got[1] != "Đoạn thứ hai" {
		t.Errorf("got[1] = %q, want %q", got[1], "Đoạn thứ hai")
	}
}

func TestParseBatchOutput_FallbackNoMarkers(t *testing.T) {
	// AI ignored format — split by double newlines and assign sequentially.
	raw := "Translation one\n\nTranslation two\n\nTranslation three"
	got := parseBatchOutput(raw, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 slots, got %d", len(got))
	}
	if got[0] != "Translation one" {
		t.Errorf("got[0] = %q, want %q", got[0], "Translation one")
	}
	if got[1] != "Translation two" {
		t.Errorf("got[1] = %q, want %q", got[1], "Translation two")
	}
	if got[2] != "Translation three" {
		t.Errorf("got[2] = %q, want %q", got[2], "Translation three")
	}
}

func TestParseBatchOutput_FallbackSingleParagraph(t *testing.T) {
	// AI returned single block without separators — goes to slot 0.
	raw := "Xin chào thế giới"
	got := parseBatchOutput(raw, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 slots, got %d", len(got))
	}
	if got[0] != raw {
		t.Errorf("got[0] = %q, want %q", got[0], raw)
	}
	if got[1] != "" || got[2] != "" {
		t.Errorf("slots 1,2 should be empty, got %q %q", got[1], got[2])
	}
}

func TestParseBatchOutput_NoCollisionWithDocListMarkers(t *testing.T) {
	// Old [N] list markers in document content should NOT trigger batch parsing.
	// The AI translates and happens to include "[1] Author..." in output.
	raw := "[1] TS. Pham Thi Minh Chau - textbook\n\n[2] Another reference"
	got := parseBatchOutput(raw, 2)
	// No <<<N>>> markers found → fallback by double newline split
	if got[0] != "[1] TS. Pham Thi Minh Chau - textbook" {
		t.Errorf("got[0] = %q", got[0])
	}
	if got[1] != "[2] Another reference" {
		t.Errorf("got[1] = %q", got[1])
	}
}

func TestParseBatchOutput_FewerThanExpected(t *testing.T) {
	// AI returns only 1 segment but we expected 3.
	raw := "<<<1>>>\nOnly one paragraph"
	got := parseBatchOutput(raw, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 slots, got %d", len(got))
	}
	if got[0] != "Only one paragraph" {
		t.Errorf("got[0] = %q", got[0])
	}
	if got[1] != "" || got[2] != "" {
		t.Errorf("missing slots should be empty")
	}
}

func TestParseBatchOutput_MultilineContent(t *testing.T) {
	raw := "<<<1>>>\nLine one\nLine two\n\n<<<2>>>\nParagraph two"
	got := parseBatchOutput(raw, 2)
	if !strings.Contains(got[0], "Line one") || !strings.Contains(got[0], "Line two") {
		t.Errorf("multiline content not preserved, got: %q", got[0])
	}
}

func TestParseBatchOutput_LeadingWhitespaceMarkers(t *testing.T) {
	// AI returns " <<<N>>>" (leading space) — must still be parsed correctly.
	raw := " <<<1>>>\nXin chào\n\n <<<2>>>\nThế giới"
	got := parseBatchOutput(raw, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0] != "Xin chào" {
		t.Errorf("got[0] = %q, want %q", got[0], "Xin chào")
	}
	if got[1] != "Thế giới" {
		t.Errorf("got[1] = %q, want %q", got[1], "Thế giới")
	}
}

func TestParseBatchOutput_AmplifiedMarkers(t *testing.T) {
	// AI "amplifies" <<<N>>> to <<<<<<<N>>>>>>> (7 brackets) — must still parse correctly
	// and NOT include the marker text in the extracted content.
	raw := "<<<<<<<1>>>>>>> \nSelf-study activities:\n\n<<<<<<<2>>>>>>> \n- Read chapter 7 beforehand"
	got := parseBatchOutput(raw, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0] != "Self-study activities:" {
		t.Errorf("got[0] = %q, want %q", got[0], "Self-study activities:")
	}
	if got[1] != "- Read chapter 7 beforehand" {
		t.Errorf("got[1] = %q, want %q", got[1], "- Read chapter 7 beforehand")
	}
}

func TestParseBatchOutput_Empty(t *testing.T) {
	got := parseBatchOutput("", 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 slots for empty input")
	}
	for _, s := range got {
		if s != "" {
			t.Errorf("expected empty slot, got %q", s)
		}
	}
}
