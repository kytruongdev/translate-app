# E2 — App Shell & Navigation

> Tham chiếu: Section 5.1 (folder structure), 5.3 (routing), 10.0 (startup flow)

---

### US-001 — App Shell Layout

> **Epic:** E2 | **Type:** Story | **Size:** M

**Status:** `Done` *(`App.tsx`: `.app-shell` + `nav-drawer` + `main` + start/chat theo `activeSessionId`; **không** dùng `MemoryRouter` / `AppShell.tsx` / route `/chat/:id` như spec — điều hướng bằng state)*

**User Story:**
> As a user, I want to see a consistent layout with a sidebar and main content area, so that I can navigate between sessions and translations easily.

**Acceptance Criteria:**
- [ ] `AppShell.tsx` render layout 2 cột: `Sidebar` (trái, 360px) + `main` (phải, flex-1)
- [ ] `MemoryRouter` setup với routes:
  - `/start` → `StartPage`
  - `/chat/:sessionId` → `ChatPage`
  - Default redirect `/` → `/start`
- [ ] `App.tsx` là root component, render `providers.tsx` wrapper + `AppShell`
- [ ] `providers.tsx` wrap Zustand providers (nếu cần) + router
- [ ] Layout không có scrollbar ngang, chiều cao = 100vh, overflow hidden

**Technical Notes:**

**FE:**
- `src/app/App.tsx`:
  ```tsx
  export default function App() {
    return (
      <Providers>
        <AppShell />
      </Providers>
    )
  }
  ```
- `src/components/layout/AppShell.tsx`:
  ```tsx
  export default function AppShell() {
    return (
      <div className="app-shell">
        <Sidebar />
        <main className="main">
          <Routes>
            <Route path="/start" element={<StartPage />} />
            <Route path="/chat/:sessionId" element={<ChatPage />} />
            <Route path="*" element={<Navigate to="/start" replace />} />
          </Routes>
        </main>
      </div>
    )
  }
  ```
- CSS: `.app-shell { display: flex; height: 100vh; overflow: hidden; }`

**Depends on:** FE-001, FE-002, FE-004

---

### US-002 — Sidebar Collapse/Expand

> **Epic:** E2 | **Type:** Story | **Size:** S

**Status:** `Done` *(`useUIStore.sidebarCollapsed`, nút menu header, `.nav-drawer.collapsed` trong `shell.css` / `mockup-override.css`; ẩn Ghim / label khi collapsed)*

**User Story:**
> As a user, I want to collapse the sidebar to have more space for reading translations, so that I can focus on the content.

**Acceptance Criteria:**
- [ ] Sidebar có nút toggle collapse/expand (icon menu ☰ hoặc chevron)
- [ ] Khi collapsed: sidebar width = 64px, chỉ hiện icons, label ẩn, session group labels ẩn, session titles ẩn
- [ ] Khi expanded: sidebar width = 360px, đầy đủ text
- [ ] State `sidebarCollapsed` persist trong `uiStore` (in-memory, không cần persist qua restart)
- [ ] Transition animation smooth (0.2s ease)
- [ ] Sidebar luôn hiện ở mọi route (`/start` và `/chat/:sessionId`)
- [ ] Khi collapsed: nhóm "GHIM" ẩn hoàn toàn (không hiện icon), sidebar-group-labels ẩn

**Technical Notes:**

**FE:**
- `uiStore.sidebarCollapsed` + `uiStore.setSidebarCollapsed(v: boolean)`
- `Sidebar.tsx` nhận class `nav-drawer` + conditional class `collapsed`
- CSS: `.nav-drawer { width: 360px; transition: width 0.2s var(--ease); }`
- `.nav-drawer.collapsed { width: 64px; }`
- `.nav-drawer.collapsed .sidebar-group-label { display: none; }`
- `.nav-drawer.collapsed .pinned-group { display: none; }`

**Depends on:** US-001, FE-002

---

### US-003 — Start Page

