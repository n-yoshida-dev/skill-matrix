import { expect, test, type Page } from '@playwright/test'
import { E2E_PORTS } from './ports.ts'

// JSON を差し替えると、トップのタイルと学習パスが変わることを確かめる（TODO 2-6 の E2E）。
// 差し替え前（e2e/fixtures/before）：「基礎」は未着手。学習パスの今ここは「基礎」で、ツリーでは「挑戦できる」
// 差し替え後（e2e/fixtures/after）：state.json だけを替え、「基礎」がレベル 2。今ここは「応用」へ進み、ツリーでは基礎に●が 2 つ
// ビルドした静的サイト（vite build → vite preview）を開くので、データはビルド時に取り込まれたものを見ている
// 作業ビューのダッシュボード（マトリクスの升目）は 2026-10-13 に画面から外したので、/plan は学習パスを開く

const site = (name: keyof typeof E2E_PORTS) => `http://localhost:${E2E_PORTS[name]}/#`

/** トップ（公開ビュー）の 1 層目の分野の行。読み上げの名前は「Lv<段> 分野名 …」 */
const overviewRow = (page: Page, level: number) =>
  page.getByRole('link', { name: new RegExp(`^Lv${level} E2E 分野 `) })

/** トップのタイル。読み上げの名前は「項目名 段」。判定の無い項目（state.json に無い）はタイルを出さない */
const tile = (page: Page, name: string) =>
  page.getByRole('button', { name: new RegExp(`^${name} `) })

/** 学習パスのツリー表示の節（項目キーで始まる名前のリンク） */
const treeNode = (page: Page, key: string) =>
  page.getByRole('link', { name: new RegExp(`^${key} `) })

/** 学習パスの一列表示の行（項目キーは画面に出さないので data-key で見る。2026-10-24） */
const row = (page: Page, key: string) => page.locator(`li.path-node[data-key="${key}"]`)

test('/plan を開くと学習パスが出て、ダッシュボードのタブは無い', async ({ page }) => {
  await page.goto(`${site('before')}/plan`)
  await expect(page.getByRole('heading', { level: 1, name: '学習パス' })).toBeVisible()
  // 見出しの下の説明の 1 行は 2026-10-24 に外した（見出しとタブが画面の仕事を示す）
  await expect(page.getByText(/自分用の画面です/)).toHaveCount(0)
  await expect(page.getByRole('heading', { level: 2, name: 'E2E 分野' })).toBeVisible()
  // /plan は最初の分野の URL へ移り、その分野のタブが選ばれていると読み上げにも伝わる
  await expect(page).toHaveURL(/#\/plan\/path\/e2e$/)
  const tabs = page.getByRole('navigation', { name: '分野の切り替え' })
  await expect(tabs.getByRole('link', { name: 'E2E 分野' })).toHaveAttribute('aria-current', 'page')
  await expect(page.getByRole('link', { name: 'ダッシュボード' })).toHaveCount(0)
  // ヘッダの入口は「理解度台帳／学習パス／GitHub」
  const nav = page.getByRole('navigation', { name: '画面の切り替え' })
  await expect(nav.getByRole('link')).toHaveText(['理解度台帳', '学習パス', 'GitHub'])
  await expect(nav.getByRole('link', { name: '学習パス' })).toHaveAttribute('aria-current', 'page')
})

test('JSON を差し替えると、トップのタイルと学習パスが変わる', async ({ page }) => {
  // 差し替え前
  await page.goto(`${site('before')}/`)
  await expect(overviewRow(page, 0)).toBeVisible()
  await expect(tile(page, '基礎')).toHaveCount(0)

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
  await page.goto(`${site('after')}/`)
  await expect(overviewRow(page, 2)).toBeVisible()
  await expect(tile(page, '基礎')).toHaveAccessibleName('基礎 L2')

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
