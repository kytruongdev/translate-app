package file

import (
	"context"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"translate-app/internal/bridge"
	"translate-app/internal/gateway"
	"translate-app/internal/model"
)

// RetranslateContentParams — params for re-running the chunked pipeline on already-extracted markdown.
type RetranslateContentParams struct {
	SessionID   string
	AssistantID string
	SourceMD    string
	TargetLang  string
	Style       model.TranslationStyle
	Provider    gateway.AIProvider
}

// RunRetranslateContent runs the same chunked translation pipeline as TranslateFile,
// but skips file extraction — source markdown is already stored in the message row.
// Emits file:progress, translation:chunk, translation:done (and file:error on failure).
func (c *controller) RunRetranslateContent(ctx context.Context, p RetranslateContentParams) {
	fail := func(msg string) {
		runtime.EventsEmit(ctx, "translation:error", msg)
		runtime.EventsEmit(ctx, "file:error", msg)
	}

	chunks := chunkMarkdownByParagraphs(p.SourceMD, charsPerChunk)
	if len(chunks) == 0 {
		fail("nội dung rỗng")
		return
	}

	total := len(chunks)
	var cumulative strings.Builder

	// Detect source language once on full document (same fix as runFileTranslate).
	docSrcHint := gateway.SourceLangForTranslate(p.SourceMD)

	for i, chunk := range chunks {
		pct := 0
		if total > 0 {
			pct = (i * 100) / total
		}
		runtime.EventsEmit(ctx, "file:progress", bridge.FileProgress{
			Chunk:   i + 1,
			Total:   total,
			Percent: pct,
		})

		srcHint := docSrcHint
		translated, err := c.streamTranslate(ctx, p.Provider, chunk, srcHint, p.TargetLang, p.Style, true, func(delta string) {
			runtime.EventsEmit(ctx, "translation:chunk", delta)
		})
		if err != nil {
			fail(err.Error())
			return
		}
		cumulative.WriteString(translated)
		sum := cumulative.String()
		if err := c.reg.Message().UpdateTranslated(ctx, p.AssistantID, sum, estimateTokens(sum)); err != nil {
			fail(err.Error())
			return
		}
	}

	runtime.EventsEmit(ctx, "file:progress", bridge.FileProgress{
		Chunk:   total,
		Total:   total,
		Percent: 100,
	})

	msg, err := c.reg.Message().GetByID(ctx, p.AssistantID)
	if err != nil || msg == nil {
		fail("không tải được tin nhắn sau khi dịch lại")
		return
	}
	runtime.EventsEmit(ctx, "translation:done", *msg)
}
