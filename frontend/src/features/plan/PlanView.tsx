import { Navigate, Route, Routes } from 'react-router'
import type { AppData } from '../../data/types'
import { ItemView } from '../item/ItemView'
import { PathView } from '../path/PathView'
import './plan.css'

// 作業ビュー（SPEC.md §7）。自分向けで、画面に出す名前は「学習パス」（2026-10-13 決定）。
// /plan/path/<分野キー> は学習パス、/plan/item/<項目キー> は項目詳細（学習パスから項目名を押して開く）。
// /plan と /plan/path は最初の分野の学習パスへ移す（URL に分野キーがあると、分野のタブに aria-current が付く）。
// ダッシュボードは画面から外して Dashboard.tsx に棚上げした（SPEC.md §10.10）。

interface Props {
  data: AppData
  /** 鮮度を測る基準日。省略すると今日（テストでは固定の日付を渡す） */
  today?: Date
}

export function PlanView({ data, today = new Date() }: Props) {
  const first = data.roadmap.domains[0]?.key
  // 分野が 1 つも無ければ移さず、学習パスの「分野がない」を出す
  const toFirst = first ? (
    <Navigate to={`/plan/path/${first}`} replace />
  ) : (
    <PathView data={data} today={today} />
  )
  return (
    <>
      <h1>学習パス</h1>
      <Routes>
        <Route index element={toFirst} />
        <Route path="path" element={toFirst} />
        <Route
          path="path/:domainKey"
          element={
            <>
              {/* 説明の 1 行は学習パスの画面だけ（項目詳細ではパンくずが行き先を示す） */}
              <p className="plan-lead">
                自分用の画面です。次に何を学ぶかを分野ごとに並べています。
              </p>
              <PathView data={data} today={today} />
            </>
          }
        />
        <Route path="item/:itemKey" element={<ItemView data={data} today={today} />} />
        <Route path="*" element={toFirst} />
      </Routes>
    </>
  )
}
