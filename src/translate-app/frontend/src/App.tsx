import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useSessionStore } from '@/stores/session/sessionStore'
import { useSettingsStore } from '@/stores/settings/settingsStore'
import { useUIStore } from '@/stores/ui/uiStore'
import { useMessageStore } from '@/stores/message/messageStore'
import type { ThemeMode } from '@/types/settings'
import type { Message } from '@/types/session'
import { WailsService, WailsEvents } from '@/services/wailsService'
import { detectLang, titleFromContent } from '@/utils/languageDetect'
import { buildOptimisticUserMessage } from '@/utils/optimisticMessage'
import { groupSessionsByDay } from '@/utils/sessionGroups'
import { LONG_TEXT_THRESHOLD } from '@/utils/messageDisplay'
import { StartHello } from '@/components/StartHello'
import { StartOutgoingPreview } from '@/components/StartOutgoingPreview'
import { SessionRow } from '@/components/SessionRow'
import { ChatSessionHeader } from '@/components/ChatSessionHeader'
import { ChatMessage, type RetranslatePayload } from '@/components/ChatMessage'
import { ChatInputBar } from '@/components/ChatInputBar'
import { SettingsPopover } from '@/components/SettingsPopover'
import { ModelAIModal } from '@/components/ModelAIModal'
import '@/styles/global.css'
import '@/styles/typography.css'
import '@/styles/animations.css'
import '@/styles/shell.css'
import '@/styles/chat-mockup.css'
import '@/styles/mockup-override.css'

const IconMenu = () => (
  <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden>
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
  </svg>
)

const IconPlus = () => (
  <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden>
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
  </svg>
)

/* Cùng SVG mockup.v1.html (sidebar settings) */
const IconSettings = () => (
  <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden>
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth={2}
      d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
    />
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth={2}
      d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
    />
  </svg>
)

const EMPTY_MESSAGES: Message[] = []

