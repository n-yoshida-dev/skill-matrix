import type { ItemState, RoadmapDomain } from '../../data/types'
import { missingLevels } from '../plan/matrix'
import { pendingLevels } from '../plan/next'

// スキルツリー表示（SPEC.md §7.2）の配置計算。純粋関数で、DOM を import しない。
// 段 = 依存の深さ（前提なし = 0 段、前提の最大段 + 1）。同じ段は定義順で横に並べる。
// 線は縦と横だけの折れ線で、四角の内側を通さない。2 段以上離れた線は、途中の段の四角のすき間を縦に通る。
// 前提が 2 つ以上ある項目は、すぐ上に合流点（横棒）を置き、前提からの線をすべてそこへ集めて 1 本で項目へ下ろす。
// 他分野の前提はゴーストノードとして合流点の横に小さく置く（最上段には並べない。2026-10-03 本人了承の 4）。
// 線の通り道・合流点の位置など、了承の外で Claude が選んだ細部は KNOWLEDGE.md 2026-10-03 の確認待ちの表にある。
// グラフ描画ライブラリは使わない（1 分野の項目が 30 を超えて線が交差したら React Flow + dagre を検討。SPEC.md §7.2）。

/** 大きさと間隔（px）。画面（TreeView）はここで計算した位置に四角と線を置く */
export const TREE_GEOMETRY = {
  /** 節の幅・高さ。高さは、項目名 2 行と鍵の項目の「必要：」3 行（前提 3 つの名前）が収まる分 */
  nodeW: 172,
  nodeH: 104,
  /** 列のあいだ。2 段以上離れた線がちょうど 1 本通れる幅（laneMargin の 2 倍） */
  gapX: 24,
  /** 段のあいだ（最小） */
  minGapY: 40,
  /** 図の外周の余白 */
  pad: 4,
  /** ゴーストノードの幅・高さ（節より小さく。幅は「他分野：データベース設計」が 1 行に収まる分） */
  ghostW: 164,
  ghostH: 54,
  /** 1 つの節の下辺から出る線どうしの間隔 */
  portStep: 12,
  /** 合流点の横棒に入る線どうしの間隔 */
  landStep: 24,
  /** 段を縦に貫く線と、その段の節とのすき間 */
  laneMargin: 12,
  /** 段を縦に貫く線どうしの間隔 */
  laneSep: 12,
  /** 段を縦に貫く線の位置を探す刻み */
  laneScan: 2,
  /** 段を縦に貫く線が曲がる 1 回あたりの重み（まっすぐ下りられる位置を選びやすくする） */
  bendCost: 40,
  /** 段のあいだを横に走る線どうしの間隔（列をまたいで入れ替わる 2 本が平行に走っても、別の線と分かる幅） */
  trackGap: 14,
  /** 節の下辺から、最初に横に走る線まで */
  topMargin: 20,
  /** 合流点の帯（無ければ最後の横の線）から、次の段の節の上辺まで */
  bottomMargin: 12,
  /** 合流点の帯の高さ（ゴーストノードを置かないとき）。横棒の下に合流点の文字が入る分 */
  junctionBand: 24,
  /** 合流点の帯の上端から横棒まで（ゴーストノードを置かないとき） */
  junctionBarTop: 6,
  /** 合流点の文字「n つとも必要 x/n」の枠。項目へ下りる線のすぐ右、横棒（ゴーストノードがあればその下端）の下に置く */
  labelW: 80,
  labelH: 13,
  /** 文字と、項目へ下りる線のあいだ */
  labelSide: 6,
  /** 文字と、その上の横棒・ゴーストノードのあいだ */
  labelGap: 3,
  /** ゴーストノードのまわりのすき間 */
  ghostClear: 6,
  /** ゴーストノードと横棒をつなぐ線の長さ（最短） */
  ghostLink: 16,
  /** ゴーストノードの置き場を探す刻み */
  ghostScan: 6,
}

export type TreeGeometry = typeof TREE_GEOMETRY

/** 点（px） */
export interface Point {
  x: number
  y: number
}

/** 四角の位置と大きさ（px。x, y は左上） */
export interface Box {
  x: number
  y: number
  w: number
  h: number
}

/** ツリーの節 1 つ（分野の項目）。row が段（上が前提）、col が段の中の横位置 */
export interface TreeNode extends Box {
  key: string
  row: number
  col: number
  domainKey: string
}

