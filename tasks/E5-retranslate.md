# E5 — Retranslate (Reply-Quote Pattern)

> Tham chiếu: Section 10.3 (flow), 7.1 (IPC — SendMessage với originalMessageId), 5.8 (card footer)

---

### US-030 — Retranslate

> **Epic:** E5 | **Type:** Story | **Size:** L

**Status:** `Todo`

**User Story:**
> As a user, I want to re-translate an existing message with a different style or model, and see the new translation as a separate card linked to the original, so that I can compare translations without losing the previous one.

**Acceptance Criteria:**

**Trigger:**
- [ ] Hover vào assistant message → xuất hiện action buttons (Retranslate, Copy, Export)
- [ ] Click Retranslate → mở `RetranslatePopover` positioned gần button (position-aware, không bị clip)

**RetranslatePopover:**
- [ ] Chứa 2 controls:
  1. **Model selector** (1 dropdown, optgroup):
     - Online: `Gemini Flash` (default), `GPT-4o-mini`
     - Offline: `Qwen2.5:7b`
  2. **Style segmented button**: `Casual` | `Business` | `Academic`
- [ ] Default values khi mở: model = global `settingsStore.activeProvider`, style = `originalMessage.style`
- [ ] Nút "Hủy" → đóng popover
- [ ] Nút "Dịch lại" → gọi SendMessage với `originalMessageId`

**SendMessage request:**
- [ ] `content = originalMessage.originalContent` (lấy lại text gốc)
- [ ] `originalMessageId = originalMessage.id`
- [ ] `targetLang = originalMessage.targetLang` (giữ nguyên, không đổi)
- [ ] `displayMode = originalMessage.displayMode` (giữ nguyên)
- [ ] `style = selectedStyle`
- [ ] `provider = selectedProvider`, `model = selectedModel`

**Reply-quote render (sau translation:done):**
- [ ] Message mới thêm vào cuối feed (display_order = MAX + 1 — Go xử lý)
- [ ] UI của reply-quote message:
  ```
  ┌──────────────────────────────────────────────────────┐
  │ ↩ Bản gốc  [click → smooth scroll + flash highlight] │  ← .reply-quote header
  │ [snippet 1-2 dòng đầu của originalMessage.translatedContent] │
  ├──────────────────────────────────────────────────────┤
  │ [Bản dịch mới — TranslationCard hoặc MessageBubble]  │
  │ HH:MM · Bản dịch lại · Casual                        │  ← footer
  └──────────────────────────────────────────────────────┘
  ```
- [ ] Click "↩ Bản gốc" → smooth scroll đến `originalMessage` + flash/highlight animation (1.5s)
- [ ] Session `target_lang` **KHÔNG được update** sau retranslate (chỉ normal send mới update)

**Technical Notes:**

**BE — `controller/message/retranslate.go`:**

Detect retranslate bằng `req.OriginalMessageID != ""` trong `SendMessage`:
```go
func (c *controller) SendMessage(ctx context.Context, req handler.SendRequest) (string, error) {
    isRetranslate := req.OriginalMessageID != ""

    msgID := uuid.New().String()
    now := time.Now().UTC().Format(time.RFC3339)

    err := c.repo.Message().Insert(ctx, &model.Message{
        ID: msgID, SessionID: req.SessionID, Role: "assistant",
        DisplayMode: req.DisplayMode, SourceLang: req.SourceLang,
        TargetLang: req.TargetLang, Style: model.TranslationStyle(req.Style),
        OriginalMessageID: nullableString(req.OriginalMessageID),
    })
    if err != nil { return "", err }

    // Chỉ update session.target_lang nếu KHÔNG phải retranslate
    if !isRetranslate {
        c.repo.Session().UpdateTargetLang(ctx, req.SessionID, req.TargetLang)
    }

    go c.streamTranslation(context.Background(), req.SessionID, msgID, req)
    return msgID, nil
}
```

