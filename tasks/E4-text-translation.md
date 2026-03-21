# E4 — Text Translation (Core Feature)

> Tham chiếu: Section 5.4 (LangChip), 5.5 (file input), 5.6 (input detection), 5.7 (events), 5.8 (display types), 9.1 (prompts), 10.2 (flow)

---

### US-020 — Language Selection (LangChip)

> **Epic:** E4 | **Type:** Story | **Size:** M

**Status:** `Todo`

**User Story:**
> As a user, I want to select the target language before sending a message, so that my text gets translated to the language I need.

**Acceptance Criteria:**
- [ ] `LangChip.tsx` hiển thị trong InputArea: label format `"Dịch · EN"`, `"Dịch · 한국어"`, ...
- [ ] Label lấy từ `LANG_MAP[activeTargetLang]`: `utils/langMap.ts`
- [ ] Click chip → mở Language Popover (position: phía trên chip, align left)
- [ ] Popover title: "Ngôn ngữ đích"
- [ ] Popover body: danh sách 10 ngôn ngữ:
  - Anh - US (`en-US`) — default
  - Anh - UK (`en-GB`)
  - Anh - AUS (`en-AU`)
  - Hàn (`ko`)
  - Nhật (`ja`)
  - Trung Giản thể (`zh-CN`)
  - Trung Phồn thể (`zh-TW`)
  - Pháp (`fr`)
  - Đức (`de`)
  - Tây Ban Nha (`es`)
- [ ] Click ngôn ngữ → `uiStore.setActiveTargetLang(code)` → chip cập nhật ngay → popover đóng
- [ ] `activeTargetLang` persist qua restart: save vào `settings` table key `last_target_lang` (gọi `SaveSettings`)
- [ ] `activeTargetLang` default = `en-US` (hoặc value từ `settings.last_target_lang` load lúc startup)

**Technical Notes:**

**FE — `utils/langMap.ts`:**
```typescript
export const LANG_MAP: Record<string, string> = {
  'en-US': 'EN', 'en-GB': 'EN-GB', 'en-AU': 'EN-AU',
  'ko': '한국어', 'ja': '日本語',
  'zh-CN': '中文(简)', 'zh-TW': '中文(繁)',
  'fr': 'FR', 'de': 'DE', 'es': 'ES',
}

export const LANG_OPTIONS = [
  { code: 'en-US', label: 'Anh - US' },
  { code: 'en-GB', label: 'Anh - UK' },
  { code: 'en-AU', label: 'Anh - AUS' },
  { code: 'ko',    label: 'Hàn' },
  { code: 'ja',    label: 'Nhật' },
  { code: 'zh-CN', label: 'Trung Giản thể' },
  { code: 'zh-TW', label: 'Trung Phồn thể' },
  { code: 'fr',    label: 'Pháp' },
  { code: 'de',    label: 'Đức' },
  { code: 'es',    label: 'Tây Ban Nha' },
]
```

**FE — `LangChip.tsx`:**
```tsx
export default function LangChip() {
  const { activeTargetLang, setActiveTargetLang } = useUIStore()
  const [open, setOpen] = useState(false)

  function selectLang(code: string) {
    setActiveTargetLang(code)
    WailsService.saveSettings({ last_target_lang: code }) // persist
    setOpen(false)
  }

  return (
    <>
      <button className="input-chip" onClick={() => setOpen(true)}>
        Dịch · {LANG_MAP[activeTargetLang] ?? activeTargetLang}
      </button>
      {open && <LangPopover onSelect={selectLang} onClose={() => setOpen(false)} />}
    </>
  )
}
```

**Depends on:** US-003, FE-002, FE-003

---

### US-021 — Style Selection (StyleChip)

> **Epic:** E4 | **Type:** Story | **Size:** S

**Status:** `Todo`

**User Story:**
> As a user, I want to choose a translation style (Casual / Business / Academic) before sending, so that the translation tone matches my needs.

**Acceptance Criteria:**
- [ ] `StyleChip.tsx` hiển thị trong InputArea, label = style hiện tại: `"Casual"`, `"Business"`, `"Academic"`
- [ ] Click chip → mở Style Popover với 3 options dạng segmented button hoặc list
- [ ] Chọn style → `uiStore.setActiveStyle(style)` → chip cập nhật → popover đóng
- [ ] `activeStyle` default = `settingsStore.defaultStyle` (load lúc startup)
- [ ] Style được pass vào `SendRequest.style` khi gửi message
- [ ] Label hiển thị: `"Casual"` / `"Business"` / `"Academic"` (không dịch sang tiếng Việt)

