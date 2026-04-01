import { useCallback, useEffect, useRef, useState, type DragEvent, type KeyboardEvent } from 'react'
import { Paperclip, ChevronDown, Send, AlertCircle } from 'lucide-react'
import { OnFileDrop, OnFileDropOff } from '../../wailsjs/runtime/runtime'
import { useSettingsStore } from '@/stores/settings/settingsStore'
import { useUIStore } from '@/stores/ui/uiStore'
import { STYLE_OPTIONS, TARGET_LANG_OPTIONS } from '@/constants/inputOptions'
import type { TranslationStyle } from '@/types/session'
import type { PendingFilePick } from '@/types/ipc'
import { FileAttachment } from '@/components/FileAttachment'


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
  onUserChoseFilePath,
  onNotifyPickError,
  onRemovePendingFile,
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
  onUserChoseFilePath: (path: string) => Promise<void>
  onNotifyPickError: (message: string) => void
  onRemovePendingFile: () => void
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

  const blockAttach = attachDisabled ?? busy

  // Wails native OnFileDrop — resolves absolute paths on macOS.
  // useDropTarget=false: fire for any drop on the window, we filter by wrapRef bounds.
  useEffect(() => {
    OnFileDrop((_x: number, _y: number, paths: string[]) => {
      setFileDragOver(false)
      if (blockAttach) return
      const path = paths[0]
      if (!path) return
      const lower = path.toLowerCase()
      if (!lower.endsWith('.docx') && !lower.endsWith('.pdf')) {
        onNotifyPickError('Chỉ hỗ trợ DOCX và PDF')
        return
      }
      void onUserChoseFilePath(path)
    }, false)
    return () => { OnFileDropOff() }
  }, [blockAttach, onNotifyPickError, onUserChoseFilePath])

  const styleLabel = STYLE_OPTIONS.find((o) => o.value === defaultStyle)?.label ?? 'Casual'
  const langOpt =
    TARGET_LANG_OPTIONS.find((o) => o.value === activeTargetLang) ?? TARGET_LANG_OPTIONS[0]
  const langChip = langOpt?.label ?? activeTargetLang

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
          onDrop={(e: DragEvent) => { e.preventDefault(); e.stopPropagation(); setFileDragOver(false) }}
        >
          {fileDragOver && (
            <div className="file-drop-overlay" aria-hidden>
              Thả file vào đây
            </div>
          )}
          {pendingFile && (
            <FileAttachment
              fileInfo={pendingFile.info}
              loading={pendingFile.loading}
              error={attachmentValidationError ?? filePickError}
              onRemove={onRemovePendingFile}
            />
          )}
          {filePickError && !pendingFile && (
            <p className="file-pick-error" role="alert">
              <AlertCircle size={13} />
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
                <Paperclip size={18} aria-hidden />
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
                  <ChevronDown size={14} aria-hidden />
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
                  <ChevronDown size={14} aria-hidden />
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
                <Send size={18} aria-hidden />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
