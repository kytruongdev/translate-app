package settings

import (
	"context"

	"translate-app/internal/model"
	"translate-app/internal/repository"
)

// Controller is settings domain API.
type Controller interface {
	GetSettings(ctx context.Context) (*model.Settings, error)
	SaveSettings(ctx context.Context, s model.Settings) error
}

type controller struct {
	reg repository.Registry
}

// New constructs a settings controller.
func New(reg repository.Registry) Controller {
	return &controller{reg: reg}
}

func (c *controller) GetSettings(ctx context.Context) (*model.Settings, error) {
	m, err := c.reg.Settings().GetAll(ctx)
	if err != nil {
		return nil, err
	}
	s := model.SettingsFromKV(m)
	return &s, nil
}

func (c *controller) SaveSettings(ctx context.Context, s model.Settings) error {
	for k, v := range s.ToKV() {
		if err := c.reg.Settings().Upsert(ctx, k, v); err != nil {
			return err
		}
	}
	return nil
}
