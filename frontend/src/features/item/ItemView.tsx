import { Link, useParams } from 'react-router'
import type {
  AppData,
  ItemState,
  PendingJudgment,
  RoadmapDomain,
  RoadmapItem,
  StateEvent,
} from '../../data/types'
import { EVIDENCE_LABELS } from '../public/summary'
import {
  PRE_STATE_LABELS,
  STALENESS_LABELS,
  daysSince,
  levelNames,
  missingLevels,
  staleness,
  stalenessConfig,
} from '../plan/matrix'
import { pendingCheckText, pendingLevels, stateOf } from '../plan/next'
import './item.css'

// 項目詳細（SPEC.md §7・§7.4）。見出しに 3 行（Verified Level／付いている印／未確認の段）、
// 到達状態と次の確認方法、印の履歴（いつ・どの根拠が・どの印を付けたか）、保留・棄却を出す。
// **この画面から編集はしない**（2026-09-23 決定。更新は data/judgments/ にファイルを足して行う）

interface Props {
  data: AppData
  /** 鮮度を測る基準日。画面では今日、テストでは固定の日付を渡す */
  today: Date
}

/** 判定を書いた主体の日本語 */
const SOURCE_LABELS: Record<StateEvent['source'], string> = {
  ai: 'AI の判定',
  manual: '手動の判定',
  migration: '移行',
}

export function ItemView({ data, today }: Props) {
  const { itemKey } = useParams()
  const found = findItem(data.roadmap.domains, itemKey ?? '')
  if (!found) {
    return (
      <section className="item-sec">
        <p>
          項目 <span className="num">{itemKey}</span> はロードマップに無い。
        </p>
        <p>
          <Link to="/plan">作業ビューへ戻る</Link>
        </p>
      </section>
    )
  }

  const { domain, item } = found
  const states = new Map(data.state.items.map((s) => [s.itemKey, s]))
  const st = stateOf(states, item.key)
  const cfg = stalenessConfig(data.settings)
  const names = levelNames(data.roadmap)
  const fresh = staleness(today, st.lastEvidenceAt, cfg)
  const missing = missingLevels(st)
  const pending = pendingLevels(st).length > 0 && missing.length > 0
  const allItems = new Map(data.roadmap.domains.flatMap((d) => d.items.map((it) => [it.key, it])))
  const dependents = data.roadmap.domains.flatMap((d) =>
    d.items.filter((it) => (it.dependsOn ?? []).includes(item.key)),
  )
  // 履歴は起きた日の新しい順。state.json の events は適用順（判定ファイル名の順）なので、
  // 起きた日で並べ直し、同じ日なら適用の遅いものを上に置く
  const events = st.events
    .map((e, i) => ({ e, i }))
    .sort((a, b) =>
      a.e.occurredAt === b.e.occurredAt ? b.i - a.i : b.e.occurredAt.localeCompare(a.e.occurredAt),
    )
    .map(({ e }) => e)
  const deferred = data.state.deferred.filter((p) => p.itemKey === item.key)
  const rejected = data.state.rejected.filter((p) => p.itemKey === item.key)

  const reasons: string[] = []
  if (st.needsReview) reasons.push('不合格の報告あり')
  if (fresh === 'stale') reasons.push(`最終根拠が ${cfg.agingWithinDays} 日より前`)

  return (
    <section className="item-sec" aria-labelledby="item-title">
      <p className="item-crumb">
        <Link to="/plan">作業ビュー</Link> ／{' '}
        <Link to={`/plan/path/${domain.key}`}>{domain.name}の学習パス</Link>
      </p>
      <p className="item-key num">{item.key}</p>
      <h2 id="item-title">{item.name}</h2>
      {item.description ? <p className="item-desc">{item.description}</p> : null}

      <dl className="item-levels">
        <div>
          <dt>Verified Level</dt>
          <dd>
            <span className="num">{st.verifiedLevel}</span>（{names[st.verifiedLevel]}）
          </dd>
        </div>
        <div>
          <dt>付いている印</dt>
          <dd className="num">
            {st.evidencedLevels.length > 0 ? st.evidencedLevels.join(', ') : 'なし'}
          </dd>
        </div>
        <div>
          <dt>未確認の段</dt>
          <dd className="num">{missing.length > 0 ? missing.join(', ') : 'なし'}</dd>
        </div>
      </dl>

      <ul className="item-facts">
        {st.preState !== 'none' ? <li>段階前の状態：{PRE_STATE_LABELS[st.preState]}</li> : null}
        <li>
          最終根拠：
          {st.lastEvidenceAt ? (
            <>
              <span className="num">{st.lastEvidenceAt}</span>（
              {daysSince(today, st.lastEvidenceAt)} 日前・
              {STALENESS_LABELS[fresh]}）
            </>
          ) : (
            'なし'
          )}
        </li>
        {reasons.length > 0 ? <li className="warn">要再確認：{reasons.join('／')}</li> : null}
      </ul>

      <h3>到達状態と次の確認方法</h3>
      <p>身につくと：{item.outcome ? item.outcome : <span className="unset">未記入</span>}</p>
      {pending ? <p>次にやること：{pendingCheckText(missing)}</p> : null}
      <p>
        {pending ? '確認方法' : '次の確認方法'}：
        {item.verifyBy ? item.verifyBy : <span className="unset">未記入</span>}
      </p>

      <h3>前提と、この項目を前提にする項目</h3>
      <ItemLinks label="前提" keys={item.dependsOn ?? []} items={allItems} states={states} />
      <ItemLinks
        label="この項目を前提にする項目"
        keys={dependents.map((it) => it.key)}
        items={allItems}
        states={states}
      />

      <h3>
        履歴（<span className="num">{events.length}</span> 件。新しい順）
      </h3>
      {events.length === 0 ? (
        <p className="plan-note">まだ判定が無い。</p>
      ) : (
        <ol className="item-events">
          {events.map((e) => (
            <EventRow key={`${e.file}#${e.index}`} e={e} />
          ))}
        </ol>
      )}

      <h3>
        保留（<span className="num">{deferred.length}</span> 件）
      </h3>
      <p className="plan-note">
        確信度が低いなどで適用していない判定。採用するなら <code>source: &quot;manual&quot;</code>{' '}
        の判定ファイルを足す（SPEC.md §4.7）。
      </p>
      <PendingList list={deferred} />
      {rejected.length > 0 ? (
        <>
          <h3>
            棄却（<span className="num">{rejected.length}</span> 件）
          </h3>
          <PendingList list={rejected} />
        </>
      ) : null}
    </section>
  )
}

