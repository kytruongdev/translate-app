package gateway

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"google.golang.org/genai"
)

// Thời gian chờ mỗi lần gọi Gemini (stream hoặc một shot). Tách khỏi context Wails để IPC không cắt ngang.
const geminiHTTPTimeout = 3 * time.Minute

var errMissingGeminiKey = errors.New("gemini: missing API key (config/keys.go)")

type geminiProvider struct {
	apiKey string
	model  string

	mu     sync.Mutex
	client *genai.Client
	cliErr error
}

func newGeminiProvider(apiKey, model string) AIProvider {
	return &geminiProvider{apiKey: strings.TrimSpace(apiKey), model: model}
}

func (p *geminiProvider) lazyClient() (*genai.Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client != nil || p.cliErr != nil {
		return p.client, p.cliErr
	}
	p.client, p.cliErr = genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  p.apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	return p.client, p.cliErr
}

func (p *geminiProvider) TranslateBatchStream(ctx context.Context, text, from, to, style string, events chan<- StreamEvent) error {
	system := BuildDocxBatchSystemPrompt(from, to, style)
	return p.doTranslateStream(ctx, system, text, events)
}

func (p *geminiProvider) TranslateStream(ctx context.Context, text, from, to, style string, preserveMarkdown bool, events chan<- StreamEvent) error {
	system := BuildTranslationSystemPrompt(from, to, style, preserveMarkdown)
	return p.doTranslateStream(ctx, system, text, events)
}

func (p *geminiProvider) doTranslateStream(ctx context.Context, system, text string, events chan<- StreamEvent) error {
	defer close(events)

	if p.apiKey == "" {
		_ = emit(ctx, events, StreamEvent{Type: "error", Error: errMissingGeminiKey})
		return errMissingGeminiKey
	}

	contents := []*genai.Content{genai.NewUserContentFromText(strings.TrimSpace(text))}
	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewUserContentFromText(system),
	}

	var sentChunk bool
	var lastErr error

	for attempt := 0; attempt < retryMaxAttempts; attempt++ {
		sentChunk = false
		if attempt > 0 {
			if !IsRetryableGemini(lastErr) {
				break
			}
			if err := sleepBackoff(ctx, attempt-1); err != nil {
				_ = emit(ctx, events, StreamEvent{Type: "error", Error: err})
				return err
			}
		}

		client, err := p.lazyClient()
		if err != nil {
			_ = emit(ctx, events, StreamEvent{Type: "error", Error: err})
			return err
		}

		apiCtx, cancel := context.WithTimeout(context.Background(), geminiHTTPTimeout)
		streamErr := p.consumeGeminiStream(apiCtx, ctx, client, contents, config, events, &sentChunk)
		cancel()

		if streamErr == nil {
			_ = emit(ctx, events, StreamEvent{Type: "done"})
			return nil
		}
		lastErr = streamErr

		if sentChunk {
			_ = emit(ctx, events, StreamEvent{Type: "error", Error: streamErr})
			return streamErr
		}

		// Chưa có chunk: thử generateContent khi stream “rỗng kỹ thuật” — không gọi khi 429/quota/4xx (trùng lỗi, tốn quota gấp đôi).
		if !sentChunk && !IsGeminiNoFallbackError(streamErr) {
			apiCtx2, cancel2 := context.WithTimeout(context.Background(), geminiHTTPTimeout)
			ok, oneErr := p.generateContentOnce(apiCtx2, ctx, client, modelName(p.model), contents, config, events)
			cancel2()
			if ok && oneErr == nil {
				_ = emit(ctx, events, StreamEvent{Type: "done"})
				return nil
			}
			if oneErr != nil {
				log.Printf("translate-app gemini: stream err=%v; generateContent fallback err=%v", streamErr, oneErr)
				lastErr = oneErr
			}
		}

		if !IsRetryableGemini(lastErr) || attempt == retryMaxAttempts-1 {
			_ = emit(ctx, events, StreamEvent{Type: "error", Error: lastErr})
			return lastErr
		}
	}

	if lastErr == nil {
		lastErr = errors.New("gemini: exhausted retries")
	}
	_ = emit(ctx, events, StreamEvent{Type: "error", Error: lastErr})
	return lastErr
}

func modelName(m string) string {
	m = strings.TrimSpace(m)
	if m == "" {
		return "gemini-2.0-flash"
	}
	return m
}

func (p *geminiProvider) consumeGeminiStream(
	apiCtx context.Context,
	emitCtx context.Context,
	client *genai.Client,
	contents []*genai.Content,
	config *genai.GenerateContentConfig,
	events chan<- StreamEvent,
	sentChunk *bool,
) error {
	model := modelName(p.model)

	for resp, err := range client.Models.GenerateContentStream(apiCtx, model, contents, config) {
		if err != nil {
			return err
		}
		if resp == nil {
			continue
		}
		chunk := geminiStreamVisibleText(resp)
		if chunk == "" {
			continue
		}
		*sentChunk = true
		if e := emit(emitCtx, events, StreamEvent{Type: "chunk", Content: chunk}); e != nil {
			return e
		}
	}
	if !*sentChunk {
		return errors.New("gemini: model returned no text (check model id, API key, safety filters; try gemini-2.0-flash)")
	}
	return nil
}

func (p *geminiProvider) generateContentOnce(
	apiCtx, emitCtx context.Context,
	client *genai.Client,
	model string,
	contents []*genai.Content,
	config *genai.GenerateContentConfig,
	events chan<- StreamEvent,
) (ok bool, err error) {
	resp, err := client.Models.GenerateContent(apiCtx, model, contents, config)
	if err != nil {
		return false, err
	}
	text := geminiStreamVisibleText(resp)
	if text == "" {
		return false, errors.New("gemini: generateContent returned empty text")
	}
	if e := emit(emitCtx, events, StreamEvent{Type: "chunk", Content: text}); e != nil {
		return false, e
	}
	return true, nil
}

// geminiStreamVisibleText prefers resp.Text() (skips thought parts). If empty, concatenates any text
// parts — fallback when the API tags streamed translation oddly so Text() is blank.
func geminiStreamVisibleText(r *genai.GenerateContentResponse) string {
	if r == nil {
		return ""
	}
	if s := r.Text(); s != "" {
		return s
	}
	if len(r.Candidates) == 0 || r.Candidates[0].Content == nil {
		return ""
	}
	var b strings.Builder
	for _, part := range r.Candidates[0].Content.Parts {
		if part == nil || part.Text == "" {
			continue
		}
		b.WriteString(part.Text)
	}
	return b.String()
}
