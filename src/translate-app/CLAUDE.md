# TranslateApp — Project Reference

> File này dùng cho cả Claude lẫn developer đọc lại. Cập nhật khi có thay đổi lớn về architecture hoặc flow.

---

## 1. Business Overview

**TranslateApp** là desktop app dịch thuật AI dành cho người dùng cá nhân (primary target: người Việt).

**Core value:** Dịch văn bản và tài liệu (DOC/DOCX) với chất lượng cao, lưu lịch sử theo session, hỗ trợ nhiều AI provider.

**Tính năng chính:**
- Dịch văn bản qua chat interface (bubble hoặc bilingual view)
- Dịch toàn bộ file DOCX / DOC với progress tracking (DOC được convert sang DOCX qua Pandoc trước khi dịch)
- Quản lý lịch sử session (tìm kiếm, pin, archive, rename)
- Retranslate với language/style khác
- Export file đã dịch
- 2 AI provider: OpenAI, Ollama (local)
- 3 translation style: casual / business / academic

---

## 2. Business Flows

### Flow 1: Dịch văn bản (Text Translation)

```
User gõ / paste text → chọn ngôn ngữ đích → Send
  │
  ├─ [Session chưa có] → CreateSessionAndSend
  │     1. FE detect source lang (vi nếu có diacritic, "unknown" nếu không chắc)
  │     2. FE chọn displayMode: bubble (gõ tay / paste ngắn) | bilingual (paste có structure / > 2000 ký tự)
  │     3. IPC call → BE tạo atomic: session + user_msg + assistant_msg (empty) trong 1 transaction
  │     4. BE emit event `translation:start` {messageId, sessionId}
  │     5. BE goroutine: stream dịch → emit `translation:chunk` liên tục
  │     6. FE buffer chunks (coalescer debounce) → render streaming text
  │     7. BE xong → UpdateTranslated DB → emit `translation:done` (full Message object)
  │     8. FE finalizeStream: upsert message vào store
  │
  └─ [Session đã có] → SendMessage (bước 3-8 như trên, không tạo session mới)
```

### Flow 2: Dịch file (File Translation)

#### 2a. Chọn file & validate

```
User click attach icon → OpenFileDialog()
  → Native file picker, filter hiển thị: "Word (*.docx)" — chỉ gợi ý, không enforce ở OS level

HOẶC drag-drop file vào chat → FE nhận path

→ FE gọi ReadFileInfo(path)
```

**ReadFileInfo — validation & metadata (BE):**
```
1. path rỗng          → error
2. path không tồn tại → error
3. path là directory  → error
4. ext = .pdf         → error "PDF chưa được hỗ trợ ở phiên bản này"
5. ext ≠ .docx        → error "chỉ hỗ trợ DOCX"
6. ext = .docx        → đọc word/document.xml từ zip (limit 32MiB)
                         → extract text → đếm rune
                         → tính pageCount  = ceil(charCount / 2800)
                         → tính chunks     = ceil(charCount / 2500)
                         → tính minutes    = ceil(chunks / 2)
                         → trả về FileInfo {name, type, fileSize, pageCount, charCount,
                                            isScanned=false, estimatedChunks, estimatedMinutes}
```

FE hiển thị preview: tên file, dung lượng, số trang ước tính, thời gian ước tính.

#### 2b. Confirm & start

```
User chọn target lang + confirm → FE gọi TranslateFile(req)
```

**TranslateFile — validation (BE, thực hiện lại):**
```
1. sessionId rỗng          → error
2. filePath rỗng           → error
3. Gọi lại ReadFileInfo()  → revalidate (phòng file bị xóa giữa chừng)
4. pageCount > 200         → error "Tệp quá lớn (tối đa 200 trang)"
```

**BE atomic transaction:**
```
- Insert file row    (status = processing)
- Insert user_msg    (display_mode = file, original_content = "📎 {filename}")
- Insert assistant_msg (display_mode = file, original_content = "")
- Update session.target_lang
```

**BE emit** `translation:start` {messageId, sessionId} → FE hiển thị loading state.

**BE tạo cancellable context** → lưu `cancels[fileID]` để support cancel.

#### 2c. Pipeline dịch (goroutine)

