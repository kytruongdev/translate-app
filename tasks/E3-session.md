# E3 — Session Management

> Tham chiếu: Section 5.10 (grouping), 5.11 (rename UX), 7.1 (IPC), 8.1 (schema), 10.1 (create flow)

---

### US-010 — Session List + Date Grouping

> **Epic:** E3 | **Type:** Story | **Size:** M

**Status:** `Todo`

**User Story:**
> As a user, I want to see my sessions organized by date in the sidebar, so that I can quickly find recent translations.

**Acceptance Criteria:**
- [ ] `SessionList.tsx` render sessions theo groups, sorted đúng theo `GetSessions()` response
- [ ] Group labels và thứ tự (từ trên xuống):
  1. **Ghim** — sessions có `status = 'pinned'`; background tint khác (màu `--active`)
  2. **Hôm nay** — `updated_at` trong ngày hiện tại
  3. **1 ngày trước** — `updated_at` hôm qua
  4. **2 ngày trước** — `updated_at` 2 ngày trước
  5. **Cũ hơn** — `updated_at` từ 3 ngày trở lên
- [ ] Group rỗng (không có session) → không render label, không render group element
- [ ] `SessionItem.tsx` hiển thị: session title (truncated nếu dài), nút ··· (3 chấm)
- [ ] `SessionItem` active (đang xem) → background `--active`
- [ ] Click session item → `sessionStore.setActiveSession(id)` → navigate `/chat/:sessionId`
- [ ] `formatDate.ts` có hàm `getGroupLabel(updatedAt: string): string` trả về đúng label
- [ ] Sessions trong mỗi group sorted `updated_at DESC`

**Technical Notes:**

**BE:**
- `controller/session/list.go`:
  ```go
  func (c *controller) GetSessions(ctx context.Context) ([]model.Session, error) {
      return c.repo.Session().List(ctx)
  }
  ```
- `repository/session/list.go`: query dùng ORDER BY pinned first, `updated_at DESC`

**FE:**
- `utils/formatDate.ts`:
  ```typescript
  export function getGroupLabel(updatedAt: string): string {
    const now = new Date()
    const date = new Date(updatedAt)
    const diffDays = differenceInCalendarDays(now, date)
    if (diffDays === 0) return 'Hôm nay'
    if (diffDays === 1) return '1 ngày trước'
    if (diffDays === 2) return '2 ngày trước'
    return 'Cũ hơn'
  }
  ```
- `SessionList.tsx` dùng `useMemo` để group sessions:
  ```typescript
  const grouped = useMemo(() => {
    const pinned = sessions.filter(s => s.status === 'pinned')
    const rest = sessions.filter(s => s.status !== 'pinned')
    const groups: Record<string, Session[]> = {}
    rest.forEach(s => {
      const label = getGroupLabel(s.updatedAt)
      if (!groups[label]) groups[label] = []
      groups[label].push(s)
    })
    return { pinned, groups }
  }, [sessions])
  ```
- Group render order: ['Hôm nay', '1 ngày trước', '2 ngày trước', 'Cũ hơn']

**IPC Used:** `GetSessions()`

**Depends on:** US-001, FE-002, BE-009

---

### US-011 — Create Session (Atomic)

> **Epic:** E3 | **Type:** Story | **Size:** M

**Status:** `Todo`

**User Story:**
> As a user, when I send my first message, I want a new session to be created automatically and my message to start translating immediately, so that I don't have to manually manage sessions.

**Acceptance Criteria:**
- [ ] Khi user nhấn Send từ Start Page → FE gọi `CreateSessionAndSend(req)` (không phải 2 calls riêng)
- [ ] `req.title` được FE gen từ content trước khi gọi:
  - Có Markdown heading (`# ...`) → lấy text heading đầu tiên, bỏ `#`
  - Không có heading → lấy line đầu tiên
  - Cắt max 50 ký tự, thêm `…` nếu dài hơn
