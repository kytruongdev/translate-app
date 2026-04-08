package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"translate-app/internal/bridge"
	"translate-app/internal/gateway"
	"translate-app/internal/model"
)

const pdfStructuredChunkSize = 2500 // max runes per translation batch

// runStructuredPDFTranslate is the single pipeline for all PDF files.
// It replaces the old Tesseract-based runPDFTranslate entirely.
//
// Pipeline:
//  1. Render all pages to PNG via go-fitz (200 DPI)
//  2. Run OCR sidecar → StructuredOCRResult (text/title/table/figure regions)
//  3. Crop all figure regions to Base64 PNG
//  4. Delete temp PNG directory
//  5. Detect source language from OCR text
//  6. Collect translatable segments, batch & translate concurrently
//  7. Assemble final HTML with translated text + embedded figure images
//  8. Write source.md + translated.html to disk, update DB, emit events
func (c *controller) runStructuredPDFTranslate(ctx context.Context, p fileTranslateParams, fail func(string)) {
	// ── 1. Render PDF pages to PNGs (bỏ qua khi dùng Mistral) ──────────────
	usingMistral := c.keys.MistralKey != ""

	var imagePaths []string
	var tempDir string

	if !usingMistral {
		c.log.Info("PDFRenderStart", "fileId", p.FileID, "fileName", filepath.Base(p.FilePath))
		var err error
		imagePaths, tempDir, err = renderPDFToImages(ctx, p.FilePath)
		if err != nil {
			fail(fmt.Sprintf("không render được PDF: %v", err))
			return
		}
		c.log.Info("PDFRenderDone", "fileId", p.FileID, "pages", len(imagePaths))
	}

	// ── 2. Run OCR ───────────────────────────────────────────────────────────
	pageHint := len(imagePaths)
	if usingMistral {
		pageHint = p.PageCount // ước tính từ ReadFileInfo
	}
	c.log.Info("OCRStart", "fileId", p.FileID, "engine", map[bool]string{true: "mistral", false: "gpt/sidecar"}[usingMistral])

	runtime.EventsEmit(ctx, "file:progress", bridge.FileProgress{
		Chunk: 0, Total: pageHint, Percent: 0,
	})

	ocrResult, err := runStructuredOCR(ctx, imagePaths, p.FilePath, c.keys.MistralKey, c.keys.OpenAIKey, func(done, total int) {
		pct := 0
		if total > 0 {
			pct = (done * 50) / total
		}
		runtime.EventsEmit(ctx, "file:progress", bridge.FileProgress{
			Chunk: done, Total: total, Percent: pct,
		})
	})
	if err != nil {
		_ = os.RemoveAll(tempDir)
		fail(fmt.Sprintf("OCR thất bại: %v", err))
		return
	}
	totalRegions := 0
	for _, pg := range ocrResult.Pages {
		totalRegions += len(pg.Regions)
	}
	c.log.Info("OCRDone", "fileId", p.FileID, "pages", len(ocrResult.Pages), "totalRegions", totalRegions)

	// ── 3. Crop figures to Base64 ────────────────────────────────────────────
	figureCrops := extractFigureCrops(ocrResult, imagePaths)

	// ── 4. Delete temp PNGs (before any heavy translation work) ─────────────
	_ = os.RemoveAll(tempDir)

	// ── 5. Prepare storage directory ────────────────────────────────────────
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

	// ── 6. Build source text (for DB preview) ───────────────────────────────
	sourceMD := extractSourceTextFromOCR(ocrResult)
	sourcePath := filepath.Join(subDir, "source.md")
	if err := os.WriteFile(sourcePath, []byte(sourceMD), 0o644); err != nil {
		fail(fmt.Sprintf("không ghi được source.md: %v", err))
		return
	}

	charCount := utf8.RuneCountInString(sourceMD)
	pageCount := len(ocrResult.Pages)
	if pageCount == 0 {
		pageCount = p.PageCount
	}

	if err := c.reg.File().UpdateExtracted(ctx, p.FileID, sourcePath, charCount, pageCount); err != nil {
		fail(err.Error())
		return
	}

	// Update assistant message so source fullscreen can show OCR text preview.
	if err := c.reg.Message().UpdateOriginalContent(ctx, p.AssistantID, sourceMD); err != nil {
		fail(err.Error())
		return
	}

	// ── 7. Detect source language ────────────────────────────────────────────
	srcLang := gateway.SourceLangForTranslate(sourceMD)
	if srcLang != "auto" {
		_ = c.reg.Message().UpdateSourceLang(ctx, p.UserID, srcLang)
		_ = c.reg.Message().UpdateSourceLang(ctx, p.AssistantID, srcLang)
	}

	runtime.EventsEmit(ctx, "file:source", map[string]string{
		"sessionId":          p.SessionID,
		"assistantMessageId": p.AssistantID,
	})

	// ── 8. Collect translatable segments ────────────────────────────────────
	segments := collectSegments(ocrResult)
	total := len(segments)
	c.log.Info("SegmentsCollected", "fileId", p.FileID, "segments", total)

	// Emit 50 % to indicate OCR is done and translation is starting.
	runtime.EventsEmit(ctx, "file:progress", bridge.FileProgress{
		Chunk:   0,
		Total:   total,
		Percent: 50,
	})

	// ── 9. Translate segments concurrently ───────────────────────────────────
	c.log.Info("TranslationStart", "fileId", p.FileID, "segments", total, "targetLang", p.TargetLang)
	translated, totalTokens, err := c.translatePDFSegments(ctx, segments, srcLang, p.TargetLang, p.Style, p.Provider,
		func(completed int) {
			pct := 50 // translation phase = 50–100 %
			if total > 0 {
				pct = 50 + (completed*50)/total
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

	// ── 10. Assemble HTML ────────────────────────────────────────────────────
	c.log.Info("AssembleHTMLStart", "fileId", p.FileID, "translatedKeys", len(translated))
	htmlContent, err := assembleStructuredHTML(ocrResult, translated, figureCrops)
	if err != nil {
		fail(fmt.Sprintf("lỗi tạo HTML: %v", err))
		return
	}
	c.log.Info("AssembleHTMLDone", "fileId", p.FileID, "htmlBytes", len(htmlContent))

	translatedPath := filepath.Join(subDir, "translated.html")
	if err := os.WriteFile(translatedPath, []byte(htmlContent), 0o644); err != nil {
		fail(fmt.Sprintf("không ghi được translated.html: %v", err))
		return
	}

	// ── 11. Persist + emit ───────────────────────────────────────────────────
	usedTokens := totalTokens
	if usedTokens == 0 {
		usedTokens = estimateTokens(sourceMD)
	}

	if err := c.reg.Message().UpdateTranslated(ctx, p.AssistantID, sourceMD, usedTokens); err != nil {
		fail(err.Error())
		return
	}

	if err := c.reg.File().UpdateTranslated(ctx, p.FileID, sourcePath, translatedPath, charCount, pageCount, p.ModelUsed, "html"); err != nil {
		fail(err.Error())
		return
	}

	msg, err := c.reg.Message().GetByID(ctx, p.AssistantID)
	if err != nil || msg == nil {
		fail("không tải được tin nhắn sau khi dịch file PDF")
		return
	}
	runtime.EventsEmit(ctx, "translation:done", *msg)

	c.log.Info("FileTranslateDone",
		"sessionId", p.SessionID, "fileId", p.FileID, "fileName", filepath.Base(p.FilePath),
		"durationMs", time.Since(p.StartTime).Milliseconds(), "tokens", usedTokens,
		"model", p.ModelUsed, "style", p.Style)

	runtime.EventsEmit(ctx, "file:done", bridge.FileResult{
		FileID:       p.FileID,
		FileName:     filepath.Base(p.FilePath),
		FileType:     "pdf",
		OutputFormat: "html",
		CharCount:    charCount,
		PageCount:    pageCount,
		TokensUsed:   usedTokens,
	})
}

// pdfSegment is a single translatable unit extracted from the OCR result.
type pdfSegment struct {
	key  string // regionKey(pageNo, regionIdx)
	text string // text to translate (plain text or HTML)
	// isHTML flags segments where the AI must preserve HTML structure.
	isHTML bool
}

// collectSegments extracts all translatable segments from the OCR result.
// Order matches the page/region order from the sidecar.
func collectSegments(result *StructuredOCRResult) []pdfSegment {
	var segs []pdfSegment
	for _, page := range result.Pages {
		for ri, region := range page.Regions {
			key := regionKey(page.PageNo, ri)
			switch region.Type {
			case "text", "title":
				if strings.TrimSpace(region.Content) != "" {
					// If content contains inline HTML tags (bold/italic/underline from GPT),
					// mark as HTML so the translation prompt preserves the tags.
					isHTML := strings.Contains(region.Content, "<strong>") ||
						strings.Contains(region.Content, "<em>") ||
						strings.Contains(region.Content, "<u>")
					segs = append(segs, pdfSegment{key: key, text: region.Content, isHTML: isHTML})
				}
			case "table":
				if strings.TrimSpace(region.HTML) != "" {
					segs = append(segs, pdfSegment{key: key, text: region.HTML, isHTML: true})
				}
			case "figure":
				if region.FigureType == "informational" && len(region.TextLines) > 0 {
					segs = append(segs, pdfSegment{key: key, text: strings.Join(region.TextLines, " | ")})
				}
			}
		}
	}
	return segs
}

// extractSourceTextFromOCR builds a plain-text markdown preview from the OCR result
// for display and language detection. Tables and figures are summarized.
func extractSourceTextFromOCR(result *StructuredOCRResult) string {
	var sb strings.Builder
	for _, page := range result.Pages {
		for _, region := range page.Regions {
			switch region.Type {
			case "title":
				sb.WriteString("## ")
				sb.WriteString(region.Content)
				sb.WriteString("\n\n")
			case "text":
				if strings.TrimSpace(region.Content) != "" {
					sb.WriteString(region.Content)
					sb.WriteString("\n\n")
				}
			case "table":
				sb.WriteString("[Bảng]\n\n")
			case "figure":
				if region.FigureType == "informational" && len(region.TextLines) > 0 {
					sb.WriteString("[Hình: ")
					sb.WriteString(strings.Join(region.TextLines, " "))
					sb.WriteString("]\n\n")
				}
			}
		}
	}
	return strings.TrimSpace(sb.String())
}

// translatePDFSegments translates all segments concurrently, respecting the
// provider's MaxBatchConcurrency. Returns a map[key]→translatedText.
func (c *controller) translatePDFSegments(
	ctx context.Context,
	segments []pdfSegment,
	srcLang, tgtLang string,
	style model.TranslationStyle,
	provider gateway.AIProvider,
	onProgress func(completed int),
) (map[string]string, int, error) {
	results := make(map[string]string, len(segments))
	if len(segments) == 0 {
		return results, 0, nil
	}

	// Group consecutive segments into batches of up to pdfStructuredChunkSize runes.
	batches := batchPDFSegments(segments)

	maxConcurrent := provider.MaxBatchConcurrency()
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	sem := make(chan struct{}, maxConcurrent)

	var (
		mu          sync.Mutex
		firstErr    error
		totalTokens int32
		completed   int32
	)
	var wg sync.WaitGroup

	for _, batch := range batches {
		batch := batch // capture
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			for _, seg := range batch {
				translatedText, tokens, err := c.streamTranslate(
					ctx, provider, seg.text,
					srcLang, tgtLang, style,
					seg.isHTML, // preserveMarkdown = true for HTML segments
					nil,        // no streaming delta for batch PDF
				)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					return
				}
				// If the AI refused to translate (e.g. privacy/PII refusal),
				// fall back to the original source text so we don't embed
				// refusal strings in the output HTML.
				if isAIRefusal(translatedText) {
					translatedText = seg.text
				}
				atomic.AddInt32(&totalTokens, int32(tokens))

				mu.Lock()
				results[seg.key] = translatedText
				mu.Unlock()

				n := int(atomic.AddInt32(&completed, 1))
				onProgress(n)
			}
		}()
	}

	wg.Wait()

	if firstErr != nil {
		return nil, 0, firstErr
	}
	return results, int(atomic.LoadInt32(&totalTokens)), nil
}

// batchPDFSegments groups segments into batches where each batch's total
// rune count does not exceed pdfStructuredChunkSize.
// HTML segments (tables) are always in their own batch (to avoid mixing
// HTML with plain text in a single AI call).
func batchPDFSegments(segments []pdfSegment) [][]pdfSegment {
	var batches [][]pdfSegment
	var current []pdfSegment
	var currentRunes int

	for _, seg := range segments {
		segRunes := utf8.RuneCountInString(seg.text)

		// HTML segments get their own batch.
		if seg.isHTML {
			if len(current) > 0 {
				batches = append(batches, current)
				current = nil
				currentRunes = 0
			}
			batches = append(batches, []pdfSegment{seg})
			continue
		}

		// If adding this segment would exceed the limit, flush current batch.
		if currentRunes+segRunes > pdfStructuredChunkSize && len(current) > 0 {
			batches = append(batches, current)
			current = nil
			currentRunes = 0
		}

		current = append(current, seg)
		currentRunes += segRunes
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

// isAIRefusal detects when the AI returned a refusal instead of a translation
// (e.g. safety filters triggered on PII-heavy legal documents).
// When detected, the caller should fall back to the original source text.
var aiRefusalPrefixes = []string{
	"i'm sorry, but i cannot",
	"i'm sorry, i cannot",
	"i'm unable to",
	"i cannot assist",
	"i can't help with that",
	"sorry, but i cannot",
	"i cannot provide",
	"i'm not able to",
	"as an ai",
}

func isAIRefusal(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	for _, prefix := range aiRefusalPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}
