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
  styleLabel,
  userMessageShowPastedPlaceholder,
} from '@/utils/messageDisplay'
import { MessageMarkdown } from '@/components/MessageMarkdown'
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
import { LazyChunkedPlainText } from '@/components/LazyChunkedMarkdown'

const IconCopySmall = () => (
  <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" width={18} height={18} aria-hidden>
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth={2}
      d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
    />
  </svg>
)

const IconChevronDown = () => (
  <svg width={16} height={16} viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden>
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
  </svg>
)

const IconPastedDoc = () => (
  <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" width={20} height={20} aria-hidden>
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth={2}
      d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
    />
  </svg>
)

/** Paperclip — cùng kích thước / hàng với icon “Văn bản đã dán” (20×20, không khung). */
const IconUserAttachmentFile = () => (
  <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" width={20} height={20} aria-hidden>
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth={2}
      d="M21.44 11.05l-9.19 9.19a6 6 0 01-8.49-8.49l9.19-9.19a4 4 0 015.66 5.66l-9.2 9.19a2 2 0 01-2.83-2.83l8.49-8.48"
    />
  </svg>
)

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
function UserPastedLongTextBubble({ meta }: { meta: string }) {
  return (
    <>
      <div
        className="text-upload-bubble"
        role="status"
        aria-label="Đã gửi văn bản dài. Nội dung đầy đủ nằm ở cột Nguồn trong thẻ dịch phía dưới."
      >
        <div className="preview-row">
          <IconPastedDoc />
          <span className="preview-title">Văn bản đã dán</span>
        </div>
      </div>
      <div className="chat-lang-label">{meta}</div>
    </>
  )
}

function UserFileAttachmentBubble({ fileName, meta }: { fileName: string; meta: string }) {
  return (
    <>
      <div
        className="text-upload-bubble"
        role="status"
        aria-label={`Đã gửi tệp ${fileName}`}
      >
        <div className="preview-row">
          <IconUserAttachmentFile />
          <span className="preview-title">{fileName}</span>
        </div>
      </div>
      <div className="chat-lang-label">{meta}</div>
    </>
  )
}

export type RetranslatePayload = {
  sourceContent: string
  assistantMessageId: string
  displayMode: 'bubble' | 'bilingual'
  sourceLang: string
  targetLang: string
  style: TranslationStyle
}

/** Heuristic: đủ dài để mặc định thu gọn + nút Mở rộng (tránh kéo thẻ quá cao trong feed). */
function translationCardIsCollapsible(src: string, dest: string, streaming: boolean): boolean {
  const s = src.length
  const d = dest.length
  if (streaming && (s > 180 || d > 60)) return true
  return s + d > 520 || Math.max(s, d) > 360
}

