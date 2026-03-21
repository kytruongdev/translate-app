package model

type Settings struct {
	Theme            string           `json:"theme"`
	ActiveProvider   string           `json:"activeProvider"`
	ActiveModel      string           `json:"activeModel"`
	DefaultStyle     TranslationStyle `json:"defaultStyle"`
	LastTargetLang   string           `json:"lastTargetLang"`
}

// SettingsFromKV maps DB settings rows (snake_case keys) to Settings.
func SettingsFromKV(kv map[string]string) Settings {
	s := Settings{
		Theme:            "system",
		ActiveProvider:   "gemini",
		ActiveModel:      "gemini-2.0-flash",
		DefaultStyle:     StyleCasual,
		LastTargetLang:   "en-US",
	}
	if v := kv["theme"]; v != "" {
		s.Theme = v
	}
	if v := kv["active_provider"]; v != "" {
		s.ActiveProvider = v
	}
	if v := kv["active_model"]; v != "" {
		s.ActiveModel = v
	}
	if v := kv["active_style"]; v != "" {
		s.DefaultStyle = TranslationStyle(v)
	}
	if v := kv["last_target_lang"]; v != "" {
		s.LastTargetLang = v
	}
	return s
}

func (s Settings) ToKV() map[string]string {
	return map[string]string{
		"theme":              s.Theme,
		"active_provider":    s.ActiveProvider,
		"active_model":       s.ActiveModel,
		"active_style":       string(s.DefaultStyle),
		"last_target_lang":   s.LastTargetLang,
	}
}
