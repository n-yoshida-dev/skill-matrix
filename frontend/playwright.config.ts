import { defineConfig, devices } from '@playwright/test'
import { E2E_PORTS } from './e2e/ports.ts'

// E2E（TODO 2-6）。data/ の JSON を差し替えると画面が変わることを、ビルドした静的サイトで確かめる。
// 差し替え前・後の 2 組のダミーデータ（e2e/fixtures/before・after）でそれぞれビルドし、別のポートで配信する。
// データの置き場は環境変数 SKILL_MATRIX_DATA_DIR で指す（vite.config.ts。既定はリポジトリ直下の data/）。

/** 1 組のデータでビルドして配信するコマンド */
function serve(name: keyof typeof E2E_PORTS): string {
  const out = `dist-e2e/${name}`
  return [
    `SKILL_MATRIX_DATA_DIR=e2e/fixtures/${name} npx vite build --outDir ${out} --emptyOutDir`,
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