**Technical Notes:**

**FE:**
```tsx
export default function StyleChip() {
  const { activeStyle, setActiveStyle } = useUIStore()
  const [open, setOpen] = useState(false)
  const styles: TranslationStyle[] = ['casual', 'business', 'academic']
  const LABELS = { casual: 'Casual', business: 'Business', academic: 'Academic' }

  return (
    <>
      <button className="input-chip" onClick={() => setOpen(true)}>
        {LABELS[activeStyle]}
      </button>
      {open && (
        <StylePopover
          current={activeStyle}
          onSelect={(s) => { setActiveStyle(s); setOpen(false) }}
          onClose={() => setOpen(false)}
        />
      )}
    </>
  )
}
```

**Depends on:** US-003, FE-002

---

### US-022 — Input Detection (Language + Structure)

> **Epic:** E4 | **Type:** Story | **Size:** S

**Status:** `Todo`

**User Story:**
> As a user, when I type or paste text, I want the app to automatically detect the language and content structure, so that the translation is displayed in the best format without me having to configure anything.

**Acceptance Criteria:**
- [ ] `utils/languageDetect.ts` — `detectLang(text: string): 'vi' | 'unknown'`:
  - Có ký tự diacritic tiếng Việt → `'vi'`
  - Ngược lại → `'unknown'`
- [ ] `utils/structureDetect.ts` — `detectStructure(text: string): boolean`:
  - Có Markdown heading (`/^#{1,6}\s/m`) → true
  - Có bold/italic (`/\*\*.+\*\*|\*.+\*/`) → true
  - Có nhiều dòng trống (`/\n{2,}/`) → true
  - Ngược lại → false
- [ ] Khi user **gõ** (keypress): `displayMode = 'bubble'` (luôn luôn, không detect structure)
- [ ] Khi user **paste** (paste event):
  - `detectStructure(text) = true` → convert HTML→Markdown (dùng `turndown`) → `displayMode = 'bilingual'`
  - Plain text ≤ 2000 ký tự → `displayMode = 'bubble'`
  - Plain text > 2000 ký tự → `displayMode = 'bilingual'`
- [ ] `sourceLang` = `detectLang(text)` — chạy song song với detectStructure
- [ ] `InputArea.tsx` track event type (keypress vs paste) bằng state `lastInputMode: 'type' | 'paste'`

**Technical Notes:**

**FE — `utils/languageDetect.ts`:**
```typescript
const VI_PATTERN = /[àáâãèéêìíòóôõùúýăđơưạảấầẩẫậắằẳẵặẹẻẽếềểễệỉịọỏốồổỗộớờởỡợụủứừửữựỳỵỷỹ]/i

export function detectLang(text: string): 'vi' | 'unknown' {
  return VI_PATTERN.test(text) ? 'vi' : 'unknown'
}
```

**FE — Markdown pipeline (paste event):**
```typescript
import TurndownService from 'turndown'
const td = new TurndownService({ headingStyle: 'atx' })

function processClipboard(e: ClipboardEvent): { content: string; isStructured: boolean } {
  const html = e.clipboardData?.getData('text/html')
  const plain = e.clipboardData?.getData('text/plain') ?? ''

  if (html && html.trim()) {
    const markdown = td.turndown(html)
    return { content: markdown, isStructured: true }
  }

  return { content: plain, isStructured: detectStructure(plain) || plain.length > 2000 }
}
```

**Depends on:** FE-001

---

### US-023 — Send Message + Optimistic UI

> **Epic:** E4 | **Type:** Story | **Size:** M

**Status:** `Todo`

**User Story:**
> As a user, when I click Send, I want to see my message appear immediately in the chat feed while the translation is being prepared, so that the app feels responsive.

**Acceptance Criteria:**
- [ ] Click Send (hoặc Enter trong textarea):
  1. `detectLang(text)` → `sourceLang`
  2. `detectStructure(text)` → `displayMode`
  3. Clear input textarea
  4. Tạo optimistic user message ngay lập tức trong UI (không cần chờ API)
  5. Gọi `Go.SendMessage(req)` (hoặc `CreateSessionAndSend` nếu session mới)
