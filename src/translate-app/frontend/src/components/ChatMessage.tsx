import { createPortal } from 'react-dom'
import {
  memo,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type MouseEvent,
} from 'react'
import type { Message, TranslationStyle } from '@/types/session'
import type { FileProgress } from '@/types/file'
import { WailsService } from '@/services/wailsService'
import { useSettingsStore } from '@/stores/settings/settingsStore'
import { useUIStore } from '@/stores/ui/uiStore'
import { useMessageStore, type MessageStore } from '@/stores/message/messageStore'
import {
  assistantMessageIsLongForm,
  formatMessageFooterTime,
  isHeavyFileBilingualInline,
  langShortLabel,
  LARGE_DOCUMENT_COPY_DISABLED_TOOLTIP,
  parseFileAttachmentDisplayName,

  userMessageShowPastedPlaceholder,
} from '@/utils/messageDisplay'
import { formatFileSize } from '@/utils/formatFileSize'
import { MessageMarkdown } from '@/components/MessageMarkdown'
import { useBilingualHoverSync } from '@/hooks/useBilingualHoverSync'
import { CardExportPopover, CardRetranslatePopover } from '@/components/MessageCardPopovers'
import {
  FileJobBilingualEdgeRail,
  fileJobDestTailMinPx,
  fileJobShowPartialDestTail,
} from '@/components/FileJobPanelProgressStrip'
import {
  TranslationFullscreenModal,
  type PanelMode,
  IconExport,
  IconRetranslate,
  IconCopy as IconCopyCard,
  IconFullscreen,
} from '@/components/TranslationFullscreenModal'
import { LazyChunkedMarkdown, LazyChunkedPlainText } from '@/components/LazyChunkedMarkdown'
import { Copy, ChevronDown, FileText, ClipboardList, X } from 'lucide-react'


/** Snippet trích từ bản dịch gốc — giống mockup (≈140 ký tự). */
function retranslateQuotedSnippet(text: string, maxLen = 140): string {
  const t = text.replace(/\s+/g, ' ').trim()
  if (t.length <= maxLen) return t
  return `${t.slice(0, maxLen)}…`
}

function scrollToQuotedRetranslateTarget(quotedMessageId: string) {
  const el = document.getElementById(`chat-msg-${quotedMessageId}`)
  if (!el) return
  const target =
    (el.querySelector('.translation-card') as HTMLElement | null) ||
    (el.querySelector('.chat-bubble') as HTMLElement | null) ||
    el
  target.scrollIntoView({ behavior: 'smooth', block: 'center' })
  target.classList.remove('jump-highlight')
  void target.offsetWidth
  target.classList.add('jump-highlight')
  window.setTimeout(() => target.classList.remove('jump-highlight'), 1300)
}

/** Mockup: `.reply-quote` phía trên `.translation-card.retranslated` */
function RetranslateReplyQuote({
  quotedAssistantId,
  snippet,
}: {
  quotedAssistantId: string
  snippet: string
}) {
  const onJump = (e: MouseEvent<HTMLAnchorElement>) => {
    e.preventDefault()
    scrollToQuotedRetranslateTarget(quotedAssistantId)
  }
  return (
    <div className="reply-quote">
      <div className="rq-top">
        <a
          className="rq-link"
          href={`#chat-msg-${quotedAssistantId}`}
          onClick={onJump}
        >
          ↩ Bản gốc
        </a>
      </div>
      {snippet ? <div className="rq-snippet">“{snippet}”</div> : null}
    </div>
  )
}

/** Mockup: văn bản dài — bubble user chỉ “Văn bản đã dán”; nội dung đầy đủ ở panel Nguồn thẻ song ngữ. */
function UserPastedLongTextBubble({ meta, charCount }: { meta: string; charCount?: number }) {
  return (
    <>
      <div
        className="text-upload-bubble user-file-bubble"
        role="status"
        aria-label="Đã gửi văn bản dài. Nội dung đầy đủ nằm ở cột Nguồn trong thẻ dịch phía dưới."
      >
        <div className="user-file-bubble-icon" aria-hidden>
          <ClipboardList size={20} strokeWidth={1.5} />
        </div>
        <div className="user-file-bubble-meta">
          <span className="user-file-bubble-name">Văn bản đã dán</span>
          {charCount != null && charCount > 0 && (
            <span className="user-file-bubble-size">{charCount.toLocaleString('vi-VN')} ký tự</span>
          )}
        </div>
      </div>
      <div className="chat-lang-label">{meta}</div>
    </>
  )
}

const IconDocxFile = () => (
  <div className="user-file-bubble-icon" aria-hidden>
    <FileText size={20} strokeWidth={1.5} />
  </div>
)

function UserFileAttachmentBubble({
  fileName,
  meta,
  fileSize,
}: {
  fileName: string
  meta: string
  fileSize?: number
}) {
  return (
    <>
      <div
        className="text-upload-bubble user-file-bubble"
        role="status"
        aria-label={`Đã gửi tệp ${fileName}`}
      >
        <IconDocxFile />
        <div className="user-file-bubble-meta">
          <span className="user-file-bubble-name" title={fileName}>{fileName}</span>
          {fileSize && fileSize > 0 && (
            <span className="user-file-bubble-size">{formatFileSize(fileSize)}</span>
          )}
        </div>
      </div>
      <div className="chat-lang-label">{meta}</div>
    </>
  )
}

