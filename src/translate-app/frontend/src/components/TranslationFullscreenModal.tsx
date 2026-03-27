import { useEffect, useMemo, useRef, useState } from 'react'
import { Download, RefreshCw, Copy, X, Maximize2 } from 'lucide-react'
import { LazyChunkedMarkdown, LazyChunkedPlainText } from '@/components/LazyChunkedMarkdown'
import { LARGE_DOCUMENT_COPY_DISABLED_TOOLTIP } from '@/utils/messageDisplay'
import { createPortal } from 'react-dom'
import { WailsService } from '@/services/wailsService'
import { CardExportPopover, CardRetranslatePopover } from '@/components/MessageCardPopovers'
import type { TranslationStyle } from '@/types/session'
import type { FileProgress } from '@/types/file'
import { HEAVY_FILE_INLINE_CHAR_THRESHOLD } from '@/utils/messageDisplay'
import {
  FileJobBilingualEdgeRail,
  fileJobDestTailMinPx,
  fileJobShowPartialDestTail,
} from '@/components/FileJobPanelProgressStrip'

export type PanelMode = 'bilingual' | 'dest' | 'src'

const IconExport = () => <Download size={18} aria-hidden />
const IconRetranslate = () => <RefreshCw size={18} aria-hidden />
const IconCopy = () => <Copy size={18} aria-hidden />
const IconClose = () => <X size={18} aria-hidden />
const IconFullscreen = () => <Maximize2 size={18} aria-hidden />

