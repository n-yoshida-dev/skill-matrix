// Vitest 実行前の共通セットアップ。toBeInTheDocument などのマッチャを有効にする
import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// globals を使っていないので、描画したコンポーネントをテストごとに自分で片付ける
// （片付けないと前のテストの DOM が残り、同じ名前の要素が複数見つかる）
afterEach(() => {
  cleanup()
})
