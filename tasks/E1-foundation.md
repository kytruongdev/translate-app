# E1 — Foundation & Infrastructure

> Tất cả tickets E1 là technical tasks (không có user-facing UI). Phải hoàn thành trước tất cả epics khác.

---

### BE-001 — Wails Project Scaffold + Clean Architecture

> **Epic:** E1 | **Type:** Task | **Size:** M

**Status:** `Todo`

**Mô tả:**
Setup toàn bộ Go backend với Wails v2, tạo đúng folder structure theo clean architecture. Đây là skeleton — chưa có logic thật.

**Acceptance Criteria:**
- [ ] `wails init` với Go template, frontend dir trỏ đến `../frontend`
- [ ] Folder structure đúng như `doc/architecture-document.md` Section 6.1:
  ```
  backend/
  ├── main.go
  ├── wails.json
  ├── Makefile
  ├── go.mod / go.sum
  ├── config/
  │   └── keys.go
  └── internal/
      ├── model/
      ├── handler/
      ├── controller/
      ├── repository/
      ├── gateway/
      └── infra/
  ```
- [ ] `go.mod` khai báo đủ dependencies: `github.com/wailsapp/wails/v2`, `modernc.org/sqlite`, `github.com/sqlc-dev/sqlc`, `google.golang.org/genai`, `github.com/sashabaranov/go-openai`
- [ ] `Makefile` có targets: `dev` (wails dev), `build` (wails build), `sqlc` (sqlc generate), `migrate`
- [ ] `wails dev` chạy được (dù app chưa có gì)
- [ ] `config/keys.go` có struct `APIKeys` với `GeminiKey` và `OpenAIKey` (placeholder values), file này trong `.gitignore`

**Technical Notes:**

**Go files cần tạo:**
- `main.go` — Wails entry, chỉ có `wails.Run()` placeholder
- `config/keys.go`:
  ```go
  package config

  type APIKeys struct {
      GeminiKey string
      OpenAIKey string
  }

  var Keys = APIKeys{
      GeminiKey: "AIza-REPLACE-ME",
      OpenAIKey:  "sk-REPLACE-ME",
  }
  ```
- Tất cả `internal/` packages tạo file `doc.go` placeholder để Go nhận diện package

**Depends on:** —

---

### BE-002 — SQLite Setup + Migrations

> **Epic:** E1 | **Type:** Task | **Size:** M

**Status:** `Todo`

**Mô tả:**
Setup SQLite connection (pure Go, no CGO) và migration system. DB tự tạo nếu chưa có, migration chạy khi app khởi động.

**Acceptance Criteria:**
- [ ] Dùng `modernc.org/sqlite` (pure Go, không CGO)
- [ ] DB file lưu tại `os.UserConfigDir()/TranslateApp/data.db` — tự tạo folder nếu chưa có
- [ ] `infra/db/sqlite.go` export `func Open() (*sql.DB, error)` — mở connection + chạy migrations
- [ ] Migration chạy theo thứ tự (`001_initial.sql`, `002_...sql`) mỗi khi app start
- [ ] Schema đúng theo `doc/architecture-document.md` Section 8.1:
  - Table `sessions` (id, title, status, target_lang, style, model, created_at, updated_at)
  - Table `messages` (id, session_id, role, display_order, display_mode, original_content, translated_content, file_id, source_lang, target_lang, style, model_used, original_message_id, tokens, created_at, updated_at)
  - Table `files` (id, session_id, file_name, file_type, file_size, original_path, source_path, translated_path, char_count, page_count, style, model_used, status, error_msg, created_at, updated_at)
  - Table `settings` (key, value, updated_at)
  - Indexes: `idx_messages_order` (UNIQUE), `idx_messages_session`, `idx_files_session`
- [ ] `settings` table được seed với default values khi chưa có: `theme=system`, `active_provider=gemini`, `active_model=gemini-2.0-flash`, `active_style=casual`, `last_target_lang=en-US`
- [ ] App data folder structure tạo sẵn `files/` subfolder

**Technical Notes:**