/** 他分野の前提（ゴーストノード）。押すとその分野のツリーへ移る */
export interface GhostNode extends Box {
  /** 前提の項目キー */
  key: string
  /** 移動先の分野 */
  domainKey: string
  /** この前提を必要とする項目。同じ前提を 2 つの項目が指すと、それぞれの横に 1 つずつ置く */
  to: string
}

/** 線 1 本。from が前提、to がそれを前提にする項目。points は折れ線の頂点（前提の側から順に） */
export interface TreeEdge {
  from: string
  to: string
  /** ゴーストノードから引く線 */
  ghost: boolean
  points: Point[]
}

/**
 * 合流点。前提が 2 つ以上ある項目のすぐ上に置く。前提からの線はすべて横棒（x1〜x2、高さ y）に入り、
 * 横棒の (x, y)（項目の中心の真上）から 1 本で項目へ下りる
 */
export interface Junction {
  to: string
  x: number
  y: number
  x1: number
  x2: number
  /** 合流点の文字（「n つとも必要 x/n」）を置く枠。項目の幅の内側にあり、線と四角に重ならない */
  label: Box
}

export interface TreeLayout {
  nodes: TreeNode[]
  ghosts: GhostNode[]
  edges: TreeEdge[]
  junctions: Junction[]
  /** 段の数 */
  rows: number
  /** 最も広い段の横の数 */
  cols: number
  /** 図全体の幅・高さ（px） */
  width: number
  height: number
}

/** 分野の項目 1 つの置き場（計算の途中。x は全体をずらす前の値） */
interface Slot {
  key: string
  /** 定義順 */
  index: number
  row: number
  col: number
  /** 左端と中心 */
  x: number
  cx: number
}

/** 分野の中の線 1 本（計算の途中） */
interface Link {
  from: Slot
  to: Slot
  /** 2 段以上離れた線が、途中の段を縦に貫く x */
  lane?: number
  /** 前提の節の下辺から出る x */
  port: number
  /** 項目の上辺（前提が 1 つ）か、合流点の横棒（前提が 2 つ以上）に入る x */
  target: number
}

/** ゴーストノード 1 つ（計算の途中） */
interface Ghost {
  key: string
  to: Slot
  /** dependsOn の中での順（出力の並び順） */
  order: number
  /** 横棒の左（-1）か右（1）か。前提がこれ 1 つだけの項目では 0（項目の真上） */
  side: -1 | 0 | 1
  /** 横棒の高さを 0 として、上へ何段目に積むか */
  level: number
  /** 左端 */
  x: number
  /** 横棒（前提が 1 つだけなら項目の上辺）に入る x */
  land: number
}

/** 合流点の横棒の左右の端 */
interface Bar {
  x1: number
  x2: number
}

/** 段のあいだを横に走る線 1 本（計算の途中） */
interface Piece {
  link: Link
  /** 上から入ってくる x と、下へ出ていく x */
  top: number
  bottom: number
  /** 隣の段への線（adj）か、2 段以上離れた線の出発側（start）・到着側（end）か */
  kind: 'adj' | 'start' | 'end'
  y: number
}

/**
 * 分野 1 つのツリーの配置を計算する。
 * domains はゴーストノードの所属分野を引くためのロードマップ全体（見つからない前提はゴーストにせず無視する）。
 * 循環参照があっても落ちない（循環の辺は深さの計算で無視し、線も描かない。循環は CLI の verify が弾く決まり）
 */
