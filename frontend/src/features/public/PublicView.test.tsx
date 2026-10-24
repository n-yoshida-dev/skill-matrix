import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import type { AppData, ItemState, StateEvent } from '../../data/types'
import { PublicView } from './PublicView'

// 公開ビュー（SPEC.md §7.3）が「出すもの」を出し、「出さないもの」を出さないことを確かめる。
// data/*.json ではなく、このファイルの中の小さなデータで確かめる（data/ を実物に差し替えても落ちないように）。
// 「何を保証しているか」：
// - 1 層目に分野ごとの要約（一番高い段・段の名前・根拠の数・資格）があり、押すとその分野の見出しへ移る
// - 分野ごとの行と、項目名が常時見えるタイルがある（項目キーは出さない）
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
    retracted: [],
  },
  settings: { site: { repoUrl: 'https://example.invalid/repo' } },
}

describe('公開ビュー', () => {
  it('分野ごとの行と、項目名の見えるタイルを出す。タイルに項目キーは出さない', () => {
    const { container } = render(<PublicView data={data} />)
    expect(screen.getByRole('heading', { level: 1, name: '理解度台帳' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: 'Go' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: 'React' })).toBeInTheDocument()
    // 分野の数は画面に出さず、読み上げにだけ渡す（1 層目の帯とタイルの数が示すため。2026-10-24）
    const counts = [...container.querySelectorAll('.domain-head .sr-only')].map(
      (el) => el.textContent,
    )
    expect(counts).toEqual(['3 項目中 3 に根拠', '1 項目中 0 に根拠'])
    // タイルには括弧を除いた項目名と段が、押さなくても出ている。項目キー（内部名）は出ない
    const tile = screen.getByRole('button', { name: /^基本構文/ })
    expect(tile).toHaveTextContent('基本構文')
    expect(tile).not.toHaveTextContent('変数')
    expect(tile).toHaveTextContent('L2')
    expect(tile).not.toHaveTextContent('go-syntax')
    expect(tile).toHaveAccessibleName('基本構文 L2')
    // 段 0 のタイルは「未着手」を画面に出さず、読み上げの名前にだけ残す
    const empty = screen.getByRole('button', { name: /未着手$/ })
    expect(within(empty).getByText('未着手')).toHaveClass('sr-only')
  })

  it('1 層目に分野ごとの要約を出し、押すとその分野の見出しへ移る', async () => {
    const withCerts: AppData = { ...data, certifications: [{ domainKey: 'go', name: 'Go 資格' }] }
    render(<PublicView data={withCerts} />)
    const overview = screen.getByRole('region', { name: '分野ごとに、確認できた一番高い段' })
    const rows = within(overview).getAllByRole('link')
    expect(rows).toHaveLength(2)
    // Go は一番高い段が 2、資格を添える。React は未着手で資格なし
    expect(rows[0]).toHaveTextContent('Lv2')
    expect(rows[0]).toHaveTextContent('自分の言葉で説明できる')
    // 数は読み上げにだけ渡す（画面では帯のコマの数が示す）
    expect(rows[0].querySelector('.sr-only')).toHaveTextContent('3 項目中 3 に根拠')
    expect(rows[0].querySelector('.lc-c')).toBeNull()
    expect(rows[0]).toHaveTextContent('資格：Go 資格（自己申告）')
    expect(rows[1]).toHaveTextContent('未着手')
    expect(rows[1]).not.toHaveTextContent('資格')
    await userEvent.click(rows[1])
    expect(screen.getByRole('heading', { level: 2, name: 'React' })).toHaveFocus()
    // 「↑ 要約へ」で 1 層目の見出しへ戻る
    const reactSection = document.getElementById('d-react')!
    await userEvent.click(within(reactSection).getByRole('link', { name: '↑ 要約へ' }))
    expect(
      screen.getByRole('heading', { level: 2, name: '分野ごとに、確認できた一番高い段' }),
    ).toHaveFocus()
  })

  it('1 層目は最初「一覧で見る」で、「図で見る」で能力値チャートと分野の一覧に切り替わる', async () => {
    render(<PublicView data={data} />)
    const overview = screen.getByRole('region', { name: '分野ごとに、確認できた一番高い段' })
    const listBtn = within(overview).getByRole('button', { name: '一覧で見る' })
    const chartBtn = within(overview).getByRole('button', { name: '図で見る' })
    expect(listBtn).toHaveAttribute('aria-pressed', 'true')
    expect(within(overview).queryByRole('img')).toBeNull()

    await userEvent.click(chartBtn)
    expect(chartBtn).toHaveAttribute('aria-pressed', 'true')
    expect(listBtn).toHaveAttribute('aria-pressed', 'false')
    expect(within(overview).getByRole('img', { name: /多角形/ })).toBeInTheDocument()
    // キーボードと読み上げの止まり場所は一覧だけ（図の分野名は読み上げから外す）。「2/9」ではなく文で読む
    const rows = within(overview).getAllByRole('link')
    expect(rows).toHaveLength(2)
    expect(rows[0]).toHaveAccessibleName('Go L2 3 項目中 3 に根拠')
    await userEvent.click(rows[1])
    expect(screen.getByRole('heading', { level: 2, name: 'React' })).toHaveFocus()

    // 一覧に戻すと、ステータス帳の行（大きな Lv の数字）に戻る。テストの環境は動きを減らす側なので数字はすぐ最終の値
    await userEvent.click(listBtn)
    expect(within(overview).queryByRole('img')).toBeNull()
    expect(within(overview).getAllByRole('link')[0]).toHaveTextContent('Lv2')
  })

  it('タイルを押すと、その根拠が新しい順に開く', async () => {
    render(<PublicView data={data} />)
    const tile = screen.getByRole('button', { name: /^基本構文/ })
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

  it('根拠の枠を「閉じる」で閉じると、開いていたタイルへフォーカスが戻る', async () => {
    render(<PublicView data={data} />)
    const tile = screen.getByRole('button', { name: /^基本構文/ })
    await userEvent.click(tile)
    await userEvent.click(screen.getByRole('button', { name: '閉じる' }))
    expect(document.getElementById('detail-go')).toBeNull()
    expect(tile).toHaveAttribute('aria-expanded', 'false')
    expect(tile).toHaveFocus()
  })

  it('[3] だけの項目は「実装済み・基礎未確認」と出る', async () => {
    render(<PublicView data={data} />)
    const tile = screen.getByRole('button', { name: /^HTTP サーバ/ })
    expect(tile).toHaveTextContent('実装済み・基礎未確認')
    // 字は読み上げだけで、画面には帯と同じ枠だけのコマを出す（2026-10-24）
    expect(within(tile).getByText('実装済み・基礎未確認')).toHaveClass('sr-only')
    expect(tile.querySelector('.u.impl')).not.toBeNull()
    await userEvent.click(tile)
    expect(screen.getByText(/基礎の確認が未了/)).toBeInTheDocument()
  })

  it('印の無い項目は「未着手」で、根拠を開くと手をつけていない旨が出る', async () => {
    render(<PublicView data={data} />)
    const tile = screen.getByRole('button', { name: /^state/ })
    expect(tile).toHaveTextContent('未着手')
    await userEvent.click(tile)
    // 自己申告の記録は無いので「手をつけていない」
    expect(screen.getByText('まだ手をつけていません')).toBeInTheDocument()
  })

  it('作業用の情報（出さないもの）を描画せず、フッターはリポジトリへのリンクだけ', async () => {
    const { container } = render(<PublicView data={data} />)
    // 不合格の報告・保留のある項目の根拠を開いた状態でも出ないことを確かめる
    await userEvent.click(screen.getByRole('button', { name: /^テスト/ }))
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
