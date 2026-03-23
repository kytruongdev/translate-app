import { create } from 'zustand'
import type { Message } from '@/types/session'
import { WailsService } from '@/services/wailsService'

/**
 * Số tin mỗi lần gọi GetMessages (BE: ORDER BY display_order DESC).
 * Lần đầu (cursor=0): chunk mới nhất; loadMore dùng nextCursor → tin cũ hơn.
 */
const MESSAGE_PAGE_SIZE = 60

/** Một frame sau IPC — loadMore: prepend tin cũ mượt hơn */
function yieldToPaint(): Promise<void> {
  return new Promise((resolve) => {
    requestAnimationFrame(() => resolve())
  })
}

export type StreamStatus = 'idle' | 'pending' | 'streaming' | 'error'

/** Toàn bộ state + action (dùng type cho selector Zustand). */
export interface MessageStore {
  messages: Record<string, Message[]>
  cursors: Record<string, number>
  hasMore: Record<string, boolean>
  /** Đã hoàn tất lần loadMessages đầu cho phiên (kể cả lỗi) — dùng cho skeleton feed, không phụ thuộc state React. */
  sessionFeedReady: Record<string, boolean>
  streamStatus: StreamStatus
  streamingText: string
  loadMessages: (sessionId: string) => Promise<void>
  loadMoreMessages: (sessionId: string) => Promise<void>
  appendMessage: (sessionId: string, msg: Message) => void
  removeMessage: (sessionId: string, messageId: string) => void
  setStreamStatus: (status: StreamStatus) => void
  appendStreamingChunk: (chunk: string) => void
  clearStreamingText: () => void
  finalizeStream: (sessionId: string) => Promise<void>
}

export const useMessageStore = create<MessageStore>((set, get) => ({
  messages: {},
  cursors: {},
  hasMore: {},
  sessionFeedReady: {},
  streamStatus: 'idle',
  streamingText: '',

  loadMessages: async (sessionId) => {
    try {
      const page = await WailsService.getMessages(sessionId, 0, MESSAGE_PAGE_SIZE)
      const sorted = [...page.messages].sort((a, b) => a.displayOrder - b.displayOrder)
      set((s) => ({
        messages: { ...s.messages, [sessionId]: sorted },
        cursors: { ...s.cursors, [sessionId]: page.nextCursor },
        hasMore: { ...s.hasMore, [sessionId]: page.hasMore },
        sessionFeedReady: { ...s.sessionFeedReady, [sessionId]: true },
      }))
    } catch {
      set((s) => ({
        sessionFeedReady: { ...s.sessionFeedReady, [sessionId]: true },
      }))
    }
  },

  loadMoreMessages: async (sessionId) => {
    const cur = get().cursors[sessionId] ?? 0
    if (!cur) return
    const page = await WailsService.getMessages(sessionId, cur, MESSAGE_PAGE_SIZE)
    const older = [...page.messages].sort((a, b) => a.displayOrder - b.displayOrder)
    await yieldToPaint()
    set((s) => {
      const existing = s.messages[sessionId] ?? []
      // older luôn có displayOrder nhỏ hơn existing (cursor guarantee) → không cần sort lại toàn bộ
      const combined = [...older, ...existing]
      const seen = new Set<string>()
      const dedup = combined.filter((m) => {
        if (seen.has(m.id)) return false
        seen.add(m.id)
        return true
      })
      return {
        messages: { ...s.messages, [sessionId]: dedup },
        cursors: { ...s.cursors, [sessionId]: page.nextCursor },
        hasMore: { ...s.hasMore, [sessionId]: page.hasMore },
      }
    })
  },

  appendMessage: (sessionId, msg) =>
    set((s) => ({
      messages: {
        ...s.messages,
        [sessionId]: [...(s.messages[sessionId] ?? []), msg].sort((a, b) => a.displayOrder - b.displayOrder),
      },
      sessionFeedReady: { ...s.sessionFeedReady, [sessionId]: true },
    })),

  removeMessage: (sessionId, messageId) =>
    set((s) => {
      const list = s.messages[sessionId]
      if (!list?.length) return s
      const next = list.filter((m) => m.id !== messageId)
      if (next.length === list.length) return s
      return { messages: { ...s.messages, [sessionId]: next } }
    }),

  setStreamStatus: (streamStatus) => set({ streamStatus }),

  appendStreamingChunk: (chunk) => set((s) => ({ streamingText: s.streamingText + chunk })),

  clearStreamingText: () => set({ streamingText: '' }),

  finalizeStream: async (sessionId) => {
    set({ streamStatus: 'idle', streamingText: '' })
    // Upsert thay vì replace: giữ nguyên toàn bộ tin đã load từ load-more,
    // chỉ cập nhật / thêm các tin trong page mới nhất.
    try {
      const page = await WailsService.getMessages(sessionId, 0, MESSAGE_PAGE_SIZE)
      const incoming = [...page.messages].sort((a, b) => a.displayOrder - b.displayOrder)
      set((s) => {
        const existing = s.messages[sessionId] ?? []
        const byId = new Map(existing.map((m) => [m.id, m]))
        for (const m of incoming) byId.set(m.id, m)
        const merged = [...byId.values()].sort((a, b) => a.displayOrder - b.displayOrder)
        return {
          messages: { ...s.messages, [sessionId]: merged },
          sessionFeedReady: { ...s.sessionFeedReady, [sessionId]: true },
        }
      })
    } catch {
      // best-effort — streamingText đã clear, UI vẫn hiển thị text đã stream
    }
  },
}))
