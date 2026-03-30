import { useEffect, useRef } from 'react'

/**
 * Hover sync cho bilingual view: hover block/sentence ở src → pair tương ứng ở dest highlight.
 * Dùng event delegation trên container ref.
 */
export function useBilingualHoverSync(
  containerRef: React.RefObject<HTMLElement | null>,
  src: string,
  dest: string,
  streaming: boolean,
  /** Extra trigger để re-attach listeners khi container mount muộn (vd: modal open state). */
  mountTrigger?: unknown,
) {
  const blockCacheRef = useRef<{ src: HTMLElement[]; dest: HTMLElement[] } | null>(null)
  useEffect(() => { blockCacheRef.current = null }, [src, dest, streaming])

  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    const getBlocks = () => {
      if (blockCacheRef.current) return blockCacheRef.current
      const getMd = (side: 'src' | 'dest') => {
        const md = container.querySelector<HTMLElement>(
          `.translation-panel--bilingual-body.${side} .message-md`
        )
        return md ? (Array.from(md.children) as HTMLElement[]) : []
      }
      blockCacheRef.current = { src: getMd('src'), dest: getMd('dest') }
      return blockCacheRef.current
    }

    const findBlock = (target: HTMLElement): { el: HTMLElement; side: 'src' | 'dest'; idx: number } | null => {
      let el: HTMLElement | null = target
      while (el && el !== container) {
        const parent: HTMLElement | null = el.parentElement
        if (parent?.classList.contains('message-md')) {
          const side: 'src' | 'dest' = parent.closest('.translation-panel--bilingual-body.src') ? 'src' : 'dest'
          const blocks = side === 'src' ? getBlocks().src : getBlocks().dest
          const idx = blocks.indexOf(el)
          if (idx !== -1) return { el, side, idx }
          return null
        }
        el = parent
      }
      return null
    }

    const PARA = 'bl-para-hover'
    const SENT = 'bl-sent-hover'

    let highlighted: Element[] = []
    const mark = (el: Element | null | undefined, cls: string) => {
      if (!el) return
      el.classList.add(cls)
      highlighted.push(el)
    }
    const clearAll = () => {
      highlighted.forEach((n) => n.classList.remove(PARA, SENT))
      highlighted = []
    }

    let activeKey: string | null = null

    const handleOver = (e: MouseEvent) => {
      const target = e.target as HTMLElement
      const { src: srcBlocks, dest: destBlocks } = getBlocks()

      const sentSpan = target.closest<HTMLElement>('.bl-sent')
      if (sentSpan) {
        const p = sentSpan.closest<HTMLElement>('.message-md > p')
        if (p) {
          const blockResult = findBlock(p)
          if (blockResult) {
            const { side, idx: pIdx } = blockResult
            const sentIdx = sentSpan.dataset.sentIdx ?? '0'
            const key = `${side}-${pIdx}-${sentIdx}`
            if (key === activeKey) return
            clearAll()
            activeKey = key
            const otherP = (side === 'src' ? destBlocks : srcBlocks)[pIdx]
            mark(p, PARA); mark(otherP, PARA)
            mark(sentSpan, SENT)
            mark(otherP?.querySelector<HTMLElement>(`.bl-sent[data-sent-idx="${sentIdx}"]`), SENT)
            return
          }
        }
      }

      const result = findBlock(target)
      if (!result) return

      const { el, side, idx } = result
      const paraKey = `${side}-${idx}`
      if (activeKey === paraKey || activeKey?.startsWith(`${paraKey}-`)) return
      clearAll()
      activeKey = paraKey
      mark(el, PARA)
      mark((side === 'src' ? destBlocks : srcBlocks)[idx], PARA)
    }

    const handleOut = (e: MouseEvent) => {
      const rel = e.relatedTarget as HTMLElement | null
      if (!rel || !container.contains(rel)) {
        clearAll()
        activeKey = null
      }
    }

    container.addEventListener('mouseover', handleOver)
    container.addEventListener('mouseout', handleOut)
    return () => {
      container.removeEventListener('mouseover', handleOver)
      container.removeEventListener('mouseout', handleOut)
    }
  }, [containerRef, mountTrigger]) // eslint-disable-line react-hooks/exhaustive-deps
}
