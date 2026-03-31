import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
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
import { HEAVY_FILE_INLINE_CHAR_THRESHOLD, LONG_TEXT_THRESHOLD } from '@/utils/messageDisplay'
import {
  enqueueStreamingTextChunk,
  flushStreamingTextCoalescerSync,
  resetStreamingTextCoalescer,
  setStreamingFileJobActive,
} from '@/utils/streamingTextCoalescer'
import { StartHello } from '@/components/StartHello'
import { StartOutgoingPreview } from '@/components/StartOutgoingPreview'
import { SessionRow } from '@/components/SessionRow'
import { ChatSessionHeader } from '@/components/ChatSessionHeader'
import { ChatMessageVirtualList, formatStickyDate } from '@/components/ChatMessageVirtualList'
import type { RetranslatePayload } from '@/components/ChatMessage'
import { ChatSessionLoadingPreview } from '@/components/ChatSessionLoadingPreview'
import { ChatInputBar } from '@/components/ChatInputBar'
import type { FileInfo, PendingFilePick, SearchResult } from '@/types/ipc'
import { MAX_FILE_PAGE_COUNT, type FileProgress } from '@/types/file'
import { titleFromFileName } from '@/utils/fileTitle'
import { SettingsPopover } from '@/components/SettingsPopover'
import { ModelAIModal } from '@/components/ModelAIModal'
import '@/styles/global.css'
import '@/styles/typography.css'
import '@/styles/animations.css'
import '@/styles/shell.css'
import '@/styles/chat-mockup.css'
import '@/styles/mockup-override.css'

