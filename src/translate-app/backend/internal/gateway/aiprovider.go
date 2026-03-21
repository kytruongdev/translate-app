package gateway

import (
	"context"
	"fmt"

	"translate-app/config"
	"translate-app/internal/model"
)

type StreamEvent struct {
	Type    string // "chunk" | "done" | "error"
	Content string
	Error   error
}

type AIProvider interface {
	// TranslateStream streams translated text. preserveMarkdown adds §9.1 Markdown preservation when true (e.g. displayMode bilingual).
	TranslateStream(ctx context.Context, text, from, to, style string, preserveMarkdown bool, events chan<- StreamEvent) error
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
