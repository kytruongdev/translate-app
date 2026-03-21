# TranslateApp — Tasks & User Stories

> **Phiên bản:** v1.0
> **Cập nhật:** 2026-03-21
> **Tham chiếu:** `doc/architecture-document.md` v1.8

---

## Epic Index

| Epic | Tên | File | Tickets | Priority |
|------|-----|------|---------|----------|
| E1 | Foundation & Infrastructure | [E1-foundation.md](E1-foundation.md) | BE-001 → BE-009, FE-001 → FE-004 | P0 |
| E2 | App Shell & Navigation | [E2-app-shell.md](E2-app-shell.md) | US-001 → US-003 | P0 |
| E3 | Session Management | [E3-session.md](E3-session.md) | US-010 → US-013 | P0 |
| E4 | Text Translation | [E4-text-translation.md](E4-text-translation.md) | US-020 → US-026 | P0 |
| E5 | Retranslate | [E5-retranslate.md](E5-retranslate.md) | US-030 | P1 |
| E6 | File Translation | [E6-file-translation.md](E6-file-translation.md) | US-040 → US-043 | P1 |
| E7 | Export & Copy | [E7-export-copy.md](E7-export-copy.md) | US-050 → US-053 | P1 |
| E8 | View Controls | [E8-view-controls.md](E8-view-controls.md) | US-060 → US-061 | P2 |
| E9 | Settings | [E9-settings.md](E9-settings.md) | US-070 → US-074 | P1 |
| E10 | Error Handling | [E10-error-handling.md](E10-error-handling.md) | US-080 → US-082 | P1 |

---

## Recommended Sprint Plan (2–3 người)

### Sprint 1 — Foundation + Shell + Session
**Goal:** App chạy được, có sidebar, có session list, có Start Page.

| Ticket | Mô tả | Size |
|--------|-------|------|
| BE-001 | Wails project scaffold + clean arch | M |
| BE-002 | SQLite + migrations | M |
| BE-003 | sqlc setup + queries | M |
| BE-004 | Repository layer | L |
| BE-008 | Controller layer skeleton | M |
| BE-009 | Handler + DI wiring | M |
| FE-001 | Frontend project setup | S |
| FE-002 | Zustand stores | S |
| FE-003 | Wails service wrapper | S |
| FE-004 | Global styles | S |
| US-001 | App Shell layout | M |
| US-002 | Sidebar collapse/expand | S |
| US-003 | Start Page | S |
| US-010 | Session list + grouping | M |
| US-013 | Pin/Unpin session | S |
| US-012 | Rename session | S |

### Sprint 2 — Core Translation (P0)
**Goal:** User có thể dịch text, xem kết quả, chọn ngôn ngữ và style.

| Ticket | Mô tả | Size |
|--------|-------|------|
| BE-005 | AI Provider Gemini | M |
| BE-006 | AI Provider Ollama | S |
| BE-007 | AI Provider OpenAI | S |
| US-011 | Create session (atomic) | M |
| US-020 | LangChip + language popover | M |
| US-021 | StyleChip | S |
| US-022 | Input detection | S |
| US-023 | Send message + optimistic UI | M |
| US-024 | Translation streaming | L |
| US-025 | Message display types | L |
| US-026 | Message pagination | M |
| US-060 | View toggle | S |
| US-061 | Fullscreen modal | M |

### Sprint 3 — Extended Features (P1)
**Goal:** Retranslate, file dịch, export, settings, error handling.

| Ticket | Mô tả | Size |
|--------|-------|------|
| US-030 | Retranslate (reply-quote) | L |
| US-040 | File upload | M |
| US-041 | ReadFileInfo preview | S |
| US-042 | File translation streaming | L |
| US-043 | File result display | M |
| US-050 | Copy translation | XS |
| US-051 | Export message | M |
| US-052 | Export session | M |
| US-053 | Export file | M |
| US-070 | Settings entry point | S |
| US-071 | AI model selection | S |
| US-072 | Theme selection | S |
| US-073 | Default style selection | XS |
| US-074 | App startup load settings | S |
| US-080 | Translation error handling | S |
| US-081 | File error handling | S |
| US-082 | API error handling | S |

---

## Dependency Map

```
E1 (Foundation)
  └── E2 (App Shell)
        └── E3 (Session)
              └── E4 (Text Translation)
                    ├── E5 (Retranslate)
                    ├── E7 (Export & Copy)
                    └── E8 (View Controls)
        └── E6 (File Translation)
              └── E7 (Export & Copy)
  └── E9 (Settings)       ← có thể làm song song với E2
  └── E10 (Error Handling) ← làm song song với E4/E5/E6
```

---

## Ticket Size Reference

| Size | Thời gian ước tính |
|------|-------------------|
| XS | < 2 giờ |
| S | nửa ngày |
| M | 1 ngày |
| L | 2–3 ngày |
| XL | 1 tuần+ |

---

## Conventions

- **BE-XXX** — Backend-only task (Go)
- **FE-XXX** — Frontend-only task (TypeScript/React)
- **US-XXX** — User Story (có cả BE + FE tasks)
- Tất cả IPC methods tham chiếu theo `doc/architecture-document.md` Section 7
- Tất cả DB schema tham chiếu Section 8
- Tất cả flows tham chiếu Section 10
