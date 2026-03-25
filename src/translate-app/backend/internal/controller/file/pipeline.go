package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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

	c.runDocxTranslate(ctx, p, fail)
}

// runDocxTranslate handles DOCX files using the XML-level translation pipeline.
// Structure (tables, images, columns) is preserved; only <w:t> text nodes are translated.
func (c *controller) runDocxTranslate(ctx context.Context, p fileTranslateParams, fail func(string)) {
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
	translations, err := c.translateDocxFile(ctx, df, docSrcHint, p.TargetLang, p.Style, p.Provider,
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
