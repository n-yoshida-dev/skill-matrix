import { describe, expect, it } from 'vitest'
import type { ItemState, Level, Roadmap, RoadmapDomain } from '../../data/types'
import { DEFAULT_STALENESS, emptyState } from '../plan/matrix'
import { DEFAULT_WEIGHTS, nextActions } from '../plan/next'
import { buildPath, progressGauge, topoSort } from './path'

// 学習パス（path.ts）が、Go の参照実装（backend/internal/domain/plan.go の BuildPath）と同じ答えを返すことを確かめる。
// 入力と期待値は backend/internal/domain/plan_test.go の TestBuildPath ほかを写した（SPEC.md §5）。
// 「何を保証しているか」：
// - 定義順がばらばらでも、前提が先に来るよう依存関係の順に並べる（同順位は定義順）
// - 済（レベル 1 以上）・今ここ・この先に分ける。今ここは 1 件だけで、まだ済んでいない着手できる項目
// - 全部済なら今ここは無い
// - 分野の目標を持ち回る
// - 分野の外を指す依存は並び順に影響せず、externalDeps に残る
// - 循環参照があっても全項目を返して落ちない
// - 上部のゲージは 1 コマ＝分野の 1 項目で、レベル 1 以上のコマを左から塗り、「基礎確認 x/n 項目」と添える。% は出さない。他分野の前提は数えない
// - 学習パスの「今ここ」と「次にやること」の 1 位は別の問いに答えるので一致しないことがある

/** Go の planRoadmap()：go-01 ── go-02 ── go-03、go-01 ── go-04、react-01（依存なし） */
const planRoadmap: Roadmap = {
  schemaVersion: 1,
  name: 'テスト用',
  levels: [],
  domains: [
    {
      key: 'go',
      name: 'Go',
      goal: 'Go で Web API を書ける',
      items: [
        { key: 'go-01', name: '基本構文' },
        { key: 'go-02', name: 'インターフェース', dependsOn: ['go-01'] },
        { key: 'go-03', name: 'エラー処理', dependsOn: ['go-02'] },
        { key: 'go-04', name: '並行処理', dependsOn: ['go-01'] },
      ],
    },
    { key: 'react', name: 'React', items: [{ key: 'react-01', name: 'コンポーネント' }] },
  ],
}
const goDomain = planRoadmap.domains[0]

const at = (itemKey: string, verifiedLevel: Level): ItemState => ({
  ...emptyState(itemKey),
  verifiedLevel,
  evidencedLevels: verifiedLevel > 0 ? [1] : [],
  lastEvidenceAt: '2026-08-11',
})
const states = (...list: ItemState[]) => new Map(list.map((s) => [s.itemKey, s]))
const keys = (d: RoadmapDomain) => d.items.map((it) => it.key)

