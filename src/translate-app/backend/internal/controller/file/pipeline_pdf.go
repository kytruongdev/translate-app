package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"translate-app/internal/bridge"
	"translate-app/internal/gateway"
	"translate-app/internal/model"
)

// runPDFTranslate handles PDF files:
// 1. Extract text page by page with rule-based cleaning + cross-page merge
// 2. Chunk by character count
// 3. Translate chunks concurrently
// 4. Write output as DOCX plain paragraphs
func (c *controller) runPDFTranslate(ctx context.Context, p fileTranslateParams, fail func(string)) {
	// Step 1: Extract + clean text.
	text, err := extractPDFWithClean(p.FilePath)
	if err != nil {
		fail(err.Error())
		return
	}
	if strings.TrimSpace(text) == "" {
		fail("không trích xuất được văn bản từ PDF")
		return
	}

	// Prepare file storage directory.
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

	// Write source.md for display.
	sourcePath := filepath.Join(subDir, "source.md")
	if err := os.WriteFile(sourcePath, []byte(text), 0o644); err != nil {
		fail(fmt.Sprintf("không ghi được source.md: %v", err))
		return
	}

	charCount := utf8.RuneCountInString(text)
	pageCount := p.PageCount
	if pageCount < 1 {
		pageCount = max(1, (charCount+docxCharsPerPage-1)/docxCharsPerPage)
	}

	if err := c.reg.File().UpdateExtracted(ctx, p.FileID, sourcePath, charCount, pageCount); err != nil {
		fail(err.Error())
		return
	}

	// Detect source language.
	srcLang := gateway.SourceLangForTranslate(text)
	if srcLang != "auto" {
		_ = c.reg.Message().UpdateSourceLang(ctx, p.UserID, srcLang)
		_ = c.reg.Message().UpdateSourceLang(ctx, p.AssistantID, srcLang)
	}

	// Emit file:source so frontend renders the source card.
	runtime.EventsEmit(ctx, "file:source", map[string]string{
		"sessionId":          p.SessionID,
		"assistantMessageId": p.AssistantID,
	})

	// Step 2: Chunk text by paragraph boundaries.
	chunks := chunkPlainText(text, charsPerChunk)
	total := len(chunks)
	if total == 0 {
		fail("không có nội dung để dịch")
		return
	}

	// Step 3: Translate chunks concurrently.
	translated, totalTokens, err := c.translatePlainChunks(ctx, chunks, srcLang, p.TargetLang, p.Style, p.Provider,
		func(completed, total int) {
			pct := 0
			if total > 0 {
				pct = (completed * 100) / total
			}
			runtime.EventsEmit(ctx, "file:progress", bridge.FileProgress{
				Chunk:   completed,
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
		Chunk:   total,
		Total:   total,
		Percent: 100,
	})

	// Step 4: Write translated DOCX plain.
	translatedPath := filepath.Join(subDir, "translated.docx")
	translatedText := strings.Join(translated, "\n\n")
	if err := writePlainDocx(translatedText, translatedPath); err != nil {
		fail(fmt.Sprintf("không tạo được file DOCX đã dịch: %v", err))
		return
	}

	usedTokens := totalTokens
	if usedTokens == 0 {
		usedTokens = estimateTokens(translatedText)
	}

	if err := c.reg.Message().UpdateTranslated(ctx, p.AssistantID, text, usedTokens); err != nil {
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

	c.log.Info("FileTranslateDone",
		"sessionId", p.SessionID, "fileId", p.FileID, "fileName", filepath.Base(p.FilePath),
		"durationMs", time.Since(p.StartTime).Milliseconds(), "tokens", usedTokens,
		"model", p.ModelUsed, "style", p.Style)

	runtime.EventsEmit(ctx, "translation:done", *msg)
	runtime.EventsEmit(ctx, "file:done", bridge.FileResult{
		FileID:    p.FileID,
		FileName:  filepath.Base(p.FilePath),
		FileType:  "pdf",
		CharCount: charCount,
		PageCount: pageCount,
	})
}

// chunkPlainText splits plain text into chunks of ~maxChars, respecting paragraph boundaries.
func chunkPlainText(text string, maxChars int) []string {
	paragraphs := strings.Split(text, "\n\n")
	var chunks []string
	var buf strings.Builder

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		// If single paragraph exceeds maxChars, flush it as its own chunk.
		if utf8.RuneCountInString(para) > maxChars {
			if buf.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(buf.String()))
				buf.Reset()
			}
			chunks = append(chunks, para)
			continue
		}
		// Flush buffer if adding this paragraph would exceed limit.
		if buf.Len() > 0 && utf8.RuneCountInString(buf.String())+utf8.RuneCountInString(para) > maxChars {
			chunks = append(chunks, strings.TrimSpace(buf.String()))
			buf.Reset()
		}
		if buf.Len() > 0 {
			buf.WriteString("\n\n")
		}
		buf.WriteString(para)
	}
	if buf.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(buf.String()))
	}
	return chunks
}

// translatePlainChunks translates text chunks concurrently using the batch stream API.
func (c *controller) translatePlainChunks(
	ctx context.Context,
	chunks []string,
	srcLang, targetLang string,
	style model.TranslationStyle,
	provider gateway.AIProvider,
	onProgress func(completed, total int),
) ([]string, int, error) {
	total := len(chunks)
	results := make([]string, total)
	var totalTokens int

	concurrency := provider.MaxBatchConcurrency()
	if concurrency < 1 {
		concurrency = 1
	}

	sem := make(chan struct{}, concurrency)
	type result struct {
		idx    int
		text   string
		tokens int
		err    error
	}
	resCh := make(chan result, total)

	for i, chunk := range chunks {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case sem <- struct{}{}:
		}
		go func(idx int, text string) {
			defer func() { <-sem }()
			events := make(chan gateway.StreamEvent, 64)
			var out strings.Builder
			var provErr error
			go func() {
				provErr = provider.TranslateBatchStream(ctx, text, srcLang, targetLang, string(style), events)
			}()
			for ev := range events {
				if ev.Type == "chunk" && ev.Content != "" {
					out.WriteString(ev.Content)
				}
			}
			if provErr != nil {
				resCh <- result{idx: idx, err: provErr}
				return
			}
			translated := strings.TrimSpace(out.String())
			resCh <- result{idx: idx, text: translated, tokens: estimateTokens(translated)}
		}(i, chunk)
	}

	completed := 0
	for range chunks {
		r := <-resCh
		if r.err != nil {
			return nil, 0, r.err
		}
		results[r.idx] = r.text
		totalTokens += r.tokens
		completed++
		onProgress(completed, total)
	}

	return results, totalTokens, nil
}
