package file

import (
	"context"
	"errors"
	"os"
	"strings"

	"translate-app/config"
	"translate-app/internal/bridge"
	"translate-app/internal/repository"
)

// Controller is file translation domain API.
type Controller interface {
	OpenFileDialog(ctx context.Context) (string, error)
	ReadFileInfo(ctx context.Context, path string) (*bridge.FileInfo, error)
	TranslateFile(ctx context.Context, req bridge.FileRequest) error
	GetFileContent(ctx context.Context, fileID string) (*bridge.FileContent, error)
	ExportFile(ctx context.Context, fileID, format string) (string, error)
	// RunRetranslateContent re-runs the chunked pipeline on already-extracted markdown (retranslate flow).
	RunRetranslateContent(ctx context.Context, p RetranslateContentParams)
}

type controller struct {
	reg  repository.Registry
	keys *config.APIKeys
}

// New constructs a file controller.
func New(reg repository.Registry, keys *config.APIKeys) Controller {
	if keys == nil {
		keys = &config.APIKeys{}
	}
	return &controller{reg: reg, keys: keys}
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

func (c *controller) ExportFile(ctx context.Context, fileID, format string) (string, error) {
	return "", errors.New("not implemented")
}
