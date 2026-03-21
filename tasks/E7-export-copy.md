# E7 — Export & Copy

> Tham chiếu: Section 5.8 (card action bar), 7.1 (IPC Export methods), 10.5 (Export flow), 10.6 (Copy flow)

---

### US-050 — Export Message (PDF / DOCX)

> **Epic:** E7 | **Type:** Story | **Size:** M

**Status:** `Todo`

**User Story:**
> As a user, I want to export a single translated message to PDF or DOCX, so that I can save and share the translation.

**Acceptance Criteria:**

**Trigger:**
- [ ] Hover vào assistant message → action buttons hiện → click **Export** (icon download)
- [ ] Click → mở `ExportMenu.tsx` popover (position-aware, không bị clip khỏi màn hình)
- [ ] ExportMenu hiện 2 options: **PDF** | **DOCX**
- [ ] Click format → đóng popover → gọi `ExportMessage(id, format)`

**Go flow:**
- [ ] Go load `translated_content` từ SQLite
- [ ] Generate file (format: heading, paragraph, Markdown-to-styled export)
- [ ] Mở `runtime.SaveFileDialog` → user chọn vị trí lưu
- [ ] Return saved path

**FE response:**
- [ ] Toast: "✅ Đã lưu: {filename}" + nút "Mở file" (gọi `shell.Open(path)` nếu Wails hỗ trợ)
- [ ] Nếu user cancel dialog → không có toast
- [ ] Nếu export thất bại → toast error: "Xuất file thất bại. Vui lòng thử lại."

**Technical Notes:**

**BE — `controller/file/export.go`:**
```go
func (c *controller) ExportMessage(ctx context.Context, id, format string) (string, error) {
    msg, err := c.repo.Message().GetByID(ctx, id)
    if err != nil { return "", err }

    content := msg.TranslatedContent  // Markdown string
    tmpPath, err := c.repo.File().Export(content, format, msg.ID)
    if err != nil { return "", err }

    savePath, err := runtime.SaveFileDialog(ctx, runtime.SaveDialogOptions{
        DefaultFilename: fmt.Sprintf("translation_%s.%s", msg.ID[:8], format),
        Filters: []runtime.FileFilter{{ DisplayName: strings.ToUpper(format), Pattern: "*."+format }},
    })
    if savePath == "" { return "", nil }  // user cancelled

    if err := os.Rename(tmpPath, savePath); err != nil { return "", err }
    return savePath, nil
}
```

**FE — `ExportMenu.tsx`:**
```tsx
interface ExportMenuProps {
  messageId: string
  anchorRect: DOMRect
  onClose: () => void
}

export default function ExportMenu({ messageId, anchorRect, onClose }: ExportMenuProps) {
  async function handleExport(format: 'pdf' | 'docx') {
    onClose()
    const savedPath = await WailsService.exportMessage(messageId, format)
    if (savedPath) {
      toast.success(`Đã lưu: ${savedPath.split('/').pop()}`)
    }
  }

  return (
    <div className="export-menu" style={positionFromRect(anchorRect)}>
      <button onClick={() => handleExport('pdf')}>PDF</button>
      <button onClick={() => handleExport('docx')}>DOCX</button>
    </div>
  )
}
```

**IPC Used:** `ExportMessage(id, format string)`

**Depends on:** US-025, BE-008

---

### US-051 — Export File Translation (PDF / DOCX)

> **Epic:** E7 | **Type:** Story | **Size:** M

**Status:** `Todo`

**User Story:**
> As a user, I want to export a complete file translation to PDF or DOCX, so that I can save the full translated document.

**Acceptance Criteria:**
- [ ] Export button hiện sau khi `file:done` event nhận được (xem US-043)
- [ ] Click → mở `ExportMenu.tsx` popover với 2 options: **PDF** | **DOCX**
- [ ] Click format → gọi `ExportFile(fileId, format)`
- [ ] Go load translated.md từ disk path (`files.translated_path`)
- [ ] Generate bilingual layout: LEFT = source, RIGHT = translation (hoặc translation only?)
  - **V1 decision:** export translation only (single-column) — đơn giản hơn
- [ ] `runtime.SaveFileDialog` → user chọn vị trí
- [ ] Return saved path → toast success

**Technical Notes:**

