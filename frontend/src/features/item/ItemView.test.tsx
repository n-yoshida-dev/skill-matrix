import { render, screen, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { describe, expect, it } from 'vitest'
import type { AppData, ItemState, StateEvent } from '../../data/types'
import { ItemView } from './ItemView'

// 項目詳細（SPEC.md §7・§7.4、TODO 2-5 の完了条件）を確かめる。
// data/*.json ではなく、このファイルの中の小さなデータで確かめる（data/ が更新されても落ちないように）。
// 「何を保証しているか」：
// - 見出しに Verified Level／付いている印／未確認の段の 3 行がある
// - 印がいつ・どの根拠で付いたかが、履歴（marked・rationale・evidenceRefs）で読める。起きた日の新しい順
// - 印を付けなかった判定は「印なし」、不合格の報告は違反の記号と説明が出る
// - 実装の根拠だけがある項目は「既存の実装について L1 / L2 を…確認する」と次の確認方法を出す
// - 保留（deferred）がこの項目の分だけ出る
// - 前提と、この項目を前提にする項目へのリンクがある
// - 編集の手段（入力欄・ボタン）を置かない
// - ロードマップに無い項目キーなら、無いと言って作業ビューへ戻るリンクを出す

const ev = (over: Partial<StateEvent>): StateEvent => ({
  file: '2026-09-28-x.json',
  index: 0,
  occurredAt: '2026-09-01',
  source: 'ai',
  evidenceType: 'drill',
  proposedLevel: 1,
  marked: [],
  evidenceRefs: [],
  rationale: '',
  confidence: 0.8,
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

const today = new Date(2026, 8, 29)

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
        key: 'db',
        name: 'データベース',
        items: [
          {
            key: 'db-schema',
            name: 'スキーマ設計',
            description: '共有 DB と tenant_id',
            outcome: '混入を防ぐ制約を説明できる',
            verifyBy: '複合外部キーが何を防ぐかを答える（L1・L2）',
          },
        ],
      },
      {
        key: 'java',
        name: 'Java',
        items: [
          { key: 'java-tenant', name: 'テナントの選択', dependsOn: ['db-schema'] },
          { key: 'java-rest', name: 'REST API' },
        ],
      },
    ],
  },
  state: {
    schemaVersion: 2,
    items: [
      item({
        itemKey: 'db-schema',
        evidencedLevels: [3],
        lastEvidenceAt: '2026-09-27',
        // events は適用順（判定ファイル名の順）。起きた日の順とは限らない
        events: [
          ev({
            file: '2026-09-27-migration-db.json',
            occurredAt: '2026-09-27',
            source: 'migration',
            evidenceType: 'implementation',
            proposedLevel: 3,
            marked: [3],
            evidenceRefs: ['repo:n-yoshida-dev/orgflow@abc1234'],
            rationale: '複合外部キーでテナントの混入を防ぐスキーマを実装した',
            confidence: null,
          }),
          ev({
            file: '2026-09-28-2026-09-10-x.json',
            occurredAt: '2026-09-10',
            evidenceType: 'explained_to',
            proposedLevel: 0,
            rationale: '共有 DB の説明を受けた',
          }),
          ev({
            file: '2026-09-28-2026-09-10-x.json',
            index: 1,
            occurredAt: '2026-09-10',
            proposedLevel: 0,
            rationale: '複合外部キーの問いに答えられなかった',
            violations: [{ code: 'V6_failed_check', detail: '不合格の報告' }],
          }),
        ],
      }),
    ],
    deferred: [
      {
        file: '2026-09-28-y.json',
        index: 2,
        itemKey: 'db-schema',
        code: 'V7_low_confidence',
        detail: '確信度 0.40 が閾値 0.50 未満のため保留',
      },
      {
        file: '2026-09-28-y.json',
        index: 3,
        itemKey: 'java-rest',
        code: 'V7_low_confidence',
        detail: '別の項目の保留',
      },
    ],
    rejected: [],
  },
  settings: {},
}