export type RetranslatePayload = {
  sourceContent: string
  assistantMessageId: string
  displayMode: 'bubble' | 'bilingual' | 'file'
  sourceLang: string
  targetLang: string
  style: TranslationStyle
  fileId?: string
  fileDisplayContent?: string
}

/** Heuristic: đủ dài để mặc định thu gọn + nút Mở rộng (tránh kéo thẻ quá cao trong feed). */
function translationCardIsCollapsible(src: string, dest: string, streaming: boolean): boolean {
  const s = src.length
  const d = dest.length
  if (streaming && (s > 180 || d > 60)) return true
  return s + d > 520 || Math.max(s, d) > 360
}

/** Nguồn trong thẻ: không parse lại Markdown mỗi chunk stream (src ổn định trong lượt dịch). */
const MemoMessageMarkdown = memo(function MemoMessageMarkdown({ content, wrapSentences }: { content: string; wrapSentences?: boolean }) {
  return <MessageMarkdown content={content} wrapSentences={wrapSentences} />
})

/** Đang stream: plain text — tránh react-markdown + GFM mỗi chunk (rất nặng khi văn bản dài). */
function StreamingPlainDest({ text }: { text: string }) {
  return <div className="message-md panel-body-text stream-dest-plain">{text}</div>
}

function TranslationCardView({
  m,
  destDisplay,
  streaming,
  precedingUserContent,
  isRetranslated,
  onRetranslate,
  fileTranslateProgress,
}: {
  m: Message
  destDisplay: string
  streaming: boolean
  precedingUserContent?: string
  /** Mockup: `.translation-card.retranslated` */
  isRetranslated?: boolean
  onRetranslate?: (p: RetranslatePayload) => Promise<void>
  fileTranslateProgress?: FileProgress | null
}) {
  const [mode, setMode] = useState<PanelMode>('bilingual')
  const [copied, setCopied] = useState(false)
  const [fullscreenOpen, setFullscreenOpen] = useState(false)
  const [exportOpen, setExportOpen] = useState(false)
  const [retranslateOpen, setRetranslateOpen] = useState(false)
  const [bodyExpanded, setBodyExpanded] = useState(false)
  const bilingualRef = useRef<HTMLDivElement>(null)
  const [bilingualScrollHeight, setBilingualScrollHeight] = useState(0)
  const [collapsedCapPx, setCollapsedCapPx] = useState(240)

  const activeProvider = useSettingsStore((s) => s.activeProvider)
  const activeModel = useSettingsStore((s) => s.activeModel)
  const defaultStyle = useSettingsStore((s) => s.defaultStyle)

  const exportRef = useRef<HTMLButtonElement>(null)
  const retranslateRef = useRef<HTMLButtonElement>(null)
  const srcPanelScrollRef = useRef<HTMLDivElement>(null)

  const bilingualFileTitle = useMemo(
    () => (m.fileId ? parseFileAttachmentDisplayName(precedingUserContent ?? '') : null),
    [m.fileId, precedingUserContent],
  )

  /* File dịch: nguồn đầy đủ ghi vào original_content của assistant; tin thường lấy từ user liền trước */
  const src =
    (m.originalContent?.trim() ? m.originalContent.trim() : '') ||
    (precedingUserContent?.trim() ?? '')
  const dest = destDisplay
  const collapsible = useMemo(() => translationCardIsCollapsible(src, dest, streaming), [src, dest, streaming])
  const heavyInlineNoExpand = useMemo(() => isHeavyFileBilingualInline(m, src), [m, src])
  const modelLabel = `${activeProvider} · ${activeModel}`
  const fileJobActive = Boolean(m.fileId && streaming)
  const showFileProgressStrip = fileJobActive
  const fileProgressIndeterminate = !fileTranslateProgress || fileTranslateProgress.total < 1
  const fileBufferPercent =
    fileTranslateProgress && fileTranslateProgress.total > 0 ? fileTranslateProgress.percent : 0
  const showPartialDestTail = fileJobShowPartialDestTail(fileJobActive, streaming, dest, fileTranslateProgress ?? null)
  const destTailMinPx = useMemo(() => fileJobDestTailMinPx(fileTranslateProgress ?? null), [fileTranslateProgress])
  const srcPanelBody = (
    <div className="panel-body" ref={srcPanelScrollRef}>
      {streaming && heavyInlineNoExpand ? (
        <LazyChunkedPlainText content={src} scrollRootRef={srcPanelScrollRef} />
      ) : heavyInlineNoExpand ? (
        <LazyChunkedMarkdown content={src} scrollRootRef={srcPanelScrollRef} />
      ) : (
        <MemoMessageMarkdown content={src} wrapSentences={!streaming && !heavyInlineNoExpand} />
      )}
    </div>
  )

  const destPanelBody = (
    <div className="panel-body">
      {streaming && !dest ? (
        <div className="stream-skeleton" aria-busy="true" aria-label="Đang dịch">
          <span className="stream-skeleton-line medium" />
          <span className="stream-skeleton-line short" />
          <span className="stream-skeleton-line medium" />
        </div>
      ) : streaming && dest && showPartialDestTail ? (
        <>
          <StreamingPlainDest text={dest} />
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
        <StreamingPlainDest text={dest} />
      ) : heavyInlineNoExpand ? (
        <LazyChunkedMarkdown content={dest} scrollRootRef={srcPanelScrollRef} />
      ) : (
        <MessageMarkdown content={dest} wrapSentences={!streaming && !heavyInlineNoExpand} />
      )}
    </div>
  )

  const pendingFullscreenId = useUIStore((s) => s.pendingTranslationFullscreenMessageId)
  const setPendingFullscreenId = useUIStore((s) => s.setPendingTranslationFullscreenMessageId)

  useEffect(() => {
    if (!pendingFullscreenId || pendingFullscreenId !== m.id) return
    setFullscreenOpen(true)
    setPendingFullscreenId(null)
  }, [pendingFullscreenId, m.id, setPendingFullscreenId])

  useEffect(() => {
    setBodyExpanded(false)
  }, [m.id])

  useEffect(() => {
    if (!collapsible) setBodyExpanded(false)
  }, [collapsible])

  useEffect(() => {
    if (heavyInlineNoExpand) setBodyExpanded(false)
  }, [heavyInlineNoExpand])

  useBilingualHoverSync(bilingualRef, src, dest, streaming)

  useEffect(() => {
    const upd = () => setCollapsedCapPx(Math.min(240, Math.round(window.innerHeight * 0.42)))
    upd()
    window.addEventListener('resize', upd)
    return () => window.removeEventListener('resize', upd)
  }, [])

  useLayoutEffect(() => {
    if (!collapsible) return
    /* File rất lớn + đang stream: đo scrollHeight mỗi lần dest đổi → layout thrash, treo UI */
    if (heavyInlineNoExpand && streaming) {
      setBilingualScrollHeight(0)
      return
    }
    const el = bilingualRef.current
    if (!el) return
    const measure = () => setBilingualScrollHeight(el.scrollHeight)
    measure()
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => ro.disconnect()
  }, [collapsible, heavyInlineNoExpand, m.id, src, dest, streaming, mode, bodyExpanded])

  const canRetranslate = Boolean(src.trim() && onRetranslate)

  const panelSrcHead = `Nguồn · ${langShortLabel(m.sourceLang || 'auto')}`
  const panelDestHead = `Bản dịch · ${langShortLabel(m.targetLang || 'en-US')}`
  const footerOutside = formatMessageFooterTime(m.updatedAt)

  const exportFileType: 'pdf' | 'docx' | 'xlsx' = bilingualFileTitle?.toLowerCase().endsWith('.xlsx')
    ? 'xlsx'
    : bilingualFileTitle?.toLowerCase().endsWith('.pdf')
    ? 'pdf'
    : 'docx'
  const exportFormat = exportFileType === 'xlsx' ? 'xlsx' : 'docx'

  const handleExport = async () => {
    if (!m.fileId) return
    try {
      await WailsService.exportFile(m.fileId, exportFormat)
    } catch (e) {
      window.alert(e instanceof Error ? e.message : String(e))
    }
  }

  const bilingualStyle = useMemo(() => {
    if (!collapsible) return undefined
    const cap = collapsedCapPx
    const expanded = bodyExpanded && !heavyInlineNoExpand
    if (streaming && expanded) {
      return {
        maxHeight: 'none' as const,
        overflow: 'visible' as const,
      }
    }
    const fullH = Math.max(bilingualScrollHeight, cap + 1)
    return {
      maxHeight: expanded ? `${fullH}px` : `${cap}px`,
      overflow: (expanded ? 'visible' : 'hidden') as 'visible' | 'hidden',
    }
  }, [collapsible, collapsedCapPx, streaming, bodyExpanded, bilingualScrollHeight, heavyInlineNoExpand])

  const runRetranslate = async (style: TranslationStyle) => {
    if (!src.trim() || !onRetranslate) return
    await onRetranslate({
      sourceContent: src.trim(),
      assistantMessageId: m.id,
      displayMode: m.displayMode,
      sourceLang: m.sourceLang,
      targetLang: m.targetLang,
      style,
      fileId: m.fileId ?? undefined,
      fileDisplayContent: m.fileId ? (precedingUserContent ?? undefined) : undefined,
    })
  }

  return (
    <>
      <div
        className={`translation-card${streaming ? ' is-streaming' : ''}${isRetranslated ? ' retranslated' : ''}`}
      >
        <div className="card-topbar">
          <div className="card-topbar-left">
            {bilingualFileTitle ? (
              <div className="card-inline-title" title={bilingualFileTitle}>
                {bilingualFileTitle}
              </div>
            ) : (
              <div className="card-inline-meta">
                <span className="card-inline-meta-title">Dịch văn bản dài</span>
                {!streaming && m.tokens > 0 && (
                  <span className="card-inline-meta-sub">{m.tokens.toLocaleString()} tokens</span>
                )}
              </div>
            )}
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
              {m.fileId && (
                <button
                  ref={exportRef}
                  type="button"
                  className="btn-icon export-trigger"
                  aria-label="Export"
                  disabled={streaming}
                  data-tooltip="Export PDF / Word"
                  onClick={() => {
                    setRetranslateOpen(false)
                    setExportOpen((v) => !v)
                  }}
                >
                  <IconExport />
                </button>
              )}
              <button
                ref={retranslateRef}
                type="button"
                className="btn-icon"
                aria-label="Dịch lại"
                disabled={streaming || !canRetranslate}
                data-tooltip="Dịch lại"
                onClick={() => {
                  setExportOpen(false)
                  setRetranslateOpen((v) => !v)
                }}
              >
                <IconRetranslate />
              </button>
              {!streaming &&
                (heavyInlineNoExpand ? (
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
                      <IconCopyCard />
                    </button>
                  </span>
                ) : (
                  <button
                    type="button"
                    className="btn-icon"
                    data-tooltip="Sao chép bản dịch"
                    aria-label="Sao chép bản dịch"
                    onClick={() => {
                      void WailsService.copyTranslation(m.id)
                        .then(() => {
                          setCopied(true)
                          window.setTimeout(() => setCopied(false), 900)
                        })
                        .catch(() => {})
                    }}
                  >
                    {copied ? '✓' : <IconCopyCard />}
                  </button>
                ))}
              <button
                type="button"
                className="btn-icon always-on"
                aria-label="Xem toàn màn hình"
                data-tooltip="Xem toàn màn hình"
                onClick={() => setFullscreenOpen(true)}
              >
                <IconFullscreen />
              </button>
            </div>
          </div>
        </div>
        <div
          className="translation-card-expandable"
          data-collapsible={collapsible ? 'true' : 'false'}
          data-expanded={
            collapsible ? (heavyInlineNoExpand || !bodyExpanded ? 'false' : 'true') : 'true'
          }
          data-heavy-inline={heavyInlineNoExpand ? 'true' : 'false'}
        >
          <div className="translation-card-expandable-main">
            <div
              ref={bilingualRef}
              className="bilingual-view"
              data-mode={mode}
              data-file-rail={showFileProgressStrip ? 'active' : 'idle'}
              id={`translation-card-body-${m.id}`}
              style={bilingualStyle}
            >
              <div className="translation-panel translation-panel--bilingual-head src">
                <div className="panel-head">{panelSrcHead}</div>
              </div>
              <div className="translation-panel translation-panel--bilingual-head dest">
                <div className="panel-head">{panelDestHead}</div>
              </div>
              <FileJobBilingualEdgeRail
                active={showFileProgressStrip}
                indeterminate={fileProgressIndeterminate}
                percent={fileBufferPercent}
                chunk={fileTranslateProgress?.chunk}
                total={fileTranslateProgress?.total}
              />
              <div className="translation-panel translation-panel--bilingual-body src">{srcPanelBody}</div>
              <div className="translation-panel translation-panel--bilingual-body dest">{destPanelBody}</div>
            </div>
          </div>
          {collapsible && (
            <div className="translation-card-expand-controls">
              {heavyInlineNoExpand ? (
                <p className="translation-card-heavy-inline-hint" role="note">
                  Tài liệu quá lớn, vui lòng chọn chế độ Toàn màn hình để xem.
                </p>
              ) : (
                <button
                  type="button"
                  className={`translation-card-expand-btn${bodyExpanded ? ' is-expanded' : ''}`}
                  aria-expanded={bodyExpanded}
                  aria-controls={`translation-card-body-${m.id}`}
                  id={`translation-card-expand-${m.id}`}
                  onClick={() => setBodyExpanded((v) => !v)}
                >
                  <span className="translation-card-expand-btn-label">{bodyExpanded ? 'Thu gọn' : 'Mở rộng'}</span>
                  <span className="translation-card-expand-btn-chevron" aria-hidden>
                    <ChevronDown size={16} aria-hidden />
                  </span>
                </button>
              )}
            </div>
          )}
        </div>
      </div>
      <CardExportPopover
        open={exportOpen}
        anchorRef={exportRef}
        onClose={() => setExportOpen(false)}
        fileType={exportFileType}
        onExport={() => void handleExport()}
      />
      <CardRetranslatePopover
        open={retranslateOpen}
        anchorRef={retranslateRef}
        onClose={() => setRetranslateOpen(false)}
        initialStyle={m.style || defaultStyle}
        modelLabel={modelLabel}
        onConfirm={(style) => void runRetranslate(style)}
      />
      <TranslationFullscreenModal
        open={fullscreenOpen}
        onClose={() => setFullscreenOpen(false)}
        messageId={m.id}
        initialMode={mode}
        src={src}
        dest={dest}
        streaming={streaming}
        panelSrcHead={panelSrcHead}
        panelDestHead={panelDestHead}
        footer={footerOutside}
        initialStyle={m.style || defaultStyle}
        modelLabel={modelLabel}
        fileType={exportFileType}
        onExport={() => void handleExport()}
        onRetranslateConfirm={(style) => void runRetranslate(style)}
        retranslateDisabled={!canRetranslate}
        fileJobActive={fileJobActive}
        fileJobProgress={fileTranslateProgress ?? null}
        copyLargeDocumentDisabled={heavyInlineNoExpand}
        topBarTitle={bilingualFileTitle}
      />
    </>
  )
}

