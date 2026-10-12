import { expect, test } from '@playwright/test'
import { E2E_PORTS } from './ports.ts'

// 公開ビューのタイルで、長い項目キーと段の表示（「実装済み・基礎未確認」）が重ならないことを確かめる。
// 重なりは見た目の問題で、単体テスト（jsdom）は配置を計算しないので、実際のブラウザで箱の位置を比べる。
// 差し替え後のダミー（e2e/fixtures/after）では、長いキーの項目に実装の印だけ（[3]）が付いていて、いちばん長い段の表示になる。
// 段の表示が長いときは次の行へ送る（2026-10-11 の見た目の点検。キーが「jav…」のように 2〜3 字しか見えなくなっていた）ので、
// 横に並んでも縦に並んでもよく、キーはタイルの幅の半分以上を使えることも確かめる。

const KEY = 'e2e-long-item-key-for-tile-overlap'

for (const width of [1200, 390]) {
  test(`長い項目キーと段の表示が重ならない（幅 ${width}px）`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await page.goto(`http://localhost:${E2E_PORTS.after}/#/`)
    const tile = page.getByRole('button', { name: /長い項目キー/ })
    const key = tile.locator('.n')
    const label = tile.locator('.lv')
    await expect(label).toHaveText('実装済み・基礎未確認')

    const k = await key.boundingBox()
    const l = await label.boundingBox()
    const t = await tile.boundingBox()
    if (!k || !l || !t) throw new Error('タイルの箱が取れない')
    // キーと段の表示の箱が重ならない（横に並ぶならキーの右端が段の左端より左、縦に並ぶならキーの下端が段の上端より上）
    const sideBySide = k.x + k.width <= l.x
    const stacked = k.y + k.height <= l.y
    expect(sideBySide || stacked).toBe(true)
    // 段の表示はタイルの中に収まっている
    expect(l.x + l.width).toBeLessThanOrEqual(t.x + t.width)
    // キーは数字しか見えないほど縮まない（タイルの幅の半分以上）
    expect(k.width).toBeGreaterThanOrEqual(t.width / 2)

    // キーは省略されても、全体は title で読める
    await expect(key).toHaveAttribute('title', KEY)
    // キーを前の部分と最後の区切りの 2 つに分けて描いても、読み上げの名前ではつながった 1 語のまま（間に空白が入らない）
    await expect(tile).toHaveAccessibleName(new RegExp(`^${KEY}`))
  })
}