function renderItem(key: string) {
  return render(
    <MemoryRouter initialEntries={[`/plan/item/${key}`]}>
      <Routes>
        <Route path="/plan/item/:itemKey" element={<ItemView data={data} today={today} />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('項目詳細', () => {
  it('見出しに Verified Level・付いている印・未確認の段の 3 行がある', () => {
    renderItem('db-schema')
    expect(screen.getByRole('heading', { level: 2, name: 'スキーマ設計' })).toBeInTheDocument()
    const dl = screen.getByText('Verified Level').closest('dl') as HTMLElement
    expect(within(dl).getByText('Verified Level').nextSibling?.textContent).toBe('0（未着手）')
    expect(within(dl).getByText('付いている印').nextSibling?.textContent).toBe('3')
    expect(within(dl).getByText('未確認の段').nextSibling?.textContent).toBe('1, 2')
  })

  it('印がいつ・どの根拠で付いたかを、起きた日の新しい順に読める', () => {
    renderItem('db-schema')
    const rows = within(
      screen.getByRole('heading', { name: /履歴/ }).nextElementSibling as HTMLElement,
    )
      .getAllByRole('listitem')
      .filter((li) => li.classList.contains('item-event'))
    expect(rows.map((li) => li.querySelector('.ev-why')?.textContent)).toEqual([
      '複合外部キーでテナントの混入を防ぐスキーマを実装した',
      '複合外部キーの問いに答えられなかった',
      '共有 DB の説明を受けた',
    ])
    const first = within(rows[0])
    expect(first.getByText('2026-09-27')).toBeInTheDocument()
    expect(first.getByText('実装した')).toBeInTheDocument()
    expect(first.getByText(/を付けた/).textContent).toBe('印 3 を付けた')
    expect(first.getByText('repo:n-yoshida-dev/orgflow@abc1234')).toBeInTheDocument()
    expect(first.getByText('移行')).toBeInTheDocument()
  })

  it('印を付けなかった判定は「印なし」、不合格の報告は違反の記号と説明が出る', () => {
    renderItem('db-schema')
    const failed = screen
      .getByText('複合外部キーの問いに答えられなかった')
      .closest('li') as HTMLElement
    expect(within(failed).getByText('印なし')).toBeInTheDocument()
    expect(within(failed).getByText('V6_failed_check')).toBeInTheDocument()
  })

  it('実装の根拠だけがある項目は、既存の実装について基礎を確認する文言と確認方法を出す', () => {
    renderItem('db-schema')
    expect(
      screen.getByText('次にやること：既存の実装について L1 / L2 を短いドリル・自己説明で確認する'),
    ).toBeInTheDocument()
    expect(
      screen.getByText('確認方法：複合外部キーが何を防ぐかを答える（L1・L2）'),
    ).toBeInTheDocument()
    expect(screen.getByText('身につくと：混入を防ぐ制約を説明できる')).toBeInTheDocument()
  })

  it('保留はこの項目の分だけ出る', () => {
    renderItem('db-schema')
    expect(screen.getByText(/確信度 0.40 が閾値 0.50 未満のため保留/)).toBeInTheDocument()
    expect(screen.queryByText(/別の項目の保留/)).toBeNull()
    expect(screen.getByRole('heading', { name: '保留（1 件）' })).toBeInTheDocument()
  })

  it('前提と、この項目を前提にする項目へのリンクがある', () => {
    renderItem('db-schema')
    expect(screen.getByRole('link', { name: 'テナントの選択' })).toHaveAttribute(
      'href',
      '/plan/item/java-tenant',
    )
  })

  it('編集の手段を置かない', () => {
    renderItem('db-schema')
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(screen.queryByRole('button')).toBeNull()
  })

  it('ロードマップに無い項目キーなら、無いと言って作業ビューへ戻るリンクを出す', () => {
    renderItem('no-such-item')
    expect(screen.getByText(/はロードマップに無い/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '作業ビューへ戻る' })).toHaveAttribute('href', '/plan')
  })
})
