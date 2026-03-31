package model

type File struct {
	ID             string           `json:"id"`
	SessionID      string           `json:"sessionId"`
	FileName       string           `json:"fileName"`
	FileType       string           `json:"fileType"`
	FileSize       int64            `json:"fileSize"`
	OriginalPath   string           `json:"originalPath"`
	SourcePath     string           `json:"sourcePath"`
	TranslatedPath string           `json:"translatedPath"`
	PageCount      int              `json:"pageCount"`
	CharCount      int              `json:"charCount"`
	Style          TranslationStyle `json:"style"`
	ModelUsed      string           `json:"modelUsed"`
	Status         string           `json:"status"`
	ErrorMsg       string           `json:"errorMsg"`
	CreatedAt      string           `json:"createdAt"` // RFC3339
	UpdatedAt      string           `json:"updatedAt"`
}
