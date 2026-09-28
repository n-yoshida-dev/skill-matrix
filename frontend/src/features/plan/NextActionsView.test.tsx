import { render as rtlRender, screen, within } from '@testing-library/react'
import type { ReactElement } from 'react'
import { MemoryRouter } from 'react-router'
import { describe, expect, it } from 'vitest'
import type { AppData, ItemState } from '../../data/types'
import { NextActionsView } from './NextActionsView'

/** 項目名が項目詳細へのリンクなので、ルーターの中で描く */
const render = (ui: ReactElement) => rtlRender(<MemoryRouter>{ui}</MemoryRouter>)

// 作業ビューの「次にやること」Top N（SPEC.md §7.4、TODO 2-5 の完了条件）を確かめる。
// data/*.json ではなく、このファイルの中の小さなデータで確かめる（data/ が更新されても落ちないように）。
// 「何を保証しているか」：
// - 印が表示レベルより上にある項目（[3] など）は「既存の実装について L1 / L2 を短いドリル・自己説明で確認する」と出し、
//   次の確認方法（verifyBy）はその下に添える。「基礎からやり直し」とは見せない
// - それ以外の項目は、次の確認方法をそのまま「何をするか」として出す
// - どの項目にも「身につくと」（outcome）を並べ、空なら「未記入」と出す
// - 済んだ項目は「深掘り（L1 → L2）」と分類し、学習パスの「今ここ」と違う問いだと分かるようにする
// - 根拠が古い項目は、古いことを添える
// - 件数は settings.json の nextActions.limit に従う

const item = (over: Partial<ItemState> & { itemKey: string }): ItemState => ({
  verifiedLevel: 0,
  evidencedLevels: [],
  preState: 'none',
  needsReview: false,
  lastEvidenceAt: null,
  events: [],
  ...over,
})

const today = new Date(2026, 8, 28)

function makeData(limit?: number): AppData {
  return {
    roadmap: {
      schemaVersion: 1,
      name: 'テスト用',
      levels: [],
      domains: [
        {
          key: 'db',
          name: 'データベース',
          items: [
            {
              key: 'db-schema',
              name: 'スキーマ設計',
              outcome: 'テナントの混入を防ぐ制約を説明できる',
              verifyBy: '複合外部キーが何を防ぐかを具体例で答える（L1・L2）',
            },
          ],
        },
        {
          key: 'go',
          name: 'Go',
          items: [
            {
              key: 'go-syntax',
              name: '基本構文',
              outcome: '型と値の流れを追える',
              verifyBy: 'ゼロ値がある理由を自分の言葉で説明する（L2）',
            },
            {
              key: 'go-methods',
              name: 'メソッド',
              dependsOn: ['go-syntax', 'go-old'],
              verifyBy: 'レシーバのドリル（L1）',
            },
            { key: 'go-old', name: '古い項目', verifyBy: '再出題（L2）' },
            { key: 'go-errors', name: 'エラー処理', dependsOn: ['go-methods'] },
          ],
        },
      ],
    },
    state: {
      schemaVersion: 2,
      items: [
        // 実装の根拠だけがあり、基礎の確認が未了（SPEC.md §1.1 の [3]）
        item({ itemKey: 'db-schema', evidencedLevels: [3], lastEvidenceAt: '2026-09-27' }),
        item({
          itemKey: 'go-syntax',
          verifiedLevel: 1,
          evidencedLevels: [1],
          lastEvidenceAt: '2026-09-20',
        }),
        // 最終根拠が 120 日以上前（古い）
        item({
          itemKey: 'go-old',
          verifiedLevel: 1,
          evidencedLevels: [1],
          lastEvidenceAt: '2026-05-01',
        }),
      ],
      deferred: [],
      rejected: [],
    },
    settings: limit === undefined ? {} : { nextActions: { limit } },
  }
}

/** 項目キーで「次にやること」の 1 件を探す */
function entry(key: string): HTMLElement {
  const li = screen.getByText(key).closest('li')
  if (!li) throw new Error(`${key} の行が無い`)
  return li
}

describe('次にやること', () => {
  it('実装の根拠だけがある項目は、既存の実装について基礎を確認する文言にし、確認方法を下に添える', () => {
    render(<NextActionsView data={makeData()} today={today} />)
    const row = within(entry('db-schema'))
    expect(row.getByText('基礎の確認')).toBeInTheDocument()
    expect(
      row.getByText('既存の実装について L1 / L2 を短いドリル・自己説明で確認する'),
    ).toBeInTheDocument()
    expect(
      row.getByText('確認方法：複合外部キーが何を防ぐかを具体例で答える（L1・L2）'),
    ).toBeInTheDocument()
    expect(row.queryByText(/やり直し/)).toBeNull()
  })

  it('済んだ項目は深掘りと分類し、次の確認方法をそのまま出す', () => {
    render(<NextActionsView data={makeData()} today={today} />)
    const row = within(entry('go-syntax'))
    expect(row.getByText('深掘り（L1 → L2）')).toBeInTheDocument()
    expect(row.getByText('ゼロ値がある理由を自分の言葉で説明する（L2）')).toBeInTheDocument()
    expect(row.getByText('身につくと：型と値の流れを追える')).toBeInTheDocument()
  })

  it('項目名を押すと項目詳細へ移れる', () => {
    render(<NextActionsView data={makeData()} today={today} />)
    expect(within(entry('go-syntax')).getByRole('link', { name: '基本構文' })).toHaveAttribute(
      'href',
      '/plan/item/go-syntax',
    )
  })

  it('到達状態が空なら未記入と出す', () => {
    render(<NextActionsView data={makeData()} today={today} />)
    expect(within(entry('go-methods')).getByText('未記入')).toBeInTheDocument()
  })

  it('根拠が古い項目は要再確認として、何日前かを添える', () => {
    render(<NextActionsView data={makeData()} today={today} />)
    expect(
      within(entry('go-old')).getByText('要再確認：最終根拠が 150 日前（古い）'),
    ).toBeInTheDocument()
  })

  it('根拠が新しい項目には、古さを何も添えない', () => {
    render(<NextActionsView data={makeData()} today={today} />)
    // go-syntax の最終根拠は 8 日前
    expect(within(entry('go-syntax')).queryByText(/最終根拠/)).toBeNull()
  })

  it('古くなりつつある項目は、要再確認とは書かずに何日前かだけを添える', () => {
    const data = makeData()
    data.state.items[1] = { ...data.state.items[1], lastEvidenceAt: '2026-07-01' }
    render(<NextActionsView data={data} today={today} />)
    const row = within(entry('go-syntax'))
    expect(
      row.getByText('最終根拠が 89 日前（古くなりつつある。90 日を超えると要再確認）'),
    ).toBeInTheDocument()
    expect(row.queryByText(/要再確認：/)).toBeNull()
  })

  it('着手できない項目は後ろに並び、前提が未達と出る', () => {
    render(<NextActionsView data={makeData(10)} today={today} />)
    const items = screen.getAllByRole('listitem')
    const last = within(items[items.length - 1])
    expect(last.getByText('go-errors')).toBeInTheDocument()
    expect(last.getByText('前提が未達')).toBeInTheDocument()
  })

  it('件数は settings.json の nextActions.limit に従う', () => {
    render(<NextActionsView data={makeData(2)} today={today} />)
    expect(screen.getAllByRole('listitem')).toHaveLength(2)
    expect(screen.getByText(/優先度の高い順に 2 件/)).toBeInTheDocument()
  })
})
