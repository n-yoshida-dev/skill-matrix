import { Link } from 'react-router'
import type { AppData, ItemState } from '../../data/types'
import { shortName, topEvidenced } from '../public/summary'
import { needsAttention, stalenessConfig } from '../plan/matrix'
import { stateOf } from '../plan/next'
import { KIND_LABELS, layoutTree, nodeKind, type NodeKind } from './layout'
import type { Path } from './path'
import './tree.css'

// スキルツリー表示（SPEC.md §7.2）。同じ分野の項目を、前提が上・派生が下になる階層図として描く。
// 節は HTML の四角（文字の折り返しとキーボード操作のため）、線だけを SVG で引く。
// 節の状態は 4 つ：未解放（灰・鍵）／解放済み・未着手（枠を強調。今ここに札）／習得済み（塗り = verifiedLevel）／深掘り候補（札）。
// 他分野の前提は灰色の点線のゴーストノードで、押すとその分野のツリーへ移る。

/** 節の幅・高さと間隔（px） */
const NODE_W = 172
const NODE_H = 70
const GAP_X = 24
const GAP_Y = 40
const PAD = 4

interface Props {
  data: AppData
  path: Path
  /** 深掘り候補（済んだ項目のうち「次にやること」に入っているもの）の項目キー */
  deepen: Set<string>
  today: Date
}

/** 節の左上の座標 */
const xOf = (col: number) => PAD + col * (NODE_W + GAP_X)
const yOf = (row: number) => PAD + row * (NODE_H + GAP_Y)

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

  const width = layout.cols > 0 ? xOf(layout.cols - 1) + NODE_W + PAD : 0
  const height = layout.rows > 0 ? yOf(layout.rows - 1) + NODE_H + PAD : 0

  return (
    <div className="tree-scroll">
      <div className="tree" style={{ width, height }}>
        <svg className="tree-lines" width={width} height={height} aria-hidden="true">
          {layout.edges.map((e) => {
            const a = byKey.get(e.from)
            const b = byKey.get(e.to)
            if (!a || !b) return null
            const x1 = xOf(a.col) + NODE_W / 2
            const y1 = yOf(a.row) + NODE_H
            const x2 = xOf(b.col) + NODE_W / 2
            const y2 = yOf(b.row)
            const mid = (y2 - y1) / 2
            // 行き先の節が未解放なら線も薄く（SPEC.md §7.2）。前提が未達でも習得済みの節へ入る線は薄くしない
            const to = pathNode.get(e.to)
            const locked = to ? nodeKind(to.state, to.ready) === 'locked' : false
            const cls = ['edge', e.ghost ? 'ghost' : '', locked ? 'locked' : ''].filter(Boolean)
            return (
              <path
                key={`${e.from}->${e.to}`}
                className={cls.join(' ')}
                d={`M ${x1} ${y1} C ${x1} ${y1 + mid} ${x2} ${y2 - mid} ${x2} ${y2}`}
              />
            )
          })}
        </svg>
        {layout.nodes.map((n) => {
          const style = { left: xOf(n.col), top: yOf(n.row), width: NODE_W, height: NODE_H }
          if (n.ghost) {
            const st = stateOf(states, n.key)
            return (
              <Link
                key={n.key}
                to={`/plan/path/${n.domainKey}`}
                className="tnode ghost"
                style={style}
                title={`${domainNames.get(n.domainKey) ?? n.domainKey} の学習パスへ移る`}
              >
                <span className="tk">他分野：{domainNames.get(n.domainKey) ?? n.domainKey}</span>
                <span className="tn">{shortName(names.get(n.key) ?? n.key)}</span>
                <span className="ts">レベル {st.verifiedLevel}</span>
              </Link>
            )
          }
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
                {pn?.item.outcome ? null : <span className="tb unset">到達状態 未記入</span>}
              </span>
            </Link>
          )
        })}
      </div>
    </div>
  )
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
