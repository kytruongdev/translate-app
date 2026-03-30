import type { TranslationStyle } from './session'

export type ThemeMode = 'light' | 'dark' | 'system'

export type ActiveProvider = 'ollama' | 'openai'

export interface Settings {
  theme: ThemeMode
  activeProvider: ActiveProvider
  activeModel: string
  defaultStyle: TranslationStyle
  lastTargetLang: string
}
