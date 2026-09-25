import { useState } from 'react'
import type { AppData, ItemState, RoadmapItem } from '../../data/types'
import {
  EVIDENCE_LABELS,
  LEVEL_NAMES,
  isImplementedButUnverified,
  shortName,
  splitEvents,
  stateByKey,
  summarizeDomains,
  summarizeOverall,
  topEvidenced,
} from './summary'
import './public.css'

// 公開ビュー（SPEC.md §7.3）。採用担当者など初めて見る人向けの 1 画面。
// 出すのは「何をどこまでできるか」と「その根拠」だけ。段階前の状態・鮮度・保留・優先度・検証の記号は出さない。

interface Props {
  data: AppData
}

export function PublicView({ data }: Props) {
  const states = stateByKey(data)
  const overall = summarizeOverall(data)
  const domains = summarizeDomains(data)
  // 押されて根拠が開いている項目。分野をまたいで 1 つだけ
  const [openKey, setOpenKey] = useState<string | null>(null)

  return (
    <>
      <h1>理解度台帳</h1>
      <p className="thesis">
        学習ログを AI
        が読んで理解度を判定し、ルールで検証したうえで履歴に残しています。自己申告ではレベルが上がりません。
      </p>
      <p className="thesis sub">
        {overall.domains} 分野 {overall.items} 項目のうち、根拠のある項目が {overall.withEvidence}
        。そのうち実装まで届いているものが {overall.canImplement}。
      </p>

      {data.roadmap.domains.map((domain, i) => {
        const summary = domains[i]
        const openItem = domain.items.find((it) => it.key === openKey)
        const openState = openItem ? states.get(openItem.key) : undefined
        return (
          <section className="domain" key={domain.key}>
            <div className="domain-head">
              <h2>{domain.name}</h2>
              {domain.goal ? <span className="goal">{domain.goal}</span> : null}
              <span className="count">
                {summary.total} 項目中 {summary.withEvidence} に根拠
              </span>
            </div>
            <div className="cells">
              {domain.items.map((it) => {
                const st = states.get(it.key)
                if (!st) return null
                const expanded = openKey === it.key
                return (
                  <button
                    key={it.key}
                    type="button"
                    className={`cell${st.verifiedLevel >= 1 ? ` l${st.verifiedLevel}` : ''}`}
                    aria-expanded={expanded}
                    aria-controls={`detail-${domain.key}`}
                    onClick={() => setOpenKey(expanded ? null : it.key)}
                  >
                    <span className="n num">{it.key}</span>
                    <span className="lv num">{tileLevelLabel(st)}</span>
                    <span className="name">{shortName(it.name)}</span>
                  </button>
                )
              })}
            </div>
            {openItem && openState ? (
              <div className="detail" id={`detail-${domain.key}`}>
                <Detail item={openItem} state={openState} onClose={() => setOpenKey(null)} />
              </div>
            ) : null}
          </section>
        )
      })}

      <section className="scale">
        <h2>色の読み方</h2>
        <div className="steps">
          {([0, 1, 2, 3, 4, 5] as const).map((l) => (
            <div className="step" key={l}>
              <i className={l >= 1 ? `l${l}` : ''} />
              <div>
                <b className="num">L{l}</b>
                <span>{LEVEL_NAMES[l]}</span>
              </div>
            </div>
          ))}
        </div>
      </section>

      <section className="how">
        <h2>どうやって決まるか</h2>
        <ol>
          <li>
            学習した日に、やったこと・理解したことを文章で書く{' '}
            <span>（ログ本文はここには載せていません）</span>
          </li>
          <li>
            AI がそのログを読み、どの項目にどれくらいの理解が示されたかを、根拠の種類つきで提案する
          </li>
          <li>
            提案は機械的に検証される{' '}
            <span>
              （根拠の種類ごとに付けられる段が決まっている。「読んだ」「聞いた」「業務でやったことがある」だけでは上がらない。実装したからといって基礎を説明できるとは見なさない）
            </span>
          </li>
          <li>通った判定だけが履歴に残り、この表の色になる</li>
        </ol>
      </section>

      <footer className="foot">
        <span>
          Built with{' '}
          <a href="https://github.com/n-yoshida-dev/skill-matrix" target="_blank" rel="noopener">
            skill-matrix
          </a>
        </span>
        {data.roadmap.description ? <span>{data.roadmap.description}</span> : null}
      </footer>
    </>
  )
}

/** タイル右上の文字。レベルか、未着手か、実装の根拠だけがある状態か */
function tileLevelLabel(st: ItemState): string {
  if (st.verifiedLevel >= 1) return `L${st.verifiedLevel}`
  if (isImplementedButUnverified(st)) return '実装済み・基礎未確認'
  return '未着手'
}

interface DetailProps {
  item: RoadmapItem
  state: ItemState
  onClose: () => void
}

/** タイルを押したときに、その分野の行の直下に開く根拠 */
function Detail({ item, state, onClose }: DetailProps) {
  const { marked, unmarked } = splitEvents(state)
  const pending = isImplementedButUnverified(state)
  return (
    <>
      <div>
        <div className="key num">{item.key}</div>
        <h3>{item.name}</h3>
        <div className="level">
          <span className={`swatch${state.verifiedLevel >= 1 ? ` l${state.verifiedLevel}` : ''}`} />
          <span>
            <span className="num">L{state.verifiedLevel}</span>　{LEVEL_NAMES[state.verifiedLevel]}
          </span>
        </div>
        {pending ? (
          <p className="pending">
            実装の根拠（L{topEvidenced(state)} まで）はありますが、基礎の確認が未了です。
          </p>
        ) : null}
        <div className="outcome">
          <span>身につくと</span>
          <br />
          {item.outcome ? item.outcome : <span>（未記入）</span>}
        </div>
      </div>
      <div className="evidence">
        {marked.length > 0 ? (
          <>
            <div className="head">印が付いた根拠</div>
            {marked.map((ev) => (
              <div className="ev" key={`${ev.file}-${ev.index}`}>
                <div className="d num">{ev.occurredAt}</div>
                <div>
                  <div className="t">
                    L{ev.marked.join('・')}　{EVIDENCE_LABELS[ev.evidenceType]}
                  </div>
                  <div className="r">{ev.rationale}</div>
                </div>
              </div>
            ))}
          </>
        ) : unmarked.length > 0 ? (
          <div className="head">まだレベルは上がっていません</div>
        ) : (
          <div className="head">まだ手をつけていません</div>
        )}
        {unmarked.map((ev) => (
          <div className="ev" key={`${ev.file}-${ev.index}`}>
            <div className="d num">{ev.occurredAt}</div>
            <div>
              <div className="t">{EVIDENCE_LABELS[ev.evidenceType]}</div>
              <div className="r dim">{ev.rationale}。これだけではレベルは上がらない扱いです</div>
            </div>
          </div>
        ))}
        <button className="close" type="button" onClick={onClose}>
          閉じる
        </button>
      </div>
    </>
  )
}
