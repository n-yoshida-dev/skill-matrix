import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// リポジトリ直下の data/（ロードマップ・理解度の JSON）。frontend/ の外にある
const dataDir = fileURLToPath(new URL('../data', import.meta.url))

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
    // テストがまだ 1 件も無い段階でも CI を通す
    passWithNoTests: true,
  },
})
