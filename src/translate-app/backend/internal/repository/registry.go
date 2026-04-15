package repository

import (
	"context"
	"database/sql"
	"fmt"

	"translate-app/internal/repository/sqlcgen"
)

type Registry interface {
	Session() SessionRepo
	Message() MessageRepo
	File() FileRepo
	Settings() SettingsRepo
	Glossary() GlossaryRepo
	DoInTx(ctx context.Context, fn func(Registry) error) error
}

type registry struct {
	db *sql.DB
	q  *sqlcgen.Queries
}

func New(db *sql.DB) Registry {
	return &registry{db: db, q: sqlcgen.New(db)}
}

func (r *registry) Session() SessionRepo   { return &sessionRepo{q: r.q} }
func (r *registry) Message() MessageRepo   { return &messageRepo{q: r.q, db: r.db} }
func (r *registry) File() FileRepo         { return &fileRepo{q: r.q} }
func (r *registry) Settings() SettingsRepo { return &settingsRepo{q: r.q} }
func (r *registry) Glossary() GlossaryRepo { return &glossaryRepo{q: r.q} }

func (r *registry) DoInTx(ctx context.Context, fn func(Registry) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	qtx := r.q.WithTx(tx)
	txReg := &registryTx{q: qtx, db: r.db}
	if err := fn(txReg); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

type registryTx struct {
	q  *sqlcgen.Queries
	db *sql.DB
}

func (r *registryTx) Session() SessionRepo   { return &sessionRepo{q: r.q} }
func (r *registryTx) Message() MessageRepo   { return &messageRepo{q: r.q, db: r.db} }
func (r *registryTx) File() FileRepo         { return &fileRepo{q: r.q} }
func (r *registryTx) Settings() SettingsRepo { return &settingsRepo{q: r.q} }
func (r *registryTx) Glossary() GlossaryRepo { return &glossaryRepo{q: r.q} }
func (r *registryTx) DoInTx(context.Context, func(Registry) error) error {
	return fmt.Errorf("nested transactions not supported")
}
