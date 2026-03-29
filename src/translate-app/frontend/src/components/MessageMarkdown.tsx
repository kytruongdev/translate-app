import { memo, Children, type ReactNode, type ImgHTMLAttributes } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import type { Components } from 'react-markdown'

type Props = {
  content: string
  className?: string
  wrapSentences?: boolean
}

const remark = [remarkGfm]

const markdownComponents: Components = {
  img: ({ node: _n, src, alt, ...props }) => {
    const p = props as ImgHTMLAttributes<HTMLImageElement>
    return (
      <img
        {...p}
        src={src}
        alt={alt ?? ''}
        loading="lazy"
        decoding="async"
        className={['message-md-img', p.className].filter(Boolean).join(' ')}
      />
    )
  },
}

function splitSentences(text: string): string[] {
  // Split sau dấu câu kết thúc + whitespace; giữ dấu câu theo câu trước
  const parts = text.split(/(?<=[.!?。！？…]["'\u2019\u201d]?)\s+/)
  return parts.map((s) => s.trim()).filter(Boolean)
}

/** Custom <p> tách câu thành <span class="bl-sent" data-sent-idx="N"> */
const SentenceSplitP = memo(function SentenceSplitP({ children }: { children: ReactNode }) {
  const childArray = Children.toArray(children)
  const allStrings = childArray.every((c) => typeof c === 'string')

  // Không phải plain text (có inline element) → wrap toàn bộ làm 1 câu
  if (!allStrings) {
    return <p><span className="bl-sent" data-sent-idx="0">{children}</span></p>
  }

  const text = childArray.join('')
  const sentences = splitSentences(text)

  // Luôn wrap — kể cả 1 câu — để hover sync hoạt động
  return (
    <p>
      {sentences.map((s, i) => (
        <span key={i} className="bl-sent" data-sent-idx={String(i)}>
          {s}
        </span>
      ))}
    </p>
  )
})

const sentenceComponents: Components = {
  ...markdownComponents,
  p: ({ children }) => <SentenceSplitP>{children as ReactNode}</SentenceSplitP>,
}

/** Markdown + GFM (bảng, strikethrough, …) cho nội dung assistant / thẻ song ngữ. */
export const MessageMarkdown = memo(function MessageMarkdown({ content, className, wrapSentences }: Props) {
  if (!content) return null
  return (
    <div className={['message-md', className].filter(Boolean).join(' ')}>
      <ReactMarkdown
        remarkPlugins={remark}
        components={wrapSentences ? sentenceComponents : markdownComponents}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
})