export function layoutTree(
  domain: RoadmapDomain,
  domains: RoadmapDomain[],
  g: TreeGeometry = TREE_GEOMETRY,
): TreeLayout {
  if (domain.items.length === 0) {
    return {
      nodes: [],
      ghosts: [],
      edges: [],
      junctions: [],
      rows: 0,
      cols: 0,
      width: 0,
      height: 0,
    }
  }
  const index = new Map(domain.items.map((it, i) => [it.key, i]))
  const owner = new Map(domains.flatMap((d) => d.items.map((it) => [it.key, d.key])))

  // 前提を、分野の中のものと他分野のもの（ゴースト）に分ける。どこにも無い前提と重複は無視する
  const deps = new Map(domain.items.map((it) => [it.key, [...new Set(it.dependsOn ?? [])]]))
  const inner = new Map<string, string[]>()
  const outer = new Map<string, string[]>()
  for (const it of domain.items) {
    const list = deps.get(it.key) ?? []
    inner.set(
      it.key,
      list.filter((d) => index.has(d)),
    )
    outer.set(
      it.key,
      list.filter((d) => !index.has(d) && owner.has(d)),
    )
  }

  const depth = depthsOf(domain, inner, outer)
  const { byRow, slots } = placeSlots(domain, depth, g)
  const cols = Math.max(...byRow.map((r) => r.length))
  const gridW = cols * (g.nodeW + g.gapX) - g.gapX

  // 描く線：分野の中の前提 → 項目。循環の辺（同じ段・下の段から上の段）は描かない
  const links: Link[] = []
  for (const it of domain.items) {
    for (const dep of inner.get(it.key) ?? []) {
      const from = slots.get(dep) as Slot
      const to = slots.get(it.key) as Slot
      if (from.row < to.row) links.push({ from, to, port: from.cx, target: to.cx })
    }
  }
  const junctions = new Set(
    domain.items
      .filter(
        (it) =>
          links.filter((l) => l.to.key === it.key).length + (outer.get(it.key) ?? []).length >= 2,
      )
      .map((it) => it.key),
  )

  assignLanes(links, byRow, gridW, junctions, g)
  const bars = assignLandings(junctions, slots, links, g)
  const ghosts = placeGhosts(domain, outer, slots, links, byRow, bars, junctions, gridW, g)
  assignPorts(byRow, links, g)
  const pieces = collectPieces(links, byRow.length)

  // 縦の位置：段のあいだの高さは、横に走る線の本数と合流点の帯（ゴーストノードを積む段数）で決まる
  const rowTop: number[] = []
  const barY: number[] = []
  const labelY: number[] = []
  let y = g.pad
  byRow.forEach((row, r) => {
    rowTop[r] = y
    if (r === byRow.length - 1) return
    const bottom = y + (row.length > 0 ? g.nodeH : 0)
    pieces[r].forEach((p, i) => {
      p.y = bottom + g.topMargin + i * g.trackGap
    })
    const cursor = bottom + g.topMargin + pieces[r].length * g.trackGap
    const levels = Math.max(
      0,
      ...ghosts.filter((gh) => gh.to.row === r + 1).map((gh) => gh.level + 1),
    )
    const hasJunction = byRow[r + 1].some((s) => junctions.has(s.key))
    const bandH =
      levels > 0
        ? levels * (g.ghostH + g.ghostClear) + g.ghostClear
        : hasJunction
          ? g.junctionBand
          : 0
    barY[r] = levels > 0 ? cursor + bandH - g.ghostClear - g.ghostH / 2 : cursor + g.junctionBarTop
    // 合流点の文字は横棒の下。ゴーストノードのある段のあいだでは、横に並ぶゴーストノードの下端より下
    labelY[r] = barY[r] + (levels > 0 ? g.ghostH / 2 : 0) + g.labelGap
    y = Math.max(bottom + g.minGapY, cursor + bandH + g.bottomMargin)
  })
  const top = (s: Slot) => rowTop[s.row]
  const ghostY = (gh: Ghost) =>
    barY[gh.to.row - 1] - gh.level * (g.ghostH + g.ghostClear) - g.ghostH / 2

  // 折れ線：前提の下辺 → （横に走る線）→ （縦に貫く線）→ 項目の上辺か横棒
  const piecesOf = new Map<Link, Partial<Record<Piece['kind'], Piece>>>()
  for (const p of pieces.flat()) piecesOf.set(p.link, { ...piecesOf.get(p.link), [p.kind]: p })
  const linkPoints = (l: Link): Point[] => {
    const ps = piecesOf.get(l) ?? {}
    const lane = l.lane ?? l.port
    const pts: Point[] = [{ x: l.port, y: top(l.from) + g.nodeH }]
    if (ps.adj) pts.push({ x: l.port, y: ps.adj.y }, { x: l.target, y: ps.adj.y })
    if (ps.start) pts.push({ x: l.port, y: ps.start.y }, { x: lane, y: ps.start.y })
    if (ps.end) pts.push({ x: lane, y: ps.end.y }, { x: l.target, y: ps.end.y })
    pts.push({ x: l.target, y: junctions.has(l.to.key) ? barY[l.to.row - 1] : top(l.to) })
    return simplify(pts)
  }
  // ゴーストノード → 項目の上辺（真上に置いたとき）か横棒の端。上に積んだものは L 字で横棒へ下りる
  const ghostPoints = (gh: Ghost): Point[] => {
    const cy = ghostY(gh) + g.ghostH / 2
    if (gh.side === 0)
      return [
        { x: gh.land, y: cy + g.ghostH / 2 },
        { x: gh.land, y: top(gh.to) },
      ]
    const edgeX = gh.side < 0 ? gh.x + g.ghostW : gh.x
    return simplify([
      { x: edgeX, y: cy },
      { x: gh.land, y: cy },
      { x: gh.land, y: barY[gh.to.row - 1] },
    ])
  }

  const edges: TreeEdge[] = []
  for (const it of domain.items) {
    for (const dep of deps.get(it.key) ?? []) {
      const link = links.find((l) => l.from.key === dep && l.to.key === it.key)
      if (link) edges.push({ from: dep, to: it.key, ghost: false, points: linkPoints(link) })
      const gh = ghosts.find((x) => x.key === dep && x.to.key === it.key)
      if (gh) edges.push({ from: dep, to: it.key, ghost: true, points: ghostPoints(gh) })
    }
  }

  // 全体を横にずらし、いちばん左の要素が余白 pad の位置に来るようにする（ゴーストノードや縦の線が 0 列目より左に出ることがある）
  const xs = [
    ...[...slots.values()].flatMap((s) => [s.x, s.x + g.nodeW]),
    ...ghosts.flatMap((gh) => [gh.x, gh.x + g.ghostW]),
    ...edges.flatMap((e) => e.points.map((p) => p.x)),
    ...[...bars.values()].flatMap((b) => [b.x1, b.x2]),
  ]
  const dx = g.pad - Math.min(...xs)
  const width = Math.max(...xs) - Math.min(...xs) + 2 * g.pad
  const lastRow = byRow.length - 1
  const height = rowTop[lastRow] + g.nodeH + g.pad
  const shift = (p: Point): Point => ({ x: p.x + dx, y: p.y })

  const nodes: TreeNode[] = [...slots.values()]
    .sort((a, b) => a.row - b.row || a.col - b.col)
    .map((s) => ({
      key: s.key,
      row: s.row,
      col: s.col,
      domainKey: domain.key,
      x: s.x + dx,
      y: top(s),
      w: g.nodeW,
      h: g.nodeH,
    }))
  const ghostNodes: GhostNode[] = [...ghosts]
    .sort((a, b) => a.to.index - b.to.index || a.order - b.order)
    .map((gh) => ({
      key: gh.key,
      domainKey: owner.get(gh.key) as string,
      to: gh.to.key,
      x: gh.x + dx,
      y: ghostY(gh),
      w: g.ghostW,
      h: g.ghostH,
    }))
  const junctionList: Junction[] = [...junctions].map((key) => {
    const t = slots.get(key) as Slot
    const bar = bars.get(key) as Bar
    // 文字は項目へ下りる線のすぐ右。項目の幅の内側なので、ほかの項目へ下りる線や段を縦に貫く線とは重ならない
    const label = {
      x: t.cx + g.labelSide + dx,
      y: labelY[t.row - 1],
      w: g.labelW,
      h: g.labelH,
    }
    return { to: key, x: t.cx + dx, y: barY[t.row - 1], x1: bar.x1 + dx, x2: bar.x2 + dx, label }
  })

  return {
    nodes,
    ghosts: ghostNodes,
    edges: edges.map((e) => ({ ...e, points: e.points.map(shift) })),
    junctions: junctionList,
    rows: byRow.length,
    cols,
    width,
    height,
  }
}

