/** Hai skeleton line trong bubble — không phải nội dung thật */
function LoadingBubbleSkeleton({ variant }: { variant: 'user' | 'assistant' }) {
  return (
    <span className={`session-loading-skel-wrap session-loading-skel-wrap--${variant}`} aria-hidden>
      <span className="session-loading-skel-line" />
      <span className="session-loading-skel-line session-loading-skel-line--second" />
    </span>
  )
}

export function ChatSessionLoadingPreview() {
  return (
    <div className="session-loading-thread" aria-busy="true" aria-live="polite" aria-label="Đang tải hội thoại">
      <div className="chat-msg user chat-msg--session-loading-preview">
        <div className="avatar" aria-hidden>
          U
        </div>
        <div className="chat-msg-body">
          <div className="chat-bubble session-loading-bubble">
            <LoadingBubbleSkeleton variant="user" />
          </div>
        </div>
      </div>

      <div className="chat-msg assistant chat-msg--session-loading-preview">
        <div className="avatar assistant-avatar" aria-hidden>
          ✦
        </div>
        <div className="chat-msg-body">
          <div className="chat-bubble session-loading-bubble">
            <LoadingBubbleSkeleton variant="assistant" />
          </div>
        </div>
      </div>

      <div className="chat-msg user chat-msg--session-loading-preview">
        <div className="avatar" aria-hidden>
          U
        </div>
        <div className="chat-msg-body">
          <div className="chat-bubble session-loading-bubble">
            <LoadingBubbleSkeleton variant="user" />
          </div>
        </div>
      </div>

      <div className="chat-msg assistant chat-msg--session-loading-preview">
        <div className="avatar assistant-avatar" aria-hidden>
          ✦
        </div>
        <div className="chat-msg-body">
          <div className="chat-bubble session-loading-bubble">
            <LoadingBubbleSkeleton variant="assistant" />
          </div>
        </div>
      </div>
    </div>
  )
}
