import { create } from 'zustand'
import type { TranslationStyle } from '@/types/session'

export interface UIStore {
  sidebarCollapsed: boolean
  activeStyle: TranslationStyle
  activeTargetLang: string
  /** Chỉ một menu ⋮ session mở tại một thời điểm */
  sessionMenuOpenId: string | null
  /** Chỉ một dòng sidebar đang đổi tên inline (đóng dòng khác không lưu) */
  sessionInlineRenameId: string | null
  /**
   * File dịch nặng: sau `file:source` App đặt id assistant — TranslationCardView mở fullscreen một lần.
   */
  pendingTranslationFullscreenMessageId: string | null
  cancelledFileIds: string[]
  setSidebarCollapsed: (v: boolean) => void
  setActiveStyle: (style: TranslationStyle) => void
  setActiveTargetLang: (lang: string) => void
  setSessionMenuOpenId: (id: string | null) => void
  setSessionInlineRenameId: (id: string | null) => void
  setPendingTranslationFullscreenMessageId: (id: string | null) => void
  addCancelledFileId: (fileId: string) => void
}

export const useUIStore = create<UIStore>((set) => ({
  sidebarCollapsed: false,
  activeStyle: 'casual',
  activeTargetLang: 'en-US',
  sessionMenuOpenId: null,
  sessionInlineRenameId: null,
  pendingTranslationFullscreenMessageId: null,
  cancelledFileIds: [],
  setSidebarCollapsed: (sidebarCollapsed) => set({ sidebarCollapsed }),
  setActiveStyle: (activeStyle) => set({ activeStyle }),
  setActiveTargetLang: (activeTargetLang) => set({ activeTargetLang }),
  setSessionMenuOpenId: (sessionMenuOpenId) => set({ sessionMenuOpenId }),
  setSessionInlineRenameId: (sessionInlineRenameId) => set({ sessionInlineRenameId }),
  setPendingTranslationFullscreenMessageId: (pendingTranslationFullscreenMessageId) =>
    set({ pendingTranslationFullscreenMessageId }),
  addCancelledFileId: (fileId) =>
    set((s) => ({ cancelledFileIds: [...s.cancelledFileIds, fileId] })),
}))