/**
 * 各項目の段（前提の最大段 + 1）。他分野の前提は 0 段にあるものとして数える
 * （その項目は 1 段以上になり、上にゴーストノードを置く場所ができる）。探索中の項目に戻る辺（循環）は無視する
 */
function depthsOf(
  domain: RoadmapDomain,
  inner: Map<string, string[]>,
  outer: Map<string, string[]>,
): Map<string, number> {
  const depth = new Map<string, number>()
  const visiting = new Set<string>()
  const depthOf = (key: string): number => {
    const known = depth.get(key)
    if (known !== undefined) return known
    visiting.add(key)
    let d = (outer.get(key) ?? []).length > 0 ? 1 : 0
    for (const dep of inner.get(key) ?? []) {
      if (!visiting.has(dep)) d = Math.max(d, depthOf(dep) + 1)
    }
    visiting.delete(key)
    depth.set(key, d)
    return d
  }
  domain.items.forEach((it) => depthOf(it.key))
  return depth
}

/** 段ごとに、項目を定義順で左から並べる（x は全体をずらす前の値） */
function placeSlots(domain: RoadmapDomain, depth: Map<string, number>, g: TreeGeometry) {
  const rowCount = Math.max(...domain.items.map((it) => depth.get(it.key) ?? 0)) + 1
  const byRow: Slot[][] = Array.from({ length: rowCount }, () => [])
  const slots = new Map<string, Slot>()
  domain.items.forEach((it, index) => {
    const row = depth.get(it.key) ?? 0
    const col = byRow[row].length
    const x = col * (g.nodeW + g.gapX)
    const s: Slot = { key: it.key, index, row, col, x, cx: x + g.nodeW / 2 }
    byRow[row].push(s)
    slots.set(it.key, s)
  })
  return { byRow, slots }
}

