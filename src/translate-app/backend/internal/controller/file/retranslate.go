package file

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"translate-app/internal/bridge"
	"translate-app/internal/gateway"
	"translate-app/internal/model"
)

// RetranslateContentParams — params for re-running translation on an already-translated file.
// For DOCX files, FileID must be set so the pipeline can load original_path from DB.
// For PDF / plain-text files, SourceMD carries the extracted text (legacy path).
type RetranslateContentParams struct {
	SessionID   string
	AssistantID string
	FileID      string // set for DOCX retranslate; empty for plain-text retranslate
	SourceMD    string // used only when FileID is empty (PDF / legacy)
	TargetLang  string
	Style       model.TranslationStyle
	Provider    gateway.AIProvider
}

// RunRetranslateContent re-runs translation for a file message.
// DOCX files use the XML-level pipeline (preserves structure).
// PDF / plain-text files use the existing chunked-markdown pipeline.
func (c *controller) RunRetranslateContent(ctx context.Context, p RetranslateContentParams) {
	fail := func(msg string) {
		runtime.EventsEmit(ctx, "translation:error", msg)
		runtime.EventsEmit(ctx, "file:error", msg)
	}

	if p.FileID != "" {
		c.retranslateDocx(ctx, p, fail)
	} else {
		c.retranslatePlain(ctx, p, fail)
	}
}

// retranslateDocx re-runs the XML pipeline on the original DOCX file.
func (c *controller) retranslateDocx(ctx context.Context, p RetranslateContentParams, fail func(string)) {
	fileRec, err := c.reg.File().GetByID(ctx, p.FileID)
	if err != nil || fileRec == nil {
		fail("không tìm thấy thông tin tệp để dịch lại")
		return
	}
	if fileRec.OriginalPath == "" {
		fail("không có đường dẫn tệp gốc để dịch lại")
		return
	}

	df, err := ParseDocx(fileRec.OriginalPath)
	if err != nil {
		fail(fmt.Sprintf("không đọc được cấu trúc DOCX: %v", err))
		return
	}
	if len(df.Paragraphs) == 0 {
		fail("DOCX không có nội dung văn bản")
		return
	}

	srcHint := gateway.SourceLangForTranslate(p.SourceMD)
	totalBatches := len(chunkDocxParagraphs(df.Paragraphs, charsPerChunk))

	translations, err := c.translateDocxFile(ctx, df, srcHint, p.TargetLang, p.Style, p.Provider,
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

	// Overwrite translated.docx in same directory.
	subDir := filepath.Dir(fileRec.SourcePath)
	if subDir == "" || subDir == "." {
		fail("không xác định được thư mục lưu file")
		return
	}
	translatedPath := filepath.Join(subDir, "translated.docx")
	if err := WriteTranslatedDocx(df, translations, translatedPath); err != nil {
		fail(fmt.Sprintf("không tạo được file DOCX đã dịch: %v", err))
		return
	}

	if err := c.reg.File().UpdateTranslated(ctx, p.FileID,
		fileRec.SourcePath, translatedPath,
		fileRec.CharCount, fileRec.PageCount, fileRec.ModelUsed,
	); err != nil {
		fail(err.Error())
		return
	}

	msg, err := c.reg.Message().GetByID(ctx, p.AssistantID)
	if err != nil || msg == nil {
		fail("không tải được tin nhắn sau khi dịch lại")
		return
	}
	runtime.EventsEmit(ctx, "translation:done", *msg)

	runtime.EventsEmit(ctx, "file:done", bridge.FileResult{
		FileID:    p.FileID,
		FileName:  filepath.Base(fileRec.OriginalPath),
		FileType:  "docx",
		CharCount: fileRec.CharCount,
		PageCount: fileRec.PageCount,
	})
}

// retranslatePlain re-runs the chunked-markdown pipeline for PDF / legacy files.
func (c *controller) retranslatePlain(ctx context.Context, p RetranslateContentParams, fail func(string)) {
	chunks := chunkMarkdownByParagraphs(p.SourceMD, charsPerChunk)
	if len(chunks) == 0 {
		fail("nội dung rỗng")
		return
	}

	total := len(chunks)
	var cumulative strings.Builder
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

		translated, err := c.streamTranslate(ctx, p.Provider, chunk, docSrcHint, p.TargetLang, p.Style, true, func(delta string) {
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
