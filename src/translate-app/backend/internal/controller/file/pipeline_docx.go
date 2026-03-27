package file

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"translate-app/internal/gateway"
	"translate-app/internal/model"
)

// translateDocxFile translates all paragraphs in df and returns a []string
// of the same length as df.Paragraphs (one translated string per paragraph).
//
// Paragraphs are batched by chunkDocxParagraphs to stay within token limits.
// Each batch is sent to the AI as a numbered list; the response is parsed back
// into individual paragraph translations.
//
// onProgress is called after each batch with (batchIndex, totalBatches).
func (c *controller) translateDocxFile(
	ctx context.Context,
	df *DocxFile,
	sourceLang, targetLang string,
	style model.TranslationStyle,
	provider gateway.AIProvider,
	onProgress func(batchIdx, totalBatches int),
) ([]string, int, error) {
	// Pre-allocate result aligned with df.Paragraphs.
	results := make([]string, len(df.Paragraphs))
	start := time.Now()

	// Build a flat index so we can map batch positions back to global positions.
	batches := chunkDocxParagraphs(df.Paragraphs, charsPerChunk)
	if len(batches) == 0 {
		return results, 0, nil
	}

	// Pre-compute global start index for each batch.
	batchGlobalStart := make([]int, len(batches))
	idx := 0
	for i, batch := range batches {
		batchGlobalStart[i] = idx
		idx += len(batch)
	}

	// Run batches in parallel with a concurrency cap (provider-aware).
	maxConcurrent := provider.MaxBatchConcurrency()
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	sem := make(chan struct{}, maxConcurrent)

	type batchResult struct {
		batchIdx int
		tokens   int
		err      error
	}
	resultCh := make(chan batchResult, len(batches))

	var completedBatches int32
	var wg sync.WaitGroup
	for batchIdx, batch := range batches {
		wg.Add(1)
		go func(batchIdx int, batch []DocxParagraph) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			globalBase := batchGlobalStart[batchIdx]

			var translatableLocal []int
			for i, p := range batch {
				if strings.TrimSpace(p.Text) == "" {
					results[globalBase+i] = ""
				} else {
					translatableLocal = append(translatableLocal, i)
				}
			}

			if len(translatableLocal) == 0 {
				done := int(atomic.AddInt32(&completedBatches, 1))
				if onProgress != nil {
					onProgress(done, len(batches))
				}
				resultCh <- batchResult{batchIdx: batchIdx}
				return
			}

			input := buildBatchInput(batch, translatableLocal)
			translated, tokens, err := c.streamTranslateDocxBatch(ctx, provider, input, sourceLang, targetLang, style)
			if err != nil {
				resultCh <- batchResult{batchIdx: batchIdx, err: fmt.Errorf("batch %d: %w", batchIdx+1, err)}
				return
			}

			parsed := parseBatchOutput(translated, len(translatableLocal))
			for i, localIdx := range translatableLocal {
				text := ""
				if i < len(parsed) {
					text = parsed[i]
				}
				if text == "" {
					text = batch[localIdx].Text
				}
				results[globalBase+localIdx] = text
			}

			done := int(atomic.AddInt32(&completedBatches, 1))
			if onProgress != nil {
				onProgress(done, len(batches))
			}
			resultCh <- batchResult{batchIdx: batchIdx, tokens: tokens}
		}(batchIdx, batch)
	}

	wg.Wait()
	close(resultCh)

	var totalTokens int
	for r := range resultCh {
		if r.err != nil {
			return nil, 0, r.err
		}
		totalTokens += r.tokens
	}

	fmt.Printf("[DEBUG] translateDocxFile done — batches=%d total_tokens=%d elapsed=%.2fs\n", len(batches), totalTokens, time.Since(start).Seconds())
	return results, totalTokens, nil
}

// batchFormatInstruction is a short reminder prepended to each batch user message.
// The detailed format rule lives in the system prompt (BuildDocxBatchSystemPrompt).
const batchFormatInstruction = "Translate each paragraph below, preserving the <<<N>>> markers:\n\n"

