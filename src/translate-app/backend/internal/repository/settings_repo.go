package repository

import (
	"context"
	"database/sql"
	"time"

	"translate-app/internal/repository/sqlcgen"
)

type SettingsRepo interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Upsert(ctx context.Context, key, value string) error
	GetAll(ctx context.Context) (map[string]string, error)
}

type settingsRepo struct {
	q *sqlcgen.Queries
}

func (r *settingsRepo) Get(ctx context.Context, key string) (string, bool, error) {
	row, err := r.q.GetSetting(ctx, key)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	return row.Value, true, nil
}

func (r *settingsRepo) Upsert(ctx context.Context, key, value string) error {
	return r.q.UpsertSetting(ctx, sqlcgen.UpsertSettingParams{
		Key:       key,
		Value:     value,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func (r *settingsRepo) GetAll(ctx context.Context) (map[string]string, error) {
	rows, err := r.q.GetAllSettings(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(rows))
	for _, row := range rows {
		m[row.Key] = row.Value
	}
	return m, nil
}
