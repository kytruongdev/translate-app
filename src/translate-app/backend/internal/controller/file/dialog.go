package file

import (
	"context"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (c *controller) OpenFileDialog(ctx context.Context) (string, error) {
	path, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{
		Title: "Chọn tệp PDF hoặc Word",
		Filters: []runtime.FileFilter{
			{DisplayName: "PDF, Word (*.pdf;*.docx)", Pattern: "*.pdf;*.docx"},
		},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(path), nil
}
