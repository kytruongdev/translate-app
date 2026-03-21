import { create } from 'zustand'
import type { ActiveProvider, Settings, ThemeMode } from '@/types/settings'
import type { TranslationStyle } from '@/types/session'
import { WailsService } from '@/services/wailsService'
import { applyTheme } from '@/utils/applyTheme'
import { useUIStore } from '@/stores/ui/uiStore'

export interface SettingsStore {
  theme: ThemeMode
  activeProvider: ActiveProvider
  activeModel: string
  defaultStyle: TranslationStyle
  lastTargetLang: string
  loadSettings: () => Promise<void>
  saveSettings: (
    partial: Partial<
      Pick<SettingsStore, 'theme' | 'activeProvider' | 'activeModel' | 'defaultStyle' | 'lastTargetLang'>
    >,
  ) => Promise<void>
}

export const useSettingsStore = create<SettingsStore>((set, get) => ({
  theme: 'system',
  activeProvider: 'gemini',
  activeModel: 'gemini-2.0-flash',
  defaultStyle: 'casual',
  lastTargetLang: 'en-US',

  loadSettings: async () => {
    const s = await WailsService.getSettings()
    const theme = (s.theme as ThemeMode) ?? 'system'
    const lastTargetLang = s.lastTargetLang || 'en-US'
    set({
      theme,
      activeProvider: (s.activeProvider as ActiveProvider) ?? 'gemini',
      activeModel: s.activeModel,
      defaultStyle: s.defaultStyle,
      lastTargetLang,
    })
    applyTheme(theme)
    useUIStore.getState().setActiveTargetLang(lastTargetLang)
    useUIStore.getState().setActiveStyle(s.defaultStyle)
  },

  saveSettings: async (partial) => {
    const cur = await WailsService.getSettings()
    const st = get()
    const payload: Settings = {
      theme: partial.theme ?? st.theme,
      activeProvider: partial.activeProvider ?? st.activeProvider,
      activeModel: partial.activeModel ?? st.activeModel,
      defaultStyle: partial.defaultStyle ?? st.defaultStyle,
      lastTargetLang:
        partial.lastTargetLang ?? st.lastTargetLang ?? cur.lastTargetLang ?? 'en-US',
    }
    await WailsService.saveSettings(payload)
    set({
      theme: payload.theme,
      activeProvider: payload.activeProvider,
      activeModel: payload.activeModel,
      defaultStyle: payload.defaultStyle,
      lastTargetLang: payload.lastTargetLang,
    })
    if (partial.theme != null) {
      applyTheme(payload.theme)
    }
    if (partial.lastTargetLang != null) {
      useUIStore.getState().setActiveTargetLang(partial.lastTargetLang)
    }
    if (partial.defaultStyle != null) {
      useUIStore.getState().setActiveStyle(partial.defaultStyle)
    }
  },
}))
