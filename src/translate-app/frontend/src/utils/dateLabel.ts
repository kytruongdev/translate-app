const DAY_NAMES = ['CN', 'Thứ 2', 'Thứ 3', 'Thứ 4', 'Thứ 5', 'Thứ 6', 'Thứ 7']

function startOfDay(d: Date): Date {
  const x = new Date(d)
  x.setHours(0, 0, 0, 0)
  return x
}

function calendarDaysBefore(now: Date, day: Date): number {
  const a = startOfDay(now).getTime()
  const b = startOfDay(day).getTime()
  return Math.round((a - b) / 86400000)
}

function shortDate(d: Date): string {
  return `${d.getDate()}/${d.getMonth() + 1}/${d.getFullYear()}`
}

/**
 * Logic nhãn ngày thống nhất cho sidebar và chat feed:
 * - Hôm nay / Hôm qua
 * - 2–3 ngày trước → "X ngày trước, D/M/YYYY"
 * - ≥4 ngày       → "Thứ X, D/M/YYYY"
 */
export function formatDateLabel(date: Date, now = new Date()): string {
  const n = calendarDaysBefore(now, date)
  if (n === 0) return 'Hôm nay'
  if (n === 1) return 'Hôm qua'
  const ds = shortDate(date)
  if (n <= 3) return `${n} ngày trước, ${ds}`
  return `${DAY_NAMES[date.getDay()]}, ${ds}`
}

/** Key ổn định cho grouping (YYYY-MM-DD) */
export function dateGroupKey(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}
