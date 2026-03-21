import { createPortal } from 'react-dom'
import { useEffect, useLayoutEffect, useState } from 'react'
import type { TranslationStyle } from '@/types/session'

type PopoverPos = { top: number; left: number; width: number; isAbove: boolean }

function computePopoverPosition(anchor: HTMLElement, estimatedHeight: number): PopoverPos {
  const r = anchor.getBoundingClientRect()
  const width = Math.min(340, window.innerWidth - 24)
  let left = r.left + r.width / 2 - width / 2
  left = Math.max(12, Math.min(left, window.innerWidth - width - 12))
  let top = r.bottom + 8
  let isAbove = false
  if (top + estimatedHeight > window.innerHeight - 12) {
    top = Math.max(12, r.top - estimatedHeight - 8)
    isAbove = true
  }
  return { top, left, width, isAbove }
}

export function CardExportPopover({
  open,
  anchorRef,
  onClose,
  onExport,
}: {
  open: boolean
  anchorRef: React.RefObject<HTMLButtonElement | null>
  onClose: () => void
  onExport: (format: 'pdf' | 'docx') => void
}) {
  const [fmt, setFmt] = useState<'pdf' | 'docx'>('pdf')
  const [pos, setPos] = useState<PopoverPos | null>(null)

  useLayoutEffect(() => {
    if (!open || !anchorRef.current) {
      setPos(null)
      return
    }
    setPos(computePopoverPosition(anchorRef.current, 220))
  }, [open, anchorRef])

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      const t = e.target as Node
      if (anchorRef.current?.contains(t)) return
      const root = document.querySelector('[data-card-export-popover="1"]')
      if (root?.contains(t)) return
      onClose()
    }
    const onReposition = () => {
      if (anchorRef.current) setPos(computePopoverPosition(anchorRef.current, 220))
    }
    document.addEventListener('mousedown', onDown)
    window.addEventListener('scroll', onReposition, true)
    window.addEventListener('resize', onReposition)
    return () => {
      document.removeEventListener('mousedown', onDown)
      window.removeEventListener('scroll', onReposition, true)
      window.removeEventListener('resize', onReposition)
    }
  }, [open, onClose, anchorRef])

  if (!open || !pos) return null

  return createPortal(
    <div
      data-card-export-popover="1"
      className={`retranslate-popover export-popover open${pos.isAbove ? ' is-above' : ''}`}
      style={{ position: 'fixed', top: pos.top, left: pos.left, width: pos.width, zIndex: 1100 }}
      role="dialog"
      aria-labelledby="export-popover-title"
    >
      <div className="popover-header">
        <div className="popover-title" id="export-popover-title">
          Export
        </div>
      </div>
      <div className="popover-body">
        <label className="settings-row compact" htmlFor="export-format-select">
          <span className="settings-row-label">Định dạng</span>
          <select
            id="export-format-select"
            className="retranslate-select"
            value={fmt}
            onChange={(e) => setFmt(e.target.value as 'pdf' | 'docx')}
            aria-label="Định dạng export"
          >
            <option value="pdf">PDF (.pdf)</option>
            <option value="docx">Word (.docx)</option>
          </select>
        </label>
      </div>
      <div className="dialog-actions">
        <button type="button" className="popover-btn cancel" onClick={onClose}>
          Hủy
        </button>
        <button
          type="button"
          className="popover-btn confirm"
          onClick={() => {
            onExport(fmt)
            onClose()
          }}
        >
          Export
        </button>
      </div>
    </div>,
    document.body,
  )
}

export function CardRetranslatePopover({
  open,
  anchorRef,
  onClose,
  initialStyle,
  modelLabel,
  onConfirm,
}: {
  open: boolean
  anchorRef: React.RefObject<HTMLButtonElement | null>
  onClose: () => void
  initialStyle: TranslationStyle
  modelLabel: string
  onConfirm: (style: TranslationStyle) => void
}) {
  const [style, setStyle] = useState<TranslationStyle>(initialStyle)
  const [pos, setPos] = useState<PopoverPos | null>(null)

  useLayoutEffect(() => {
    if (!open || !anchorRef.current) {
      setPos(null)
      return
    }
    setStyle(initialStyle)
    setPos(computePopoverPosition(anchorRef.current, 320))
  }, [open, initialStyle, anchorRef])

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      const t = e.target as Node
      if (anchorRef.current?.contains(t)) return
      const root = document.querySelector('[data-card-retranslate-popover="1"]')
      if (root?.contains(t)) return
      onClose()
    }
    const onReposition = () => {
      if (anchorRef.current) setPos(computePopoverPosition(anchorRef.current, 320))
    }
    document.addEventListener('mousedown', onDown)
    window.addEventListener('scroll', onReposition, true)
    window.addEventListener('resize', onReposition)
    return () => {
      document.removeEventListener('mousedown', onDown)
      window.removeEventListener('scroll', onReposition, true)
      window.removeEventListener('resize', onReposition)
    }
  }, [open, onClose, anchorRef])

  if (!open || !pos) return null

  const segBtn = (s: TranslationStyle, label: string) => (
    <button
      key={s}
      type="button"
      className={style === s ? 'active' : ''}
      onClick={() => setStyle(s)}
    >
      {label}
    </button>
  )

  return createPortal(
    <div
      data-card-retranslate-popover="1"
      className={`retranslate-popover open${pos.isAbove ? ' is-above' : ''}`}
      style={{ position: 'fixed', top: pos.top, left: pos.left, width: pos.width, zIndex: 1100 }}
      role="dialog"
      aria-labelledby="retranslate-popover-title"
    >
      <div className="popover-header">
        <div className="popover-title" id="retranslate-popover-title">
          Dịch lại
        </div>
      </div>
      <div className="popover-body">
        <p className="message-card-popover-model" style={{ fontSize: 12, color: 'var(--text-secondary)', margin: '0 0 10px' }}>
          Model: {modelLabel}
        </p>
        <div className="retranslate-style-block">
          <div>
            <div className="settings-row-label">Kiểu dịch</div>
            <div className="settings-row-desc">Casual, Business, Academic</div>
          </div>
          <div className="theme-segmented retranslate-style-segmented" role="group" aria-label="Kiểu dịch">
            {segBtn('casual', 'Casual')}
            {segBtn('business', 'Business')}
            {segBtn('academic', 'Academic')}
          </div>
        </div>
      </div>
      <div className="dialog-actions">
        <button type="button" className="popover-btn cancel" onClick={onClose}>
          Hủy
        </button>
        <button
          type="button"
          className="popover-btn confirm"
          onClick={() => {
            onConfirm(style)
            onClose()
          }}
        >
          Dịch lại
        </button>
      </div>
    </div>,
    document.body,
  )
}
