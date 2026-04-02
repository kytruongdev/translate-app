package file

import (
	"context"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (c *controller) OpenFileDialog(ctx context.Context) (string, error) {
	path, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{
		Title: "Chọn tệp",
		Filters: []runtime.FileFilter{
			{DisplayName: "Tài liệu (*.docx, *.pdf, *.xlsx)", Pattern: "*.docx;*.pdf;*.xlsx"},
			{DisplayName: "Word Documents (*.docx)", Pattern: "*.docx"},
			{DisplayName: "PDF Documents (*.pdf)", Pattern: "*.pdf"},
			{DisplayName: "Excel Documents (*.xlsx)", Pattern: "*.xlsx"},
		},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(path), nil
}
