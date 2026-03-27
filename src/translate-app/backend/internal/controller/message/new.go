package message

import (
	"context"
	"errors"
	"strings"

	"translate-app/config"
	"translate-app/internal/bridge"
	"translate-app/internal/controller/file"
	"translate-app/internal/model"
	"translate-app/internal/repository"
)

// Controller is message domain business API.
type Controller interface {
	GetMessages(ctx context.Context, sessionID string, cursor, limit int) (*bridge.MessagesPage, error)
	SendMessage(ctx context.Context, req bridge.SendRequest) (string, error)
	CreateSessionAndSend(ctx context.Context, req bridge.CreateSessionAndSendRequest) (bridge.CreateSessionAndSendResult, error)
	// CreateEmptySession creates a session row with no messages (e.g. file translation from start view).
	CreateEmptySession(ctx context.Context, title, targetLang, style string) (string, error)
	// CopyTranslationText returns trimmed translated text for an assistant message (clipboard set in handler).
	CopyTranslationText(ctx context.Context, messageID string) (string, error)
}

type controller struct {
	reg      repository.Registry
	keys     *config.APIKeys
	fileCtrl file.Controller
}

// New constructs a message controller.
func New(reg repository.Registry, keys *config.APIKeys, fileCtrl file.Controller) Controller {
	if keys == nil {
		keys = &config.APIKeys{}
	}
	return &controller{reg: reg, keys: keys, fileCtrl: fileCtrl}
}

func (c *controller) GetMessages(ctx context.Context, sessionID string, cursor, limit int) (*bridge.MessagesPage, error) {
	if limit <= 0 {
		limit = 20 // khớp FE MESSAGE_PAGE_SIZE — trang đầu = tin mới nhất (ListByCursor DESC)
	}
	rows, err := c.reg.Message().ListByCursor(ctx, sessionID, cursor, limit)
	if err != nil {
		return nil, err
	}
	cancelledIDs, _ := c.reg.File().ListCancelledIDsBySession(ctx, sessionID)
	if cancelledIDs == nil {
		cancelledIDs = []string{}
	}

	if len(rows) == 0 {
		return &bridge.MessagesPage{Messages: []model.Message{}, NextCursor: 0, HasMore: false, CancelledFileIds: cancelledIDs}, nil
	}

	msgs := make([]model.Message, len(rows))
	copy(msgs, rows)

	minOrder := rows[len(rows)-1].DisplayOrder
	var nextCursor int
	var hasMore bool
	if len(rows) < limit {
		hasMore = false
		nextCursor = 0
	} else {
		older, err := c.reg.Message().ListByCursor(ctx, sessionID, minOrder, 1)
		if err != nil {
			return nil, err
		}
		hasMore = len(older) > 0
		if hasMore {
			nextCursor = minOrder
		}
	}

	return &bridge.MessagesPage{
		Messages:         msgs,
		NextCursor:       nextCursor,
		HasMore:          hasMore,
		CancelledFileIds: cancelledIDs,
	}, nil
}

func (c *controller) CopyTranslationText(ctx context.Context, messageID string) (string, error) {
	m, err := c.reg.Message().GetByID(ctx, messageID)
	if err != nil {
		return "", err
	}
	if m.Role != model.RoleAssistant {
		return "", errors.New("only assistant messages have a translation")
	}
	t := strings.TrimSpace(m.TranslatedContent)
	if t == "" {
		return "", errors.New("translation is empty")
	}
	return t, nil
}