```go
// infra/db/sqlite.go
func Open() (*sql.DB, error) {
    dir, _ := os.UserConfigDir()
    appDir := filepath.Join(dir, "TranslateApp")
    os.MkdirAll(filepath.Join(appDir, "files"), 0755)
    dbPath := filepath.Join(appDir, "data.db")

    db, err := sql.Open("sqlite", dbPath)
    // enable WAL mode, foreign keys
    db.Exec("PRAGMA journal_mode=WAL")
    db.Exec("PRAGMA foreign_keys=ON")
    // run migrations
    runMigrations(db)
    // seed defaults
    seedSettings(db)
    return db, err
}
```

- Migration files đặt tại `internal/infra/db/migrations/001_initial.sql`
- Dùng `//go:embed migrations/*.sql` để embed vào binary

**Depends on:** BE-001

---

### BE-003 — sqlc Setup + Type-Safe Queries

> **Epic:** E1 | **Type:** Task | **Size:** M

**Status:** `Todo`

**Mô tả:**
Setup sqlc để generate type-safe Go code từ SQL queries. Mỗi domain có file query riêng.

**Acceptance Criteria:**
- [ ] `sqlc.yaml` config tại `backend/sqlc.yaml`, output vào `internal/repository/sqlc/`
- [ ] Query files tại `internal/repository/sqlc/queries/`:
  - `sessions.sql` — GetSessions, CreateSession, UpdateSessionTitle, UpdateSessionStatus, UpdateSessionTargetLang
  - `messages.sql` — InsertMessage, GetMessagesBySessionCursor, UpdateMessageTranslated, GetMessageById
  - `files.sql` — InsertFile, UpdateFileStatus, UpdateFileTranslated, GetFileById
  - `settings.sql` — GetSetting, UpsertSetting, GetAllSettings
- [ ] `make sqlc` generate thành công, không có lỗi
- [ ] Generated structs map đúng với `model/` structs (các field snake_case → camelCase)
- [ ] `GetSessions` query dùng `WHERE status NOT IN ('deleted', 'archived')` ORDER BY: pinned trước, rồi `updated_at DESC`
- [ ] `GetMessagesBySessionCursor`: nhận `cursor int` (display_order), `limit int`; nếu cursor = 0 → lấy batch mới nhất; nếu > 0 → lấy messages có `display_order < cursor`

**Technical Notes:**

```sql
-- sessions.sql
-- name: GetSessions :many
SELECT * FROM sessions
WHERE status NOT IN ('deleted', 'archived')
ORDER BY
  CASE WHEN status = 'pinned' THEN 0 ELSE 1 END ASC,
  updated_at DESC;

-- name: GetMessagesBySessionCursor :many
SELECT * FROM messages
WHERE session_id = ?
  AND (? = 0 OR display_order < ?)
ORDER BY display_order DESC
LIMIT ?;
```

**Depends on:** BE-002

---

### BE-004 — Repository Layer

> **Epic:** E1 | **Type:** Task | **Size:** L

**Status:** `Todo`

**Mô tả:**
Implement toàn bộ repository layer: Registry pattern, DoInTx, và tất cả domain repositories.

**Acceptance Criteria:**
- [ ] `repository/registry.go` export interface `Registry` với tất cả sub-repositories + `DoInTx(ctx, func) error`
- [ ] `registry.New(db *sql.DB) Registry` — constructor
- [ ] Session repository implement: `Create`, `List`, `UpdateTitle`, `UpdateStatus`, `UpdateTargetLang`
- [ ] Message repository implement: `Insert`, `ListByCursor`, `UpdateTranslated`, `GetByID`, `GetMaxDisplayOrder`
- [ ] File repository implement: `Insert`, `UpdateStatus`, `UpdateTranslated`, `GetByID`
- [ ] Settings repository implement: `Get(key string)`, `Upsert(key, value string)`, `GetAll() map[string]string`
- [ ] Tất cả timestamps lưu dạng ISO 8601 string (`time.RFC3339`)
- [ ] IDs dùng `uuid.New().String()` (thư viện `github.com/google/uuid`)
- [ ] `DoInTx` wrap trong `db.BeginTx`, rollback nếu error, commit nếu success

