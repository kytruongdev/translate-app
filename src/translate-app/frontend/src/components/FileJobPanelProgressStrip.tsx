/**
 * Tiến độ dịch file — thanh mỏng ngay dưới tiêu đề cột Nguồn / Bản dịch (không banner xám).
 */

function fileJobEdgeTitle(
  indeterminate: boolean,
  percent: number,
  chunk?: number,
  total?: number,
): string {
  if (indeterminate) return 'Đang dịch tệp…'
  const c = chunk != null && total != null && total > 0 ? ` · Phần ${chunk}/${total}` : ''
  return `${percent}%${c}`
}

/** Thanh tiến độ full-width giữa header song ngữ và nội dung; gradient chạy + đốm accent tĩnh (một lớp animate). */
export function FileJobBilingualEdgeRail({
  active,
  indeterminate,
  percent,
  chunk,
  total,
}: {
  active: boolean
  indeterminate: boolean
  percent: number
  chunk?: number
  total?: number
}) {
  if (!active) {
    return <div className="bilingual-head-separator" aria-hidden />
  }

  const showChunk = !indeterminate && chunk != null && total != null && total > 0
  const ariaLabel =
    indeterminate
      ? 'Đang dịch tệp'
      : showChunk
        ? `Đã dịch ${percent}%, phần ${chunk} trên ${total}`
        : `Đã dịch ${percent}%`

  const pctDisplay = indeterminate
    ? '—'
    : `${Math.round(Math.min(100, Math.max(0, percent)))}%`

  return (
    <div
      className="bilingual-file-edge-rail"
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={indeterminate ? undefined : percent}
      aria-label={ariaLabel}
      title={fileJobEdgeTitle(indeterminate, percent, chunk, total)}
    >
      <div className="bilingual-file-edge-track-shell">
        <div className="bilingual-file-edge-track">
          <div
            className={
              indeterminate
                ? 'bilingual-file-edge-fill bilingual-file-edge-fill--indeterminate'
                : 'bilingual-file-edge-fill'
            }
            style={indeterminate ? undefined : { width: `${Math.min(100, Math.max(0, percent))}%` }}
          />
        </div>
      </div>
      <span className="bilingual-file-edge-pct" aria-hidden="true">
        {pctDisplay}
      </span>
    </div>
  )
}

/** Skeleton phần bản dịch còn lại (file job đang chạy, chưa 100%). */
export function fileJobShowPartialDestTail(
  fileJobActive: boolean,
  streaming: boolean,
  dest: string,
  fileJobProgress: { total: number; percent: number } | null | undefined,
): boolean {
  if (!fileJobActive || !streaming || !dest.trim()) return false
  if (!fileJobProgress || fileJobProgress.total < 1) return true
  return fileJobProgress.percent < 100
}

export function fileJobDestTailMinPx(fileJobProgress: { total: number; percent: number } | null | undefined): number {
  if (!fileJobProgress || fileJobProgress.total < 1) return 220
  const left = Math.max(0, 100 - fileJobProgress.percent)
  return Math.min(520, Math.max(80, (left / 100) * 480))
}
