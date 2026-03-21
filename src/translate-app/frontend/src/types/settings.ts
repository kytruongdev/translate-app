import type { TranslationStyle } from './session'

export type ThemeMode = 'light' | 'dark' | 'system'

/** V1: gemini | ollama only in Settings UI (openai reserved). */
export type ActiveProvider = 'gemini' | 'ollama'

export interface Settings {
  theme: ThemeMode
  activeProvider: ActiveProvider
  activeModel: string
  defaultStyle: TranslationStyle
  lastTargetLang: string
}
