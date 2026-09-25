import { NavLink, Route, Routes } from 'react-router'
import { loadData } from './data/load'
import { PublicView } from './features/public/PublicView'
import { PlanView } from './features/plan/PlanView'

// 画面の入口は 2 つ（SPEC.md §7）。
// 公開ビュー（/）は採用担当者など初めて見る人向け、作業ビュー（/plan）は自分向け。
// データはどちらも同じ data/*.json をビルド時に取り込む
const data = loadData()

function App() {
  return (
    <div className="page">
      {/* ヘッダは画面の切り替えとリポジトリへのリンクだけ。個人名・プロフィールは置かない（SPEC.md §7.3） */}
      <header className="top">
        <span>skill-matrix</span>
        <nav aria-label="画面の切り替え">
          <NavLink to="/" end>
            理解度台帳
          </NavLink>
          <NavLink to="/plan">作業ビュー</NavLink>
          <a href="https://github.com/n-yoshida-dev/skill-matrix" target="_blank" rel="noopener">
            GitHub
          </a>
        </nav>
      </header>
      <main>
        <Routes>
          <Route index element={<PublicView data={data} />} />
          <Route path="plan/*" element={<PlanView data={data} />} />
          <Route path="*" element={<PublicView data={data} />} />
        </Routes>
      </main>
    </div>
  )
}

export default App
