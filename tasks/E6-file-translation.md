# E6 — File Translation

> Tham chiếu: Section 5.5 (file input methods), 5.7 (event listeners), 7.1 (IPC), 8.1 (schema), 10.4 (file flow)

---

### US-040 — File Upload (Dialog + Drag-Drop)

> **Epic:** E6 | **Type:** Story | **Size:** M

**Status:** `Todo`

**User Story:**
> As a user, I want to attach a PDF or DOCX file via a file picker or drag-and-drop, so that I can start a file translation.

**Acceptance Criteria:**

**Click attach icon:**
- [ ] Click icon đính kèm (📎) trong InputArea → gọi `OpenFileDialog()` → mở native file picker (Wails)
- [ ] File picker filter chỉ cho chọn `.pdf` hoặc `.docx`
- [ ] Chọn file → nhận `filePath` string → gọi `ReadFileInfo(filePath)`
- [ ] Cancel file picker → không có gì xảy ra

**Drag-Drop:**
- [ ] `InputArea` hoặc toàn bộ chat area bắt `dragover` event → hiện drag overlay (dashed border + "Thả file vào đây")
- [ ] `drop` event → lấy `file.path` từ `DataTransfer.files[0]`
- [ ] Validate extension: chỉ chấp nhận `.pdf` hoặc `.docx` → các loại khác → toast error "Chỉ hỗ trợ PDF và DOCX"
- [ ] Sau khi validate → gọi `ReadFileInfo(filePath)` như flow click

**IPC Used:** `OpenFileDialog()`, `ReadFileInfo(path string)`

**Depends on:** FE-001, FE-003, US-001

---

### US-041 — File Info Preview

> **Epic:** E6 | **Type:** Story | **Size:** S

**Status:** `Todo`

**User Story:**
> As a user, after selecting a file, I want to see a preview chip with file metadata before sending, so that I can confirm it's the correct file.

**Acceptance Criteria:**
- [ ] Sau khi `ReadFileInfo` trả về → hiện `FileAttachment.tsx` chip trong InputArea:
  - File icon (PDF / DOCX icon tùy loại)
  - Tên file (truncated nếu dài)
  - Dung lượng (format: "2.4 MB")
  - Số trang (nếu có)
  - Thời gian ước tính: "~7 phút"
- [ ] Nếu `isScanned = true` → hiện error inline: "PDF scan không hỗ trợ, vui lòng dùng PDF có text"
  - Không cho phép gửi
  - Nút x để clear
- [ ] Nếu `pageCount > 100` → hiện error inline: "Tệp quá lớn (tối đa 100 trang)"
  - Không cho phép gửi
- [ ] Nút `×` trên chip → xóa file preview, clear state
- [ ] Style/Provider chip vẫn hiển thị bình thường (file dùng global settings)
- [ ] Nhấn Send → chuyển sang US-042 flow

**Technical Notes:**

**FE — `FileAttachment.tsx`:**
```tsx
interface FileAttachmentProps {
  fileInfo: FileInfo
  onRemove: () => void
  error?: string
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}
```

**FE — `useFileTranslation.ts` local state:**
```typescript
// local state — không dùng Zustand store
const [fileInfo, setFileInfo] = useState<FileInfo | null>(null)
const [fileError, setFileError] = useState<string | null>(null)

async function handleFileSelect(path: string) {
  const info = await WailsService.readFileInfo(path)
  if (info.isScanned) {
    setFileError('PDF scan không hỗ trợ, vui lòng dùng PDF có text')
    return
  }
  if (info.pageCount && info.pageCount > 100) {
    setFileError('Tệp quá lớn (tối đa 100 trang)')
    return
  }
  setFileInfo(info)
  setFileError(null)
}
```

**IPC Used:** `ReadFileInfo(path string)`

**Depends on:** US-040

---

### US-042 — File Translation Streaming

> **Epic:** E6 | **Type:** Story | **Size:** L

**Status:** `Todo`

**User Story:**
> As a user, after sending a file, I want to see the source text appear on the left and the translation fill in on the right as it progresses, so that I can follow along in real time.

**Acceptance Criteria:**

**Trigger:**
- [ ] User click Send khi có file preview (không có error) → gọi `TranslateFile(req)`
- [ ] InputArea bị disable trong lúc dịch (không cho gửi text/file khác)
- [ ] `FileProgress.tsx` component hiện progress bar với: "Đang dịch... N/M trang (XX%)"

**Event flow:**
- [ ] `file:source` → render LEFT panel (`FileResult.tsx` source side) với source Markdown
  - Markdown được render ra HTML (dùng một Markdown renderer đơn giản hoặc `dangerouslySetInnerHTML`)
- [ ] `file:progress` → update progress bar: `{ chunk, total, percent }`
- [ ] `file:chunk_done` → append chunk text vào RIGHT panel (translation side)
  - Mỗi chunk hiện theo thứ tự `chunkIndex`
  - Smooth scroll xuống cuối right panel khi có chunk mới
- [ ] `file:done` → ẩn progress bar, hiện action buttons (Export, Fullscreen, Copy)
  - Lưu `fileResult` trong local state
- [ ] `file:error` → hiện error toast, cho phép retry

**Technical Notes:**

