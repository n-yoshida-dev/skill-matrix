import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { HashRouter } from 'react-router'
import './index.css'
import './looks.css'
import App from './App.tsx'

// ハッシュルーティング（#/plan）にするのは、GitHub Pages では深いパスの直リンクが 404 になるため（SPEC.md §8.2）
createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <HashRouter>
      <App />
    </HashRouter>
  </StrictMode>,
)
