export interface Session {
  id: string
  title: string
  status: SessionStatus
  targetLang: string
  style: TranslationStyle | ''
  model: string
  createdAt: string
  updatedAt: string
}

export interface Message {
  id: string
  sessionId: string
  role: 'user' | 'assistant'
  displayOrder: number
  displayMode: 'bubble' | 'bilingual' | 'file'
  originalContent: string
  translatedContent: string
  fileId: string | null
  sourceLang: string
  targetLang: string
  style: TranslationStyle
  modelUsed: string
  originalMessageId: string | null
  tokens: number
  fileSize?: number
  createdAt: string
  updatedAt: string
}

export type TranslationStyle = 'casual' | 'business' | 'academic'
export type SessionStatus = 'active' | 'pinned' | 'archived' | 'deleted'
