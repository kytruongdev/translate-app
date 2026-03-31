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

func TestBuildDocxBatchSystemPrompt_NoConflict(t *testing.T) {
	p := BuildDocxBatchSystemPrompt("vi", "en-US", "academic")
	// Must contain marker format rule
	if !strings.Contains(p, "<<<N>>>") {
		t.Fatal("missing <<<N>>> marker format rule")
	}
	if !strings.Contains(p, "<<<1>>>") {
		t.Fatal("missing example marker <<<1>>>")
	}
	// Must NOT have the conflicting "output only" instruction
	if strings.Contains(p, "Output ONLY the translated text") {
		t.Fatal("should not contain conflicting output-only instruction")
	}
	// Must still have language guards
	if !strings.Contains(p, "ABSOLUTE OUTPUT LANGUAGE RULE") {
		t.Fatal("missing absolute output language guard")
	}
	if !strings.Contains(p, "MONOLINGUAL OUTPUT") {
		t.Fatal("missing monolingual constraint")
	}
	// Must mention source language
	if !strings.Contains(p, "Vietnamese") {
		t.Fatal("missing source language (Vietnamese)")
	}
	if !strings.Contains(p, "English (United States)") {
		t.Fatal("missing target language")
	}
}

func TestBuildDocxBatchSystemPrompt_AutoSource(t *testing.T) {
	p := BuildDocxBatchSystemPrompt("auto", "vi", "casual")
	// auto source should not mention a specific source language
	if strings.Contains(p, "from Vietnamese") || strings.Contains(p, "from English") {
		t.Fatal("auto source should not specify source language")
	}
	if !strings.Contains(p, "Vietnamese") {
		t.Fatal("missing target language Vietnamese")
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
