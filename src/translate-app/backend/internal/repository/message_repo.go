package repository

import (
	"context"
	"database/sql"
	"time"

	"translate-app/internal/model"
	"translate-app/internal/repository/sqlcgen"
)

type MessageRepo interface {
	Insert(ctx context.Context, msg *model.Message) error
	ListByCursor(ctx context.Context, sessionID string, cursor, limit int) ([]model.Message, error)
	UpdateTranslated(ctx context.Context, id, translated string, tokens int) error
	GetByID(ctx context.Context, id string) (*model.Message, error)
}

type messageRepo struct {
	q *sqlcgen.Queries
}

func (r *messageRepo) Insert(ctx context.Context, msg *model.Message) error {
	raw, err := r.q.GetMaxDisplayOrder(ctx, msg.SessionID)
	if err != nil {
		return err
	}
	maxOrder, err := maxOrderFromSQL(raw)
	if err != nil {
		return err
	}
	msg.DisplayOrder = int(maxOrder) + 1
	now := time.Now().UTC().Format(time.RFC3339)
	var fileID sql.NullString
	if msg.FileID != nil {
		fileID = sql.NullString{String: *msg.FileID, Valid: true}
	}
	var orig sql.NullString
	if msg.OriginalMessageID != nil {
		orig = sql.NullString{String: *msg.OriginalMessageID, Valid: true}
	}
	return r.q.InsertMessage(ctx, sqlcgen.InsertMessageParams{
		ID:                msg.ID,
		SessionID:         msg.SessionID,
		Role:              string(msg.Role),
		DisplayOrder:      int64(msg.DisplayOrder),
		DisplayMode:       string(msg.DisplayMode),
		OriginalContent:   msg.OriginalContent,
		TranslatedContent: sqlNullStr(msg.TranslatedContent),
		FileID:            fileID,
		SourceLang:        sqlNullStr(msg.SourceLang),
		TargetLang:        sqlNullStr(msg.TargetLang),
		Style:             sqlNullStr(string(msg.Style)),
		ModelUsed:         sqlNullStr(msg.ModelUsed),
		OriginalMessageID: orig,
		Tokens:            sqlNullInt64(msg.Tokens),
		CreatedAt:         now,
		UpdatedAt:         now,
	})
}

func (r *messageRepo) ListByCursor(ctx context.Context, sessionID string, cursor, limit int) ([]model.Message, error) {
	c := int64(cursor)
	rows, err := r.q.GetMessagesBySessionCursor(ctx, sqlcgen.GetMessagesBySessionCursorParams{
		SessionID:    sessionID,
		Cursor:       c,
		CursorBefore: c,
		RowLimit:     int64(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]model.Message, 0, len(rows))
	for _, row := range rows {
		m, err := messageFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func (r *messageRepo) UpdateTranslated(ctx context.Context, id, translated string, tokens int) error {
	return r.q.UpdateMessageTranslated(ctx, sqlcgen.UpdateMessageTranslatedParams{
		TranslatedContent: sql.NullString{String: translated, Valid: true},
		Tokens:            sql.NullInt64{Int64: int64(tokens), Valid: true},
		UpdatedAt:         time.Now().UTC().Format(time.RFC3339),
		ID:                id,
	})
}

func (r *messageRepo) GetByID(ctx context.Context, id string) (*model.Message, error) {
	row, err := r.q.GetMessageById(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	m, err := messageFromRow(row)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
