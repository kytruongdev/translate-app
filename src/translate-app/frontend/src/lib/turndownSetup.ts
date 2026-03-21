import TurndownService from 'turndown'
import { gfm } from 'turndown-plugin-gfm'

/** Configured once for clipboard HTML → Markdown (E4). */
export function createTurndown(): TurndownService {
  const td = new TurndownService()
  td.use(gfm)
  return td
}