/**
 * 2 段以上離れた線が、途中の段を縦に貫く x を決める。
 * 守ること：途中の段の節から laneMargin 以上離す／出発・到着の段のほかの節の幅にも入らない（その節から出入りする線とぶつからないように）／
 * 区間が重なるほかの縦の線から laneSep 以上離す。そのうえで、左右に曲がる長さと回数が少ない位置を選ぶ。
 * 短い線から先に決める（内側を通るので、外側を回る長い線と交差しにくい）
 */
function assignLanes(
  links: Link[],
  byRow: Slot[][],
  gridW: number,
  junctions: Set<string>,
  g: TreeGeometry,
): void {
  const long = links
    .filter((l) => l.to.row - l.from.row >= 2)
    .sort(
      (p, q) =>
        p.to.row - p.from.row - (q.to.row - q.from.row) ||
        p.to.index - q.to.index ||
        p.from.index - q.from.index,
    )
  const blocks = (x: number, s: Slot) => x > s.x - g.laneMargin && x < s.x + g.nodeW + g.laneMargin
  // 全部の線を図の外側に並べても収まる幅まで探す（必ずどこかに置ける）
  const reach = g.nodeW + g.laneSep * (long.length + 1)
  const done: Link[] = []
  for (const l of long) {
    const a = l.from.row
    const b = l.to.row
    const ok = (x: number) =>
      byRow.slice(a + 1, b).every((row) => row.every((s) => !blocks(x, s))) &&
      byRow[a].every((s) => !blocks(x, s) || (s === l.from && x === s.cx)) &&
      byRow[b].every((s) => !blocks(x, s) || (s === l.to && !junctions.has(s.key) && x === s.cx)) &&
      done.every(
        (o) =>
          o.from.row > b - 1 || o.to.row - 1 < a || Math.abs((o.lane as number) - x) >= g.laneSep,
      )
    // 比べる順：曲がる長さと回数 → 前提と項目の中間からの距離 → 図の外へはみ出さない → 左
    const cost = (x: number): number[] => [
      Math.abs(x - l.from.cx) +
        Math.abs(x - l.to.cx) +
        (x === l.from.cx ? 0 : g.bendCost) +
        (x === l.to.cx ? 0 : g.bendCost),
      Math.abs(x - (l.from.cx + l.to.cx) / 2),
      x < 0 || x > gridW ? 1 : 0,
      x,
    ]
    const candidates = [l.from.cx, l.to.cx]
    for (let x = -reach; x <= gridW + reach; x += g.laneScan) candidates.push(x)
    let best: { x: number; cost: number[] } | undefined
    for (const x of candidates) {
      if (!ok(x)) continue
      const c = cost(x)
      if (!best || lexLess(c, best.cost)) best = { x, cost: c }
    }
    l.lane = best ? best.x : gridW + reach
    done.push(l)
  }
}

/** 縦に貫く線があればその x、無ければ前提の中心（合流点の横棒に入る順を決めるときの「来る向き」） */
const approach = (l: Link) => l.lane ?? l.from.cx

