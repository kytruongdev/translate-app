import { FileText } from 'lucide-react'
import type { FileInfo } from '@/types/ipc'
import { formatFileSize } from '@/utils/formatFileSize'

export function FileAttachment({
  fileInfo,
  onRemove,
  error,
  loading,
}: {
  fileInfo: FileInfo
  onRemove: () => void
  error?: string | null
  /** Đang gọi ReadFileInfo — hiện ngay tên file, tránh cảm giác "treo" sau khi chọn */
  loading?: boolean
}) {
  const label = fileInfo.name.length > 48 ? `${fileInfo.name.slice(0, 45)}…` : fileInfo.name

  return (
    <div
      className="file-attachment"
      data-invalid={error ? 'true' : 'false'}
      data-loading={loading ? 'true' : 'false'}
    >
      <div className="file-attachment-main">
        <div className="file-attachment-icon-circle" aria-hidden>
          <FileText size={16} strokeWidth={1.5} />
        </div>
        <div className="file-attachment-meta">
          <span className="file-attachment-name" title={fileInfo.name}>
            {label}
          </span>
          <span className="file-attachment-sub">
            {error ? (
              error
            ) : loading ? (
              <>Đang đọc tệp…</>
            ) : (
              <>{formatFileSize(fileInfo.fileSize)}</>
            )}
          </span>
        </div>
        <button
          type="button"
          className="file-attachment-remove"
          aria-label="Bỏ tệp đính kèm"
          onClick={onRemove}
        >
          ×
        </button>
      </div>
    </div>
  )
}
