import type { Message } from './session'

export interface CreateSessionAndSendResult {
  sessionId: string
  messageId: string
}

export interface CreateSessionAndSendRequest {
  title?: string
  content: string
  displayMode: 'bubble' | 'bilingual'
  sourceLang: string
  targetLang: string
  style?: string
}

export interface SendRequest {
  sessionId: string
  content: string
  displayMode: 'bubble' | 'bilingual'
  sourceLang: string
  targetLang: string
  style?: string
  originalMessageId?: string
  provider?: string
  model?: string
}

export interface FileRequest {
  sessionId: string
  filePath: string
  style?: string
  provider?: string
  model?: string
}

export interface FileInfo {
  name: string
  type: 'pdf' | 'docx'
  fileSize: number
  pageCount?: number
  charCount: number
  isScanned?: boolean
  estimatedChunks: number
  estimatedMinutes: number
}

export interface FileContent {
  sourceMarkdown: string
  translatedMarkdown: string
}

export interface MessagesPage {
  messages: Message[]
  nextCursor: number
  hasMore: boolean
}
