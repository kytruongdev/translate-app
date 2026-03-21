import type { TranslationStyle } from '@/types/session'

export const TARGET_LANG_OPTIONS: { value: string; label: string; chip: string }[] = [
  { value: 'en-US', label: 'English (US)', chip: 'EN' },
  { value: 'en-GB', label: 'English (UK)', chip: 'EN-GB' },
  { value: 'vi-VN', label: 'Tiếng Việt', chip: 'VI' },
  { value: 'ja-JP', label: '日本語', chip: 'JA' },
  { value: 'ko-KR', label: '한국어', chip: 'KO' },
  { value: 'zh-CN', label: '中文 (Giản thể)', chip: '中文' },
  { value: 'zh-TW', label: '中文 (Phồn thể)', chip: '繁中' },
  { value: 'fr-FR', label: 'Français', chip: 'FR' },
  { value: 'de-DE', label: 'Deutsch', chip: 'DE' },
  { value: 'es-ES', label: 'Español', chip: 'ES' },
]

export const STYLE_OPTIONS: { value: TranslationStyle; label: string }[] = [
  { value: 'casual', label: 'Casual' },
  { value: 'business', label: 'Business' },
  { value: 'academic', label: 'Academic' },
]
