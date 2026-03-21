package handler

import (
	"context"

	"translate-app/internal/controller"
)

// App is the Wails-bound façade (IPC §7.1).
type App struct {
	ctx  context.Context
	ctrl *controller.Controllers
}

// New constructs the handler app and the Wails OnStartup callback (avoids exporting lifecycle to JS).
func New(ctrl *controller.Controllers) (*App, func(context.Context)) {
	a := &App{ctrl: ctrl}
	return a, func(ctx context.Context) {
		a.ctx = ctx
	}
}

func (a *App) appCtx() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
