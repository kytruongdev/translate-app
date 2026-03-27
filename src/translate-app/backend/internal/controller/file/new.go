package file

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"translate-app/config"
	"translate-app/internal/bridge"
	"translate-app/internal/repository"
)

// Controller is file translation domain API.
type Controller interface {
	OpenFileDialog(ctx context.Context) (string, error)
	ReadFileInfo(ctx context.Context, path string) (*bridge.FileInfo, error)
	TranslateFile(ctx context.Context, req bridge.FileRequest) error
	CancelFileTranslate(ctx context.Context, fileID string) error
	GetFileContent(ctx context.Context, fileID string) (*bridge.FileContent, error)
	ExportFile(ctx context.Context, fileID, format string) (string, error)
	// RunRetranslateContent re-runs the chunked pipeline on already-extracted markdown (retranslate flow).
	RunRetranslateContent(ctx context.Context, p RetranslateContentParams)
}

type controller struct {
	reg      repository.Registry
	keys     *config.APIKeys
	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc // fileID → cancel func for active jobs
}

// New constructs a file controller.
func New(reg repository.Registry, keys *config.APIKeys) Controller {
	if keys == nil {
		keys = &config.APIKeys{}
	}
	return &controller{
		reg:     reg,
		keys:    keys,
		cancels: make(map[string]context.CancelFunc),
	}
}

func (c *controller) CancelFileTranslate(ctx context.Context, fileID string) error {
	c.cancelMu.Lock()
	cancel, ok := c.cancels[fileID]
	c.cancelMu.Unlock()
	if !ok {
		return errors.New("không tìm thấy tiến trình dịch đang chạy")
	}
	cancel()
	return nil
}

func (c *controller) GetFileContent(ctx context.Context, fileID string) (*bridge.FileContent, error) {
	if strings.TrimSpace(fileID) == "" {
		return nil, errors.New("thiếu fileId")
	}
	f, err := c.reg.File().GetByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, errors.New("không tìm thấy tệp")
	}
	if f.SourcePath == "" {
		return nil, errors.New("chưa có nội dung nguồn")
	}
	src, err := os.ReadFile(f.SourcePath)
	if err != nil {
		return nil, err
	}
	out := &bridge.FileContent{SourceMarkdown: string(src)}
	if f.TranslatedPath != "" {
		tr, err := os.ReadFile(f.TranslatedPath)
		if err != nil {
			return nil, err
		}
		out.TranslatedMarkdown = string(tr)
	}
	return out, nil
}

func (c *controller) ExportFile(ctx context.Context, fileID, _ string) (string, error) {
	if strings.TrimSpace(fileID) == "" {
		return "", errors.New("thiếu fileId")
	}
	f, err := c.reg.File().GetByID(ctx, fileID)
	if err != nil {
		return "", err
	}
	if f == nil {
		return "", errors.New("không tìm thấy tệp")
	}
	if f.TranslatedPath == "" {
		return "", errors.New("file chưa được dịch")
	}
	if _, err := os.Stat(f.TranslatedPath); err != nil {
		return "", errors.New("file đã dịch không tồn tại trên ổ đĩa")
	}

	ext := strings.ToLower(filepath.Ext(f.TranslatedPath))
	defaultName := strings.TrimSuffix(f.FileName, filepath.Ext(f.FileName)) + "_translated" + ext

	savePath, err := runtime.SaveFileDialog(ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultName,
		Filters: []runtime.FileFilter{
			{DisplayName: "DOCX Document (*.docx)", Pattern: "*.docx"},
		},
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(savePath) == "" {
		return "", nil // user cancelled
	}

	if err := copyFile(f.TranslatedPath, savePath); err != nil {
		return "", err
	}
	return savePath, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
