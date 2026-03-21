import type { Message, TranslationStyle } from '@/types/session'

/** Ngưỡng “văn bản dài”: song ngữ, placeholder dán, v.v. */
export const LONG_TEXT_THRESHOLD = 1000

export function styleLabel(style: TranslationStyle | ''): string {
  if (style === 'business') return 'Business'
  if (style === 'academic') return 'Academic'
  return 'Casual'
}

export function formatMessageFooterTime(iso: string): string {
  try {
    const d = new Date(iso)
    if (Number.isNaN(d.getTime())) return ''
    return d.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
  } catch {
    return ''
  }
}

/** Legacy / heuristic: tin user “dài” hoặc bilingual (dùng chung logic ngưỡng). */
export function userMessageUseCard(m: Message): boolean {
  return m.displayMode === 'bilingual' || m.originalContent.length >= LONG_TEXT_THRESHOLD
}

/** Assistant “dạng dài”: thẻ song ngữ hoặc bubble nhưng bản dịch đủ dài (kể cả đang stream). */
export function assistantMessageIsLongForm(
  assistant: Message,
  opts: { streamingAssistantId: string | null; streamingText: string },
): boolean {
  if (assistant.displayMode === 'bilingual') return true
  const body =
    assistant.id === opts.streamingAssistantId ? opts.streamingText : (assistant.translatedContent ?? '')
  return body.length >= LONG_TEXT_THRESHOLD
}

/**
 * Hiển thị bubble “Văn bản đã dán” thay vì full text khi:
 * - Assistant kế tiếp là thẻ song ngữ **hoặc** bản dịch (đã có / đang stream) ≥ ngưỡng dài, hoặc
 * - User đã bilingual / đủ dài.
 */
export function userMessageShowPastedPlaceholder(m: Message, pairedAssistantLongForm: boolean): boolean {
  if (m.role !== 'user') return false
  return (
    pairedAssistantLongForm ||
    m.displayMode === 'bilingual' ||
    m.originalContent.length >= LONG_TEXT_THRESHOLD
  )
}

export function langShortLabel(code: string): string {
  const map: Record<string, string> = {
    'en-US': 'EN',
    'en-GB': 'EN-GB',
    vi: 'VI',
    'vi-VN': 'VI',
    ja: 'JA',
    'ja-JP': 'JA',
    ko: 'KO',
    'zh-CN': '中文',
    'zh-TW': '繁中',
    fr: 'FR',
    de: 'DE',
    es: 'ES',
  }
  return map[code] ?? code
}
