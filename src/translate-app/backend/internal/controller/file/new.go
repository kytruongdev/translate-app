package file

import (
	"context"
	"errors"

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
}

type controller struct {
	reg repository.Registry
}

// New constructs a file controller.
func New(reg repository.Registry) Controller {
	return &controller{reg: reg}
}

func (c *controller) OpenFileDialog(ctx context.Context) (string, error) {
	return "", errors.New("not implemented")
}

func (c *controller) ReadFileInfo(ctx context.Context, path string) (*bridge.FileInfo, error) {
	return nil, errors.New("not implemented")
}

func (c *controller) TranslateFile(ctx context.Context, req bridge.FileRequest) error {
	return errors.New("not implemented")
}

func (c *controller) GetFileContent(ctx context.Context, fileID string) (*bridge.FileContent, error) {
	return nil, errors.New("not implemented")
}

func (c *controller) ExportFile(ctx context.Context, fileID, format string) (string, error) {
	return "", errors.New("not implemented")
}
