import type { Message } from './session'

export interface CreateSessionAndSendResult {
  sessionId: string
  messageId: string
}

export interface CreateSessionAndSendRequest {
  title?: string
  content: string
  displayMode: 'bubble' | 'bilingual' | 'file'
  sourceLang: string
  targetLang: string
  style?: string
}

export interface SendRequest {
  sessionId: string
  content: string
  displayMode: 'bubble' | 'bilingual' | 'file'
  sourceLang: string
  targetLang: string
  style?: string
  originalMessageId?: string
  provider?: string
  model?: string
  /** File retranslate: copy fileId to new assistant message. */
  fileId?: string
  /** File retranslate: stored as user message originalContent ("📎 filename.ext"). */
  fileDisplayContent?: string
}

export interface FileRequest {
  sessionId: string
  filePath: string
  /** Khớp ngôn ngữ đích trên UI (tránh dùng target_lang cũ của phiên). */
  targetLang?: string
  style?: string
  provider?: string
  model?: string
}

export interface FileInfo {
  name: string
  type: 'pdf' | 'docx' | 'doc'
  fileSize: number
  pageCount?: number
  charCount: number
  isScanned?: boolean
  estimatedChunks: number
  estimatedMinutes: number
}

/** Tệp vừa chọn — `loading` khi BE còn `ReadFileInfo` (pdfcpu có thể >1s). */
export type PendingFilePick = { path: string; info: FileInfo; loading?: boolean }

export interface FileContent {
  sourceMarkdown: string
  translatedMarkdown: string
}

export interface MessagesPage {
  messages: Message[]
  nextCursor: number
  hasMore: boolean
  cancelledFileIds: string[]
}

export interface SearchResult {
  messageId: string
  sessionId: string
  sessionTitle: string
  role: string
  snippet: string
  createdAt: string
}
