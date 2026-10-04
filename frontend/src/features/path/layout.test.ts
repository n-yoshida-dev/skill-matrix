import { describe, expect, it } from 'vitest'
import { loadData } from '../../data/load'
import type { RoadmapDomain } from '../../data/types'
import { layoutTree, type Box, type Point, type TreeLayout } from './layout'

// スキルツリーの配置計算（layout.ts。SPEC.md §7.2）を確かめる。
// 「何を保証しているか」：
// - 段は依存の深さ（前提なし = 0 段、前提の最大段 + 1）。前提が上、派生が下。同じ段は定義順で左から
// - 線は縦と横だけの折れ線で、前提の四角の下辺から出て、項目の四角の上辺か合流点の横棒で終わる
// - どの線も、四角（ゴーストノードを含む）の内側を通らない。四角どうしも重ならない
// - 2 本の線が重なって 1 本に見えるところが無い（平行な線どうしは 8px 以上離れている）
// - 前提が 2 つ以上ある項目には、すぐ上に合流点がある。前提からの線（他分野の前提を含む）はすべてその横棒に入り、
//   合流点は項目の中心の真上にある。横棒と、そこから項目へ下りる線を、ほかの線が横切らない
// - 他分野の前提はゴーストノードとして、最上段ではなく、それを必要とする項目のすぐ上の段のあいだに置く。
//   合流点がある項目では横棒の横、前提がそれ 1 つだけの項目では真上。ゴースト自身の依存は描かない
// - 同じ他分野の前提を 2 つの項目が指すと、それぞれの横に 1 つずつ置く
// - 1 つの四角から出る線は、下辺の別々の位置から出る
// - 循環参照があっても全項目を置いて落ちない（循環の辺は描かない）
// - 上の線と四角の決まりが、実データ（data/roadmap.json）の全分野で成り立つ

// SPEC.md §7.2 の図の形：go-01 → go-02 → (go-03, go-04)、go-03 → go-05 → go-06。go-05 は react-03 にも依存
const goDomain: RoadmapDomain = {
  key: 'go',
  name: 'Go',
  items: [
    { key: 'go-01', name: '基本構文' },
    { key: 'go-02', name: 'メソッド', dependsOn: ['go-01'] },
    { key: 'go-03', name: 'エラー', dependsOn: ['go-02'] },
    { key: 'go-04', name: '並行処理', dependsOn: ['go-02'] },
    { key: 'go-05', name: 'net/http', dependsOn: ['go-03', 'react-03'] },
    { key: 'go-06', name: 'テスト', dependsOn: ['go-05'] },
  ],
}
const reactDomain: RoadmapDomain = {
  key: 'react',
  name: 'React',
  items: [
    { key: 'react-03', name: 'fetch' },
    { key: 'react-04', name: 'ルーティング' },
    { key: 'react-05', name: 'フォーム' },
  ],
}
const all = [goDomain, reactDomain]

// 2026-10-04 時点の実データの Go と同じ形（線が四角の裏を通る・3 本が 1 本に重なる、が起きていた形）。
// 実データが変わってもこのテストの意味が変わらないよう、形だけを写す
const goReal: RoadmapDomain = {
  key: 'go',
  name: 'Go',
  items: [
    { key: 'syntax', name: '' },
    { key: 'struct', name: '', dependsOn: ['syntax'] },
    { key: 'closure', name: '', dependsOn: ['syntax'] },
    { key: 'methods', name: '', dependsOn: ['struct'] },
    { key: 'interfaces', name: '', dependsOn: ['methods'] },
    { key: 'errors', name: '', dependsOn: ['interfaces'] },
    { key: 'concurrency', name: '', dependsOn: ['errors', 'closure'] },
    { key: 'nethttp', name: '', dependsOn: ['interfaces', 'errors', 'concurrency'] },
    { key: 'testing', name: '', dependsOn: ['nethttp'] },
  ],
}

// 同じく Java と同じ形（2 本の線が交差し、他分野の前提を持つ項目が 3 つある）
const dbDomain: RoadmapDomain = {
  key: 'db',
  name: 'DB',
  items: [
    { key: 'db-tenant', name: '' },
    { key: 'db-rbac', name: '' },
    { key: 'db-flyway', name: '' },
  ],
}
const javaReal: RoadmapDomain = {
  key: 'java',
  name: 'Java',
  items: [
    { key: 'rest', name: '' },
    { key: 'jwt', name: '' },
    { key: 'tenant', name: '', dependsOn: ['jwt', 'db-tenant'] },
    { key: 'request', name: '', dependsOn: ['tenant', 'db-rbac'] },
    { key: 'exception', name: '', dependsOn: ['rest'] },
    { key: 'logging', name: '', dependsOn: ['exception'] },
    { key: 'junit', name: '' },
    { key: 'testinfra', name: '', dependsOn: ['junit', 'db-flyway'] },
    { key: 'adr', name: '' },
  ],
}