```
runFileTranslate → runDocxTranslate:

  a. Extract text:
     - Nếu có bin/pandoc → dùng Pandoc (chất lượng cao hơn, handle tables/headings)
     - Fallback → custom regex XML parser (docxXMLToMarkdown)
     → Lưu source.md vào ~/.config/TranslateApp/files/{fileId}/source.md

  b. Detect source lang từ source.md:
     - Có ký tự tiếng Việt → "vi"
     - Không detect được → "auto"
     → Update source_lang của user_msg + assistant_msg trong DB

  c. Emit `file:source` {sessionId, assistantMessageId}
     (KHÔNG kèm markdown — FE render FileTranslationCard, không mở fullscreen)

  d. Parse DOCX XML (ParseDocx) → cấu trúc paragraphs giữ nguyên

  e. Chunk paragraphs (2500 rune/batch, KHÔNG cắt giữa paragraph):
     - Đoạn nhỏ gom lại đến đủ 2500 rune
     - Đoạn lớn hơn 2500 rune → batch riêng

  f. translateDocxFile — dịch concurrent batches:
     → Emit `file:progress` {chunk, total, percent} sau mỗi batch xong

  g. WriteTranslatedDocx → overwrite translated.docx (giữ nguyên XML structure,
     chỉ thay text trong <w:t> nodes)

  h. UpdateTranslated DB (assistant_msg.translated_content = sourceMD, tokens)
     UpdateFile DB    (status = done, source_path, translated_path, char_count, page_count)

  i. Emit `translation:done` (full Message object)
     Emit `file:done` {fileId, fileName, fileType, charCount, pageCount}
```

**Error / Cancel:**
```
- ctx bị cancel → status = cancelled → emit `file:cancelled`
- lỗi bất kỳ   → status = error     → emit `file:error` + `translation:error`
```

**File storage:** `~/.config/TranslateApp/files/{fileId}/source.md` và `translated.docx`

### Flow 3: Retranslate

```
User click "Retranslate" trên assistant message
  │
  1. Popover hiện ra: chọn target lang + style mới
  2. FE gọi SendMessage với:
     - originalMessageId = ID của message gốc
     - content = original_content của message gốc (hoặc source markdown nếu là file)
     - fileId (nếu là file retranslate)
  3. BE tạo cặp user_msg + assistant_msg mới (cùng session, display_order tiếp theo)
  4. [Text] → runTranslationStream bình thường
  5. [File] → emit file:source + RunRetranslateContent (re-run pipeline với provider/style mới)
```

### Flow 4: Load/Pagination Messages

```
Switch session → loadMessages(sessionId, cursor=0, limit=60)
  → BE trả MessagesPage {messages (newest 60), nextCursor, hasMore}
  → FE sort by displayOrder → render

Scroll lên đầu (infinite scroll) → loadMoreMessages(sessionId, cursor=prevCursor)
  → BE trả older messages → FE prepend + dedup
```

### Flow 5: Export file đã dịch

```
User click Export trên assistant message (file translation)
  │
  1. FE gọi ExportFile(fileId, format)
     - format param hiện tại bị ignore ở BE (chỉ support DOCX)
  2. BE lookup file record → kiểm tra translated_path tồn tại trên disk
  3. BE mở native Save Dialog, default filename = "{tên_gốc}_translated.docx"
  4. User chọn đường dẫn lưu (hoặc cancel → return "")
  5. BE copyFile(translated_path → savePath)
  6. Trả về savePath → FE hiển thị thông báo thành công
```

### Flow 6: Search

```
User click search icon → gõ keyword
  → SearchMessages(query) IPC call
  → BE: LIKE search trên original_content + translated_content, JOIN sessions
  → FE hiển thị results với snippet + session title
  → Click result → switch session + scroll đến message đó
```

---

## 3. Technical Architecture

### Desktop Framework

**Wails v2** — bundle Go backend + React frontend thành native binary (.exe / .app).
- FE chạy trong webview embedded, communicate với BE qua IPC (không phải HTTP)
- Wails tự generate TypeScript bindings từ Go methods

### Backend — Clean Architecture (Uncle Bob)

```
┌──────────────────────────────────────────────┐
│  Frameworks & Drivers                        │
│  internal/infra/   (SQLite, migrations)      │
│  Wails runtime     (IPC, events)             │
├──────────────────────────────────────────────┤
│  Interface Adapters                          │
│  internal/handler/     (Wails IPC facade)   │
│  internal/gateway/     (AI provider adapter)│
│  internal/repository/  (SQLite adapter)     │
├──────────────────────────────────────────────┤
│  Use Cases                                   │
│  internal/controller/  (business logic)     │
├──────────────────────────────────────────────┤
│  Entities                                    │
│  internal/model/       (domain structs)     │
└──────────────────────────────────────────────┘
```

