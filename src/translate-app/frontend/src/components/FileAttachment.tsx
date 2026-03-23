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
  /** Đang gọi ReadFileInfo — hiện ngay tên file, tránh cảm giác “treo” sau khi chọn */
  loading?: boolean
}) {
  const label = fileInfo.name.length > 48 ? `${fileInfo.name.slice(0, 45)}…` : fileInfo.name
  const isPdf = fileInfo.type === 'pdf'

  return (
    <div
      className="file-attachment"
      data-invalid={error ? 'true' : 'false'}
      data-loading={loading ? 'true' : 'false'}
    >
      <div className="file-attachment-main">
        <span className="file-attachment-icon" aria-hidden>
          {isPdf ? 'PDF' : 'DOC'}
        </span>
        <div className="file-attachment-meta">
          <span className="file-attachment-name" title={fileInfo.name}>
            {label}
          </span>
          <span className="file-attachment-sub">
            {loading ? (
              <>Đang đọc tệp…</>
            ) : (
              <>
                {formatFileSize(fileInfo.fileSize)}
                {fileInfo.pageCount != null && fileInfo.pageCount > 0 ? ` · ${fileInfo.pageCount} trang` : ''}
                {fileInfo.estimatedMinutes > 0 ? ` · ~${fileInfo.estimatedMinutes} phút` : ''}
              </>
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
      {error ? <p className="file-attachment-error">{error}</p> : null}
    </div>
  )
}