- [ ] Go thực hiện trong 1 transaction:
  1. INSERT session (`id=uuid`, `title=req.title`, `status='active'`, `target_lang=req.targetLang`)
  2. INSERT user message (`role='user'`, `display_order=1`, `display_mode=req.displayMode`, `original_content=req.content`)
  3. Bắt đầu stream translation (như flow text translation Section 10.2)
- [ ] Go trả về `messageId` (string) ngay sau khi INSERT, stream chạy async
- [ ] FE nhận `messageId`:
  - `sessionStore.appendSession(newSession)` — thêm session mới lên đầu sidebar
  - `sessionStore.setActiveSession(newSessionId)` — set active
  - Navigate → `/chat/:newSessionId`
  - `messageStore.appendMessage(sessionId, userMsg)` — thêm optimistic user message
- [ ] Sau khi navigate → ChatPage tự subscribe events `translation:start/chunk/done/error`

**Technical Notes:**

**BE — `controller/session/create.go`:**
```go
func (c *controller) CreateSessionAndSend(ctx context.Context, req handler.CreateSessionAndSendRequest) (string, error) {
    var msgID string
    err := c.repo.DoInTx(ctx, func(tx repository.Registry) error {
        // 1. Create session
        sessionID := uuid.New().String()
        now := time.Now().UTC().Format(time.RFC3339)
        err := tx.Session().Create(ctx, &model.Session{
            ID: sessionID, Title: req.Title, Status: "active",
            TargetLang: req.TargetLang, CreatedAt: now, UpdatedAt: now,
        })
        if err != nil { return err }

        // 2. Insert user message
        msgID = uuid.New().String()
        err = tx.Message().Insert(ctx, &model.Message{
            ID: msgID, SessionID: sessionID, Role: "user",
            DisplayMode: req.DisplayMode, OriginalContent: req.Content,
            SourceLang: req.SourceLang, TargetLang: req.TargetLang,
            Style: req.Style, CreatedAt: now, UpdatedAt: now,
        })
        return err
    })
    if err != nil { return "", err }

    // 3. Start translation stream async (goroutine)
    go c.streamTranslation(ctx, msgID, req)
    return msgID, nil
}
```

**FE — Title generation (`utils/titleGen.ts`):**
```typescript
export function genSessionTitle(content: string): string {
  const headingMatch = content.match(/^#{1,6}\s+(.+)$/m)
  const raw = headingMatch ? headingMatch[1] : content.split('\n')[0]
  return raw.length > 50 ? raw.slice(0, 50) + '…' : raw
}
```

**FE — `useTranslation.ts` (or `useSessionManager.ts`):**
```typescript
async function handleFirstSend(content: string, opts: SendOptions) {
  const title = genSessionTitle(content)
  const msgId = await WailsService.createSessionAndSend({ title, content, ...opts })
  sessionStore.appendSession({ id: newSessionId, title, ... })
  sessionStore.setActiveSession(newSessionId)
  navigate(`/chat/${newSessionId}`)
}
```

**IPC Used:** `CreateSessionAndSend(req CreateSessionAndSendRequest)`

**Depends on:** US-003, US-010, BE-004, BE-005

---

### US-012 — Rename Session

> **Epic:** E3 | **Type:** Story | **Size:** S

**Status:** `Todo`

**User Story:**
> As a user, I want to rename a session either from the sidebar menu or by clicking the title in the header, so that I can organize my translations with meaningful names.

**Acceptance Criteria:**

**Entry 1 — Sidebar:**
- [ ] Click ··· → dropdown menu → click "Đổi tên" → dropdown đóng, title của session item trong sidebar chuyển sang `contenteditable=true` + class `is-editing`
- [ ] Cursor đặt ở cuối text
- [ ] **Enter** → save, gọi `RenameSession(id, newTitle)`, `contenteditable=false`, class `is-editing` xóa
- [ ] **Esc** → cancel, title revert về giá trị trước, `contenteditable=false`
- [ ] **blur** → **không làm gì** (giữ nguyên editing state cho đến Enter/Esc)
- [ ] Nếu title rỗng sau Enter → revert về title cũ, không gọi API

