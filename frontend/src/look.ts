// 作業ビューの見た目の案（フェーズ2b の 5 本目「見た目」）。DOM を import しない純粋関数と定数だけを置く。
// 案ごとの色と質感は index.css の `:root[data-look='…']` にあり、ここは「どの案を当てるか」と案ごとの書体だけを持つ。
// 公開ビューは変えない（広げるかは本人に聞く。TODO.md フェーズ2b）ので、作業ビュー（/plan）のときだけ案を当てる。

/** 見た目の案の名前。'plain' は 2026-10-05 までの見た目（藍と IBM Plex） */
export type Look = 'plain' | 'book' | 'window' | 'board'

/** 案の一覧（URL の ?look= で受け付ける名前） */
export const LOOKS: readonly Look[] = ['plain', 'book', 'window', 'board']

/** 案を指定しないときの作業ビューの見た目 */
export const DEFAULT_LOOK: Look = 'plain'

/** 案ごとに追加で読み込む Google Fonts の書体（plain は index.html で読み込み済み） */
export const LOOK_FONTS: Record<Look, string | null> = {
  plain: null,
  book: 'family=Shippori+Mincho:wght@700&family=Zen+Kaku+Gothic+New:wght@400;700',
  window: 'family=DotGothic16&family=BIZ+UDPGothic:wght@400;700',
  board: 'family=Zen+Kaku+Gothic+New:wght@400;700',
}

/** URL の検索部分（例 "?look=book"）から案を読む。知らない名前や指定なしは既定の案 */
export function lookFromSearch(search: string): Look {
  const v = new URLSearchParams(search).get('look')
  return LOOKS.find((l) => l === v) ?? DEFAULT_LOOK
}

/** 画面のパス（ハッシュの中。例 "/plan/path/go"）に当てる案。作業ビューのときだけ当て、ほかは当てない（null） */
export function lookForPath(pathname: string, look: Look): Look | null {
  return pathname === '/plan' || pathname.startsWith('/plan/') ? look : null
}