function highlightKeyword(text: string, keyword: string): React.ReactNode {
  if (!keyword.trim()) return text
  const regex = new RegExp(`(${keyword.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi')
  const parts = text.split(regex)
  return parts.map((part, i) =>
    regex.test(part) ? <mark key={i} className="search-keyword-mark">{part}</mark> : part
  )
}

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

const IconSearch = () => (
  <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden>
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-4.35-4.35m0 0A7.5 7.5 0 104.5 4.5a7.5 7.5 0 0012.15 12.15z" />
  </svg>
)

const IconArrowLeft = () => (
  <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden>
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
  </svg>
)

const EMPTY_MESSAGES: Message[] = []

export default function App() {
  const sessions = useSessionStore((s) => s.sessions)
  const loadSessions = useSessionStore((s) => s.loadSessions)
  const activeSessionId = useSessionStore((s) => s.activeSessionId)
  const sessionTransitionPending = useSessionStore((s) => s.sessionTransitionPending)
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
  const clearStreamingText = useMessageStore((s) => s.clearStreamingText)
  const finalizeStream = useMessageStore((s) => s.finalizeStream)
  const appendMessage = useMessageStore((s) => s.appendMessage)
  const removeMessage = useMessageStore((s) => s.removeMessage)
  const hasMoreMap = useMessageStore((s) => s.hasMore)
  /** Zustand: true sau khi loadMessages (hoặc append) cho phiên — skeleton không phụ thuộc useEffect / paint. */
  const sessionFeedReady = useMessageStore((s) =>
    activeSessionId ? s.sessionFeedReady[activeSessionId] === true : true,
  )

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
  const [chatStickyDate, setChatStickyDate] = useState<string | null>(null)
  const [chatStickyVisible, setChatStickyVisible] = useState(false)

  const handleScrollDate = useCallback((date: string | null, visible: boolean) => {
    if (date) setChatStickyDate(date)
    setChatStickyVisible(visible)
  }, [])
  const [pendingFile, setPendingFile] = useState<PendingFilePick | null>(null)
  const [filePickError, setFilePickError] = useState<string | null>(null)
  const [fileTranslateProgress, setFileTranslateProgress] = useState<FileProgress | null>(null)
  const addCancelledFileId = useUIStore((s) => s.addCancelledFileId)

  const [searchOpen, setSearchOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState<SearchResult[]>([])
  const [searchLoading, setSearchLoading] = useState(false)
  const [scrollToMessageId, setScrollToMessageId] = useState<string | null>(null)
  const scrollToMessageIdRef = useRef<string | null>(null)
  const [highlightMessageId, setHighlightMessageId] = useState<string | null>(null)
  const searchInputRef = useRef<HTMLInputElement>(null)
  const searchDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const feedRef = useRef<HTMLDivElement>(null)
  const sendLockRef = useRef(false)
  const settingsAnchorRef = useRef<HTMLDivElement>(null)
  const pendingFileRef = useRef<PendingFilePick | null>(null)
  const attachmentValidationRef = useRef<string | null>(null)
  const autoSendOnReadyRef = useRef(false)

  const hasMore = activeSessionId ? !!hasMoreMap[activeSessionId] : false

  /** O(1) lookup thay cho messages.find mỗi hàng — tránh O(n²) khi feed dài sau load-more */
  const assistantById = useMemo(() => {
    const map = new Map<string, Message>()
    for (const msg of messages) {
      if (msg.role === 'assistant') map.set(msg.id, msg)
    }
    return map
  }, [messages])

  const scrollFeedToBottom = useCallback(() => {
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        const el = feedRef.current
        if (el) el.scrollTop = el.scrollHeight
      })
    })
  }, [])

  const handleSearchQueryChange = useCallback((q: string) => {
    setSearchQuery(q)
    if (searchDebounceRef.current) clearTimeout(searchDebounceRef.current)
    if (!q.trim()) { setSearchResults([]); setSearchLoading(false); return }
    setSearchLoading(true)
    searchDebounceRef.current = setTimeout(async () => {
      try {
        const results = await WailsService.searchMessages(q.trim())
        setSearchResults(results)
      } catch {
        setSearchResults([])
      } finally {
        setSearchLoading(false)
      }
    }, 300)
  }, [])

  const handleJumpToMessage = useCallback((result: SearchResult) => {
    let targetId = result.messageId
    // Remap user → assistant khi user message không hiển thị bubble riêng:
    // - Bilingual/file: source text nằm trong assistant card
    // - Retranslate: user message render dạng invisible placeholder
    if (result.role === 'user') {
      const msgs = useMessageStore.getState().messages[result.sessionId] ?? []
      const idx = msgs.findIndex((m) => m.id === result.messageId)
      const userMsg = idx !== -1 ? msgs[idx] : undefined
      const next = idx !== -1 ? msgs[idx + 1] : undefined
      const isRetranslate = Boolean(userMsg?.originalMessageId)
      if (next?.role === 'assistant' && (next.displayMode !== 'bubble' || isRetranslate)) {
        targetId = next.id
      }
    }
    scrollToMessageIdRef.current = targetId
    setScrollToMessageId(targetId)
    setActiveSession(result.sessionId)
  }, [setActiveSession])

  const handleScrollToMessageDone = useCallback(() => {
    const id = scrollToMessageId
    scrollToMessageIdRef.current = null
    setScrollToMessageId(null)
    setHighlightMessageId(id)
    setTimeout(() => setHighlightMessageId(null), 2500)
  }, [scrollToMessageId])

  // If scrollToMessageId target not yet loaded, load more pages until found.
  useEffect(() => {
    if (!scrollToMessageId || !activeSessionId) return
    const found = messages.some((m) => m.id === scrollToMessageId)
    if (found) return
    // undefined = loadMessages hasn't completed yet for this session — wait for it
    if (hasMoreMap[activeSessionId] === undefined) return
    if (hasMoreMap[activeSessionId]) {
      void loadMoreMessages(activeSessionId)
    } else {
      setScrollToMessageId(null)
    }
  }, [scrollToMessageId, messages, activeSessionId, hasMoreMap, loadMoreMessages])

  useEffect(() => {
    void loadSettings()
    void loadSessions()
  }, [loadSettings, loadSessions])

  useEffect(() => {
    const u1 = WailsEvents.onTranslationStart((p) => {
      resetStreamingTextCoalescer()
      setStreamingFileJobActive(false)
      setStreamingAssistantId(p.messageId)
      setStreamStatus('pending')
      clearStreamingText()
      setSendError(null)
    })
    const u2 = WailsEvents.onTranslationChunk((chunk) => {
      const st = useMessageStore.getState().streamStatus
      if (st === 'pending') setStreamStatus('streaming')
      enqueueStreamingTextChunk(chunk)
    })
    const u3 = WailsEvents.onTranslationDone((msg: Message) => {
      flushStreamingTextCoalescerSync()
      setStreamingAssistantId(null)
      setFileTranslateProgress(null)
      setStreamingFileJobActive(false)
      void finalizeStream(msg.sessionId)
    })
    const u4 = WailsEvents.onTranslationError((err: string) => {
      flushStreamingTextCoalescerSync()
      setStreamingAssistantId(null)
      setStreamStatus('error')
      clearStreamingText()
      setFileTranslateProgress(null)
      setStreamingFileJobActive(false)
      setSendError(err || 'Dịch thất bại')
    })
    const uf1 = WailsEvents.onFileProgress((p) => {
      setStreamingFileJobActive(true)
      setFileTranslateProgress(p)
    })
    const uf2 = WailsEvents.onFileError((err) => {
      setFileTranslateProgress(null)
      setStreamingFileJobActive(false)
      setSendError(err || 'Dịch file thất bại')
    })
    const uf3 = WailsEvents.onFileDone(() => {
      setFileTranslateProgress(null)
      setStreamingFileJobActive(false)
    })
    const uf4 = WailsEvents.onFileCancelled((p) => {
      flushStreamingTextCoalescerSync()
      setStreamingAssistantId(null)
      setStreamStatus('idle')
      clearStreamingText()
      setFileTranslateProgress(null)
      setStreamingFileJobActive(false)
      addCancelledFileId(p.fileId)
    })
    const uf0 = WailsEvents.onFileSource((p) => {
      const sid = p.sessionId?.trim()
      if (sid) void loadMessages(sid)
      const md = typeof p.markdown === 'string' ? p.markdown : ''
      const aid = typeof p.assistantMessageId === 'string' ? p.assistantMessageId.trim() : ''
      const active = useSessionStore.getState().activeSessionId
      if (md.length >= HEAVY_FILE_INLINE_CHAR_THRESHOLD && aid && sid === active) {
        useUIStore.getState().setPendingTranslationFullscreenMessageId(aid)
      }
    })
    return () => {
      u1()
      u2()
      u3()
      u4()
      uf0()
      uf1()
      uf2()
      uf3()
      uf4()
      resetStreamingTextCoalescer()
      setStreamingFileJobActive(false)
    }
  }, [clearStreamingText, finalizeStream, loadMessages, setStreamStatus])

  useEffect(() => {
    if (!activeSessionId) return
    const sid = activeSessionId
    let cancelled = false
    void loadMessages(sid).then(() => {
      if (cancelled) return
      if (useSessionStore.getState().activeSessionId !== sid) return
      if (scrollToMessageIdRef.current) return // search jump will handle scroll
      scrollFeedToBottom()
    })

    return () => {
      cancelled = true
    }
  }, [activeSessionId, loadMessages, scrollFeedToBottom])

  /**
   * Sau 2 frame + thêm một nhịp ngắn: vẫn tránh mount list dài cùng lúc unmount,
   * nhưng đủ lâu để skeleton (vệt trượt ~0.88s) kịp nhìn thấy chuyển động.
   */
  useEffect(() => {
    if (!sessionTransitionPending) return
    let cancelled = false
    let raf1 = 0
    let raf2 = 0
    let hold: ReturnType<typeof setTimeout> | undefined
    raf1 = requestAnimationFrame(() => {
      raf2 = requestAnimationFrame(() => {
        if (cancelled) return
        hold = window.setTimeout(() => {
          if (cancelled) return
          useSessionStore.getState().clearSessionTransition()
          if (!scrollToMessageIdRef.current) scrollFeedToBottom()
        }, 110)
      })
    })
    return () => {
      cancelled = true
      cancelAnimationFrame(raf1)
      cancelAnimationFrame(raf2)
      if (hold !== undefined) window.clearTimeout(hold)
    }
  }, [activeSessionId, sessionTransitionPending, scrollFeedToBottom])

  const warmTransitionShell = Boolean(activeSessionId && sessionTransitionPending)
  const showSessionFeedPlaceholder = Boolean(
    warmTransitionShell || (activeSessionId && !sessionFeedReady && messages.length === 0),
  )

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
  /** Chỉ khóa ô nhập khi đang trong một phiên — tránh streamStatus kẹt sau phiên cũ chặn gửi ở màn start. */
  const streamBlocksInput = Boolean(activeSessionId && busyStream)
  const inputBusy = sending || streamBlocksInput

  const attachmentValidationError = useMemo(() => {
    if (!pendingFile) return null
    if (pendingFile.loading) return null
    if (pendingFile.info.isScanned === true) {
      return 'Ứng dụng chưa hỗ trợ dịch thuật từ văn bản scan'
    }
    const raw = pendingFile.info.pageCount
    const pc = typeof raw === 'number' ? raw : raw != null ? Number(raw) : NaN
    if (Number.isFinite(pc) && pc > MAX_FILE_PAGE_COUNT) {
      return `Tệp quá lớn (tối đa ${MAX_FILE_PAGE_COUNT} trang)`
    }
    return null
  }, [pendingFile])

  useEffect(() => {
    pendingFileRef.current = pendingFile
    attachmentValidationRef.current = attachmentValidationError
  }, [pendingFile, attachmentValidationError])

  const ingestFilePath = useCallback(async (path: string) => {
    const lower = path.toLowerCase()
    if (!lower.endsWith('.docx') && !lower.endsWith('.pdf')) {
      setFilePickError('Chỉ hỗ trợ DOCX và PDF')
      setPendingFile(null)
      return
    }
    const name = path.replace(/^.*[/\\]/, '') || path
    const type: FileInfo['type'] = lower.endsWith('.pdf') ? 'pdf' : 'docx'
    const placeholder: FileInfo = {
      name,
      type,
      fileSize: 0,
      charCount: 1,
      estimatedChunks: 1,
      estimatedMinutes: 1,
    }
    autoSendOnReadyRef.current = true
    setFilePickError(null)
    setPendingFile({ path, info: placeholder, loading: true })
    try {
      const info = await WailsService.readFileInfo(path)
      setPendingFile({ path, info, loading: false })
    } catch (err) {
      autoSendOnReadyRef.current = false
      setPendingFile((prev) => prev ? { ...prev, loading: false } : null)
      setFilePickError(err instanceof Error ? err.message : String(err))
    }
  }, [])

  /**
   * Mở dialog ngay trong user gesture (không queueMicrotask) — macOS/Wails hay nhạy timing IPC.
   * Chỉ chặn khi đang `sending`.
   */
  const handleOpenFilePicker = useCallback(() => {
    if (sending) return
    void (async () => {
      try {
        const path = await WailsService.openFileDialog()
        if (!path) return
        await ingestFilePath(path)
      } catch (e) {
        autoSendOnReadyRef.current = false
        setFilePickError(e instanceof Error ? e.message : String(e))
      }
    })()
  }, [sending, ingestFilePath])

  const onNotifyPickError = useCallback((message: string) => {
    setFilePickError(message)
    setPendingFile(null)
  }, [])

  const handleSend = useCallback(async () => {
    if (sendLockRef.current) return

    const text = draft.trim()
    const snap = pendingFileRef.current
    const attachErr = attachmentValidationRef.current
    const sendFile = snap != null && !snap.loading && attachErr == null

    if (!text && !sendFile) return

    if (activeSessionId && busyStream) return

    if (sendFile && snap) {
      sendLockRef.current = true
      setSending(true)
      setSendError(null)
      setFilePickError(null)
      setFileTranslateProgress(null)
      try {
        let sessionId = activeSessionId
        if (!sessionId) {
          const t = titleFromFileName(snap.info.name)
          sessionId = await WailsService.createEmptySession(t, activeTargetLang, defaultStyle)
          setActiveSession(sessionId)
          await loadSessions()
        }
        await WailsService.translateFile({
          sessionId,
          filePath: snap.path,
          targetLang: activeTargetLang,
          style: defaultStyle,
          provider: activeProvider,
          model: activeModel,
        })
        setPendingFile(null)
        setDraft('')
        await loadMessages(sessionId)
        void loadSessions()
        scrollFeedToBottom()
      } catch (e) {
        setSendError(e instanceof Error ? e.message : String(e))
      } finally {
        sendLockRef.current = false
        setSending(false)
      }
      return
    }

    if (!text) return
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
          void loadSessions()
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
    activeProvider,
    activeModel,
    activeTargetLang,
  ])

  // "Latest ref" pattern — always points to current handleSend without adding it to effect deps
  const handleSendRef = useRef(handleSend)
  handleSendRef.current = handleSend

  // Auto-send after file picker: user picks file → translation starts immediately
  useEffect(() => {
    if (!autoSendOnReadyRef.current) return
    if (!pendingFile) {
      autoSendOnReadyRef.current = false
      return
    }
    if (pendingFile.loading) return
    if (attachmentValidationError) {
      autoSendOnReadyRef.current = false
      return
    }
    autoSendOnReadyRef.current = false
    void handleSendRef.current()
  }, [pendingFile, attachmentValidationError])

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
          content: p.fileDisplayContent ?? p.sourceContent,
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
            fileId: p.fileId,
            fileDisplayContent: p.fileDisplayContent,
          })
          await loadMessages(sessionId)
          void loadSessions()
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
    if (el) {
      const headerElev = el.scrollTop > 2
      setFeedHeaderElevated((prev) => (prev === headerElev ? prev : headerElev))
    }
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

  const [sidebarAnimReady, setSidebarAnimReady] = useState(false)
  useEffect(() => {
    const t = window.setTimeout(() => setSidebarAnimReady(true), 400)
    return () => window.clearTimeout(t)
  }, [])

  const sidebarList =
    listSessions.length === 0
      ? null
      : dayGroups.map((g) => (
          <div key={g.key} className="sidebar-group today-group">
            <div className="sidebar-group-label">{g.label}</div>
            {g.sessions.map((sess) => (
              <SessionRow key={`${g.key}-${sess.id}`} sess={sess} active={sess.id === activeSessionId} />
            ))}
          </div>
        ))

  return (
    <div className="layout">
      <aside className={drawerClass}>
        {searchOpen && (
          <div className="search-panel">
            <div className="search-panel-header">
              <button
                type="button"
                className="search-panel-back"
                onClick={() => { setSearchOpen(false); setSearchQuery(''); setSearchResults([]) }}
                aria-label="Đóng tìm kiếm"
              >
                <IconArrowLeft />
              </button>
              <input
                ref={searchInputRef}
                type="text"
                className="search-panel-input"
                placeholder="Tìm kiếm trong tất cả phiên…"
                value={searchQuery}
                onChange={(e) => handleSearchQueryChange(e.target.value)}
                autoComplete="off"
                autoCorrect="off"
                spellCheck={false}
                autoFocus
              />
            </div>
            <div className="search-panel-body">
              {searchLoading && (
                <div className="search-panel-state">Đang tìm kiếm…</div>
              )}
              {!searchLoading && searchQuery.trim() && searchResults.length === 0 && (
                <div className="search-panel-state">Không tìm thấy kết quả</div>
              )}
              {!searchLoading && searchResults.map((r) => (
                <button
                  key={r.messageId}
                  type="button"
                  className="search-result-item"
                  onClick={() => handleJumpToMessage(r)}
                >
                  <div className="search-result-session">{r.sessionTitle}</div>
                  <div className="search-result-snippet">{highlightKeyword(r.snippet, searchQuery)}</div>
                  <div className="search-result-meta">
                    {r.role === 'assistant' ? 'Bản dịch' : 'Văn bản gốc'}
                    {' · '}
                    {new Date(r.createdAt).toLocaleDateString('vi-VN')}
                  </div>
                </button>
              ))}
            </div>
          </div>
        )}
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
            setPendingFile(null)
            setFilePickError(null)
            setFileTranslateProgress(null)
            setStreamingAssistantId(null)
            setStreamStatus('idle')
            clearStreamingText()
          }}
        >
          <IconPlus />
          <span className="sidebar-label">Bắt đầu phiên dịch mới</span>
        </button>
        <div className={`sidebar-groups${sidebarAnimReady ? ' sidebar-anim-ready' : ''}`}>
          {pinnedSessions.length > 0 && (
            <div className="sidebar-group pinned-group">
              <div className="sidebar-group-label">Ghim</div>
              {pinnedSessions.map((sess) => (
                <SessionRow key={`pinned-${sess.id}`} sess={sess} active={sess.id === activeSessionId} />
              ))}
            </div>
          )}
          {sidebarList}
        </div>
        <div className="sidebar-footer">
          <button
            type="button"
            className="btn-sidebar-search"
            onClick={() => setSearchOpen(true)}
            aria-label="Tìm kiếm"
          >
            <IconSearch />
            <span className="sidebar-label">Tìm kiếm</span>
          </button>
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
              {sendError && (
                <div className="chat-error-banner start-send-banner" role="alert">
                  {sendError}
                </div>
              )}
              {startPending && (
                <div className="start-outgoing-preview">
                  <StartOutgoingPreview content={startPending.content} displayMode={startPending.displayMode} />
                </div>
              )}
            </div>
            <ChatInputBar
              draft={draft}
              setDraft={setDraft}
              onSend={() => void handleSend()}
              busy={inputBusy}
              attachDisabled={sending}
              pendingFile={pendingFile}
              filePickError={filePickError}
              attachmentValidationError={attachmentValidationError}
              onAttachClick={handleOpenFilePicker}
              onUserChoseFilePath={ingestFilePath}
              onNotifyPickError={onNotifyPickError}
              onRemovePendingFile={() => { setPendingFile(null); setFilePickError(null) }}
            />
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
            <div className="chat-feed-wrap">
              <div className="chat-feed" ref={feedRef} onScroll={onFeedScroll}>
                {loadingMore && hasMore && !warmTransitionShell && (
                  <div className="chat-load-more" role="status" aria-live="polite">
                    <div className="chat-load-more-ring" aria-hidden />
                    Đang tải nội dung cũ...
                  </div>
                )}
                <div key={activeSessionId ?? 'none'} className="chat-feed-session">
                  {sendError && <div className="chat-error-banner">{sendError}</div>}
                  {showSessionFeedPlaceholder && <ChatSessionLoadingPreview />}
                  {!warmTransitionShell && (
                    <ChatMessageVirtualList
                      messages={messages}
                      assistantById={assistantById}
                      streamingAssistantId={streamingAssistantId}
                      fileTranslateProgress={fileTranslateProgress}
                      onRetranslate={handleRetranslate}
                      scrollElementRef={feedRef}
                      onScrollDate={handleScrollDate}
                      scrollToMessageId={scrollToMessageId}
                      onScrollToMessageDone={handleScrollToMessageDone}
                      highlightMessageId={highlightMessageId}
                    />
                  )}
                </div>
              </div>
              {chatStickyDate && (
                <div className={`chat-sticky-date${chatStickyVisible ? ' chat-sticky-date--visible' : ''}`}>
                  {formatStickyDate(chatStickyDate)}
                </div>
              )}
            </div>
            <ChatInputBar
              draft={draft}
              setDraft={setDraft}
              onSend={() => void handleSend()}
              busy={inputBusy}
              attachDisabled={sending}
              pendingFile={pendingFile}
              filePickError={filePickError}
              attachmentValidationError={attachmentValidationError}
              onAttachClick={handleOpenFilePicker}
              onUserChoseFilePath={ingestFilePath}
              onNotifyPickError={onNotifyPickError}
              onRemovePendingFile={() => { setPendingFile(null); setFilePickError(null) }}
            />
          </div>
        )}
      </main>
      {modelModalOpen && <ModelAIModal onClose={() => setModelModalOpen(false)} />}
    </div>
  )
}
