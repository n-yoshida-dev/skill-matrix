import { Link } from 'react-router'
import type { AppData, ItemState } from '../../data/types'
import { shortName } from '../public/summary'
import { isReady, stateOf } from '../plan/next'
import {
  basicsUnverified,
  KIND_LABELS,
  layoutTree,
  levelDots,
  nodeKind,
  type NodeKind,
  type Point,
} from './layout'
import type { Path } from './path'
import { isJunctionOpen, isLit, junctionCount, junctionLabel, missingPrereqs } from './unlock'
import './tree.css'

// スキルツリー表示（SPEC.md §7.2）。同じ分野の項目を、前提が上・派生が下になる階層図として描く。
// 節・ゴーストノード・線・合流点の位置はすべて layout.ts が計算し、ここはその位置に置くだけ。
// 節は HTML の四角（文字の折り返しとキーボード操作のため）、線と合流点だけを SVG で引く。
// 節の状態は 3 つ（2026-10-03 本人了承の 1）：前提待ち（灰・鍵・「必要：」）／挑戦できる（太い枠と文字）／確認済み（塗り 1 色と●の数＝レベル）。
// 要再確認の枠・上位の根拠の三角・レベルによる色の濃淡・点線・深掘り候補・今ここはツリーに出さない（同じ了承の 5・6）。
// 「実装の根拠あり・基礎は未確認」だけは文字で出す（同じ了承の 5）。
// 他分野の前提はゴーストノードで、合流点の横に小さく置き、押すとその分野のツリーへ移る。状態は分野の項目と同じ 3 つで見せる。
// 解放の条件の見せ方（2026-10-03 本人了承の 2・3）：前提が確認済みの線に色を付け、合流点に「n つとも必要 x/n」、
// 鍵の項目に確認済みでない直接の前提の名前（「必要：○○」）を出す。どれも unlock.ts の同じ判定から出す。

/** 線の曲がり角を丸める半径（px）。曲がり角と、線どうしの交差を見分けやすくする */
const CORNER_R = 4

interface Props {
  data: AppData
  path: Path
}