const pos = (l: TreeLayout) => Object.fromEntries(l.nodes.map((n) => [n.key, [n.row, n.col]]))
const nodeOf = (l: TreeLayout, key: string) => l.nodes.find((n) => n.key === key) as Box
const edgeOf = (l: TreeLayout, from: string, to: string) =>
  l.edges.find((e) => e.from === from && e.to === to) as TreeLayout['edges'][number]
const last = (pts: Point[]) => pts[pts.length - 1]

/** 平行な線どうしをこれより近づけない（重なって 1 本に見えないように） */
const MIN_SEP = 8

interface Seg {
  a: Point
  b: Point
  edge: string
  to: string
}

/** 2 つの区間が正の長さで重なる */
const overlaps = (a1: number, a2: number, b1: number, b2: number) =>
  Math.min(Math.max(a1, a2), Math.max(b1, b2)) - Math.max(Math.min(a1, a2), Math.min(b1, b2)) > 0
/** v が lo と hi のあいだ（両端を含まない） */
const strictlyIn = (v: number, lo: number, hi: number) =>
  v > Math.min(lo, hi) && v < Math.max(lo, hi)

/** 縦か横の線分が、四角の内側（辺の上は含まない）を通る */
function cutsBox(s: Seg, b: Box): boolean {
  if (s.a.x === s.b.x) {
    return strictlyIn(s.a.x, b.x, b.x + b.w) && overlaps(s.a.y, s.b.y, b.y, b.y + b.h)
  }
  return strictlyIn(s.a.y, b.y, b.y + b.h) && overlaps(s.a.x, s.b.x, b.x, b.x + b.w)
}

