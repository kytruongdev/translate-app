package bridge

import "translate-app/internal/model"

// CreateSessionAndSendResult — returned after DB commit; translation stream starts asynchronously.
type CreateSessionAndSendResult struct {
	SessionID string `json:"sessionId"`
	MessageID string `json:"messageId"` // assistant message id (streaming target)
}

// CreateSessionAndSendRequest — atomic create session + first message (IPC §7.3).
type CreateSessionAndSendRequest struct {
	Title       string            `json:"title,omitempty"`
	Content     string            `json:"content"`
	DisplayMode model.DisplayMode `json:"displayMode"`
	SourceLang  string            `json:"sourceLang"`
	TargetLang  string            `json:"targetLang"`
	Style       string            `json:"style,omitempty"`
}

// SendRequest — subsequent messages.
type SendRequest struct {
	SessionID          string            `json:"sessionId"`
	Content            string            `json:"content"`
	DisplayMode        model.DisplayMode `json:"displayMode"`
	SourceLang         string            `json:"sourceLang"`
	TargetLang         string            `json:"targetLang"`
	Style              string            `json:"style,omitempty"`
	OriginalMessageID  string            `json:"originalMessageId,omitempty"`
	Provider           string            `json:"provider,omitempty"`
	Model              string            `json:"model,omitempty"`
	// File retranslate: copy fileId to new assistant message + emit file:source for auto-fullscreen.
	FileID             string            `json:"fileId,omitempty"`
	// File retranslate: stored as user message originalContent for bilingualFileTitle display ("📎 filename.ext").
	FileDisplayContent string            `json:"fileDisplayContent,omitempty"`
}

// FileRequest — file translation.
type FileRequest struct {
	SessionID  string `json:"sessionId"`
	FilePath   string `json:"filePath"`
	TargetLang string `json:"targetLang,omitempty"` // bắt buộc từ UI; cập nhật session để khớp ngôn ngữ đích đang chọn
	Style      string `json:"style,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
}

// FileInfo — ReadFileInfo response.
type FileInfo struct {
	Name              string  `json:"name"`
	Type              string  `json:"type"`
	FileSize          int64   `json:"fileSize"`
	PageCount         int     `json:"pageCount,omitempty"`
	CharCount         int     `json:"charCount"`
	IsScanned         bool    `json:"isScanned,omitempty"`
	EstimatedChunks   int     `json:"estimatedChunks"`
	EstimatedMinutes  int     `json:"estimatedMinutes"`
}

// FileContent — lazy markdown from disk.
type FileContent struct {
	SourceMarkdown     string `json:"sourceMarkdown"`
	TranslatedMarkdown string `json:"translatedMarkdown"`
}

// FileResult — file:done event payload.
type FileResult struct {
	FileID     string `json:"fileId"`
	FileName   string `json:"fileName"`
	FileType   string `json:"fileType"`
	CharCount  int    `json:"charCount"`
	PageCount  int    `json:"pageCount"`
	TokensUsed int    `json:"tokensUsed"`
}

// MessagesPage — paginated messages.
type MessagesPage struct {
	Messages         []model.Message `json:"messages"`
	NextCursor       int             `json:"nextCursor"`
	HasMore          bool            `json:"hasMore"`
	CancelledFileIds []string        `json:"cancelledFileIds"`
}

// FileProgress — file:progress event.
type FileProgress struct {
	Chunk   int `json:"chunk"`
	Total   int `json:"total"`
	Percent int `json:"percent"`
}

// SearchResult — one hit from SearchMessages.
type SearchResult struct {
	MessageID    string `json:"messageId"`
	SessionID    string `json:"sessionId"`
	SessionTitle string `json:"sessionTitle"`
	Role         string `json:"role"`
	Snippet      string `json:"snippet"`
	CreatedAt    string `json:"createdAt"`
}
