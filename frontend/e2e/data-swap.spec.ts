import { expect, test, type Page } from '@playwright/test'
import { E2E_PORTS } from './ports.ts'

// JSON を差し替えると、マトリクスと学習パスが変わることを確かめる（TODO 2-6 の E2E）。
// 差し替え前（e2e/fixtures/before）：「基礎」は未着手。学習パスの今ここは「基礎」で、ツリーでは「挑戦できる」
// 差し替え後（e2e/fixtures/after）：state.json だけを替え、「基礎」がレベル 2。今ここは「応用」へ進み、ツリーでは基礎に●が 2 つ
// ビルドした静的サイト（vite build → vite preview）を開くので、データはビルド時に取り込まれたものを見ている

const site = (name: keyof typeof E2E_PORTS) => `http://localhost:${E2E_PORTS[name]}/#`

/** マトリクスの升目（項目キーと項目名が名前になっているボタン） */
const cell = (page: Page, key: string, name: string) =>
  page.getByRole('button', { name: `${key} ${name}` })

/** 学習パスのツリー表示の節（項目キーで始まる名前のリンク） */
const treeNode = (page: Page, key: string) =>
  page.getByRole('link', { name: new RegExp(`^${key} `) })

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
  // ツリー表示（SPEC.md §7.2 の 3 状態）。今ここはツリーに出ない
  await page.getByRole('button', { name: 'ツリー' }).click()
  await expect(treeNode(page, 'e2e-basics')).toContainText('挑戦できる')
  await expect(treeNode(page, 'e2e-next')).toContainText('必要：基礎')
  await expect(page.locator('.tree')).not.toContainText('今ここ')
  await page.getByRole('button', { name: '一列' }).click()

  // 差し替え後（state.json だけが違う）
  await page.goto(`${site('after')}/plan`)
  await expect(cell(page, 'e2e-basics', '基礎')).toHaveClass(/\bl2\b/)

  await page.goto(`${site('after')}/plan/path/e2e`)
  await expect(row(page, 'e2e-basics')).toContainText('✓')
  await expect(row(page, 'e2e-next')).toContainText('今ここ')
  // ツリー表示：基礎はレベル 2 で●が 2 つ、応用は挑戦できる
  await page.getByRole('button', { name: 'ツリー' }).click()
  await expect(treeNode(page, 'e2e-basics')).toContainText('●●○○○Lv2')
  await expect(treeNode(page, 'e2e-basics')).toHaveAccessibleName(/確認済み・レベル 2/)
  await expect(treeNode(page, 'e2e-next')).toContainText('挑戦できる')
  // 上位の根拠（印 3）だけがある項目は、バッジではなく文字で出る
  await expect(treeNode(page, 'e2e-long-item-key-for-tile-overlap')).toContainText(
    '実装の根拠あり・基礎は未確認',
  )
})
