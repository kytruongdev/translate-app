# E8 — View Controls

> Tham chiếu: Section 5.8 (action bar visibility, always-on), 10.7 (view toggle), 5.1 (FullscreenModal)

---

### US-060 — View Toggle (Song ngữ / Chỉ bản dịch / Chỉ nguồn)

> **Epic:** E8 | **Type:** Story | **Size:** S

**Status:** `Done` *(`TranslationCardView`: 3 mode + `data-mode`; `TranslationFullscreenModal` có `mode` state **độc lập**, `initialMode` kế thừa khi mở — `useEffect` reset khi `open`)*

**User Story:**
> As a user, I want to toggle between bilingual, translation-only, and source-only views on a translation card, so that I can focus on the content I need.

**Acceptance Criteria:**
- [ ] Toggle **chỉ áp dụng** cho `TranslationCard` (bilingual mode — `displayMode = 'bilingual'`); không áp dụng cho `MessageBubble`
- [ ] 3 trạng thái toggle (segmented control hoặc icon cycle):
  - **Song ngữ** (`bilingual`) — hiện cả 2 panel (mặc định)
  - **Chỉ bản dịch** (`dest`) — ẩn left panel, right panel full width
  - **Chỉ nguồn** (`src`) — ẩn right panel, left panel full width
- [ ] State toggle là **LOCAL** — `useState` trong `TranslationCard.tsx`
  - Không persist vào DB
  - Reset về "Song ngữ" khi reload session
- [ ] Toggle inline card và toggle trong fullscreen modal **ĐỘCLẬP** nhau (2 useState riêng)
- [ ] Khi mở fullscreen từ inline card → fullscreen **INHERIT** state của card tại thời điểm mở
- [ ] CSS-driven implementation:

```css
/* TranslationCard root */
.bilingual-view[data-mode="bilingual"] .source-panel { display: flex; }
.bilingual-view[data-mode="bilingual"] .dest-panel   { display: flex; }

.bilingual-view[data-mode="dest"]     .source-panel { display: none; }
.bilingual-view[data-mode="dest"]     .dest-panel   { display: flex; width: 100%; }

.bilingual-view[data-mode="src"]      .source-panel { display: flex; width: 100%; }
.bilingual-view[data-mode="src"]      .dest-panel   { display: none; }
```

**Technical Notes:**

**FE — `TranslationCard.tsx`:**
```tsx
type ViewMode = 'bilingual' | 'dest' | 'src'

export default function TranslationCard({ message }: { message: Message }) {
  const [viewMode, setViewMode] = useState<ViewMode>('bilingual')
  const [fullscreen, setFullscreen] = useState(false)

  return (
    <div className="bilingual-view" data-mode={viewMode}>
      <div className="card-toolbar">
        <ViewToggle mode={viewMode} onChange={setViewMode} />
        <button className="fullscreen-btn always-on" onClick={() => setFullscreen(true)}>⛶</button>
      </div>
      <div className="source-panel">...</div>
      <div className="dest-panel">...</div>

      {fullscreen && (
        <FullscreenModal
          message={message}
          initialViewMode={viewMode}   // inherit card state khi mở
          onClose={() => setFullscreen(false)}
        />
      )}
    </div>
  )
}
```

**FE — `ViewToggle.tsx`:**
```tsx
const VIEW_OPTIONS: { value: ViewMode; label: string }[] = [
  { value: 'bilingual', label: 'Song ngữ' },
  { value: 'dest',      label: 'Bản dịch' },
  { value: 'src',       label: 'Nguồn' },
]

export default function ViewToggle({ mode, onChange }: ViewToggleProps) {
  return (
    <div className="view-toggle theme-segmented">
      {VIEW_OPTIONS.map(opt => (
        <button
          key={opt.value}
          className={mode === opt.value ? 'active' : ''}
          onClick={() => onChange(opt.value)}
        >
          {opt.label}
        </button>
      ))}
    </div>
  )
}
```

**Depends on:** US-025

---

### US-061 — Fullscreen Modal

> **Epic:** E8 | **Type:** Story | **Size:** M

**Status:** `Partial` *(`TranslationFullscreenModal.tsx`: portal, backdrop, Esc, view toggle độc lập, copy, retranslate popover; export UI gọi `ExportMessage` — **BE chưa implement**; scroll/body lock cần đối chiếu chi tiết spec)*

**User Story:**
> As a user, I want to open a translation in fullscreen, so that I can read it more comfortably with larger text.

**Acceptance Criteria:**

**Open:**
- [ ] Nút **Fullscreen** luôn hiện trên `TranslationCard` (`always-on` — không cần hover)
- [ ] Click → mở `FullscreenModal.tsx` overlay toàn màn hình (z-index cao nhất)
- [ ] Modal hiện bilingual layout giống `TranslationCard` nhưng lớn hơn, full viewport
- [ ] Initial `viewMode` = giá trị hiện tại của inline card tại thời điểm click

**Controls trong modal:**
- [ ] View toggle (Song ngữ / Chỉ bản dịch / Chỉ nguồn) — STATE ĐỘC LẬP với inline card
- [ ] Export button → `ExportMenu` → `ExportMessage(id, format)` (xem US-050)
- [ ] Copy button → copy bản dịch vào clipboard (client-side, xem US-053)
- [ ] Nút close `×` (top-right) → đóng modal
- [ ] Nhấn **Escape** → đóng modal
- [ ] Click backdrop (vùng tối bên ngoài) → đóng modal

**Scroll:**
- [ ] Source panel và dest panel scroll độc lập
- [ ] Không lock scroll ngoài modal khi modal mở (`body { overflow: hidden }` khi modal active)

**Technical Notes:**

**FE — `FullscreenModal.tsx`:**
```tsx
interface FullscreenModalProps {
  message: Message
  initialViewMode: ViewMode
  onClose: () => void
}

export default function FullscreenModal({ message, initialViewMode, onClose }: FullscreenModalProps) {
  const [viewMode, setViewMode] = useState<ViewMode>(initialViewMode)

  // Lock body scroll
  useEffect(() => {
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = '' }
  }, [])

  // Escape key
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  return createPortal(
    <div className="fullscreen-backdrop" onClick={onClose}>
      <div className="fullscreen-modal" onClick={e => e.stopPropagation()}
           data-mode={viewMode}>
        <div className="fullscreen-header">
          <ViewToggle mode={viewMode} onChange={setViewMode} />
          <div className="fullscreen-actions">
            <CopyButton messageId={message.id} />
            <ExportButton messageId={message.id} />
            <button className="close-btn" onClick={onClose}>×</button>
          </div>
        </div>
        <div className="fullscreen-body">
          <div className="source-panel">
            <div className="panel-body">{message.originalContent}</div>
          </div>
          <div className="dest-panel">
            <div className="panel-body">{message.translatedContent}</div>
          </div>
        </div>
      </div>
    </div>,
    document.body
  )
}
```

**CSS:**
```css
.fullscreen-backdrop {
  position: fixed; inset: 0;
  background: rgba(0,0,0,0.6);
  z-index: 1000;
  display: flex; align-items: center; justify-content: center;
}

.fullscreen-modal {
  width: 90vw; height: 85vh;
  background: var(--surface);
  border-radius: var(--radius-xl);
  display: flex; flex-direction: column;
  overflow: hidden;
}

.fullscreen-body {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1px;
  background: var(--divider);
  flex: 1; overflow: hidden;
}

.fullscreen-body .source-panel,
.fullscreen-body .dest-panel {
  overflow-y: auto;
  padding: 24px;
  background: var(--surface);
}
```

**Depends on:** US-025, US-050, US-053, US-060
