import { expect, test, type Page } from '@playwright/test'
import { E2E_PORTS } from './ports.ts'

// JSON を差し替えると、マトリクスと学習パスが変わることを確かめる（TODO 2-6 の E2E）。
// 差し替え前（e2e/fixtures/before）：「基礎」は未着手。学習パスの今ここは「基礎」
// 差し替え後（e2e/fixtures/after）：state.json だけを替え、「基礎」がレベル 2。今ここは「応用」へ進む
// ビルドした静的サイト（vite build → vite preview）を開くので、データはビルド時に取り込まれたものを見ている

const site = (name: keyof typeof E2E_PORTS) => `http://localhost:${E2E_PORTS[name]}/#`

/** マトリクスの升目（項目キーと項目名が名前になっているボタン） */
const cell = (page: Page, key: string, name: string) =>
  page.getByRole('button', { name: `${key} ${name}` })

/** 学習パスの一列表示の行 */
const row = (page: Page, key: string) =>
  page.getByRole('listitem').filter({ has: page.getByText(key, { exact: true }) })

test('JSON を差し替えると、マトリクスと学習パスが変わる', async ({ page }) => {
  // 差し替え前
  await page.goto(`${site('before')}/plan`)
  await expect(cell(page, 'e2e-basics', '基礎')).toBeVisible()
  await expect(cell(page, 'e2e-basics', '基礎')).not.toHaveClass(/\bl2\b/)

  await page.goto(`${site('before')}/plan/path/e2e`)
  await expect(row(page, 'e2e-basics')).toContainText('今ここ')
  await expect(row(page, 'e2e-next')).toContainText('前提が未達')

  // 差し替え後（state.json だけが違う）
  await page.goto(`${site('after')}/plan`)
  await expect(cell(page, 'e2e-basics', '基礎')).toHaveClass(/\bl2\b/)

  await page.goto(`${site('after')}/plan/path/e2e`)
  await expect(row(page, 'e2e-basics')).toContainText('✓')
  await expect(row(page, 'e2e-next')).toContainText('今ここ')
})
