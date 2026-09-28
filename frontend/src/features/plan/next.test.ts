import { describe, expect, it } from 'vitest'
import type { ItemState, Level, Roadmap } from '../../data/types'
import { DEFAULT_STALENESS, emptyState } from './matrix'
import {
  DEFAULT_WEIGHTS,
  isReady,
  nextActions,
  nextLimitFrom,
  pendingLevels,
  weightsFrom,
} from './next'

// 「次にやること」（next.ts）が、Go の参照実装（backend/internal/domain/plan.go の NextActions）と同じ答えを返すことを確かめる。
// 入力と期待値は backend/internal/domain/plan_test.go の TestNextActions / TestNextActions_PendingLevels を写した（SPEC.md §5）。
// 「何を保証しているか」：
// - 着手できない項目は、着手できる項目より必ず後ろに来る
// - 後に続く項目を多く開く項目が先に来る
// - 件数を n 件に絞り、0 件なら空
// - 到達状態と次の確認方法を必ず載せる
// - レベル 5 の項目は出さないが、要再確認なら出す
// - 前提がレベル 1 に達すると、その項目は着手できるようになる
// - 印はあるがレベルに届いていない段（[3] など）を pendingLevels として持つ
// - 重みと件数は settings.json から読み、書かれていないものだけ既定値にする

// Go のテストの now（2026-08-11）
const today = new Date(2026, 7, 11, 12, 0, 0)

/** today から n 日前の YYYY-MM-DD */
function daysBefore(n: number): string {
  const d = new Date(today)
  d.setDate(d.getDate() - n)
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

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
        {
          key: 'go-01',
          name: '基本構文',
          outcome: '型と値の流れを追える',
          verifyBy: 'Tour Basics 後にドリル',
        },
        { key: 'go-02', name: 'インターフェース', dependsOn: ['go-01'] },
        { key: 'go-03', name: 'エラー処理', dependsOn: ['go-02'] },
        { key: 'go-04', name: '並行処理', dependsOn: ['go-01'] },
      ],
    },
    { key: 'react', name: 'React', items: [{ key: 'react-01', name: 'コンポーネント' }] },
  ],
}

const cfg = DEFAULT_STALENESS
const w = DEFAULT_WEIGHTS

const states = (...list: ItemState[]) => new Map(list.map((s) => [s.itemKey, s]))

/** レベルと最終根拠日だけを決めた状態 */
const at = (itemKey: string, verifiedLevel: Level, lastEvidenceAt: string | null): ItemState => ({
  ...emptyState(itemKey),
  verifiedLevel,
  lastEvidenceAt,
})

const keys = (list: { itemKey: string }[]) => list.map((a) => a.itemKey)

describe('nextActions（次にやること）', () => {
  it('着手できない項目は着手できる項目より必ず後ろに来る', () => {
    const got = nextActions(planRoadmap, new Map(), today, cfg, w, 10)
    const firstBlocked = got.findIndex((a) => a.blocked)
    expect(firstBlocked).toBeGreaterThan(0)
    expect(got.slice(firstBlocked).every((a) => a.blocked)).toBe(true)
    // 未着手の状態で着手できるのは go-01 と react-01 だけ
    expect(keys(got.filter((a) => !a.blocked))).toEqual(['go-01', 'react-01'])
  })

  it('道を多く開く項目が先に来る', () => {
    // go-01 は go-02 と go-04 を開く。react-01 は何も開かない
    expect(keys(nextActions(planRoadmap, new Map(), today, cfg, w, 2))[0]).toBe('go-01')
  })

  it('件数を n 件に絞る', () => {
    expect(nextActions(planRoadmap, new Map(), today, cfg, w, 2)).toHaveLength(2)
    expect(nextActions(planRoadmap, new Map(), today, cfg, w, 0)).toEqual([])
  })

  it('到達状態と次の確認方法を必ず載せる', () => {
    const [top] = nextActions(planRoadmap, new Map(), today, cfg, w, 1)
    expect(top.outcome).toBe('型と値の流れを追える')
    expect(top.verifyBy).toBe('Tour Basics 後にドリル')
    expect(top.domainName).toBe('Go')
  })

  it('最大レベルに達した項目は出さない', () => {
    const got = nextActions(planRoadmap, states(at('go-01', 5, daysBefore(0))), today, cfg, w, 10)
    expect(keys(got)).not.toContain('go-01')
  })

  it('最大レベルでも要再確認なら出す', () => {
    const got = nextActions(planRoadmap, states(at('go-01', 5, daysBefore(200))), today, cfg, w, 10)
    const go01 = got.find((a) => a.itemKey === 'go-01')
    expect(go01?.staleness).toBe('stale')
  })

  it('依存が満たされると着手可能になる', () => {
    const got = nextActions(planRoadmap, states(at('go-01', 1, daysBefore(0))), today, cfg, w, 10)
    expect(got.find((a) => a.itemKey === 'go-02')?.blocked).toBe(false)
    expect(got.find((a) => a.itemKey === 'go-03')?.blocked).toBe(true)
  })

  it('印はあるがレベルに届いていない段を pendingLevels に持つ（Go の TestNextActions_PendingLevels）', () => {
    // ガイドなしの実装まであるが、基礎の確認が未了（印 [3, 4]・レベル 0）
    const go01: ItemState = { ...at('go-01', 0, daysBefore(0)), evidencedLevels: [3, 4] }
    const got = nextActions(planRoadmap, states(go01), today, cfg, w, 5)
    const a = got.find((x) => x.itemKey === 'go-01')
    expect(a?.pendingLevels).toEqual([3, 4])
    expect(a?.level).toBe(0)
    expect(got.find((x) => x.itemKey === 'react-01')?.pendingLevels).toEqual([])
  })

  it('並びが Go と同じになる（未着手の全件）', () => {
    // Go の NextActions(planRoadmap, nil, …, 10) の並び。
    // go-01（着手可・2 項目を開く）> react-01（着手可）> 着手不能のうち go-02（1 項目を開く）> go-03 > go-04
    expect(keys(nextActions(planRoadmap, new Map(), today, cfg, w, 10))).toEqual([
      'go-01',
      'react-01',
      'go-02',
      'go-03',
      'go-04',
    ])
  })
})

describe('isReady・pendingLevels', () => {
  it('分野の外の前提も見る', () => {
    const item = { key: 'x', name: 'x', dependsOn: ['react-01'] }
    expect(isReady(new Map(), item)).toBe(false)
    expect(isReady(states(at('react-01', 1, daysBefore(0))), item)).toBe(true)
  })

  it('印が表示レベル以下なら pendingLevels は空', () => {
    expect(pendingLevels({ ...at('a', 2, null), evidencedLevels: [1, 2] })).toEqual([])
    expect(pendingLevels({ ...at('a', 1, null), evidencedLevels: [1, 3] })).toEqual([3])
  })
})

describe('weightsFrom・nextLimitFrom（設定の読み込み）', () => {
  it('書かれていない値だけ既定値で埋め、0 は 0 として使う', () => {
    expect(weightsFrom({})).toEqual(DEFAULT_WEIGHTS)
    expect(weightsFrom({ weights: { gap: 0, unlocks: 2 } })).toEqual({
      readiness: 1.0,
      gap: 0,
      staleness: 0.3,
      unlocks: 2,
    })
    expect(nextLimitFrom({})).toBe(5)
    expect(nextLimitFrom({ nextActions: { limit: 3 } })).toBe(3)
  })
})