**Technical Notes:**

```go
// repository/registry.go
type Registry interface {
    Session() SessionRepo
    Message() MessageRepo
    File()    FileRepo
    Settings() SettingsRepo
    DoInTx(ctx context.Context, fn func(Registry) error) error
}

// repository/message/create.go — GetMaxDisplayOrder trước khi INSERT
func (r *repo) Insert(ctx context.Context, msg *model.Message) error {
    maxOrder, _ := r.q.GetMaxDisplayOrder(ctx, msg.SessionID)
    msg.DisplayOrder = maxOrder + 1
    // ... INSERT
}
```

**Depends on:** BE-003

---

### BE-005 — AI Provider: Gemini Flash

> **Epic:** E1 | **Type:** Task | **Size:** M

**Status:** `Todo`

**Mô tả:**
Implement `AIProvider` interface cho Gemini 2.0 Flash với streaming support.

**Acceptance Criteria:**
- [ ] `gateway/aiprovider.go` define interface:
  ```go
  type AIProvider interface {
      TranslateStream(ctx context.Context, text, from, to, style string, events chan<- StreamEvent) error
  }
  type StreamEvent struct {
      Type    string // "chunk" | "done" | "error"
      Content string
      Error   error
  }
  ```
- [ ] `gateway/gemini/new.go` implement AIProvider dùng `google.golang.org/genai`
- [ ] Stream từng token từ Gemini response, gửi qua `events` channel
- [ ] Retry logic: tối đa 3 lần, exponential backoff (1s → 2s → 4s), jitter ±20%
- [ ] Retry on: 429, 500, 502, 503. No retry on: 400, 401, 403
- [ ] Context cancellation: nếu `ctx.Done()` → dừng stream, gửi `StreamEvent{Type: "error"}`
- [ ] Prompt được build theo `sourceLang` + `style` + `targetLang` đúng theo `doc/architecture-document.md` Section 9.1
- [ ] Nếu `displayMode = "bilingual"` (structured content) → thêm Markdown preservation instruction vào prompt

**Technical Notes:**

```go
// gateway/gemini/new.go
type GeminiProvider struct {
    client *genai.Client
    model  string
}

func New(apiKey, model string) *GeminiProvider {
    client, _ := genai.NewClient(context.Background(), option.WithAPIKey(apiKey))
    return &GeminiProvider{client: client, model: model}
}

func (g *GeminiProvider) TranslateStream(ctx context.Context, text, from, to, style string, events chan<- StreamEvent) error {
    prompt := buildPrompt(from, to, style)
    // dùng client.GenerativeModel(g.model).GenerateContentStream()
    // mỗi response → events <- StreamEvent{Type: "chunk", Content: text}
    // done → events <- StreamEvent{Type: "done"}
}
```

- `buildPrompt()` — function riêng, nhận (sourceLang, targetLang, style, isMarkdown) → string
- Default model: `"gemini-2.0-flash"`

**Depends on:** BE-001

---

### BE-006 — AI Provider: Ollama (Offline)

> **Epic:** E1 | **Type:** Task | **Size:** S

**Status:** `Todo`

**Mô tả:**
Implement AIProvider cho Ollama dùng OpenAI-compatible API (tái dùng `go-openai` SDK).

**Acceptance Criteria:**
- [ ] `gateway/ollama/new.go` implement AIProvider interface
- [ ] Dùng `github.com/sashabaranov/go-openai` với `BaseURL = "http://localhost:11434/v1"`
- [ ] Streaming qua `CreateChatCompletionStream`
- [ ] Default model: `"qwen2.5:7b"`
- [ ] Không cần API key (truyền `"ollama"` placeholder)
- [ ] Nếu Ollama không chạy (connection refused) → gửi `StreamEvent{Type: "error", Error: ErrOllamaNotRunning}`
- [ ] Retry logic giống BE-005 nhưng không retry connection refused (chỉ retry 5xx)

**Depends on:** BE-005

---

### BE-007 — AI Provider: OpenAI GPT-4o-mini

> **Epic:** E1 | **Type:** Task | **Size:** S

