package repository

import (
	"context"
	"time"

	"translate-app/internal/model"
	"translate-app/internal/repository/sqlcgen"
)

type SessionRepo interface {
	Create(ctx context.Context, s *model.Session) error
	List(ctx context.Context) ([]model.Session, error)
	UpdateTitle(ctx context.Context, id, title string) error
	UpdateStatus(ctx context.Context, id string, status model.SessionStatus) error
	UpdateTargetLang(ctx context.Context, id, lang string) error
}

type sessionRepo struct {
	q *sqlcgen.Queries
}

func (r *sessionRepo) Create(ctx context.Context, s *model.Session) error {
	now := time.Now().UTC().Format(time.RFC3339)
	tl := sqlNullStr(s.TargetLang)
	return r.q.CreateSession(ctx, sqlcgen.CreateSessionParams{
		ID:         s.ID,
		Title:      s.Title,
		Status:     string(s.Status),
		TargetLang: tl,
		Style:      sqlNullStr(s.Style),
		Model:      sqlNullStr(s.Model),
		CreatedAt:  now,
		UpdatedAt:  now,
	})
}

func (r *sessionRepo) List(ctx context.Context) ([]model.Session, error) {
	rows, err := r.q.GetSessions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.Session, 0, len(rows))
	for _, row := range rows {
		s, err := sessionFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func (r *sessionRepo) UpdateTitle(ctx context.Context, id, title string) error {
	return r.q.UpdateSessionTitle(ctx, sqlcgen.UpdateSessionTitleParams{
		Title:     title,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		ID:        id,
	})
}

func (r *sessionRepo) UpdateStatus(ctx context.Context, id string, status model.SessionStatus) error {
	return r.q.UpdateSessionStatus(ctx, sqlcgen.UpdateSessionStatusParams{
		Status:    string(status),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		ID:        id,
	})
}

func (r *sessionRepo) UpdateTargetLang(ctx context.Context, id, lang string) error {
	return r.q.UpdateSessionTargetLang(ctx, sqlcgen.UpdateSessionTargetLangParams{
		TargetLang: sqlNullStr(lang),
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
		ID:         id,
	})
}
