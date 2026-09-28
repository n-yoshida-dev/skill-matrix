// 学習パスの表示（一列／ツリー）の選択を、ブラウザに記憶する（SPEC.md §7.2「切り替えを記憶する」）。
// 画面を開く人ごとの好みなので localStorage に置く。初回は一列表示。

/** 学習パスの表示 */
export type PathViewMode = 'list' | 'tree'

/** localStorage のキー */
export const VIEW_MODE_KEY = 'skill-matrix.pathView'

/**
 * 記憶した表示を読む。無い・読めない（プライベートブラウズ等で localStorage が使えない）ときは一列表示。
 * 読めないときは記憶が効かないだけで画面は動くので、例外は投げずに警告を残す
 */
export function readViewMode(storage: Storage | undefined = globalThis.localStorage): PathViewMode {
  try {
    return storage?.getItem(VIEW_MODE_KEY) === 'tree' ? 'tree' : 'list'
  } catch (e) {
    console.warn('学習パスの表示の記憶を読めなかった（一列表示で開く）', e)
    return 'list'
  }
}

/** 選んだ表示を記憶する。書けないときは記憶しないだけで、表示の切り替えはそのまま効く */
export function writeViewMode(
  mode: PathViewMode,
  storage: Storage | undefined = globalThis.localStorage,
): void {
  try {
    storage?.setItem(VIEW_MODE_KEY, mode)
  } catch (e) {
    console.warn('学習パスの表示を記憶できなかった（次に開くと一列表示に戻る）', e)
  }
}