**Dependency Rule:** Chỉ trỏ vào trong. `controller` không bao giờ import `handler` hay `infra`.

**Key patterns:**
- **Facade** — `handler/App` chỉ delegate sang controller, zero business logic
- **Repository + Unit of Work** — `Registry` interface aggregates all repos; `DoInTx(fn)` wrap transaction transparent
- **Strategy** — `AIProvider` interface, 3 implementations interchangeable
- **Constructor DI (manual)** — wire thủ công trong `main.go`, không dùng DI framework
- **DTOs tại boundary** — `internal/bridge/types.go` tách IPC types khỏi domain model

### Frontend — Flux via Zustand

```
App.tsx (Smart Component / Orchestrator)
  ├── sessionStore    — sessions list, activeSessionId
  ├── messageStore    — messages per session, cursor pagination, streaming state
  ├── settingsStore   — theme, provider, model, style
  └── uiStore         — ephemeral: sidebar, modals, cancelled files

WailsService           — Adapter layer wrap Wails IPC calls
WailsEvents            — Event listeners (translation:*, file:*)
```

**Key patterns:**
- 4 stores tách biệt theo domain (không god store)
- `App.tsx` là orchestrator duy nhất subscribe WailsEvents → dispatch store actions
- Components còn lại là **presentational** — chỉ nhận props
- **Optimistic UI** — append message ngay, finalize sau khi stream xong
- **Streaming coalescer** — debounce chunks phía client, tránh re-render liên tục

### FE ↔ BE Communication

**Không có HTTP API.** Có 2 kênh:

| Kênh | Hướng | Dùng cho |
|---|---|---|
| **IPC call** (Wails binding) | FE → BE | Request/response: CRUD, get data |
| **Wails Events** | BE → FE (push) | Streaming: chunks, progress, done, error |

**IPC Methods** (expose qua `handler/App`):
```
Sessions:  GetSessions, CreateSessionAndSend, CreateEmptySession, RenameSession, UpdateSessionStatus
Messages:  GetMessages, SendMessage, SearchMessages, CopyTranslation
Files:     OpenFileDialog, ReadFileInfo, TranslateFile, CancelFileTranslate, GetFileContent, ExportFile
Settings:  GetSettings, SaveSettings
```

**Wails Events** (BE emit → FE listen):
```
translation:start    {messageId, sessionId}
translation:chunk    string (raw chunk)
translation:done     Message (full object)
translation:error    string

file:source          {sessionId, assistantMessageId}
file:progress        {chunk, total, percent}
file:chunk_done      {chunkIndex, text}
file:done            FileResult
file:error           string
file:cancelled       {fileId, sessionId}
```

### AI Provider Gateway

```go
type AIProvider interface {
    TranslateStream(ctx, text, from, to, style, preserveMarkdown, events chan<- StreamEvent) error
    TranslateBatchStream(ctx, text, from, to, style, events chan<- StreamEvent) error  // DOCX batches
    MaxBatchConcurrency() int  // Ollama=1, cloud providers=4-10
}
```

Factory: `gateway.ForProvider("openai"|"ollama", modelName, keys)`

---

## 4. Database Schema

**DB path:** `~/.config/TranslateApp/data.db` (SQLite, WAL mode, foreign_keys ON)

### sessions
```sql
CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,           -- UUID v4
    title       TEXT NOT NULL,              -- auto-generated từ nội dung đầu tiên (truncate 80 chars)
    status      TEXT NOT NULL DEFAULT 'active'
                CHECK (status IN ('active','pinned','archived','deleted')),
    target_lang TEXT,                       -- vd: "en-US", "vi", "ja" — cố định cả session
    style       TEXT CHECK (style IN ('casual','business','academic')),
    model       TEXT,                       -- model đã dùng khi tạo session
    created_at  TEXT NOT NULL,              -- RFC3339 UTC
    updated_at  TEXT NOT NULL
);
```

