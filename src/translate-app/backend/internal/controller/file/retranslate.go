package file

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"translate-app/internal/bridge"
	"translate-app/internal/gateway"
	"translate-app/internal/model"
)

// RetranslateContentParams — params for re-running translation on an already-translated DOCX file.
type RetranslateContentParams struct {
	SessionID   string
	AssistantID string
	FileID      string
	SourceMD    string
	TargetLang  string
	Style       model.TranslationStyle
	Provider    gateway.AIProvider
}

// RunRetranslateContent re-runs translation for a DOCX file message.
func (c *controller) RunRetranslateContent(ctx context.Context, p RetranslateContentParams) {
	fail := func(msg string) {
		runtime.EventsEmit(ctx, "translation:error", msg)
		runtime.EventsEmit(ctx, "file:error", msg)
	}

	c.retranslateDocx(ctx, p, fail)
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

