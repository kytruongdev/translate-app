package gateway

import (
	"context"
	"errors"
	"strings"

	"github.com/sashabaranov/go-openai"
)

var errMissingOpenAIKey = errors.New("openai: missing API key (config/keys.go)")

type openaiProvider struct {
	apiKey string
	model  string
	client *openai.Client
}

func newOpenAIProvider(apiKey, model string) AIProvider {
	key := strings.TrimSpace(apiKey)
	cfg := openai.DefaultConfig(key)
	return &openaiProvider{
		apiKey: key,
		model:  model,
		client: openai.NewClientWithConfig(cfg),
	}
}

func (p *openaiProvider) TranslateBatchStream(ctx context.Context, text, from, to, style string, events chan<- StreamEvent) error {
	defer close(events)

	if p.apiKey == "" {
		_ = emit(ctx, events, StreamEvent{Type: "error", Error: errMissingOpenAIKey})
		return errMissingOpenAIKey
	}

	model := strings.TrimSpace(p.model)
	if model == "" {
		model = "gpt-4o-mini"
	}

	system := BuildDocxBatchSystemPromptGPT(from, to, style)
	userText := strings.TrimSpace(text)

	err := openAIChatStream(ctx, p.client, model, system, userText, events, IsRetryableOpenAI)
	if err != nil {
		_ = emit(ctx, events, StreamEvent{Type: "error", Error: err})
		return err
	}
	_ = emit(ctx, events, StreamEvent{Type: "done"})
	return nil
}

func (p *openaiProvider) TranslateStream(ctx context.Context, text, from, to, style string, preserveMarkdown bool, events chan<- StreamEvent) error {
	defer close(events)

	if p.apiKey == "" {
		_ = emit(ctx, events, StreamEvent{Type: "error", Error: errMissingOpenAIKey})
		return errMissingOpenAIKey
	}

	model := strings.TrimSpace(p.model)
	if model == "" {
		model = "gpt-4o-mini"
	}

	system := BuildTranslationSystemPromptGPT(from, to, style, preserveMarkdown)
	userText := strings.TrimSpace(text)

	err := openAIChatStream(ctx, p.client, model, system, userText, events, IsRetryableOpenAI)
	if err != nil {
		_ = emit(ctx, events, StreamEvent{Type: "error", Error: err})
		return err
	}
	_ = emit(ctx, events, StreamEvent{Type: "done"})
	return nil
}
