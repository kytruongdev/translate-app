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

/** Ước lượng ban đầu (px); hàng đo thật qua measureElement */
const ESTIMATE_ROW = 156
const ROW_GAP = 16
const OVERSCAN = 10

/**
 * Chỉ mount tin trong / gần viewport — giảm mạnh lag khi session có hàng trăm tin sau load-more.
 * Căn trái/phải giữ đúng bubble user / assistant (flex align-items).
 */
export function ChatMessageVirtualList({
  messages,
  assistantById,
  streamingAssistantId,
  fileTranslateProgress,
  onRetranslate,
  scrollElementRef,
}: Props) {
  const count = messages.length

  const virtualizer = useVirtualizer({
    count,
    getScrollElement: () => scrollElementRef.current,
    estimateSize: () => ESTIMATE_ROW,
    overscan: OVERSCAN,
    gap: ROW_GAP,
    getItemKey: (index) => messages[index]?.id ?? index,
    useFlushSync: false,
  })

  if (count === 0) return null

  return (
    <div
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
