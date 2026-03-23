import type { Message, TranslationStyle } from '@/types/session'

/** Ngưỡng “văn bản dài”: song ngữ, placeholder dán, v.v. */
export const LONG_TEXT_THRESHOLD = 1000

/**
 * File dịch: nguồn dài hơn ngưỡng này thì trong feed không cho “Mở rộng” inline —
 * chỉ hướng dẫn dùng toàn màn hình (tránh DOM nặng / cuộn kép).
 */
export const HEAVY_FILE_INLINE_CHAR_THRESHOLD = 28_000

export function isHeavyFileBilingualInline(m: { fileId: string | null }, sourceText: string): boolean {
  return Boolean(m.fileId && sourceText.length >= HEAVY_FILE_INLINE_CHAR_THRESHOLD)
}

/** Tin user đính kèm file: backend ghi `originalContent` dạng `📎 tên.ext`. */
const FILE_ATTACH_LINE = /^📎\s*(.+)$/u

export function parseFileAttachmentDisplayName(originalContent: string): string | null {
  const t = originalContent.trim()
  const m = t.match(FILE_ATTACH_LINE)
  if (!m?.[1]) return null
  const name = m[1].trim()
  return name || null
}

/** Tooltip khi tắt copy — cùng điều kiện `isHeavyFileBilingualInline` (gợi ý fullscreen / export). */
export const LARGE_DOCUMENT_COPY_DISABLED_TOOLTIP =
  'Chức năng sao chép không khả dụng với tài liệu lớn. Vui lòng sử dụng chức năng Export'

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
  // Tin đính kèm file: không dùng nhãn “Văn bản đã dán”.
  if (m.fileId) return false
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
