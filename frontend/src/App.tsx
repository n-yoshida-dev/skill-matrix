import { useEffect } from 'react'
import { NavLink, Route, Routes, useLocation } from 'react-router'
import { DEFAULT_REPO_URL, loadData } from './data/load'
import { PublicView } from './features/public/PublicView'
import { PlanView } from './features/plan/PlanView'
import { LOOK_FONTS, lookForPath, lookFromSearch, type Look } from './look'

// 画面の入口は 2 つ（SPEC.md §7）。
// 公開ビュー（/）は初めて見る人（SNS・個人サイトから来た人、友人、採用担当者）向け、作業ビュー（/plan）は自分向け。
// データはどちらも同じ data/*.json をビルド時に取り込む
const data = loadData()

function App() {
  const { pathname } = useLocation()
  useLook(lookForPath(pathname, lookFromSearch(window.location.search)))
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
          <a href={data.settings.site?.repoUrl ?? DEFAULT_REPO_URL} target="_blank" rel="noopener">
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

/**
 * 見た目の案を <html data-look="…"> に当て、案の書体を読み込む（look.ts）。
 * 印を html に付けるのは、地の色（body の背景）とページ全体の書体まで案で変えるため
 */
function useLook(look: Look | null) {
  useEffect(() => {
    const root = document.documentElement
    if (look) root.dataset.look = look
    else delete root.dataset.look
    const family = look ? LOOK_FONTS[look] : null
    if (!family) return
    const id = `look-font-${look}`
    if (document.getElementById(id)) return
    const link = document.createElement('link')
    link.id = id
    link.rel = 'stylesheet'
    link.href = `https://fonts.googleapis.com/css2?${family}&display=swap`
    document.head.appendChild(link)
  }, [look])
}

export default App
