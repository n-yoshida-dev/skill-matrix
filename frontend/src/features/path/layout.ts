import type { ItemState, RoadmapDomain } from '../../data/types'

// スキルツリー表示（SPEC.md §7.2）の配置計算。純粋関数で、DOM を import しない。
// 段 = 依存の深さ（前提なし = 0 段、前提の最大段 + 1）。同じ段は定義順で横に並べる。
// 他分野の前提はゴーストノード（灰色）として置く。ゴーストノード自身の依存は描かないので、前提なし＝0 段になる。
// グラフ描画ライブラリは使わない（1 分野の項目が 30 を超えて線が交差したら React Flow + dagre を検討。SPEC.md §7.2）。

/** ツリーの節 1 つ。row が段（上が前提）、col が段の中の横位置 */
export interface TreeNode {
  key: string
  row: number
  col: number
  /** 他分野の前提を示すゴーストノード */
  ghost: boolean
  /** その項目が属する分野（ゴーストノードなら移動先の分野） */
  domainKey: string
}

/** 線 1 本。from が前提、to がそれを前提にする項目 */
export interface TreeEdge {
  from: string
  to: string
  /** ゴーストノードから引く線 */
  ghost: boolean
}

export interface TreeLayout {
  nodes: TreeNode[]
  edges: TreeEdge[]
  /** 段の数 */
  rows: number
  /** 最も広い段の横の数 */
  cols: number
}

/**
 * 分野 1 つのツリーの配置を計算する。
 * domains はゴーストノードの所属分野を引くためのロードマップ全体（見つからない前提はゴーストにせず無視する）。
 * 循環参照があっても落ちない（循環の辺は深さの計算で無視する。循環は CLI の verify が弾く決まり）
 */
export function layoutTree(domain: RoadmapDomain, domains: RoadmapDomain[]): TreeLayout {
  const inDomain = new Map(domain.items.map((it, i) => [it.key, i]))
  const owner = new Map(domains.flatMap((d) => d.items.map((it) => [it.key, d.key])))

  // ゴーストノード：分野の外の前提。最初に前提として現れた順（定義順）に 1 つずつ
  const ghosts: string[] = []
  for (const it of domain.items) {
    for (const dep of it.dependsOn ?? []) {
      if (!inDomain.has(dep) && owner.has(dep) && !ghosts.includes(dep)) ghosts.push(dep)
    }
  }

  // 深さ：前提の最大段 + 1。ゴーストは 0 段。探索中の項目に戻る辺（循環）は無視する
  const depth = new Map<string, number>(ghosts.map((g) => [g, 0]))
  const visiting = new Set<string>()
  const depthOf = (key: string): number => {
    const known = depth.get(key)
    if (known !== undefined) return known
    visiting.add(key)
    const it = domain.items[inDomain.get(key) as number]
    let d = 0
    for (const dep of it.dependsOn ?? []) {
      if (visiting.has(dep)) continue
      if (inDomain.has(dep) || ghosts.includes(dep)) d = Math.max(d, depthOf(dep) + 1)
    }
    visiting.delete(key)
    depth.set(key, d)
    return d
  }
  domain.items.forEach((it) => depthOf(it.key))

  // 段ごとに、分野の項目を定義順に並べ、そのあとにゴーストを並べる
  const nodes: TreeNode[] = []
  const widths: number[] = []
  const place = (key: string, ghost: boolean) => {
    const row = depth.get(key) ?? 0
    const col = widths[row] ?? 0
    widths[row] = col + 1
    nodes.push({ key, row, col, ghost, domainKey: owner.get(key) ?? domain.key })
  }
  domain.items.forEach((it) => place(it.key, false))
  ghosts.forEach((g) => place(g, true))
  nodes.sort((a, b) => a.row - b.row || a.col - b.col)

  const edges: TreeEdge[] = []
  for (const it of domain.items) {
    for (const dep of it.dependsOn ?? []) {
      if (inDomain.has(dep)) edges.push({ from: dep, to: it.key, ghost: false })
      else if (ghosts.includes(dep)) edges.push({ from: dep, to: it.key, ghost: true })
    }
  }

  return {
    nodes,
    edges,
    rows: widths.length,
    cols: Math.max(0, ...widths.map((w) => w ?? 0)),
  }
}

/** 節の状態（SPEC.md §7.2 の表）。深掘り候補は習得済みの上に札を足すだけなので、ここには入れない */
export type NodeKind = 'locked' | 'open' | 'done'

/** 節の状態の日本語 */
export const KIND_LABELS: Record<NodeKind, string> = {
  locked: '未解放',
  open: '解放済み・未着手',
  done: '習得済み',
}

/** 未解放＝前提にレベル 0 がある、解放済み・未着手＝着手できてレベル 0、習得済み＝レベル 1 以上 */
export function nodeKind(st: ItemState, ready: boolean): NodeKind {
  if (st.verifiedLevel >= 1) return 'done'
  return ready ? 'open' : 'locked'
}
