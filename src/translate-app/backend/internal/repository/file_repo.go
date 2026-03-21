package repository

import (
	"context"
	"database/sql"
	"time"

	"translate-app/internal/model"
	"translate-app/internal/repository/sqlcgen"
)

type FileRepo interface {
	Insert(ctx context.Context, f *model.File) error
	UpdateStatus(ctx context.Context, id, status, errMsg string) error
	UpdateTranslated(ctx context.Context, id, sourcePath, translatedPath string, charCount, pageCount int, modelUsed string) error
	GetByID(ctx context.Context, id string) (*model.File, error)
}

type fileRepo struct {
	q *sqlcgen.Queries
}

func (r *fileRepo) Insert(ctx context.Context, f *model.File) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.q.InsertFile(ctx, sqlcgen.InsertFileParams{
		ID:             f.ID,
		SessionID:      f.SessionID,
		FileName:       f.FileName,
		FileType:       f.FileType,
		FileSize:       f.FileSize,
		OriginalPath:   sqlNullStr(f.OriginalPath),
		SourcePath:     sqlNullStr(f.SourcePath),
		TranslatedPath: sqlNullStr(f.TranslatedPath),
		CharCount:      sqlNullInt64(f.CharCount),
		PageCount:      sqlNullInt64(f.PageCount),
		Style:          sqlNullStr(string(f.Style)),
		ModelUsed:      sqlNullStr(f.ModelUsed),
		Status:         f.Status,
		ErrorMsg:       sqlNullStr(f.ErrorMsg),
		CreatedAt:      now,
		UpdatedAt:      now,
	})
}

func (r *fileRepo) UpdateStatus(ctx context.Context, id, status, errMsg string) error {
	return r.q.UpdateFileStatus(ctx, sqlcgen.UpdateFileStatusParams{
		Status:    status,
		ErrorMsg:  sqlNullStr(errMsg),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		ID:        id,
	})
}

func (r *fileRepo) UpdateTranslated(ctx context.Context, id, sourcePath, translatedPath string, charCount, pageCount int, modelUsed string) error {
	return r.q.UpdateFileTranslated(ctx, sqlcgen.UpdateFileTranslatedParams{
		SourcePath:     sqlNullStr(sourcePath),
		TranslatedPath: sqlNullStr(translatedPath),
		Status:         "done",
		CharCount:      sqlNullInt64(charCount),
		PageCount:      sqlNullInt64(pageCount),
		ModelUsed:      sqlNullStr(modelUsed),
		UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
		ID:             id,
	})
}

func (r *fileRepo) GetByID(ctx context.Context, id string) (*model.File, error) {
	row, err := r.q.GetFileById(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	f, err := fileFromRow(row)
	if err != nil {
		return nil, err
	}
	return &f, nil
}
