package gateway

import (
	"strings"
	"testing"
)

func TestBuildTranslationSystemPrompt_ViCasual(t *testing.T) {
	p := BuildTranslationSystemPrompt("vi", "en-US", "casual", false)
	if !strings.Contains(p, "Vietnamese") || !strings.Contains(p, "English (United States)") {
		t.Fatalf("unexpected prompt: %s", p)
	}
	if !strings.Contains(p, "Output ONLY the translated text") {
		t.Fatal("missing output rule")
	}
}

func TestBuildTranslationSystemPrompt_Markdown(t *testing.T) {
	p := BuildTranslationSystemPrompt("unknown", "en-US", "business", true)
	if !strings.Contains(p, "Markdown") {
		t.Fatal("expected markdown preservation")
	}
}
