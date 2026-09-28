import { render as rtlRender, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactElement } from 'react'
import { MemoryRouter } from 'react-router'
import { describe, expect, it } from 'vitest'
import type { AppData, ItemState } from '../../data/types'
import { PlanView } from './PlanView'

/** 作業ビューは中で画面を切り替えるので、ルーターの中で描く（初期の画面はダッシュボード） */
const render = (ui: ReactElement) => rtlRender(<MemoryRouter>{ui}</MemoryRouter>)

// 作業ビューのマトリクス（SPEC.md §7.1、TODO 2-5 の完了条件）を確かめる。
// data/*.json ではなく、このファイルの中の小さなデータで確かめる（data/ が更新されても落ちないように）。
// 「何を保証しているか」：
// - 升目の塗りは verifiedLevel（印が [3] だけなら塗らない）
// - 印の最上段が verifiedLevel より上の升目にだけ、右上の三角が付く
// - 升目に乗せると吹き出しに「Verified n / 根拠の印 … / 未確認 …」と段階前の状態・最終根拠日が出る
// - 不合格の報告があった項目と、根拠が古い項目に枠線が付く
// - サマリー帯に分野ごとの L0〜L5 の件数と「実装根拠あり・理解未確認」の列がある
// - 押すと吹き出しが固定され、もう一度押すと閉じる

const item = (over: Partial<ItemState> & { itemKey: string }): ItemState => ({
  verifiedLevel: 0,
  evidencedLevels: [],
  preState: 'none',
  needsReview: false,
  lastEvidenceAt: null,
  events: [],
  ...over,
})

// 基準日。go-old の最終根拠（2026-05-01）はここから 120 日前で「古い」
const today = new Date(2026, 7, 29)

const data: AppData = {
  roadmap: {
    schemaVersion: 1,
    name: 'テスト用',
    levels: [
      { level: 1, name: '基礎理解を確認', criteria: '' },
      { level: 2, name: '自分で説明可能', criteria: '' },
      { level: 3, name: 'ガイド付き実装', criteria: '' },
      { level: 4, name: '自力実装・レビュー', criteria: '' },
      { level: 5, name: '別文脈へ応用', criteria: '' },
    ],
    domains: [
      {
        key: 'go',
        name: 'Go',
        items: [
          { key: 'go-syntax', name: '基本構文' },
          { key: 'go-http', name: 'HTTP サーバ' },
          { key: 'go-old', name: '古い項目' },
        ],
      },
      {
        key: 'java',
        name: 'Java',
        items: [
          { key: 'java-rest', name: 'REST API' },
          { key: 'java-test', name: 'テスト' },
        ],
      },
    ],
  },
  state: {
    schemaVersion: 2,
    items: [
      item({
        itemKey: 'go-syntax',
        verifiedLevel: 2,
        evidencedLevels: [1, 2],
        lastEvidenceAt: '2026-08-20',
      }),
      // 実装の根拠だけがあり、基礎の確認が未了（SPEC.md §1.1 の [3]）
      item({ itemKey: 'go-http', evidencedLevels: [3], lastEvidenceAt: '2026-08-01' }),
      item({
        itemKey: 'go-old',
        verifiedLevel: 1,
        evidencedLevels: [1],
        lastEvidenceAt: '2026-05-01',
      }),
      item({
        itemKey: 'java-rest',
        verifiedLevel: 3,
        evidencedLevels: [1, 2, 3],
        lastEvidenceAt: '2026-08-23',
      }),
      // 不合格の報告があり、印は無い。段階前の状態だけがある
      item({ itemKey: 'java-test', needsReview: true, preState: 'explained_only' }),
    ],
    deferred: [],
    rejected: [],
  },
  settings: {},
}

/** 項目キーで升目を探す */
const cell = (key: string) => screen.getByRole('button', { name: new RegExp(`^${key} `) })

