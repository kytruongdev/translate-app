package file

import (
	"context"
	"encoding/json"
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
	"translate-app/internal/repository"
)

const pdfStructuredChunkSize = 5000 // max runes per translation batch
const pdfMaxSegmentsPerBatch = 30   // max segments per text batch (prevents marker skip)

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
	// Always clear the glossary file tag when this pipeline exits — success or failure.
	// Uses Background context because ctx may already be cancelled on the error path.
	defer func() {
		_ = c.reg.Glossary().ClearFileGlossary(context.Background(), filepath.Base(p.FilePath))
	}()

	// ── 1. Run Mistral OCR (không cần render PNG) ────────────────────────────
	ocrStart := time.Now()
	c.log.Info(strings.Repeat("-", 80))
	c.log.Info("SessionStart", "fileId", p.FileID, "file", filepath.Base(p.FilePath))
	c.log.Info("OCRStart", "fileId", p.FileID, "engine", "mistral")

	runtime.EventsEmit(ctx, "file:progress", bridge.FileProgress{
		Chunk: 0, Total: 0, Percent: 0, Phase: "ocr",
	})

	ocrResult, rawMarkdown, err := runMistralOCR(ctx, p.FilePath, c.keys.MistralKey, c.log, func(done, total int) {
		runtime.EventsEmit(ctx, "file:progress", bridge.FileProgress{
			Chunk: done, Total: total, Percent: 0, Phase: "ocr",
		})
	})
	if err != nil {
		fail(fmt.Sprintf("Mistral OCR thất bại: %v", err))
		return
	}
	totalRegions := 0
	for _, pg := range ocrResult.Pages {
		totalRegions += len(pg.Regions)
	}
	c.log.Info("OCRDone",
		"fileId", p.FileID,
		"pages", len(ocrResult.Pages),
		"totalRegions", totalRegions,
		"durationMs", time.Since(ocrStart).Milliseconds(),
	)

	// ── 2. Prepare storage directory ────────────────────────────────────────
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
		Chunk: 0, Total: total, Percent: 0, Phase: "glossary",
	})

	// ── 8b. Context extraction (Call 1) + Glossary extraction (Call 2) ─────────
	// Call 1: send first 3 pages to extract doc_type + translation context summary.
	// Call 2: send full markdown + context to extract glossary terms.
	// Both calls are sequential (Call 2 depends on Call 1 output).
	glossary := ""
	activeRules := ""
	detectedDocType := ""
	docContext := ""
	var glossaryTokens int
	{
		extractStart := time.Now()
		c.log.Info("ContextExtractionStart", "fileId", p.FileID, "pages", pageCount)

		docTypeIDs, dtErr := c.reg.Glossary().ListDocTypeIDs(ctx)
		if dtErr != nil {
			c.log.Info("ContextExtractionWarn", "fileId", p.FileID, "step", "ListDocTypeIDs", "error", dtErr.Error())
		} else {
			docTypesList := strings.Join(docTypeIDs, ", ")

			// Call 1 — context extraction from first 3 pages.
			firstPagesMarkdown := extractFirstPagesMarkdown(ocrResult, 3)
			contextSystem := gateway.BuildContextExtractionPrompt(docTypesList)
			c.log.Info("PromptContextExtraction", "fileId", p.FileID, "system", contextSystem)
			contextJSON, contextTokens, contextErr := c.streamTranslateWithSystem(ctx, p.Provider, contextSystem, firstPagesMarkdown, nil)
			if contextErr != nil {
				c.log.Info("ContextExtractionWarn", "fileId", p.FileID, "step", "call1", "error", contextErr.Error())
			} else {
				glossaryTokens += contextTokens
				detectedDocType, docContext = parseContextResult(contextJSON)
				c.log.Info("ContextExtractionDone",
					"fileId", p.FileID,
					"docType", detectedDocType,
					"tokens", contextTokens,
					"durationMs", time.Since(extractStart).Milliseconds(),
				)
				if detectedDocType != "" {
					if err := c.reg.Glossary().EnsureDocType(ctx, detectedDocType, ""); err != nil {
						c.log.Info("ContextDocTypeWarn", "fileId", p.FileID, "docType", detectedDocType, "error", err.Error())
					}
				}
			}

			// Call 2 — glossary extraction from full markdown, informed by context.
			glossarySystem := gateway.BuildGlossaryExtractionPrompt(docContext)
			c.log.Info("PromptGlossaryExtraction", "fileId", p.FileID, "system", glossarySystem)
			rawJSON, extractTokens, extractErr := c.streamTranslateWithSystem(ctx, p.Provider, glossarySystem, rawMarkdown, nil)
			if extractErr != nil {
				c.log.Info("GlossaryExtractionWarn", "fileId", p.FileID, "error", extractErr.Error())
			} else {
				glossaryTokens += extractTokens
				c.log.Info("GlossaryExtractionDone",
					"fileId", p.FileID,
					"tokens", extractTokens,
					"durationMs", time.Since(extractStart).Milliseconds(),
				)
				glossary = c.processGlossaryExtractionResult(ctx, rawJSON, srcLang, p.TargetLang, filepath.Base(p.FilePath), detectedDocType)
				termCount := 0
				if glossary != "" {
					termCount = strings.Count(glossary, "\n") + 1
				}
				c.log.Info("GlossaryReady", "fileId", p.FileID, "terms", termCount, "docType", detectedDocType)
			}
		}
	}

	// Load active translation rules (global + doc-type-specific).
	if r, err := c.reg.Glossary().LoadActiveRules(ctx, detectedDocType, p.TargetLang); err != nil {
		c.log.Info("LoadActiveRulesWarn", "fileId", p.FileID, "error", err.Error())
	} else {
		activeRules = r
	}

	// ── 9. Translate segments (batch text, individual HTML) ──────────────────
	translationStart := time.Now()
	glossaryTermCount := 0
	if glossary != "" {
		glossaryTermCount = strings.Count(glossary, "\n") + 1
	}
	c.log.Info("TranslationStart",
		"fileId", p.FileID,
		"segments", total,
		"targetLang", p.TargetLang,
		"glossaryTerms", glossaryTermCount,
	)
	// Signal start of translation phase (real progress begins here).
	runtime.EventsEmit(ctx, "file:progress", bridge.FileProgress{
		Chunk: 0, Total: total, Percent: 0, Phase: "translating",
	})
	translated, totalTokens, err := c.translatePDFSegments(ctx, p.FileID, segments, srcLang, p.TargetLang, p.Provider, glossary, docContext, activeRules,
		func(completed int) {
			pct := 0
			if total > 0 {
				pct = (completed * 100) / total
			}
			runtime.EventsEmit(ctx, "file:progress", bridge.FileProgress{
				Chunk: completed, Total: total, Percent: pct, Phase: "translating",
			})
		},
	)
	if err != nil {
		fail(err.Error())
		return
	}

	c.log.Info("TranslationDone",
		"fileId", p.FileID,
		"segments", total,
		"totalTokens", totalTokens,
		"durationMs", time.Since(translationStart).Milliseconds(),
	)

	runtime.EventsEmit(ctx, "file:progress", bridge.FileProgress{
		Chunk: total, Total: total, Percent: 100, Phase: "translating",
	})

	// ── 10. Assemble HTML ────────────────────────────────────────────────────
	assembleStart := time.Now()
	htmlContent, err := assembleStructuredHTML(ocrResult, translated)
	if err != nil {
		fail(fmt.Sprintf("lỗi tạo HTML: %v", err))
		return
	}
	c.log.Info("AssembleHTMLDone",
		"fileId", p.FileID,
		"htmlBytes", len(htmlContent),
		"durationMs", time.Since(assembleStart).Milliseconds(),
	)

	translatedPath := filepath.Join(subDir, "translated.html")
	if err := os.WriteFile(translatedPath, []byte(htmlContent), 0o644); err != nil {
		fail(fmt.Sprintf("không ghi được translated.html: %v", err))
		return
	}

	// ── 11. Persist + emit ───────────────────────────────────────────────────
	usedTokens := totalTokens + glossaryTokens
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


