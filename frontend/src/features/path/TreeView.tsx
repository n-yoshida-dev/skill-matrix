import { Link } from 'react-router'
import type { AppData, ItemState } from '../../data/types'
import { shortName, topEvidenced } from '../public/summary'
import { needsAttention, stalenessConfig } from '../plan/matrix'
import { stateOf } from '../plan/next'
import { KIND_LABELS, layoutTree, nodeKind, type NodeKind, type Point } from './layout'
import type { Path } from './path'
import './tree.css'

// スキルツリー表示（SPEC.md §7.2）。同じ分野の項目を、前提が上・派生が下になる階層図として描く。
// 節・ゴーストノード・線・合流点の位置はすべて layout.ts が計算し、ここはその位置に置くだけ。
// 節は HTML の四角（文字の折り返しとキーボード操作のため）、線と合流点だけを SVG で引く。
// 節の状態は 4 つ：未解放（灰・鍵）／解放済み・未着手（枠を強調。今ここに札）／習得済み（塗り = verifiedLevel）／深掘り候補（札）。
// 他分野の前提は灰色の点線のゴーストノードで、合流点の横に小さく置き、押すとその分野のツリーへ移る。

/** 線の曲がり角を丸める半径（px）。曲がり角と、線どうしの交差を見分けやすくする */
const CORNER_R = 4

interface Props {
  data: AppData
  path: Path
  /** 深掘り候補（済んだ項目のうち「次にやること」に入っているもの）の項目キー */
  deepen: Set<string>
  today: Date
}

export function TreeView({ data, path, deepen, today }: Props) {
  const domain = data.roadmap.domains.find((d) => d.key === path.domainKey)
  if (!domain) return null
  const layout = layoutTree(domain, data.roadmap.domains)
  const states = new Map(data.state.items.map((s) => [s.itemKey, s]))
  const cfg = stalenessConfig(data.settings)
  const byKey = new Map(layout.nodes.map((n) => [n.key, n]))
  const pathNode = new Map(path.nodes.map((n) => [n.item.key, n]))
  const names = new Map(data.roadmap.domains.flatMap((d) => d.items.map((it) => [it.key, it.name])))
  const domainNames = new Map(data.roadmap.domains.map((d) => [d.key, d.name]))
  // 行き先の節が未解放なら、そこへ入る線と合流点も薄く（SPEC.md §7.2）。前提が未達でも習得済みの節へ入る線は薄くしない
  const lockedTo = (key: string) => {
    const to = pathNode.get(key)
    return to ? nodeKind(to.state, to.ready) === 'locked' : false
  }

  return (
    <div className="tree-scroll">
      <div className="tree" style={{ width: layout.width, height: layout.height }}>
        <svg className="tree-lines" width={layout.width} height={layout.height} aria-hidden="true">
          {layout.edges.map((e) => {
            const cls = ['edge', e.ghost ? 'ghost' : '', lockedTo(e.to) ? 'locked' : ''].filter(
              Boolean,
            )
            return (
              <path key={`${e.from}->${e.to}`} className={cls.join(' ')} d={pathData(e.points)} />
            )
          })}
          {layout.junctions.map((j) => {
            const to = byKey.get(j.to)
            if (!to) return null
            const cls = ['junction', lockedTo(j.to) ? 'locked' : ''].filter(Boolean)
            // 横棒と、合流点から項目の上辺へ下りる 1 本。合流点には小さな点を打つ
            return (
              <g key={`junction-${j.to}`} className={cls.join(' ')}>
                <path d={`M ${j.x1} ${j.y} H ${j.x2} M ${j.x} ${j.y} V ${to.y}`} />
                <circle cx={j.x} cy={j.y} r={3} />
              </g>
            )
          })}
        </svg>
        {layout.ghosts.map((g) => {
          const st = stateOf(states, g.key)
          return (
            <Link
              key={`${g.key}@${g.to}`}
              to={`/plan/path/${g.domainKey}`}
              className="tnode ghost"
              style={{ left: g.x, top: g.y, width: g.w, height: g.h }}
              title={`${domainNames.get(g.domainKey) ?? g.domainKey} の学習パスへ移る`}
            >
              <span className="tk">他分野：{domainNames.get(g.domainKey) ?? g.domainKey}</span>
              <span className="tn">{shortName(names.get(g.key) ?? g.key)}</span>
              <span className="ts">レベル {st.verifiedLevel}</span>
            </Link>
          )
        })}
        {layout.nodes.map((n) => {
          const style = { left: n.x, top: n.y, width: n.w, height: n.h }
          const pn = pathNode.get(n.key)
          const st = pn?.state ?? stateOf(states, n.key)
          const kind = nodeKind(st, pn?.ready ?? true)
          const cls = nodeClass(kind, st, needsAttention(st, today, cfg), pn?.status === 'current')
          return (
            <Link
              key={n.key}
              to={`/plan/item/${n.key}`}
              className={cls}
              style={style}
              aria-label={`${n.key} ${names.get(n.key) ?? ''}（${KIND_LABELS[kind]}・レベル ${st.verifiedLevel}）`}
            >
              <span className="tk">{n.key}</span>
              <span className="tn">{shortName(names.get(n.key) ?? n.key)}</span>
              <span className="ts">
                {kind === 'locked' ? <LockIcon /> : null}
                レベル {st.verifiedLevel}
                {pn?.status === 'current' ? <span className="tb now">今ここ</span> : null}
                {deepen.has(n.key) && kind === 'done' ? (
                  <span className="tb">深掘り候補</span>
                ) : null}
                {/* 到達状態が空の項目は「未記入」と分かるように（SPEC.md §7.2「両表示に共通の制約」） */}
                {pn?.item.outcome ? null : <span className="tb blank">到達状態 未記入</span>}
              </span>
            </Link>
          )
        })}
      </div>
    </div>
  )
}