describe('作業ビューのマトリクス', () => {
  it('升目の塗りは verifiedLevel で、[3] だけの項目は塗らずに右上の三角を付ける', () => {
    render(<PlanView data={data} today={today} />)
    expect(cell('go-syntax')).toHaveClass('l2')
    expect(cell('go-syntax')).not.toHaveClass('up')
    expect(cell('java-rest')).toHaveClass('l3')
    expect(cell('java-rest')).not.toHaveClass('up')
    // 印 [3]・verifiedLevel 0：塗らない（l1〜l5 のどれも付かない）が、三角は L3 の色
    const http = cell('go-http')
    expect([...http.classList].filter((c) => /^l[1-5]$/.test(c))).toEqual([])
    expect(http).toHaveClass('up', 'up-l3')
  })

  it('不合格の報告と古い根拠に枠線を付け、新しい根拠には付けない', () => {
    render(<PlanView data={data} today={today} />)
    expect(cell('java-test')).toHaveClass('attn')
    expect(cell('go-old')).toHaveClass('attn')
    expect(cell('go-syntax')).not.toHaveClass('attn')
    expect(cell('go-http')).not.toHaveClass('attn')
  })

  it('升目に乗せると吹き出しに印・未確認の段・最終根拠日が出る', async () => {
    render(<PlanView data={data} today={today} />)
    await userEvent.hover(cell('go-http'))
    const tip = screen.getByRole('tooltip')
    expect(tip).toHaveTextContent('Verified 0 / 根拠の印 3 / 未確認 1, 2')
    expect(tip).toHaveTextContent('2026-08-01（28 日前・新しい）')
    expect(cell('go-http')).toHaveAttribute('aria-describedby', tip.id)
    await userEvent.unhover(cell('go-http'))
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument()
  })

  it('吹き出しに段階前の状態と、要再確認の理由が出る', async () => {
    render(<PlanView data={data} today={today} />)
    await userEvent.hover(cell('java-test'))
    const tip = screen.getByRole('tooltip')
    expect(tip).toHaveTextContent('Verified 0 / 根拠の印 なし / 未確認 なし')
    expect(tip).toHaveTextContent('段階前の状態：説明済み・理解未確認')
    expect(tip).toHaveTextContent('最終根拠：なし')
    expect(tip).toHaveTextContent('要再確認：不合格の報告あり')
    await userEvent.unhover(cell('java-test'))
    await userEvent.hover(cell('go-old'))
    expect(screen.getByRole('tooltip')).toHaveTextContent('要再確認：最終根拠が 90 日より前')
  })

  it('押すと吹き出しが固定され、もう一度押すと閉じる', async () => {
    render(<PlanView data={data} today={today} />)
    await userEvent.click(cell('go-syntax'))
    await userEvent.unhover(cell('go-syntax'))
    expect(screen.getByRole('tooltip')).toHaveTextContent('Verified 2 / 根拠の印 1, 2')
    await userEvent.click(cell('go-syntax'))
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument()
  })

  it('固定した吹き出しにだけ、項目詳細へのリンクが出る', async () => {
    render(<PlanView data={data} today={today} />)
    await userEvent.hover(cell('go-syntax'))
    expect(
      within(screen.getByRole('tooltip')).queryByRole('link', { name: '項目詳細を開く' }),
    ).toBeNull()
    await userEvent.click(cell('go-syntax'))
    expect(
      within(screen.getByRole('tooltip')).getByRole('link', { name: '項目詳細を開く' }),
    ).toHaveAttribute('href', '/plan/item/go-syntax')
  })

  it('サマリー帯に L0〜L5 の件数と「実装根拠あり・理解未確認」の列がある', () => {
    render(<PlanView data={data} today={today} />)
    const table = screen.getByRole('table')
    const headers = within(table)
      .getAllByRole('columnheader')
      .map((th) => th.textContent)
    expect(headers).toEqual([
      '分野',
      '項目',
      'L0',
      'L1',
      'L2',
      'L3',
      'L4',
      'L5',
      '実装根拠あり・理解未確認',
      '要再確認',
      '進捗',
    ])
    const cellsOf = (name: string) =>
      within(within(table).getByRole('rowheader', { name }).closest('tr')!)
        .getAllByRole('cell')
        .map((td) => td.textContent)
    // Go：3 項目。L0 が 1（go-http）・L1 が 1・L2 が 1。[3] だけの go-http が理解未確認、go-old が要再確認。(0+1+2)/15 = 20%
    expect(cellsOf('Go')).toEqual(['3', '1', '1', '1', '0', '0', '0', '1', '1', '20%'])
    // Java：2 項目。L0 が 1・L3 が 1。java-test が要再確認。3/10 = 30%
    expect(cellsOf('Java')).toEqual(['2', '1', '0', '0', '1', '0', '0', '0', '1', '30%'])
    // 合計：5 項目。6/25 = 24%
    expect(cellsOf('合計')).toEqual(['5', '2', '1', '1', '1', '0', '0', '1', '2', '24%'])
  })
})