// buildPDFBatchInput formats a slice of text segments as a <<<N>>> numbered list
// for a single batched AI translation call.
func buildPDFBatchInput(segments []pdfSegment) string {
	const instruction = "Translate each segment below, preserving the <<<N>>> markers:\n\n"
	var sb strings.Builder
	sb.WriteString(instruction)
	for i, seg := range segments {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		fmt.Fprintf(&sb, "<<<%d>>>\n%s", i+1, seg.text)
	}
	return sb.String()
}

// translatePDFSegments translates all segments concurrently.
//
// Text batches (isHTML=false) are sent as <<<N>>>-marked calls.
// HTML segments (tables) are sent individually with HTML-specific instructions.
// All batches receive the same glossary for cross-batch terminology consistency.
func (c *controller) translatePDFSegments(
	ctx context.Context,
	fileID string,
	segments []pdfSegment,
	srcLang, tgtLang string,
	provider gateway.AIProvider,
	glossary, docContext, rules string,
	onProgress func(completed int),
) (map[string]string, int, error) {
	results := make(map[string]string, len(segments))
	if len(segments) == 0 {
		return results, 0, nil
	}

	batches := batchPDFSegments(segments)
	totalBatches := len(batches)

	var (
		mu              sync.Mutex
		totalTokens     int64
		completed       int64
		firstErr        error
		errOnce         sync.Once
		promptLogOnce   sync.Once
	)

	sem := make(chan struct{}, provider.MaxBatchConcurrency())
	var wg sync.WaitGroup

	for bi, batch := range batches {
		if ctx.Err() != nil {
			break
		}

		batchIdx := bi
		batchCopy := batch

		wg.Add(1)
		sem <- struct{}{}

		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			batchType := "text"
			if len(batchCopy) == 1 && batchCopy[0].isHTML {
				batchType = "html"
			}

			batchStart := time.Now()
			c.log.Info("PDFBatchStart",
				"fileId", fileID,
				"batch", fmt.Sprintf("%d/%d", batchIdx+1, totalBatches),
				"type", batchType,
				"segments", len(batchCopy),
			)

			var batchResults map[string]string
			var tokens int
			var err error

			if batchType == "html" {
				system := gateway.BuildPDFHTMLSystemPromptGPT(tgtLang, glossary, docContext, rules)
				promptLogOnce.Do(func() {
					c.log.Info("PromptBatchTranslation", "fileId", fileID, "type", "html", "system", system)
				})
				translatedText, tok, e := c.streamTranslateWithSystem(ctx, provider, system, batchCopy[0].text, nil)
				tokens = tok
				if e != nil {
					err = e
				} else {
					if isAIRefusal(translatedText) {
						translatedText = batchCopy[0].text
					}
					batchResults = map[string]string{batchCopy[0].key: translatedText}
				}
			} else {
				system := gateway.BuildPDFBatchSystemPromptGPT(tgtLang, glossary, docContext, rules)
				promptLogOnce.Do(func() {
					c.log.Info("PromptBatchTranslation", "fileId", fileID, "type", "text", "system", system)
				})
				input := buildPDFBatchInput(batchCopy)
				raw, tok, e := c.streamTranslateWithSystem(ctx, provider, system, input, nil)
				tokens = tok
				if e != nil {
					err = e
				} else {
					parsed := parseBatchOutput(raw, len(batchCopy))
					batchResults = make(map[string]string, len(batchCopy))
					var skipped []pdfSegment
					for i, seg := range batchCopy {
						text := ""
						if i < len(parsed) {
							text = parsed[i]
						}
						if text == "" || isAIRefusal(text) {
							skipped = append(skipped, seg)
						} else {
							batchResults[seg.key] = text
						}
					}
					// Retry each skipped segment individually.
					retryTokens := 0
					for _, seg := range skipped {
						retrySys := gateway.BuildPDFBatchSystemPromptGPT(tgtLang, glossary, docContext, rules)
						retryRaw, rTok, rErr := c.streamTranslateWithSystem(ctx, provider, retrySys, seg.text, nil)
						retryTokens += rTok
						if rErr != nil || strings.TrimSpace(retryRaw) == "" || isAIRefusal(retryRaw) {
							batchResults[seg.key] = seg.text // last resort: keep original
						} else {
							batchResults[seg.key] = strings.TrimSpace(retryRaw)
						}
					}
					tokens += retryTokens
					if len(skipped) > 0 {
						c.log.Info("PDFBatchRetry",
							"fileId", fileID,
							"batch", fmt.Sprintf("%d/%d", batchIdx+1, totalBatches),
							"skipped", len(skipped),
						)
					}
				}
			}

			if err != nil {
				errOnce.Do(func() { firstErr = err })
				return
			}

			mu.Lock()
			for k, v := range batchResults {
				results[k] = v
			}
			mu.Unlock()

			atomic.AddInt64(&totalTokens, int64(tokens))
			done := int(atomic.AddInt64(&completed, int64(len(batchCopy))))
			onProgress(done)

			c.log.Info("PDFBatchDone",
				"fileId", fileID,
				"batch", fmt.Sprintf("%d/%d", batchIdx+1, totalBatches),
				"type", batchType,
				"tokens", tokens,
				"durationMs", time.Since(batchStart).Milliseconds(),
			)
		}()
	}

	wg.Wait()

	if firstErr != nil {
		return nil, 0, firstErr
	}
	return results, int(totalTokens), nil
}

