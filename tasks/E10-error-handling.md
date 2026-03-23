# E10 — Error Handling & Edge Cases

> Tham chiếu: Section 11.1 (error categories), 11.2 (user-facing messages), 11.3 (streaming error)

---

### US-080 — Translation Error States

> **Epic:** E10 | **Type:** Story | **Size:** M

**Status:** `Partial` *(`translation:error` → `sendError` + `chat-error-banner` / hint start view; **chưa** ErrorBubble + partial text + nút “Thử lại”, **chưa** toast theo loại lỗi / phân loại network-key-rate-limit như spec)*

**User Story:**
> As a user, when a translation fails, I want to see a clear error message and be able to retry, so that I don't lose my input.

**Acceptance Criteria:**

**Streaming error (mid-stream):**
- [ ] Go emits `translation:error` → FE nhận → `messageStore.setStreamStatus('error')`
- [ ] Streaming bubble chuyển sang error state:
  - Hiện partial text (nếu có) + label `[Dịch bị gián đoạn]`
  - Hiện nút **"Thử lại"** dưới bubble
- [ ] Nút "Thử lại" → re-send với cùng params (không phải retranslate — không có `originalMessageId`)
- [ ] Thử lại → clear error state → set `streamStatus = 'pending'` → gọi lại `SendMessage`

**Error types và messages:**
- [ ] Network error → toast: "Không có kết nối mạng. Vui lòng thử lại."
- [ ] API key invalid (401/403) → toast: "API key không hợp lệ. Vui lòng liên hệ admin."
- [ ] Rate limit (429) → toast: "Đã vượt quá giới hạn API. Thử lại sau {N} giây." (hiện countdown nếu BE trả về `retry-after`)
- [ ] Server error (500/502/503) → retry với backoff (tối đa 3 lần), nếu vẫn lỗi → toast generic
- [ ] Generic → toast: "Đã xảy ra lỗi. Vui lòng thử lại."

**streamStatus error state UI:**
- [ ] Input area vẫn active trong error state (user có thể gửi message mới)
- [ ] Error state reset về `idle` khi user gửi message mới

**Technical Notes:**

**FE — Error bubble render trong `MessageFeed.tsx`:**
```tsx
function renderAssistantMessage(msg: Message, streamStatus: string, streamingText: string) {
  const isStreaming = streamStatus !== 'idle' && streamStatus !== 'error'
  const hasError = streamStatus === 'error'

  return (
    <div id={`msg-${msg.id}`}>
      {isStreaming && <StreamingBubble text={streamingText} />}
      {hasError && (
        <ErrorBubble
          partialText={streamingText}
          onRetry={() => retryLastMessage()}
        />
      )}
      {!isStreaming && !hasError && renderFinalMessage(msg)}
    </div>
  )
}
```

**FE — Retry handler:**
```typescript
function retryLastMessage() {
  if (!lastSendRequest) return
  messageStore.setStreamStatus('pending')
  WailsService.sendMessage(lastSendRequest)
}
```

**BE — Error propagation:**
```go
// Trong streamTranslation goroutine
func (c *controller) streamTranslation(ctx context.Context, sessionID, msgID string, req handler.SendRequest) {
    events := make(chan gateway.StreamEvent)
    go c.gateway.AI().TranslateStream(ctx, req.Content, req.SourceLang, req.TargetLang, req.Style, events)

    runtime.EventsEmit(ctx, "translation:start", map[string]string{"messageId": msgID})

    var sb strings.Builder
    for event := range events {
        switch event.Type {
        case "chunk":
            sb.WriteString(event.Content)
            runtime.EventsEmit(ctx, "translation:chunk", map[string]string{"chunk": event.Content})
        case "error":
            runtime.EventsEmit(ctx, "translation:error", map[string]string{"error": event.Error.Error()})
            return
        case "done":
            c.repo.Message().UpdateTranslated(ctx, msgID, sb.String())
            finalMsg, _ := c.repo.Message().GetByID(ctx, msgID)
            runtime.EventsEmit(ctx, "translation:done", map[string]any{"message": finalMsg})
        }
    }
}
```

**Depends on:** US-024, FE-003

---

### US-081 — File Translation Error Handling

> **Epic:** E10 | **Type:** Story | **Size:** S

**Status:** `Todo`

