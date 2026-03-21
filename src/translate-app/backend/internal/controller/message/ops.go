package message

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"translate-app/internal/bridge"
	"translate-app/internal/model"
	"translate-app/internal/repository"
)

// CreateSessionAndSend creates a session, inserts user + assistant rows, then starts translation in a goroutine.
func (c *controller) CreateSessionAndSend(ctx context.Context, req bridge.CreateSessionAndSendRequest) (bridge.CreateSessionAndSendResult, error) {
	var zero bridge.CreateSessionAndSendResult
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return zero, fmt.Errorf("empty content")
	}
	if strings.TrimSpace(req.TargetLang) == "" {
		return zero, fmt.Errorf("empty targetLang")
	}

	provider, st, err := c.resolveProvider(ctx, "", "")
	if err != nil {
		return zero, err
	}

	style := effectiveStyle(req.Style, st.DefaultStyle)
	modelUsed := resolveModelUsed(st, "", "")
	preserveMD := req.DisplayMode == model.DisplayModeBilingual

	sessionID := uuid.New().String()
	userID := uuid.New().String()
	assistantID := uuid.New().String()
	title := sessionTitleFromRequest(req.Title, content)

	sess := &model.Session{
		ID:         sessionID,
		Title:      title,
		Status:     model.SessionStatusActive,
		TargetLang: req.TargetLang,
		Style:      string(style),
		Model:      modelUsed,
	}

	userMsg := &model.Message{
		ID:              userID,
		SessionID:       sessionID,
		Role:            model.RoleUser,
		DisplayMode:     req.DisplayMode,
		OriginalContent: content,
		SourceLang:      req.SourceLang,
		TargetLang:      req.TargetLang,
		Style:           style,
		ModelUsed:       modelUsed,
		OriginalMessageID: nil,
	}

	asstMsg := &model.Message{
		ID:                assistantID,
		SessionID:         sessionID,
		Role:              model.RoleAssistant,
		DisplayMode:       req.DisplayMode,
		OriginalContent:   "",
		TranslatedContent: "",
		SourceLang:        req.SourceLang,
		TargetLang:        req.TargetLang,
		Style:             style,
		ModelUsed:         modelUsed,
	}

	if err := c.reg.DoInTx(ctx, func(tx repository.Registry) error {
		if err := tx.Session().Create(ctx, sess); err != nil {
			return err
		}
		if err := tx.Message().Insert(ctx, userMsg); err != nil {
			return err
		}
		if err := tx.Message().Insert(ctx, asstMsg); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return zero, err
	}

	_ = c.reg.Settings().Upsert(ctx, "last_target_lang", req.TargetLang)

	runtime.EventsEmit(ctx, "translation:start", map[string]string{
		"messageId": assistantID,
		"sessionId": sessionID,
	})
	go c.runTranslationStream(ctx, sessionID, assistantID, content, req.SourceLang, req.TargetLang, style, preserveMD, provider)
	return bridge.CreateSessionAndSendResult{SessionID: sessionID, MessageID: assistantID}, nil
}

// SendMessage inserts user + assistant messages in the existing session and starts translation.
func (c *controller) SendMessage(ctx context.Context, req bridge.SendRequest) (string, error) {
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return "", fmt.Errorf("empty content")
	}
	if strings.TrimSpace(req.SessionID) == "" {
		return "", fmt.Errorf("empty sessionId")
	}
	if strings.TrimSpace(req.TargetLang) == "" {
		return "", fmt.Errorf("empty targetLang")
	}

	provider, st, err := c.resolveProvider(ctx, req.Provider, req.Model)
	if err != nil {
		return "", err
	}

	style := effectiveStyle(req.Style, st.DefaultStyle)
	modelUsed := resolveModelUsed(st, req.Provider, req.Model)
	preserveMD := req.DisplayMode == model.DisplayModeBilingual

	userID := uuid.New().String()
	assistantID := uuid.New().String()

	userMsg := &model.Message{
		ID:                userID,
		SessionID:         req.SessionID,
		Role:              model.RoleUser,
		DisplayMode:       req.DisplayMode,
		OriginalContent:   content,
		SourceLang:        req.SourceLang,
		TargetLang:        req.TargetLang,
		Style:             style,
		ModelUsed:         modelUsed,
		OriginalMessageID: ptrIfNonEmpty(req.OriginalMessageID),
	}

	asstMsg := &model.Message{
		ID:                assistantID,
		SessionID:         req.SessionID,
		Role:              model.RoleAssistant,
		DisplayMode:       req.DisplayMode,
		OriginalContent:   "",
		TranslatedContent: "",
		SourceLang:        req.SourceLang,
		TargetLang:        req.TargetLang,
		Style:             style,
		ModelUsed:         modelUsed,
	}

	if err := c.reg.DoInTx(ctx, func(tx repository.Registry) error {
		if err := tx.Message().Insert(ctx, userMsg); err != nil {
			return err
		}
		if err := tx.Message().Insert(ctx, asstMsg); err != nil {
			return err
		}
		if err := tx.Session().UpdateTargetLang(ctx, req.SessionID, req.TargetLang); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return "", err
	}

	_ = c.reg.Settings().Upsert(ctx, "last_target_lang", req.TargetLang)

	runtime.EventsEmit(ctx, "translation:start", map[string]string{
		"messageId": assistantID,
		"sessionId": req.SessionID,
	})
	go c.runTranslationStream(ctx, req.SessionID, assistantID, content, req.SourceLang, req.TargetLang, style, preserveMD, provider)
	return assistantID, nil
}
