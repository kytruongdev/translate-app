package main

// mistral_vision.go — Dùng Pixtral (Mistral multimodal chat) để OCR từng page dưới dạng ảnh PNG.
// Khác với mistral_ocr.go (gửi raw PDF lên /v1/ocr), file này gửi PNG lên /v1/chat/completions
// với model pixtral-large-latest — mỗi page là 1 image trong message.
//
// Mục tiêu test: xem Pixtral có extract được cross-page table tốt hơn OCR API không
// bằng cách gửi 2 page liên tiếp trong cùng 1 request.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const (
	pixtralModel    = "pixtral-large-latest"
	mistralChatURL  = "https://api.mistral.ai/v1/chat/completions"
	pixtralMaxTok   = 16384
)

const pixtralPrompt = `These are consecutive pages from a Vietnamese legal document (PDF scan).
Extract ALL content in reading order. Return ONLY valid JSON — no markdown fences.

JSON format:
{
  "regions": [
    {"type": "text",   "content": "exact Vietnamese text", "alignment": "left|center|right"},
    {"type": "title",  "content": "heading or section label", "alignment": "left|center|right"},
    {"type": "table",  "html": "<table>...</table>"},
    {"type": "figure", "figure_type": "decorative"}
  ]
}

IMPORTANT RULES:
- Copy Vietnamese text EXACTLY — preserve all diacritics, do NOT translate
- Tables that SPAN ACROSS PAGES: treat as ONE continuous table, output all rows in a single "table" region
- Header row on page 1 + data rows on page 2 = one complete table
- Every row must have the same number of cells as the widest row (use <td></td> for empty cells)
- Use <th> for header cells, <td> for data cells
- Numbered sections (e.g. "2. Tổng thu nhập...") → "text" region
- Stamps/seals/signatures → "figure" figure_type "decorative"`

// pixtralContent is the Mistral chat API content item.
type pixtralContent struct {
	Type     string            `json:"type"`
	Text     string            `json:"text,omitempty"`
	ImageURL *pixtralImageURL  `json:"image_url,omitempty"`
}

type pixtralImageURL struct {
	URL string `json:"url"`
}

type pixtralMessage struct {
	Role    string           `json:"role"`
	Content []pixtralContent `json:"content"`
}

type pixtralRequest struct {
	Model       string           `json:"model"`
	Messages    []pixtralMessage `json:"messages"`
	MaxTokens   int              `json:"max_tokens"`
	Temperature float64          `json:"temperature"`
}

type pixtralChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type pixtralResponse struct {
	Choices []pixtralChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// runMistralVision renders all PDF pages to PNG then calls Pixtral page-by-page.
// For page boundaries where a cross-page table might exist (page N ends with table),
// pages N and N+1 are sent together in one request so Pixtral sees the full context.
func runMistralVision(pdfPath string, pageFilter map[int]bool, apiKey string) ([]pageResult, error) {
	fmt.Println("Rendering pages to PNG…")
	imagePaths, tempDir, err := renderPDF(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("render PDF: %w", err)
	}
	defer os.RemoveAll(tempDir)
	fmt.Printf("  %d pages rendered\n", len(imagePaths))

	// Apply page filter
	type indexedPage struct {
		pageNo int
		path   string
	}
	var todo []indexedPage
	for i, p := range imagePaths {
		pageNo := i + 1
		if pageFilter != nil && !pageFilter[pageNo] {
			continue
		}
		todo = append(todo, indexedPage{pageNo, p})
	}

	var results []pageResult
	processed := map[int]bool{}

	for idx, item := range todo {
		if processed[item.pageNo] {
			continue
		}

		// Check if next page exists in todo and we should send them together
		// (cross-page table heuristic: always try sending pairs for now during testing)
		var imgPaths []string
		var pageNos []int

		imgPaths = append(imgPaths, item.path)
		pageNos = append(pageNos, item.pageNo)

		// If next page is adjacent, include it in the same request
		if idx+1 < len(todo) && todo[idx+1].pageNo == item.pageNo+1 {
			imgPaths = append(imgPaths, todo[idx+1].path)
			pageNos = append(pageNos, todo[idx+1].pageNo)
			processed[todo[idx+1].pageNo] = true
		}

		if len(pageNos) == 1 {
			fmt.Printf("  Page %d… ", pageNos[0])
		} else {
			fmt.Printf("  Pages %d+%d (pair)… ", pageNos[0], pageNos[1])
		}

		regions, err := callPixtral(apiKey, imgPaths)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			for _, no := range pageNos {
				results = append(results, pageResult{pageNo: no, err: err})
			}
			continue
		}

		fmt.Printf("%d regions\n", len(regions))

		// All regions go into the first page of the pair — viewer sees them together
		results = append(results, pageResult{
			pageNo:  pageNos[0],
			regions: regions,
		})
		// Second page of pair is empty (content merged into first)
		if len(pageNos) == 2 {
			results = append(results, pageResult{pageNo: pageNos[1]})
		}
	}

	return results, nil
}

// callPixtral sends one or more page images to pixtral-large-latest and returns parsed regions.
func callPixtral(apiKey string, imgPaths []string) ([]region, error) {
	// Build content: images first, then prompt
	var content []pixtralContent
	for _, p := range imgPaths {
		imgData, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read image %s: %w", p, err)
		}
		b64 := base64.StdEncoding.EncodeToString(imgData)
		content = append(content, pixtralContent{
			Type:     "image_url",
			ImageURL: &pixtralImageURL{URL: "data:image/png;base64," + b64},
		})
	}
	content = append(content, pixtralContent{
		Type: "text",
		Text: pixtralPrompt,
	})

	reqBody := pixtralRequest{
		Model: pixtralModel,
		Messages: []pixtralMessage{
			{Role: "user", Content: content},
		},
		MaxTokens:   pixtralMaxTok,
		Temperature: 0,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", mistralChatURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var pixtralResp pixtralResponse
	if err := json.Unmarshal(respBody, &pixtralResp); err != nil {
		return nil, fmt.Errorf("parse response: %w\nraw: %s", err, truncate(string(respBody), 300))
	}

	if pixtralResp.Error != nil {
		return nil, fmt.Errorf("Pixtral API error: %s", pixtralResp.Error.Message)
	}

	if len(pixtralResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned, raw: %s", truncate(string(respBody), 300))
	}

	raw := stripFences(pixtralResp.Choices[0].Message.Content)
	fmt.Fprintf(os.Stderr, "\n=== PIXTRAL RAW ===\n%s\n=== END ===\n", truncate(raw, 1000))

	regions := parseRegions(raw)
	if regions == nil {
		return nil, fmt.Errorf("parseRegions failed, raw[:200]=%q", truncate(raw, 200))
	}
	return regions, nil
}