**BE — `controller/file/export.go`:**
```go
func (c *controller) ExportFile(ctx context.Context, fileId, format string) (string, error) {
    fileRecord, err := c.repo.File().GetByID(ctx, fileId)
    if err != nil { return "", err }

    // Read translated.md từ disk
    content, err := os.ReadFile(fileRecord.TranslatedPath)
    if err != nil { return "", err }

    tmpPath, err := c.repo.File().Export(string(content), format, fileId)
    if err != nil { return "", err }

    savePath, err := runtime.SaveFileDialog(ctx, runtime.SaveDialogOptions{
        DefaultFilename: strings.TrimSuffix(fileRecord.FileName, filepath.Ext(fileRecord.FileName)) + "_translated." + format,
    })
    if savePath == "" { return "", nil }
    return savePath, os.Rename(tmpPath, savePath)
}
```

**IPC Used:** `ExportFile(fileId, format string)`

**Depends on:** US-042, BE-008

---

### US-052 — Export Session (PDF / DOCX)

> **Epic:** E7 | **Type:** Story | **Size:** M

**Status:** `Todo`

**User Story:**
> As a user, I want to export all translations in a session to a single file, so that I can save the complete conversation history.

**Acceptance Criteria:**
- [ ] Trigger: nút Export ở header của ChatPage (session-level) hoặc action trong SessionMenu (nếu thiết kế có)
  - **V1 decision:** nút Export ở top-right của ChatPage header
- [ ] Click → `ExportMenu` popover: PDF | DOCX
- [ ] Click format → `ExportSession(sessionId, format)`
- [ ] Go load tất cả messages của session (role=assistant, sorted by display_order)
- [ ] Generate: heading = session title, mỗi message = 1 section (timestamp + translated content)
- [ ] Messages là file translation → bỏ qua (chỉ export text translation)
- [ ] `runtime.SaveFileDialog` → user chọn vị trí
- [ ] Return saved path → toast

**Technical Notes:**

**BE — `controller/session/export.go` (hoặc `controller/file/export.go`):**
```go
func (c *controller) ExportSession(ctx context.Context, id, format string) (string, error) {
    session, err := c.repo.Session().GetByID(ctx, id)
    if err != nil { return "", err }

    messages, err := c.repo.Message().ListAll(ctx, id)  // load all, no pagination
    if err != nil { return "", err }

    // Build content
    var builder strings.Builder
    builder.WriteString(fmt.Sprintf("# %s\n\n", session.Title))
    for _, msg := range messages {
        if msg.Role != "assistant" || msg.FileID != nil { continue }
        t := msg.CreatedAt.Format("15:04")
        builder.WriteString(fmt.Sprintf("## %s · %s\n\n%s\n\n", t, msg.Style, msg.TranslatedContent))
    }

    tmpPath, err := c.repo.File().Export(builder.String(), format, id)
    if err != nil { return "", err }

    savePath, _ := runtime.SaveFileDialog(ctx, runtime.SaveDialogOptions{
        DefaultFilename: session.Title + "." + format,
    })
    if savePath == "" { return "", nil }
    return savePath, os.Rename(tmpPath, savePath)
}
```

**IPC Used:** `ExportSession(id, format string)`

**Depends on:** US-010, BE-008

---

### US-053 — Copy Translation

> **Epic:** E7 | **Type:** Story | **Size:** S

**Status:** `Todo`

**User Story:**
> As a user, I want to copy a translation to my clipboard with a single click, so that I can paste it elsewhere quickly.

**Acceptance Criteria:**
- [ ] Hover vào assistant message → action buttons → click **Copy** (icon clipboard)
- [ ] Client-side only — không gọi Go IPC:
  - `TranslationCard` (bilingual): lấy text từ `.translation-panel.dest .panel-body` → `innerText`
  - `MessageBubble` (bubble mode): lấy text từ `.chat-bubble` → `innerText`
- [ ] Gọi `navigator.clipboard.writeText(text)`
- [ ] Button icon thay đổi 1s (icon check ✓) sau đó revert về icon clipboard
- [ ] Nếu `navigator.clipboard` không khả dụng → fallback: `document.execCommand('copy')`

**Technical Notes:**

**FE — Copy handler (inline trong card component):**
```typescript
async function handleCopy(e: React.MouseEvent) {
  const button = e.currentTarget
  const text = extractTranslationText(messageId)  // lấy từ DOM

  try {
    await navigator.clipboard.writeText(text)
  } catch {
    // fallback
    const textarea = document.createElement('textarea')
    textarea.value = text
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
  }

  button.classList.add('copied')
  setTimeout(() => button.classList.remove('copied'), 1000)
}
```

> Copy là **client-side only**. `CopyTranslation(messageId)` IPC method chỉ cần implement nếu có edge case nơi DOM không accessible.

**Depends on:** US-025