**Status:** `Todo`

**Mô tả:**
Implement AIProvider cho OpenAI GPT-4o-mini. Dùng cho Retranslate popover khi user chọn GPT-4o-mini.

**Acceptance Criteria:**
- [ ] `gateway/openai/new.go` implement AIProvider interface
- [ ] Dùng `github.com/sashabaranov/go-openai`, endpoint mặc định
- [ ] Streaming qua `CreateChatCompletionStream`
- [ ] Default model: `"gpt-4o-mini"`
- [ ] Retry logic giống BE-005
- [ ] `gateway.New(settings, keys)` factory function — switch case trả đúng provider:
  - `"gemini"` → GeminiProvider
  - `"ollama"` → OllamaProvider
  - `"openai"` → OpenAIProvider
  - Per-message override: nếu `req.Provider != ""` → dùng provider đó thay vì global settings

**Depends on:** BE-005, BE-006

---

### BE-008 — Controller Layer Skeleton

> **Epic:** E1 | **Type:** Task | **Size:** M

**Status:** `Todo`

**Mô tả:**
Implement toàn bộ controller layer với đầy đủ interfaces và constructors. Logic thật sẽ implement ở các US tickets sau.

**Acceptance Criteria:**
- [ ] Mỗi domain có file `new.go` với interface + constructor:
  - `controller/session/new.go` — `SessionController` interface
  - `controller/message/new.go` — `MessageController` interface
  - `controller/file/new.go` — `FileController` interface
  - `controller/settings/new.go` — `SettingsController` interface
- [ ] `SessionController` interface:
  ```go
  type SessionController interface {
      GetSessions(ctx context.Context) ([]model.Session, error)
      CreateSessionAndSend(ctx context.Context, req handler.CreateSessionAndSendRequest) (string, error)
      RenameSession(ctx context.Context, id, title string) error
      UpdateSessionStatus(ctx context.Context, id, status string) error
  }
  ```
- [ ] `MessageController` interface:
  ```go
  type MessageController interface {
      GetMessages(ctx context.Context, sessionId string, cursor, limit int) (*handler.MessagesPage, error)
      SendMessage(ctx context.Context, req handler.SendRequest) (string, error)
  }
  ```
- [ ] `FileController` interface:
  ```go
  type FileController interface {
      OpenFileDialog(ctx context.Context) (string, error)
      ReadFileInfo(ctx context.Context, path string) (*handler.FileInfo, error)
      TranslateFile(ctx context.Context, req handler.FileRequest) error
      GetFileContent(ctx context.Context, fileId string) (*handler.FileContent, error)
      ExportFile(ctx context.Context, fileId, format string) (string, error)
  }
  ```
- [ ] `SettingsController` interface:
  ```go
  type SettingsController interface {
      GetSettings(ctx context.Context) (*model.Settings, error)
      SaveSettings(ctx context.Context, s model.Settings) error
  }
  ```
- [ ] Tất cả implementation stub return `errors.New("not implemented")` — sẽ fill sau

**Depends on:** BE-004

---

### BE-009 — Handler Layer + DI Wiring

> **Epic:** E1 | **Type:** Task | **Size:** M

**Status:** `Todo`

**Mô tả:**
Implement handler layer (Wails bridge adapter) và wire toàn bộ DI trong `main.go`.

**Acceptance Criteria:**
- [ ] `handler/new.go` define `App` struct với tất cả controller fields, export `New(...)` constructor
- [ ] `handler/types.go` define tất cả request/response types:
  - `CreateSessionAndSendRequest`, `SendRequest`, `FileRequest`, `FileInfo`, `FileContent`, `FileResult`, `MessagesPage`
