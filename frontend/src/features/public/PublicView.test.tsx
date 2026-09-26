import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import type { AppData, ItemState, StateEvent } from '../../data/types'
import { PublicView } from './PublicView'

// 公開ビュー（SPEC.md §7.3）が「出すもの」を出し、「出さないもの」を出さないことを確かめる。
// data/*.json ではなく、このファイルの中の小さなデータで確かめる（data/ を実物に差し替えても落ちないように）。
// 「何を保証しているか」：
// - 分野ごとの行と、項目名が常時見えるタイルがある
// - タイルを押すと根拠（日付・根拠の種類・一文）が開く
// - [3] だけの項目は「実装済み・基礎未確認」と出る
// - 段階前の状態・鮮度・保留・優先度・検証ルールの記号・確信度・evidenceRefs の生の文字列は出ない
// - フッターはリポジトリへのリンクだけ

const ev = (over: Partial<StateEvent>): StateEvent => ({
  file: '2026-01-01-x.json',
  index: 0,
  occurredAt: '2026-01-01',
  source: 'ai',
  evidenceType: 'drill',
  proposedLevel: 1,
  marked: [1],
  evidenceRefs: ['repo:example/study@0000000/logs/2026-01-01.md'],
  rationale: '確認質問に答えた',
  confidence: 0.9,
  violations: [],
  ...over,
})

const item = (over: Partial<ItemState> & { itemKey: string }): ItemState => ({
  verifiedLevel: 0,
  evidencedLevels: [],
  preState: 'none',
  needsReview: false,
  lastEvidenceAt: null,
  events: [],
  ...over,
})

const data: AppData = {
  roadmap: {
    schemaVersion: 1,
    name: 'テスト用',
    description: 'これはダミーである旨の注記（公開ビューには出さない）',
    levels: [],
    domains: [
      {
        key: 'go',
        name: 'Go',
        goal: 'Go で API を書ける',
        items: [
          { key: 'go-syntax', name: '基本構文（変数・型）', outcome: '型の流れを追える' },
          { key: 'go-http', name: 'HTTP サーバ', outcome: 'ルーティングを組める' },
          { key: 'go-test', name: 'テスト' },
        ],
      },
      { key: 'react', name: 'React', items: [{ key: 'react-state', name: 'state' }] },
    ],
  },
  state: {
    schemaVersion: 2,
    items: [
      item({
        itemKey: 'go-syntax',
        verifiedLevel: 2,
        evidencedLevels: [1, 2],
        lastEvidenceAt: '2026-02-01',
        events: [
          ev({ occurredAt: '2026-01-01', evidenceType: 'drill', marked: [1] }),
          ev({
            index: 1,
            occurredAt: '2026-02-01',
            evidenceType: 'self_explanation',
            proposedLevel: 2,
            marked: [1, 2],
            rationale: 'ゼロ値を自分の言葉で説明した',
          }),
        ],
      }),
      // 実装の根拠だけがあり、基礎の確認が未了（SPEC.md §1.1 の [3]）
      item({
        itemKey: 'go-http',
        verifiedLevel: 0,
        evidencedLevels: [3],
        lastEvidenceAt: '2026-03-01',
        events: [
          ev({
            occurredAt: '2026-03-01',
            evidenceType: 'implementation',
            proposedLevel: 3,
            marked: [3],
            rationale: '資料を見ながらサーバを書いた',
          }),
        ],
      }),
      // 不合格の報告と保留がある項目（公開ビューにはその情報を出さない）
      item({
        itemKey: 'go-test',
        verifiedLevel: 1,
        evidencedLevels: [1],
        needsReview: true,
        lastEvidenceAt: '2025-01-01',
        events: [
          ev({ occurredAt: '2025-01-01' }),
          ev({
            index: 1,
            occurredAt: '2026-04-01',
            proposedLevel: 0,
            marked: [],
            rationale: '確認質問に答えられなかった',
            violations: [{ code: 'V6_failed_check', detail: '不合格の報告' }],
          }),
        ],
      }),
      item({ itemKey: 'react-state', preState: 'self_reported' }),
    ],
    deferred: [
      { file: 'f.json', index: 0, itemKey: 'go-test', code: 'V7_low_confidence', detail: '0.4' },
    ],
    rejected: [],
  },
  settings: { site: { repoUrl: 'https://example.invalid/repo' } },
}