/** Nguồn trong thẻ: không parse lại Markdown mỗi chunk stream (src ổn định trong lượt dịch). */
const MemoMessageMarkdown = memo(function MemoMessageMarkdown({ content }: { content: string }) {
  return <MessageMarkdown content={content} />
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
      ) : (
        <MemoMessageMarkdown content={src} />
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
      ) : (
        <MessageMarkdown content={dest} />
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

  const canRetranslate = Boolean(precedingUserContent?.trim() && onRetranslate)

  const panelSrcHead = `Nguồn · ${langShortLabel(m.sourceLang || 'auto')}`
  const panelDestHead = `Bản dịch · ${langShortLabel(m.targetLang || 'en-US')}`
  const footerOutside = `${formatMessageFooterTime(m.updatedAt)} · ${styleLabel(m.style)}`

  const handleExport = async (format: 'pdf' | 'docx') => {
    try {
      await WailsService.exportMessage(m.id, format)
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
    if (!precedingUserContent?.trim() || !onRetranslate) return
    await onRetranslate({
      sourceContent: precedingUserContent.trim(),
      assistantMessageId: m.id,
      displayMode: m.displayMode,
      sourceLang: m.sourceLang,
      targetLang: m.targetLang,
      style,
    })
  }

  return (
    <>
      <div
        className={`translation-card${streaming ? ' is-streaming' : ''}${isRetranslated ? ' retranslated' : ''}`}
      >
        <div className="card-topbar">
          <div className="card-topbar-left">
            <div className="card-inline-title" title={bilingualFileTitle ?? undefined}>
              {bilingualFileTitle ?? ''}
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
                    <IconChevronDown />
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
        onExport={(f) => void handleExport(f)}
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
        onExport={(f) => void handleExport(f)}
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

function BubbleActions({
  messageId,
  heavyInlineNoExpand,
}: {
  messageId: string
  heavyInlineNoExpand: boolean
}) {
  const [flash, setFlash] = useState(false)
  return (
    <div className="assistant-bubble-actions">
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
            <IconCopySmall />
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
          {flash ? '✓' : <IconCopySmall />}
        </button>
      )}
    </div>
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

  const pairedAssistantLongForm = useMemo(() => {
    if (m.role !== 'user' || nextAssistant?.role !== 'assistant') return false
    return assistantMessageIsLongForm(nextAssistant, {
      streamingAssistantId,
      streamingText: streamBuf,
    })
  }, [m.role, nextAssistant, streamingAssistantId, streamBuf])

  const userLang = langShortLabel(m.sourceLang || 'auto')
  const userMeta = `${formatMessageFooterTime(m.createdAt)} · ${userLang}`

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
            U
          </div>
          <div className="chat-msg-body">
            <UserFileAttachmentBubble fileName={fileAttachName} meta={userMeta} />
          </div>
        </div>
      )
    }
    if (userMessageShowPastedPlaceholder(m, pairedAssistantLongForm)) {
      return (
        <div className="chat-msg user user-text-upload" id={`chat-msg-${m.id}`}>
          <div className="avatar" aria-hidden>
            U
          </div>
          <div className="chat-msg-body">
            <UserPastedLongTextBubble meta={userMeta} />
          </div>
        </div>
      )
    }
    return (
      <div className="chat-msg user" id={`chat-msg-${m.id}`}>
        <div className="avatar" aria-hidden>
          U
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

  if (m.displayMode === 'bilingual') {
    return (
      <div
        className={`chat-msg assistant${retranslateFollowUp ? ' retranslate-reply' : ''}`}
        id={`chat-msg-${m.id}`}
      >
        <div className="avatar assistant-avatar" aria-hidden>
          ✦
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
              {retranslateFollowUp ? ' · Bản dịch lại' : ''} · {styleLabel(m.style)}
            </div>
          )}
        </div>
      </div>
    )
  }

  return (
    <div className="chat-msg assistant" id={`chat-msg-${m.id}`}>
      <div className="avatar assistant-avatar" aria-hidden>
        ✦
      </div>
      <div className="chat-msg-body">
        <div className="assistant-bubble-wrap">
          <BubbleActions messageId={m.id} heavyInlineNoExpand={bubbleHeavyInline} />
          <div className={`chat-bubble assistant${streaming ? ' chat-bubble--streaming' : ''}`}>
            {streaming && !destText ? (
              <div className="assistant-bubble-stream-placeholder" aria-busy="true" aria-label="Đang dịch" />
            ) : streaming && destText ? (
              <StreamingPlainDest text={destText} />
            ) : (
              <MessageMarkdown content={destText || ''} />
            )}
          </div>
          {!streaming && (
            <div className="chat-lang-label">
              {formatMessageFooterTime(m.updatedAt)} · {styleLabel(m.style)}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function chatMessagePropsEqual(a: ChatMessageProps, b: ChatMessageProps): boolean {
  return (
    a.m === b.m &&
    a.streamingAssistantId === b.streamingAssistantId &&
    a.fileTranslateProgress === b.fileTranslateProgress &&
    a.nextAssistant === b.nextAssistant &&
    a.precedingUserContent === b.precedingUserContent &&
    a.retranslateQuoteAssistant === b.retranslateQuoteAssistant &&
    a.retranslateFollowUp === b.retranslateFollowUp &&
    a.onRetranslate === b.onRetranslate
  )
}

/** memo: App không re-render cả list khi state không liên quan (scroll nhẹ, v.v.) */
export const ChatMessage = memo(ChatMessageImpl, chatMessagePropsEqual)
