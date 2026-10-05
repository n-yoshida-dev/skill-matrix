import { useState } from 'react'
import { Link, NavLink, useParams } from 'react-router'
import type { AppData, ItemState, Level } from '../../data/types'
import { stateByKey, topEvidenced } from '../public/summary'
import { stalenessConfig } from '../plan/matrix'
import { nextActions, nextLimitFrom, stateOf, weightsFrom } from '../plan/next'
import { buildPath, type PathNode } from './path'
import { TreeView } from './TreeView'
import { readViewMode, writeViewMode, type PathViewMode } from './viewMode'
import './path.css'

// 学習パス（SPEC.md §7.2）。分野を選ぶと、その分野の項目を一列表示かスキルツリー表示で見せる。
// 一列表示は依存順に縦一列（次に何をやるか）、ツリー表示は前提が上・派生が下の階層図（どの前提を取ると何が解放されるか）。
// マトリクスが「今の状態」を見せるのに対し、こちらは「道筋」を見せる。
// 常に上部に分野の目標と進捗を出し、「あと何個でどうなるか」が視界から消えないようにする。
// 表示の選択はブラウザに記憶する（初回は一列表示）。

interface Props {
  data: AppData
  /** 「深掘り候補」（次にやること Top N）の計算に使う基準日。画面では今日、テストでは固定の日付を渡す */
  today: Date
}

/** 行頭の記号。済／今ここ／この先 */
const MARKS = { done: '✓', current: '▶', upcoming: '・' } as const

export function PathView({ data, today }: Props) {
  const { domainKey } = useParams()
  const [mode, setMode] = useState<PathViewMode>(() => readViewMode())
  const domains = data.roadmap.domains
  const domain = domains.find((d) => d.key === domainKey) ?? domains[0]
  if (!domain) {
    return <p className="plan-note">ロードマップに分野がない。</p>
  }

  const states = stateByKey(data)
  const path = buildPath(domain, states)
  // 済んだ項目のうち「次にやること」に入っているものは深掘り候補（「今ここ」とは別の問いに答える。SPEC.md §7.2）
  const top = nextActions(
    data.roadmap,
    states,
    today,
    stalenessConfig(data.settings),
    weightsFrom(data.settings),
    nextLimitFrom(data.settings),
  )
  const deepen = new Set(top.map((a) => a.itemKey))
  const names = new Map(domains.flatMap((d) => d.items.map((it) => [it.key, it.name])))

  /** 表示を切り替え、選択を記憶する */
  const choose = (m: PathViewMode) => {
    setMode(m)
    writeViewMode(m)
  }

  return (
    <section className="path-sec" aria-labelledby="path-title">
      <nav className="path-tabs" aria-label="分野の切り替え">
        {domains.map((d) => (
          <NavLink
            key={d.key}
            to={`/plan/path/${d.key}`}
            className={({ isActive }) =>
              isActive || (domainKey === undefined && d.key === domain.key) ? 'active' : undefined
            }
          >
            {d.name}
          </NavLink>
        ))}
      </nav>

      <header className="path-head">
        <h2 id="path-title">{path.domainName}</h2>
        <p className="goal">
          目標：{path.goal ? path.goal : <span className="unset">未記入</span>}
        </p>
        <p className="progress">
          進捗：<span className="num">{path.totalCount}</span> 項目中{' '}
          <span className="num">{path.doneCount}</span> 項目（レベル 1 以上）
        </p>
        <div className="path-mode" role="group" aria-label="表示の切り替え">
          <button type="button" aria-pressed={mode === 'list'} onClick={() => choose('list')}>
            一列
          </button>
          <button type="button" aria-pressed={mode === 'tree'} onClick={() => choose('tree')}>
            ツリー
          </button>
        </div>
      </header>

      {mode === 'list' ? (
        <>
          <ol className="path-list">
            {path.nodes.map((n) => (
              <PathRow
                key={n.item.key}
                n={n}
                deepen={n.status === 'done' && deepen.has(n.item.key)}
                externals={n.externalDeps.map((k) => ({
                  key: k,
                  name: names.get(k) ?? k,
                  st: stateOf(states, k),
                }))}
              />
            ))}
          </ol>
          <p className="plan-note path-legend">
            ✓ 済（レベル 1 以上）／▶
            今ここ（依存順で最初の、まだ済んでいない着手できる項目）／・この先。
            「深掘り候補」は、済んだ項目のうち「次にやること」に入っているもの。
          </p>
        </>
      ) : (
        <>
          <TreeView data={data} path={path} />
          {/* 凡例は短く残す。図は凡例なしで読めることを本人の読み取りテストで確かめてある（2026-10-05。KNOWLEDGE.md 2026-10-03） */}
          <p className="plan-note path-legend">
            上が前提、下がそれを前提にする項目。●の数がレベル（5
            段）。色の付いた線は、その前提が確認済み。
            節を押すと項目詳細、「他分野」の四角を押すとその分野のツリーへ移る。
          </p>
        </>
      )}
    </section>
  )
}

interface RowProps {
  n: PathNode
  deepen: boolean
  externals: { key: string; name: string; st: ItemState }[]
}

/** 学習パスの 1 行 */
function PathRow({ n, deepen, externals }: RowProps) {
  const { item, state: st, status } = n
  const top = topEvidenced(st)
  return (
    <li className={`path-node ${status}`}>
      <span className="mark" aria-hidden="true">
        {MARKS[status]}
      </span>
      <div className="body">
        <div className="head">
          <span className="k num">{item.key}</span>
          <Link className="nm" to={`/plan/item/${item.key}`}>
            {item.name}
          </Link>
          <span className="lvtxt">
            <LevelSwatch level={st.verifiedLevel} />
            レベル <span className="num">{st.verifiedLevel}</span>
          </span>
          {status === 'current' ? <span className="badge now">今ここ</span> : null}
          {deepen ? <span className="badge">深掘り候補</span> : null}
          {top > st.verifiedLevel ? (
            <span className="badge">上位の根拠あり（印 {st.evidencedLevels.join(', ')}）</span>
          ) : null}
          {/* 「まだ着手できない」ことを伝える札なので、済んだ項目には出さない */}
          {!n.ready && status !== 'done' ? <span className="badge dim">前提が未達</span> : null}
        </div>
        {item.outcome ? (
          <p className="outcome">身につくと：{item.outcome}</p>
        ) : (
          <p className="outcome unset">身につくと：未記入</p>
        )}
        {status === 'current' && item.verifyBy ? (
          <p className="verify">次の確認方法：{item.verifyBy}</p>
        ) : null}
        {externals.length > 0 ? (
          <p className="ext">
            ほかの分野の前提：
            {externals.map((e, i) => (
              <span key={e.key}>
                {i > 0 ? '、' : null}
                <span className="num">{e.key}</span> {e.name}（レベル {e.st.verifiedLevel}）
              </span>
            ))}
          </p>
        ) : null}
      </div>
    </li>
  )
}

/** マトリクスと同じ 6 階調の小さな見本 */
function LevelSwatch({ level }: { level: Level }) {
  return <i className={`sw${level >= 1 ? ` l${level}` : ''}`} aria-hidden="true" />
}