describe('公開ビュー', () => {
  it('分野ごとの行と、項目名の見えるタイルを出す', () => {
    const { container } = render(<PublicView data={data} />)
    expect(screen.getByRole('heading', { level: 1, name: '理解度台帳' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: 'Go' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: 'React' })).toBeInTheDocument()
    // 数字と文字が別の要素に分かれるので、要素ごとの文字列で見る
    const counts = [...container.querySelectorAll('.count')].map((el) => el.textContent)
    expect(counts).toEqual(['3 項目中 3 に根拠', '1 項目中 0 に根拠'])
    expect(container.querySelector('.thesis.sub')?.textContent).toBe(
      '2 分野 4 項目のうち、根拠のある項目が 3。そのうち実装まで届いているものが 0。',
    )
    // タイルには括弧を除いた項目名が、押さなくても出ている
    const tile = screen.getByRole('button', { name: /go-syntax/ })
    expect(tile).toHaveTextContent('基本構文')
    expect(tile).not.toHaveTextContent('変数')
    expect(tile).toHaveTextContent('L2')
  })

  it('タイルを押すと、その根拠が新しい順に開く', async () => {
    render(<PublicView data={data} />)
    const tile = screen.getByRole('button', { name: /go-syntax/ })
    expect(tile).toHaveAttribute('aria-expanded', 'false')
    await userEvent.click(tile)
    expect(tile).toHaveAttribute('aria-expanded', 'true')
    const detail = document.getElementById('detail-go')!
    expect(within(detail).getByText('印が付いた根拠')).toBeInTheDocument()
    const dates = within(detail)
      .getAllByText(/^2026-0[12]-01$/)
      .map((el) => el.textContent)
    expect(dates).toEqual(['2026-02-01', '2026-01-01'])
    expect(detail.textContent).toContain('L1・2　自分の言葉で説明した')
    expect(detail.textContent).toContain('ゼロ値を自分の言葉で説明した')
    expect(within(detail).getByText('型の流れを追える')).toBeInTheDocument()
  })

  it('[3] だけの項目は「実装済み・基礎未確認」と出る', async () => {
    render(<PublicView data={data} />)
    const tile = screen.getByRole('button', { name: /go-http/ })
    expect(tile).toHaveTextContent('実装済み・基礎未確認')
    await userEvent.click(tile)
    expect(screen.getByText(/基礎の確認が未了/)).toBeInTheDocument()
  })

  it('印の無い項目は「未着手」で、根拠を開くと手をつけていない旨が出る', async () => {
    render(<PublicView data={data} />)
    const tile = screen.getByRole('button', { name: /react-state/ })
    expect(tile).toHaveTextContent('未着手')
    await userEvent.click(tile)
    // 自己申告の記録は無いので「手をつけていない」
    expect(screen.getByText('まだ手をつけていません')).toBeInTheDocument()
  })

  it('作業用の情報（出さないもの）を描画せず、フッターはリポジトリへのリンクだけ', async () => {
    const { container } = render(<PublicView data={data} />)
    // 不合格の報告・保留のある項目の根拠を開いた状態でも出ないことを確かめる
    await userEvent.click(screen.getByRole('button', { name: /go-test/ }))
    const text = container.textContent ?? ''
    for (const banned of [
      '鮮度',
      '要再確認',
      '保留',
      'レビュー待ち',
      '優先度',
      'V6',
      'V7',
      'confidence',
      'repo:',
      '自己申告のみ',
      'ダミーである旨',
    ]) {
      expect(text).not.toContain(banned)
    }
    const footer = container.querySelector('footer')!
    const links = within(footer).getAllByRole('link')
    expect(links).toHaveLength(1)
    expect(links[0]).toHaveAttribute('href', 'https://example.invalid/repo')
  })
})
