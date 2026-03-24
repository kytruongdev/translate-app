import { useCallback, useEffect, useRef, useState, type DragEvent, type KeyboardEvent } from 'react'
import { CanResolveFilePaths, ResolveFilePaths } from '../../wailsjs/runtime/runtime'
import { useSettingsStore } from '@/stores/settings/settingsStore'
import { useUIStore } from '@/stores/ui/uiStore'
import { STYLE_OPTIONS, TARGET_LANG_OPTIONS } from '@/constants/inputOptions'
import type { TranslationStyle } from '@/types/session'
import type { PendingFilePick } from '@/types/ipc'
import { FileAttachment } from '@/components/FileAttachment'

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
  attachDisabled,
  pendingFile,
  filePickError,
  attachmentValidationError,
  onAttachClick,
  onClearPendingFile,
  onUserChoseFilePath,
  onNotifyPickError,
}: {
  draft: string
  setDraft: (v: string) => void
  onSend: () => void
  busy: boolean
  /** Mặc định: dùng `busy` — truyền riêng (vd. chỉ `sending`) để vẫn bấm đính kèm khi đang stream */
  attachDisabled?: boolean
  pendingFile: PendingFilePick | null
  filePickError: string | null
  attachmentValidationError: string | null
  onAttachClick: () => void
  onClearPendingFile: () => void
  onUserChoseFilePath: (path: string) => Promise<void>
  onNotifyPickError: (message: string) => void
}) {
  const defaultStyle = useSettingsStore((s) => s.defaultStyle)
  const saveSettings = useSettingsStore((s) => s.saveSettings)
  const activeTargetLang = useUIStore((s) => s.activeTargetLang)

  const [popover, setPopover] = useState<Popover>(null)
  const [fileDragOver, setFileDragOver] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)
  const textFieldRef = useRef<HTMLTextAreaElement>(null)
  const wrapRef = useRef<HTMLDivElement>(null)
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
  const langChip = langOpt?.label ?? activeTargetLang

  const blockAttach = attachDisabled ?? busy

  const canSendFile =
    Boolean(pendingFile) && !pendingFile?.loading && attachmentValidationError == null
  const canSubmit = Boolean(draft.trim() || canSendFile) && !busy

  /** Enter gửi kể cả khi focus không nằm trong textarea (vd. sau khi bấm đính kèm). */
  const trySubmitOnEnter = useCallback(
    (e: KeyboardEvent) => {
      const isEnter = e.key === 'Enter' || e.code === 'NumpadEnter'
      if (!isEnter || e.shiftKey) return
      if (e.nativeEvent.isComposing) return
      if (busy || !canSubmit) return
      const t = e.target as HTMLElement | null
      if (!t) return
      if (t.closest('.input-popover')) return
      if (t.closest('button.file-attachment-remove')) return
      if (t.closest('.btn-attach')) return
      if (t.closest('.input-chip')) return
      /* Để Enter trên nút gửi kích hoạt click mặc định — tránh double / chặn nhầm */
      if (t.closest('.btn-send-icon')) return
      e.preventDefault()
      e.stopPropagation()
      onSend()
    },
    [busy, canSubmit, onSend],
  )

  const onDragOver = useCallback((e: DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (!blockAttach) setFileDragOver(true)
  }, [blockAttach])

  const onDragLeave = useCallback((e: DragEvent) => {
    e.preventDefault()
    if (!wrapRef.current?.contains(e.relatedTarget as Node)) {
      setFileDragOver(false)
    }
  }, [])

  const onDrop = useCallback(
    async (e: DragEvent) => {
      e.preventDefault()
      e.stopPropagation()
      setFileDragOver(false)
      if (blockAttach) return
      const files = Array.from(e.dataTransfer.files)
      if (files.length === 0) return
      const f = files[0]
      let path: string | undefined
      if (CanResolveFilePaths()) {
        ResolveFilePaths(files)
        path = (f as File & { path?: string }).path
      }
      const nameLower = f.name.toLowerCase()
      const ref = (path ?? f.name).toLowerCase()
      if (!ref.endsWith('.pdf') && !ref.endsWith('.docx')) {
        onNotifyPickError('Chỉ hỗ trợ PDF và DOCX')
        return
      }
      if (!path) {
        if (nameLower.endsWith('.pdf') || nameLower.endsWith('.docx')) {
          onNotifyPickError('Không lấy được đường dẫn tệp — hãy dùng nút đính kèm hoặc bản build Wails')
        } else {
          onNotifyPickError('Chỉ hỗ trợ PDF và DOCX')
        }
        return
      }
      await onUserChoseFilePath(path)
    },
    [blockAttach, onNotifyPickError, onUserChoseFilePath],
  )

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
    <div
      className="chat-input-area"
      ref={rootRef}
      onKeyDownCapture={trySubmitOnEnter}
    >
      <div className="chat-input-row">
        <div
          ref={wrapRef}
          className={`input-wrap${fileDragOver ? ' is-file-dragover' : ''}`}
          onDragEnter={onDragOver}
          onDragOver={onDragOver}
          onDragLeave={onDragLeave}
          onDrop={(e: DragEvent) => void onDrop(e)}
        >
          {fileDragOver && (
            <div className="file-drop-overlay" aria-hidden>
              Thả file vào đây
            </div>
          )}
          {pendingFile && (
            <FileAttachment
              fileInfo={pendingFile.info}
              onRemove={onClearPendingFile}
              error={attachmentValidationError ?? undefined}
              loading={pendingFile.loading}
            />
          )}
          {filePickError && !pendingFile && (
            <p className="file-pick-error" role="alert">
              {filePickError}
            </p>
          )}
          <textarea
            ref={textFieldRef}
            className="text-field"
            rows={3}
            placeholder="Nhập hoặc dán văn bản, hoặc đính kèm file để dịch…"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onScroll={updateFooterScrollDivider}
            onKeyDown={(e) => {
              const isEnter = e.key === 'Enter' || e.code === 'NumpadEnter'
              if (!isEnter || e.shiftKey) return
              if (e.nativeEvent.isComposing) return
              if (!canSubmit) return
              e.preventDefault()
              onSend()
            }}
          />
          <div
            className="input-footer"
            data-scroll-divider={footerScrollDivider ? 'true' : 'false'}
          >
            <div className="input-controls" aria-label="Tuỳ chọn nhập">
              <button
                type="button"
                className="btn-attach"
                aria-label="Đính kèm file"
                disabled={blockAttach}
                onClick={() => onAttachClick()}
              >
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
                disabled={!canSubmit}
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
