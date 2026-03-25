package gateway

import (
	"context"
	"errors"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// ErrOllamaNotRunning is returned when local Ollama is unreachable (BE-006).
var ErrOllamaNotRunning = errors.New("ollama: connection refused (is Ollama running?)")

type ollamaProvider struct {
	model  string
	client *openai.Client
}

func newOllamaProvider(model string) AIProvider {
	cfg := openai.DefaultConfig("ollama")
	cfg.BaseURL = "http://localhost:11434/v1"
	return &ollamaProvider{
		model:  model,
		client: openai.NewClientWithConfig(cfg),
	}
}

func (p *ollamaProvider) TranslateBatchStream(ctx context.Context, text, from, to, style string, events chan<- StreamEvent) error {
	defer close(events)

	model := strings.TrimSpace(p.model)
	if model == "" {
		model = "qwen2.5:7b"
	}

	system := BuildDocxBatchSystemPrompt(from, to, style)
	userText := strings.TrimSpace(text)

	isRetryable := func(err error) bool {
		if err == nil || IsConnectionRefused(err) {
			return false
		}
		return IsRetryableOpenAI(err)
	}

	err := openAIChatStream(ctx, p.client, model, system, userText, events, isRetryable)
	if err != nil {
		if IsConnectionRefused(err) {
			err = ErrOllamaNotRunning
		}
		_ = emit(ctx, events, StreamEvent{Type: "error", Error: err})
		return err
	}
	_ = emit(ctx, events, StreamEvent{Type: "done"})
	return nil
}

func (p *ollamaProvider) TranslateStream(ctx context.Context, text, from, to, style string, preserveMarkdown bool, events chan<- StreamEvent) error {
	defer close(events)

	model := strings.TrimSpace(p.model)
	if model == "" {
		model = "qwen2.5:7b"
	}

	system := BuildTranslationSystemPrompt(from, to, style, preserveMarkdown)
	userText := strings.TrimSpace(text)

	isRetryable := func(err error) bool {
		if err == nil || IsConnectionRefused(err) {
			return false
		}
		return IsRetryableOpenAI(err)
	}

	err := openAIChatStream(ctx, p.client, model, system, userText, events, isRetryable)
	if err != nil {
		if IsConnectionRefused(err) {
			err = ErrOllamaNotRunning
		}
		_ = emit(ctx, events, StreamEvent{Type: "error", Error: err})
		return err
	}
	_ = emit(ctx, events, StreamEvent{Type: "done"})
	return nil
}