/**
 * 合流点の横棒に、前提からの線が入る x を決める。項目の中心を挟んで landStep おきに、来る向きの順（左から来る線が左）に並べる。
 * 戻り値は合流点のある項目ごとの横棒の左右の端（この時点ではゴーストノードの分を含まない）
 */
function assignLandings(
  junctions: Set<string>,
  slots: Map<string, Slot>,
  links: Link[],
  g: TreeGeometry,
): Map<string, Bar> {
  const bars = new Map<string, Bar>()
  for (const key of junctions) {
    const t = slots.get(key) as Slot
    const ins = links
      .filter((l) => l.to === t)
      .sort((p, q) => approach(p) - approach(q) || p.from.index - q.from.index)
    ins.forEach((l, i) => {
      l.target = t.cx + (i - (ins.length - 1) / 2) * g.landStep
    })
    const xs = [t.cx, ...ins.map((l) => l.target)]
    bars.set(key, { x1: Math.min(...xs), x2: Math.max(...xs) })
  }
  return bars
}

/**
 * ゴーストノード（他分野の前提）を置く。前提がそれ 1 つだけの項目では項目の真上、合流点のある項目では横棒の横。
 * 横棒の横は、分野の中の前提が来る向きの反対側（右から来るなら左。真上から来るか、分野の中の前提が無ければ左）を先に試す。
 * 合流点の帯を縦に通る線・ほかの横棒・置き済みのゴーストノードとぶつかるなら外へずらし、1 つだけのときはずらす量の少ない側を選ぶ。
 * 2 つ目は反対側、3 つ目からは左右交互に上へ積み、横棒の端の内側へ L 字の線で入る
 */
