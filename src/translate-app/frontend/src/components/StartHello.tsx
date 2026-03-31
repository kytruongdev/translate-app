import { useEffect, useMemo, useState } from 'react'

/** Một space giữa các từ — hai space + --ws padding từng span làm khoảng trắng phình gấp đôi */
const PHRASES = ['Hi there', 'Xin chào'] as const

/** Lệch pha gradient giữa hai chữ liền kề → sóng màu (animation-delay âm, giây) */
const GLYPH_GRADIENT_STAGGER_S = 0.35

const TYPE_MS = 170
const TYPE_FIRST_DELAY_MS = 20
const PAUSE_AFTER_TYPE_MS = 1600
const PAUSE_AFTER_DELETE_MS = 500
const EXIT_DUR_MS = 120

type CharAnim = '' | 'enter' | 'exit'

type HelloDebug =
  | { mode: 'animate' }
  | { mode: 'static'; phraseIndex: 0 | 1 }

/**
 * Tạm dừng animation để chỉnh tay CSS trong DevTools:
 * - Thêm query: `?startHelloStatic=1` (optional `&startHelloPhrase=1` = cụm "Xin  chào")
 * - Hoặc: `localStorage.setItem('startHelloStatic', '1')` rồi reload — tắt: `removeItem('startHelloStatic')`
 */
function readStartHelloDebug(): HelloDebug {
  if (typeof window === 'undefined') return { mode: 'animate' }
  try {
    const q = new URLSearchParams(window.location.search)
    const fromQuery = q.get('startHelloStatic') === '1'
    const fromStorage = localStorage.getItem('startHelloStatic') === '1'
    if (fromQuery || fromStorage) {
      const p = parseInt(q.get('startHelloPhrase') || '0', 10)
      const phraseIndex: 0 | 1 = p === 1 ? 1 : 0
      return { mode: 'static', phraseIndex }
    }
  } catch {
    /* private / storage blocked */
  }
  return { mode: 'animate' }
}

/**
 * Port trực tiếp logic IIFE trong mockup (setTimeout + phase),
 * dùng async/await + state thay DOM.
 */
export function StartHello() {
  const debug = useMemo(() => readStartHelloDebug(), [])

  const [chars, setChars] = useState<{ ch: string; anim: CharAnim }[]>(() =>
    debug.mode === 'static'
      ? PHRASES[debug.phraseIndex].split('').map((ch) => ({ ch, anim: 'enter' as const }))
      : []
  )

  useEffect(() => {
    if (debug.mode === 'static') return

    const timers: number[] = []
    const sleep = (ms: number) =>
      new Promise<void>((resolve) => {
        timers.push(window.setTimeout(resolve, ms))
      })

    let cancelled = false

    async function typeWord(word: string) {
      const initial = word.split('').map((ch) => ({ ch, anim: '' as const }))
      setChars(initial)
      for (let k = 0; k < word.length; k++) {
        await sleep(k === 0 ? TYPE_FIRST_DELAY_MS : TYPE_MS)
        if (cancelled) return
        const ki = k
        setChars((prev) => {
          if (prev.length !== word.length) return prev
          const next = [...prev]
          if (next[ki]) next[ki] = { ...next[ki], anim: 'enter' }
          return next
        })
      }
      await sleep(TYPE_MS)
    }

    async function deleteWord(wordLen: number) {
      for (let i = wordLen - 1; i >= 0; i--) {
        if (cancelled) return
        const idx = i
        setChars((prev) => {
          if (prev.length !== wordLen) return prev
          const next = [...prev]
          if (next[idx]) next[idx] = { ...next[idx], anim: 'exit' }
          return next
        })
        await sleep(EXIT_DUR_MS)
      }
      if (!cancelled) setChars([])
    }

    void (async () => {
      while (!cancelled) {
        await typeWord(PHRASES[0])
        if (cancelled) break
        await sleep(PAUSE_AFTER_TYPE_MS)
        if (cancelled) break
        await deleteWord(PHRASES[0].length)
        if (cancelled) break
        await sleep(PAUSE_AFTER_DELETE_MS)
        if (cancelled) break
        await typeWord(PHRASES[1])
        if (cancelled) break
        await sleep(PAUSE_AFTER_TYPE_MS)
        if (cancelled) break
        await deleteWord(PHRASES[1].length)
        if (cancelled) break
        await sleep(PAUSE_AFTER_DELETE_MS)
      }
    })()

    return () => {
      cancelled = true
      timers.forEach((t) => clearTimeout(t))
    }
  }, [debug.mode])

  const ariaLabel = 'Hi there / Xin chào'

  return (
    <h1 className="start-hello" aria-label={ariaLabel}>
      {chars.map((c, i) => {
        const isSpace = c.ch === ' '
        return (
          <span
            key={i}
            className={['start-hello-char', isSpace && 'start-hello-char--ws', c.anim]
              .filter(Boolean)
              .join(' ')}
            style={{ animationDelay: `${-(i * GLYPH_GRADIENT_STAGGER_S)}s` }}
          >
            {c.ch}
          </span>
        )
      })}
    </h1>
  )
}