/** 縦横の折れ線を、曲がり角を少し丸めた SVG の path にする（半径は両側の線分の半分を超えない） */
function pathData(points: Point[], radius = CORNER_R): string {
  if (points.length === 0) return ''
  const parts = [`M ${points[0].x} ${points[0].y}`]
  for (let i = 1; i < points.length - 1; i++) {
    const prev = points[i - 1]
    const p = points[i]
    const next = points[i + 1]
    const r = Math.min(radius, dist(prev, p) / 2, dist(p, next) / 2)
    const a = toward(p, prev, r)
    const b = toward(p, next, r)
    parts.push(`L ${a.x} ${a.y}`, `Q ${p.x} ${p.y} ${b.x} ${b.y}`)
  }
  const end = points[points.length - 1]
  if (points.length > 1) parts.push(`L ${end.x} ${end.y}`)
  return parts.join(' ')
}

/** 2 点の距離 */
const dist = (a: Point, b: Point) => Math.hypot(b.x - a.x, b.y - a.y)

/** from から to へ向かって r だけ進んだ点 */
function toward(from: Point, to: Point, r: number): Point {
  const d = dist(from, to)
  if (d === 0) return from
  return { x: from.x + ((to.x - from.x) * r) / d, y: from.y + ((to.y - from.y) * r) / d }
}

/** 節のクラス。塗り（l1〜l5）・要再確認（attn）・上位の根拠（up up-lN）はマトリクスと同じ符号化 */
function nodeClass(kind: NodeKind, st: ItemState, attention: boolean, current: boolean): string {
  const cls = ['tnode', kind]
  if (st.verifiedLevel >= 1) cls.push(`l${st.verifiedLevel}`)
  if (attention) cls.push('attn')
  const top = topEvidenced(st)
  if (top > st.verifiedLevel) cls.push('up', `up-l${top}`)
  if (current) cls.push('current')
  return cls.join(' ')
}

/** 鍵の形（未解放の印） */
function LockIcon() {
  return (
    <svg className="lock" width="10" height="11" viewBox="0 0 10 11" aria-hidden="true">
      <rect x="1" y="5" width="8" height="6" rx="1" />
      <path d="M3 5 V3.5 a2 2 0 0 1 4 0 V5" fill="none" strokeWidth="1.4" />
    </svg>
  )
}
