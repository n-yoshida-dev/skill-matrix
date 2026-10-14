// 動きを減らす設定（prefers-reduced-motion）の判定。公開ビューの動き（分野へ移る・図が広がる・帯が埋まる）が共通で使う

/** 動きを減らす設定の端末か。テストの環境（matchMedia が無い）では減らす側に倒す */
export function prefersReducedMotion(): boolean {
  return (
    typeof window.matchMedia !== 'function' ||
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  )
}
