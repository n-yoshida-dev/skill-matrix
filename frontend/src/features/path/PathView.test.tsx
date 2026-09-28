import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { describe, expect, it } from 'vitest'
import type { AppData, ItemState } from '../../data/types'
import { PathView } from './PathView'

// 学習パス・一列表示（SPEC.md §7.2、TODO 2-5）を確かめる。
// data/*.json ではなく、このファイルの中の小さなデータで確かめる（data/ が更新されても落ちないように）。
// 「何を保証しているか」：
// - 分野を選ぶと、その分野の項目が依存順に縦一列で並ぶ（前提が先）
// - 上部に分野の目標と「N 項目中 M 項目（レベル 1 以上）」の進捗が出る
// - 済・今ここ・この先が記号と「今ここ」の札で分かる。今ここには次の確認方法が出る
// - どの行にも到達状態（身につくと）が出て、空なら「未記入」
// - 済んだ項目のうち「次にやること」に入っているものに「深掘り候補」が付く
// - 印が表示レベルより上にある項目に「上位の根拠あり」が付く
// - 分野の外の前提は、名前とレベルを添えて出る
// - 分野のタブを押すと別の分野に切り替わる

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

const data: AppData = {
  roadmap: {
    schemaVersion: 1,
    name: 'テスト用',
    levels: [],
    domains: [
      {
        key: 'go',
        name: 'Go',
        goal: 'Go で Web API を書ける',
        items: [
          // 定義順は依存順と逆にしてある
          {
            key: 'go-http',
            name: 'HTTP サーバ',
            dependsOn: ['go-syntax', 'db-schema'],
            outcome: '最小の API を書ける',
          },
          {
            key: 'go-syntax',
            name: '基本構文',
            outcome: '型と値の流れを追える',
            verifyBy: 'ゼロ値のドリル（L2）',
          },
          {
            key: 'go-errors',
            name: 'エラー処理',
            dependsOn: ['go-syntax'],
            verifyBy: 'error を返すドリル（L1）',
          },
        ],
      },
      {
        key: 'db',
        name: 'データベース',
        goal: 'スキーマを設計できる',
        items: [{ key: 'db-schema', name: 'スキーマ設計', outcome: '制約を説明できる' }],
      },
    ],
  },
  state: {
    schemaVersion: 2,
    items: [
      item({
        itemKey: 'go-syntax',
        verifiedLevel: 1,
        evidencedLevels: [1, 3],
        lastEvidenceAt: '2026-09-20',
      }),
    ],
    deferred: [],
    rejected: [],
  },
  settings: {},
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/plan/path" element={<PathView data={data} today={today} />} />
        <Route path="/plan/path/:domainKey" element={<PathView data={data} today={today} />} />
      </Routes>
    </MemoryRouter>,
  )
}

/** 学習パスの行を、上から項目キーの並びで返す */
function rowKeys(): string[] {
  const list = screen.getByRole('list')
  return within(list)
    .getAllByRole('listitem')
    .map((li) => li.querySelector('.k')?.textContent ?? '')
}

function row(key: string) {
  const li = within(screen.getByRole('list')).getByText(key).closest('li')
  if (!li) throw new Error(`${key} の行が無い`)
  return within(li)
}

describe('学習パス・一列表示', () => {
  it('分野を省略すると先頭の分野を、依存順に縦一列で並べる', () => {
    renderAt('/plan/path')
    expect(screen.getByRole('heading', { name: 'Go' })).toBeInTheDocument()
    expect(rowKeys()).toEqual(['go-syntax', 'go-http', 'go-errors'])
  })

  it('上部に目標と進捗が出る', () => {
    renderAt('/plan/path/go')
    expect(screen.getByText('目標：Go で Web API を書ける')).toBeInTheDocument()
    expect(screen.getByText(/進捗：/).textContent).toBe('進捗：3 項目中 1 項目（レベル 1 以上）')
  })

  it('済・今ここ・この先が分かり、今ここには次の確認方法が出る', () => {
    renderAt('/plan/path/go')
    // go-http は分野の外の前提（db-schema）がレベル 0 なので着手できず、今ここは go-errors
    expect(row('go-syntax').getByText('✓')).toBeInTheDocument()
    expect(row('go-http').getByText('前提が未達')).toBeInTheDocument()
    expect(row('go-errors').getByText('今ここ')).toBeInTheDocument()
    expect(row('go-errors').getByText('次の確認方法：error を返すドリル（L1）')).toBeInTheDocument()
    expect(row('go-syntax').queryByText(/次の確認方法/)).toBeNull()
  })

  it('済んだ項目には、前提が未達でも「前提が未達」を出さない', () => {
    const done: AppData = {
      ...data,
      state: {
        ...data.state,
        items: [
          ...data.state.items,
          item({ itemKey: 'go-http', verifiedLevel: 3, evidencedLevels: [1, 2, 3] }),
        ],
      },
    }
    render(
      <MemoryRouter initialEntries={['/plan/path/go']}>
        <Routes>
          <Route path="/plan/path/:domainKey" element={<PathView data={done} today={today} />} />
        </Routes>
      </MemoryRouter>,
    )
    // go-http の前提 db-schema はレベル 0 のまま
    expect(row('go-http').getByText('✓')).toBeInTheDocument()
    expect(row('go-http').queryByText('前提が未達')).toBeNull()
  })

  it('項目名を押すと項目詳細へ移れる', () => {
    renderAt('/plan/path/go')
    expect(row('go-http').getByRole('link', { name: 'HTTP サーバ' })).toHaveAttribute(
      'href',
      '/plan/item/go-http',
    )
  })

  it('到達状態を出し、空なら未記入と出す', () => {
    renderAt('/plan/path/go')
    expect(row('go-http').getByText('身につくと：最小の API を書ける')).toBeInTheDocument()
    expect(row('go-errors').getByText('身につくと：未記入')).toBeInTheDocument()
  })

  it('済んだ項目のうち「次にやること」に入るものに深掘り候補、上位の印がある項目に上位の根拠ありが付く', () => {
    renderAt('/plan/path/go')
    expect(row('go-syntax').getByText('深掘り候補')).toBeInTheDocument()
    expect(row('go-syntax').getByText('上位の根拠あり（印 1, 3）')).toBeInTheDocument()
    expect(row('go-errors').queryByText('深掘り候補')).toBeNull()
  })

  it('分野の外の前提を、名前とレベルを添えて出す', () => {
    renderAt('/plan/path/go')
    expect(row('go-http').getByText(/ほかの分野の前提：/).textContent).toBe(
      'ほかの分野の前提：db-schema スキーマ設計（レベル 0）',
    )
  })

  it('分野のタブを押すと、その分野の学習パスに切り替わる', async () => {
    renderAt('/plan/path/go')
    await userEvent.click(screen.getByRole('link', { name: 'データベース' }))
    expect(screen.getByRole('heading', { name: 'データベース' })).toBeInTheDocument()
    expect(rowKeys()).toEqual(['db-schema'])
  })
})
