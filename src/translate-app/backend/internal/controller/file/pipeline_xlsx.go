package file

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"translate-app/internal/bridge"
	"translate-app/internal/gateway"
	"translate-app/internal/model"
)

// xlsxEntry represents one <si> element from sharedStrings.xml.
type xlsxEntry struct {
	text         string // plain text (all runs concatenated)
	firstRPr     string // inner XML of first <rPr> element; empty = plain text cell
	translatable bool   // true if text contains at least one letter
}

// xlsxItem is a translatable entry with its position in the entries slice.
type xlsxItem struct {
	entryIdx int
	text     string
}

var (
	reXlsxSI          = regexp.MustCompile(`(?s)<si>(.*?)</si>`)
	reXlsxT            = regexp.MustCompile(`(?s)<t(?:[^>]*)>(.*?)</t>`)
	reXlsxRPr          = regexp.MustCompile(`(?s)<rPr>(.*?)</rPr>`)
	reWorkbookSheetAttr = regexp.MustCompile(`(<sheet\b[^>]*?\bname=")([^"]*)(")`)
)

// parseXlsxSharedStrings reads xl/sharedStrings.xml from the xlsx ZIP.
// Returns parsed entries and the raw XML (used for reconstruction), or nil/empty if no shared strings exist.
func parseXlsxSharedStrings(xlsxPath string) ([]xlsxEntry, string, error) {
	zr, err := zip.OpenReader(xlsxPath)
	if err != nil {
		return nil, "", fmt.Errorf("không mở được xlsx: %w", err)
	}
	defer zr.Close()

	var ssFile *zip.File
	for _, f := range zr.File {
		if f.Name == "xl/sharedStrings.xml" {
			ssFile = f
			break
		}
	}
	if ssFile == nil {
		return nil, "", nil // No text cells
	}

	rc, err := ssFile.Open()
	if err != nil {
		return nil, "", err
	}
	defer rc.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(rc, 32<<20)); err != nil {
		return nil, "", err
	}
	rawXML := buf.String()

	matches := reXlsxSI.FindAllStringSubmatch(rawXML, -1)
	entries := make([]xlsxEntry, len(matches))
	for i, m := range matches {
		inner := m[1]

		// Concatenate text from all <t> elements (handles both plain and rich text cells).
		tMatches := reXlsxT.FindAllStringSubmatch(inner, -1)
		var sb strings.Builder
		for _, tm := range tMatches {
			sb.WriteString(xlsxXMLUnescape(tm[1]))
		}
		text := sb.String()

		// Preserve first <rPr> so we can restore at least the dominant character format.
		firstRPr := ""
		if rPrMatch := reXlsxRPr.FindStringSubmatch(inner); rPrMatch != nil {
			firstRPr = rPrMatch[1]
		}

		entries[i] = xlsxEntry{
			text:         text,
			firstRPr:     firstRPr,
			translatable: xlsxHasLetter(text),
		}
	}
	return entries, rawXML, nil
}

func xlsxHasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func xlsxXMLUnescape(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&apos;", "'")
	return s
}

func xlsxXMLEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// buildXlsxSI constructs a <si> XML element from a translated string.
// If the original cell had rich text (firstRPr non-empty), the first run's
// character format is applied to the entire translated string.
func buildXlsxSI(text, firstRPr string) string {
	esc := xlsxXMLEscape(text)
	if firstRPr != "" {
		return fmt.Sprintf(`<si><r><rPr>%s</rPr><t xml:space="preserve">%s</t></r></si>`, firstRPr, esc)
	}
	return fmt.Sprintf(`<si><t xml:space="preserve">%s</t></si>`, esc)
}

// buildNewSharedStrings reconstructs the full sharedStrings.xml with translated text.
func buildNewSharedStrings(originalXML string, entries []xlsxEntry, translations []string) string {
	locs := reXlsxSI.FindAllStringIndex(originalXML, -1)
	var sb strings.Builder
	prev := 0
	for i, loc := range locs {
		sb.WriteString(originalXML[prev:loc[0]])
		text := ""
		firstRPr := ""
		if i < len(entries) {
			text = entries[i].text
			firstRPr = entries[i].firstRPr
		}
		if i < len(translations) && translations[i] != "" {
			text = translations[i]
		}
		sb.WriteString(buildXlsxSI(text, firstRPr))
		prev = loc[1]
	}
	sb.WriteString(originalXML[prev:])
	return sb.String()
}

