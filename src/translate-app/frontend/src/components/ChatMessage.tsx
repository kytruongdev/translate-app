import {
  memo,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type MouseEvent,
} from 'react'
import type { Message, TranslationStyle } from '@/types/session'
import { WailsService } from '@/services/wailsService'
import { useSettingsStore } from '@/stores/settings/settingsStore'
import { useMessageStore, type MessageStore } from '@/stores/message/messageStore'
import {
  assistantMessageIsLongForm,
  formatMessageFooterTime,
  langShortLabel,
  styleLabel,
  userMessageShowPastedPlaceholder,
} from '@/utils/messageDisplay'
import { MessageMarkdown } from '@/components/MessageMarkdown'
import { CardExportPopover, CardRetranslatePopover } from '@/components/MessageCardPopovers'
import {
  TranslationFullscreenModal,
  type PanelMode,
  IconExport,
  IconRetranslate,
  IconCopy as IconCopyCard,
  IconFullscreen,
} from '@/components/TranslationFullscreenModal'

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
}: {
  m: Message
  destDisplay: string
  streaming: boolean
  precedingUserContent?: string
  /** Mockup: `.translation-card.retranslated` */
  isRetranslated?: boolean
  onRetranslate?: (p: RetranslatePayload) => Promise<void>
}) {
  const [mode, setMode] = useState<PanelMode>('bilingual')
  const [copied, setCopied] = useState(false)
  const [fullscreenOpen, setFullscreenOpen] = useState(false)
  const [exportOpen, setExportOpen] = useState(false)
  const [retranslateOpen, setRetranslateOpen] = useState(false)
  const [bodyExpanded, setBodyExpanded] = useState(false)

  const activeProvider = useSettingsStore((s) => s.activeProvider)
  const activeModel = useSettingsStore((s) => s.activeModel)
  const defaultStyle = useSettingsStore((s) => s.defaultStyle)

  const exportRef = useRef<HTMLButtonElement>(null)
  const retranslateRef = useRef<HTMLButtonElement>(null)

  /* Assistant thường không có originalContent — lấy từ tin user liền trước */
  const src = precedingUserContent?.trim() || m.originalContent || ''
  const dest = destDisplay
  const collapsible = useMemo(() => translationCardIsCollapsible(src, dest, streaming), [src, dest, streaming])
  const modelLabel = `${activeProvider} · ${activeModel}`

  useEffect(() => {
    setBodyExpanded(false)
  }, [m.id])

  useEffect(() => {
    if (!collapsible) setBodyExpanded(false)
  }, [collapsible])
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
            <div className="card-inline-title" />
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
              {!streaming && (
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
              )}
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
          data-expanded={collapsible ? (bodyExpanded ? 'true' : 'false') : 'true'}
        >
          <div className="translation-card-expandable-main">
            <div className="bilingual-view" data-mode={mode} id={`translation-card-body-${m.id}`}>
              <div className="translation-panel src">
                <div className="panel-head">{panelSrcHead}</div>
                <div className="panel-body">
                  <MemoMessageMarkdown content={src} />
                </div>
              </div>
              <div className="translation-panel dest">
                <div className="panel-head">{panelDestHead}</div>
                <div className="panel-body">
                  {streaming && !dest ? (
                    <div className="stream-skeleton" aria-busy="true" aria-label="Đang dịch">
                      <span className="stream-skeleton-line medium" />
                      <span className="stream-skeleton-line short" />
                      <span className="stream-skeleton-line medium" />
                    </div>
                  ) : streaming && dest ? (
                    <StreamingPlainDest text={dest} />
                  ) : (
                    <MessageMarkdown content={dest} />
                  )}
                </div>
              </div>
            </div>
          </div>
          {collapsible && (
            <div className="translation-card-expand-controls">
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
      />
    </>
  )
}

function BubbleActions({ messageId }: { messageId: string }) {
  const [flash, setFlash] = useState(false)
  return (
    <div className="assistant-bubble-actions">
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
    </div>
  )
}

export function ChatMessage({
  m,
  streamingAssistantId,
  nextAssistant,
  precedingUserContent,
  retranslateQuoteAssistant,
  retranslateFollowUp,
  onRetranslate,
}: {
  m: Message
  /** Chỉ tin đang stream subscribe buffer — tránh App re-render cả layout mỗi chunk. */
  streamingAssistantId: string | null
  /** Tin user: assistant ngay sau (nếu có), để tính “Văn bản đã dán” + buffer stream. */
  nextAssistant?: Message
  precedingUserContent?: string
  /** Bản dịch gốc (assistant) để quote + jump — `prev.originalMessageId`. */
  retranslateQuoteAssistant?: Message
  /** Tin assistant ngay sau user dịch lại (`originalMessageId`). */
  retranslateFollowUp?: boolean
  onRetranslate?: (p: RetranslatePayload) => Promise<void>
}) {
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
          <BubbleActions messageId={m.id} />
          <div className="chat-bubble assistant">
            {streaming && !destText ? (
              <div className="stream-skeleton stream-skeleton--compact" aria-busy="true" aria-label="Đang dịch">
                <span className="stream-skeleton-line" />
                <span className="stream-skeleton-line short" />
              </div>
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
