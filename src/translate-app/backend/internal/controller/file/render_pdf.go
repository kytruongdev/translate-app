package file

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/gen2brain/go-fitz"
)

const pdfRenderDPI = 200.0

// renderPDFToImages renders every page of a PDF file to PNG images at 200 DPI.
// Images are written to a temporary directory. The caller is responsible for
// deleting the directory when done (os.RemoveAll(tempDir)).
//
// Returns:
//   - imagePaths: ordered list of PNG file paths (one per page)
//   - tempDir:    the temporary directory containing the images
func renderPDFToImages(ctx context.Context, pdfPath string) (imagePaths []string, tempDir string, err error) {
	doc, err := fitz.New(pdfPath)
	if err != nil {
		return nil, "", fmt.Errorf("fitz: open PDF: %w", err)
	}
	defer doc.Close()

	n := doc.NumPage()
	if n == 0 {
		return nil, "", fmt.Errorf("PDF has no pages: %s", pdfPath)
	}

	tempDir, err = os.MkdirTemp("", "gnj-pdf-render-*")
	if err != nil {
		return nil, "", fmt.Errorf("create temp dir: %w", err)
	}

	imagePaths = make([]string, 0, n)

	for i := 0; i < n; i++ {
		// Check for cancellation before each page
		select {
		case <-ctx.Done():
			_ = os.RemoveAll(tempDir)
			return nil, "", ctx.Err()
		default:
		}

		img, err := doc.ImageDPI(i, pdfRenderDPI)
		if err != nil {
			_ = os.RemoveAll(tempDir)
			return nil, "", fmt.Errorf("fitz: render page %d: %w", i+1, err)
		}

		outPath := filepath.Join(tempDir, fmt.Sprintf("page-%04d.png", i+1))
		f, err := os.Create(outPath)
		if err != nil {
			_ = os.RemoveAll(tempDir)
			return nil, "", fmt.Errorf("create PNG file page %d: %w", i+1, err)
		}

		if err := png.Encode(f, img); err != nil {
			f.Close()
			_ = os.RemoveAll(tempDir)
			return nil, "", fmt.Errorf("encode PNG page %d: %w", i+1, err)
		}
		f.Close()

		imagePaths = append(imagePaths, outPath)
	}

	return imagePaths, tempDir, nil
}
