import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// 画面に取り込むデータ（ロードマップ・理解度の JSON）の置き場。既定はリポジトリ直下の data/（frontend/ の外）。
// 環境変数 SKILL_MATRIX_DATA_DIR で別の置き場を指せる（frontend/ からの相対パスか絶対パス）。
// E2E が、差し替え前・後のダミーデータで画面をビルドするのに使う（playwright.config.ts）
const frontendDir = fileURLToPath(new URL('.', import.meta.url))
const dataDir = process.env.SKILL_MATRIX_DATA_DIR
  ? resolve(frontendDir, process.env.SKILL_MATRIX_DATA_DIR)
  : fileURLToPath(new URL('../data', import.meta.url))

// https://vite.dev/config/
export default defineConfig({
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
})