- [ ] Mỗi handler file (`session.go`, `message.go`, `file.go`, `settings.go`) delegate sang controller tương ứng
- [ ] Method names trên `App` struct đúng với IPC contract (Section 7.1): `GetSessions`, `CreateSessionAndSend`, `RenameSession`, `UpdateSessionStatus`, `GetMessages`, `SendMessage`, `OpenFileDialog`, `ReadFileInfo`, `TranslateFile`, `GetFileContent`, `ExportMessage`, `ExportSession`, `ExportFile`, `CopyTranslation`, `GetSettings`, `SaveSettings`
- [ ] `main.go` wire đúng thứ tự:
  1. `infra/db.Open()` → `*sql.DB`
  2. `repository.New(db)` → `Registry`
  3. `gateway.New(settings, config.Keys)` → `AIProvider` (load settings trước)
  4. `controller.New(repo, gateway)` → controllers
  5. `handler.New(controllers)` → `*App`
  6. `wails.Run(options)` với `Bind: [app]`
- [ ] Wails bind đúng để FE nhận JS wrappers tự động trong `wailsjs/go/`

**Depends on:** BE-007, BE-008

---

### FE-001 — Frontend Project Setup

> **Epic:** E1 | **Type:** Task | **Size:** S

**Status:** `Todo`

**Mô tả:**
Setup React + Vite + TypeScript project trong `frontend/`, cài đủ dependencies.

**Acceptance Criteria:**
- [ ] Vite 6 + React 19 + TypeScript 5 — `vite.config.ts` có Wails-specific config
- [ ] `tsconfig.json` strict mode, path aliases: `@/` → `src/`
- [ ] Dependencies: `zustand`, `react-router-dom`, `turndown`, `turndown-plugin-gfm`
- [ ] `frontend/wailsjs/` — **không tạo tay**, được generate tự động bởi Wails CLI khi `wails dev` chạy
- [ ] `src/types/` có 3 files: `session.ts`, `file.ts`, `settings.ts` với đúng TypeScript interfaces từ Section 7.3
- [ ] `npm run dev` build thành công (dù app chưa có UI)

**Technical Notes:**

```typescript
// src/types/session.ts
export interface Session {
  id: string
  title: string
  status: 'active' | 'pinned' | 'archived' | 'deleted'
  targetLang: string
  style: 'casual' | 'business' | 'academic' | ''
  model: string
  createdAt: string
  updatedAt: string
}

export interface Message {
  id: string
  sessionId: string
  role: 'user' | 'assistant'
  displayOrder: number
  displayMode: 'bubble' | 'bilingual'
  originalContent: string
  translatedContent: string
  fileId: string | null
  sourceLang: string
  targetLang: string
  style: 'casual' | 'business' | 'academic'
  modelUsed: string
  originalMessageId: string | null
  tokens: number
  createdAt: string
  updatedAt: string
}

export type TranslationStyle = 'casual' | 'business' | 'academic'
export type SessionStatus = 'active' | 'pinned' | 'archived' | 'deleted'
```

**Depends on:** BE-001

---

### FE-002 — Zustand Stores Setup

> **Epic:** E1 | **Type:** Task | **Size:** S

**Status:** `Todo`

**Mô tả:**
Tạo 4 Zustand stores với đầy đủ interface và initial state. Actions sẽ được implement ở các US tickets.

**Acceptance Criteria:**
- [ ] `stores/session/sessionStore.ts` — `SessionStore` interface đúng với Section 5.2
- [ ] `stores/message/messageStore.ts` — `MessageStore` interface với `streamStatus: 'idle'|'pending'|'streaming'|'error'`, `streamingText`, `cursors`, `hasMore`, `messages`
- [ ] `stores/settings/settingsStore.ts` — `SettingsStore` với `theme`, `activeProvider`, `activeModel`, `defaultStyle`
- [ ] `stores/ui/uiStore.ts` — `UIStore` với `sidebarCollapsed`, `activeStyle`, `activeTargetLang`
- [ ] Tất cả stores export typed hook: `useSessionStore()`, `useMessageStore()`, `useSettingsStore()`, `useUIStore()`
- [ ] Initial state hợp lý: `activeTargetLang = 'en-US'`, `activeStyle = 'casual'`, `sidebarCollapsed = false`, `streamStatus = 'idle'`

**Depends on:** FE-001

---

### FE-003 — Wails Service Wrapper

> **Epic:** E1 | **Type:** Task | **Size:** S

**Status:** `Todo`

**Mô tả:**
Tạo `services/wailsService.ts` — typed wrapper cho tất cả `window.go.main.App.*` calls.

