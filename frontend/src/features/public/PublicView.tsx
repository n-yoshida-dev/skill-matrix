import { useEffect, useRef, useState, type MouseEvent } from 'react'
import type { AppData, ItemState, RoadmapItem } from '../../data/types'
import {
  EVIDENCE_LABELS,
  IMPLEMENTED_BUT_UNVERIFIED,
  LEVEL_NAMES,
  domainOverview,
  isImplementedButUnverified,
  shortName,
  splitEvents,
  stateByKey,
  summarizeDomains,
  topEvidenced,
} from './summary'
import { Overview } from './Overview'
import { prefersReducedMotion } from './motion'
import { DEFAULT_REPO_URL } from '../../data/load'
import './public.css'

// 公開ビュー（SPEC.md §7.3）。初めて見る人（SNS・個人サイトから来た人、友人、採用担当者）向けの 1 画面。
// 出すのは「何をどこまでできるか」と「その根拠」だけ。段階前の状態・鮮度・保留・優先度・検証の記号は出さない。
// 並びは 3 層：1 層目は分野ごとの要約（Overview）、2 層目は分野ごとのタイル、3 層目はタイルを押すと開く根拠
// （2026-10-12 決定。いきなり 46 枚のタイルから始まり、何を見たらいいか分かりづらかった）

interface Props {
  data: AppData
}

/** 要素までページの中で移り、移り先の見出しにフォーカスを置く（読み上げとキーボードでも移った先が分かるように） */
function moveTo(id: string): void {
  const el = document.getElementById(id)
  if (!el) return
  el.scrollIntoView?.({ behavior: prefersReducedMotion() ? 'auto' : 'smooth', block: 'start' })
  const heading = el.matches('h2') ? el : el.querySelector('h2')
  if (heading instanceof HTMLElement) heading.focus({ preventScroll: true })
}

export function PublicView({ data }: Props) {
  const states = stateByKey(data)
  const domains = summarizeDomains(data)
  const overview = domainOverview(data)
  // 押されて根拠が開いている項目。分野をまたいで 1 つだけ
  const [openKey, setOpenKey] = useState<string | null>(null)
  // 1 層目から移ってきた分野。見出しを一瞬だけ朱にして、どこへ来たかを示す
  const [arrived, setArrived] = useState<string | null>(null)
  // 根拠の枠の「閉じる」で、開いていたタイルへフォーカスを戻すための参照（キーボードで読み進めた場所を失わないため）
  const tileRefs = useRef(new Map<string, HTMLButtonElement>())

  useEffect(() => {
    if (!arrived) return
    const t = window.setTimeout(() => setArrived(null), 900)
    return () => window.clearTimeout(t)
  }, [arrived])

  /** 根拠の枠を閉じ、開いていたタイルへフォーカスを戻す */
  const closeDetail = () => {
    const key = openKey
    setOpenKey(null)
    if (key) tileRefs.current.get(key)?.focus()
  }

  /** 1 層目で押した分野のタイルへ移る */
  const jumpToDomain = (key: string) => {
    moveTo(`d-${key}`)
    setArrived(key)
  }

  /** 2 層目の「↑ 要約へ」で 1 層目へ戻る */
  const backToOverview = (e: MouseEvent<HTMLAnchorElement>) => {
    e.preventDefault()
    moveTo('overview-title')
  }

  return (
    <>
      <h1>理解度台帳</h1>
      <p className="thesis">
        学習ログを AI
        が読んで理解度を判定し、ルールで検証したうえで履歴に残しています。自己申告ではレベルが上がりません。
      </p>

      <Overview domains={overview} onJump={jumpToDomain} />

      {data.roadmap.domains.map((domain, i) => {
        const summary = domains[i]
        const openItem = domain.items.find((it) => it.key === openKey)
        const openState = openItem ? states.get(openItem.key) : undefined
        return (
          <section
            className={`domain${arrived === domain.key ? ' arrived' : ''}`}
            key={domain.key}
            id={`d-${domain.key}`}
          >
            <div className="domain-head">
              <h2 tabIndex={-1}>{domain.name}</h2>
              {domain.goal ? <span className="goal">{domain.goal}</span> : null}
              {/* 数は 1 層目の帯とタイルの数が示すので、読み上げにだけ残す（2026-10-24） */}
              <span className="meta">
                <span className="sr-only">
                  {summary.total} 項目中 {summary.withEvidence} に根拠
                </span>
                <a className="up" href="#overview" onClick={backToOverview}>
                  ↑ 要約へ
                </a>
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
                    ref={(el) => {
                      if (el) tileRefs.current.set(it.key, el)
                      else tileRefs.current.delete(it.key)
                    }}
                    type="button"
                    className={`cell${st.verifiedLevel >= 1 ? ` l${st.verifiedLevel}` : ''}`}
                    aria-expanded={expanded}
                    aria-controls={`detail-${domain.key}`}
                    onClick={() => setOpenKey(expanded ? null : it.key)}
                  >
                    {/* 項目名と段だけ。項目キー（内部名）は初めて見る人に意味が無いので出さない（2026-10-12 決定） */}
                    {/* 間の空白は、読み上げの名前を「項目名 段」と区切るため（グリッドの中なので見た目には出ない） */}
                    {/* 段 0 のタイルは、地の色（未着手）と右下の枠だけのコマ（実装済み・基礎未確認。1 層目の帯と同じ形）が状態を示すので、
                        段の名前は読み上げにだけ出す（2026-10-24） */}
                    <span className="name">{shortName(it.name)}</span>{' '}
                    {st.verifiedLevel >= 1 ? (
                      <span className="lv num">{tileLevelLabel(st)}</span>
                    ) : (
                      <span className="lv">
                        {isImplementedButUnverified(st) ? (
                          <i className="u impl" aria-hidden="true" />
                        ) : null}
                        <span className="sr-only">{tileLevelLabel(st)}</span>
                      </span>
                    )}
                  </button>
                )
              })}
            </div>
            {openItem && openState ? (
              <div className="detail" id={`detail-${domain.key}`}>
                <Detail item={openItem} state={openState} onClose={closeDetail} />
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
          {/* 1 層目の帯と 2 層目のタイルの右下に出る、枠だけのコマの読み方。凡例をここ 1 か所に寄せた（2026-10-24） */}
          <div className="step">
            <i className="impl" />
            <div>
              <b className="num">L0</b>
              <span>{IMPLEMENTED_BUT_UNVERIFIED}</span>
            </div>
          </div>
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

      {/* フッターはリポジトリへのリンクだけ（SPEC.md §7.3。個人のリンクは README に限る） */}
      <footer className="foot">
        <span>
          Built with{' '}
          <a href={data.settings.site?.repoUrl ?? DEFAULT_REPO_URL} target="_blank" rel="noopener">
            skill-matrix
          </a>
        </span>
      </footer>
    </>
  )
}

/** タイル右下の文字。レベルか、未着手か、実装の根拠だけがある状態か */
function tileLevelLabel(st: ItemState): string {
  if (st.verifiedLevel >= 1) return `L${st.verifiedLevel}`
  if (isImplementedButUnverified(st)) return IMPLEMENTED_BUT_UNVERIFIED
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
