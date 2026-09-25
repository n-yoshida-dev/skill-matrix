import type { AppData } from '../../data/types'

// 作業ビュー（SPEC.md §7 の 3 画面）。自分向け。まだ土台だけで、画面は TODO 2-5 で順に足す。
// ここに置くもの：マトリクス（サマリー帯・可変長グリッド・次にやること）、学習パス、項目詳細。

interface Props {
  data: AppData
}

export function PlanView({ data }: Props) {
  const total = data.roadmap.domains.reduce((n, d) => n + d.items.length, 0)
  return (
    <>
      <h1>作業ビュー</h1>
      <p>
        自分向けの画面。次にやること・学習パス・鮮度・保留をここに出す。まだ土台だけで、画面はこれから作る。
      </p>
      <p>
        読み込んでいるデータ：{data.roadmap.name}（{data.roadmap.domains.length} 分野 {total} 項目、
        判定の保留 {data.state.deferred.length} 件、棄却 {data.state.rejected.length} 件）
      </p>
      {data.roadmap.description ? <p>{data.roadmap.description}</p> : null}
    </>
  )
}
