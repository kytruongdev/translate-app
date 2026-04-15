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
	UpdateExtracted(ctx context.Context, id, sourcePath string, charCount, pageCount int) error
	UpdateTranslated(ctx context.Context, id, sourcePath, translatedPath string, charCount, pageCount int, modelUsed, outputFormat string) error
	GetByID(ctx context.Context, id string) (*model.File, error)
	DeleteByID(ctx context.Context, id string) error
	ListCancelledIDsBySession(ctx context.Context, sessionID string) ([]string, error)
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
		OutputFormat:   f.OutputFormat,
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

func (r *fileRepo) UpdateExtracted(ctx context.Context, id, sourcePath string, charCount, pageCount int) error {
	return r.q.UpdateFileExtracted(ctx, sqlcgen.UpdateFileExtractedParams{
		SourcePath: sqlNullStr(sourcePath),
		CharCount:  sqlNullInt64(charCount),
		PageCount:  sqlNullInt64(pageCount),
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
		ID:         id,
	})
}

func (r *fileRepo) UpdateTranslated(ctx context.Context, id, sourcePath, translatedPath string, charCount, pageCount int, modelUsed, outputFormat string) error {
	return r.q.UpdateFileTranslated(ctx, sqlcgen.UpdateFileTranslatedParams{
		SourcePath:     sqlNullStr(sourcePath),
		TranslatedPath: sqlNullStr(translatedPath),
		Status:         "done",
		CharCount:      sqlNullInt64(charCount),
		PageCount:      sqlNullInt64(pageCount),
		ModelUsed:      sqlNullStr(modelUsed),
		OutputFormat:   outputFormat,
		UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
		ID:             id,
	})
}

func (r *fileRepo) DeleteByID(ctx context.Context, id string) error {
	return r.q.DeleteFileByID(ctx, id)
}

func (r *fileRepo) ListCancelledIDsBySession(ctx context.Context, sessionID string) ([]string, error) {
	return r.q.GetCancelledFileIdsBySession(ctx, sessionID)
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
