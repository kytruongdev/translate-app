/** Preview tin user đang gửi từ Start Page (trước khi `CreateSessionAndSend` xong). */

import { MessageMarkdown } from '@/components/MessageMarkdown'
import { LONG_TEXT_THRESHOLD } from '@/utils/messageDisplay'

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

export function StartOutgoingPreview({
  content,
  displayMode,
}: {
  content: string
  displayMode: 'bubble' | 'bilingual'
}) {
  const useCard = displayMode === 'bilingual' || content.length >= LONG_TEXT_THRESHOLD
  const meta = 'Đang tạo phiên…'

  if (useCard) {
    return (
      <div className="chat-msg user user-text-upload start-pending-chat-msg">
        <div className="avatar" aria-hidden>
          U
        </div>
        <div className="chat-msg-body">
          <div className="text-upload-bubble" role="status" aria-label="Đang gửi văn bản dài">
            <div className="preview-row">
              <IconPastedDoc />
              <span className="preview-title">Văn bản đã dán</span>
            </div>
          </div>
          <div className="chat-lang-label">{meta}</div>
        </div>
      </div>
    )
  }

  return (
    <div className="chat-msg user start-pending-chat-msg">
      <div className="avatar" aria-hidden>
        U
      </div>
      <div className="chat-msg-body">
        <div className="chat-bubble">
          <MessageMarkdown content={content} />
        </div>
        <div className="chat-lang-label">{meta}</div>
      </div>
    </div>
  )
}