/** 項目キーから分野と項目を探す */
function findItem(
  domains: RoadmapDomain[],
  key: string,
): { domain: RoadmapDomain; item: RoadmapItem } | undefined {
  for (const domain of domains) {
    const item = domain.items.find((it) => it.key === key)
    if (item) return { domain, item }
  }
  return undefined
}

interface LinksProps {
  label: string
  keys: string[]
  items: Map<string, RoadmapItem>
  states: Map<string, ItemState>
}

/** 項目へのリンクの並び。レベルを添える */
function ItemLinks({ label, keys, items, states }: LinksProps) {
  return (
    <p>
      {label}：
      {keys.length === 0
        ? 'なし'
        : keys.map((k, i) => (
            <span key={k}>
              {i > 0 ? '、' : null}
              <Link to={`/plan/item/${k}`}>{items.get(k)?.name ?? k}</Link>（レベル{' '}
              {stateOf(states, k).verifiedLevel}）
            </span>
          ))}
    </p>
  )
}

/** 履歴の 1 件。いつ・どの根拠が・どの印を付けたか・なぜか・どこを根拠にしたか */
function EventRow({ e }: { e: StateEvent }) {
  return (
    <li className="item-event">
      <div className="ev-head">
        <span className="num">{e.occurredAt}</span>
        <span>{EVIDENCE_LABELS[e.evidenceType]}</span>
        <span className="marked">
          {e.marked.length > 0 ? (
            <>
              印 <span className="num">{e.marked.join(', ')}</span> を付けた
            </>
          ) : (
            '印なし'
          )}
        </span>
        <span className="src">
          {SOURCE_LABELS[e.source]}
          {e.confidence !== null ? `・確信度 ${e.confidence}` : ''}
        </span>
      </div>
      <p className="ev-why">{e.rationale}</p>
      {e.violations.map((v, i) => (
        <p className="ev-violation" key={i}>
          <span className="num">{v.code}</span>：{v.detail}
        </p>
      ))}
      <ul className="ev-refs">
        {e.evidenceRefs.map((r) => (
          <li key={r} className="num">
            {r}
          </li>
        ))}
      </ul>
      <p className="ev-file num">
        {e.file} [{e.index}]
      </p>
    </li>
  )
}

/** 保留・棄却の一覧 */
function PendingList({ list }: { list: PendingJudgment[] }) {
  if (list.length === 0) return <p>なし</p>
  return (
    <ul className="item-pending">
      {list.map((p) => (
        <li key={`${p.file}#${p.index}`}>
          <span className="num">
            {p.file} [{p.index}]
          </span>{' '}
          <span className="num">{p.code}</span>：{p.detail}
        </li>
      ))}
    </ul>
  )
}
