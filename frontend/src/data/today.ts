import type { Settings } from './types'

// 鮮度を測る「今日」を決める純粋関数（SPEC.md §5・§8.3）。DOM・ファイル読み込みを import しない。

/**
 * settings.json の site.fixedToday があればその日（その日の 0 時）、無ければ now を返す。
 * 見本の画面は日付の止まった架空のデータなので、日が経っても「古い根拠」の見え方が変わらないよう固定する。
 * 形が YYYY-MM-DD でない・暦に無い日付なら例外にする（打ち間違いで黙って本当の今日に戻らないように）
 */
export function resolveToday(settings: Settings, now: Date): Date {
  const fixed = settings.site?.fixedToday
  if (fixed === undefined) return now
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(fixed)
  const d = m ? new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3])) : null
  if (!m || !d || d.getMonth() !== Number(m[2]) - 1 || d.getDate() !== Number(m[3])) {
    throw new Error(`settings.json の site.fixedToday が YYYY-MM-DD の日付ではない: ${fixed}`)
  }
  return d
}