export function TreeView({ data, path }: Props) {
  const domain = data.roadmap.domains.find((d) => d.key === path.domainKey)
  if (!domain) return null
  const layout = layoutTree(domain, data.roadmap.domains)
  const states = new Map(data.state.items.map((s) => [s.itemKey, s]))
  const byKey = new Map(layout.nodes.map((n) => [n.key, n]))
  const pathNode = new Map(path.nodes.map((n) => [n.item.key, n]))
  const items = new Map(data.roadmap.domains.flatMap((d) => d.items.map((it) => [it.key, it])))
  const domainNames = new Map(data.roadmap.domains.map((d) => [d.key, d.name]))
  const nameOf = (key: string) => shortName(items.get(key)?.name ?? key)
  // 色の付いた線を後から描き、交差するところでは色の付いた線が上に来るようにする
  const edges = [...layout.edges].sort(
    (a, b) => Number(isLit(a, states)) - Number(isLit(b, states)),
  )

  return (
    <div className="tree-scroll">
      <div className="tree" style={{ width: layout.width, height: layout.height }}>
        <svg className="tree-lines" width={layout.width} height={layout.height} aria-hidden="true">
          {edges.map((e) => (
            <path
              key={`${e.from}->${e.to}`}
              className={isLit(e, states) ? 'edge lit' : 'edge'}
              d={pathData(e.points)}
            />
          ))}
          {layout.junctions.map((j) => {
            const to = byKey.get(j.to)
            if (!to) return null
            const count = junctionCount(j.to, layout.edges, states)
            const cls = ['junction', isJunctionOpen(count) ? 'open' : ''].filter(Boolean)
            // 横棒と、合流点から項目の上辺へ下りる 1 本。合流点には小さな点を打ち、下りる線の右に「n つとも必要 x/n」
            return (
              <g key={`junction-${j.to}`} className={cls.join(' ')}>
                <path d={`M ${j.x1} ${j.y} H ${j.x2} M ${j.x} ${j.y} V ${to.y}`} />
                <circle cx={j.x} cy={j.y} r={3} />
                <text x={j.label.x} y={j.label.y + j.label.h - 3}>
                  {junctionLabel(count)}
                </text>
              </g>
            )
          })}
        </svg>
        {layout.ghosts.map((g) => {
          const st = stateOf(states, g.key)
          // 他分野の項目が着手できるかは、その分野での前提で決まる（分野の項目と同じ判定）
          const it = items.get(g.key)
          const kind = nodeKind(st, it ? isReady(states, it) : true)
          const domainName = domainNames.get(g.domainKey) ?? g.domainKey
          return (
            <Link
              key={`${g.key}@${g.to}`}
              to={`/plan/path/${g.domainKey}`}
              className={`tnode ghost ${kind}`}
              style={{ left: g.x, top: g.y, width: g.w, height: g.h }}
              title={`${domainName} の学習パスへ移る`}
              aria-label={`他分野：${domainName} ${it?.name ?? g.key}（${stateText(kind, st)}）`}
            >
              <span className="tk">他分野：{domainName}</span>
              <span className="tn">{nameOf(g.key)}</span>
              <span className="ts">
                <StateMark kind={kind} st={st} />
              </span>
            </Link>
          )
        })}
        {layout.nodes.map((n) => {
          const style = { left: n.x, top: n.y, width: n.w, height: n.h }
          const pn = pathNode.get(n.key)
          const st = pn?.state ?? stateOf(states, n.key)
          const kind = nodeKind(st, pn?.ready ?? true)
          // 鍵の項目には、確認済みでない直接の前提の名前を出す（レベルは 0 と決まっているので出さない）
          const need =
            kind === 'locked' && pn
              ? `必要：${missingPrereqs(pn.item, states).map(nameOf).join('、')}`
              : ''
          const basics = basicsUnverified(st)
          return (
            <Link
              key={n.key}
              to={`/plan/item/${n.key}`}
              className={`tnode ${kind}`}
              style={style}
              aria-label={`${n.key} ${items.get(n.key)?.name ?? ''}（${stateText(kind, st)}）${need}${basics ? `。${BASICS_TEXT}` : ''}`}
            >
              <span className="tk">{n.key}</span>
              <span className="tn">{nameOf(n.key)}</span>
              <span className="ts">
                {need ? (
                  <>
                    <LockIcon />
                    <span className="need">{need}</span>
                  </>
                ) : (
                  <StateMark kind={kind} st={st} />
                )}
                {basics ? <span className="note">{BASICS_TEXT}</span> : null}
                {/* 到達状態が空の項目は「未記入」と分かるように（SPEC.md §7.2「両表示に共通の制約」） */}
                {pn?.item.outcome ? null : <span className="tb">到達状態 未記入</span>}
              </span>
            </Link>
          )
        })}
      </div>
    </div>
  )
}

/** 上位の根拠がある項目に添える文（SPEC.md §7.2。一列表示のバッジの代わり） */
const BASICS_TEXT = '実装の根拠あり・基礎は未確認'

/** 状態を読み上げる文。確認済みはレベルも添える */
function stateText(kind: NodeKind, st: ItemState): string {
  return kind === 'done' ? `${KIND_LABELS.done}・レベル ${st.verifiedLevel}` : KIND_LABELS[kind]
}

/** 状態の印。確認済み＝●の数と「Lv n」、挑戦できる＝文字、前提待ち＝鍵と文字（ゴーストノード用。分野の項目は「必要：」を出す） */
function StateMark({ kind, st }: { kind: NodeKind; st: ItemState }) {
  if (kind === 'done') {
    return (
      <span className="lv">
        <span className="dots" aria-hidden="true">
          {levelDots(st.verifiedLevel)}
        </span>
        Lv{st.verifiedLevel}
      </span>
    )
  }
  if (kind === 'open') return <span className="challenge">{KIND_LABELS.open}</span>
  return (
    <>
      <LockIcon />
      {KIND_LABELS.locked}
    </>
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

/** 鍵の形（未解放の印） */
function LockIcon() {
  return (
    <svg className="lock" width="10" height="11" viewBox="0 0 10 11" aria-hidden="true">
      <rect x="1" y="5" width="8" height="6" rx="1" />
      <path d="M3 5 V3.5 a2 2 0 0 1 4 0 V5" fill="none" strokeWidth="1.4" />
    </svg>
  )
}
