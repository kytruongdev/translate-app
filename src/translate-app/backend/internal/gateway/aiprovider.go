package gateway

import (
	"context"
	"fmt"

	"translate-app/config"
	"translate-app/internal/model"
)

type StreamEvent struct {
	Type       string // "chunk" | "done" | "error" | "usage"
	Content    string
	Error      error
	TokensUsed int // set on Type=="usage" (OpenAI only; 0 for other providers)
}

type AIProvider interface {
	// TranslateStream streams translated text. preserveMarkdown adds §9.1 Markdown preservation when true (e.g. displayMode bilingual).
	TranslateStream(ctx context.Context, text, from, to, style string, preserveMarkdown bool, events chan<- StreamEvent) error
	// TranslateBatchStream streams a DOCX paragraph batch translation using a dedicated system prompt
	// that explicitly preserves <<<N>>> markers without conflicting "output-only" instructions.
	TranslateBatchStream(ctx context.Context, text, from, to, style string, events chan<- StreamEvent) error
	// MaxBatchConcurrency returns how many DOCX batches can run in parallel.
	// Local providers (Ollama) return 1; cloud providers return higher values.
	MaxBatchConcurrency() int
}

// NewFromSettings returns the default AI client from persisted settings.
func NewFromSettings(settings *model.Settings, keys *config.APIKeys) (AIProvider, error) {
	return ForProvider(settings.ActiveProvider, settings.ActiveModel, keys)
}

// New is an alias for NewFromSettings (BE-007 factory name).
func New(settings *model.Settings, keys *config.APIKeys) (AIProvider, error) {
	return NewFromSettings(settings, keys)
}

// ForProvider builds a provider by name (global or per-request override).
func ForProvider(providerName, modelName string, keys *config.APIKeys) (AIProvider, error) {
	switch providerName {
	case "gemini":
		m := modelName
		if m == "" {
			m = "gemini-2.0-flash"
		}
		return newGeminiProvider(keys.GeminiKey, m), nil
	case "ollama":
		m := modelName
		if m == "" {
			m = "qwen2.5:7b"
		}
		return newOllamaProvider(m), nil
	case "openai":
		m := modelName
		if m == "" {
			m = "gpt-4o-mini"
		}
		return newOpenAIProvider(keys.OpenAIKey, m), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", providerName)
	}
}
