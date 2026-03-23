package handler

import "translate-app/internal/model"

// GetSessions returns non-deleted/non-archived sessions (pinned first).
func (a *App) GetSessions() ([]model.Session, error) {
	return a.ctrl.Session.GetSessions(a.appCtx())
}

// CreateSessionAndSend creates a session and sends the first message (atomic) — E4.
func (a *App) CreateSessionAndSend(req CreateSessionAndSendRequest) (CreateSessionAndSendResult, error) {
	return a.ctrl.Session.CreateSessionAndSend(a.appCtx(), req)
}

// CreateEmptySession creates a session with no messages (file upload from start view).
func (a *App) CreateEmptySession(title string, targetLang string, style string) (string, error) {
	return a.ctrl.Message.CreateEmptySession(a.appCtx(), title, targetLang, style)
}

// RenameSession updates session title.
func (a *App) RenameSession(id string, title string) error {
	return a.ctrl.Session.RenameSession(a.appCtx(), id, title)
}

// UpdateSessionStatus pins or unpins a session (V1: active | pinned).
func (a *App) UpdateSessionStatus(id string, status string) error {
	return a.ctrl.Session.UpdateSessionStatus(a.appCtx(), id, status)
}