describe('buildPath（学習パス）', () => {
  it('定義順がばらばらでも依存関係の順に並べる', () => {
    const d: RoadmapDomain = {
      key: 'go',
      name: 'Go',
      items: [
        { key: 'go-03', name: '', dependsOn: ['go-02'] },
        { key: 'go-01', name: '' },
        { key: 'go-02', name: '', dependsOn: ['go-01'] },
      ],
    }
    expect(buildPath(d, new Map()).nodes.map((n) => n.item.key)).toEqual([
      'go-01',
      'go-02',
      'go-03',
    ])
  })

  it('済・今ここ・この先に分類する', () => {
    const got = buildPath(goDomain, states(at('go-01', 2)))
    const status = Object.fromEntries(got.nodes.map((n) => [n.item.key, n.status]))
    expect(status['go-01']).toBe('done')
    expect(status['go-03']).toBe('upcoming') // go-02 が未達なので着手できない
    expect(got.nodes.filter((n) => n.status === 'current')).toHaveLength(1)
    expect(got.doneCount).toBe(1)
    expect(got.totalCount).toBe(4)
  })

  it('並びと分類が Go と同じになる', () => {
    // go-01 済 → 依存順は go-01, go-02, go-03, go-04（同順位の go-02 と go-04 は定義順）。
    // 今ここは依存順で最初の「未達かつ着手できる」go-02
    const got = buildPath(goDomain, states(at('go-01', 1)))
    expect(got.nodes.map((n) => `${n.item.key}:${n.status}`)).toEqual([
      'go-01:done',
      'go-02:current',
      'go-03:upcoming',
      'go-04:upcoming',
    ])
  })

  it('全部済なら今ここは無い', () => {
    const got = buildPath(goDomain, states(...keys(goDomain).map((k) => at(k, 1))))
    expect(got.nodes.every((n) => n.status === 'done')).toBe(true)
    expect(got.doneCount).toBe(4)
  })

  it('分野の目標を持ち回る', () => {
    expect(buildPath(goDomain, new Map()).goal).toBe('Go で Web API を書ける')
  })

  it('分野の外を指す依存は並び順に影響せず、externalDeps に残る', () => {
    const d: RoadmapDomain = {
      key: 'go',
      name: 'Go',
      items: [
        { key: 'go-01', name: '', dependsOn: ['react-01'] }, // 分野外
        { key: 'go-02', name: '', dependsOn: ['go-01'] },
      ],
    }
    const got = buildPath(d, new Map())
    expect(got.nodes.map((n) => n.item.key)).toEqual(['go-01', 'go-02'])
    expect(got.nodes[0].externalDeps).toEqual(['react-01'])
    expect(got.nodes[1].externalDeps).toEqual([])
    // 分野の外の前提がレベル 0 なので、go-01 はまだ着手できない
    expect(got.nodes[0].ready).toBe(false)
  })

  it('循環参照があっても全項目を返して落ちない', () => {
    const got = topoSort([
      { key: 'a', name: '', dependsOn: ['b'] },
      { key: 'b', name: '', dependsOn: ['a'] },
      { key: 'c', name: '' },
    ])
    expect(got.map((it) => it.key)).toEqual(['c', 'a', 'b'])
  })

  it('今ここは常に、まだ済んでいない着手できる項目', () => {
    const got = buildPath(goDomain, states(at('go-01', 1), at('go-02', 1)))
    const current = got.nodes.find((n) => n.status === 'current')
    expect(current?.item.key).toBe('go-03')
    expect(current?.state.verifiedLevel).toBe(0)
    expect(current?.ready).toBe(true)
  })
})

// Go の TestBuildPathとNextActionsは別の問いに答える
it('学習パスの「今ここ」と「次にやること」の 1 位は別の問いに答える', () => {
  const st = states(at('go-01', 1))
  const today = new Date(2026, 7, 11)
  const topInGo = nextActions(planRoadmap, st, today, DEFAULT_STALENESS, DEFAULT_WEIGHTS, 10).find(
    (a) => a.domainKey === 'go' && !a.blocked,
  )?.itemKey
  const current = buildPath(goDomain, st).nodes.find((n) => n.status === 'current')?.item.key
  // go-01 はレベル 1 に達したので、パス上の先端は go-02 へ進む
  expect(current).toBe('go-02')
  // 一方で「次にやること」は深掘りを含むので、済んだ go-01 を 1 位に挙げうる
  expect(topInGo).toBe('go-01')
})

describe('進捗ゲージ', () => {
  it('1 コマ＝分野の 1 項目で、レベル 1 以上の数だけ左から塗り、「基礎確認 x/n 項目」と添える', () => {
    // go-01 がレベル 2、go-04 がレベル 1。go-02・go-03 はレベル 0
    const got = progressGauge(buildPath(goDomain, states(at('go-01', 2), at('go-04', 1))))
    expect(got.cells).toEqual([true, true, false, false])
    expect(got.label).toBe('基礎確認 2/4 項目')
    expect(got.label).not.toMatch(/%/)
  })

  it('他分野の前提はコマに数えない', () => {
    // go-02 は分野の外の react-01 を前提に持つ。react-01 がレベル 1 でも、コマは分野の 2 項目だけ
    const d: RoadmapDomain = {
      key: 'go',
      name: 'Go',
      items: [
        { key: 'go-01', name: '基本構文' },
        { key: 'go-02', name: 'HTTP', dependsOn: ['go-01', 'react-01'] },
      ],
    }
    const got = progressGauge(buildPath(d, states(at('react-01', 1), at('go-01', 1))))
    expect(got.cells).toEqual([true, false])
    expect(got.label).toBe('基礎確認 1/2 項目')
  })

  it('項目が無い分野はコマも無い', () => {
    expect(progressGauge({ doneCount: 0, totalCount: 0 })).toEqual({
      cells: [],
      label: '基礎確認 0/0 項目',
    })
  })
})
