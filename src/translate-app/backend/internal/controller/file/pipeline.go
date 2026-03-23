package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"translate-app/internal/bridge"
	"translate-app/internal/gateway"
	"translate-app/internal/model"
)

type fileTranslateParams struct {
	SessionID   string
	FilePath    string
	FileID      string
	AssistantID string
	TargetLang  string
	Style       model.TranslationStyle
	ModelUsed   string
	PageCount   int
	Provider    gateway.AIProvider
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

func (c *controller) runFileTranslate(ctx context.Context, p fileTranslateParams) {
	fail := func(msg string) {
		_ = c.reg.File().UpdateStatus(ctx, p.FileID, "error", msg)
		runtime.EventsEmit(ctx, "translation:error", msg)
		runtime.EventsEmit(ctx, "file:error", msg)
	}

	ext := fileExt(p.FilePath)
	plain, err := extractSourceMarkdown(p.FilePath, ext)
	if err != nil {
		fail(err.Error())
		return
	}
	sourceMD := sourceMarkdownFromPlain(plain)
	if sourceMD == "" {
		fail("không trích được văn bản từ tệp")
		return
	}

	chunks := chunkMarkdownByParagraphs(sourceMD, charsPerChunk)
	if len(chunks) == 0 {
		fail("nội dung tệp rỗng")
		return
	}

	dir, err := userFilesDir()
	if err != nil {
		fail(err.Error())
		return
	}
	subDir := filepath.Join(dir, p.FileID)
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		fail(fmt.Sprintf("không tạo được thư mục lưu: %v", err))
		return
	}
	sourcePath := filepath.Join(subDir, "source.md")
	if err := os.WriteFile(sourcePath, []byte(sourceMD), 0o644); err != nil {
		fail(fmt.Sprintf("không ghi được source.md: %v", err))
		return
	}

	charCount, pageCount := charAndPageCount(sourceMD, ext, p.PageCount)
	if err := c.reg.File().UpdateExtracted(ctx, p.FileID, sourcePath, charCount, pageCount); err != nil {
		fail(err.Error())
		return
	}

	if err := c.reg.Message().UpdateOriginalContent(ctx, p.AssistantID, sourceMD); err != nil {
		fail(err.Error())
		return
	}

	runtime.EventsEmit(ctx, "file:source", map[string]string{
		"markdown":           sourceMD,
		"sessionId":          p.SessionID,
		"assistantMessageId": p.AssistantID,
	})

	total := len(chunks)
	var cumulative strings.Builder
	preserveMD := true // bilingual-friendly output

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

		// Không cứng "auto" — với văn có dấu Việt dùng nhánh "từ tiếng Việt → đích" (prompt chặt hơn "detect language").
		srcHint := gateway.SourceLangForTranslate(chunk)
		translated, err := c.streamTranslate(ctx, p.Provider, chunk, srcHint, p.TargetLang, p.Style, preserveMD, func(delta string) {
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
		runtime.EventsEmit(ctx, "file:chunk_done", map[string]any{
			"chunkIndex": i,
			"text":       translated,
		})
	}

	runtime.EventsEmit(ctx, "file:progress", bridge.FileProgress{
		Chunk:   total,
		Total:   total,
		Percent: 100,
	})

	translatedPath := filepath.Join(subDir, "translated.md")
	fullTranslated := cumulative.String()
	if err := os.WriteFile(translatedPath, []byte(fullTranslated), 0o644); err != nil {
		fail(fmt.Sprintf("không ghi được translated.md: %v", err))
		return
	}

	if err := c.reg.File().UpdateTranslated(ctx, p.FileID, sourcePath, translatedPath, charCount, pageCount, p.ModelUsed); err != nil {
		fail(err.Error())
		return
	}

	msg, err := c.reg.Message().GetByID(ctx, p.AssistantID)
	if err != nil || msg == nil {
		fail("không tải được tin nhắn sau khi dịch file")
		return
	}
	runtime.EventsEmit(ctx, "translation:done", *msg)

	runtime.EventsEmit(ctx, "file:done", bridge.FileResult{
		FileID:     p.FileID,
		FileName:   filepath.Base(p.FilePath),
		FileType:   strings.TrimPrefix(ext, "."),
		CharCount:  charCount,
		PageCount:  pageCount,
		TokensUsed: estimateTokens(fullTranslated),
	})
}

func (c *controller) streamTranslate(
	ctx context.Context,
	provider gateway.AIProvider,
	text, sourceLang, targetLang string,
	style model.TranslationStyle,
	preserveMD bool,
	onDelta func(string),
) (string, error) {
	events := make(chan gateway.StreamEvent, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- provider.TranslateStream(ctx, text, sourceLang, targetLang, string(style), preserveMD, events)
	}()

	var full strings.Builder
	for ev := range events {
		switch ev.Type {
		case "chunk":
			if ev.Content != "" {
				full.WriteString(ev.Content)
				if onDelta != nil {
					onDelta(ev.Content)
				}
			}
		case "error":
			errMsg := errors.New("translation failed")
			if ev.Error != nil {
				errMsg = ev.Error
			}
			<-errCh
			return "", errMsg
		case "done":
		}
	}

	streamErr := <-errCh
	if streamErr != nil {
		return "", streamErr
	}
	return full.String(), nil
}

func userFilesDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, "TranslateApp", "files")
	if err := os.MkdirAll(p, 0o755); err != nil {
		return "", err
	}
	return p, nil
}
