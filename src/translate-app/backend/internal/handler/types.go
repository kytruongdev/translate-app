package handler

import "translate-app/internal/bridge"

// IPC request/response types (aliases to shared bridge package for Wails bindings).
type (
	CreateSessionAndSendResult  = bridge.CreateSessionAndSendResult
	CreateSessionAndSendRequest = bridge.CreateSessionAndSendRequest
	SendRequest                 = bridge.SendRequest
	FileRequest                 = bridge.FileRequest
	FileInfo                    = bridge.FileInfo
	FileContent                 = bridge.FileContent
	FileResult                  = bridge.FileResult
	MessagesPage                = bridge.MessagesPage
	SearchResult                = bridge.SearchResult
)