**Acceptance Criteria:**
- [ ] Wrap tất cả IPC methods từ Section 7.1 với đúng TypeScript types:
  ```typescript
  // services/wailsService.ts
  import * as Go from '../../wailsjs/go/main/App'

  export const WailsService = {
    getSessions: (): Promise<Session[]> => Go.GetSessions(),
    createSessionAndSend: (req: CreateSessionAndSendRequest): Promise<string> => Go.CreateSessionAndSend(req),
    renameSession: (id: string, title: string): Promise<void> => Go.RenameSession(id, title),
    updateSessionStatus: (id: string, status: string): Promise<void> => Go.UpdateSessionStatus(id, status),
    getMessages: (sessionId: string, cursor: number, limit: number): Promise<MessagesPage> => Go.GetMessages(sessionId, cursor, limit),
    sendMessage: (req: SendRequest): Promise<string> => Go.SendMessage(req),
    openFileDialog: (): Promise<string> => Go.OpenFileDialog(),
    readFileInfo: (path: string): Promise<FileInfo> => Go.ReadFileInfo(path),
    translateFile: (req: FileRequest): Promise<void> => Go.TranslateFile(req),
    getFileContent: (fileId: string): Promise<FileContent> => Go.GetFileContent(fileId),
    exportMessage: (id: string, format: string): Promise<string> => Go.ExportMessage(id, format),
    exportSession: (id: string, format: string): Promise<string> => Go.ExportSession(id, format),
    exportFile: (fileId: string, format: string): Promise<string> => Go.ExportFile(fileId, format),
    getSettings: (): Promise<Settings> => Go.GetSettings(),
    saveSettings: (s: Settings): Promise<void> => Go.SaveSettings(s),
  }
  ```
- [ ] Event subscriptions helper:
  ```typescript
  export const WailsEvents = {
    onTranslationStart: (cb: (payload: { messageId: string }) => void) => EventsOn('translation:start', cb),
    onTranslationChunk: (cb: (chunk: string) => void) => EventsOn('translation:chunk', cb),
    onTranslationDone: (cb: (msg: Message) => void) => EventsOn('translation:done', cb),
    onTranslationError: (cb: (err: string) => void) => EventsOn('translation:error', cb),
    onFileSource: (cb: (payload: { markdown: string }) => void) => EventsOn('file:source', cb),
    onFileProgress: (cb: (p: FileProgress) => void) => EventsOn('file:progress', cb),
    onFileChunkDone: (cb: (c: { chunkIndex: number; text: string }) => void) => EventsOn('file:chunk_done', cb),
    onFileDone: (cb: (r: FileResult) => void) => EventsOn('file:done', cb),
    onFileError: (cb: (err: string) => void) => EventsOn('file:error', cb),
  }
  ```
- [ ] Import `EventsOn` từ `wailsjs/runtime`

**Depends on:** FE-001, BE-009

---

### FE-004 — Global Styles & Design Tokens

> **Epic:** E1 | **Type:** Task | **Size:** S

**Status:** `Todo`

**Mô tả:**
Setup CSS Custom Properties (Material Design 3 tokens), typography, animations. Light + dark theme variables.

**Acceptance Criteria:**
- [ ] `styles/global.css` — tất cả CSS variables đúng với mockup (`--bg`, `--surface`, `--sidebar`, `--active`, `--input-bg`, `--card-bg`, `--text`, `--text-secondary`, `--text-tertiary`, `--primary`, `--accent`, `--shadow-*`, `--r-*`)
- [ ] `[data-theme="dark"]` override tất cả variables cho dark mode
- [ ] `styles/typography.css` — font stack: `"Google Sans Text", "Google Sans", Inter, system-ui`, font sizes, weights
- [ ] `styles/animations.css` — keyframes: `dialog-enter`, `popover-enter`, `overlay-enter`
- [ ] Theme apply: `document.documentElement.setAttribute('data-theme', theme)` khi load/change
- [ ] Avatar + bubble gradient variables (user = pink/rose, assistant = cyan/blue)

**Depends on:** FE-001
