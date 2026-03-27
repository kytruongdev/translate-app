package model

type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
)

type TranslationStyle string

const (
	StyleCasual   TranslationStyle = "casual"
	StyleBusiness TranslationStyle = "business"
	StyleAcademic TranslationStyle = "academic"
)

type DisplayMode string

const (
	DisplayModeBubble    DisplayMode = "bubble"
	DisplayModeBilingual DisplayMode = "bilingual"
	DisplayModeFile      DisplayMode = "file"
)

type Message struct {
	ID                  string           `json:"id"`
	SessionID           string           `json:"sessionId"`
	Role                MessageRole      `json:"role"`
	DisplayOrder        int              `json:"displayOrder"`
	DisplayMode         DisplayMode      `json:"displayMode"`
	OriginalContent     string           `json:"originalContent"`
	TranslatedContent   string           `json:"translatedContent"`
	FileID              *string          `json:"fileId"`
	SourceLang          string           `json:"sourceLang"`
	TargetLang          string           `json:"targetLang"`
	Style               TranslationStyle `json:"style"`
	ModelUsed           string           `json:"modelUsed"`
	OriginalMessageID   *string          `json:"originalMessageId"`
	Tokens              int              `json:"tokens"`
	FileSize            int64            `json:"fileSize"`
	CreatedAt           string           `json:"createdAt"` // RFC3339
	UpdatedAt           string           `json:"updatedAt"`
}
