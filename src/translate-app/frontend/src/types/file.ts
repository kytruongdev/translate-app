/** Giới hạn số trang cho một lần dịch file — đồng bộ BE `internal/controller/file/translate.go` (`maxFilePages`). */
export const MAX_FILE_PAGE_COUNT = 200

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