**User Story:**
> As a user, when a file translation fails or the file is unsupported, I want to see a clear error message, so that I understand what went wrong.

**Acceptance Criteria:**

**Scanned PDF:**
- [ ] `ReadFileInfo` trả về `isScanned = true` → FE hiện error inline trong FileAttachment chip:
  - "PDF scan không hỗ trợ, vui lòng dùng PDF có text"
  - Nút × để clear, không cho gửi

**File quá lớn:**
- [ ] `pageCount > 200` → FE hiện error inline: "Tệp quá lớn (tối đa 200 trang)"
  - Không cho gửi

**File format không hợp lệ (drag-drop):**
- [ ] Extension không phải `.pdf` hoặc `.docx` → toast error: "Chỉ hỗ trợ PDF và DOCX"

**Mid-translation error:**
- [ ] Go emits `file:error` → FE:
  - Ẩn progress bar
  - Hiện toast error: "Dịch file thất bại. Vui lòng thử lại."
  - Nút **"Thử lại"** → gọi lại `TranslateFile(req)` với cùng params
  - Update `files.status = 'error'` trong DB (BE làm)

**Technical Notes:**

**BE — `repository/file/reader.go` — detect scanned PDF:**
```go
func IsScannedPDF(path string) (bool, error) {
    // Dùng pdfcpu để đọc page content
    // Nếu tất cả pages đều không có text (chỉ có images) → return true
    // Heuristic: total text chars < 100 → likely scanned
}
```

**FE — Error state trong `useFileTranslation.ts`:**
```typescript
const unsub5 = EventsOn('file:error', (err: string) => {
  setFileError(err)
  setFileTranslating(false)
  setProgress(null)
  toast.error('Dịch file thất bại. Vui lòng thử lại.')
})
```

**Depends on:** US-041, US-042

---

### US-082 — Toast Notification System

> **Epic:** E10 | **Type:** Story | **Size:** S

**Status:** `Todo`

**User Story:**
> As a user, I want to see brief, non-intrusive notifications for success and error events, so that I'm informed without being disrupted.

**Acceptance Criteria:**
- [ ] Toast component tự implement (không dùng thư viện ngoài) — hoặc dùng thư viện nhẹ (react-hot-toast ~1KB)
- [ ] Vị trí: bottom-center hoặc bottom-right của màn hình
- [ ] **Types:**
  - `success` — icon ✅, màu green
  - `error` — icon ❌, màu red
  - `info` — icon ℹ️, màu neutral
- [ ] **Auto-dismiss:**
  - `success` / `info` → tự đóng sau 3s
  - `error` → tự đóng sau 5s hoặc user click ×
- [ ] Tối đa 3 toast hiện cùng lúc — toast mới đẩy cũ lên (queue)
- [ ] Animation: slide-in từ dưới + fade-out khi dismiss
- [ ] Toast có thể chứa action button (ví dụ "Mở file" sau export success)

**Technical Notes:**

**FE — Global toast API:**
```typescript
// Dùng react-hot-toast (hoặc implement tương đương)
import toast from 'react-hot-toast'

// Success
toast.success('Đã lưu: report_translated.pdf')

// Error
toast.error('Không có kết nối mạng. Vui lòng thử lại.')

// With action
toast.success(
  (t) => (
    <span>
      Đã lưu: report.pdf
      <button onClick={() => { openFile(path); toast.dismiss(t.id) }}>Mở file</button>
    </span>
  ),
  { duration: 5000 }
)
```

**FE — Setup trong `App.tsx` / `providers.tsx`:**
```tsx
import { Toaster } from 'react-hot-toast'

export default function Providers({ children }: { children: React.ReactNode }) {
  return (
    <>
      {children}
      <Toaster
        position="bottom-center"
        toastOptions={{
          success: { duration: 3000 },
          error: { duration: 5000 },
        }}
      />
    </>
  )
}
```

**CSS custom styles (nếu implement tự):**
```css
@keyframes toast-in {
  from { transform: translateY(100%); opacity: 0; }
  to   { transform: translateY(0);    opacity: 1; }
}

@keyframes toast-out {
  from { transform: translateY(0);    opacity: 1; }
  to   { transform: translateY(100%); opacity: 0; }
}

.toast-container {
  position: fixed;
  bottom: 24px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 2000;
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: center;
}
```

**Depends on:** FE-001, FE-004
