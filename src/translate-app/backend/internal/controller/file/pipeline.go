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
	UserID      string
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

	if ext == ".docx" {
		c.runDocxTranslate(ctx, p, fail)
	} else {
		c.runPlainTranslate(ctx, p, ext, fail)
	}
}

// runDocxTranslate handles DOCX files using the XML-level translation pipeline.
// Structure (tables, images, columns) is preserved; only <w:t> text nodes are translated.
func (c *controller) runDocxTranslate(ctx context.Context, p fileTranslateParams, fail func(string)) {
	fmt.Printf("[DEBUG] runDocxTranslate — file=%s\n", filepath.Base(p.FilePath))
	ext := ".docx"

	// Extract plain text for preview display (source.md).
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

	// Detect source language.
	docSrcHint := gateway.SourceLangForTranslate(sourceMD)
	if docSrcHint != "auto" {
		_ = c.reg.Message().UpdateSourceLang(ctx, p.UserID, docSrcHint)
		_ = c.reg.Message().UpdateSourceLang(ctx, p.AssistantID, docSrcHint)
	}

	runtime.EventsEmit(ctx, "file:source", map[string]string{
		"sessionId":          p.SessionID,
		"assistantMessageId": p.AssistantID,
	})

	// Parse DOCX XML structure.
	df, err := ParseDocx(p.FilePath)
	if err != nil {
		fail(fmt.Sprintf("không đọc được cấu trúc DOCX: %v", err))
		return
	}
	if len(df.Paragraphs) == 0 {
		fail("DOCX không có nội dung văn bản")
		return
	}

	totalBatches := len(chunkDocxParagraphs(df.Paragraphs, charsPerChunk))

	// Translate all paragraphs via XML pipeline.
	translations, totalTokens, err := c.translateDocxFile(ctx, df, docSrcHint, p.TargetLang, p.Style, p.Provider,
		func(batchIdx, total int) {
			pct := 0
			if total > 0 {
				pct = (batchIdx * 100) / total
			}
			runtime.EventsEmit(ctx, "file:progress", bridge.FileProgress{
				Chunk:   batchIdx + 1,
				Total:   total,
				Percent: pct,
			})
		},
	)
	if err != nil {
		fail(err.Error())
		return
	}

	runtime.EventsEmit(ctx, "file:progress", bridge.FileProgress{
		Chunk:   totalBatches,
		Total:   totalBatches,
		Percent: 100,
	})

	// Write translated DOCX.
	translatedPath := filepath.Join(subDir, "translated.docx")
	if err := WriteTranslatedDocx(df, translations, translatedPath); err != nil {
		fail(fmt.Sprintf("không tạo được file DOCX đã dịch: %v", err))
		return
	}

	usedTokens := totalTokens
	if usedTokens == 0 {
		usedTokens = estimateTokens(sourceMD)
	}
	if err := c.reg.Message().UpdateTranslated(ctx, p.AssistantID, sourceMD, usedTokens); err != nil {
		fail(err.Error())
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
		FileID:    p.FileID,
		FileName:  filepath.Base(p.FilePath),
		FileType:  "docx",
		CharCount: charCount,
		PageCount: pageCount,
	})
}

// runPlainTranslate handles non-DOCX files (PDF) using the existing text pipeline.
func (c *controller) runPlainTranslate(ctx context.Context, p fileTranslateParams, ext string, fail func(string)) {
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

	translationText, err := extractTranslationText(p.FilePath, ext)
	if err != nil || translationText == "" {
		translationText = sourceMD
	}

	chunks := chunkMarkdownByParagraphs(translationText, charsPerChunk)
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

	docSrcHint := gateway.SourceLangForTranslate(sourceMD)
	if docSrcHint != "auto" {
		_ = c.reg.Message().UpdateSourceLang(ctx, p.UserID, docSrcHint)
		_ = c.reg.Message().UpdateSourceLang(ctx, p.AssistantID, docSrcHint)
	}

	runtime.EventsEmit(ctx, "file:source", map[string]string{
		"markdown":           sourceMD,
		"sessionId":          p.SessionID,
		"assistantMessageId": p.AssistantID,
	})

	total := len(chunks)
	var cumulative strings.Builder
	var totalTokens int

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

		translated, tokens, err := c.streamTranslate(ctx, p.Provider, chunk, docSrcHint, p.TargetLang, p.Style, true, func(delta string) {
			runtime.EventsEmit(ctx, "translation:chunk", delta)
		})
		if err != nil {
			fail(err.Error())
			return
		}
		totalTokens += tokens
		cumulative.WriteString(translated)
		sum := cumulative.String()
		usedTokens := totalTokens
		if usedTokens == 0 {
			usedTokens = estimateTokens(sum)
		}
		if err := c.reg.Message().UpdateTranslated(ctx, p.AssistantID, sum, usedTokens); err != nil {
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

	finalTokens := totalTokens
	if finalTokens == 0 {
		finalTokens = estimateTokens(cumulative.String())
	}
	runtime.EventsEmit(ctx, "file:done", bridge.FileResult{
		FileID:     p.FileID,
		FileName:   filepath.Base(p.FilePath),
		FileType:   strings.TrimPrefix(ext, "."),
		CharCount:  charCount,
		PageCount:  pageCount,
		TokensUsed: finalTokens,
	})
}

// streamTranslate streams a translation and returns (text, tokensUsed, error).
// tokensUsed is the real API token count when the provider reports it (OpenAI),
// or 0 for providers that don't emit usage events (Ollama, Gemini).
func (c *controller) streamTranslate(
	ctx context.Context,
	provider gateway.AIProvider,
	text, sourceLang, targetLang string,
	style model.TranslationStyle,
	preserveMD bool,
	onDelta func(string),
) (string, int, error) {
	events := make(chan gateway.StreamEvent, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- provider.TranslateStream(ctx, text, sourceLang, targetLang, string(style), preserveMD, events)
	}()

	var full strings.Builder
	var tokensUsed int
	for ev := range events {
		switch ev.Type {
		case "chunk":
			if ev.Content != "" {
				full.WriteString(ev.Content)
				if onDelta != nil {
					onDelta(ev.Content)
				}
			}
		case "usage":
			tokensUsed = ev.TokensUsed
		case "error":
			errMsg := errors.New("translation failed")
			if ev.Error != nil {
				errMsg = ev.Error
			}
			<-errCh
			return "", 0, errMsg
		}
	}

	if streamErr := <-errCh; streamErr != nil {
		return "", 0, streamErr
	}
	return full.String(), tokensUsed, nil
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
