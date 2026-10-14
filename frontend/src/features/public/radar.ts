// 公開ビューの 1 層目の「図で見る」（能力値チャート）の形を計算する純粋関数。DOM を触らない。
// 分野を等間隔の軸に並べ（最初の分野が真上、時計回り）、中心から外へ L0〜L5。各分野で確認できた一番高い段を結ぶ。
// 大きさの倍率 t（0〜1）を掛けて描けるので、中心から広がる動きの途中の形も同じ関数で描く（2026-10-12 決定の試作と同じ形）

/** 図の大きさ。viewBox の幅と高さ、中心、L5 の輪の半径 */
export const RADAR = { width: 660, height: 470, cx: 330, cy: 235, radius: 150 } as const

/** 段の上限（L5） */
const MAX_LEVEL = 5

/** k 番目の軸の角度（ラジアン）。0 番目が真上で、時計回りに等間隔 */
function axisAngle(k: number, n: number): number {
  return -Math.PI / 2 + (2 * Math.PI * k) / n
}

/** k 番目の軸の上で、中心から r 離れた点 */
export function axisPoint(k: number, n: number, r: number): [number, number] {
  const a = axisAngle(k, n)
  return [RADAR.cx + r * Math.cos(a), RADAR.cy + r * Math.sin(a)]
}

/** 点の並びを SVG の points の文字列にする（小数 1 桁） */
export function toPoints(pts: [number, number][]): string {
  return pts.map(([x, y]) => `${x.toFixed(1)},${y.toFixed(1)}`).join(' ')
}

/** 段の値（0〜5）を結んだ多角形の頂点。t は大きさの倍率（0 で中心に集まり、1 で本来の形） */
export function valuePoints(levels: number[], t: number): [number, number][] {
  const n = levels.length
  return levels.map((lv, k) =>
    axisPoint(k, n, (RADAR.radius * Math.min(lv, MAX_LEVEL) * t) / MAX_LEVEL),
  )
}

/** 目盛りの輪（L1〜L5）。軸の数と同じ角の多角形 */
export function ringPoints(n: number, level: number): [number, number][] {
  return Array.from({ length: n }, (_, k) => axisPoint(k, n, (RADAR.radius * level) / MAX_LEVEL))
}

/**
 * 分野名の字の大きさと行の高さ（viewBox の単位）。図は画面の幅に合わせて縮むので、スマホ（図の幅 335px で約 0.51 倍）でも
 * 11px 以上になるよう大きめにする（14 ではスマホで約 7px になっていた。2026-10-14 の点検）。CSS の .lab・.tick と同じ値
 */
export const LABEL_FONT = 22
const LABEL_LINE = 26

/** これより長い分野名は 2 行に折る（大きめの字でも図の幅に収めるため） */
const WRAP_OVER = 8

/**
 * 分野名を図に描く行に分ける。長い名前だけ、「 / 」の後・「・」の後・「（」の前のどれか最初の所で 2 行に折る。
 * 例：「React / TypeScript」→「React /」「TypeScript」、「課金（Stripe）」→「課金」「（Stripe）」
 */
export function labelLines(name: string): string[] {
  if (name.length <= WRAP_OVER) return [name]
  const slash = name.indexOf(' / ')
  if (slash > 0) return [name.slice(0, slash + 2), name.slice(slash + 3)]
  const dot = name.indexOf('・')
  if (dot > 0 && dot < name.length - 1) return [name.slice(0, dot + 1), name.slice(dot + 1)]
  const paren = name.indexOf('（')
  if (paren > 0) return [name.slice(0, paren), name.slice(paren)]
  return [name]
}

export interface LabelPlace {
  x: number
  /** 1 行目の字の並ぶ線（ベースライン）の高さ。2 行目以降は LABEL_LINE ずつ下 */
  y: number
  /** 行の高さ（2 行目以降の送り） */
  lineHeight: number
  /** 文字の寄せ方。軸が上下なら中央、右なら左寄せ、左なら右寄せ */
  anchor: 'start' | 'middle' | 'end'
}

/**
 * k 番目の分野名（lines 行）の置き場所。L5 の輪の少し外。
 * 真上の名前は最後の行が輪の上に来るよう上へ、真下の名前は 1 行目が輪の下に来るよう下へ、左右の名前は行の真ん中を軸の高さにそろえる
 */
export function labelPlace(k: number, n: number, lines = 1): LabelPlace {
  const [x, y] = axisPoint(k, n, RADAR.radius + 24)
  const a = axisAngle(k, n)
  const c = Math.cos(a)
  const s = Math.sin(a)
  const anchor = Math.abs(c) < 0.3 ? 'middle' : c > 0 ? 'start' : 'end'
  const extra = (lines - 1) * LABEL_LINE
  // ベースラインは字の下寄りにあるので、字の高さの 3 分の 1 ほど下げると行の真ん中が軸の高さに来る
  const first = s < -0.9 ? y - 6 - extra : s > 0.9 ? y + LABEL_FONT : y + LABEL_FONT / 3 - extra / 2
  return { x, y: first, lineHeight: LABEL_LINE, anchor }
}

/** 動きの速さの変わり方（だんだん遅くなる。3 次） */
export function easeOut(t: number): number {
  return 1 - Math.pow(1 - Math.min(Math.max(t, 0), 1), 3)
}
