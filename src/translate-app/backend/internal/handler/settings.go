package handler

import "translate-app/internal/model"

// GetSettings loads persisted UI / AI defaults.
func (a *App) GetSettings() (*model.Settings, error) {
	return a.ctrl.Settings.GetSettings(a.appCtx())
}

// SaveSettings persists settings to SQLite.
func (a *App) SaveSettings(s model.Settings) error {
	return a.ctrl.Settings.SaveSettings(a.appCtx(), s)
}