### files
```sql
CREATE TABLE files (
    id              TEXT PRIMARY KEY,       -- UUID v4
    session_id      TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    file_name       TEXT NOT NULL,          -- tên file gốc (vd: "report.docx")
    file_type       TEXT NOT NULL CHECK (file_type IN ('docx','doc')),
    file_size       INTEGER NOT NULL DEFAULT 0,   -- bytes
    original_path   TEXT,                   -- path file gốc user chọn
    source_path     TEXT,                   -- path source.md đã extract (disk)
    translated_path TEXT,                   -- path translated.md / translated.docx (disk)
    char_count      INTEGER DEFAULT 0,
    page_count      INTEGER DEFAULT 0,
    style           TEXT CHECK (style IN ('casual','business','academic')),
    model_used      TEXT,
    status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','processing','done','error','cancelled')),
                    -- cancelled: user bấm cancel (thêm ở migration 003)
    error_msg       TEXT,                   -- lý do lỗi nếu status=error
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);
CREATE INDEX idx_files_session ON files(session_id);
```

### messages
```sql
CREATE TABLE messages (
    id                  TEXT PRIMARY KEY,   -- UUID v4
    session_id          TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role                TEXT NOT NULL CHECK (role IN ('user','assistant')),
    display_order       INTEGER NOT NULL,   -- thứ tự hiển thị, auto-increment per session
    display_mode        TEXT NOT NULL DEFAULT 'bubble'
                        CHECK (display_mode IN ('bubble','bilingual','file')),
                        -- bubble   : chat thông thường
                        -- bilingual: side-by-side source/translation (paste dài / có structure)
                        -- file     : file translation card (thêm ở migration 002)
    original_content    TEXT NOT NULL DEFAULT '',
                        -- user: text gốc user nhập
                        -- assistant (text): rỗng (FE dùng streamingText)
                        -- assistant (file DOCX): source markdown (để hiển thị)
    translated_content  TEXT,               -- kết quả dịch (set sau khi stream xong)
    file_id             TEXT REFERENCES files(id) ON DELETE SET NULL,
                        -- chỉ set ở assistant message khi dịch file
    source_lang         TEXT,               -- "vi", "en", "auto" — detect từ nội dung
    target_lang         TEXT,               -- ngôn ngữ đích
    style               TEXT CHECK (style IN ('casual','business','academic')),
    model_used          TEXT,               -- model đã dùng để dịch
    original_message_id TEXT REFERENCES messages(id),
                        -- set khi retranslate: trỏ về message gốc
    tokens              INTEGER DEFAULT 0,  -- ước tính hoặc thực tế (OpenAI)
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_messages_order ON messages(session_id, display_order);
CREATE INDEX idx_messages_session ON messages(session_id, display_order);
```

### settings
```sql
CREATE TABLE settings (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);
-- Keys mặc định:
-- theme            → "system" | "light" | "dark"
-- active_provider  → "ollama" | "openai"
-- active_model     → vd: "qwen2.5:7b", "gpt-4o-mini"
-- active_style     → "casual" | "business" | "academic"
-- last_target_lang → vd: "en-US", "vi"
```

---

## 5. Dev Commands

```bash
# Chạy dev (hot reload)
cd backend && wails dev

# Build production
cd backend && make build

# Download pandoc binary (cần 1 lần)
cd backend && make fetch-pandoc

# Generate SQLC (sau khi thay đổi SQL queries)
cd backend && sqlc generate

# Build frontend riêng
cd frontend && npm run build
```

**Frontend output** → `backend/dist/` (embedded vào Go binary lúc build).

---

## 6. Rules cho Claude

- Commit title phải có prefix `[claude]`
- BE theo Clean Architecture — không để business logic vào `handler/`, không để DB logic vào `controller/`
- Không tạo file mới nếu không cần thiết — prefer edit file hiện có
- Được phép tư duy/phản biện nếu yêu cầu của developer không thỏa đáng
- Mọi vấn đề liên quan tới dự án phải phân tích / fact check cẩn thận chớ không được diễn giả / suy đoán
- SQL queries mới phải qua SQLC (`sqlc/queries/*.sql` → `sqlc generate`) — không viết raw SQL trong repo code trừ search (đã có exception trong `message_repo.go`)
- Không expose `*sql.Tx` ra ngoài repository layer — dùng `Registry.DoInTx()`
- Context phải được propagate xuyên suốt mọi method signature
- `internal/bridge/types.go` là boundary DTOs — không dùng `model.*` trực tiếp trong handler response
