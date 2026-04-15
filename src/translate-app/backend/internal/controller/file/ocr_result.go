package file

import "fmt"

// StructuredOCRResult is the top-level JSON payload returned by the OCR sidecar.
type StructuredOCRResult struct {
	Pages []OCRPage `json:"pages"`
}

// OCRPage represents one PDF page's detected layout regions.
type OCRPage struct {
	PageNo  int         `json:"page_no"`
	Width   int         `json:"width"`
	Height  int         `json:"height"`
	Regions []OCRRegion `json:"regions"`
}

// OCRRegion is a single detected layout region on a page.
type OCRRegion struct {
	// Type is one of: "text", "title", "table", "figure".
	Type string `json:"type"`
	// FigureType is set when Type=="figure": "decorative" or "informational".
	FigureType string `json:"figure_type,omitempty"`
	// BBox is [x1, y1, x2, y2] in pixels relative to the page PNG.
	BBox []int `json:"bbox"`
	// Content holds OCR text for text/title regions.
	Content string `json:"content,omitempty"`
	// Alignment is "left", "center", or "right" — inferred from bbox position.
	Alignment string `json:"alignment,omitempty"`
	// HTML holds the table HTML produced by rapid_table.
	HTML string `json:"html,omitempty"`
	// ImageData holds the base64 data URL (data:image/png;base64,...) for figure
	// regions. Populated from Mistral OCR's embedded image response — no extra API
	// call needed.
	ImageData string `json:"image_data,omitempty"`
	// TextLines holds OCR-detected text for informational figure regions.
	TextLines []string `json:"text_lines,omitempty"`
}

// regionKey returns the lookup key used to correlate regions across
// the translated-content map and the figure-crops map.
// Format: "{pageNo}_{regionIdx}"
func regionKey(pageNo, regionIdx int) string {
	return fmt.Sprintf("%d_%d", pageNo, regionIdx)
}