export function TranslationFullscreenModal({
  open,
  onClose,
  messageId,
  initialMode,
  src,
  dest,
  streaming,
  panelSrcHead,
  panelDestHead,
  footer,
  initialStyle,
  modelLabel,
  onExport,
  onRetranslateConfirm,
  retranslateDisabled,
  fileJobActive,
  fileJobProgress,
  copyLargeDocumentDisabled = false,
  topBarTitle,
}: {
  open: boolean
  onClose: () => void
  messageId: string
  initialMode: PanelMode
  src: string
  dest: string
  streaming: boolean
  panelSrcHead: string
  panelDestHead: string
  footer: string
  initialStyle: TranslationStyle
  modelLabel: string
  onExport: (format: 'pdf' | 'docx') => void
  onRetranslateConfirm: (style: TranslationStyle) => void
  retranslateDisabled?: boolean
  /** Dịch file đang chạy — thanh buffer + skeleton phần chưa xong */
  fileJobActive?: boolean
  fileJobProgress?: FileProgress | null
  /** Cùng điều kiện `isHeavyFileBilingualInline` (feed: không mở rộng inline) — tắt copy */
  copyLargeDocumentDisabled?: boolean
  /** Tên tệp (dịch file) — canh trái topbar */
  topBarTitle?: string | null
}) {
  const [mode, setMode] = useState<PanelMode>(initialMode)
  const [copied, setCopied] = useState(false)
  const [exportOpen, setExportOpen] = useState(false)
  const [retranslateOpen, setRetranslateOpen] = useState(false)
  const exportBtnRef = useRef<HTMLButtonElement>(null)
  const retranslateBtnRef = useRef<HTMLButtonElement>(null)
  const fsSrcScrollRef = useRef<HTMLDivElement>(null)
  const fsDestScrollRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (open) setMode(initialMode)
  }, [open, initialMode])

  useEffect(() => {
    if (!open) {
      setExportOpen(false)
      setRetranslateOpen(false)
    }
  }, [open])

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  const destTailMinPx = useMemo(() => fileJobDestTailMinPx(fileJobProgress ?? null), [fileJobProgress])

  if (!open) return null

  const showFileBuffer = Boolean(fileJobActive && streaming)
  const indeterminateProgress = !fileJobProgress || fileJobProgress.total < 1
  const bufferPercent = fileJobProgress && fileJobProgress.total > 0 ? fileJobProgress.percent : 0

  const showDestTail = fileJobShowPartialDestTail(!!fileJobActive, streaming, dest, fileJobProgress ?? null)

  const fsSrcPanelBody = (
    <div className="panel-body" ref={fsSrcScrollRef}>
      {streaming && src.length >= HEAVY_FILE_INLINE_CHAR_THRESHOLD ? (
        <LazyChunkedPlainText content={src} scrollRootRef={fsSrcScrollRef} />
      ) : (
        <LazyChunkedMarkdown content={src} scrollRootRef={fsSrcScrollRef} className="panel-body-text" />
      )}
    </div>
  )

  const fsDestPanelBody = (
    <div className="panel-body" ref={fsDestScrollRef}>
      {streaming && !dest ? (
        <div className="stream-skeleton" aria-busy="true" aria-label="Đang dịch">
          <span className="stream-skeleton-line medium" />
          <span className="stream-skeleton-line short" />
          <span className="stream-skeleton-line medium" />
        </div>
      ) : streaming && dest && showDestTail ? (
        <>
          <div className="message-md panel-body-text stream-dest-plain">{dest}</div>
          <div
            className="translation-fs-dest-tail"
            style={{ minHeight: destTailMinPx }}
            aria-hidden
          >
            <div className="translation-fs-skeleton-lines">
              <span className="translation-fs-skel-line" />
              <span className="translation-fs-skel-line short" />
              <span className="translation-fs-skel-line medium" />
              <span className="translation-fs-skel-line short" />
            </div>
          </div>
        </>
      ) : streaming && dest ? (
        <div className="message-md panel-body-text stream-dest-plain">{dest}</div>
      ) : (
        <LazyChunkedMarkdown content={dest} scrollRootRef={fsDestScrollRef} className="panel-body-text" />
      )}
    </div>
  )

  const onCopy = () => {
    void WailsService.copyTranslation(messageId)
      .then(() => {
        setCopied(true)
        window.setTimeout(() => setCopied(false), 900)
      })
      .catch(() => {})
  }

  return createPortal(
    <>
      <div className="fullscreen-backdrop open" onClick={onClose} aria-hidden />
      <div className="fullscreen-modal open" role="dialog" aria-modal="true" aria-label="Xem toàn màn hình">
        <div className="modal-body">
          <div className="fullscreen-topbar">
            <div className="card-topbar card-topbar--in-modal">
              <div className="card-topbar-left">
                <div className="card-inline-title" title={topBarTitle ?? undefined}>
                  {topBarTitle ?? ''}
                </div>
              </div>
              <div className="card-topbar-center">
                <div className="view-toggle" role="tablist" aria-label="Chế độ xem">
                  <button
                    type="button"
                    role="tab"
                    className={mode === 'bilingual' ? 'active' : ''}
                    onClick={() => setMode('bilingual')}
                  >
                    Song ngữ
                  </button>
                  <button
                    type="button"
                    role="tab"
                    className={mode === 'dest' ? 'active' : ''}
                    onClick={() => setMode('dest')}
                  >
                    Chỉ bản dịch
                  </button>
                  <button
                    type="button"
                    role="tab"
                    className={mode === 'src' ? 'active' : ''}
                    onClick={() => setMode('src')}
                  >
                    Chỉ nguồn
                  </button>
                </div>
              </div>
              <div className="card-topbar-right">
                <div className="card-inline-actions">
                  {topBarTitle && (
                    <button
                      ref={exportBtnRef}
                      type="button"
                      className="btn-icon"
                      aria-label="Export"
                      data-tooltip="Export PDF / Word"
                      disabled={streaming}
                      onClick={() => {
                        setRetranslateOpen(false)
                        setExportOpen((v) => !v)
                      }}
                    >
                      <IconExport />
                    </button>
                  )}
                  <button
                    ref={retranslateBtnRef}
                    type="button"
                    className="btn-icon"
                    aria-label="Dịch lại"
                    data-tooltip="Dịch lại"
                    disabled={streaming || retranslateDisabled}
                    onClick={() => {
                      setExportOpen(false)
                      setRetranslateOpen((v) => !v)
                    }}
                  >
                    <IconRetranslate />
                  </button>
                  {copyLargeDocumentDisabled ? (
                    <span
                      className="btn-icon-tooltip-host"
                      data-tooltip={LARGE_DOCUMENT_COPY_DISABLED_TOOLTIP}
                      data-tooltip-multiline=""
                    >
                      <button
                        type="button"
                        className="btn-icon"
                        disabled
                        aria-label="Sao chép bản dịch — không khả dụng với tài liệu lớn"
                      >
                        <IconCopy />
                      </button>
                    </span>
                  ) : (
                    <button type="button" className="btn-icon" aria-label="Sao chép bản dịch" data-tooltip="Sao chép bản dịch" onClick={onCopy}>
                      {copied ? '✓' : <IconCopy />}
                    </button>
                  )}
                  <button type="button" className="btn-icon modal-close" aria-label="Đóng" onClick={onClose}>
                    <IconClose />
                  </button>
                </div>
              </div>
            </div>
          </div>
          <div className="fullscreen-content">
            <div
              className="bilingual-view"
              data-mode={mode}
              data-file-rail={showFileBuffer ? 'active' : 'idle'}
            >
              <div className="translation-panel translation-panel--bilingual-head src">
                <div className="panel-head">{panelSrcHead}</div>
              </div>
              <div className="translation-panel translation-panel--bilingual-head dest">
                <div className="panel-head">{panelDestHead}</div>
              </div>
              <FileJobBilingualEdgeRail
                active={showFileBuffer}
                indeterminate={indeterminateProgress}
                percent={bufferPercent}
                chunk={fileJobProgress?.chunk}
                total={fileJobProgress?.total}
              />
              <div className="translation-panel translation-panel--bilingual-body src">{fsSrcPanelBody}</div>
              <div className="translation-panel translation-panel--bilingual-body dest">{fsDestPanelBody}</div>
            </div>
          </div>
        </div>
        <div className="fullscreen-footer">{footer}</div>
      </div>
      <CardExportPopover
        open={exportOpen}
        anchorRef={exportBtnRef}
        onClose={() => setExportOpen(false)}
        onExport={onExport}
      />
      <CardRetranslatePopover
        open={retranslateOpen}
        anchorRef={retranslateBtnRef}
        onClose={() => setRetranslateOpen(false)}
        initialStyle={initialStyle}
        modelLabel={modelLabel}
        onConfirm={onRetranslateConfirm}
      />
    </>,
    document.body,
  )
}

export { IconExport, IconRetranslate, IconCopy, IconFullscreen }
