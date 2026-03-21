# E9 — Settings

> Tham chiếu: Section 5.9 (Settings UX), 7.1 (IPC: GetSettings, SaveSettings), 8.1 (settings table), 10.0 (startup)

---

### US-070 — Settings Popover (Entry Point)

> **Epic:** E9 | **Type:** Story | **Size:** S

**Status:** `Todo`

**User Story:**
> As a user, I want to access settings from a button at the bottom of the sidebar, so that I can quickly configure the app without navigating away.

**Acceptance Criteria:**
- [ ] Nút **"Setting"** ở cuối sidebar (bottom-left), luôn hiện dù sidebar collapsed hay expanded
- [ ] Click nút → mở `SettingsPopover.tsx` ngay phía trên nút (anchor = nút, popover mở lên trên)
- [ ] Popover chứa 2 rows:
  - **Model AI** (icon 🖥 hoặc CPU icon) — click → đóng popover → mở `ModelAIModal`
  - **Giao diện** (icon ☀️ hoặc theme icon) — hover → submenu trượt sang phải
- [ ] Click ngoài popover hoặc press **Escape** → đóng popover
- [ ] Nếu sidebar collapsed → popover vẫn mở đúng vị trí (anchor là icon nút Setting)

**Technical Notes:**

**FE — `SettingsPopover.tsx`:**
```tsx
interface SettingsPopoverProps {
  anchorRect: DOMRect
  onClose: () => void
}

export default function SettingsPopover({ anchorRect, onClose }: SettingsPopoverProps) {
  const [showModelAI, setShowModelAI] = useState(false)

  function openModelAI() {
    onClose()              // đóng popover trước
    setShowModelAI(true)   // sau đó hiện modal
  }

  // Position: ngay trên anchor button
  const style = {
    position: 'fixed' as const,
    bottom: window.innerHeight - anchorRect.top + 8,
    left: anchorRect.left,
  }

  return (
    <>
      <div className="settings-popover" style={style}>
        <button className="popover-item" onClick={openModelAI}>
          <span className="item-icon">🖥</span>
          <span>Model AI</span>
        </button>
        <div className="popover-item has-submenu">
          <span className="item-icon">☀️</span>
          <span>Giao diện</span>
          <ThemeSubmenu />  {/* submenu hiện khi hover */}
        </div>
      </div>
      {showModelAI && <ModelAIModal onClose={() => setShowModelAI(false)} />}
    </>
  )
}
```

**Depends on:** US-001, US-002, FE-002

---

### US-071 — ModelAI Modal (Provider + Style)

> **Epic:** E9 | **Type:** Story | **Size:** M

**Status:** `Todo`

**User Story:**
> As a user, I want to choose between Online and Offline AI models and set a default translation style, so that the app uses my preferred settings for all translations.

**Acceptance Criteria:**

**Open:**
- [ ] Click "Model AI" trong SettingsPopover → SettingsPopover đóng → `ModelAIModal.tsx` mở
- [ ] Modal nhỏ, khoảng 1/3 từ trên màn hình (vị trí center-top)
- [ ] Title: "Model AI"

**Nội dung:**
- [ ] **Row 1 — Chọn Model** (dropdown):
  - `Online` — maps to `activeProvider = 'gemini'` (Gemini Flash)
  - `Offline` — maps to `activeProvider = 'ollama'` (Qwen2.5:7b)
  - Default hiện: giá trị hiện tại của `settingsStore.activeProvider`
  - ⚠️ GPT-4o-mini **không có trong dropdown này** — V2
- [ ] **Row 2 — Kiểu dịch mặc định** (dropdown):
  - `Phổ thông` → `'casual'` (default)
  - `Học thuật` → `'academic'`
  - `Kinh Doanh` → `'business'`
  - Default hiện: giá trị hiện tại của `settingsStore.defaultStyle`

**Actions:**
- [ ] **Hủy** → đóng modal, không lưu
- [ ] **Lưu** → gọi `SaveSettings({ activeProvider: selected, defaultStyle: selectedStyle })` → đóng modal
- [ ] Sau khi Lưu → `settingsStore.saveSettings(partial)` update store
- [ ] Press **Escape** → đóng modal (như "Hủy")

**Technical Notes:**