export default function App() {
  const sessions = useSessionStore((s) => s.sessions)
  const loadSessions = useSessionStore((s) => s.loadSessions)
  const activeSessionId = useSessionStore((s) => s.activeSessionId)
  const setActiveSession = useSessionStore((s) => s.setActiveSession)

  const loadSettings = useSettingsStore((s) => s.loadSettings)
  const saveSettings = useSettingsStore((s) => s.saveSettings)
  const theme = useSettingsStore((s) => s.theme)
  const defaultStyle = useSettingsStore((s) => s.defaultStyle)
  const activeProvider = useSettingsStore((s) => s.activeProvider)
  const activeModel = useSettingsStore((s) => s.activeModel)

  const sidebarCollapsed = useUIStore((s) => s.sidebarCollapsed)
  const setSidebarCollapsed = useUIStore((s) => s.setSidebarCollapsed)
  const activeTargetLang = useUIStore((s) => s.activeTargetLang)

  const messagesFromStore = useMessageStore((s) =>
    activeSessionId ? s.messages[activeSessionId] : undefined,
  )
  const messages = messagesFromStore ?? EMPTY_MESSAGES
  const streamStatus = useMessageStore((s) => s.streamStatus)
  const loadMessages = useMessageStore((s) => s.loadMessages)
  const loadMoreMessages = useMessageStore((s) => s.loadMoreMessages)
  const setStreamStatus = useMessageStore((s) => s.setStreamStatus)
  const appendStreamingChunk = useMessageStore((s) => s.appendStreamingChunk)
  const clearStreamingText = useMessageStore((s) => s.clearStreamingText)
  const finalizeStream = useMessageStore((s) => s.finalizeStream)
  const appendMessage = useMessageStore((s) => s.appendMessage)
  const removeMessage = useMessageStore((s) => s.removeMessage)
  const hasMoreMap = useMessageStore((s) => s.hasMore)

  const [draft, setDraft] = useState('')
  const [sendError, setSendError] = useState<string | null>(null)
  const [startPending, setStartPending] = useState<{
    content: string
    displayMode: 'bubble' | 'bilingual'
  } | null>(null)
  const [sending, setSending] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [modelModalOpen, setModelModalOpen] = useState(false)
  const [streamingAssistantId, setStreamingAssistantId] = useState<string | null>(null)
  const [loadingMore, setLoadingMore] = useState(false)
  const [feedHeaderElevated, setFeedHeaderElevated] = useState(false)

  const feedRef = useRef<HTMLDivElement>(null)
  const sendLockRef = useRef(false)
  const settingsAnchorRef = useRef<HTMLDivElement>(null)

  const hasMore = activeSessionId ? !!hasMoreMap[activeSessionId] : false

  const scrollFeedToBottom = useCallback(() => {
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        const el = feedRef.current
        if (el) el.scrollTop = el.scrollHeight
      })
    })
  }, [])

  useEffect(() => {
    void loadSettings()
    void loadSessions()
  }, [loadSettings, loadSessions])

  useEffect(() => {
    const u1 = WailsEvents.onTranslationStart((p) => {
      setStreamingAssistantId(p.messageId)
      setStreamStatus('pending')
      clearStreamingText()
      setSendError(null)
    })
    const u2 = WailsEvents.onTranslationChunk((chunk) => {
      const st = useMessageStore.getState().streamStatus
      if (st === 'pending') setStreamStatus('streaming')
      appendStreamingChunk(chunk)
    })
    const u3 = WailsEvents.onTranslationDone((msg: Message) => {
      setStreamingAssistantId(null)
      void finalizeStream(msg.sessionId)
    })
    const u4 = WailsEvents.onTranslationError((err: string) => {
      setStreamingAssistantId(null)
      setStreamStatus('error')
      clearStreamingText()
      setSendError(err || 'Dịch thất bại')
    })
    return () => {
      u1()
      u2()
      u3()
      u4()
    }
  }, [appendStreamingChunk, clearStreamingText, finalizeStream, setStreamStatus])

  useEffect(() => {
    if (!activeSessionId) return
    let cancelled = false
    void loadMessages(activeSessionId).then(() => {
      if (!cancelled) scrollFeedToBottom()
    })
    return () => {
      cancelled = true
    }
  }, [activeSessionId, loadMessages, scrollFeedToBottom])

  useEffect(() => {
    if (!settingsOpen) return
    const onDown = (e: MouseEvent) => {
      if (settingsAnchorRef.current && !settingsAnchorRef.current.contains(e.target as Node)) {
        setSettingsOpen(false)
      }
    }
    const onEsc = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setSettingsOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    window.addEventListener('keydown', onEsc)
    return () => {
      document.removeEventListener('mousedown', onDown)
      window.removeEventListener('keydown', onEsc)
    }
  }, [settingsOpen])

  const busyStream = streamStatus === 'pending' || streamStatus === 'streaming'
  const inputBusy = busyStream || sending

  const handleSend = useCallback(async () => {
    const text = draft.trim()
    if (!text || busyStream || sendLockRef.current) return
    sendLockRef.current = true
    setSending(true)
    setSendError(null)
    const sourceLang = detectLang(text)
    const displayMode = text.length >= LONG_TEXT_THRESHOLD ? 'bilingual' : 'bubble'
    const style = defaultStyle

    try {
      if (!activeSessionId) {
        setStartPending({ content: text, displayMode })
        setDraft('')
        try {
          const res = await WailsService.createSessionAndSend({
            title: titleFromContent(text),
            content: text,
            displayMode,
            sourceLang,
            targetLang: activeTargetLang,
            style,
          })
          setStartPending(null)
          setActiveSession(res.sessionId)
          await loadSessions()
        } catch (e) {
          setStartPending(null)
          setDraft(text)
          setSendError(e instanceof Error ? e.message : String(e))
          setStreamStatus('idle')
        }
      } else {
        const sessionId = activeSessionId
        const list = useMessageStore.getState().messages[sessionId] ?? []
        const maxOrder = list.reduce((a, m) => Math.max(a, m.displayOrder), 0)
        const optimistic = buildOptimisticUserMessage({
          sessionId,
          content: text,
          displayMode,
          sourceLang,
          targetLang: activeTargetLang,
          style,
          nextDisplayOrder: maxOrder + 1,
        })
        appendMessage(sessionId, optimistic)
        setDraft('')
        try {
          await WailsService.sendMessage({
            sessionId,
            content: text,
            displayMode,
            sourceLang,
            targetLang: activeTargetLang,
            style,
          })
          await loadMessages(sessionId)
          scrollFeedToBottom()
        } catch (e) {
          removeMessage(sessionId, optimistic.id)
          setDraft(text)
          setSendError(e instanceof Error ? e.message : String(e))
          setStreamStatus('idle')
        }
      }
    } finally {
      sendLockRef.current = false
      setSending(false)
    }
  }, [
    draft,
    busyStream,
    activeSessionId,
    defaultStyle,
    activeTargetLang,
    setActiveSession,
    loadSessions,
    loadMessages,
    appendMessage,
    removeMessage,
    setStreamStatus,
    scrollFeedToBottom,
  ])

  const handleRetranslate = useCallback(
    async (p: RetranslatePayload) => {
      if (!activeSessionId || busyStream || sendLockRef.current) return
      sendLockRef.current = true
      setSending(true)
      setSendError(null)
      const sessionId = activeSessionId
      try {
        const list = useMessageStore.getState().messages[sessionId] ?? []
        const maxOrder = list.reduce((a, msg) => Math.max(a, msg.displayOrder), 0)
        const optimistic = buildOptimisticUserMessage({
          sessionId,
          content: p.sourceContent,
          displayMode: p.displayMode,
          sourceLang: p.sourceLang,
          targetLang: p.targetLang,
          style: p.style,
          nextDisplayOrder: maxOrder + 1,
          originalMessageId: p.assistantMessageId,
        })
        appendMessage(sessionId, optimistic)
        try {
          await WailsService.sendMessage({
            sessionId,
            content: p.sourceContent,
            displayMode: p.displayMode,
            sourceLang: p.sourceLang,
            targetLang: p.targetLang,
            style: p.style,
            originalMessageId: p.assistantMessageId,
            provider: activeProvider,
            model: activeModel,
          })
          await loadMessages(sessionId)
          scrollFeedToBottom()
        } catch (e) {
          removeMessage(sessionId, optimistic.id)
          setSendError(e instanceof Error ? e.message : String(e))
          setStreamStatus('idle')
        }
      } finally {
        sendLockRef.current = false
        setSending(false)
      }
    },
    [
      activeSessionId,
      busyStream,
      appendMessage,
      removeMessage,
      loadMessages,
      scrollFeedToBottom,
      setStreamStatus,
      activeProvider,
      activeModel,
    ],
  )

  const onFeedScroll = useCallback(() => {
    const el = feedRef.current
    if (el) setFeedHeaderElevated(el.scrollTop > 2)
    if (!el || !activeSessionId || loadingMore) return
    if (el.scrollTop > 120) return
    if (!useMessageStore.getState().hasMore[activeSessionId]) return

    const prevScrollHeight = el.scrollHeight
    const prevScrollTop = el.scrollTop

    setLoadingMore(true)
    void loadMoreMessages(activeSessionId)
      .then(() => {
        requestAnimationFrame(() => {
          requestAnimationFrame(() => {
            const feed = feedRef.current
            if (!feed) return
            feed.scrollTop = feed.scrollHeight - prevScrollHeight + prevScrollTop
          })
        })
      })
      .finally(() => setLoadingMore(false))
  }, [activeSessionId, loadingMore, loadMoreMessages])

  const drawerClass = `nav-drawer${sidebarCollapsed ? ' collapsed' : ''}`
  const activeSession = sessions.find((s) => s.id === activeSessionId)

  const pinnedSessions = useMemo(() => sessions.filter((s) => s.status === 'pinned'), [sessions])
  const listSessions = useMemo(() => sessions.filter((s) => s.status !== 'pinned'), [sessions])
  const dayGroups = useMemo(() => groupSessionsByDay(listSessions), [listSessions])

  const sidebarList =
    listSessions.length === 0
      ? null
      : dayGroups.map((g) => (
          <div key={g.key} className="sidebar-group today-group">
            <div className="sidebar-group-label">{g.label}</div>
            {g.sessions.map((sess) => (
              <SessionRow key={sess.id} sess={sess} active={sess.id === activeSessionId} />
            ))}
          </div>
        ))

  return (
    <div className="layout">
      <aside className={drawerClass}>
        <div className="drawer-header">
          <button
            type="button"
            className="nav-icon drawer-hamburger"
            onClick={() => setSidebarCollapsed(!sidebarCollapsed)}
            aria-label="Thu gọn sidebar"
          >
            <IconMenu />
          </button>
        </div>
        <button
          type="button"
          className="btn-new-session"
          aria-label="Phiên mới"
          onClick={() => {
            setActiveSession(null)
            setDraft('')
          }}
        >
          <IconPlus />
          <span className="sidebar-label">Bắt đầu phiên dịch mới</span>
        </button>
        <div className="sidebar-groups">
          {pinnedSessions.length > 0 && (
            <div className="sidebar-group pinned-group">
              <div className="sidebar-group-label">Ghim</div>
              {pinnedSessions.map((sess) => (
                <SessionRow key={sess.id} sess={sess} active={sess.id === activeSessionId} />
              ))}
            </div>
          )}
          {sidebarList}
        </div>
        <div className="sidebar-footer">
          <div className="settings-anchor" ref={settingsAnchorRef}>
            <button
              type="button"
              className="btn-sidebar-settings"
              onClick={() => setSettingsOpen((v) => !v)}
              aria-expanded={settingsOpen}
              aria-haspopup="menu"
              aria-label="Cài đặt"
            >
              <IconSettings />
              <span className="sidebar-label">Setting</span>
            </button>
            {settingsOpen && (
              <SettingsPopover
                theme={theme}
                onOpenModelAI={() => {
                  setSettingsOpen(false)
                  setModelModalOpen(true)
                }}
                onPickTheme={(nextTheme: ThemeMode) => {
                  setSettingsOpen(false)
                  void saveSettings({ theme: nextTheme })
                }}
              />
            )}
          </div>
        </div>
      </aside>

      <main className="main">
        {!activeSessionId ? (
          <div className="start-view">
            <div className="start-view-inner">
              <div className="start-hello-wrap">
                <StartHello />
              </div>
              <p className="start-subtext">
                Rất vui được hỗ trợ bạn. Bạn có thể gõ hoặc dán văn bản, đính kèm file
                <br />
                — bản dịch sẽ hiện ngay.
              </p>
              {sendError && <p className="shell-dev-hint" style={{ color: 'var(--accent)' }}>{sendError}</p>}
              {startPending && (
                <div className="start-outgoing-preview">
                  <StartOutgoingPreview content={startPending.content} displayMode={startPending.displayMode} />
                </div>
              )}
            </div>
            <ChatInputBar draft={draft} setDraft={setDraft} onSend={() => void handleSend()} busy={inputBusy} />
          </div>
        ) : (
          <div className="chat-view">
            {activeSession && (
              <ChatSessionHeader
                sessionId={activeSession.id}
                title={activeSession.title}
                elevated={feedHeaderElevated}
              />
            )}
            <div className="chat-feed" ref={feedRef} onScroll={onFeedScroll}>
              {loadingMore && hasMore && <div className="chat-load-more">Đang tải tin cũ…</div>}
              {sendError && <div className="chat-error-banner">{sendError}</div>}
              {messages.map((m, i) => {
                const prev = messages[i - 1]
                const retranslateFollowUp =
                  m.role === 'assistant' &&
                  prev?.role === 'user' &&
                  Boolean(prev.originalMessageId)
                const retranslateQuoteAssistant =
                  m.role === 'assistant' &&
                  prev?.role === 'user' &&
                  prev.originalMessageId
                    ? messages.find((x) => x.id === prev.originalMessageId && x.role === 'assistant')
                    : undefined
                return (
                  <ChatMessage
                    key={m.id}
                    m={m}
                    streamingAssistantId={streamingAssistantId}
                    nextAssistant={messages[i + 1]?.role === 'assistant' ? messages[i + 1] : undefined}
                    precedingUserContent={
                      m.role === 'assistant' && prev?.role === 'user' ? prev.originalContent : undefined
                    }
                    retranslateQuoteAssistant={retranslateQuoteAssistant}
                    retranslateFollowUp={retranslateFollowUp}
                    onRetranslate={handleRetranslate}
                  />
                )
              })}
            </div>
            <ChatInputBar draft={draft} setDraft={setDraft} onSend={() => void handleSend()} busy={inputBusy} />
          </div>
        )}
      </main>
      {modelModalOpen && <ModelAIModal onClose={() => setModelModalOpen(false)} />}
    </div>
  )
}
