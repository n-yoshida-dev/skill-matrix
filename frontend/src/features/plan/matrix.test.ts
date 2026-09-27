import { describe, expect, it } from 'vitest'
import type { ItemState, Level, RoadmapDomain } from '../../data/types'
import {
  DEFAULT_STALENESS,
  daysSince,
  emptyState,
  levelLine,
  missingLevels,
  needsAttention,
  rollupDomain,
  staleness,
  stalenessConfig,
  totalRollup,
} from './matrix'

// マトリクスの集計（matrix.ts）が、Go の参照実装（backend/internal/domain/summary.go）と同じ答えを返すことを確かめる。
// 入力と期待値は backend/internal/domain/summary_test.go の TestStaleness / TestNeedsAttention /
// TestRollupDomain / TestRollupDomain_PendingCount を写した（SPEC.md §5「同じ入力例で同じ答えになる」）。
// 「何を保証しているか」：
// - 鮮度は 30 日以内が新しい、90 日以内が古くなりつつある、それより前が古い。境界の日は新しい側に入る
// - 設定が無い・0 のときは既定の 30 日 / 90 日ではたらく
// - 要再確認は「不合格の報告」か「根拠が古い」のどちらか。根拠の無い未着手は要再確認ではない
// - 分野の集計は、state.json に無い項目を未着手として数え、範囲外のレベルでも落ちない
// - 「実装根拠あり・理解未確認」は印が verifiedLevel より上にある項目だけを数える
// - 吹き出しの 1 行目は「Verified n / 根拠の印 … / 未確認 …」

// Go のテストの now（2026-08-10）
const today = new Date(2026, 7, 10, 12, 0, 0)

