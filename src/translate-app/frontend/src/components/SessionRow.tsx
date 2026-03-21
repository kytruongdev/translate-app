import { useEffect, useRef, useState } from 'react'
import type { Session } from '@/types/session'
import { useSessionStore } from '@/stores/session/sessionStore'
import { useUIStore } from '@/stores/ui/uiStore'

function formatSessionTime(iso: string): string {
  try {
    const d = new Date(iso)
    if (Number.isNaN(d.getTime())) return ''
    return d.toLocaleString(undefined, { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })
  } catch {
    return ''
  }
}

const IconDots = () => (
  <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden>
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth={2}
      d="M12 5v.01M12 12v.01M12 19v.01M12 6a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2z"
    />
  </svg>
)

export function SessionRow({ sess, active }: { sess: Session; active: boolean }) {
  const setActiveSession = useSessionStore((s) => s.setActiveSession)
  const renameSession = useSessionStore((s) => s.renameSession)
  const updateStatus = useSessionStore((s) => s.updateStatus)
  const loadSessions = useSessionStore((s) => s.loadSessions)
  const menuOpenId = useUIStore((s) => s.sessionMenuOpenId)
  const setSessionMenuOpenId = useUIStore((s) => s.setSessionMenuOpenId)
  const inlineRenameId = useUIStore((s) => s.sessionInlineRenameId)
  const setSessionInlineRenameId = useUIStore((s) => s.setSessionInlineRenameId)
  const wrapRef = useRef<HTMLDivElement>(null)
  const titleInputRef = useRef<HTMLInputElement>(null)
  const [draftTitle, setDraftTitle] = useState(sess.title)

  const open = menuOpenId === sess.id
  const isRenaming = inlineRenameId === sess.id

  useEffect(() => {
    if (!open) return
    const close = (e: MouseEvent) => {
      if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) {
        setSessionMenuOpenId(null)
      }
    }
    document.addEventListener('mousedown', close)
    return () => document.removeEventListener('mousedown', close)
  }, [open, setSessionMenuOpenId])

  useEffect(() => {
    if (!isRenaming) return
    setDraftTitle(sess.title)
    const id = requestAnimationFrame(() => {
      const el = titleInputRef.current
      el?.focus()
      el?.select()
    })
    return () => cancelAnimationFrame(id)
  }, [isRenaming, sess.id, sess.title])

  async function onPinToggle() {
    setSessionInlineRenameId(null)
    const next = sess.status === 'pinned' ? 'active' : 'pinned'
    await updateStatus(sess.id, next)
    setSessionMenuOpenId(null)
    await loadSessions()
  }

  function onRename() {
    setSessionMenuOpenId(null)
    setSessionInlineRenameId(sess.id)
  }

  async function commitInlineRename() {
    const t = draftTitle.trim()
    if (!t) {
      setDraftTitle(sess.title)
      setSessionInlineRenameId(null)
      return
    }
    if (t !== (sess.title || '').trim()) {
      await renameSession(sess.id, t)
      await loadSessions()
    }
    setSessionInlineRenameId(null)
  }

  function cancelInlineRename() {
    setDraftTitle(sess.title)
    setSessionInlineRenameId(null)
  }

  async function onArchive() {
    setSessionInlineRenameId(null)
    if (!window.confirm('Lưu phiên vào mục đã lưu trữ? (ẩn khỏi danh sách chính)')) {
      setSessionMenuOpenId(null)
      return
    }
    await updateStatus(sess.id, 'archived')
    setSessionMenuOpenId(null)
    if (active) setActiveSession(null)
    await loadSessions()
  }

  return (
    <div
      ref={wrapRef}
      className={`session-item${sess.status === 'pinned' ? ' pinned' : ''}${active ? ' active' : ''}${isRenaming ? ' is-renaming' : ''}`}
    >
      {isRenaming ? (
        <div className="session-item-main session-item-main--rename">
          <div className="session-item-content">
            <input
              ref={titleInputRef}
              type="text"
              className="session-item-title-input"
              value={draftTitle}
              aria-label="Tên phiên dịch"
              onChange={(e) => setDraftTitle(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault()
                  void commitInlineRename()
                }
                if (e.key === 'Escape') {
                  e.preventDefault()
                  cancelInlineRename()
                }
              }}
              onClick={(e) => e.stopPropagation()}
            />
            <div className="session-item-meta">{formatSessionTime(sess.updatedAt)}</div>
          </div>
        </div>
      ) : (
        <button
          type="button"
          className="session-item-main"
          onClick={() => setActiveSession(sess.id)}
        >
          <div className="session-item-content">
            <div className="session-item-title">{sess.title || 'Không tiêu đề'}</div>
            <div className="session-item-meta">{formatSessionTime(sess.updatedAt)}</div>
          </div>
        </button>
      )}
      <div className="session-item-menu-wrap">
        <button
          type="button"
          className={`session-item-menu${open ? ' is-open' : ''}`}
          aria-label="Tùy chọn phiên"
          aria-expanded={open}
          onClick={(e) => {
            e.stopPropagation()
            if (isRenaming) setSessionInlineRenameId(null)
            setSessionMenuOpenId(open ? null : sess.id)
          }}
        >
          <IconDots />
        </button>
        <div className={`dropdown-menu session-dropdown${open ? ' open' : ''}`} role="menu">
          <button type="button" role="menuitem" onClick={() => void onPinToggle()}>
            {sess.status === 'pinned' ? 'Bỏ ghim' : 'Ghim'}
          </button>
          <button type="button" role="menuitem" onClick={() => void onRename()}>
            Đổi tên
          </button>
          <button type="button" role="menuitem" onClick={() => void onArchive()}>
            Lưu trữ
          </button>
        </div>
      </div>
    </div>
  )
}
