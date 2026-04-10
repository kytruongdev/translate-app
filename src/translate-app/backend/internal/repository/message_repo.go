package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"translate-app/internal/bridge"
	"translate-app/internal/model"
	"translate-app/internal/repository/sqlcgen"
)

type MessageRepo interface {
	Insert(ctx context.Context, msg *model.Message) error
	ListByCursor(ctx context.Context, sessionID string, cursor, limit int) ([]model.Message, error)
	UpdateTranslated(ctx context.Context, id, translated string, tokens int) error
	UpdateOriginalContent(ctx context.Context, id, original string) error
	UpdateSourceLang(ctx context.Context, id, sourceLang string) error
	GetByID(ctx context.Context, id string) (*model.Message, error)
	DeleteByFileID(ctx context.Context, fileID string) error
	SearchMessages(ctx context.Context, query string) ([]bridge.SearchResult, error)
}

type messageRepo struct {
	q  *sqlcgen.Queries
	db *sql.DB
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
	const q = `
		SELECT m.id, m.session_id, m.role, m.display_order, m.display_mode,
		       m.original_content, m.translated_content, m.file_id, m.source_lang, m.target_lang,
		       m.style, m.model_used, m.original_message_id, m.tokens, m.created_at, m.updated_at,
		       COALESCE(f.file_size, 0) AS file_size
		FROM messages m
		LEFT JOIN files f ON f.id = m.file_id
		WHERE m.session_id = ?
		  AND (? = 0 OR m.display_order < ?)
		ORDER BY m.display_order DESC
		LIMIT ?`
	c := int64(cursor)
	rows, err := r.db.QueryContext(ctx, q, sessionID, c, c, int64(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Message, 0, limit)
	for rows.Next() {
		var row sqlcgen.GetMessageByIdRow
		if err := rows.Scan(
			&row.ID, &row.SessionID, &row.Role, &row.DisplayOrder, &row.DisplayMode,
			&row.OriginalContent, &row.TranslatedContent, &row.FileID, &row.SourceLang, &row.TargetLang,
			&row.Style, &row.ModelUsed, &row.OriginalMessageID, &row.Tokens, &row.CreatedAt, &row.UpdatedAt,
			&row.FileSize,
		); err != nil {
			return nil, err
		}
		m, err := messageFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *messageRepo) UpdateTranslated(ctx context.Context, id, translated string, tokens int) error {
	return r.q.UpdateMessageTranslated(ctx, sqlcgen.UpdateMessageTranslatedParams{
		TranslatedContent: sql.NullString{String: translated, Valid: true},
		Tokens:            sql.NullInt64{Int64: int64(tokens), Valid: true},
		UpdatedAt:         time.Now().UTC().Format(time.RFC3339),
		ID:                id,
	})
}

func (r *messageRepo) UpdateSourceLang(ctx context.Context, id, sourceLang string) error {
	return r.q.UpdateMessageSourceLang(ctx, sqlcgen.UpdateMessageSourceLangParams{
		SourceLang: sqlNullStr(sourceLang),
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
		ID:         id,
	})
}

func (r *messageRepo) UpdateOriginalContent(ctx context.Context, id, original string) error {
	return r.q.UpdateMessageOriginalContent(ctx, sqlcgen.UpdateMessageOriginalContentParams{
		OriginalContent: original,
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
		ID:              id,
	})
}

func (r *messageRepo) DeleteByFileID(ctx context.Context, fileID string) error {
	return r.q.DeleteMessagesByFileID(ctx, sqlNullStr(fileID))
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

func (r *messageRepo) SearchMessages(ctx context.Context, query string) ([]bridge.SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return []bridge.SearchResult{}, nil
	}
	const searchSQL = `
		SELECT m.id, m.session_id, m.role, m.original_content, m.translated_content, m.created_at,
		       s.title AS session_title
		FROM messages m
		JOIN sessions s ON s.id = m.session_id
		WHERE s.status NOT IN ('deleted', 'archived')
		  AND (
		    LOWER(m.original_content)   LIKE '%' || LOWER(?) || '%'
		    OR LOWER(m.translated_content) LIKE '%' || LOWER(?) || '%'
		  )
		ORDER BY m.created_at DESC
		LIMIT 50
	`
	rows, err := r.db.QueryContext(ctx, searchSQL, query, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []bridge.SearchResult
	for rows.Next() {
		var (
			id, sessionID, role, originalContent, createdAt, sessionTitle string
			translatedContent                                              sql.NullString
		)
		if err := rows.Scan(&id, &sessionID, &role, &originalContent, &translatedContent, &createdAt, &sessionTitle); err != nil {
			return nil, err
		}
		content := originalContent
		if translatedContent.Valid && translatedContent.String != "" {
			content = translatedContent.String
		}
		results = append(results, bridge.SearchResult{
			MessageID:    id,
			SessionID:    sessionID,
			SessionTitle: sessionTitle,
			Role:         role,
			Snippet:      extractSnippet(content, query),
			CreatedAt:    createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		return []bridge.SearchResult{}, nil
	}
	return results, nil
}

func extractSnippet(content, query string) string {
	const maxLen = 120
	content = strings.ReplaceAll(content, "\n", " ")
	if len(content) <= maxLen {
		return content
	}
	lower := strings.ToLower(content)
	idx := strings.Index(lower, strings.ToLower(query))
	if idx == -1 {
		return content[:maxLen] + "…"
	}
	start := idx - 40
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + 60
	if end > len(content) {
		end = len(content)
	}
	snippet := content[start:end]
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(content) {
		snippet += "…"
	}
	return snippet
}
