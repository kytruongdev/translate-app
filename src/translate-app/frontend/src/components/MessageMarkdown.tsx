import { memo } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

type Props = {
  content: string
  className?: string
}

const remark = [remarkGfm]

/** Markdown + GFM (bảng, strikethrough, …) cho nội dung assistant / thẻ song ngữ. */
export const MessageMarkdown = memo(function MessageMarkdown({ content, className }: Props) {
  if (!content) return null
  return (
    <div className={['message-md', className].filter(Boolean).join(' ')}>
      <ReactMarkdown remarkPlugins={remark}>{content}</ReactMarkdown>
    </div>
  )
})
