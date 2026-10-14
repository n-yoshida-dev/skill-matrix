import { expect, test } from '@playwright/test'
import { E2E_PORTS } from './ports.ts'

// 公開ビューのタイルと 1 層目（分野の要約）を、実際のブラウザで確かめる。
// 重なりと読み上げの名前は見た目・ブラウザの計算に依るので、単体テスト（jsdom）では確かめられない。
// 差し替え後のダミー（e2e/fixtures/after）では、「長い項目キー」の項目に実装の印だけ（[3]）が付いていて、いちばん長い段の表示になる。
// タイルは項目名と段だけで、項目キー（内部名）は出さない（2026-10-12 決定）。

const NAME = '長い項目キー'
const KEY = 'e2e-long-item-key-for-tile-overlap'
const LABEL = '実装済み・基礎未確認'

for (const width of [1200, 390]) {
  test(`タイルの項目名と段の表示が重ならず、項目キーを出さない（幅 ${width}px）`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 })
    await page.goto(`http://localhost:${E2E_PORTS.after}/#/`)
    const tile = page.getByRole('button', { name: new RegExp(`^${NAME}`) })
    const name = tile.locator('.name')
    const label = tile.locator('.lv')
    await expect(label).toHaveText(LABEL)

    const n = await name.boundingBox()
    const l = await label.boundingBox()
    const t = await tile.boundingBox()
    if (!n || !l || !t) throw new Error('タイルの箱が取れない')
    // 項目名の下に段の表示が来る（重ならない）
    expect(n.y + n.height).toBeLessThanOrEqual(l.y)
    // 段の表示はタイルの中に収まっている
    expect(l.x + l.width).toBeLessThanOrEqual(t.x + t.width)

    // 項目キーは見た目にも読み上げにも出ない。読み上げの名前は「項目名 段」
    await expect(tile).not.toContainText(KEY)
    await expect(tile).toHaveAccessibleName(`${NAME} ${LABEL}`)
  })
}

test('1 層目の分野を押すと、その分野のタイルへ移って見出しにフォーカスが来る', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 700 })
  await page.goto(`http://localhost:${E2E_PORTS.after}/#/`)
  const overview = page.getByRole('region', { name: '分野ごとに、確認できた一番高い段' })
  await overview.getByRole('link', { name: /E2E 分野/ }).click()
  const heading = page.getByRole('heading', { level: 2, name: 'E2E 分野' })
  await expect(heading).toBeFocused()
  await expect(heading).toBeInViewport()
  // 経路（HashRouter の #/）は変わらない
  await expect(page).toHaveURL(/#\/$/)
})

// 「図で見る」の多角形の点が、広がり終わったあとに正しい位置（中心より上）へ来るか。
// 差し替え後のダミーの分野は 1 つで、一番高い段は 2（「基礎」がレベル 2）。点は中心から真上へ半径の 2/5 の所に来る
for (const reducedMotion of ['no-preference', 'reduce'] as const) {
  test(`「図で見る」で多角形が描かれる（動き：${reducedMotion}）`, async ({ page }) => {
    await page.emulateMedia({ reducedMotion })
    await page.goto(`http://localhost:${E2E_PORTS.after}/#/`)
    await page.getByRole('button', { name: '図で見る' }).click()
    const dot = page.locator('.lb-svg .dot').first()
    // 動きを減らす設定なら押した直後から最終の形。動く場合は広がり終わる（0.55 秒）のを待つ
    if (reducedMotion === 'no-preference') await page.waitForTimeout(800)
    const cy = Number(await dot.getAttribute('cy'))
    // RADAR の中心 cy=235、半径 150 → 段 2 なら 235 - 60 = 175
    expect(cy).toBeCloseTo(175, 0)
    await expect(page.getByRole('img', { name: /多角形/ })).toBeVisible()
  })
}