**FE — `RetranslatePopover.tsx`:**
```tsx
interface RetranslatePopoverProps {
  originalMessage: Message
  anchorRect: DOMRect
  onClose: () => void
}

export default function RetranslatePopover({ originalMessage, anchorRect, onClose }: RetranslatePopoverProps) {
  const { activeProvider } = useSettingsStore()
  const [selectedModel, setSelectedModel] = useState(activeProvider)
  const [selectedStyle, setSelectedStyle] = useState<TranslationStyle>(originalMessage.style)

  async function handleRetranslate() {
    onClose()
    await WailsService.sendMessage({
      sessionId: originalMessage.sessionId,
      content: originalMessage.originalContent,
      originalMessageId: originalMessage.id,
      targetLang: originalMessage.targetLang,   // giữ nguyên
      displayMode: originalMessage.displayMode,  // giữ nguyên
      style: selectedStyle,
      provider: selectedModel,
      sourceLang: originalMessage.sourceLang,
    })
  }

  return (
    <div className="retranslate-popover" style={positionFromRect(anchorRect)}>
      <div className="popover-header"><span>Dịch lại</span></div>
      <div className="popover-body">
        {/* Model selector */}
        <select value={selectedModel} onChange={e => setSelectedModel(e.target.value)}>
          <optgroup label="Online">
            <option value="gemini">Gemini Flash</option>
            <option value="openai">GPT-4o-mini</option>
          </optgroup>
          <optgroup label="Offline">
            <option value="ollama">Qwen2.5:7b</option>
          </optgroup>
        </select>
        {/* Style segmented */}
        <div className="theme-segmented">
          {(['casual', 'business', 'academic'] as TranslationStyle[]).map(s => (
            <button key={s} className={selectedStyle === s ? 'active' : ''}
              onClick={() => setSelectedStyle(s)}>
              {s.charAt(0).toUpperCase() + s.slice(1)}
            </button>
          ))}
        </div>
      </div>
      <div className="dialog-actions">
        <button onClick={onClose}>Hủy</button>
        <button onClick={handleRetranslate}>Dịch lại</button>
      </div>
    </div>
  )
}
```

**FE — Reply-quote snippet:**
```typescript
// Lấy snippet từ originalMessage.translatedContent
function getSnippet(text: string, maxChars = 100): string {
  if (text.length <= maxChars) return text
  return text.slice(0, maxChars) + '…'
}
```

**FE — Scroll + highlight animation:**
```typescript
function scrollToMessage(messageId: string) {
  const el = document.getElementById(`msg-${messageId}`)
  if (!el) return
  el.scrollIntoView({ behavior: 'smooth', block: 'center' })
  el.classList.add('highlight-flash')
  setTimeout(() => el.classList.remove('highlight-flash'), 1500)
}
```

**CSS:**
```css
@keyframes highlight-flash {
  0%   { background: rgba(var(--accent-rgb), 0.3); }
  100% { background: transparent; }
}
.highlight-flash { animation: highlight-flash 1.5s ease-out; }
```

**FE — Nhận biết retranslate message:**
```typescript
// Trong MessageFeed, render reply-quote nếu có originalMessageId
function renderAssistantMessage(msg: Message, allMessages: Message[]) {
  const original = msg.originalMessageId
    ? allMessages.find(m => m.id === msg.originalMessageId)
    : null

  return (
    <div id={`msg-${msg.id}`}>
      {original && (
        <div className="reply-quote" onClick={() => scrollToMessage(original.id)}>
          <span>↩ Bản gốc</span>
          <span className="snippet">{getSnippet(original.translatedContent)}</span>
        </div>
      )}
      {msg.displayMode === 'bilingual'
        ? <TranslationCard message={msg} />
        : <MessageBubble message={msg} role="assistant" />}
    </div>
  )
}
```

**IPC Used:** `SendMessage(req SendRequest)` với `req.originalMessageId != ""`

**Depends on:** US-023, US-024, US-025
