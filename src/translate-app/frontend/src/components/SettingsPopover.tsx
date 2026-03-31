import type { ThemeMode } from '@/types/settings'

/* mockup.v1.html — Model AI */
const IconCpu = () => (
  <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden>
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth={2}
      d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17H3a2 2 0 01-2-2V5a2 2 0 012-2h14a2 2 0 012 2v10a2 2 0 01-2 2h-2"
    />
  </svg>
)

/* mockup.v1.html — Giao diện */
const IconTheme = () => (
  <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden>
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth={2}
      d="M12 3v1m0 16v1m8.66-9h-1M4.34 12h-1m15.07-6.07l-.71.71M6.34 17.66l-.71.71m12.02 0l-.71-.71M6.34 6.34l-.71-.71M12 7a5 5 0 100 10A5 5 0 0012 7z"
    />
  </svg>
)

const IconChevronRight = () => (
  <svg className="chevron" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden>
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
  </svg>
)

export function SettingsPopover({
  theme,
  onOpenModelAI,
  onPickTheme,
}: {
  theme: ThemeMode
  onOpenModelAI: () => void
  onPickTheme: (theme: ThemeMode) => void
}) {
  return (
    <div className="settings-popover" role="menu" aria-label="Settings">
      <button type="button" className="config-menu-item" role="menuitem" onClick={onOpenModelAI}>
        <IconCpu />
        <span className="label">Model AI</span>
      </button>
      <div className="config-menu-item-wrap" role="none">
        <button type="button" className="config-menu-item" role="menuitem" aria-haspopup="true">
          <IconTheme />
          <span className="label">Giao diện</span>
          <IconChevronRight />
        </button>
        <div className="config-submenu" role="menu">
          <button
            type="button"
            className={`config-submenu-item${theme === 'system' ? ' active' : ''}`}
            role="menuitemradio"
            aria-checked={theme === 'system'}
            onClick={() => onPickTheme('system')}
          >
            <span className="label">Hệ thống</span>
            <span className="check">✓</span>
          </button>
          <button
            type="button"
            className={`config-submenu-item${theme === 'light' ? ' active' : ''}`}
            role="menuitemradio"
            aria-checked={theme === 'light'}
            onClick={() => onPickTheme('light')}
          >
            <span className="label">Sáng</span>
            <span className="check">✓</span>
          </button>
          <button
            type="button"
            className={`config-submenu-item${theme === 'dark' ? ' active' : ''}`}
            role="menuitemradio"
            aria-checked={theme === 'dark'}
            onClick={() => onPickTheme('dark')}
          >
            <span className="label">Tối</span>
            <span className="check">✓</span>
          </button>
        </div>
      </div>
    </div>
  )
}