// writeXlsxTranslated creates a new xlsx (ZIP) with the sharedStrings.xml (and optionally
// workbook.xml) replaced. All other ZIP entries are copied verbatim so formatting is preserved.
func writeXlsxTranslated(srcPath, dstPath, newSharedStrings, newWorkbook string) error {
	zr, err := zip.OpenReader(srcPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer zw.Close()

	for _, f := range zr.File {
		var data []byte
		switch f.Name {
		case "xl/sharedStrings.xml":
			data = []byte(newSharedStrings)
		case "xl/workbook.xml":
			if newWorkbook != "" {
				data = []byte(newWorkbook)
			}
		}
		if data == nil {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, rc)
			rc.Close()
			data = buf.Bytes()
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	return nil
}

// xlsxSourceMarkdown produces a plain-text preview of the xlsx content for display.
func xlsxSourceMarkdown(entries []xlsxEntry, fileName string) string {
	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(fileName)
	sb.WriteString("\n\n")
	for _, e := range entries {
		if e.translatable && strings.TrimSpace(e.text) != "" {
			sb.WriteString(strings.TrimSpace(e.text))
			sb.WriteString("\n\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

// parseXlsxWorkbookSheetNames extracts sheet names from xl/workbook.xml.
// Returns names, raw XML (for reconstruction), and error. Empty names/XML means no workbook found.
func parseXlsxWorkbookSheetNames(xlsxPath string) ([]string, string, error) {
	zr, err := zip.OpenReader(xlsxPath)
	if err != nil {
		return nil, "", fmt.Errorf("không mở được xlsx: %w", err)
	}
	defer zr.Close()

	var wbFile *zip.File
	for _, f := range zr.File {
		if f.Name == "xl/workbook.xml" {
			wbFile = f
			break
		}
	}
	if wbFile == nil {
		return nil, "", nil
	}

	rc, err := wbFile.Open()
	if err != nil {
		return nil, "", err
	}
	defer rc.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(rc, 4<<20)); err != nil {
		return nil, "", err
	}
	rawXML := buf.String()

	matches := reWorkbookSheetAttr.FindAllStringSubmatch(rawXML, -1)
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, xlsxXMLUnescape(m[2]))
	}
	return names, rawXML, nil
}

// buildNewWorkbook reconstructs workbook.xml with translated sheet names.
func buildNewWorkbook(originalXML string, translatedNames []string) string {
	locs := reWorkbookSheetAttr.FindAllStringSubmatchIndex(originalXML, -1)
	if len(locs) == 0 {
		return originalXML
	}
	var sb strings.Builder
	prev := 0
	for i, loc := range locs {
		if i >= len(translatedNames) {
			break
		}
		// loc[4],loc[5] = group 2 = the name value between the quotes
		sb.WriteString(originalXML[prev:loc[4]])
		sb.WriteString(xlsxXMLEscape(translatedNames[i]))
		prev = loc[5]
	}
	sb.WriteString(originalXML[prev:])
	return sb.String()
}

// translateXlsxSheetNames translates sheet names in a single batch call.
// Returns original names on failure (non-fatal).
func (c *controller) translateXlsxSheetNames(
	ctx context.Context,
	names []string,
	srcLang, targetLang string,
	style model.TranslationStyle,
	provider gateway.AIProvider,
) []string {
	type item struct {
		idx  int
		name string
	}
	var toTranslate []item
	for i, n := range names {
		if xlsxHasLetter(n) {
			toTranslate = append(toTranslate, item{i, n})
		}
	}
	if len(toTranslate) == 0 {
		return names
	}

	var sb strings.Builder
	sb.WriteString("Translate each sheet name below, preserving the <<<N>>> markers:\n\n")
	for n, it := range toTranslate {
		if n > 0 {
			sb.WriteString("\n\n")
		}
		fmt.Fprintf(&sb, "<<<%d>>>\n%s", n+1, it.name)
	}

	translated, _, err := c.streamTranslateDocxBatch(ctx, provider, sb.String(), srcLang, targetLang, style)
	if err != nil {
		return names // non-fatal
	}

	parsed := parseBatchOutput(translated, len(toTranslate))
	result := make([]string, len(names))
	copy(result, names)
	for n, it := range toTranslate {
		if n < len(parsed) && parsed[n] != "" {
			result[it.idx] = parsed[n]
		}
	}
	return result
}

// buildXlsxItems extracts translatable entries with their original index.
func buildXlsxItems(entries []xlsxEntry) []xlsxItem {
	var items []xlsxItem
	for i, e := range entries {
		if e.translatable {
			items = append(items, xlsxItem{entryIdx: i, text: e.text})
		}
	}
	return items
}

// chunkXlsxItems groups translatable items into batches by character count.
func chunkXlsxItems(items []xlsxItem, maxChars int) [][]xlsxItem {
	var batches [][]xlsxItem
	var cur []xlsxItem
	curLen := 0
	for _, it := range items {
		n := utf8.RuneCountInString(it.text)
		if len(cur) > 0 && curLen+n > maxChars {
			batches = append(batches, cur)
			cur = nil
			curLen = 0
		}
		cur = append(cur, it)
		curLen += n
	}
	if len(cur) > 0 {
		batches = append(batches, cur)
	}
	return batches
}

// translateXlsxStrings translates all translatable entries concurrently using <<<N>>> batch format.
// Returns a translations slice aligned with entries (empty string = keep original).
func (c *controller) translateXlsxStrings(
	ctx context.Context,
	items []xlsxItem,
	totalEntries int,
	srcLang, targetLang string,
	style model.TranslationStyle,
	provider gateway.AIProvider,
	onProgress func(completed, total int),
) ([]string, int, error) {
	translations := make([]string, totalEntries)
	if len(items) == 0 {
		return translations, 0, nil
	}

	batches := chunkXlsxItems(items, charsPerChunk)
	total := len(batches)

	type batchResult struct {
		results map[int]string
		tokens  int
		err     error
	}

	concurrency := provider.MaxBatchConcurrency()
	if concurrency < 1 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	resCh := make(chan batchResult, total)
	var completedAtomic int64

	for _, batch := range batches {
		sem <- struct{}{}
		go func(b []xlsxItem) {
			defer func() { <-sem }()

			var sb strings.Builder
			sb.WriteString("Translate each cell text below, preserving the <<<N>>> markers:\n\n")
			for n, it := range b {
				if n > 0 {
					sb.WriteString("\n\n")
				}
				fmt.Fprintf(&sb, "<<<%d>>>\n%s", n+1, it.text)
			}

			translated, tokens, err := c.streamTranslateDocxBatch(ctx, provider, sb.String(), srcLang, targetLang, style)
			if err != nil {
				resCh <- batchResult{err: err}
				return
			}

			parsed := parseBatchOutput(translated, len(b))
			results := make(map[int]string, len(b))
			for n, it := range b {
				if n < len(parsed) && parsed[n] != "" {
					results[it.entryIdx] = parsed[n]
				}
			}

			cnt := int(atomic.AddInt64(&completedAtomic, 1))
			onProgress(cnt, total)

			resCh <- batchResult{results: results, tokens: tokens}
		}(batch)
	}

	var totalTokens int
	var mu sync.Mutex
	for range batches {
		r := <-resCh
		if r.err != nil {
			return nil, 0, r.err
		}
		mu.Lock()
		for idx, text := range r.results {
			translations[idx] = text
		}
		totalTokens += r.tokens
		mu.Unlock()
	}

	return translations, totalTokens, nil
}

// runXlsxTranslate handles .xlsx files:
// 1. Parse sharedStrings.xml — the global string pool shared by all sheets
// 2. Translate all unique text strings concurrently (<<<N>>> batch format)
// 3. Translate sheet names from workbook.xml
// 4. Write a new xlsx with sharedStrings.xml and workbook.xml replaced — all formatting preserved
func (c *controller) runXlsxTranslate(ctx context.Context, p fileTranslateParams, fail func(string)) {
	// Step 1: Parse shared strings.
	entries, rawSS, err := parseXlsxSharedStrings(p.FilePath)
	if err != nil {
		fail(err.Error())
		return
	}
	if len(entries) == 0 || rawSS == "" {
		fail("tệp Excel không có nội dung văn bản")
		return
	}

	// Collect translatable items and compute batch count for progress reporting.
	items := buildXlsxItems(entries)
	if len(items) == 0 {
		fail("tệp Excel không có văn bản cần dịch")
		return
	}
	batches := chunkXlsxItems(items, charsPerChunk)
	totalBatches := len(batches)

	// Step 2: Build source preview + write source.md.
	sourceMD := xlsxSourceMarkdown(entries, filepath.Base(p.FilePath))
	charCount := utf8.RuneCountInString(sourceMD)
	pageCount := p.PageCount
	if pageCount < 1 {
		pageCount = max(1, (charCount+docxCharsPerPage-1)/docxCharsPerPage)
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
	if err := c.reg.File().UpdateExtracted(ctx, p.FileID, sourcePath, charCount, pageCount); err != nil {
		fail(err.Error())
		return
	}

	// Step 3: Detect source language.
	srcLang := gateway.SourceLangForTranslate(sourceMD)
	if srcLang != "auto" {
		_ = c.reg.Message().UpdateSourceLang(ctx, p.UserID, srcLang)
		_ = c.reg.Message().UpdateSourceLang(ctx, p.AssistantID, srcLang)
	}

	// Emit file:source so FE renders the source card.
	runtime.EventsEmit(ctx, "file:source", map[string]string{
		"sessionId":          p.SessionID,
		"assistantMessageId": p.AssistantID,
	})

	// Emit initial progress so ring shows 0% immediately instead of spinning.
	runtime.EventsEmit(ctx, "file:progress", bridge.FileProgress{
		Chunk:   0,
		Total:   totalBatches,
		Percent: 0,
	})

	// Step 4: Translate all unique strings concurrently.
	translations, totalTokens, err := c.translateXlsxStrings(ctx, items, len(entries), srcLang, p.TargetLang, p.Style, p.Provider,
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
		Chunk:   totalBatches,
		Total:   totalBatches,
		Percent: 100,
	})

	// Step 5: Translate sheet names from workbook.xml (non-fatal if fails).
	sheetNames, rawWorkbook, _ := parseXlsxWorkbookSheetNames(p.FilePath)
	newWorkbook := ""
	if len(sheetNames) > 0 && rawWorkbook != "" {
		translatedSheetNames := c.translateXlsxSheetNames(ctx, sheetNames, srcLang, p.TargetLang, p.Style, p.Provider)
		newWorkbook = buildNewWorkbook(rawWorkbook, translatedSheetNames)
	}

	// Step 6: Build new sharedStrings.xml and write translated xlsx.
	newSS := buildNewSharedStrings(rawSS, entries, translations)
	translatedPath := filepath.Join(subDir, "translated.xlsx")
	if err := writeXlsxTranslated(p.FilePath, translatedPath, newSS, newWorkbook); err != nil {
		fail(fmt.Sprintf("không tạo được file Excel đã dịch: %v", err))
		return
	}

	// Step 6: Persist results.
	usedTokens := totalTokens
	if usedTokens == 0 {
		usedTokens = estimateTokens(sourceMD)
	}
	if err := c.reg.Message().UpdateTranslated(ctx, p.AssistantID, sourceMD, usedTokens); err != nil {
		fail(err.Error())
		return
	}
	if err := c.reg.File().UpdateTranslated(ctx, p.FileID, sourcePath, translatedPath, charCount, pageCount, p.ModelUsed, "xlsx"); err != nil {
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
		FileID:       p.FileID,
		FileName:     filepath.Base(p.FilePath),
		FileType:     "xlsx",
		OutputFormat: "xlsx",
		CharCount:    charCount,
		PageCount:    pageCount,
	})
}
