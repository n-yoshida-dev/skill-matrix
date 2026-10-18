import type { AppData } from '../../data/types'
import { MatrixView } from './MatrixView'
import { NextActionsView } from './NextActionsView'
import { toDateKey } from './matrix'

// 作業ビューのダッシュボード（サマリー帯・マトリクス・次にやること Top N）。
// 2026-10-13 に画面から外し、棚上げした（SPEC.md §10.10、logs/decisions.md 2026-10-13）。どこからも呼ばない。
// 学習パスだけでは次にやることが選びにくくなったら、PlanView の経路に戻せるよう消さずに残している。

interface Props {
  data: AppData
  /** 鮮度を測る基準日 */
  today: Date
}

/** ダッシュボード。どこまで確かめたか・どこが古いか・次に何をやるかを 1 画面で見る */
export function Dashboard({ data, today }: Props) {
  const total = data.roadmap.domains.reduce((n, d) => n + d.items.length, 0)
  return (
    <>
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
