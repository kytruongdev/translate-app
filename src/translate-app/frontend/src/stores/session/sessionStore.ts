import { create } from 'zustand'
import type { Session, SessionStatus } from '@/types/session'
import { WailsService } from '@/services/wailsService'

export interface SessionStore {
  sessions: Session[]
  activeSessionId: string | null
  /**
   * Vừa đổi sang một phiên chat (id khác trước đó): UI feed chỉ hiện view giả,
   * chưa mount ChatMessage — tránh khựng khi quay lại phiên đã cache nhưng list dài.
   */
  sessionTransitionPending: boolean
  loadSessions: () => Promise<void>
  appendSession: (session: Session) => void
  renameSession: (id: string, title: string) => Promise<void>
  updateStatus: (id: string, status: SessionStatus) => Promise<void>
  setActiveSession: (id: string | null) => void
  clearSessionTransition: () => void
}

export const useSessionStore = create<SessionStore>((set, get) => ({
  sessions: [],
  activeSessionId: null,
  sessionTransitionPending: false,

  loadSessions: async () => {
    const sessions = (await WailsService.getSessions()) as Session[]
    set({ sessions })
  },

  appendSession: (session) => {
    set({ sessions: [session, ...get().sessions.filter((s) => s.id !== session.id)] })
  },

  renameSession: async (id, title) => {
    await WailsService.renameSession(id, title)
    await get().loadSessions()
  },

  updateStatus: async (id, status) => {
    await WailsService.updateSessionStatus(id, status)
    await get().loadSessions()
  },

  setActiveSession: (id) => {
    const prev = get().activeSessionId
    const openingChat = id != null && id !== ''
    const changed = id !== prev
    set({
      activeSessionId: id,
      sessionTransitionPending: openingChat && changed,
    })
  },

  clearSessionTransition: () => set({ sessionTransitionPending: false }),
}))