- [ ] Optimistic user message render đúng:
  - Plain text ngắn (≤ 2000 ký tự, `displayMode='bubble'`) → `MessageBubble`
  - Plain text dài hoặc structured → `UserTextCard` (collapsed, hiện ~3 dòng + "Xem thêm")
- [ ] Skeleton (assistant placeholder) xuất hiện sau khi `translation:start` event nhận được
- [ ] Input disabled khi đang streaming (`streamStatus !== 'idle'`)
- [ ] Nếu `SendMessage` trả về error → xóa optimistic message, restore input text

**Technical Notes:**

**BE — `controller/message/send.go`:**
```go
func (c *controller) SendMessage(ctx context.Context, req handler.SendRequest) (string, error) {
    msgID := uuid.New().String()
    now := time.Now().UTC().Format(time.RFC3339)

    // INSERT user message
    err := c.repo.Message().Insert(ctx, &model.Message{
        ID: msgID, SessionID: req.SessionID, Role: "user",
        DisplayMode: req.DisplayMode, OriginalContent: req.Content,
        SourceLang: req.SourceLang, TargetLang: req.TargetLang,
        Style: model.TranslationStyle(req.Style),
    })
    if err != nil { return "", err }

    // UPDATE session.target_lang (chỉ normal send, không phải retranslate)
    if req.OriginalMessageID == "" {
        c.repo.Session().UpdateTargetLang(ctx, req.SessionID, req.TargetLang)
    }

    // Start stream async
    go c.streamTranslation(context.Background(), req.SessionID, msgID, req)
    return msgID, nil
}
```

**FE — `InputArea.tsx`:**
```tsx
async function handleSend() {
  const content = inputValue.trim()
  if (!content || streamStatus !== 'idle') return

  const sourceLang = detectLang(content)
  const displayMode = lastInputMode === 'paste'
    ? (detectStructure(content) || content.length > 2000 ? 'bilingual' : 'bubble')
    : 'bubble'

  const optimisticMsg: Message = {
    id: `optimistic-${Date.now()}`, role: 'user',
    displayMode, originalContent: content, ...
  }
  messageStore.appendMessage(activeSessionId, optimisticMsg)
  setInputValue('')

  try {
    const msgId = await WailsService.sendMessage({
      sessionId: activeSessionId, content, displayMode, sourceLang,
      targetLang: activeTargetLang, style: activeStyle,
    })
    // Replace optimistic msg with real msgId
    messageStore.confirmMessage(optimisticMsg.id, msgId)
  } catch (err) {
    messageStore.removeMessage(activeSessionId, optimisticMsg.id)
    setInputValue(content) // restore
  }
}
```

**IPC Used:** `SendMessage(req SendRequest)`

**Depends on:** US-011, US-020, US-021, US-022, FE-002

---

### US-024 — Translation Streaming

> **Epic:** E4 | **Type:** Story | **Size:** L

**Status:** `Todo`

**User Story:**
> As a user, when I send a message, I want to see the translation appear word by word as it's being generated, so that I don't have to stare at a loading spinner.

**Acceptance Criteria:**

**State machine `streamStatus`:**
- `idle` → (Send) → `pending`
- `pending` → (`translation:start`) → hiện skeleton assistant bubble
- `pending` → (first `translation:chunk`) → `streaming`
- `streaming` → (`translation:done`) → `idle`, render final TranslationCard
- `pending/streaming` → (`translation:error`) → `error`
- `error` → (new Send) → `idle`

**Events:**
- [ ] `translation:start` `{ messageId }` → `messageStore.setStreamStatus('pending')`, thêm skeleton placeholder với messageId
- [ ] `translation:chunk` `{ chunk: string }` → `messageStore.setStreamStatus('streaming')`, `messageStore.updateStreamingText(chunk)` (append)
- [ ] `translation:done` `{ message: Message }` → `messageStore.finalizeStream(sessionId, message)` — replace streaming bubble với final Message
- [ ] `translation:error` `{ error: string }` → `messageStore.setStreamStatus('error')`, bubble hiện error state với retry option

**Final message (translation:done):**
- [ ] TranslationCard render nếu `displayMode = 'bilingual'`; MessageBubble nếu `displayMode = 'bubble'`
- [ ] Card footer: `"HH:MM · Casual"` (giờ:phút theo local time)
- [ ] Scroll to bottom sau mỗi chunk (auto-scroll)
- [ ] Auto-scroll dừng nếu user đã scroll lên (detect bằng `scrollTop + clientHeight < scrollHeight - threshold`)

