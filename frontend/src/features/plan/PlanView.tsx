import type { AppData } from '../../data/types'
import { MatrixView } from './MatrixView'
import { NextActionsView } from './NextActionsView'
import { toDateKey } from './matrix'

// 作業ビュー（SPEC.md §7 の 3 画面）。自分向け。画面は TODO 2-5 で順に足す。
// 今あるもの：マトリクス（サマリー帯・可変長グリッド）と次にやること Top N。
// これから足すもの：学習パス、項目詳細。

interface Props {
  data: AppData
  /** 鮮度を測る基準日。省略すると今日（テストでは固定の日付を渡す） */
  today?: Date
}

export function PlanView({ data, today = new Date() }: Props) {
  const total = data.roadmap.domains.reduce((n, d) => n + d.items.length, 0)
  return (
    <>
      <h1>作業ビュー</h1>
      <p className="plan-lead">
        自分向けの画面。どこまで確かめたか・どこが古いかを見る。鮮度は今日（
        <span className="num">{toDateKey(today)}</span>）から数えている。
      </p>
      <p className="plan-lead">
        読み込んでいるデータ：{data.roadmap.name}（{data.roadmap.domains.length} 分野 {total}{' '}
        項目、判定の保留 {data.state.deferred.length} 件、棄却 {data.state.rejected.length} 件）
      </p>
      <MatrixView data={data} today={today} />
      <NextActionsView data={data} today={today} />
    </>
  )
}
