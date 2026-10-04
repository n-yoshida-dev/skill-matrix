import type { ItemState, RoadmapItem } from '../../data/types'
import type { TreeEdge } from './layout'

// スキルツリーで解放の条件を読ませるための純粋関数（SPEC.md §7.2 の線の色・合流点・前提待ち）。DOM を import しない。
// 線の色・合流点の「n つとも必要 x / n」・鍵の項目の「必要：」は、どれも「前提が確認済み（verifiedLevel 1 以上）か」という
// 1 つの判定から出す。判定が 1 か所なので、合流点の x と色の付いた線の本数がずれない（KNOWLEDGE.md 2026-10-03）。
// この判定は、着手できるか（plan/next.ts の isReady）と同じ。

/** 確認済み（verifiedLevel 1 以上）か。理解度の記録が無い項目は確認済みでない */
export function isVerified(states: Map<string, ItemState>, key: string): boolean {
  return (states.get(key)?.verifiedLevel ?? 0) >= 1
}

/** 線に色を付けるか。前提（線の出どころ）が確認済みなら付ける。行き先の状態は見ない（2026-10-03 本人了承の 2） */
export function isLit(edge: Pick<TreeEdge, 'from'>, states: Map<string, ItemState>): boolean {
  return isVerified(states, edge.from)
}

/** 合流点の数：n は項目に入る線（前提）の本数、x はそのうち色の付いた線の本数 */
export interface JunctionCount {
  n: number
  x: number
}

/** 項目 to に入る線から、合流点の数を数える */
export function junctionCount(
  to: string,
  edges: Pick<TreeEdge, 'from' | 'to'>[],
  states: Map<string, ItemState>,
): JunctionCount {
  const ins = edges.filter((e) => e.to === to)
  return { n: ins.length, x: ins.filter((e) => isLit(e, states)).length }
}

/** 合流点の文言（SPEC.md §7.2。例「2 つとも必要 0/2」） */
export function junctionLabel({ n, x }: JunctionCount): string {
  return `${n} つとも必要 ${x}/${n}`
}

/** 合流点から項目へ下りる線に色を付けるか。前提がすべて確認済み（x = n）のときだけ */
export function isJunctionOpen({ n, x }: JunctionCount): boolean {
  return n > 0 && x === n
}

/**
 * 鍵の項目に「必要：」として出す、確認済みでない直接の前提の項目キー（dependsOn の順、重複なし）。
 * ロードマップに無い前提も確認済みではないので含める（着手できない理由がそこにあるため。CLI の verify が弾く決まり）
 */
export function missingPrereqs(item: RoadmapItem, states: Map<string, ItemState>): string[] {
  return [...new Set(item.dependsOn ?? [])].filter((dep) => !isVerified(states, dep))
}