function placeGhosts(
  domain: RoadmapDomain,
  outer: Map<string, string[]>,
  slots: Map<string, Slot>,
  links: Link[],
  byRow: Slot[][],
  bars: Map<string, Bar>,
  junctions: Set<string>,
  gridW: number,
  g: TreeGeometry,
): Ghost[] {
  const ghosts: Ghost[] = []
  for (const it of domain.items) {
    const outs = outer.get(it.key) ?? []
    if (outs.length === 1 && !junctions.has(it.key)) {
      const t = slots.get(it.key) as Slot
      ghosts.push({
        key: outs[0],
        to: t,
        order: 0,
        side: 0,
        level: 0,
        x: t.cx - g.ghostW / 2,
        land: t.cx,
      })
    }
  }
  // ずらす上限：図の外側の縦の線とゴーストノードをすべて越える幅
  const far =
    gridW +
    2 * (g.nodeW + g.ghostW + g.ghostLink) +
    g.laneSep * links.length +
    g.ghostW * domain.items.length
  for (const it of domain.items) {
    const outs = outer.get(it.key) ?? []
    if (outs.length === 0 || !junctions.has(it.key)) continue
    const t = slots.get(it.key) as Slot
    const bar = bars.get(it.key) as Bar
    const c = t.row - 1
    // 合流点の帯を縦に通る線：前提が 1 つの項目へ下りる線と、この段のあいだを貫く・ここから下りていく縦の線
    const lines = [
      ...byRow[c + 1]
        .filter((s) => !junctions.has(s.key) && links.some((l) => l.to === s))
        .map((s) => s.cx),
      ...links
        .filter((l) => l.lane !== undefined && l.from.row <= c && c <= l.to.row - 2)
        .map((l) => l.lane as number),
    ]
    // ぶつかってはいけない横の範囲：同じ段のほかの横棒と、置き済みのゴーストノード（つなぐ線を含む）
    const spans: [number, number][] = [
      ...byRow[c + 1]
        .filter((s) => s !== t && junctions.has(s.key))
        .map((s): [number, number] => [(bars.get(s.key) as Bar).x1, (bars.get(s.key) as Bar).x2]),
      ...ghosts
        .filter((gh) => gh.to.row === t.row)
        .map((gh): [number, number] => [
          Math.min(gh.x, gh.land),
          Math.max(gh.x + g.ghostW, gh.land),
        ]),
    ]
    const fits = (x: number, land: number) => {
      const lo = Math.min(x, land) - g.ghostClear
      const hi = Math.max(x + g.ghostW, land) + g.ghostClear
      return (
        lines.every((v) => v <= x - g.ghostClear || v >= x + g.ghostW + g.ghostClear) &&
        spans.every(([a, b]) => b < lo || a > hi)
      )
    }
    // ずらす前の位置：横棒の端（edge）から ghostLink だけ外。上に積むものも同じ列にそろえる
    const start = (side: -1 | 1, edge: number) =>
      side < 0 ? edge - g.ghostLink - g.ghostW : edge + g.ghostLink
    // 横棒から外へ向かってずらし、最初にぶつからない位置。上限まで見つからなければ undefined
    const slide = (side: -1 | 1, edge: number, land: number): number | undefined => {
      for (let d = 0; d <= far; d += g.ghostScan) {
        const x = start(side, edge) + side * d
        if (fits(x, land)) return x
      }
      return undefined
    }

    const ins = links.filter((l) => l.to === t)
    const mean = ins.length > 0 ? ins.reduce((sum, l) => sum + approach(l), 0) / ins.length : t.cx
    const pref: -1 | 1 = mean < t.cx ? 1 : -1
    const other: -1 | 1 = pref < 0 ? 1 : -1
    if (outs.length === 1) {
      const options = [pref, other].map((side) => {
        const land = side < 0 ? bar.x1 : bar.x2
        const x = slide(side, land, land)
        return {
          side,
          land,
          x,
          moved: x === undefined ? Infinity : Math.abs(x - start(side, land)),
        }
      })
      const best = options[1].moved < options[0].moved ? options[1] : options[0]
      ghosts.push({
        key: outs[0],
        to: t,
        order: 0,
        side: best.side,
        level: 0,
        x: best.x ?? start(best.side, best.land),
        land: best.land,
      })
      continue
    }
    const sides = outs.map((_, i) => (i % 2 === 0 ? pref : other))
    const count = (side: -1 | 1) => sides.filter((s) => s === side).length
    // 片側に 2 つ以上積むときは、横棒をその数だけ外へ伸ばし、上に積んだものが入る位置を作る
    const end = (side: -1 | 1) => {
      const m = count(side)
      if (side < 0) return m <= 1 ? bar.x1 : bar.x1 - m * g.landStep
      return m <= 1 ? bar.x2 : bar.x2 + m * g.landStep
    }
    const placed = { [-1]: 0, [1]: 0 } as Record<-1 | 1, number>
    const x1 = end(-1)
    const x2 = end(1)
    outs.forEach((key, order) => {
      const side = sides[order]
      const level = placed[side]++
      const land = level === 0 ? end(side) : end(side) - side * level * g.landStep
      const x = slide(side, end(side), land) ?? start(side, end(side))
      ghosts.push({ key, to: t, order, side, level, x, land })
    })
    if (count(-1) > 0) bar.x1 = Math.min(bar.x1, x1)
    if (count(1) > 0) bar.x2 = Math.max(bar.x2, x2)
  }
  return ghosts
}

/**
 * 前提の節の下辺から線が出る x を決める。真下の節へ入る線（と、真下へ縦に貫く線）はその位置からまっすぐ出し、
 * ほかは下辺の中心から portStep おきに、行き先が左の線は左、右の線は右へ、遠い行き先ほど外側に並べる（兄弟の線どうしが交差しないように）。
 * 真下の節へ下りるほかの線の x（中心・横棒に入る位置）は避ける（同じ縦の線の上に 2 本が重ならないように）。
 * 片側に 7 本以上出ると四角の幅からはみ出す（今のロードマップは最大 2 本）
 */
function assignPorts(byRow: Slot[][], links: Link[], g: TreeGeometry): void {
  for (const s of byRow.flat()) {
    const outs = links.filter((l) => l.from === s)
    if (outs.length === 0) continue
    const below = byRow[s.row + 1]?.find((n) => n.col === s.col)
    const head = (l: Link) => l.lane ?? l.target
    const straight = outs.filter((l) => (below !== undefined && l.to === below) || head(l) === s.cx)
    for (const l of straight) l.port = head(l)
    const used = straight.map((l) => l.port)
    const forbidden = below
      ? [below.cx, ...links.filter((l) => l.to === below && l.from !== s).map((l) => l.target)]
      : []
    const free = (x: number) => !used.includes(x) && !forbidden.includes(x)
    const rest = outs.filter((l) => !straight.includes(l))
    if (straight.length === 0 && rest.length === 1 && free(s.cx)) {
      rest[0].port = s.cx
      continue
    }
    const left = rest.filter((l) => head(l) < s.cx).sort((p, q) => head(q) - head(p))
    const right = rest.filter((l) => head(l) > s.cx).sort((p, q) => head(p) - head(q))
    for (const [group, dir] of [
      [left, -1],
      [right, 1],
    ] as const) {
      let k = 1
      for (const l of group) {
        while (!free(s.cx + dir * k * g.portStep)) k++
        l.port = s.cx + dir * k * g.portStep
        used.push(l.port)
        k++
      }
    }
  }
}