**Entry 2 — Header:**
- [ ] Click vào `chat-session-title` (h1 ở top header ChatPage) → chuyển sang `contenteditable=true`
- [ ] **Enter** → save + gọi `RenameSession(id, newTitle)` + blur
- [ ] **Esc** → cancel, revert
- [ ] **blur** → auto-save (gọi `RenameSession`)
- [ ] Nếu title rỗng → revert về title cũ, không gọi API

**Cả 2 entries:**
- [ ] Sau khi save thành công → cập nhật `sessionStore.sessions` (update title in-place)
- [ ] Header title sync với sidebar title (cùng từ store)
- [ ] Tối đa 1 session đang edit tại 1 thời điểm (mở edit mới → close edit cũ)

**Technical Notes:**

**BE — `controller/session/update.go`:**
```go
func (c *controller) RenameSession(ctx context.Context, id, title string) error {
    if strings.TrimSpace(title) == "" {
        return errors.New("title cannot be empty")
    }
    return c.repo.Session().UpdateTitle(ctx, id, title)
}
```

**FE — `SessionItem.tsx`:**
```typescript
const [isEditing, setIsEditing] = useState(false)
const titleRef = useRef<HTMLDivElement>(null)
const prevTitle = useRef(session.title)

function startEdit() {
  prevTitle.current = session.title
  setIsEditing(true)
  setTimeout(() => { titleRef.current?.focus(); selectAll(titleRef.current) }, 0)
}

async function saveEdit() {
  const newTitle = titleRef.current?.textContent?.trim()
  if (!newTitle) { cancelEdit(); return }
  setIsEditing(false)
  await WailsService.renameSession(session.id, newTitle)
  sessionStore.renameSession(session.id, newTitle)
}

function cancelEdit() {
  if (titleRef.current) titleRef.current.textContent = prevTitle.current
  setIsEditing(false)
}
```

**IPC Used:** `RenameSession(id, title string)`

**Depends on:** US-010, BE-009

---

### US-013 — Pin / Unpin Session

> **Epic:** E3 | **Type:** Story | **Size:** S

**Status:** `Todo`

**User Story:**
> As a user, I want to pin important sessions to the top of the sidebar, so that I can quickly access frequently used translations.

**Acceptance Criteria:**
- [ ] Click ··· → dropdown menu → click "Ghim" / "Bỏ ghim" → dropdown đóng
- [ ] Label toggle: nếu `session.status = 'pinned'` → hiện "Bỏ ghim"; ngược lại → hiện "Ghim"
- [ ] Sau khi pin: session chuyển lên group "Ghim" ở top sidebar, sidebar re-render với nhóm Ghim
- [ ] Sau khi unpin: session chuyển về group đúng theo `updated_at`
- [ ] Gọi `UpdateSessionStatus(id, 'pinned'/'active')` và cập nhật store ngay (optimistic)
- [ ] Nhóm "Ghim" chỉ hiện khi có ít nhất 1 session ghim; ẩn khi không có session ghim nào

**Technical Notes:**

**BE — `controller/session/update.go`:**
```go
func (c *controller) UpdateSessionStatus(ctx context.Context, id, status string) error {
    allowed := map[string]bool{"active": true, "pinned": true}
    if !allowed[status] {
        return fmt.Errorf("invalid status for V1: %s", status)
    }
    return c.repo.Session().UpdateStatus(ctx, id, status)
}
```

**FE:**
```typescript
async function togglePin(session: Session) {
  const newStatus = session.status === 'pinned' ? 'active' : 'pinned'
  // Optimistic update
  sessionStore.updateStatus(session.id, newStatus)
  try {
    await WailsService.updateSessionStatus(session.id, newStatus)
  } catch {
    // Revert on error
    sessionStore.updateStatus(session.id, session.status)
  }
}
```

**IPC Used:** `UpdateSessionStatus(id, status string)`

**Depends on:** US-010, BE-009
