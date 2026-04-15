package repository

import (
	"database/sql"
	"fmt"
	"strconv"

	"translate-app/internal/model"
	"translate-app/internal/repository/sqlcgen"
)

func maxOrderFromSQL(v interface{}) (int64, error) {
	if v == nil {
		return 0, nil
	}
	switch n := v.(type) {
	case int64:
		return n, nil
	case int:
		return int64(n), nil
	case []byte:
		return strconv.ParseInt(string(n), 10, 64)
	case string:
		return strconv.ParseInt(n, 10, 64)
	default:
		return 0, fmt.Errorf("unexpected max order type %T", v)
	}
}

func nullStr(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}

func sqlNullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func sqlNullInt64(n int) sql.NullInt64 {
	if n == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(n), Valid: true}
}

func sessionFromRow(r sqlcgen.Session) (model.Session, error) {
	return model.Session{
		ID:         r.ID,
		Title:      r.Title,
		Status:     model.SessionStatus(r.Status),
		TargetLang: nullStr(r.TargetLang),
		Style:      nullStr(r.Style),
		Model:      nullStr(r.Model),
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}, nil
}

func messageFromRow(r sqlcgen.GetMessageByIdRow) (model.Message, error) {
	var fileID *string
	if r.FileID.Valid {
		fileID = &r.FileID.String
	}
	var orig *string
	if r.OriginalMessageID.Valid {
		orig = &r.OriginalMessageID.String
	}
	tokens := 0
	if r.Tokens.Valid {
		tokens = int(r.Tokens.Int64)
	}
	translated := ""
	if r.TranslatedContent.Valid {
		translated = r.TranslatedContent.String
	}
	return model.Message{
		ID:                r.ID,
		SessionID:         r.SessionID,
		Role:              model.MessageRole(r.Role),
		DisplayOrder:      int(r.DisplayOrder),
		DisplayMode:       model.DisplayMode(r.DisplayMode),
		OriginalContent:   r.OriginalContent,
		TranslatedContent: translated,
		FileID:            fileID,
		SourceLang:        nullStr(r.SourceLang),
		TargetLang:        nullStr(r.TargetLang),
		Style:             model.TranslationStyle(nullStr(r.Style)),
		ModelUsed:         nullStr(r.ModelUsed),
		OriginalMessageID: orig,
		Tokens:            tokens,
		FileSize:          r.FileSize,
		CreatedAt:         r.CreatedAt,
		UpdatedAt:         r.UpdatedAt,
	}, nil
}

func fileFromRow(r sqlcgen.File) (model.File, error) {
	ch := 0
	if r.CharCount.Valid {
		ch = int(r.CharCount.Int64)
	}
	pc := 0
	if r.PageCount.Valid {
		pc = int(r.PageCount.Int64)
	}
	return model.File{
		ID:             r.ID,
		SessionID:      r.SessionID,
		FileName:       r.FileName,
		FileType:       r.FileType,
		FileSize:       r.FileSize,
		OriginalPath:   nullStr(r.OriginalPath),
		SourcePath:     nullStr(r.SourcePath),
		TranslatedPath: nullStr(r.TranslatedPath),
		PageCount:      pc,
		CharCount:      ch,
		Style:          model.TranslationStyle(nullStr(r.Style)),
		ModelUsed:      nullStr(r.ModelUsed),
		Status:         r.Status,
		ErrorMsg:       nullStr(r.ErrorMsg),
		OutputFormat:   r.OutputFormat,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}, nil
}