// streamTranslateDocxBatch calls the dedicated batch translation method on the provider,
// which uses BuildDocxBatchSystemPrompt — a prompt that explicitly preserves <<<N>>> markers
// without conflicting "output-only" instructions.
func (c *controller) streamTranslateDocxBatch(
	ctx context.Context,
	provider gateway.AIProvider,
	text, sourceLang, targetLang string,
	style model.TranslationStyle,
) (string, int, error) {
	events := make(chan gateway.StreamEvent, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- provider.TranslateBatchStream(ctx, text, sourceLang, targetLang, string(style), events)
	}()

	var full strings.Builder
	var tokensUsed int
	for ev := range events {
		switch ev.Type {
		case "chunk":
			if ev.Content != "" {
				full.WriteString(ev.Content)
			}
		case "usage":
			tokensUsed = ev.TokensUsed
		case "error":
			errMsg := errors.New("batch translation failed")
			if ev.Error != nil {
				errMsg = ev.Error
			}
			<-errCh
			return "", 0, errMsg
		}
	}
	if err := <-errCh; err != nil {
		return "", 0, err
	}
	fmt.Printf("[DEBUG] streamTranslateDocxBatch done — tokens=%d\n", tokensUsed)
	return full.String(), tokensUsed, nil
}


// buildBatchInput formats a subset of paragraphs (identified by localIndices)
// as a numbered list for the AI translation prompt.
//
// Format:
//
//	<<<1>>>
//	Paragraph one text
//
//	<<<2>>>
//	Paragraph two text
func buildBatchInput(batch []DocxParagraph, localIndices []int) string {
	var sb strings.Builder
	sb.WriteString(batchFormatInstruction)
	for n, idx := range localIndices {
		if n > 0 {
			sb.WriteString("\n\n")
		}
		fmt.Fprintf(&sb, "<<<%d>>>\n%s", n+1, batch[idx].Text)
	}
	return sb.String()
}

// reBatchMarker matches "<<<N>>>" at the start of a line, allowing:
//   - optional leading whitespace (AI sometimes indents markers)
//   - 3 or more angle brackets on each side (AI sometimes "amplifies" to <<<<<<<N>>>>>>>)
//
// Using angle-bracket triples avoids collision with [N] list markers common in documents.
var reBatchMarker = regexp.MustCompile(`(?m)^[ \t]*<{3,}(\d+)>{3,}`)

// parseBatchOutput splits an AI response that uses <<<N>>> markers back into
// individual paragraph strings.
//
// If the AI returns fewer segments than expected, the missing ones are "".
// Fallback: if no <<<N>>> markers are found, split on blank lines and assign sequentially.
func parseBatchOutput(raw string, expected int) []string {
	raw = strings.TrimSpace(raw)
	results := make([]string, expected)

	// FindAllStringSubmatchIndex returns [fullStart, fullEnd, capStart, capEnd] per match.
	// cap group 1 is the digit(s) — works regardless of how many < or > the AI uses.
	matches := reBatchMarker.FindAllStringSubmatchIndex(raw, -1)
	if len(matches) == 0 {
		// AI did not use <<<N>>> markers — best effort: split on double newlines.
		parts := strings.Split(raw, "\n\n")
		slot := 0
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if slot >= expected {
				break
			}
			results[slot] = p
			slot++
		}
		return results
	}

	for i, m := range matches {
		// m[2]:m[3] is the captured digit string — no Sscanf format needed.
		var num int
		fmt.Sscanf(raw[m[2]:m[3]], "%d", &num)
		slot := num - 1
		if slot < 0 || slot >= expected {
			continue
		}

		// Content runs from end of this marker to start of next marker (or EOF).
		contentStart := m[1]
		contentEnd := len(raw)
		if i+1 < len(matches) {
			contentEnd = matches[i+1][0]
		}

		results[slot] = strings.TrimSpace(raw[contentStart:contentEnd])
	}

	return results
}
