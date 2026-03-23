import { useMessageStore } from '@/stores/message/messageStore'

/**
 * Gộp `translation:chunk` trước khi đẩy vào Zustand — tránh mỗi token một lần re-render
 * và O(n) nối chuỗi khổng lồ 60 lần/giây (file lớn → treo UI).
 */
const pending = { buf: '' }
let raf = 0
let timeout = 0
/** `true` sau `file:progress` đầu tiên cho tới done/error/start mới. */
let fileJobActive = false
let lastApplyAt = 0

/** Bật throttle chậm khi đang dịch file và buffer đã đủ dài. */
const FILE_HEAVY_CHAR_THRESHOLD = 8_000
const FILE_HEAVY_MIN_INTERVAL_MS = 150

function cancelTimers() {
  if (raf) cancelAnimationFrame(raf)
  if (timeout) clearTimeout(timeout)
  raf = 0
  timeout = 0
}

function applyPendingToStore() {
  const add = pending.buf
  pending.buf = ''
  if (!add) return
  lastApplyAt = performance.now()
  useMessageStore.setState((s) => ({ streamingText: s.streamingText + add }))
}

function scheduleFlush() {
  if (raf || timeout) return

  const state = useMessageStore.getState()
  const combinedLen = state.streamingText.length + pending.buf.length
  const useSlowThrottle = fileJobActive && combinedLen >= FILE_HEAVY_CHAR_THRESHOLD

  const run = () => {
    raf = 0
    timeout = 0
    applyPendingToStore()
    if (pending.buf) scheduleFlush()
  }

  if (!useSlowThrottle) {
    raf = requestAnimationFrame(run)
    return
  }

  const elapsed = performance.now() - lastApplyAt
  const wait = Math.max(0, FILE_HEAVY_MIN_INTERVAL_MS - elapsed)
  if (wait <= 2) {
    raf = requestAnimationFrame(run)
  } else {
    timeout = window.setTimeout(run, wait)
  }
}

export function setStreamingFileJobActive(active: boolean) {
  fileJobActive = active
}

/** Xóa timer + buffer (không đẩy vào store). Dùng khi start stream mới. */
export function resetStreamingTextCoalescer() {
  cancelTimers()
  pending.buf = ''
  lastApplyAt = 0
}

/** Đẩy hết buffer vào store ngay (trước translation:done / error). */
export function flushStreamingTextCoalescerSync() {
  cancelTimers()
  applyPendingToStore()
}

export function enqueueStreamingTextChunk(chunk: string) {
  if (!chunk) return
  pending.buf += chunk
  scheduleFlush()
}
