import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { loadData } from '../../data/load'
import { PublicView } from './PublicView'

// 公開ビュー（SPEC.md §7.3）が「出すもの」を出し、「出さないもの」を出さないことを確かめる。
// 「何を保証しているか」：
// - 分野ごとの行と、項目名が常時見えるタイルがある
// - タイルを押すと根拠（日付・根拠の種類・一文）が開く
// - [3] だけの項目は「実装済み・基礎未確認」と出る
// - 段階前の状態・鮮度・保留・優先度・検証ルールの記号・確信度・evidenceRefs の生の文字列は出ない

const data = loadData()

describe('公開ビュー', () => {
  it('分野ごとの行と、項目名の見えるタイルを出す', () => {
    render(<PublicView data={data} />)
    expect(screen.getByRole('heading', { level: 1, name: '理解度台帳' })).toBeInTheDocument()
    for (const d of data.roadmap.domains) {
      expect(screen.getByRole('heading', { level: 2, name: d.name })).toBeInTheDocument()
    }
    // タイルには項目名が押さなくても出ている
    expect(screen.getByRole('button', { name: /go-01/ })).toHaveTextContent('基本構文')
  })

  it('タイルを押すと、その根拠が開く', async () => {
    render(<PublicView data={data} />)
    const tile = screen.getByRole('button', { name: /go-01/ })
    expect(tile).toHaveAttribute('aria-expanded', 'false')
    await userEvent.click(tile)
    expect(tile).toHaveAttribute('aria-expanded', 'true')
    const detail = document.getElementById('detail-go')
    expect(detail).not.toBeNull()
    expect(within(detail!).getByText('印が付いた根拠')).toBeInTheDocument()
    expect(within(detail!).getByText('2026-08-03')).toBeInTheDocument()
    expect(within(detail!).getByText(/自分の言葉で説明した/)).toBeInTheDocument()
  })

  it('[3] だけの項目は「実装済み・基礎未確認」と出る', async () => {
    render(<PublicView data={data} />)
    const tile = screen.getByRole('button', { name: /react-02/ })
    expect(tile).toHaveTextContent('実装済み・基礎未確認')
    await userEvent.click(tile)
    expect(screen.getByText(/基礎の確認が未了/)).toBeInTheDocument()
  })

  it('作業用の情報（出さないもの）を描画しない', async () => {
    const { container } = render(<PublicView data={data} />)
    // 根拠を開いた状態でも出ないことを確かめる
    await userEvent.click(screen.getByRole('button', { name: /go-05/ }))
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
    ]) {
      expect(text).not.toContain(banned)
    }
  })
})
