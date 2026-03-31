package model

type SessionStatus string

const (
	SessionStatusActive   SessionStatus = "active"
	SessionStatusPinned   SessionStatus = "pinned"
	SessionStatusArchived SessionStatus = "archived"
	SessionStatusDeleted  SessionStatus = "deleted"
)

type Session struct {
	ID         string        `json:"id"`
	Title      string        `json:"title"`
	Status     SessionStatus `json:"status"`
	TargetLang string        `json:"targetLang"`
	Style      string        `json:"style"`
	Model      string        `json:"model"`
	CreatedAt  string        `json:"createdAt"` // RFC3339 (Wails TS binding)
	UpdatedAt  string        `json:"updatedAt"`
}
