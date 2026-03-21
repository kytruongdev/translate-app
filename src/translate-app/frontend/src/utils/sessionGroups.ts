import type { Session } from '@/types/session'

export interface SessionDayGroup {
  key: string
  label: string
  sessions: Session[]
}

function startOfDay(d: Date): Date {
  const x = new Date(d)
  x.setHours(0, 0, 0, 0)
  return x
}

/** Số ngày (theo lịch) từ `day` đến `now` — 0 = cùng ngày. */
function calendarDaysBefore(now: Date, day: Date): number {
  const a = startOfDay(now).getTime()
  const b = startOfDay(day).getTime()
  return Math.round((a - b) / 86400000)
}

function labelForDay(now: Date, day: Date): { key: string; label: string } {
  const n = calendarDaysBefore(now, day)
  if (n === 0) return { key: 'today', label: 'Hôm nay' }
  if (n === 1) return { key: 'yesterday', label: 'Hôm qua' }
  if (n >= 2 && n < 7) return { key: `ago-${n}`, label: `${n} ngày trước` }
  const key = `${day.getFullYear()}-${String(day.getMonth() + 1).padStart(2, '0')}-${String(day.getDate()).padStart(2, '0')}`
  const label = day.toLocaleDateString('vi-VN', {
    weekday: 'long',
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
  })
  return { key, label }
}

/** Nhóm phiên (đã loại pinned) theo ngày cập nhật — mới nhất trước. */
export function groupSessionsByDay(sessions: Session[], now = new Date()): SessionDayGroup[] {
  const map = new Map<string, { label: string; sessions: Session[] }>()

  for (const s of sessions) {
    const d = new Date(s.updatedAt)
    if (Number.isNaN(d.getTime())) continue
    const { key, label } = labelForDay(now, d)
    if (!map.has(key)) {
      map.set(key, { label, sessions: [] })
    }
    map.get(key)!.sessions.push(s)
  }

  const groups: SessionDayGroup[] = []
  for (const [key, { label, sessions }] of map) {
    sessions.sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())
    groups.push({ key, label, sessions })
  }

  groups.sort((a, b) => {
    const ta = Math.max(0, ...a.sessions.map((s) => new Date(s.updatedAt).getTime()))
    const tb = Math.max(0, ...b.sessions.map((s) => new Date(s.updatedAt).getTime()))
    return tb - ta
  })

  return groups
}
