package file

import (
	"context"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (c *controller) OpenFileDialog(ctx context.Context) (string, error) {
	path, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{
		Title: "Chọn tệp Word",
		Filters: []runtime.FileFilter{
			{DisplayName: "Word Documents (*.docx, *.doc)", Pattern: "*.docx;*.doc"},
		},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(path), nil
}
