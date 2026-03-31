import type { ThemeMode } from '@/types/settings'

/** Maps persisted theme + optional OS preference to `data-theme` on `<html>`. */
export function applyTheme(theme: ThemeMode): void {
  const root = document.documentElement
  if (theme === 'system') {
    const dark = window.matchMedia?.('(prefers-color-scheme: dark)').matches
    root.setAttribute('data-theme', dark ? 'dark' : 'light')
    return
  }
  root.setAttribute('data-theme', theme)
}