**FE — `ModelAIModal.tsx`:**
```tsx
export default function ModelAIModal({ onClose }: { onClose: () => void }) {
  const { activeProvider, defaultStyle } = useSettingsStore()
  const [selectedProvider, setSelectedProvider] = useState(activeProvider)
  const [selectedStyle, setSelectedStyle] = useState(defaultStyle)

  async function handleSave() {
    await WailsService.saveSettings({
      activeProvider: selectedProvider,
      defaultStyle: selectedStyle,
    })
    useSettingsStore.getState().saveSettings({
      activeProvider: selectedProvider,
      defaultStyle: selectedStyle,
    })
    onClose()
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="model-ai-modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header"><h2>Model AI</h2></div>
        <div className="modal-body">
          <div className="settings-row">
            <label>Chọn Model</label>
            <select value={selectedProvider} onChange={e => setSelectedProvider(e.target.value)}>
              <option value="gemini">Online (Gemini Flash)</option>
              <option value="ollama">Offline (Qwen2.5:7b)</option>
            </select>
          </div>
          <div className="settings-row">
            <label>Kiểu dịch mặc định</label>
            <select value={selectedStyle} onChange={e => setSelectedStyle(e.target.value as TranslationStyle)}>
              <option value="casual">Phổ thông</option>
              <option value="academic">Học thuật</option>
              <option value="business">Kinh Doanh</option>
            </select>
          </div>
        </div>
        <div className="dialog-actions">
          <button onClick={onClose}>Hủy</button>
          <button className="primary" onClick={handleSave}>Lưu</button>
        </div>
      </div>
    </div>
  )
}
```

**BE — `controller/settings/save.go`:**
```go
func (c *controller) SaveSettings(ctx context.Context, s model.Settings) error {
    entries := map[string]string{
        "theme":           s.Theme,
        "active_provider": s.ActiveProvider,
        "active_model":    s.ActiveModel,
        "active_style":    string(s.DefaultStyle),
    }
    for key, val := range entries {
        if val == "" { continue }
        if err := c.repo.Settings().Set(ctx, key, val); err != nil { return err }
    }
    return nil
}
```

**IPC Used:** `SaveSettings(s Settings)`

**Depends on:** US-070, FE-002, BE-008

---

### US-072 — Theme Switcher (Sáng / Tối / Hệ thống)

> **Epic:** E9 | **Type:** Story | **Size:** S

**Status:** `Todo`

**User Story:**
> As a user, I want to switch between light, dark, and system themes, so that the app matches my preferred environment.

**Acceptance Criteria:**
- [ ] Hover vào "Giao diện" row trong SettingsPopover → `ThemeSubmenu` trượt sang phải (CSS hover-driven)
- [ ] `ThemeSubmenu` có 3 options (radio-style, checkmark trên active):
  - **Hệ thống** (`system`) — default; theo OS `prefers-color-scheme`
  - **Sáng** (`light`)
  - **Tối** (`dark`)
- [ ] Click option → apply **ngay lập tức** (không cần Lưu):
  - `document.documentElement.setAttribute('data-theme', selectedTheme)`
  - Gọi `SaveSettings({ theme: selectedTheme })` (persist vào DB)
  - `settingsStore` update
- [ ] Active option có checkmark, bold text
- [ ] Submenu đóng khi rời khỏi hover area (CSS `:hover` driven, no JS needed)

**Technical Notes:**

**FE — Theme apply logic:**
```typescript
function applyTheme(theme: 'light' | 'dark' | 'system') {
  if (theme === 'system') {
    const isDark = window.matchMedia('(prefers-color-scheme: dark)').matches
    document.documentElement.setAttribute('data-theme', isDark ? 'dark' : 'light')
  } else {
    document.documentElement.setAttribute('data-theme', theme)
  }
}

// Khi user chọn theme
async function handleThemeChange(theme: 'light' | 'dark' | 'system') {
  applyTheme(theme)
  settingsStore.saveSettings({ theme })
  await WailsService.saveSettings({ theme })
}
```

**FE — CSS variables (global.css):**
```css
[data-theme="light"] {
  --surface: #ffffff;
  --on-surface: #1a1a1a;
  --primary: #6750A4;
  /* ... */
}

[data-theme="dark"] {
  --surface: #1c1b1f;
  --on-surface: #e6e1e5;
  --primary: #d0bcff;
  /* ... */
}
```

