package message

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

func defaultModelForProvider(_ string) string {
	return "gpt-4o-mini"
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

func sessionTitleFromRequest(title, content string) string {
	t := strings.TrimSpace(title)
	if t != "" {
		return truncateRunes(t, 80)
	}
	return titleFromContent(content)
}

func titleFromContent(content string) string {
	s := strings.TrimSpace(content)
	if s == "" {
		return "Phiên dịch"
	}
	lines := strings.Split(s, "\n")
	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, "#") {
		first = strings.TrimSpace(strings.TrimLeft(first, "#"))
	}
	return truncateRunes(first, 50)
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func ptrIfNonEmpty(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
