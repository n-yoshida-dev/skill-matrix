import { describe, expect, it } from 'vitest'
import { loadData } from '../../data/load'
import type { ItemState, Level, RoadmapDomain } from '../../data/types'
import { isReady } from '../plan/next'
import { layoutTree } from './layout'
import { isJunctionOpen, isLit, junctionCount, junctionLabel, missingPrereqs } from './unlock'

// スキルツリーで解放の条件を読ませる計算（unlock.ts。SPEC.md §7.2）を確かめる。
// 「何を保証しているか」：
// - 線に色が付くのは、前提（線の出どころ）が確認済み（verifiedLevel 1 以上）のときだけ。行き先の状態は見ない
// - 他分野の前提（ゴーストノード）からの線も、同じ決まりで色が付く
// - 合流点の x は色の付いた線の本数と一致し、n は項目に入る線の本数。文言は「n つとも必要 x/n」
// - 合流点から項目へ下りる線に色が付くのは、前提がすべて確認済み（x = n）のときだけで、それは「着手できる」と同じ判定
// - 鍵の項目の「必要：」には、確認済みでない直接の前提だけが dependsOn の順に出る（他分野の前提も含む）
// - 実データでも、合流点の x と色の付いた線の本数が一致する

const st = (itemKey: string, verifiedLevel: Level): ItemState => ({
  itemKey,
  verifiedLevel,
  evidencedLevels: verifiedLevel > 0 ? [verifiedLevel] : [],
  preState: 'none',
  needsReview: false,
  lastEvidenceAt: null,
  events: [],
})

// 並行処理は エラーハンドリング（L0）と 関数値（L1）と 他分野の fetch（L2）が前提
const go: RoadmapDomain = {
  key: 'go',
  name: 'Go',
  items: [
    { key: 'errors', name: 'エラーハンドリング' },
    { key: 'closure', name: '関数値・クロージャ' },
    { key: 'concurrency', name: '並行処理', dependsOn: ['errors', 'closure', 'react-fetch'] },
    { key: 'nethttp', name: 'net/http', dependsOn: ['concurrency'] },
  ],
}
const react: RoadmapDomain = {
  key: 'react',
  name: 'React',
  items: [{ key: 'react-fetch', name: 'fetch' }],
}
const states = new Map(
  [st('errors', 0), st('closure', 1), st('react-fetch', 2)].map((s) => [s.itemKey, s]),
)

describe('線の色', () => {
  it('前提が確認済み（レベル 1 以上）の線だけに色が付く。行き先が前提待ちでも付く', () => {
    const l = layoutTree(go, [go, react])
    const lit = l.edges.filter((e) => isLit(e, states)).map((e) => e.from)
    // closure（L1）と他分野の react-fetch（L2）だけ。行き先の concurrency は前提待ち
    expect(lit).toEqual(['closure', 'react-fetch'])
  })

  it('理解度の記録が無い項目からの線には色が付かない', () => {
    expect(isLit({ from: 'unknown' }, states)).toBe(false)
  })
})

describe('合流点の数', () => {
  it('x は色の付いた線の本数、n は入る線の本数。文言は「n つとも必要 x/n」', () => {
    const l = layoutTree(go, [go, react])
    const count = junctionCount('concurrency', l.edges, states)
    expect(count).toEqual({ n: 3, x: 2 })
    expect(junctionLabel(count)).toBe('3 つとも必要 2/3')
    expect(junctionLabel({ n: 2, x: 0 })).toBe('2 つとも必要 0/2')
  })

  it('項目へ下りる線に色が付くのは、前提がすべて確認済みのときだけ。それは着手できるかと同じ判定', () => {
    const l = layoutTree(go, [go, react])
    const item = go.items[2]
    expect(isJunctionOpen(junctionCount('concurrency', l.edges, states))).toBe(false)
    expect(isReady(states, item)).toBe(false)
    const all = new Map(states).set('errors', st('errors', 1))
    expect(isJunctionOpen(junctionCount('concurrency', l.edges, all))).toBe(true)
    expect(isReady(all, item)).toBe(true)
  })

  it('実データの全分野で、合流点の x と色の付いた線の本数が一致し、開くのは着手できる項目だけ', () => {
    const data = loadData()
    const real = new Map(data.state.items.map((s) => [s.itemKey, s]))
    for (const d of data.roadmap.domains) {
      const l = layoutTree(d, data.roadmap.domains)
      for (const j of l.junctions) {
        const ins = l.edges.filter((e) => e.to === j.to)
        const count = junctionCount(j.to, l.edges, real)
        expect(count.x).toBe(ins.filter((e) => isLit(e, real)).length)
        const item = d.items.find((it) => it.key === j.to)
        if (item) expect(isJunctionOpen(count)).toBe(isReady(real, item))
      }
    }
  })
})

describe('鍵の項目の「必要：」', () => {
  it('確認済みでない直接の前提だけを、dependsOn の順に出す', () => {
    expect(missingPrereqs(go.items[2], states)).toEqual(['errors'])
    const none = new Map<string, ItemState>()
    expect(missingPrereqs(go.items[2], none)).toEqual(['errors', 'closure', 'react-fetch'])
  })

  it('上流の前提（前提の前提）は出さない', () => {
    // nethttp の直接の前提は concurrency だけ。その前提の errors は出さない
    expect(missingPrereqs(go.items[3], states)).toEqual(['concurrency'])
  })
})
