package file

import (
	"strings"
	"testing"
)

func TestChunkDocxParagraphs_Basic(t *testing.T) {
	paras := []DocxParagraph{
		{Index: 0, Text: "First paragraph"},
		{Index: 1, Text: "Second paragraph"},
		{Index: 2, Text: "Third paragraph"},
	}
	batches := chunkDocxParagraphs(paras, 100)
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(batches))
	}
	if len(batches[0]) != 3 {
		t.Fatalf("expected 3 paragraphs in batch, got %d", len(batches[0]))
	}
}

func TestChunkDocxParagraphs_SplitsOnLimit(t *testing.T) {
	paras := []DocxParagraph{
		{Index: 0, Text: strings.Repeat("a", 20)},
		{Index: 1, Text: strings.Repeat("b", 20)},
		{Index: 2, Text: strings.Repeat("c", 20)},
	}
	// maxChars=25: each para is 20 runes, two paras = 20+1+20=41 > 25, so each goes solo
	batches := chunkDocxParagraphs(paras, 25)
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches, got %d", len(batches))
	}
}

func TestChunkDocxParagraphs_PreservesIndexAlignment(t *testing.T) {
	paras := make([]DocxParagraph, 10)
	for i := range paras {
		paras[i] = DocxParagraph{Index: i, Text: strings.Repeat("x", 50)}
	}
	batches := chunkDocxParagraphs(paras, 120)
	total := 0
	for _, b := range batches {
		total += len(b)
	}
	if total != len(paras) {
		t.Errorf("total paragraphs across batches = %d, want %d", total, len(paras))
	}
}

func TestChunkDocxParagraphs_EmptyInput(t *testing.T) {
	batches := chunkDocxParagraphs(nil, 2500)
	if len(batches) != 0 {
		t.Errorf("expected 0 batches for nil input, got %d", len(batches))
	}
}
