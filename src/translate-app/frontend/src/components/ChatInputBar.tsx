import { useCallback, useEffect, useRef, useState } from 'react'
import { useSettingsStore } from '@/stores/settings/settingsStore'
import { useUIStore } from '@/stores/ui/uiStore'
import { STYLE_OPTIONS, TARGET_LANG_OPTIONS } from '@/constants/inputOptions'
import type { TranslationStyle } from '@/types/session'

const IconAttach = () => (
  <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden>
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth={2}
      d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"
    />
  </svg>
)

const IconChevronDown = () => (
  <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden>
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
  </svg>
)

const IconSend = () => (
  <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden>
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
  </svg>
)

type Popover = null | 'style' | 'lang'

export function ChatInputBar({
  draft,
  setDraft,
  onSend,
  busy,
}: {
  draft: string
  setDraft: (v: string) => void
  onSend: () => void
  busy: boolean
}) {
  const defaultStyle = useSettingsStore((s) => s.defaultStyle)
  const saveSettings = useSettingsStore((s) => s.saveSettings)
  const activeTargetLang = useUIStore((s) => s.activeTargetLang)

  const [popover, setPopover] = useState<Popover>(null)
  const rootRef = useRef<HTMLDivElement>(null)
  const textFieldRef = useRef<HTMLTextAreaElement>(null)
  /** Line + shadow chỉ khi cuộn giữa chừng — ẩn ở đầu (scrollTop≈0) và ở cuối (thumb chạm đáy). */
  const [footerScrollDivider, setFooterScrollDivider] = useState(false)

  const updateFooterScrollDivider = useCallback(() => {
    const el = textFieldRef.current
    if (!el) return
    const { scrollTop, scrollHeight, clientHeight } = el
    const eps = 4
    const atTop = scrollTop <= eps
    const atBottom = scrollTop + clientHeight >= scrollHeight - eps
    setFooterScrollDivider(!atTop && !atBottom)
  }, [])

  useEffect(() => {
    updateFooterScrollDivider()
  }, [draft, updateFooterScrollDivider])

  useEffect(() => {
    const el = textFieldRef.current
    if (!el || typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(() => updateFooterScrollDivider())
    ro.observe(el)
    return () => ro.disconnect()
  }, [updateFooterScrollDivider])

  useEffect(() => {
    if (!popover) return
    const fn = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setPopover(null)
    }
    document.addEventListener('mousedown', fn)
    return () => document.removeEventListener('mousedown', fn)
  }, [popover])

  const styleLabel = STYLE_OPTIONS.find((o) => o.value === defaultStyle)?.label ?? 'Casual'
  const langOpt =
    TARGET_LANG_OPTIONS.find((o) => o.value === activeTargetLang) ?? TARGET_LANG_OPTIONS[0]
  const langChip = `Dịch · ${langOpt?.chip ?? activeTargetLang}`

  const pickStyle = useCallback(
    async (v: TranslationStyle) => {
      await saveSettings({ defaultStyle: v })
      setPopover(null)
    },
    [saveSettings],
  )

  const pickLang = useCallback(
    async (value: string) => {
      await saveSettings({ lastTargetLang: value })
      setPopover(null)
    },
    [saveSettings],
  )

  return (
    <div className="chat-input-area" ref={rootRef}>
      <div className="chat-input-row">
        <div className="input-wrap">
          <textarea
            ref={textFieldRef}
            className="text-field"
            rows={3}
            placeholder="Nhập hoặc dán văn bản, hoặc đính kèm file để dịch…"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onScroll={updateFooterScrollDivider}
            onKeyDown={(e) => {
              // Mockup: Enter gửi, Shift+Enter xuống hàng (mockup.v1.html ~3307–3308)
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault()
                if (!busy && draft.trim()) onSend()
              }
            }}
          />
          <div
            className="input-footer"
            data-scroll-divider={footerScrollDivider ? 'true' : 'false'}
          >
            <div className="input-controls" aria-label="Tuỳ chọn nhập">
              <button type="button" className="btn-attach" aria-label="Đính kèm file">
                <IconAttach />
              </button>
              <div className="input-chip-wrap">
                <button
                  type="button"
                  className={`input-chip${popover === 'style' ? ' is-open' : ''}`}
                  aria-expanded={popover === 'style'}
                  aria-haspopup="listbox"
                  onClick={() => setPopover((p) => (p === 'style' ? null : 'style'))}
                >
                  <span>{styleLabel}</span>
                  <IconChevronDown />
                </button>
                {popover === 'style' && (
                  <div className="input-popover" role="listbox">
                    {STYLE_OPTIONS.map((o) => (
                      <button
                        key={o.value}
                        type="button"
                        role="option"
                        className={o.value === defaultStyle ? 'is-active' : ''}
                        onClick={() => void pickStyle(o.value)}
                      >
                        {o.label}
                      </button>
                    ))}
                  </div>
                )}
              </div>
              <div className="input-chip-wrap">
                <button
                  type="button"
                  className={`input-chip${popover === 'lang' ? ' is-open' : ''}`}
                  aria-expanded={popover === 'lang'}
                  aria-haspopup="listbox"
                  onClick={() => setPopover((p) => (p === 'lang' ? null : 'lang'))}
                >
                  <span>{langChip}</span>
                  <IconChevronDown />
                </button>
                {popover === 'lang' && (
                  <div className="input-popover input-popover-wide" role="listbox">
                    {TARGET_LANG_OPTIONS.map((o) => (
                      <button
                        key={o.value}
                        type="button"
                        role="option"
                        className={o.value === activeTargetLang ? 'is-active' : ''}
                        onClick={() => void pickLang(o.value)}
                      >
                        {o.label}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            </div>
            <div className="input-actions">
              <button
                type="button"
                className="btn-send-icon"
                disabled={!draft.trim() || busy}
                aria-label="Gửi"
                onClick={() => onSend()}
              >
                <IconSend />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
