import { describe, expect, it } from 'vitest'
import type { AppData, ItemState } from '../../data/types'
import {
  isImplementedButUnverified,
  shortName,
  splitEvents,
  summarizeDomains,
  summarizeOverall,
  topEvidenced,
} from './summary'

// 公開ビューの集計が SPEC.md §1.1・§7.3 のとおりかを確かめる。
// 「何を保証しているか」：
// - 印の最上段と verifiedLevel の差で「実装済み・基礎未確認」を判定する
// - 「根拠のある項目」は印が 1 つでもある項目（verifiedLevel が 0 でも数える）
// - 「実装まで届いている」は verifiedLevel が 3 以上
// - タイルの短い名前は括弧の補足を外す

function item(key: string, evidenced: number[], verified: number): ItemState {
  return {
    itemKey: key,
    verifiedLevel: verified as ItemState['verifiedLevel'],
    evidencedLevels: evidenced as ItemState['evidencedLevels'],
    preState: 'none',
    needsReview: false,
    lastEvidenceAt: null,
    events: [],
  }
}

const data: AppData = {
  roadmap: {
    schemaVersion: 1,
    name: 'テスト',
    levels: [],
    domains: [
      {
        key: 'go',
        name: 'Go',
        goal: 'goal',
        items: [
          { key: 'go-01', name: 'a' },
          { key: 'go-02', name: 'b' },
          { key: 'go-03', name: 'c' },
        ],
      },
      { key: 'react', name: 'React', items: [{ key: 'react-01', name: 'd' }] },
    ],
  },
  state: {
    schemaVersion: 2,
    items: [
      item('go-01', [1, 2, 3], 3),
      item('go-02', [3], 0),
      item('go-03', [], 0),
      item('react-01', [1], 1),
    ],
    deferred: [],
    rejected: [],
  },
  settings: {},
}

describe('印と verifiedLevel の読み取り', () => {
  it('印の最上段を返し、印が無ければ 0', () => {
    expect(topEvidenced(item('x', [1, 3], 1))).toBe(3)
    expect(topEvidenced(item('x', [], 0))).toBe(0)
  })

  it('[3] だけの項目は「実装済み・基礎未確認」', () => {
    expect(isImplementedButUnverified(item('x', [3], 0))).toBe(true)
    expect(isImplementedButUnverified(item('x', [1, 2, 3], 3))).toBe(false)
    expect(isImplementedButUnverified(item('x', [], 0))).toBe(false)
  })
})

describe('集計', () => {
  it('分野ごとに「根拠のある項目」を数える。verifiedLevel が 0 でも印があれば数える', () => {
    const s = summarizeDomains(data)
    expect(s[0]).toMatchObject({ key: 'go', total: 3, withEvidence: 2 })
    expect(s[1]).toMatchObject({ key: 'react', total: 1, withEvidence: 1 })
  })

  it('全体の数。実装まで届いているのは verifiedLevel 3 以上だけ', () => {
    expect(summarizeOverall(data)).toEqual({
      domains: 2,
      items: 4,
      withEvidence: 3,
      canImplement: 1,
    })
  })
})

describe('表示用の整形', () => {
  it('括弧の補足を外す', () => {
    expect(shortName('基本構文（変数・型・関数）')).toBe('基本構文')
    expect(shortName('テスト')).toBe('テスト')
  })

  it('印を付けた判定は新しい順、付けなかった判定は別に分ける', () => {
    const st = item('x', [1, 3], 1)
    const base = {
      file: 'f',
      source: 'ai' as const,
      proposedLevel: 1 as const,
      evidenceRefs: ['repo:a/b@c'],
      rationale: 'r',
      confidence: 0.9,
      violations: [],
    }
    st.events = [
      { ...base, index: 0, occurredAt: '2026-01-01', evidenceType: 'drill', marked: [1] },
      { ...base, index: 1, occurredAt: '2026-02-01', evidenceType: 'explained_to', marked: [] },
      { ...base, index: 2, occurredAt: '2026-03-01', evidenceType: 'implementation', marked: [3] },
    ]
    const { marked, unmarked } = splitEvents(st)
    expect(marked.map((e) => e.occurredAt)).toEqual(['2026-03-01', '2026-01-01'])
    expect(unmarked.map((e) => e.evidenceType)).toEqual(['explained_to'])
  })
})