**FE — `useFileTranslation.ts` event subscription:**
```typescript
// State toàn bộ là local — KHÔNG dùng Zustand
const [sourceMarkdown, setSourceMarkdown] = useState<string>('')
const [translatedChunks, setTranslatedChunks] = useState<TranslatedChunk[]>([])
const [progress, setProgress] = useState<{ chunk: number; total: number; percent: number } | null>(null)
const [fileResult, setFileResult] = useState<FileResult | null>(null)
const [fileTranslating, setFileTranslating] = useState(false)

useEffect(() => {
  const unsub1 = EventsOn('file:source', (payload: { markdown: string }) => {
    setSourceMarkdown(payload.markdown)
  })
  const unsub2 = EventsOn('file:progress', (p: typeof progress) => {
    setProgress(p)
  })
  const unsub3 = EventsOn('file:chunk_done', (chunk: TranslatedChunk) => {
    setTranslatedChunks(prev => [...prev, chunk])
  })
  const unsub4 = EventsOn('file:done', (result: FileResult) => {
    setFileResult(result)
    setFileTranslating(false)
    setProgress(null)
  })
  const unsub5 = EventsOn('file:error', (err: string) => {
    setFileError(err)
    setFileTranslating(false)
  })
  return () => { unsub1(); unsub2(); unsub3(); unsub4(); unsub5() }
}, [])

async function handleFileSend(filePath: string, sessionId: string) {
  setFileTranslating(true)
  setTranslatedChunks([])
  setSourceMarkdown('')
  setFileResult(null)
  await WailsService.translateFile({ sessionId, filePath })
}
```

**BE — `controller/file/translate.go`:**
```go
func (c *controller) TranslateFile(ctx context.Context, req handler.FileRequest) error {
    // 1. Parse file → source.md
    sourceMD, err := c.repo.File().Parse(req.FilePath)
    if err != nil { return err }

    // 2. INSERT file record
    fileID := uuid.New().String()
    c.repo.File().Create(ctx, &model.FileAttachment{
        ID: fileID, SessionID: req.SessionID,
        Status: "processing", ...
    })

    // 3. Emit source
    runtime.EventsEmit(ctx, "file:source", map[string]string{"markdown": sourceMD})

    // 4. Chunk + translate
    chunks := chunkText(sourceMD, 2500) // ~2500 tokens/chunk
    var translatedParts []string
    for i, chunk := range chunks {
        translated, err := c.gateway.AI().TranslateChunk(ctx, chunk, req)
        if err != nil {
            runtime.EventsEmit(ctx, "file:error", err.Error())
            c.repo.File().UpdateStatus(ctx, fileID, "error", err.Error())
            return err
        }
        translatedParts = append(translatedParts, translated)
        runtime.EventsEmit(ctx, "file:progress", map[string]any{
            "chunk": i + 1, "total": len(chunks),
            "percent": (i+1)*100/len(chunks),
        })
        runtime.EventsEmit(ctx, "file:chunk_done", map[string]any{
            "chunkIndex": i, "text": translated,
        })
    }

    // 5. Save translated.md
    translatedMD := strings.Join(translatedParts, "\n\n")
    c.repo.File().SaveTranslated(ctx, fileID, translatedMD)
    c.repo.File().UpdateStatus(ctx, fileID, "done", "")

    // 6. Emit done
    runtime.EventsEmit(ctx, "file:done", handler.FileResult{
        FileID: fileID, FileName: req.FileName,
        FileType: req.FileType, PageCount: len(chunks),
    })
    return nil
}
```

**IPC Used:** `TranslateFile(req FileRequest)`

**Events:** `file:source`, `file:progress`, `file:chunk_done`, `file:done`, `file:error`

**Depends on:** US-041, BE-005, BE-006

---

### US-043 — File Result Display

> **Epic:** E6 | **Type:** Story | **Size:** M

**Status:** `Todo`

**User Story:**
> As a user, after file translation completes, I want to see both the source and translated text side-by-side, with options to export, copy, or view fullscreen.

**Acceptance Criteria:**
- [ ] `FileResult.tsx` hiển thị bilingual layout:
  - LEFT panel: source Markdown (rendered)
  - RIGHT panel: translated text (filled theo chunks)
  - Scroll hai panel đồng bộ (scroll-lock option — CSS `overflow-y: auto` cả 2 col)
- [ ] Action bar sau khi done:
  - **Export** → mở `ExportMenu` (PDF | DOCX) → `ExportFile(fileId, format)` (xem US-051)
  - **Fullscreen** → mở `FullscreenModal.tsx` với cùng bilingual layout
  - **Copy** → copy bản dịch vào clipboard (client-side)
- [ ] Card footer: tên file + số trang + thời gian hoàn thành (HH:MM)
- [ ] Nếu session được tạo mới → session title = tên file (không có `.pdf`/`.docx`)
- [ ] `GetFileContent(fileId)` — dùng khi load lại session từ history:
  - Lazy load source + translated markdown từ disk
  - Render lại `FileResult.tsx` từ disk content

**Technical Notes:**

**FE — File session title:**
```typescript
// Trong useFileTranslation.ts
function getFileTitleForSession(fileName: string): string {
  return fileName.replace(/\.(pdf|docx)$/i, '').slice(0, 50)
}
```

**FE — Load file content từ history:**
```typescript
// Trong MessageFeed khi render file message (message.fileId != null)
useEffect(() => {
  if (msg.fileId && !fileContent) {
    WailsService.getFileContent(msg.fileId).then(setFileContent)
  }
}, [msg.fileId])
```

**IPC Used:** `GetFileContent(fileId string)`, `ExportFile(fileId, format string)`

**Depends on:** US-042, US-051
