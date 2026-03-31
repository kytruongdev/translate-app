package message

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"translate-app/internal/bridge"
	"translate-app/internal/controller/file"
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

	c.log.Info("SessionCreated",
		"sessionId", sessionID, "targetLang", req.TargetLang, "style", style, "model", modelUsed)
	c.log.Info("MessageSent",
		"sessionId", sessionID, "msgId", assistantID, "charCount", len(content),
		"sourceLang", req.SourceLang, "targetLang", req.TargetLang, "style", style, "model", modelUsed)

	runtime.EventsEmit(ctx, "translation:start", map[string]string{
		"messageId": assistantID,
		"sessionId": sessionID,
	})
	go c.runTranslationStream(ctx, sessionID, assistantID, content, req.SourceLang, req.TargetLang, style, preserveMD, provider)
	return bridge.CreateSessionAndSendResult{SessionID: sessionID, MessageID: assistantID}, nil
}

// CreateEmptySession creates a session with no messages (e.g. attach file from start view before TranslateFile).
func (c *controller) CreateEmptySession(ctx context.Context, title, targetLang, style string) (string, error) {
	if strings.TrimSpace(targetLang) == "" {
		return "", fmt.Errorf("empty targetLang")
	}
	_, st, err := c.resolveProvider(ctx, "", "")
	if err != nil {
		return "", err
	}
	styleEff := effectiveStyle(style, st.DefaultStyle)
	modelUsed := resolveModelUsed(st, "", "")

	sessionID := uuid.New().String()
	t := strings.TrimSpace(title)
	if t == "" {
		t = "Phiên dịch"
	}

	sess := &model.Session{
		ID:         sessionID,
		Title:      truncateRunes(t, 80),
		Status:     model.SessionStatusActive,
		TargetLang: targetLang,
		Style:      string(styleEff),
		Model:      modelUsed,
	}
	if err := c.reg.Session().Create(ctx, sess); err != nil {
		return "", err
	}
	_ = c.reg.Settings().Upsert(ctx, "last_target_lang", targetLang)
	return sessionID, nil
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

	// File retranslate: user message stores display name ("📎 filename.ext"), not full source text.
	userOriginalContent := content
	if req.FileDisplayContent != "" {
		userOriginalContent = req.FileDisplayContent
	}

	userMsg := &model.Message{
		ID:                userID,
		SessionID:         req.SessionID,
		Role:              model.RoleUser,
		DisplayMode:       req.DisplayMode,
		OriginalContent:   userOriginalContent,
		SourceLang:        req.SourceLang,
		TargetLang:        req.TargetLang,
		Style:             style,
		ModelUsed:         modelUsed,
		OriginalMessageID: ptrIfNonEmpty(req.OriginalMessageID),
	}

	// File retranslate: copy fileId + store source text in originalContent (mirrors file pipeline).
	asstMsg := &model.Message{
		ID:                assistantID,
		SessionID:         req.SessionID,
		Role:              model.RoleAssistant,
		DisplayMode:       req.DisplayMode,
		OriginalContent:   func() string { if req.FileID != "" { return content }; return "" }(),
		TranslatedContent: "",
		FileID:            ptrIfNonEmpty(req.FileID),
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

	if req.OriginalMessageID != "" {
		c.log.Info("RetranslateTriggered",
			"sessionId", req.SessionID, "originalMsgId", req.OriginalMessageID,
			"newMsgId", assistantID, "style", style, "targetLang", req.TargetLang, "model", modelUsed)
	} else {
		c.log.Info("MessageSent",
			"sessionId", req.SessionID, "msgId", assistantID, "charCount", len(content),
			"sourceLang", req.SourceLang, "targetLang", req.TargetLang, "style", style, "model", modelUsed)
	}

	runtime.EventsEmit(ctx, "translation:start", map[string]string{
		"messageId": assistantID,
		"sessionId": req.SessionID,
	})
	// File retranslate: emit file:source.
	// For DOCX files, omit markdown (FE shows FileTranslationCard, not bilingual view).
	// For PDF files, include markdown so FE can open auto-fullscreen for heavy content.
	if req.FileID != "" {
		fileSourcePayload := map[string]interface{}{
			"sessionId":          req.SessionID,
			"assistantMessageId": assistantID,
		}
		if f, err2 := c.reg.File().GetByID(ctx, req.FileID); err2 == nil && f != nil && f.FileType != "docx" {
			fileSourcePayload["markdown"] = content
		}
		runtime.EventsEmit(ctx, "file:source", fileSourcePayload)
		go c.fileCtrl.RunRetranslateContent(ctx, file.RetranslateContentParams{
			SessionID:   req.SessionID,
			AssistantID: assistantID,
			FileID:      req.FileID,
			SourceMD:    content,
			TargetLang:  req.TargetLang,
			Style:       style,
			Provider:    provider,
		})
	} else {
		go c.runTranslationStream(ctx, req.SessionID, assistantID, content, req.SourceLang, req.TargetLang, style, preserveMD, provider)
	}
	return assistantID, nil
}
