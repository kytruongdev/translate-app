package file

import (
	"context"
	"strings"

	"translate-app/internal/gateway"
	"translate-app/internal/model"
)

func effectiveStyle(reqStyle string, def model.TranslationStyle) model.TranslationStyle {
	s := strings.ToLower(strings.TrimSpace(reqStyle))
	switch s {
	case "business":
		return model.StyleBusiness
	case "academic":
		return model.StyleAcademic
	case "casual":
		return model.StyleCasual
	default:
		return def
	}
}

func resolveModelUsed(settings model.Settings, overrideProvider, overrideModel string) string {
	if strings.TrimSpace(overrideProvider) != "" {
		if strings.TrimSpace(overrideModel) != "" {
			return strings.TrimSpace(overrideModel)
		}
		return defaultModelForProvider(overrideProvider)
	}
	return settings.ActiveModel
}

func defaultModelForProvider(name string) string {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "ollama":
		return "qwen2.5:7b"
	case "openai":
		return "gpt-4o-mini"
	default:
		return "gemini-2.0-flash"
	}
}

func (c *controller) resolveProvider(ctx context.Context, overrideProvider, overrideModel string) (gateway.AIProvider, model.Settings, error) {
	kv, err := c.reg.Settings().GetAll(ctx)
	if err != nil {
		return nil, model.Settings{}, err
	}
	st := model.SettingsFromKV(kv)
	if strings.TrimSpace(overrideProvider) != "" {
		p, err := gateway.ForProvider(overrideProvider, overrideModel, c.keys)
		return p, st, err
	}
	p, err := gateway.ForProvider(st.ActiveProvider, st.ActiveModel, c.keys)
	return p, st, err
}
