package file

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"translate-app/internal/bridge"
	"translate-app/internal/model"
	"translate-app/internal/repository"
)

// maxFilePages — giới hạn số trang một lần dịch file (đồng bộ FE MAX_FILE_PAGE_COUNT).
const maxFilePages = 200

func (c *controller) TranslateFile(ctx context.Context, req bridge.FileRequest) error {
	if strings.TrimSpace(req.SessionID) == "" {
		return errors.New("chưa có phiên làm việc")
	}
	path := strings.TrimSpace(req.FilePath)
	if path == "" {
		return errors.New("chưa chọn tệp")
	}

	sess, err := c.sessionByID(ctx, req.SessionID)
	if err != nil {
		return err
	}
	targetLang := strings.TrimSpace(req.TargetLang)
	if targetLang == "" {
		targetLang = strings.TrimSpace(sess.TargetLang)
	}
	if targetLang == "" {
		return errors.New("phiên chưa có ngôn ngữ đích")
	}

	info, err := c.ReadFileInfo(ctx, path)
	if err != nil {
		return err
	}
	if info.PageCount > maxFilePages {
		return fmt.Errorf("Tệp quá lớn (tối đa %d trang)", maxFilePages)
	}

	provider, st, err := c.resolveProvider(ctx, req.Provider, req.Model)
	if err != nil {
		return err
	}
	style := effectiveStyle(req.Style, st.DefaultStyle)
	modelUsed := resolveModelUsed(st, req.Provider, req.Model)

	fileID := uuid.New().String()
	userID := uuid.New().String()
	assistantID := uuid.New().String()

	clean := filepath.Clean(path)
	fileRow := &model.File{
		ID:           fileID,
		SessionID:    req.SessionID,
		FileName:     filepath.Base(clean),
		FileType:     info.Type,
		FileSize:     info.FileSize,
		OriginalPath: clean,
		PageCount:    info.PageCount,
		CharCount:    info.CharCount,
		Style:        style,
		ModelUsed:    modelUsed,
		Status:       "processing",
	}

	fid := fileID
	userMsg := &model.Message{
		ID:                userID,
		SessionID:         req.SessionID,
		Role:              model.RoleUser,
		DisplayMode:       model.DisplayModeFile,
		OriginalContent:   fmt.Sprintf("📎 %s", filepath.Base(clean)),
		TranslatedContent: "",
		FileID:            &fid,
		SourceLang:        "auto",
		TargetLang:        targetLang,
		Style:             style,
		ModelUsed:         modelUsed,
	}

	asstMsg := &model.Message{
		ID:                assistantID,
		SessionID:         req.SessionID,
		Role:              model.RoleAssistant,
		DisplayMode:       model.DisplayModeFile,
		OriginalContent:   "",
		TranslatedContent: "",
		FileID:            &fid,
		SourceLang:        "auto",
		TargetLang:        targetLang,
		Style:             style,
		ModelUsed:         modelUsed,
	}

	if err := c.reg.DoInTx(ctx, func(tx repository.Registry) error {
		if err := tx.File().Insert(ctx, fileRow); err != nil {
			return err
		}
		if err := tx.Message().Insert(ctx, userMsg); err != nil {
			return err
		}
		if err := tx.Message().Insert(ctx, asstMsg); err != nil {
			return err
		}
		if err := tx.Session().UpdateTargetLang(ctx, req.SessionID, targetLang); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	_ = c.reg.Settings().Upsert(ctx, "last_target_lang", targetLang)

	runtime.EventsEmit(ctx, "translation:start", map[string]string{
		"messageId": assistantID,
		"sessionId": req.SessionID,
	})

	go c.runFileTranslate(ctx, fileTranslateParams{
		SessionID:   req.SessionID,
		FilePath:    clean,
		FileID:      fileID,
		UserID:      userID,
		AssistantID: assistantID,
		TargetLang:  targetLang,
		Style:       style,
		ModelUsed:   modelUsed,
		PageCount:   info.PageCount,
		Provider:    provider,
	})
	return nil
}

func (c *controller) sessionByID(ctx context.Context, id string) (*model.Session, error) {
	list, err := c.reg.Session().List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i], nil
		}
	}
	return nil, errors.New("không tìm thấy phiên")
}
