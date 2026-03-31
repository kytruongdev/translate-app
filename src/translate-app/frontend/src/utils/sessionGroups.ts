import type { Session } from '@/types/session'
import { formatDateLabel, dateGroupKey } from '@/utils/dateLabel'

export interface SessionDayGroup {
  key: string
  label: string
  sessions: Session[]
}

function labelForDay(now: Date, day: Date): { key: string; label: string } {
  const key = dateGroupKey(day)
  const label = formatDateLabel(day, now)
  // sidebar dùng key cố định cho today/yesterday để CSS target dễ hơn
  if (label === 'Hôm nay') return { key: 'today', label }
  if (label === 'Hôm qua') return { key: 'yesterday', label }
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
