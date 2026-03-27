package handler

import (
	"errors"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// OpenFileDialog opens a native file picker — E6.
func (a *App) OpenFileDialog() (string, error) {
	return a.ctrl.File.OpenFileDialog(a.appCtx())
}

// ReadFileInfo returns PDF/DOCX metadata — E6.
func (a *App) ReadFileInfo(path string) (*FileInfo, error) {
	return a.ctrl.File.ReadFileInfo(a.appCtx(), path)
}

// TranslateFile starts file translation — E6.
func (a *App) TranslateFile(req FileRequest) error {
	return a.ctrl.File.TranslateFile(a.appCtx(), req)
}

// CancelFileTranslate cancels an in-progress file translation and deletes its records — E6.
func (a *App) CancelFileTranslate(fileID string) error {
	return a.ctrl.File.CancelFileTranslate(a.appCtx(), fileID)
}

// GetFileContent loads source + translated markdown from disk — E6.
func (a *App) GetFileContent(fileID string) (*FileContent, error) {
	return a.ctrl.File.GetFileContent(a.appCtx(), fileID)
}

// ExportFile exports a translated file — E7.
func (a *App) ExportFile(fileID string, format string) (string, error) {
	return a.ctrl.File.ExportFile(a.appCtx(), fileID, format)
}

// ExportMessage exports one assistant message — E7.
func (a *App) ExportMessage(id string, format string) (string, error) {
	return "", errors.New("not implemented")
}

// ExportSession exports a whole session — E7.
func (a *App) ExportSession(id string, format string) (string, error) {
	return "", errors.New("not implemented")
}

// CopyTranslation copies translated text to the system clipboard and returns it — E7.
func (a *App) CopyTranslation(messageID string) (string, error) {
	ctx := a.appCtx()
	text, err := a.ctrl.Message.CopyTranslationText(ctx, messageID)
	if err != nil {
		return "", err
	}
	if err := runtime.ClipboardSetText(ctx, text); err != nil {
		return text, err
	}
	return text, nil
}
