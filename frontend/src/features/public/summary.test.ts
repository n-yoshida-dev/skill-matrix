import { describe, expect, it } from 'vitest'
import type { AppData, ItemState } from '../../data/types'
import {
  domainOverview,
  isImplementedButUnverified,
  shortName,
  splitEvents,
  summarizeDomains,
  topEvidenced,
} from './summary'

// 公開ビューの集計が SPEC.md §1.1・§7.3 のとおりかを確かめる。
// 「何を保証しているか」：
// - 印の最上段と verifiedLevel の差で「実装済み・基礎未確認」を判定する
// - 「根拠のある項目」は印が 1 つでもある項目（verifiedLevel が 0 でも数える）
// - 1 層目の要約：分野の一番高い段、段の名前（段 0 で実装の印だけがあれば「実装済み・基礎未確認」）、
//   1 コマ＝1 項目の並び（段の高い順）、その分野の資格だけ
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
    retracted: [],
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
})

describe('1 層目の要約', () => {
  it('分野の一番高い段と段の名前、1 コマ＝1 項目を段の高い順に並べる', () => {
    const [go, react] = domainOverview(data)
    expect(go).toMatchObject({
      key: 'go',
      top: 3,
      topName: '資料を見れば実装できる',
      total: 3,
      withEvidence: 2,
    })
    // go-01 は段 3、go-02 は段 0 で実装の印だけ（枠だけのコマ）、go-03 は未着手
    expect(go.units).toEqual([
      { level: 3, implOnly: false },
      { level: 0, implOnly: true },
      { level: 0, implOnly: false },
    ])
    expect(react).toMatchObject({
      top: 1,
      topName: '基礎を確認した',
      units: [{ level: 1, implOnly: false }],
    })
  })

  it('段 0 の分野は、実装の印だけの項目があれば「実装済み・基礎未確認」、無ければ「未着手」', () => {
    const only = (evidenced: number[]): AppData => ({
      ...data,
      roadmap: {
        ...data.roadmap,
        domains: [{ key: 'db', name: 'DB', items: [{ key: 'db-01', name: 'x' }] }],
      },
      state: { ...data.state, items: [item('db-01', evidenced, 0)] },
    })
    expect(domainOverview(only([3]))[0]).toMatchObject({ top: 0, topName: '実装済み・基礎未確認' })
    expect(domainOverview(only([]))[0]).toMatchObject({ top: 0, topName: '未着手' })
  })

  it('資格はその分野の行にだけ添え、資格が無ければ空', () => {
    const withCerts: AppData = {
      ...data,
      certifications: [
        { domainKey: 'react', name: 'R 資格' },
        { domainKey: 'react', name: 'R 資格 2' },
      ],
    }
    const [go, react] = domainOverview(withCerts)
    expect(go.certifications).toEqual([])
    expect(react.certifications).toEqual(['R 資格', 'R 資格 2'])
    // certifications.json が無い（undefined）ときも崩れない
    expect(domainOverview(data)[1].certifications).toEqual([])
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