/** 配置が決まりを破っている箇所を、読める文で返す（空なら問題なし） */
function problems(l: TreeLayout): string[] {
  const out: string[] = []
  const segs: Seg[] = l.edges.flatMap((e) =>
    e.points
      .slice(1)
      .map((p, i) => ({ a: e.points[i], b: p, edge: `${e.from}->${e.to}`, to: e.to })),
  )
  const boxes = [
    ...l.nodes.map((n) => ({ name: n.key, box: n as Box })),
    ...l.ghosts.map((g) => ({ name: `ゴースト ${g.key}（${g.to} の前提）`, box: g as Box })),
  ]

  for (const s of segs) {
    if (s.a.x !== s.b.x && s.a.y !== s.b.y) out.push(`${s.edge} に斜めの線がある`)
    for (const b of boxes) if (cutsBox(s, b.box)) out.push(`${s.edge} が ${b.name} の内側を通る`)
  }
  for (let i = 0; i < boxes.length; i++) {
    for (let j = i + 1; j < boxes.length; j++) {
      const p = boxes[i].box
      const q = boxes[j].box
      if (overlaps(p.x, p.x + p.w, q.x, q.x + q.w) && overlaps(p.y, p.y + p.h, q.y, q.y + q.h)) {
        out.push(`${boxes[i].name} と ${boxes[j].name} が重なる`)
      }
    }
  }
  for (let i = 0; i < segs.length; i++) {
    for (let j = i + 1; j < segs.length; j++) {
      const p = segs[i]
      const q = segs[j]
      if (p.edge === q.edge) continue
      const bothV = p.a.x === p.b.x && q.a.x === q.b.x
      const bothH = p.a.y === p.b.y && q.a.y === q.b.y
      if (bothV && Math.abs(p.a.x - q.a.x) < MIN_SEP && overlaps(p.a.y, p.b.y, q.a.y, q.b.y)) {
        out.push(`${p.edge} と ${q.edge} の縦の線が重なる（x=${p.a.x}, ${q.a.x}）`)
      }
      if (bothH && Math.abs(p.a.y - q.a.y) < MIN_SEP && overlaps(p.a.x, p.b.x, q.a.x, q.b.x)) {
        out.push(`${p.edge} と ${q.edge} の横の線が重なる（y=${p.a.y}, ${q.a.y}）`)
      }
    }
  }

  // 線の始まりと終わり
  for (const e of l.edges) {
    const name = `${e.from}->${e.to}`
    const first = e.points[0]
    const end = last(e.points)
    const to = nodeOf(l, e.to)
    const j = l.junctions.find((x) => x.to === e.to)
    if (e.ghost) {
      const g = l.ghosts.find((x) => x.key === e.from && x.to === e.to)
      if (!g) out.push(`${name} のゴーストノードが無い`)
      else {
        const onBorder =
          ((first.x === g.x || first.x === g.x + g.w) && first.y > g.y && first.y < g.y + g.h) ||
          (first.y === g.y + g.h && first.x > g.x && first.x < g.x + g.w)
        if (!onBorder) out.push(`${name} がゴーストノードの辺から出ていない`)
      }
    } else {
      const from = nodeOf(l, e.from)
      if (first.y !== from.y + from.h || !strictlyIn(first.x, from.x, from.x + from.w)) {
        out.push(`${name} が前提の四角の下辺から出ていない`)
      }
    }
    if (j) {
      if (end.y !== j.y || end.x < j.x1 || end.x > j.x2)
        out.push(`${name} が合流点の横棒に入っていない`)
    } else if (end.y !== to.y || !strictlyIn(end.x, to.x, to.x + to.w)) {
      out.push(`${name} が項目の四角の上辺に入っていない`)
    }
  }

  // 合流点：前提が 2 つ以上の項目だけに、項目の中心の真上に置く。横棒と下りる線をほかの線が横切らない
  const prereqs = (key: string) => l.edges.filter((e) => e.to === key).length
  for (const n of l.nodes) {
    const j = l.junctions.find((x) => x.to === n.key)
    if (prereqs(n.key) >= 2 && !j) out.push(`${n.key} に合流点が無い`)
    if (prereqs(n.key) < 2 && j) out.push(`${n.key} は前提が 1 つ以下なのに合流点がある`)
  }
  for (const j of l.junctions) {
    const t = nodeOf(l, j.to)
    if (j.x !== t.x + t.w / 2 || j.x < j.x1 || j.x > j.x2 || j.y >= t.y) {
      out.push(`${j.to} の合流点が項目の中心の真上に無い`)
    }
    for (const s of segs) {
      if (s.to === j.to) continue
      const v = s.a.x === s.b.x
      if (v && s.a.x >= j.x1 && s.a.x <= j.x2 && strictlyIn(j.y, s.a.y, s.b.y)) {
        out.push(`${s.edge} が ${j.to} の合流点の横棒を横切る`)
      }
      if (!v && strictlyIn(s.a.y, j.y, t.y) && strictlyIn(j.x, s.a.x, s.b.x)) {
        out.push(`${s.edge} が ${j.to} の合流点から下りる線を横切る`)
      }
    }
  }

  // ゴーストノード：それを必要とする項目のすぐ上の段のあいだ。合流点があれば横棒の横、無ければ真上
  for (const g of l.ghosts) {
    const t = l.nodes.find((n) => n.key === g.to)
    if (!t) {
      out.push(`ゴースト ${g.key} の行き先 ${g.to} が無い`)
      continue
    }
    const above = l.nodes.filter((n) => n.row === t.row - 1)
    const aboveBottom = above.length > 0 ? Math.max(...above.map((n) => n.y + n.h)) : 0
    if (g.y + g.h > t.y || g.y < aboveBottom)
      out.push(`ゴースト ${g.key} が ${g.to} のすぐ上の段のあいだに無い`)
    const j = l.junctions.find((x) => x.to === g.to)
    if (j && !(g.x + g.w < j.x1 || g.x > j.x2))
      out.push(`ゴースト ${g.key} が合流点の横棒の横に無い`)
    if (!j && g.x + g.w / 2 !== t.x + t.w / 2) out.push(`ゴースト ${g.key} が ${g.to} の真上に無い`)
  }
  return out
}

