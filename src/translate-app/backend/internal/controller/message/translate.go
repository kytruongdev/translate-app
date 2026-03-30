package message

import (
	"context"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"translate-app/internal/gateway"
	"translate-app/internal/model"
)

// runTranslationStream consumes gateway stream events and emits Wails events; updates assistant row on success.
func (c *controller) runTranslationStream(
	ctx context.Context,
	sessionID string,
	assistantMsgID string,
	text string,
	sourceLang string,
	targetLang string,
	style model.TranslationStyle,
	preserveMarkdown bool,
	provider gateway.AIProvider,
) {
	startTime := time.Now()
	events := make(chan gateway.StreamEvent, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- provider.TranslateStream(ctx, text, sourceLang, targetLang, string(style), preserveMarkdown, events)
	}()

	var full strings.Builder
	for ev := range events {
		switch ev.Type {
		case "chunk":
			if ev.Content != "" {
				full.WriteString(ev.Content)
				runtime.EventsEmit(ctx, "translation:chunk", ev.Content)
			}
		case "error":
			errMsg := "translation failed"
			if ev.Error != nil {
				errMsg = ev.Error.Error()
			}
			<-errCh
			c.log.Error("TranslationFailed",
				"msgId", assistantMsgID, "sessionId", sessionID,
				"durationMs", time.Since(startTime).Milliseconds(), "error", errMsg)
			runtime.EventsEmit(ctx, "translation:error", errMsg)
			return
		case "done":
			// Marker from provider; chunks already accumulated.
		}
	}

	streamErr := <-errCh
	if streamErr != nil {
		c.log.Error("TranslationFailed",
			"msgId", assistantMsgID, "sessionId", sessionID,
			"durationMs", time.Since(startTime).Milliseconds(), "error", streamErr.Error())
		runtime.EventsEmit(ctx, "translation:error", streamErr.Error())
		return
	}

	tokens := estimateTokens(full.String())
	if err := c.reg.Message().UpdateTranslated(ctx, assistantMsgID, full.String(), tokens); err != nil {
		runtime.EventsEmit(ctx, "translation:error", err.Error())
		return
	}
	msg, err := c.reg.Message().GetByID(ctx, assistantMsgID)
	if err != nil || msg == nil {
		runtime.EventsEmit(ctx, "translation:error", "failed to load message after translate")
		return
	}
	c.log.Info("TranslationDone",
		"msgId", assistantMsgID, "sessionId", sessionID,
		"durationMs", time.Since(startTime).Milliseconds(), "tokens", tokens)
	runtime.EventsEmit(ctx, "translation:done", *msg)
}

func estimateTokens(s string) int {
	if s == "" {
		return 0
	}
	n := len(s) / 4
	if n < 1 {
		return 1
	}
	return n
}
