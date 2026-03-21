import { create } from 'zustand'
import type { Message } from '@/types/session'
import { WailsService } from '@/services/wailsService'

export type StreamStatus = 'idle' | 'pending' | 'streaming' | 'error'

/** Toàn bộ state + action (dùng type cho selector Zustand). */
export interface MessageStore {
  messages: Record<string, Message[]>
  cursors: Record<string, number>
  hasMore: Record<string, boolean>
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
  streamStatus: 'idle',
  streamingText: '',

  loadMessages: async (sessionId) => {
    const page = await WailsService.getMessages(sessionId, 0, 50)
    const sorted = [...page.messages].sort((a, b) => a.displayOrder - b.displayOrder)
    set((s) => ({
      messages: { ...s.messages, [sessionId]: sorted },
      cursors: { ...s.cursors, [sessionId]: page.nextCursor },
      hasMore: { ...s.hasMore, [sessionId]: page.hasMore },
    }))
  },

  loadMoreMessages: async (sessionId) => {
    const cur = get().cursors[sessionId] ?? 0
    if (!cur) return
    const page = await WailsService.getMessages(sessionId, cur, 50)
    const older = [...page.messages].sort((a, b) => a.displayOrder - b.displayOrder)
    set((s) => {
      const existing = s.messages[sessionId] ?? []
      const merged = [...older, ...existing].sort((a, b) => a.displayOrder - b.displayOrder)
      const seen = new Set<string>()
      const dedup = merged.filter((m) => {
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
    await get().loadMessages(sessionId)
  },
}))
