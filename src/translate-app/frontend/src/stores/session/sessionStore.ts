import { create } from 'zustand'
import type { Session, SessionStatus } from '@/types/session'
import { WailsService } from '@/services/wailsService'

export interface SessionStore {
  sessions: Session[]
  activeSessionId: string | null
  loadSessions: () => Promise<void>
  appendSession: (session: Session) => void
  renameSession: (id: string, title: string) => Promise<void>
  updateStatus: (id: string, status: SessionStatus) => Promise<void>
  setActiveSession: (id: string | null) => void
}

export const useSessionStore = create<SessionStore>((set, get) => ({
  sessions: [],
  activeSessionId: null,

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

  setActiveSession: (id) => set({ activeSessionId: id }),
}))
