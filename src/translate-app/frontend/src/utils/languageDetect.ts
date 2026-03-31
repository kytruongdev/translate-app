/** Heuristic: Vietnamese diacritics → 'vi', else 'unknown' (architecture §5.6). */
export function detectLang(text: string): 'vi' | 'unknown' {
  if (/[àáâãèéêìíòóôõùúýăđơưạảấầẩẫậắằẳẵặẹẻẽếềểễệỉịọỏốồổỗộớờởỡợụủứừửữựỳỵỷỹ]/i.test(text)) {
    return 'vi'
  }
  return 'unknown'
}

export function titleFromContent(content: string, maxLen = 50): string {
  const s = content.trim()
  if (!s) return 'Phiên dịch'
  const firstLine = s.split('\n')[0]?.trim() ?? s
  let t = firstLine.startsWith('#') ? firstLine.replace(/^#+\s*/, '').trim() : firstLine
  if (t.length > maxLen) t = t.slice(0, maxLen) + '…'
  return t || 'Phiên dịch'
}