**FE — ThemeSubmenu CSS:**
```css
.popover-item.has-submenu:hover .theme-submenu { display: block; }
.theme-submenu {
  display: none;
  position: absolute;
  left: 100%;
  top: 0;
  /* ... */
}
```

**IPC Used:** `SaveSettings(s Settings)` (partial — chỉ cần `theme` field)

**Depends on:** US-070, FE-004

---

### US-073 — Load Settings on Startup

> **Epic:** E9 | **Type:** Story | **Size:** S

**Status:** `Todo`

**User Story:**
> As a user, when I open the app, I want my previous settings to be automatically applied, so that the app remembers my preferences.

**Acceptance Criteria:**
- [ ] Startup sequence (trong `App.tsx` `useEffect`):
  1. `GetSettings()` → populate `settingsStore`
  2. Apply theme ngay lập tức: `applyTheme(settings.theme)`
  3. Restore `uiStore.activeTargetLang = settings.last_target_lang ?? 'en-US'`
- [ ] Nếu `GetSettings()` fail → dùng defaults: `{ theme: 'system', activeProvider: 'gemini', defaultStyle: 'casual' }`
- [ ] Nếu settings row chưa có trong DB → Go trả về default values (không error)
- [ ] Theme apply trước khi render để tránh flash of wrong theme (FOWT)

**Technical Notes:**

**BE — `controller/settings/get.go`:**
```go
var defaults = map[string]string{
    "theme":            "system",
    "active_provider":  "gemini",
    "active_model":     "gemini-2.0-flash",
    "active_style":     "casual",
    "last_target_lang": "en-US",
}

func (c *controller) GetSettings(ctx context.Context) (model.Settings, error) {
    result := model.Settings{}
    for key, def := range defaults {
        val, err := c.repo.Settings().Get(ctx, key)
        if err != nil || val == "" { val = def }
        switch key {
        case "theme":           result.Theme = val
        case "active_provider": result.ActiveProvider = val
        case "active_model":    result.ActiveModel = val
        case "active_style":    result.DefaultStyle = model.TranslationStyle(val)
        }
    }
    return result, nil
}
```

**FE — App.tsx startup:**
```tsx
useEffect(() => {
  async function init() {
    const [settings, sessions] = await Promise.allSettled([
      WailsService.getSettings(),
      WailsService.getSessions(),
    ])

    if (settings.status === 'fulfilled') {
      settingsStore.setAll(settings.value)
      applyTheme(settings.value.theme)
      uiStore.setActiveTargetLang(settings.value.lastTargetLang ?? 'en-US')
    } else {
      applyTheme('system')  // fallback
    }

    if (sessions.status === 'fulfilled') {
      sessionStore.setSessions(sessions.value)
    }

    setReady(true)
  }
  init()
}, [])
```

**IPC Used:** `GetSettings()`

**Depends on:** US-004, BE-009

---

### US-074 — Persist last_target_lang

> **Epic:** E9 | **Type:** Story | **Size:** S

**Status:** `Todo`

**User Story:**
> As a user, I want the app to remember the last target language I used, so that I don't have to re-select it every time I open the app.

**Acceptance Criteria:**
- [ ] Mỗi khi user chọn ngôn ngữ đích mới từ LangChip popover:
  - `uiStore.setActiveTargetLang(lang)` — update in-memory
  - Gọi `SaveSettings({ lastTargetLang: lang })` — persist vào `settings` table
- [ ] Key trong DB: `last_target_lang`, value: locale code (e.g. `"ko"`, `"en-GB"`)
- [ ] Khi app khởi động → `GetSettings()` → restore `uiStore.activeTargetLang`
- [ ] Default nếu không có trong DB: `"en-US"`

**Technical Notes:**

**FE — `LangChip.tsx` onChange handler:**
```typescript
async function handleLangChange(lang: string) {
  uiStore.setActiveTargetLang(lang)
  // Fire-and-forget persist
  WailsService.saveSettings({ lastTargetLang: lang }).catch(console.error)
}
```

> `SaveSettings` chấp nhận partial — chỉ update các field được truyền, không ghi đè field khác.

**BE — `repository/settings/save.go`:**
```go
func (r *repo) Set(ctx context.Context, key, value string) error {
    now := time.Now().UTC().Format(time.RFC3339)
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
        ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
    `, key, value, now)
    return err
}
```

**IPC Used:** `SaveSettings(s Settings)` (partial — chỉ `lastTargetLang`)

**Depends on:** US-020, US-073
