import type { ItemState, RoadmapDomain, RoadmapItem } from '../../data/types'
import { isDone, isReady, stateOf } from '../plan/next'

// 学習パス（SPEC.md §7.2）のための純粋関数。DOM・ファイル読み込みを import しない。
// 「今日」に依存しないので、同じ roadmap.json と state.json からいつでも同じ並びになる（SPEC.md §5）。
// Go の参照実装は backend/internal/domain/plan.go の BuildPath / topoSort。
// 同じ入力に同じ答えを返すことを path.test.ts が Go の plan_test.go の例を写して確かめる。

/** 学習パス上の位置。済（レベル 1 以上）／今ここ／この先 */
export type PathStatus = 'done' | 'current' | 'upcoming'

/** 学習パスの 1 行 */
export interface PathNode {
  item: RoadmapItem
  state: ItemState
  status: PathStatus
  /** 前提がすべてレベル 1 以上で、着手できる */
  ready: boolean
  /** 分野の外にある前提の項目キー（一列表示では注記、ツリー表示ではゴーストノードにする） */
  externalDeps: string[]
}

/** 分野 1 つの学習パス */
export interface Path {
  domainKey: string
  domainName: string
  goal?: string
  nodes: PathNode[]
  /** 済（レベル 1 以上）の項目数 */
  doneCount: number
  totalCount: number
}

/**
 * 依存関係の順に項目を並べる（トポロジカルソート。Go の topoSort）。
 * - 同じ順位のものは定義順を保つ
 * - 分野の外を指す依存は順序付けに使えないので無視する
 * - 循環参照が残っていたら、残りを定義順で末尾に付ける（無限ループを避ける。循環は verify が弾く決まり）
 */
export function topoSort(items: RoadmapItem[]): RoadmapItem[] {
  const index = new Map(items.map((it, i) => [it.key, i]))
  const inDegree = items.map(() => 0)
  const children: number[][] = items.map(() => [])
  items.forEach((it, i) => {
    for (const dep of it.dependsOn ?? []) {
      const j = index.get(dep)
      if (j === undefined) continue // 分野外への依存
      inDegree[i]++
      children[j].push(i)
    }
  })

  const ready = items.flatMap((_, i) => (inDegree[i] === 0 ? [i] : []))
  const out: RoadmapItem[] = []
  const placed = items.map(() => false)
  while (ready.length > 0) {
    // 定義順で最も早いものから取り出して並びを安定させる
    ready.sort((a, b) => a - b)
    const i = ready.shift() as number
    out.push(items[i])
    placed[i] = true
    for (const c of children[i]) {
      inDegree[c]--
      if (inDegree[c] === 0) ready.push(c)
    }
  }
  items.forEach((it, i) => {
    if (!placed[i]) out.push(it)
  })
  return out
}

/**
 * 分野 1 つの学習パスを組み立てる（Go の BuildPath）。
 * 「今ここ」は、依存順に見て最初に現れる「まだ済んでいない・着手できる」項目。
 * 道のりのどこまで来たかを示す先端で、優先度とは無関係に決まる。
 * そのため「次にやること」の 1 位とは一致しないことがある（別の問いに答えている。SPEC.md §7.2）
 */
export function buildPath(domain: RoadmapDomain, states: Map<string, ItemState>): Path {
  const ordered = topoSort(domain.items)
  const inDomain = new Set(domain.items.map((it) => it.key))
  const current = ordered.find((it) => !isDone(states, it) && isReady(states, it))?.key

  let doneCount = 0
  const nodes = ordered.map((it): PathNode => {
    let status: PathStatus = 'upcoming'
    if (isDone(states, it)) {
      status = 'done'
      doneCount++
    } else if (it.key === current) {
      status = 'current'
    }
    return {
      item: it,
      state: stateOf(states, it.key),
      status,
      ready: isReady(states, it),
      externalDeps: (it.dependsOn ?? []).filter((d) => !inDomain.has(d)),
    }
  })
  return {
    domainKey: domain.key,
    domainName: domain.name,
    goal: domain.goal,
    nodes,
    doneCount,
    totalCount: domain.items.length,
  }
}
