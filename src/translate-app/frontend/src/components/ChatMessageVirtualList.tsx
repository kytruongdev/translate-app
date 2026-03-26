import { memo, useEffect, useRef } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import type { RefObject } from 'react'
import type { Message } from '@/types/session'
import type { FileProgress } from '@/types/file'
import { ChatMessage, type RetranslatePayload } from '@/components/ChatMessage'
import { formatDateLabel } from '@/utils/dateLabel'

function toDateKey(iso: string): string {
  return iso.slice(0, 10)
}

export function formatStickyDate(iso: string): string {
  return formatDateLabel(new Date(iso))
}

type Props = {
  messages: Message[]
  assistantById: Map<string, Message>
  streamingAssistantId: string | null
  fileTranslateProgress: FileProgress | null
  onRetranslate: (p: RetranslatePayload) => Promise<void>
  /** `.chat-feed` — phần tử overflow-y: auto */
  scrollElementRef: RefObject<HTMLElement | null>
  /** Callback khi scroll: trả về date của tin đầu tiên visible */
  onScrollDate?: (date: string | null, visible: boolean) => void
}

const ROW_GAP = 16
/** Session nhỏ: render hết → scrollbar ổn định. Session lớn: giới hạn DOM node. */
const SMALL_SESSION_THRESHOLD = 150
const OVERSCAN_LARGE = 8

function ChatMessageVirtualListImpl({
  messages,
  assistantById,
  streamingAssistantId,
  fileTranslateProgress,
  onRetranslate,
  scrollElementRef,
  onScrollDate,
}: Props) {
  const count = messages.length
  const initialScrollDone = useRef(false)
  const spacerRef = useRef<HTMLDivElement>(null)
  const hideTimerRef = useRef<number>(0)

  const overscan = count <= SMALL_SESSION_THRESHOLD ? count : OVERSCAN_LARGE

  const virtualizer = useVirtualizer({
    count,
    getScrollElement: () => scrollElementRef.current,
    estimateSize: (index) => {
      const m = messages[index]
      if (!m) return 160
      if (m.displayMode === 'bilingual') return 480
      const len = (m.translatedContent || m.originalContent || '').length
      if (len > 400) return 200
      return 100
    },
    overscan,
    gap: ROW_GAP,
    getItemKey: (index) => messages[index]?.id ?? index,
    useFlushSync: false,
  })

  // Sticky date badge: update theo tin đầu tiên visible trong viewport
  useEffect(() => {
    const el = scrollElementRef.current
    if (!el || !onScrollDate) return

    const handleScroll = () => {
      const scrollTop = el.scrollTop
      const items = virtualizer.getVirtualItems()
      const first = items.find((row) => row.start + row.size > scrollTop)
      if (first) {
        const msg = messages[first.index]
        if (msg) onScrollDate(msg.createdAt, true)
      }
      clearTimeout(hideTimerRef.current)
      hideTimerRef.current = window.setTimeout(() => onScrollDate(null, false), 1200)
    }

    el.addEventListener('scroll', handleScroll, { passive: true })
    return () => {
      el.removeEventListener('scroll', handleScroll)
      clearTimeout(hideTimerRef.current)
    }
  }, [scrollElementRef, virtualizer, messages, onScrollDate])

  // Lần đầu tiên có tin: scroll xuống cuối.
  useEffect(() => {
    if (count === 0 || initialScrollDone.current) return
    initialScrollDone.current = true

    const el = scrollElementRef.current
    const spacer = spacerRef.current
    if (!el || !spacer) return

    virtualizer.scrollToIndex(count - 1, { align: 'end' })

    let done = false
    const snap = () => { if (!done) el.scrollTop = el.scrollHeight }

    snap()
    const ro = new ResizeObserver(snap)
    ro.observe(spacer)
    const timer = window.setTimeout(() => { done = true; ro.disconnect() }, 1500)
    return () => { done = true; ro.disconnect(); window.clearTimeout(timer) }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [count])

  if (count === 0) return null

  return (
    <div
        ref={spacerRef}
        className="chat-feed-virtual-spacer"
        style={{
          height: virtualizer.getTotalSize(),
          width: '100%',
          position: 'relative',
        }}
      >
        {virtualizer.getVirtualItems().map((virtualRow) => {
          const i = virtualRow.index
          const m = messages[i]
          if (!m) return null
          const prev = messages[i - 1]
          const retranslateFollowUp =
            m.role === 'assistant' && prev?.role === 'user' && Boolean(prev.originalMessageId)
          const retranslateQuoteAssistant =
            m.role === 'assistant' && prev?.role === 'user' && prev.originalMessageId
              ? assistantById.get(prev.originalMessageId)
              : undefined
          const alignEnd = m.role === 'user'
          const showDateSep = prev
            ? toDateKey(m.createdAt) !== toDateKey(prev.createdAt)
            : false
          return (
            <div
              key={virtualRow.key}
              data-index={virtualRow.index}
              ref={virtualizer.measureElement}
              className={`chat-feed-virtual-row${alignEnd ? ' chat-feed-virtual-row--user' : ' chat-feed-virtual-row--assistant'}`}
              style={{
                position: 'absolute',
                top: 0,
                left: 0,
                width: '100%',
                transform: `translateY(${virtualRow.start}px)`,
              }}
            >
              {showDateSep && (
                <div className="chat-date-sep">
                  <span className="chat-date-sep-pill">{formatStickyDate(m.createdAt)}</span>
                </div>
              )}
              <ChatMessage
                m={m}
                streamingAssistantId={streamingAssistantId}
                fileTranslateProgress={
                  m.role === 'assistant' && m.id === streamingAssistantId ? fileTranslateProgress : null
                }
                nextAssistant={messages[i + 1]?.role === 'assistant' ? messages[i + 1] : undefined}
                precedingUserContent={
                  m.role === 'assistant' && prev?.role === 'user' ? prev.originalContent : undefined
                }
                retranslateQuoteAssistant={retranslateQuoteAssistant}
                retranslateFollowUp={retranslateFollowUp}
                onRetranslate={onRetranslate}
              />
            </div>
          )
        })}
      </div>
  )
}

/** memo: ngăn re-render khi App.tsx update state không liên quan (draft, feedHeaderElevated, v.v.) */
export const ChatMessageVirtualList = memo(ChatMessageVirtualListImpl)