describe('layoutTree（スキルツリーの配置）', () => {
  it('段は依存の深さ、同じ段は定義順で左から', () => {
    const got = pos(layoutTree(goDomain, all))
    expect(got['go-01']).toEqual([0, 0])
    expect(got['go-02']).toEqual([1, 0])
    expect(got['go-03']).toEqual([2, 0])
    expect(got['go-04']).toEqual([2, 1])
    expect(got['go-05']).toEqual([3, 0])
    expect(got['go-06']).toEqual([4, 0])
  })

  it('段の数と、最も広い段の横の数を返す（ゴーストノードは段の横の数に入れない）', () => {
    const l = layoutTree(goDomain, all)
    expect(l.rows).toBe(5)
    expect(l.cols).toBe(2)
  })

  it('線は前提 → 派生の向きで、分野の中の依存とゴーストからの線を区別する', () => {
    const l = layoutTree(goDomain, all)
    expect(l.edges.map((e) => [e.from, e.to, e.ghost])).toEqual([
      ['go-01', 'go-02', false],
      ['go-02', 'go-03', false],
      ['go-02', 'go-04', false],
      ['go-03', 'go-05', false],
      ['react-03', 'go-05', true],
      ['go-05', 'go-06', false],
    ])
  })

  it('真下の項目へはまっすぐ下りる', () => {
    const l = layoutTree(goDomain, all)
    expect(edgeOf(l, 'go-01', 'go-02').points).toHaveLength(2)
    expect(edgeOf(l, 'go-05', 'go-06').points).toHaveLength(2)
  })

  it('1 つの四角から出る線は、下辺の別々の位置から出る', () => {
    const l = layoutTree(goDomain, all)
    const xs = l.edges.filter((e) => e.from === 'go-02').map((e) => e.points[0].x)
    expect(new Set(xs).size).toBe(2)
  })

  it('前提が 3 つの項目（実データの net/http の形）は、3 本とも合流点の横棒に入り、1 本で項目へ下りる', () => {
    const l = layoutTree(goReal, [goReal])
    const j = l.junctions.find((x) => x.to === 'nethttp')
    expect(j).toBeDefined()
    const ends = ['interfaces', 'errors', 'concurrency'].map((k) =>
      last(edgeOf(l, k, 'nethttp').points),
    )
    for (const p of ends) {
      expect(p.y).toBe(j?.y)
      expect(p.x).toBeGreaterThanOrEqual(j?.x1 as number)
      expect(p.x).toBeLessThanOrEqual(j?.x2 as number)
    }
    // 3 本は横棒の別々の位置に入る（1 本に重ならない）
    expect(new Set(ends.map((p) => p.x)).size).toBe(3)
    const t = nodeOf(l, 'nethttp')
    expect(j?.x).toBe(t.x + t.w / 2)
  })

  it('2 段以上離れた線（インターフェース → net/http）は、途中の段の四角を避けて通る', () => {
    const l = layoutTree(goReal, [goReal])
    const e = edgeOf(l, 'interfaces', 'nethttp')
    // 途中の段にある エラーハンドリング・並行処理 は 0 列目。線はその四角の外を縦に通る
    const between = ['errors', 'concurrency'].map((k) => nodeOf(l, k))
    const verticals = e.points
      .slice(1)
      .filter((p, i) => p.x === e.points[i].x && p.y !== e.points[i].y)
    expect(verticals.length).toBeGreaterThan(0)
    for (const b of between) {
      const x = verticals.find((p) => p.y > b.y)?.x as number
      expect(x <= b.x || x >= b.x + b.w).toBe(true)
    }
    expect(problems(l)).toEqual([])
  })

  it('他分野の前提は最上段に置かず、合流点の横に置く。ゴースト自身の依存は描かない', () => {
    const l = layoutTree(goDomain, all)
    expect(l.ghosts).toHaveLength(1)
    const g = l.ghosts[0]
    expect(g).toMatchObject({ key: 'react-03', to: 'go-05', domainKey: 'react' })
    // 最上段（go-01）より下、go-05 のすぐ上
    const top = nodeOf(l, 'go-01')
    const target = nodeOf(l, 'go-05')
    expect(g.y).toBeGreaterThan(top.y + top.h)
    expect(g.y + g.h).toBeLessThanOrEqual(target.y)
    // 横棒の高さに並び、横棒の外側にある
    const j = l.junctions.find((x) => x.to === 'go-05')
    expect(g.y + g.h / 2).toBe(j?.y)
    expect(g.x + g.w < (j?.x1 as number) || g.x > (j?.x2 as number)).toBe(true)
    // ゴーストノードへ入る線は無い
    expect(l.edges.filter((e) => e.to === 'react-03')).toEqual([])
    expect(l.nodes.map((n) => n.key)).not.toContain('react-03')
    expect(problems(l)).toEqual([])
  })

  it('同じ他分野の前提を 2 つの項目が指すと、それぞれの合流点の横に 1 つずつ置く', () => {
    const d: RoadmapDomain = {
      key: 'java',
      name: 'Java',
      items: [
        { key: 'j0', name: '' },
        { key: 'j1', name: '', dependsOn: ['j0', 'db-1'] },
        { key: 'j2', name: '', dependsOn: ['j0', 'db-1'] },
      ],
    }
    const l = layoutTree(d, [d, { key: 'db', name: 'DB', items: [{ key: 'db-1', name: '' }] }])
    expect(l.ghosts.map((g) => [g.key, g.to])).toEqual([
      ['db-1', 'j1'],
      ['db-1', 'j2'],
    ])
    expect(problems(l)).toEqual([])
  })

  it('前提が他分野の 1 つだけの項目は、ゴーストを真上に置き、合流点は置かない', () => {
    const d: RoadmapDomain = {
      key: 'x',
      name: '',
      items: [
        { key: 'a', name: '' },
        { key: 'b', name: '', dependsOn: ['react-03'] },
      ],
    }
    const l = layoutTree(d, [d, reactDomain])
    expect(l.junctions).toEqual([])
    const g = l.ghosts[0]
    const b = nodeOf(l, 'b')
    expect(pos(l)['b']).toEqual([1, 0])
    expect(g.x + g.w / 2).toBe(b.x + b.w / 2)
    expect(g.y + g.h).toBeLessThanOrEqual(b.y)
    expect(problems(l)).toEqual([])
  })

  it('他分野の前提が 2 つなら合流点の左右に、3 つ目からは上に積む', () => {
    const d: RoadmapDomain = {
      key: 'x',
      name: '',
      items: [
        { key: 'a', name: '' },
        { key: 'b', name: '', dependsOn: ['a', 'react-03', 'react-04', 'react-05'] },
      ],
    }
    const l = layoutTree(d, [d, reactDomain])
    const j = l.junctions[0]
    const [g3, g4, g5] = l.ghosts
    // 1 つ目と 2 つ目は横棒の高さで左右に分かれる
    expect(g3.y + g3.h / 2).toBe(j.y)
    expect(g4.y + g4.h / 2).toBe(j.y)
    expect(Math.sign(g3.x - j.x)).toBe(-Math.sign(g4.x - j.x))
    // 3 つ目はその上
    expect(g5.y + g5.h).toBeLessThan(g3.y)
    expect(problems(l)).toEqual([])
  })

  it('交差する 2 本の線と、他分野の前提を持つ項目が 3 つある形（実データの Java の形）', () => {
    const l = layoutTree(javaReal, [javaReal, dbDomain])
    expect(l.ghosts.map((g) => g.to)).toEqual(['tenant', 'request', 'testinfra'])
    expect(problems(l)).toEqual([])
  })

  it('ロードマップのどこにも無い前提は置かない', () => {
    const d: RoadmapDomain = {
      key: 'x',
      name: '',
      items: [{ key: 'a', name: '', dependsOn: ['nowhere'] }],
    }
    const l = layoutTree(d, [d])
    expect(l.nodes.map((n) => n.key)).toEqual(['a'])
    expect(l.ghosts).toEqual([])
    expect(l.edges).toEqual([])
  })

  it('循環参照があっても全項目を置いて落ちない（循環の辺は描かない）', () => {
    const d: RoadmapDomain = {
      key: 'x',
      name: '',
      items: [
        { key: 'a', name: '', dependsOn: ['b'] },
        { key: 'b', name: '', dependsOn: ['a'] },
        { key: 'c', name: '' },
      ],
    }
    const l = layoutTree(d, [d])
    expect(l.nodes.map((n) => n.key).sort()).toEqual(['a', 'b', 'c'])
    expect(problems(l)).toEqual([])
  })

  it('項目が無い分野でも落ちない', () => {
    const l = layoutTree({ key: 'e', name: '', items: [] }, [])
    expect(l).toEqual({
      nodes: [],
      ghosts: [],
      edges: [],
      junctions: [],
      rows: 0,
      cols: 0,
      width: 0,
      height: 0,
    })
  })

  describe('実データ（data/roadmap.json）の全分野で、線と四角の決まりが成り立つ', () => {
    const { domains } = loadData().roadmap
    it.each(domains.map((d) => [d.key, d] as const))('%s', (_, d) => {
      expect(problems(layoutTree(d, domains))).toEqual([])
    })
  })
})