/** today から n 日前の YYYY-MM-DD */
function daysBefore(n: number): string {
  const d = new Date(today)
  d.setDate(d.getDate() - n)
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

const state = (over: Partial<ItemState> & { itemKey: string }): ItemState => ({
  ...emptyState(over.itemKey),
  ...over,
})

/** 印から verifiedLevel を導いた状態（Go のテストの stateWith と同じ） */
function stateWith(itemKey: string, ...marks: Level[]): ItemState {
  let v = 0
  while (marks.includes((v + 1) as Level)) v++
  return state({ itemKey, verifiedLevel: v as Level, evidencedLevels: marks })
}

// Go の testRoadmap().Domains[0]（go-01, go-02 の 2 項目）
const domain: RoadmapDomain = {
  key: 'go',
  name: 'Go',
  items: [
    { key: 'go-01', name: '基本構文' },
    { key: 'go-02', name: 'エラー処理' },
  ],
}

describe('staleness（鮮度）', () => {
  it.each([
    ['根拠がまだ無い', null, 'unknown'],
    ['今日', daysBefore(0), 'fresh'],
    ['30日前（境界。fresh に含む）', daysBefore(30), 'fresh'],
    ['31日前', daysBefore(31), 'aging'],
    ['90日前（境界。aging に含む）', daysBefore(90), 'aging'],
    ['91日前', daysBefore(91), 'stale'],
    ['1年前', daysBefore(365), 'stale'],
  ] as const)('%s', (_name, last, want) => {
    expect(staleness(today, last, DEFAULT_STALENESS)).toBe(want)
  })

  it('設定が無い・0 のときは既定値で埋まる', () => {
    expect(stalenessConfig({})).toEqual(DEFAULT_STALENESS)
    const cfg = stalenessConfig({ staleness: { freshWithinDays: 0, agingWithinDays: 0 } })
    expect(staleness(today, daysBefore(100), cfg)).toBe('stale')
    expect(stalenessConfig({ staleness: { freshWithinDays: 7, agingWithinDays: 14 } })).toEqual({
      freshWithinDays: 7,
      agingWithinDays: 14,
    })
  })

  it('暦日で数えるので、時刻が変わっても同じ日なら同じ答え', () => {
    expect(daysSince(new Date(2026, 7, 10, 0, 0, 1), '2026-07-11')).toBe(30)
    expect(daysSince(new Date(2026, 7, 10, 23, 59, 59), '2026-07-11')).toBe(30)
  })

  it('日付の形が崩れていれば黙らずに例外にする', () => {
    expect(() => staleness(today, '2026/08/01', DEFAULT_STALENESS)).toThrow(/YYYY-MM-DD/)
  })
})

describe('needsAttention（要再確認）', () => {
  it.each([
    ['新しい根拠がある', { lastEvidenceAt: daysBefore(1) }, false],
    ['根拠が古い', { lastEvidenceAt: daysBefore(100) }, true],
    ['降格の提案があった', { lastEvidenceAt: daysBefore(0), needsReview: true }, true],
    ['未着手（根拠なし）は要再確認ではない', {}, false],
  ] as const)('%s', (_name, over, want) => {
    expect(needsAttention(state({ itemKey: 'x', ...over }), today, DEFAULT_STALENESS)).toBe(want)
  })
})

describe('rollupDomain（分野の集計）', () => {
  it('状態が無い項目は未着手として数える', () => {
    const got = rollupDomain(domain, new Map(), today, DEFAULT_STALENESS)
    expect(got.total).toBe(2)
    expect(got.byLevel[0]).toBe(2)
    expect(got.progress).toBe(0)
    expect(got.staleCount).toBe(0)
  })

  it('レベルごとの内訳と進捗率を出す', () => {
    const states = new Map([
      ['go-01', state({ itemKey: 'go-01', verifiedLevel: 2, lastEvidenceAt: daysBefore(0) })],
      ['go-02', state({ itemKey: 'go-02', verifiedLevel: 1, lastEvidenceAt: daysBefore(0) })],
    ])
    const got = rollupDomain(domain, states, today, DEFAULT_STALENESS)
    expect(got.byLevel).toEqual([0, 1, 1, 0, 0, 0])
    // (2 + 1) / (2項目 × 最大レベル5) = 0.3
    expect(got.progress).toBeCloseTo(0.3)
  })

  it('古い根拠と降格提案を要再確認として数える', () => {
    const states = new Map([
      ['go-01', state({ itemKey: 'go-01', verifiedLevel: 2, lastEvidenceAt: daysBefore(100) })],
      [
        'go-02',
        state({
          itemKey: 'go-02',
          verifiedLevel: 1,
          lastEvidenceAt: daysBefore(0),
          needsReview: true,
        }),
      ],
    ])
    expect(rollupDomain(domain, states, today, DEFAULT_STALENESS).staleCount).toBe(2)
  })

  it('保存値が範囲外でも落ちない', () => {
    const states = new Map([
      ['go-01', state({ itemKey: 'go-01', verifiedLevel: 99 as Level })],
      ['go-02', state({ itemKey: 'go-02', verifiedLevel: -5 as Level })],
    ])
    const got = rollupDomain(domain, states, today, DEFAULT_STALENESS)
    expect(got.byLevel[5]).toBe(1)
    expect(got.byLevel[0]).toBe(1)
  })

  it('「実装根拠あり・理解未確認」は印が verifiedLevel より上にある項目だけ', () => {
    const states = new Map([
      ['go-01', stateWith('go-01', 3)], // 実装の根拠だけ。表示は 0
      ['go-02', stateWith('go-02', 1, 2, 3)], // 途切れずに付いている
    ])
    const got = rollupDomain(domain, states, today, DEFAULT_STALENESS)
    expect(got.pendingCount).toBe(1)
    expect(got.byLevel[0]).toBe(1)
    expect(got.byLevel[3]).toBe(1)
  })
})

describe('totalRollup（合計行）', () => {
  it('件数を足し、進捗率は全項目で割り直す', () => {
    const a = rollupDomain(
      domain,
      new Map([['go-01', stateWith('go-01', 1, 2, 3)]]),
      today,
      DEFAULT_STALENESS,
    )
    const b = rollupDomain(
      { key: 'r', name: 'React', items: [{ key: 'r-01', name: 'state' }] },
      new Map([['r-01', stateWith('r-01', 3)]]),
      today,
      DEFAULT_STALENESS,
    )
    const got = totalRollup([a, b])
    expect(got.total).toBe(3)
    expect(got.byLevel).toEqual([2, 0, 0, 1, 0, 0])
    expect(got.pendingCount).toBe(1)
    // 3 / (3項目 × 5)
    expect(got.progress).toBeCloseTo(0.2)
  })
})

describe('missingLevels と levelLine（吹き出しの 1 行目）', () => {
  it.each([
    [[], [], 'Verified 0 / 根拠の印 なし / 未確認 なし'],
    [[1], [], 'Verified 1 / 根拠の印 1 / 未確認 なし'],
    [[3], [1, 2], 'Verified 0 / 根拠の印 3 / 未確認 1, 2'],
    [[1, 3, 4], [2], 'Verified 1 / 根拠の印 1, 3, 4 / 未確認 2'],
    [[1, 2, 3], [], 'Verified 3 / 根拠の印 1, 2, 3 / 未確認 なし'],
  ] as const)('印 %j', (marks, missing, line) => {
    const st = stateWith('x', ...marks)
    expect(missingLevels(st)).toEqual(missing)
    expect(levelLine(st)).toBe(line)
  })
})