// processGlossaryExtractionResult parses the raw JSON from the GPT glossary extraction call,
// saves the extracted terms to the DB (tagged to fileName), and returns the formatted glossary
// string ready for injection into translation prompts.
// docType is the already-detected document type from the context extraction call.
// On any error it logs a warning and returns "" so translation proceeds without a glossary.
func (c *controller) processGlossaryExtractionResult(ctx context.Context, rawJSON, srcLang, tgtLang, fileName, docType string) string {
	clean := strings.TrimSpace(rawJSON)
	if idx := strings.Index(clean, "{"); idx > 0 {
		clean = clean[idx:]
	}
	if idx := strings.LastIndex(clean, "}"); idx >= 0 && idx < len(clean)-1 {
		clean = clean[:idx+1]
	}

	var result gateway.GlossaryExtractionResult
	if err := json.Unmarshal([]byte(clean), &result); err != nil {
		c.log.Info("GlossaryParseWarn", "fileName", fileName, "error", err.Error())
		return ""
	}

	terms := make([]repository.GlossaryTerm, 0, len(result.Glossary))
	for _, g := range result.Glossary {
		if len(g.Sources) == 0 || strings.TrimSpace(g.Target) == "" {
			continue
		}
		terms = append(terms, repository.GlossaryTerm{Sources: g.Sources, Target: g.Target})
	}

	if len(terms) == 0 {
		return ""
	}

	if err := c.reg.Glossary().SaveExtractedGlossary(ctx, srcLang, tgtLang, docType, fileName, terms); err != nil {
		c.log.Info("GlossarySaveWarn", "fileName", fileName, "error", err.Error())
		return ""
	}

	loaded, err := c.reg.Glossary().LoadGlossaryForFile(ctx, fileName)
	if err != nil {
		c.log.Info("GlossaryLoadWarn", "fileName", fileName, "error", err.Error())
		return ""
	}
	return loaded
}

