import { memo, useEffect, useRef } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import type { RefObject } from 'react'
import type { Message } from '@/types/session'
import type { FileProgress } from '@/types/file'
import { ChatMessage, type RetranslatePayload } from '@/components/ChatMessage'

type Props = {
  messages: Message[]
  assistantById: Map<string, Message>
  streamingAssistantId: string | null
  fileTranslateProgress: FileProgress | null
  onRetranslate: (p: RetranslatePayload) => Promise<void>
  /** `.chat-feed` — phần tử overflow-y: auto */
  scrollElementRef: RefObject<HTMLElement | null>
}

const ROW_GAP = 16
/** Session nhỏ: render hết → scrollbar ổn định. Session lớn: giới hạn DOM node. */
const SMALL_SESSION_THRESHOLD = 150
const OVERSCAN_LARGE = 8

/**
 * Chỉ mount tin trong / gần viewport — giảm mạnh lag khi session có hàng trăm tin sau load-more.
 * Căn trái/phải giữ đúng bubble user / assistant (flex align-items).
 */
function ChatMessageVirtualListImpl({
  messages,
  assistantById,
  streamingAssistantId,
  fileTranslateProgress,
  onRetranslate,
  scrollElementRef,
}: Props) {
  const count = messages.length
  const initialScrollDone = useRef(false)
  const spacerRef = useRef<HTMLDivElement>(null)

  // Session nhỏ: overscan = count → render hết → đo thật hết → scrollbar không nhảy.
  // Session lớn: overscan nhỏ để tránh quá nhiều DOM node.
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

  // Lần đầu tiên có tin: scroll xuống cuối.
  // Virtualizer dùng estimated size → scrollToIndex chỉ đưa gần đúng.
  // ResizeObserver theo dõi spacer: mỗi lần item được đo thật + height cập nhật → re-snap.
  // Dừng sau 1.5s (đủ để tất cả item visible được đo xong).
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
