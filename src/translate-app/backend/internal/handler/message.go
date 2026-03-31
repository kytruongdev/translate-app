package handler

// GetMessages loads a page of messages for a session.
func (a *App) GetMessages(sessionID string, cursor int, limit int) (*MessagesPage, error) {
	return a.ctrl.Message.GetMessages(a.appCtx(), sessionID, cursor, limit)
}

// SendMessage sends user text and starts translation stream — E4.
func (a *App) SendMessage(req SendRequest) (string, error) {
	return a.ctrl.Message.SendMessage(a.appCtx(), req)
}

// SearchMessages searches messages across all sessions.
func (a *App) SearchMessages(query string) ([]SearchResult, error) {
	return a.ctrl.Message.SearchMessages(a.appCtx(), query)
}