// parseContextResult parses the JSON response from BuildContextExtractionPrompt.
// Returns (docType, summary). On parse error returns ("", "").
func parseContextResult(rawJSON string) (docType, summary string) {
	clean := strings.TrimSpace(rawJSON)
	if idx := strings.Index(clean, "{"); idx > 0 {
		clean = clean[idx:]
	}
	if idx := strings.LastIndex(clean, "}"); idx >= 0 && idx < len(clean)-1 {
		clean = clean[:idx+1]
	}
	var result gateway.ContextExtractionResult
	if err := json.Unmarshal([]byte(clean), &result); err != nil {
		return "", ""
	}
	return result.DocType, result.Summary
}

// extractFirstPagesMarkdown returns the raw OCR markdown for the first n pages,
// used to extract document context without sending the full document to the API.
func extractFirstPagesMarkdown(result *StructuredOCRResult, n int) string {
	var sb strings.Builder
	for i, page := range result.Pages {
		if i >= n {
			break
		}
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
			}
		}
	}
	return strings.TrimSpace(sb.String())
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

		// Flush if rune limit OR segment count limit would be exceeded.
		if len(current) > 0 && (currentRunes+segRunes > pdfStructuredChunkSize || len(current) >= pdfMaxSegmentsPerBatch) {
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

