import { useEffect, useState, type MouseEvent } from 'react'
import type { DomainOverview } from './summary'
import {
  LABEL_FONT,
  RADAR,
  axisPoint,
  easeOut,
  labelLines,
  labelPlace,
  ringPoints,
  toPoints,
  valuePoints,
} from './radar'
import { prefersReducedMotion } from './motion'

// 公開ビューの 1 層目の「図で見る」（能力値チャート。2026-10-12 決定）。
// 左に 8 分野の多角形（各分野で確認できた一番高い段を結ぶ）、右に分野の一覧（到達の段と「根拠のある項目 / 全項目」）。
// 図の分野名はマウスで押せるが、キーボードの止まり場所と読み上げは一覧だけにする（同じリンクを 2 回並べない）

interface Props {
  domains: DomainOverview[]
  /** 分野を押したときに、その分野のタイルへ移る */
  onJump: (domainKey: string) => void
  /** 切り替えて出したときだけ、多角形が中心から広がる（最初から図を出すことは無い） */
  animate: boolean
}

/** 広がる動きの長さ（ミリ秒）。試作と同じ */
const GROW_MS = 550

/** 多角形の大きさの倍率（0〜1）。animate なら 0 から 1 へ GROW_MS で広げる。動きを減らす設定では最初から 1 */
function useGrowth(animate: boolean): number {
  const [t, setT] = useState(() => (animate && !prefersReducedMotion() ? 0 : 1))
  useEffect(() => {
    if (t >= 1) return
    let frame = 0
    const start = performance.now()
    const step = (now: number) => {
      const p = Math.min(1, (now - start) / GROW_MS)
      setT(easeOut(p))
      if (p < 1) frame = requestAnimationFrame(step)
    }
    frame = requestAnimationFrame(step)
    return () => cancelAnimationFrame(frame)
    // 出したときに 1 回だけ動かす
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])
  return t
}

export function RadarChart({ domains, onJump, animate }: Props) {
  const t = useGrowth(animate)
  const n = domains.length
  const area = valuePoints(
    domains.map((d) => d.top),
    t,
  )

  /** リンクの既定の動き（HashRouter の経路が変わる）を止めて、同じページの中で移る */
  const jump = (e: MouseEvent<Element>, key: string) => {
    e.preventDefault()
    onJump(key)
  }

  return (
    <div className="lb">
      <svg
        className="lb-svg"
        viewBox={`0 0 ${RADAR.width} ${RADAR.height}`}
        role="img"
        aria-label={`${n} 分野で確認できた一番高い段を結んだ多角形。値は図の横（狭い画面では下）の一覧と同じ`}
      >
        {[1, 2, 3, 4, 5].map((lv) => (
          <polygon
            key={lv}
            className={`ring${lv === 5 ? ' outer' : ''}`}
            points={toPoints(ringPoints(n, lv))}
          />
        ))}
        {domains.map((d, k) => {
          const [x2, y2] = axisPoint(k, n, RADAR.radius)
          return <line key={d.key} className="axis" x1={RADAR.cx} y1={RADAR.cy} x2={x2} y2={y2} />
        })}
        <polygon className="area" points={toPoints(area)} />
        {area.map(([x, y], k) => (
          <circle key={domains[k].key} className="dot" cx={x} cy={y} r={8} />
        ))}
        {[1, 3, 5].map((lv) => (
          <text
            key={lv}
            className="tick"
            x={RADAR.cx - 14}
            y={RADAR.cy - (RADAR.radius * lv) / 5 + LABEL_FONT / 3}
            textAnchor="end"
          >
            {lv}
          </text>
        ))}
        {domains.map((d, k) => {
          const lines = labelLines(d.name)
          const p = labelPlace(k, n, lines.length)
          return (
            <a
              key={d.key}
              href={`#d-${d.key}`}
              tabIndex={-1}
              aria-hidden="true"
              onClick={(e) => jump(e, d.key)}
            >
              <text className="lab" x={p.x} y={p.y} textAnchor={p.anchor}>
                {lines.map((line, i) => (
                  <tspan key={i} x={p.x} dy={i === 0 ? 0 : p.lineHeight}>
                    {line}
                  </tspan>
                ))}
              </text>
            </a>
          )
        })}
      </svg>
      <div className="lb-list">
        <div className="lb-h" aria-hidden="true">
          <span>分野</span>
          <span>到達</span>
          <span>根拠</span>
        </div>
        <ul>
          {domains.map((d) => (
            <li key={d.key}>
              <a className="lb-row" href={`#d-${d.key}`} onClick={(e) => jump(e, d.key)}>
                <span className="lb-n">{d.name}</span>{' '}
                <span className="lb-l">
                  <i className={`swatch${d.top >= 1 ? ` l${d.top}` : ''}`} aria-hidden="true" />
                  <b className="num">L{d.top}</b>
                </span>{' '}
                {/* 「2/9」は読み上げで意味が通らないので、読み上げには文で渡す */}
                <span className="lb-c num" aria-hidden="true">
                  {d.withEvidence}/{d.total}
                </span>
                <span className="sr-only">
                  {d.total} 項目中 {d.withEvidence} に根拠
                </span>
              </a>
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}