**Technical Notes:**

**BE — `controller/message/send.go` — `streamTranslation()`:**
```go
func (c *controller) streamTranslation(ctx context.Context, sessionID, msgID string, req handler.SendRequest) {
    events := make(chan gateway.StreamEvent, 10)

    runtime.EventsEmit(ctx, "translation:start", map[string]string{"messageId": msgID})

    provider := c.getProvider(req.Provider, req.Model)
    go provider.TranslateStream(ctx, req.Content, req.SourceLang, req.TargetLang, req.Style, events)

    var fullText strings.Builder
    for ev := range events {
        switch ev.Type {
        case "chunk":
            fullText.WriteString(ev.Content)
            runtime.EventsEmit(ctx, "translation:chunk", ev.Content)
        case "done":
            // UPDATE DB
            now := time.Now().UTC().Format(time.RFC3339)
            c.repo.Message().UpdateTranslated(ctx, msgID, fullText.String(), ev.Tokens, now)
            // Load full message để emit
            msg, _ := c.repo.Message().GetByID(ctx, msgID)
            runtime.EventsEmit(ctx, "translation:done", msg)
        case "error":
            runtime.EventsEmit(ctx, "translation:error", ev.Error.Error())
        }
    }
}
```

**FE — `hooks/translation/useTranslation.ts`:**
```typescript
export function useTranslation() {
  const { activeSessionId } = useSessionStore()
  const { setStreamStatus, updateStreamingText, finalizeStream } = useMessageStore()

  useEffect(() => {
    const unsub0 = WailsEvents.onTranslationStart(({ messageId }) => {
      setStreamStatus('pending')
    })
    const unsub1 = WailsEvents.onTranslationChunk((chunk) => {
      setStreamStatus('streaming')
      updateStreamingText(chunk)
    })
    const unsub2 = WailsEvents.onTranslationDone((msg) => {
      finalizeStream(activeSessionId!, msg)
      setStreamStatus('idle')
    })
    const unsub3 = WailsEvents.onTranslationError((err) => {
      setStreamStatus('error')
    })
    return () => { unsub0(); unsub1(); unsub2(); unsub3() }
  }, [activeSessionId])
}
```

**Events:** `translation:start`, `translation:chunk`, `translation:done`, `translation:error`

**Depends on:** US-023, BE-005, BE-006, FE-002, FE-003

---

### US-025 — Message Display Types

> **Epic:** E4 | **Type:** Story | **Size:** L

**Status:** `Todo`

**User Story:**
> As a user, I want short text to appear as a bubble and long/structured text to appear as a formatted bilingual card, so that the layout matches the content type.

**Acceptance Criteria:**

**User messages:**
- [ ] `displayMode='bubble'` + ≤ 2000 ký tự → `MessageBubble` (bubble đơn giản, không có action bar)
- [ ] `displayMode='bilingual'` hoặc > 2000 ký tự → `UserTextCard`:
  - Hiện ~3 dòng đầu (CSS line-clamp: 3)
  - Nút "Xem thêm" → expand full content
  - Nút "Thu gọn" → collapse lại

**Assistant messages (TranslationCard — `displayMode='bilingual'`):**
- [ ] 2-panel layout: LEFT = source (tint xanh), RIGHT = translation (tint hồng)
- [ ] Action bar (hover-gated — chỉ xuất hiện khi hover):
  - Retranslate button
  - Copy button
  - Export button (dropdown PDF/DOCX)
- [ ] Fullscreen button (always-on — luôn hiện, không cần hover)
- [ ] Card footer dưới RIGHT panel: `"HH:MM · Casual"`

**Assistant messages (MessageBubble — `displayMode='bubble'`):**
- [ ] Bubble đơn giản với translated text
- [ ] Action bar giống TranslationCard (hover-gated)

**Streaming state:**
- [ ] Khi `streamStatus='pending'` → hiện skeleton (grey pulse animation)
- [ ] Khi `streamStatus='streaming'` → text xuất hiện dần (typewriter, không có animation riêng — chỉ cần append text)
- [ ] Streaming bubble không có card footer cho đến khi `translation:done`

**Technical Notes:**