const IconDownload = () => (
  <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" width={18} height={18} aria-hidden>
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth={2}
      d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
    />
  </svg>
)


function FileTranslationCard({
  m,
  streaming,
  fileTranslateProgress,
  precedingUserContent,
}: {
  m: Message
  streaming: boolean
  fileTranslateProgress?: FileProgress | null
  precedingUserContent?: string
}) {
  const cancelledFileIds = useUIStore((s) => s.cancelledFileIds)
  const cancelledByUser = Boolean(m.fileId && cancelledFileIds.includes(m.fileId))
  const [downloading, setDownloading] = useState(false)
  const [downloadError, setDownloadError] = useState<string | null>(null)
  const [cancelPopoverOpen, setCancelPopoverOpen] = useState(false)
  const [cancelTooltipPos, setCancelTooltipPos] = useState<{ top: number; left: number } | null>(null)
  const cancelBtnRef = useRef<HTMLButtonElement | null>(null)
  const [cancelPopoverPos, setCancelPopoverPos] = useState<{ top: number; left: number; width: number; isAbove: boolean } | null>(null)

  useLayoutEffect(() => {
    if (!cancelPopoverOpen || !cancelBtnRef.current) {
      setCancelPopoverPos(null)
      return
    }
    const r = cancelBtnRef.current.getBoundingClientRect()
    const width = Math.min(300, window.innerWidth - 24)
    let left = r.left + r.width / 2 - width / 2
    left = Math.max(12, Math.min(left, window.innerWidth - width - 12))
    let top = r.bottom + 8
    let isAbove = false
    if (top + 160 > window.innerHeight - 12) {
      top = Math.max(12, r.top - 160 - 8)
      isAbove = true
    }
    setCancelPopoverPos({ top, left, width, isAbove })
  }, [cancelPopoverOpen])

  const fileName =
    parseFileAttachmentDisplayName(precedingUserContent ?? '') ??
    parseFileAttachmentDisplayName(m.originalContent ?? '') ??
    'document.docx'
  const isXlsx = fileName.toLowerCase().endsWith('.xlsx')
  const fileExportFormat = isXlsx ? 'xlsx' : 'docx'

  const rawPct = fileTranslateProgress?.percent ?? 0

  // Chỉ tăng, không giảm — tránh giật lùi giữa các batch
  const maxPctRef = useRef(0)
  if (!streaming) maxPctRef.current = 0
  else if (rawPct > maxPctRef.current) maxPctRef.current = rawPct
  const pct = streaming ? maxPctRef.current : rawPct

  const indeterminate = !fileTranslateProgress || fileTranslateProgress.total < 1

  const handleDownload = async () => {
    if (!m.fileId) return
    setDownloading(true)
    setDownloadError(null)
    try {
      await WailsService.exportFile(m.fileId, fileExportFormat)
    } catch (e) {
      setDownloadError(e instanceof Error ? e.message : String(e))
    } finally {
      setDownloading(false)
    }
  }

  return (
    <>
    <div className="file-translation-card-wrap">
      <div className="file-translation-card">
        {/* Main row: icon + info */}
        <div className="file-translation-card-main">
          <div className={`file-card-icon-wrap${streaming ? ' file-card-icon-wrap--streaming' : ''}`}>
            {streaming ? (
              <button
                ref={cancelBtnRef}
                type="button"
                className="file-card-cancel-btn"
                aria-label="Hủy phiên dịch này"
                onMouseEnter={() => {
                  if (!cancelBtnRef.current) return
                  const r = cancelBtnRef.current.getBoundingClientRect()
                  setCancelTooltipPos({ top: r.top - 8, left: r.left + r.width / 2 })
                }}
                onMouseLeave={() => setCancelTooltipPos(null)}
                onClick={() => { setCancelTooltipPos(null); setCancelPopoverOpen((v) => !v) }}
              >
                <X size={18} strokeWidth={2} />
              </button>
            ) : (
              <div className="user-file-bubble-icon" aria-hidden>
                <FileText size={20} strokeWidth={1.5} />
              </div>
            )}
            {streaming && (
              <svg className="file-card-progress-ring" viewBox="0 0 40 40" aria-hidden>
                <defs>
                  <linearGradient id="file-ring-grad" x1="0" y1="0" x2="40" y2="40" gradientUnits="userSpaceOnUse">
                    <stop offset="0%" stopColor="#89f7fe" />
                    <stop offset="100%" stopColor="#818cf8" />
                  </linearGradient>
                </defs>
                <circle
                  className={`file-card-progress-arc${indeterminate ? ' file-card-progress-arc--spin' : ''}`}
                  cx="20" cy="20" r="19"
                  stroke="url(#file-ring-grad)"
                  style={indeterminate ? undefined : { strokeDashoffset: `${119.4 * (1 - pct / 100)}` }}
                />
              </svg>
            )}
          </div>
          <div className="file-translation-card-info">
            <span className={`file-translation-card-name${cancelledByUser ? ' file-translation-card-name--cancelled' : ''}`} title={fileName}>
              {fileName}
            </span>
            <span className="file-translation-card-sub">
              {cancelledByUser ? (
                <span className="file-translation-card-cancelled">Đã hủy phiên dịch</span>
              ) : streaming ? (
                indeterminate ? 'Đang dịch...' : `Đang dịch · ${pct}%`
              ) : downloadError ? (
                <span className="file-translation-card-error">{downloadError}</span>
              ) : m.tokens > 0 ? (
                `${m.tokens.toLocaleString()} tokens`
              ) : null}
            </span>
          </div>
        </div>

      </div>

      {/* Floating action buttons */}
      <div className="file-translation-card-actions">
        <button
          type="button"
          className="btn-icon file-card-action-btn"
          aria-label="Tải file đã dịch"
          data-tooltip="Tải file đã dịch"
          disabled={streaming || downloading || !m.fileId || cancelledByUser}
          onClick={() => void handleDownload()}
        >
          <IconDownload />
        </button>
      </div>

    </div>
    {/* Cancel confirm popover — rendered via portal to escape transform stacking context */}
    {cancelPopoverOpen && cancelPopoverPos && createPortal(
      <div className="file-cancel-popover-backdrop" onClick={() => setCancelPopoverOpen(false)}>
        <div
          className={`file-cancel-popover${cancelPopoverPos.isAbove ? ' is-above' : ''}`}
          role="dialog"
          aria-modal="true"
          aria-labelledby="file-cancel-popover-title"
          style={{ top: cancelPopoverPos.top, left: cancelPopoverPos.left, width: cancelPopoverPos.width }}
          onClick={(e) => e.stopPropagation()}
        >
          <div className="file-cancel-popover-header">
            <span id="file-cancel-popover-title" className="file-cancel-popover-title">Hủy phiên dịch này</span>
          </div>
          <div className="file-cancel-popover-body">
            Bạn có chắc muốn hủy phiên dịch file <strong>{fileName}</strong>?
          </div>
          <div className="file-cancel-popover-actions">
            <button
              type="button"
              className="file-cancel-popover-btn cancel"
              onClick={() => setCancelPopoverOpen(false)}
            >
              Thoát
            </button>
            <button
              type="button"
              className="file-cancel-popover-btn confirm"
              onClick={() => {
                setCancelPopoverOpen(false)
                if (m.fileId) void WailsService.cancelFileTranslate(m.fileId).catch(() => {})
              }}
            >
              Hủy
            </button>
          </div>
        </div>
      </div>,
      document.body,
    )}
    {cancelTooltipPos && createPortal(
      <div
        className="file-cancel-btn-tooltip"
        style={{ top: cancelTooltipPos.top, left: cancelTooltipPos.left }}
      >
        Hủy phiên dịch này
      </div>,
      document.body,
    )}
</>
  )
}

