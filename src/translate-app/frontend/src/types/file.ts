export interface FileAttachment {
  id: string
  sessionId: string
  fileName: string
  fileType: 'pdf' | 'docx'
  fileSize: number
  originalPath: string
  sourcePath: string
  translatedPath: string
  pageCount: number
  charCount: number
  style: 'casual' | 'business' | 'academic'
  modelUsed: string
  status: 'pending' | 'processing' | 'done' | 'error'
  errorMsg: string
  createdAt: string
  updatedAt: string
}

export interface FileResult {
  fileId: string
  fileName: string
  fileType: 'pdf' | 'docx'
  charCount: number
  pageCount: number
  tokensUsed: number
}

export interface FileProgress {
  chunk: number
  total: number
  percent: number
}
