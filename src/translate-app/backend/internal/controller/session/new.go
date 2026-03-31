package session

import (
	"context"
	"fmt"

	"translate-app/internal/bridge"
	"translate-app/internal/controller/message"
	"translate-app/internal/model"
	"translate-app/internal/repository"
)

// Controller is session domain business API.
type Controller interface {
	GetSessions(ctx context.Context) ([]model.Session, error)
	CreateSessionAndSend(ctx context.Context, req bridge.CreateSessionAndSendRequest) (bridge.CreateSessionAndSendResult, error)
	RenameSession(ctx context.Context, id, title string) error
	UpdateSessionStatus(ctx context.Context, id, status string) error
}

type controller struct {
	reg  repository.Registry
	msgs message.Controller
}

// New constructs a session controller.
func New(reg repository.Registry, msgs message.Controller) Controller {
	return &controller{reg: reg, msgs: msgs}
}

func (c *controller) GetSessions(ctx context.Context) ([]model.Session, error) {
	return c.reg.Session().List(ctx)
}

func (c *controller) CreateSessionAndSend(ctx context.Context, req bridge.CreateSessionAndSendRequest) (bridge.CreateSessionAndSendResult, error) {
	return c.msgs.CreateSessionAndSend(ctx, req)
}

func (c *controller) RenameSession(ctx context.Context, id, title string) error {
	if title == "" {
		return fmt.Errorf("empty title")
	}
	return c.reg.Session().UpdateTitle(ctx, id, title)
}

func (c *controller) UpdateSessionStatus(ctx context.Context, id, status string) error {
	st := model.SessionStatus(status)
	if st != model.SessionStatusActive && st != model.SessionStatusPinned {
		return fmt.Errorf("invalid session status: %s", status)
	}
	return c.reg.Session().UpdateStatus(ctx, id, st)
}
