import { memo, useEffect, useMemo, useRef, useState, type RefObject } from 'react'
import { MessageMarkdown } from '@/components/MessageMarkdown'

/** Dưới ngưỡng: một lần parse như cũ (đủ nhẹ). */
export const FULLSCREEN_LAZY_MD_CHAR_THRESHOLD = 12_000

const CHUNK_TARGET_CHARS = 4_500
const CHUNK_HARD_MAX = 9_000
const INITIAL_VISIBLE_CHUNKS = 3
const LOAD_MORE_CHUNKS = 3
const SCROLL_ROOT_MARGIN = '720px 0px'

function splitOversized(text: string, max: number): string[] {
  const t = text.trim()
  if (t.length <= max) return t ? [t] : []
  const parts: string[] = []
  let start = 0
  while (start < t.length) {
    let end = Math.min(start + max, t.length)
    if (end < t.length) {
      const segment = t.slice(start, end)
      const a = segment.lastIndexOf('\n\n')
      const b = segment.lastIndexOf('\n')
      const c = segment.lastIndexOf(' ')
      const breakAt = Math.max(a, b, c)
      if (breakAt > max * 0.35) {
        end = start + breakAt + 1
      }
    }
    const slice = t.slice(start, end).trim()
    if (slice) parts.push(slice)
    start = end
  }
  return parts
}

/** Gom đoạn (\n\n) rồi cắt chunk quá dài — tránh một paragraph khổng lồ. */
function splitMarkdownIntoChunks(text: string, targetMax: number): string[] {
  const trimmed = text.trim()
  if (trimmed === '') return []
  if (trimmed.length <= targetMax) return [trimmed]

  const paras = trimmed.split(/\n\n+/)
  const merged: string[] = []
  let buf = ''
  for (const p of paras) {
    const piece = buf === '' ? p : `${buf}\n\n${p}`
    if (piece.length > targetMax && buf !== '') {
      merged.push(buf)
      buf = p
    } else {
      buf = piece
    }
  }
  if (buf) merged.push(buf)

  const out: string[] = []
  for (const ch of merged) {
    if (ch.length <= CHUNK_HARD_MAX) {
      out.push(ch)
    } else {
      out.push(...splitOversized(ch, CHUNK_HARD_MAX))
    }
  }
  return out.length ? out : [trimmed]
}

type Props = {
  content: string
  /** Phần tử có overflow-y: auto (panel-body fullscreen). */
  scrollRootRef: RefObject<HTMLElement | null>
  className?: string
}

/**
 * Fullscreen / văn bản rất dài: không mount toàn bộ react-markdown một lần.
 * Nạp thêm chunk khi sentinel vào vùng gần viewport (cuộn).
 */
export const LazyChunkedMarkdown = memo(function LazyChunkedMarkdown({
  content,
  scrollRootRef,
  className,
}: Props) {
  const chunks = useMemo(
    () => splitMarkdownIntoChunks(content, CHUNK_TARGET_CHARS),
    [content],
  )

  const [visibleCount, setVisibleCount] = useState(() =>
    Math.min(INITIAL_VISIBLE_CHUNKS, Math.max(1, chunks.length)),
  )
  const sentinelRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    setVisibleCount(Math.min(INITIAL_VISIBLE_CHUNKS, Math.max(1, chunks.length)))
  }, [content, chunks.length])

  useEffect(() => {
    if (chunks.length <= 1 || visibleCount >= chunks.length) return
    const root = scrollRootRef.current
    const sentinel = sentinelRef.current
    if (!root || !sentinel) return

    const io = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          setVisibleCount((n) => Math.min(n + LOAD_MORE_CHUNKS, chunks.length))
        }
      },
      { root, rootMargin: SCROLL_ROOT_MARGIN, threshold: 0 },
    )
    io.observe(sentinel)
    return () => io.disconnect()
  }, [scrollRootRef, chunks.length, visibleCount])

  if (!content) return null

  if (content.length < FULLSCREEN_LAZY_MD_CHAR_THRESHOLD) {
    return <MessageMarkdown content={content} className={className} />
  }

  const slice = chunks.slice(0, visibleCount)

  return (
    <div className="lazy-md-stack">
      {slice.map((chunk, i) => (
        <div key={i} className="lazy-md-chunk">
          <MessageMarkdown content={chunk} className={className} />
        </div>
      ))}
      {visibleCount < chunks.length ? (
        <div ref={sentinelRef} className="lazy-md-sentinel" aria-hidden />
      ) : null}
    </div>
  )
})

/**
 * Giống LazyChunkedMarkdown nhưng không parse markdown — dùng khi stream nguồn file lớn (tránh một DOM text khổng lồ).
 * Chỉ tăng / giảm `visibleCount` khi số chunk đổi, không reset mỗi lần `content` dài thêm trong cùng chunk.
 */
export const LazyChunkedPlainText = memo(function LazyChunkedPlainText({
  content,
  scrollRootRef,
  className = 'message-md panel-body-text stream-src-plain',
}: {
  content: string
  scrollRootRef: RefObject<HTMLElement | null>
  className?: string
}) {
  const chunks = useMemo(
    () => splitMarkdownIntoChunks(content, CHUNK_TARGET_CHARS),
    [content],
  )

  const [visibleCount, setVisibleCount] = useState(() =>
    chunks.length === 0 ? 0 : Math.min(INITIAL_VISIBLE_CHUNKS, chunks.length),
  )
  const sentinelRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    setVisibleCount((prev) => {
      if (chunks.length === 0) return 0
      if (prev === 0) return Math.min(INITIAL_VISIBLE_CHUNKS, chunks.length)
      if (prev > chunks.length) return chunks.length
      return Math.min(Math.max(prev, INITIAL_VISIBLE_CHUNKS), chunks.length)
    })
  }, [chunks.length])

  useEffect(() => {
    if (chunks.length <= 1 || visibleCount >= chunks.length) return
    const root = scrollRootRef.current
    const sentinel = sentinelRef.current
    if (!root || !sentinel) return

    const io = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          setVisibleCount((n) => Math.min(n + LOAD_MORE_CHUNKS, chunks.length))
        }
      },
      { root, rootMargin: SCROLL_ROOT_MARGIN, threshold: 0 },
    )
    io.observe(sentinel)
    return () => io.disconnect()
  }, [scrollRootRef, chunks.length, visibleCount])

  if (!content) return null

  if (content.length < FULLSCREEN_LAZY_MD_CHAR_THRESHOLD) {
    return <div className={className}>{content}</div>
  }

  const slice = chunks.slice(0, visibleCount)

  return (
    <div className="lazy-md-stack">
      {slice.map((chunk, i) => (
        <div key={i} className="lazy-md-chunk">
          <div className={className}>{chunk}</div>
        </div>
      ))}
      {visibleCount < chunks.length ? (
        <div ref={sentinelRef} className="lazy-md-sentinel" aria-hidden />
      ) : null}
    </div>
  )
})
