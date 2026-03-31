import { useCallback, useEffect, useRef, useState } from 'react'
import { useSessionStore } from '@/stores/session/sessionStore'

export function ChatSessionHeader({
  sessionId,
  title,
  elevated = false,
}: {
  sessionId: string
  title: string
  /** Khi feed cuộn xuống (mockup: header tách khỏi nền) */
  elevated?: boolean
}) {
  const renameSession = useSessionStore((s) => s.renameSession)
  const loadSessions = useSessionStore((s) => s.loadSessions)
  const [editing, setEditing] = useState(false)
  const [value, setValue] = useState(title)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!editing) setValue(title)
  }, [title, editing])

  useEffect(() => {
    if (editing) inputRef.current?.focus()
  }, [editing])

  const commit = useCallback(async () => {
    const t = value.trim()
    setEditing(false)
    if (!t || t === title) return
    await renameSession(sessionId, t)
    await loadSessions()
  }, [value, title, sessionId, renameSession, loadSessions])

  return (
    <header className={`chat-session-header${elevated ? ' is-elevated' : ''}`}>
      <div className="chat-session-title-wrap">
        {editing ? (
          <input
            ref={inputRef}
            className="chat-session-title-input"
            value={value}
            aria-label="Tên phiên dịch"
            onChange={(e) => setValue(e.target.value)}
            onBlur={() => void commit()}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                void commit()
              }
              if (e.key === 'Escape') {
                setValue(title)
                setEditing(false)
              }
            }}
          />
        ) : (
          <button
            type="button"
            className="chat-session-title"
            aria-label="Đổi tên phiên"
            onClick={() => setEditing(true)}
          >
            {title || 'Phiên dịch'}
          </button>
        )}
      </div>
    </header>
  )
}
