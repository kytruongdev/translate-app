/** Session title from attachment file name (strip extension). */
export function titleFromFileName(name: string): string {
  const t = name.replace(/\.[^.]+$/u, '').trim()
  return t || name.trim() || 'Phiên dịch'
}
