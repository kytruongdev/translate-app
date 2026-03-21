import type { Message, TranslationStyle } from '@/types/session'

/** Prefix FE dùng cho tin nhắn user tạm trước khi `SendMessage` xong. */
export const OPTIMISTIC_USER_PREFIX = 'optimistic-user:'

export function isOptimisticUserMessage(m: Message): boolean {
  return m.id.startsWith(OPTIMISTIC_USER_PREFIX)
}

export function buildOptimisticUserMessage(params: {
  sessionId: string
  content: string
  displayMode: 'bubble' | 'bilingual'
  sourceLang: string
  targetLang: string
  style: TranslationStyle
  nextDisplayOrder: number
  /** Tin user trả lời / dịch lại — trỏ tới assistant message (hoặc message gốc) */
  originalMessageId?: string | null
}): Message {
  const now = new Date().toISOString()
  return {
    id: `${OPTIMISTIC_USER_PREFIX}${Date.now()}`,
    sessionId: params.sessionId,
    role: 'user',
    displayOrder: params.nextDisplayOrder,
    displayMode: params.displayMode,
    originalContent: params.content,
    translatedContent: '',
    fileId: null,
    sourceLang: params.sourceLang,
    targetLang: params.targetLang,
    style: params.style,
    modelUsed: '',
    originalMessageId: params.originalMessageId ?? null,
    tokens: 0,
    createdAt: now,
    updatedAt: now,
  }
}
