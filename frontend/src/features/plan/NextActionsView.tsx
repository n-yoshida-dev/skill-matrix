import { Link } from 'react-router'
import type { AppData, Level } from '../../data/types'
import { stateByKey } from '../public/summary'
import {
  STALENESS_LABELS,
  daysSince,
  emptyState,
  missingLevels,
  stalenessConfig,
  type StalenessConfig,
} from './matrix'
import {
  actionKind,
  actionText,
  nextActions,
  nextLimitFrom,
  weightsFrom,
  type Action,
} from './next'

// 作業ビューの「次にやること」Top N（SPEC.md §5.1・§7.4）。
// 1 件ごとに「何をするか」と「身につくと何ができるか（outcome）」を必ず並べて見せる。
// 学習パスの「今ここ」とは別の問い（今いちばん時間を使うべき項目）に答えるので、済んだ項目の深掘りも入る。

interface Props {
  data: AppData
  /** 鮮度を測る基準日。画面では今日、テストでは固定の日付を渡す */
  today: Date
}

export function NextActionsView({ data, today }: Props) {
  const cfg = stalenessConfig(data.settings)
  const states = stateByKey(data)
  const limit = nextLimitFrom(data.settings)
  const actions = nextActions(data.roadmap, states, today, cfg, weightsFrom(data.settings), limit)

  return (
    <section className="next-sec" aria-labelledby="next-title">
      <h2 id="next-title">次にやること</h2>
      <p className="plan-note">
        優先度の高い順に {limit}{' '}
        件。優先度は「前提が済んでいるか・伸びしろ・根拠の古さ・後に続く項目の数」の重み付きの和（重みは
        settings.json の
        weights）。学習パスの「今ここ」とは別の問いに答えるので、済んだ項目の深掘りも入る。
      </p>
      {actions.length === 0 ? (
        <p className="plan-note">やることが残っていない（全項目がレベル 5 で、要再確認も無い）。</p>
      ) : (
        <ol className="next-list">
          {actions.map((a, i) => (
            <NextItem
              key={a.itemKey}
              rank={i + 1}
              a={a}
              missing={missingLevels(states.get(a.itemKey) ?? emptyState(a.itemKey))}
              needsReview={states.get(a.itemKey)?.needsReview ?? false}
              lastEvidenceAt={states.get(a.itemKey)?.lastEvidenceAt ?? null}
              today={today}
              cfg={cfg}
            />
          ))}
        </ol>
      )}
    </section>
  )
}

interface ItemProps {
  rank: number
  a: Action
  missing: Level[]
  needsReview: boolean
  lastEvidenceAt: string | null
  today: Date
  cfg: StalenessConfig
}

/** 「次にやること」の 1 件 */
function NextItem({ rank, a, missing, needsReview, lastEvidenceAt, today, cfg }: ItemProps) {
  const pending = a.pendingLevels.length > 0 && missing.length > 0
  const ago = lastEvidenceAt ? `最終根拠が ${daysSince(today, lastEvidenceAt)} 日前` : ''
  // 橙の線はマトリクスの枠線と同じ「要再確認」だけに使う。古くなりつつある段階は灰色の文字で添える
  const reasons: string[] = []
  if (needsReview) reasons.push('不合格の報告あり')
  if (a.staleness === 'stale') reasons.push(`${ago}（${STALENESS_LABELS.stale}）`)
  const aging =
    a.staleness === 'aging'
      ? `${ago}（${STALENESS_LABELS.aging}。${cfg.agingWithinDays} 日を超えると要再確認）`
      : null
  return (
    <li className="next-item">
      <div className="next-head">
        <span className="rank num" aria-label={`${rank} 位`}>
          {rank}
        </span>
        <span className="dom">{a.domainName}</span>
        <span className="k num">{a.itemKey}</span>
        <span className="kind">{actionKind(a)}</span>
      </div>
      <div className="next-name">
        <span className={`lv${a.level >= 1 ? ` l${a.level}` : ''}`} aria-hidden="true" />
        <Link to={`/plan/item/${a.itemKey}`}>{a.name}</Link>
        <span className="num lvtxt">L{a.level}</span>
      </div>
      <p className="next-do">{actionText(a, missing)}</p>
      {pending && a.verifyBy ? <p className="next-sub">確認方法：{a.verifyBy}</p> : null}
      <p className="next-sub">
        身につくと：{a.outcome ? a.outcome : <span className="unset">未記入</span>}
      </p>
      {aging ? <p className="next-sub">{aging}</p> : null}
      {reasons.length > 0 ? <p className="next-warn">要再確認：{reasons.join('／')}</p> : null}
    </li>
  )
}