**FE — `MessageFeed.tsx`:**
```tsx
function renderUserMessage(msg: Message) {
  if (msg.displayMode === 'bilingual' || msg.originalContent.length > 2000) {
    return <UserTextCard key={msg.id} message={msg} />
  }
  return <MessageBubble key={msg.id} message={msg} role="user" />
}

function renderAssistantMessage(msg: Message) {
  if (msg.displayMode === 'bilingual') {
    return <TranslationCard key={msg.id} message={msg} />
  }
  return <MessageBubble key={msg.id} message={msg} role="assistant" />
}
```

**FE — `TranslationCard.tsx` props:**
```typescript
interface TranslationCardProps {
  message: Message
  isStreaming?: boolean   // true khi đang stream → ẩn action bar, ẩn footer
  streamingText?: string  // text đang stream
}
```

**FE — Card footer format:**
```typescript
function formatFooter(msg: Message, isRetranslate: boolean): string {
  const time = new Date(msg.createdAt).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
  const styleLabel = { casual: 'Casual', business: 'Business', academic: 'Academic' }[msg.style]
  return isRetranslate ? `${time} · Bản dịch lại · ${styleLabel}` : `${time} · ${styleLabel}`
}
```

**Depends on:** US-024, FE-001

---

### US-026 — Message Pagination (Load More)

> **Epic:** E4 | **Type:** Story | **Size:** M

**Status:** `Todo`

**User Story:**
> As a user, when I scroll up in a session with many messages, I want older messages to load automatically, so that I can see the full history without it all loading at once.

**Acceptance Criteria:**
- [ ] Khi mở session → gọi `GetMessages(sessionId, cursor=0, limit=30)` → load batch mới nhất
- [ ] `cursor=0` = "load 30 messages mới nhất" (display_order DESC LIMIT 30)
- [ ] Response `nextCursor`: `0 = EOF` (đã load hết), `> 0 = display_order` của message tiếp theo
- [ ] `messageStore.cursors[sessionId]` lưu nextCursor; `hasMore[sessionId]` lưu flag
- [ ] Khi user scroll lên top (scroll position < 100px từ top) + `hasMore = true`:
  → Gọi `GetMessages(sessionId, cursor=currentCursor, limit=30)`
  → Prepend messages vào đầu feed
  → Giữ nguyên scroll position (không nhảy lên top)
- [ ] Loading indicator ở top feed khi đang load more
- [ ] Không load lại nếu đang loading (`isLoadingMore` flag)

**Technical Notes:**

**BE — `repository/message/list.go`:**
```go
// cursor=0: lấy 30 messages mới nhất
// cursor>0: lấy 30 messages có display_order < cursor
func (r *repo) ListByCursor(ctx context.Context, sessionID string, cursor, limit int) ([]model.Message, error) {
    if cursor == 0 {
        return r.q.GetLatestMessages(ctx, sessionID, limit) // ORDER BY display_order DESC LIMIT ?
    }
    return r.q.GetMessagesBefore(ctx, sessionID, cursor, limit) // WHERE display_order < ? ORDER BY display_order DESC LIMIT ?
}
```

```go
// handler trả về MessagesPage
func buildMessagesPage(msgs []model.Message, requestedLimit int) handler.MessagesPage {
    // msgs được sort DESC → reverse để render theo thứ tự ASC
    sort.Slice(msgs, func(i, j int) bool { return msgs[i].DisplayOrder < msgs[j].DisplayOrder })
    nextCursor := 0
    if len(msgs) == requestedLimit {
        nextCursor = msgs[0].DisplayOrder // message cũ nhất trong batch
    }
    return handler.MessagesPage{
        Messages:   convertToHandlerMessages(msgs),
        NextCursor: nextCursor,
        HasMore:    nextCursor > 0,
    }
}
```

**FE — `hooks/translation/useScrollControl.ts`:**
```typescript
export function useScrollControl(feedRef: RefObject<HTMLDivElement>) {
  function handleScroll() {
    if (!feedRef.current) return
    const { scrollTop } = feedRef.current
    if (scrollTop < 100 && hasMore && !isLoadingMore) {
      loadMoreMessages(activeSessionId)
    }
  }

  // Giữ scroll position khi prepend messages
  function preserveScrollOnPrepend(prevScrollHeight: number) {
    if (!feedRef.current) return
    const newScrollHeight = feedRef.current.scrollHeight
    feedRef.current.scrollTop = newScrollHeight - prevScrollHeight
  }
}
```

**IPC Used:** `GetMessages(sessionId, cursor, limit)`

**Depends on:** US-025, BE-003, BE-004