function BubbleActions({
  messageId,
  heavyInlineNoExpand,
  m,
  streaming,
  precedingUserContent,
  onRetranslate,
}: {
  messageId: string
  heavyInlineNoExpand: boolean
  m: Message
  streaming: boolean
  precedingUserContent?: string
  onRetranslate?: (p: RetranslatePayload) => Promise<void>
}) {
  const [flash, setFlash] = useState(false)
  const [retranslateOpen, setRetranslateOpen] = useState(false)
  const retranslateRef = useRef<HTMLButtonElement | null>(null)
  const { activeProvider, activeModel } = useSettingsStore()
  const modelLabel = `${activeProvider} · ${activeModel}`
  const sourceContent = m.originalContent?.trim() || precedingUserContent?.trim() || ''
  const canRetranslate = Boolean(sourceContent && onRetranslate)

  return (
    <>
    <div className="assistant-bubble-actions">
      {canRetranslate && (
        <button
          ref={retranslateRef}
          type="button"
          className="btn-icon"
          aria-label="Dịch lại"
          disabled={streaming}
          data-tooltip="Dịch lại"
          onClick={() => setRetranslateOpen((v) => !v)}
        >
          <IconRetranslate />
        </button>
      )}
      {heavyInlineNoExpand ? (
        <span
          className="btn-icon-tooltip-host"
          data-tooltip={LARGE_DOCUMENT_COPY_DISABLED_TOOLTIP}
          data-tooltip-multiline=""
        >
          <button
            type="button"
            className="btn-icon"
            disabled
            aria-label="Sao chép — không khả dụng với tài liệu lớn"
          >
            <Copy size={18} aria-hidden />
          </button>
        </span>
      ) : (
        <button
          type="button"
          className="btn-icon"
          data-tooltip="Sao chép"
          aria-label="Sao chép"
          onClick={() => {
            void WailsService.copyTranslation(messageId)
              .then(() => {
                setFlash(true)
                window.setTimeout(() => setFlash(false), 900)
              })
              .catch(() => {})
          }}
        >
          {flash ? '✓' : <Copy size={18} aria-hidden />}
        </button>
      )}
    </div>
    <CardRetranslatePopover
      open={retranslateOpen}
      anchorRef={retranslateRef}
      onClose={() => setRetranslateOpen(false)}
      initialStyle={m.style ?? 'casual'}
      modelLabel={modelLabel}
      onConfirm={(style) => {
        setRetranslateOpen(false)
        if (!onRetranslate) return
        void onRetranslate({
          sourceContent,
          assistantMessageId: m.id,
          displayMode: 'bubble',
          sourceLang: m.sourceLang ?? 'auto',
          targetLang: m.targetLang ?? 'en-US',
          style,
        })
      }}
    />
    </>
  )
}

