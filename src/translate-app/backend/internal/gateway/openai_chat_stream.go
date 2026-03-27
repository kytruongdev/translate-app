package gateway

import (
	"context"
	"errors"
	"io"

	"github.com/sashabaranov/go-openai"
)

// openAIChatStream runs streaming chat completion with retry (no retry after first token).
func openAIChatStream(
	ctx context.Context,
	client *openai.Client,
	model string,
	systemPrompt string,
	userText string,
	events chan<- StreamEvent,
	isRetryable func(error) bool,
) error {
	var sentChunk bool
	var lastErr error

	for attempt := 0; attempt < retryMaxAttempts; attempt++ {
		if attempt > 0 {
			if !isRetryable(lastErr) {
				break
			}
			if err := sleepBackoff(ctx, attempt-1); err != nil {
				return err
			}
		}

		temp := float32(0)
		stream, err := client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: userText},
			},
			Temperature:   temp,
			Stream:        true,
			StreamOptions: &openai.StreamOptions{IncludeUsage: true},
		})
		if err != nil {
			lastErr = err
			if !sentChunk && isRetryable(err) && attempt < retryMaxAttempts-1 {
				continue
			}
			return err
		}

		streamErr := func() error {
			defer stream.Close()
			return drainOpenAIStream(ctx, stream, events, &sentChunk)
		}()
		if streamErr == nil {
			return nil
		}
		lastErr = streamErr

		if sentChunk {
			return streamErr
		}
		if !isRetryable(streamErr) || attempt == retryMaxAttempts-1 {
			return streamErr
		}
	}

	if lastErr == nil {
		lastErr = errors.New("openai: exhausted retries")
	}
	return lastErr
}

func drainOpenAIStream(ctx context.Context, stream *openai.ChatCompletionStream, events chan<- StreamEvent, sentChunk *bool) error {
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		// Final chunk with usage data (IncludeUsage=true, Choices is empty).
		if len(resp.Choices) == 0 {
			if resp.Usage != nil {
				_ = emit(ctx, events, StreamEvent{Type: "usage", TokensUsed: resp.Usage.TotalTokens})
			}
			continue
		}
		delta := resp.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		*sentChunk = true
		if e := emit(ctx, events, StreamEvent{Type: "chunk", Content: delta}); e != nil {
			return e
		}
	}
}