/**
 * 段のあいだを横に走る線を集め、段のあいだごとに上から並べる順を決める（1 本ずつ別の高さに置く）。
 * 右へ向かう線を上に（上から入る x が右のものほど上）、左へ向かう線をその下に（上から入る x が左のものほど上）並べると、
 * 向きの同じ線どうしは交差しない
 */
function collectPieces(links: Link[], rows: number): Piece[][] {
  const pieces: Piece[][] = Array.from({ length: Math.max(0, rows - 1) }, () => [])
  for (const l of links) {
    if (l.lane === undefined) {
      if (l.port !== l.target)
        pieces[l.from.row].push({ link: l, top: l.port, bottom: l.target, kind: 'adj', y: 0 })
      continue
    }
    if (l.port !== l.lane)
      pieces[l.from.row].push({ link: l, top: l.port, bottom: l.lane, kind: 'start', y: 0 })
    if (l.lane !== l.target)
      pieces[l.to.row - 1].push({ link: l, top: l.lane, bottom: l.target, kind: 'end', y: 0 })
  }
  for (const list of pieces) {
    list.sort((p, q) => {
      const pRight = p.bottom > p.top
      const qRight = q.bottom > q.top
      if (pRight !== qRight) return pRight ? -1 : 1
      return pRight ? q.top - p.top : p.top - q.top
    })
  }
  return pieces
}

/** 続けて同じ点と、まっすぐな線の途中の点を除く */
function simplify(pts: Point[]): Point[] {
  const out: Point[] = []
  for (const p of pts) {
    const last = out[out.length - 1]
    if (last && last.x === p.x && last.y === p.y) continue
    const prev = out[out.length - 2]
    if (
      prev &&
      last &&
      ((prev.x === last.x && last.x === p.x) || (prev.y === last.y && last.y === p.y))
    ) {
      out.pop()
    }
    out.push(p)
  }
  return out
}

/** 数の並びを辞書順で比べる（a が先なら true） */
function lexLess(a: number[], b: number[]): boolean {
  for (let i = 0; i < a.length; i++) if (a[i] !== b[i]) return a[i] < b[i]
  return false
}

/** 節の状態（SPEC.md §7.2 の表。2026-10-03 本人了承の 1 で 3 つにまとめた） */
export type NodeKind = 'locked' | 'open' | 'done'

/** 節の状態の日本語。レベル 1 は「習得済み」と呼ばない（同じ了承の 1） */
export const KIND_LABELS: Record<NodeKind, string> = {
  locked: '前提待ち',
  open: '挑戦できる',
  done: '確認済み',
}

/** 前提待ち＝前提にレベル 0 がある、挑戦できる＝着手できてレベル 0、確認済み＝レベル 1 以上（前提が未達でも鍵に戻さない） */
export function nodeKind(st: ItemState, ready: boolean): NodeKind {
  if (st.verifiedLevel >= 1) return 'done'
  return ready ? 'open' : 'locked'
}

/** ●の総数（レベルの最大値。SPEC.md §7.2「5 つ中 verifiedLevel 個」） */
const MAX_DOTS = 5

/** ●を level 個、残りを○で 5 つ並べる（例：レベル 2 → ●●○○○） */
export function levelDots(level: number): string {
  const n = Math.min(Math.max(level, 0), MAX_DOTS)
  return '●'.repeat(n) + '○'.repeat(MAX_DOTS - n)
}

/**
 * 「実装の根拠あり・基礎は未確認」を出すか。印が表示レベルより上にあり、その下に印の無い段がある項目
 * （項目詳細の「既存の実装について L1 / L2 を確認する」と同じ判定。SPEC.md §7.4）
 */
export function basicsUnverified(st: ItemState): boolean {
  return pendingLevels(st).length > 0 && missingLevels(st).length > 0
}
