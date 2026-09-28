import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// 画面に取り込むデータ（ロードマップ・理解度の JSON）の置き場。リポジトリ直下の data/（frontend/ の外）。
// E2E だけは、ビルドのモード e2e-before / e2e-after で e2e/fixtures/<before|after> のダミーを取り込む
// （playwright.config.ts。JSON を差し替えると画面が変わることを確かめるため）。
// 環境変数では切り替えない（SPEC.md §8.3「環境変数は v1 では使わない」）。E2E 以外のモードでは常に data/
function dataDirFor(mode: string): string {
  const e2e = /^e2e-(before|after)$/.exec(mode)
  const dir = e2e ? `./e2e/fixtures/${e2e[1]}` : '../data'
  return fileURLToPath(new URL(dir, import.meta.url))
}

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const dataDir = dataDirFor(mode)
  return {
    plugins: [react()],
    // GitHub Pages の配置先（/<repo>/ か独自ドメインか）に依存しないよう、相対パスで出力する
    base: './',
    resolve: {
      // 画面からは @data/... で data/ を指す（SPEC.md §3.1・§8.2）
      alias: { '@data': dataDir },
    },
    server: {
      // dev サーバは既定で frontend/ の外を配信しないので、data/ を許可する
      fs: { allow: ['.', dataDir] },
    },
    test: {
      // コンポーネントのテストで DOM が必要なため jsdom を使う
      environment: 'jsdom',
      setupFiles: ['./src/setupTests.ts'],
      // 単体テストは src/ だけ。e2e/ は Playwright が走らせる（Vitest に拾わせない）
      include: ['src/**/*.test.{ts,tsx}'],
      // テストがまだ 1 件も無い段階でも CI を通す
      passWithNoTests: true,
    },
  }
})
