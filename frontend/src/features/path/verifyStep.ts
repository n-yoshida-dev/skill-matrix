import type { Level } from '../../data/types'
import { LEVEL_NAMES } from '../public/summary'

// 「次の確認方法」（roadmap.json の verifyBy）の見せ方を作る純粋関数。DOM を触らない。
// 「次」が何の次か分からない（2026-11-08 本人）ので、上がる段を段の名前と数字で先に言う。
// verifyBy は末尾に「（L1）」「（L1・L2）」のように、その確認で付く段を書く決まりで、そこから段を取り出す。

/** verifyBy の見出し（「〜になるには」）と、末尾の段の表示を外した本文 */
export type VerifyStep = { lead: string; text: string }

/** 末尾の「（L1）」「（L1・L2）」。全角の括弧と中黒 */
const TRAILING_LEVELS = /（(L[0-5](?:・L[0-5])*)）\s*$/

/** 段の表示が読めないときの見出し（データの書き間違いでも画面は描く） */
const FALLBACK_LEAD = '次のレベルに上がるには'

/**
 * verifyBy から見出しと本文を作る。
 * 例：「確認問題に答える（L1）」→ 見出し「「基礎を確認した」（L1）になるには」、本文「確認問題に答える」。
 * 段が 2 つ書いてあれば（「（L1・L2）」）、上の段で言う（その確認で上の段まで届くため）。
 */
export function verifyStep(verifyBy: string): VerifyStep {
  const m = TRAILING_LEVELS.exec(verifyBy)
  if (!m) return { lead: FALLBACK_LEAD, text: verifyBy }
  const levels = m[1].split('・').map((s) => Number(s.slice(1)) as Level)
  const top = Math.max(...levels) as Level
  return {
    lead: `「${LEVEL_NAMES[top]}」（L${top}）になるには`,
    text: verifyBy.slice(0, m.index).trimEnd(),
  }
}
