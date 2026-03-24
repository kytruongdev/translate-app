import type { TranslationStyle } from '@/types/session'

export const TARGET_LANG_OPTIONS: { value: string; label: string; chip: string }[] = [
  { value: 'vi-VN', label: 'Tiếng Việt', chip: 'VI' },
  { value: 'en-US', label: 'Tiếng Anh (Mỹ)', chip: 'EN' },
  { value: 'en-GB', label: 'Tiếng Anh (Anh)', chip: 'EN-GB' },
  { value: 'zh-CN', label: 'Tiếng Hoa (Giản thể)', chip: '中文' },
  { value: 'zh-TW', label: 'Tiếng Hoa (Phồn thể)', chip: '繁中' },
  { value: 'ko-KR', label: 'Tiếng Hàn', chip: 'KO' },
  { value: 'ja-JP', label: 'Tiếng Nhật', chip: 'JA' },
  { value: 'fr-FR', label: 'Tiếng Pháp', chip: 'FR' },
  { value: 'de-DE', label: 'Tiếng Đức', chip: 'DE' },
  { value: 'es-ES', label: 'Tiếng Tây Ban Nha', chip: 'ES' },
]

export const STYLE_OPTIONS: { value: TranslationStyle; label: string; description: string }[] = [
  { value: 'casual', label: 'Thông thường', description: 'Tự nhiên, gần gũi — phù hợp giao tiếp hằng ngày' },
  { value: 'business', label: 'Chuyên nghiệp', description: 'Lịch sự, trang trọng — phù hợp môi trường công sở' },
  { value: 'academic', label: 'Học thuật', description: 'Thuật ngữ chuyên ngành — phù hợp nghiên cứu & báo cáo' },
]