> **Epic:** E2 | **Type:** Story | **Size:** S

**Status:** `Partial` *(`StartHello` + `ChatInputBar` khi `activeSessionId == null`; Phiên mới → `setActiveSession(null)`; **không** `StartPage.tsx` / `InputArea.tsx` / route `/start`; nút **đính kèm** có UI nhưng **chưa** nối E6)*

**User Story:**
> As a user, when I open the app or click "Phiên mới", I want to see a welcoming Start Page with an input area, so that I can immediately start translating.

**Acceptance Criteria:**
- [ ] `StartPage.tsx` hiển thị khi `activeSessionId = null` (route `/start`)
- [ ] Greeting text ở giữa màn hình (dùng font MTD Geraldyne nếu có, fallback Inter): "Xin chào" / "Hi there"
- [ ] Input area (`InputArea.tsx`) được render ở Start Page — cùng component với ChatPage input
- [ ] Input area bao gồm: textarea, LangChip, StyleChip, nút attach file, nút send
- [ ] Sidebar có nút "Phiên mới" (`btn-new-session`) ở top — click → navigate `/start`, clear active session
- [ ] Khi `activeSessionId = null`: không có session nào active trong sidebar
- [ ] App load → đọc `activeSessionId` từ `sessionStore` → nếu null → `/start`

**Technical Notes:**

**FE:**
- `src/pages/StartPage.tsx`:
  ```tsx
  export default function StartPage() {
    return (
      <div className="start-view">
        <div className="start-greeting">
          <h1>Xin chào</h1>
        </div>
        <InputArea />
      </div>
    )
  }
  ```
- `sessionStore.setActiveSession(null)` khi click "Phiên mới"
- Navigate to `/start` via `useNavigate()`
- Sidebar nút "Phiên mới": `onClick={() => { sessionStore.setActiveSession(null); navigate('/start') }}`

**Depends on:** US-001, FE-002

---

### US-004 — App Startup Flow

> **Epic:** E2 | **Type:** Story | **Size:** S

**Status:** `Partial` *(`useEffect`: `loadSettings()` + `loadSessions()` khi mount; theme qua `loadSettings` → `applyTheme`; **chưa** gate “không render đến khi xong / timeout 3s”, **chưa** toast khi `GetSessions` fail, **không** restore `activeSession` sau restart — mở app về start nếu chưa chọn phiên)*

**User Story:**
> As a user, when I open the app, I want to see my previous sessions loaded in the sidebar and the app ready to use, so that I can continue where I left off.

**Acceptance Criteria:**
- [ ] App startup sequence (trong `App.tsx` hoặc `useEffect` top-level):
  1. `GetSettings()` → populate `settingsStore` + apply theme (`data-theme` attribute)
  2. `GetSessions()` → populate `sessionStore.sessions`
  3. Navigate đến `/start` (không restore active session qua restart — fresh start mỗi lần)
- [ ] Nếu `GetSettings()` fail → dùng default values, không crash app
- [ ] Nếu `GetSessions()` fail → sidebar trống, hiện toast error nhẹ
- [ ] Loading state: app không render gì cho đến khi settings + sessions load xong (hoặc timeout 3s)

**Technical Notes:**

**BE:**
- `controller/settings/get.go` — `GetSettings()` trả về `model.Settings` từ `settings` table
- Map từng key: `theme`, `active_provider`, `active_model`, `active_style`

**FE:**
- `src/app/App.tsx` dùng `useEffect` để load:
  ```tsx
  useEffect(() => {
    Promise.all([
      WailsService.getSettings().then(s => settingsStore.setAll(s)),
      WailsService.getSessions().then(ss => sessionStore.setSessions(ss)),
    ]).finally(() => setReady(true))
  }, [])
  ```
- Apply theme: `document.documentElement.setAttribute('data-theme', settings.theme)`

**IPC Used:** `GetSettings()`, `GetSessions()`

**Depends on:** US-001, FE-002, FE-003, BE-009