type ChatMessageProps = {
  m: Message
  /** Chỉ tin đang stream subscribe buffer — tránh App re-render cả layout mỗi chunk. */
  streamingAssistantId: string | null
  /** Tiến độ chunk khi dịch file (fullscreen buffer bar). */
  fileTranslateProgress?: FileProgress | null
  /** Tin user: assistant ngay sau (nếu có), để tính “Văn bản đã dán” + buffer stream. */
  nextAssistant?: Message
  precedingUserContent?: string
  /** Bản dịch gốc (assistant) để quote + jump — `prev.originalMessageId`. */
  retranslateQuoteAssistant?: Message
  /** Tin assistant ngay sau user dịch lại (`originalMessageId`). */
  retranslateFollowUp?: boolean
  onRetranslate?: (p: RetranslatePayload) => Promise<void>
  /** Highlight this message (from search jump). */
  highlight?: boolean
}

function ChatMessageImpl({
  m,
  streamingAssistantId,
  fileTranslateProgress,
  nextAssistant,
  precedingUserContent,
  retranslateQuoteAssistant,
  retranslateFollowUp,
  onRetranslate,
  highlight,
}: ChatMessageProps) {
  const selectStreamBuf = useCallback(
    (s: MessageStore) => {
      if (m.role === 'assistant' && m.id === streamingAssistantId) return s.streamingText
      if (m.role === 'user' && nextAssistant?.id === streamingAssistantId) return s.streamingText
      return ''
    },
    [m.role, m.id, nextAssistant?.id, streamingAssistantId],
  )
  const streamBuf = useMessageStore(selectStreamBuf)

  // Highlight toàn bộ row khi jump từ search — áp background lên virtual row (parent của chat-msg).
  // onScrollToMessageDone đã chờ scroll dừng hẳn → không cần delay.
  useEffect(() => {
    if (!highlight) return
    const msgEl = document.getElementById(`chat-msg-${m.id}`)
    const row = msgEl?.parentElement as HTMLElement | null
    if (!row) return
    row.classList.remove('row-highlight')
    void row.offsetWidth
    row.classList.add('row-highlight')
    const t = window.setTimeout(() => row.classList.remove('row-highlight'), 2500)
    return () => {
      window.clearTimeout(t)
      row.classList.remove('row-highlight')
    }
  }, [highlight, m.id])

  const pairedAssistantLongForm = useMemo(() => {
    if (m.role !== 'user' || nextAssistant?.role !== 'assistant') return false
    return assistantMessageIsLongForm(nextAssistant, {
      streamingAssistantId,
      streamingText: streamBuf,
    })
  }, [m.role, nextAssistant, streamingAssistantId, streamBuf])

  const userMeta = formatMessageFooterTime(m.createdAt)

  if (m.role === 'user') {
    if (m.originalMessageId) {
      /* Mockup: không có bubble user riêng — quote nằm trên thẻ assistant phía dưới */
      return (
        <div
          className="chat-msg user user-retranslate-placeholder"
          id={`chat-msg-${m.id}`}
          aria-hidden="true"
        />
      )
    }
    const fileAttachName = m.fileId ? parseFileAttachmentDisplayName(m.originalContent) : null
    if (fileAttachName) {
      return (
        <div className="chat-msg user user-text-upload user-file-attachment" id={`chat-msg-${m.id}`}>
          <div className="avatar" aria-hidden>
            G
          </div>
          <div className="chat-msg-body">
            <UserFileAttachmentBubble fileName={fileAttachName} meta={userMeta} fileSize={m.fileSize} />
          </div>
        </div>
      )
    }
    if (userMessageShowPastedPlaceholder(m, pairedAssistantLongForm)) {
      return (
        <div className="chat-msg user user-text-upload" id={`chat-msg-${m.id}`}>
          <div className="avatar" aria-hidden>
            G
          </div>
          <div className="chat-msg-body">
            <UserPastedLongTextBubble meta={userMeta} charCount={m.originalContent?.length} />
          </div>
        </div>
      )
    }
    return (
      <div className={`chat-msg user${highlight ? ' chat-msg--highlight' : ''}`} id={`chat-msg-${m.id}`}>
        <div className="avatar" aria-hidden>
          G
        </div>
        <div className="chat-msg-body">
          <div className="chat-bubble">
            <MessageMarkdown content={m.originalContent} />
          </div>
          <div className="chat-lang-label">{userMeta}</div>
        </div>
      </div>
    )
  }

  const streaming = m.id === streamingAssistantId
  const destText = streaming ? streamBuf : m.translatedContent
  const bubbleHeavyInline = isHeavyFileBilingualInline(m, m.originalContent?.trim() ?? '')

  const quotedSnippet =
    retranslateFollowUp && retranslateQuoteAssistant
      ? retranslateQuotedSnippet(retranslateQuoteAssistant.translatedContent || '')
      : ''

  if (m.displayMode === 'file') {
    return (
      <div className={`chat-msg assistant${highlight ? ' chat-msg--highlight' : ''}`} id={`chat-msg-${m.id}`}>
        <div className="avatar assistant-avatar" aria-hidden>
          J
        </div>
        <div className="chat-msg-body">
          <FileTranslationCard
            m={m}
            streaming={streaming}
            fileTranslateProgress={fileTranslateProgress}
            precedingUserContent={precedingUserContent}
            />
          {!streaming && (
            <div className="card-footer-outside">
              {formatMessageFooterTime(m.updatedAt)}
            </div>
          )}
        </div>
      </div>
    )
  }

  if (m.displayMode === 'bilingual') {
    return (
      <div
        className={`chat-msg assistant${retranslateFollowUp ? ' retranslate-reply' : ''}${highlight ? ' chat-msg--highlight' : ''}`}
        id={`chat-msg-${m.id}`}
      >
        <div className="avatar assistant-avatar" aria-hidden>
          J
        </div>
        <div className="chat-msg-body translation-card-wrap">
          {retranslateFollowUp && retranslateQuoteAssistant ? (
            <RetranslateReplyQuote
              quotedAssistantId={retranslateQuoteAssistant.id}
              snippet={quotedSnippet}
            />
          ) : null}
          <TranslationCardView
            m={m}
            destDisplay={destText}
            streaming={streaming}
            precedingUserContent={precedingUserContent}
            isRetranslated={retranslateFollowUp}
            onRetranslate={onRetranslate}
            fileTranslateProgress={fileTranslateProgress}
          />
          {!streaming && (
            <div className="card-footer-outside">
              {formatMessageFooterTime(m.updatedAt)}
            </div>
          )}
        </div>
      </div>
    )
  }

  return (
    <div className={`chat-msg assistant${retranslateFollowUp ? ' retranslate-reply' : ''}${highlight ? ' chat-msg--highlight' : ''}`} id={`chat-msg-${m.id}`}>
      <div className="avatar assistant-avatar" aria-hidden>
        J
      </div>
      <div className="chat-msg-body">
        {retranslateFollowUp && retranslateQuoteAssistant && (
          <RetranslateReplyQuote
            quotedAssistantId={retranslateQuoteAssistant.id}
            snippet={quotedSnippet}
          />
        )}
        <div className="assistant-bubble-wrap">
          <BubbleActions messageId={m.id} heavyInlineNoExpand={bubbleHeavyInline} m={m} streaming={streaming} precedingUserContent={precedingUserContent} onRetranslate={onRetranslate} />
          <div className={`chat-bubble assistant${streaming ? ' chat-bubble--streaming' : ''}`}>
            {streaming && !destText ? (
              <div className="assistant-bubble-stream-placeholder" aria-busy="true" aria-label="Đang dịch">
                <span className="assistant-bubble-skel-line" />
                <span className="assistant-bubble-skel-line" />
              </div>
            ) : streaming && destText ? (
              <StreamingPlainDest text={destText} />
            ) : (
              <MessageMarkdown content={destText || ''} />
            )}
          </div>
          {!streaming && (
            <div className="chat-lang-label">
              {formatMessageFooterTime(m.updatedAt)}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function chatMessagePropsEqual(a: ChatMessageProps, b: ChatMessageProps): boolean {
  const mId = a.m.id
  // Chỉ re-render nếu CHÍNH tin này (hoặc cặp user của nó) đang/vừa liên quan streaming.
  // Không so sánh streamingAssistantId global — tránh toàn bộ list re-render khi stream bắt đầu/kết thúc.
  const prevStreaming =
    a.streamingAssistantId === mId ||
    (a.m.role === 'user' && a.nextAssistant?.id === a.streamingAssistantId)
  const nextStreaming =
    b.streamingAssistantId === mId ||
    (b.m.role === 'user' && b.nextAssistant?.id === b.streamingAssistantId)
  if (prevStreaming || nextStreaming) return false

  return (
    a.m === b.m &&
    a.precedingUserContent === b.precedingUserContent &&
    a.retranslateFollowUp === b.retranslateFollowUp &&
    a.retranslateQuoteAssistant === b.retranslateQuoteAssistant &&
    a.nextAssistant === b.nextAssistant &&
    a.highlight === b.highlight
  )
}

/** memo: App không re-render cả list khi state không liên quan (scroll nhẹ, v.v.) */
export const ChatMessage = memo(ChatMessageImpl, chatMessagePropsEqual)
