package file

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"os"
)

// extractFigureCrops iterates over all figure regions in result and crops each one
// from the corresponding page PNG image.
//
// Returns a map keyed by regionKey(pageNo, regionIdx) → base64 PNG.
// Individual crop failures are silently skipped (the figure will be absent in the HTML).
// After all crops are extracted the caller should delete the image temp directory.
func extractFigureCrops(result *StructuredOCRResult, imagePaths []string) map[string]string {
	crops := make(map[string]string)
	for _, page := range result.Pages {
		pageIdx := page.PageNo - 1
		if pageIdx < 0 || pageIdx >= len(imagePaths) {
			continue
		}
		imgPath := imagePaths[pageIdx]
		for ri, region := range page.Regions {
			if region.Type != "figure" {
				continue
			}
			if len(region.BBox) < 4 {
				continue
			}
			b64, err := cropImageToBase64(imgPath, region.BBox)
			if err != nil {
				continue
			}
			crops[regionKey(page.PageNo, ri)] = b64
		}
	}
	return crops
}


// cropImageToBase64 reads an image from path, crops it to the given bounding box [x1, y1, x2, y2],
// and returns the cropped area as a base64 encoded PNG string.
func cropImageToBase64(imagePath string, bbox []int) (string, error) {
	if len(bbox) < 4 {
		return "", fmt.Errorf("invalid bbox format")
	}

	f, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("open image: %w", err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}

	// Define cropping area
	x1, y1, x2, y2 := bbox[0], bbox[1], bbox[2], bbox[3]
	
	// Ensure coordinates are within bounds
	bounds := img.Bounds()
	if x1 < bounds.Min.X { x1 = bounds.Min.X }
	if y1 < bounds.Min.Y { y1 = bounds.Min.Y }
	if x2 > bounds.Max.X { x2 = bounds.Max.X }
	if y2 > bounds.Max.Y { y2 = bounds.Max.Y }

	// Ensure width/height are positive
	if x2 <= x1 || y2 <= y1 {
		return "", fmt.Errorf("invalid crop dimensions")
	}

	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}

	sub, ok := img.(subImager)
	if !ok {
		return "", fmt.Errorf("image does not support sub-imaging")
	}

	cropped := sub.SubImage(image.Rect(x1, y1, x2, y2))

	// Encode to PNG Base64
	var buf bytes.Buffer
	if err := png.Encode(&buf, cropped); err != nil {
		return "", fmt.Errorf("encode png: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
