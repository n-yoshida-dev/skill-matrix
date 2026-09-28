import { NavLink, Route, Routes } from 'react-router'
import type { AppData } from '../../data/types'
import { ItemView } from '../item/ItemView'
import { PathView } from '../path/PathView'
import { MatrixView } from './MatrixView'
import { NextActionsView } from './NextActionsView'
import { toDateKey } from './matrix'

// 作業ビュー（SPEC.md §7 の 3 画面）。自分向け。
// /plan はダッシュボード（サマリー帯・マトリクス・次にやること Top N）、/plan/path は学習パス、
// /plan/item/<項目キー> は項目詳細（ダッシュボード・学習パスから項目名を押して開く）。

interface Props {
  data: AppData
  /** 鮮度を測る基準日。省略すると今日（テストでは固定の日付を渡す） */
  today?: Date
}

export function PlanView({ data, today = new Date() }: Props) {
  return (
    <>
      <h1>作業ビュー</h1>
      <nav className="plan-tabs" aria-label="作業ビューの画面">
        <NavLink to="/plan" end>
          ダッシュボード
        </NavLink>
        <NavLink to="/plan/path">学習パス</NavLink>
      </nav>
      <Routes>
        <Route index element={<Dashboard data={data} today={today} />} />
        <Route path="path" element={<PathView data={data} today={today} />} />
        <Route path="path/:domainKey" element={<PathView data={data} today={today} />} />
        <Route path="item/:itemKey" element={<ItemView data={data} today={today} />} />
        <Route path="*" element={<Dashboard data={data} today={today} />} />
      </Routes>
    </>
  )
}

/** ダッシュボード。どこまで確かめたか・どこが古いか・次に何をやるかを 1 画面で見る */
function Dashboard({ data, today }: { data: AppData; today: Date }) {
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
