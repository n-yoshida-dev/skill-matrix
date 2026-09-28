import { defineConfig, devices } from '@playwright/test'
import { E2E_PORTS } from './e2e/ports.ts'

// E2E（TODO 2-6）。data/ の JSON を差し替えると画面が変わることを、ビルドした静的サイトで確かめる。
// 差し替え前・後の 2 組のダミーデータ（e2e/fixtures/before・after）でそれぞれビルドし、別のポートで配信する。
// どちらのダミーを取り込むかは、ビルドのモード（--mode e2e-before / e2e-after）で vite.config.ts に伝える。
// process.env.CI は GitHub Actions が立てる印で、やり直しの回数と出力の形だけに使う（画面の設定ではない）。

/** 1 組のデータでビルドして配信するコマンド */
function serve(name: keyof typeof E2E_PORTS): string {
  const out = `dist-e2e/${name}`
  return [
    `npx vite build --mode e2e-${name} --outDir ${out} --emptyOutDir`,
    `npx vite preview --outDir ${out} --port ${E2E_PORTS[name]} --strictPort`,
  ].join(' && ')
}

export default defineConfig({
  testDir: 'e2e',
  // CI で失敗したら 1 回だけやり直す（ビルド直後の起動の遅れで落ちるのを避ける）
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: { ...devices['Desktop Chrome'] },
  webServer: (['before', 'after'] as const).map((name) => ({
    command: serve(name),
    url: `http://localhost:${E2E_PORTS[name]}/`,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  })),
})
