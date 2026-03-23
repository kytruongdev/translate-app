import { memo, type ImgHTMLAttributes } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import type { Components } from 'react-markdown'

type Props = {
  content: string
  className?: string
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

/** Markdown + GFM (bảng, strikethrough, …) cho nội dung assistant / thẻ song ngữ. */
export const MessageMarkdown = memo(function MessageMarkdown({ content, className }: Props) {
  if (!content) return null
  return (
    <div className={['message-md', className].filter(Boolean).join(' ')}>
      <ReactMarkdown remarkPlugins={remark} components={markdownComponents}>
        {content}
      </ReactMarkdown>
    </div>
  )
})
