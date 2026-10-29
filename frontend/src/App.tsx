import { NavLink, Route, Routes } from 'react-router'
import { DEFAULT_REPO_URL, loadData } from './data/load'
import { resolveToday } from './data/today'
import { PublicView } from './features/public/PublicView'
import { PlanView } from './features/plan/PlanView'

// 画面の入口は 2 つ（SPEC.md §7）。
// 公開ビュー（/）は初めて見る人（SNS・個人サイトから来た人、友人、採用担当者）向け、作業ビュー（/plan）は自分向け。
// 作業ビューの入口の名前は「学習パス」（中身が学習パスと項目詳細だけになったため。2026-10-13 決定）
// データはどちらも同じ data/*.json をビルド時に取り込む（見本の画面は examples/demo/data。vite.config.ts）
const data = loadData()

/** 画面の全体（ヘッダ・見本の注記・2 つの入口）。今日は settings の fixedToday があればその日に固定する */
function App() {
  const today = resolveToday(data.settings, new Date())
  const notice = data.settings.site?.notice
  return (
    <div className="page">
      {/* ヘッダは画面の切り替えとリポジトリへのリンクだけ。個人名・プロフィールは置かない（SPEC.md §7.3） */}
      <header className="top">
        <span>skill-matrix</span>
        <nav aria-label="画面の切り替え">
          <NavLink to="/" end>
            理解度台帳
          </NavLink>
          <NavLink to="/plan">学習パス</NavLink>
          <a href={data.settings.site?.repoUrl ?? DEFAULT_REPO_URL} target="_blank" rel="noopener">
            GitHub
          </a>
        </nav>
      </header>
      {/* 見本の画面だけに出す 1 行（settings.json の site.notice）。本物の data/ には書かないので出ない */}
      {notice && <p className="notice">{notice}</p>}
      <main>
        <Routes>
          <Route index element={<PublicView data={data} />} />
          <Route path="plan/*" element={<PlanView data={data} today={today} />} />
          <Route path="*" element={<PublicView data={data} />} />
        </Routes>
      </main>
    </div>
  )
}

export default App
