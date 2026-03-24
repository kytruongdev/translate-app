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
	if !strings.Contains(p, "CRITICAL OUTPUT LANGUAGE") {
		t.Fatal("expected output-language guard for markdown mode")
	}
	if !strings.Contains(p, "MONOLINGUAL OUTPUT") {
		t.Fatal("expected monolingual guard for non-zh target")
	}
	if !strings.Contains(p, "ABSOLUTE OUTPUT LANGUAGE RULE") {
		t.Fatal("expected absolute output language guard at top of prompt")
	}
}

func TestBuildTranslationSystemPrompt_EnglishSourceBranch(t *testing.T) {
	p := BuildTranslationSystemPrompt("en", "vi", "academic", false)
	if !strings.Contains(p, "source text is in English") || !strings.Contains(p, "Vietnamese") {
		t.Fatalf("unexpected en→vi prompt: %s", p)
	}
}

func TestSourceLangForTranslate(t *testing.T) {
	if SourceLangForTranslate("Xin chào thế giới") != "vi" {
		t.Fatal("expected vi for Vietnamese text")
	}
	if SourceLangForTranslate("Hello world") != "auto" {
		t.Fatal("expected auto for short ASCII")
	}
	longEn := "The Ministry of Education and Training has not yet taken many organizing measures at the university level. " +
		"Higher education lecturers need additional knowledge about human rights and relevant law."
	if SourceLangForTranslate(longEn) != "en" {
		t.Fatal("expected en for long Latin-only prose")
	}
	if SourceLangForTranslate(longEn+" 人") != "auto" {
		t.Fatal("expected auto when chunk contains Han script")
	}
}
