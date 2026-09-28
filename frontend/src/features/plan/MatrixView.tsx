import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router'
import type { AppData, ItemState, Level, RoadmapItem } from '../../data/types'
import { stateByKey, topEvidenced } from '../public/summary'
import {
  PRE_STATE_LABELS,
  STALENESS_LABELS,
  daysSince,
  emptyState,
  levelNames,
  levelParts,
  needsAttention,
  rollupAll,
  staleness,
  stalenessConfig,
  totalRollup,
  type DomainRollup,
  type StalenessConfig,
} from './matrix'
import './plan.css'

// 作業ビューのマトリクス（SPEC.md §7.1）。自分向け。
// 上にサマリー帯（分野 × verifiedLevel の項目数と「実装根拠あり・理解未確認」の列）、
// 下に可変長グリッド（行 = 分野、升目 = 項目）を置く。
// 升目は 3 つの情報を重ねる：塗り = verifiedLevel、枠線 = 要再確認、右上の三角 = 上の段にも根拠あり。

const LEVELS: Level[] = [0, 1, 2, 3, 4, 5]

/** 吹き出しの幅の上限（px） */
const TIP_WIDTH = 300

interface Props {
  data: AppData
  /** 鮮度を測る基準日。画面では今日、テストでは固定の日付を渡す */
  today: Date
}

/** 吹き出しの位置（マトリクスの枠の左上から）と、押して固定したか */
interface Tip {
  key: string
  left: number
  top: number
  width: number
  pinned: boolean
}

export function MatrixView({ data, today }: Props) {
  const cfg = stalenessConfig(data.settings)
  const states = stateByKey(data)
  const rollups = rollupAll(data.roadmap, states, today, cfg)
  const names = levelNames(data.roadmap)
  const gridRef = useRef<HTMLDivElement>(null)
  const [tip, setTip] = useState<Tip | null>(null)

  // 押して固定した吹き出しは、マトリクスの外を押すと閉じる（タップ操作で閉じる手段を残すため）
  const pinned = tip?.pinned ?? false
  useEffect(() => {
    if (!pinned) return
    const onDown = (e: PointerEvent) => {
      if (!gridRef.current?.contains(e.target as Node)) setTip(null)
    }
    document.addEventListener('pointerdown', onDown)
    return () => document.removeEventListener('pointerdown', onDown)
  }, [pinned])

  /** 升目の真下に吹き出しを出す。マトリクスの幅からはみ出さないよう左右を詰める */
  function place(key: string, el: HTMLElement, pin: boolean) {
    const grid = gridRef.current
    if (!grid) return
    const g = grid.getBoundingClientRect()
    const c = el.getBoundingClientRect()
    const width = g.width > 0 ? Math.min(TIP_WIDTH, g.width) : TIP_WIDTH
    const center = c.left - g.left + c.width / 2
    const left = Math.max(0, Math.min(center - width / 2, g.width - width))
    setTip({ key, left, top: c.bottom - g.top + 6, width, pinned: pin })
  }

  const tipItem = tip ? findItem(data, tip.key) : undefined

  return (
    <>
      <section className="band-sec" aria-labelledby="band-title">
        <h2 id="band-title">サマリー</h2>
        <p className="plan-note">分野ごとに、Verified Level ごとの項目数を数えている。</p>
        <div className="band-scroll">
          <table className="band">
            <thead>
              <tr>
                <th scope="col" className="dom">
                  分野
                </th>
                <th scope="col">項目</th>
                {LEVELS.map((l) => (
                  <th scope="col" key={l} title={names[l]}>
                    <i className={`sw${l >= 1 ? ` l${l}` : ''}`} aria-hidden="true" />
                    <span className="num">L{l}</span>
                  </th>
                ))}
                <th scope="col" className="wide">
                  実装根拠あり・理解未確認
                </th>
                <th scope="col">要再確認</th>
                <th scope="col">進捗</th>
              </tr>
            </thead>
            <tbody>
              {rollups.map((r) => (
                <BandRow key={r.key} r={r} />
              ))}
            </tbody>
            <tfoot>
              <BandRow r={totalRollup(rollups)} />
            </tfoot>
          </table>
        </div>
      </section>

      <section className="mx-sec" aria-labelledby="mx-title">
        <h2 id="mx-title">マトリクス</h2>
        <p className="plan-note">
          升目にカーソルを乗せる・選ぶと詳細が出る。押すと固定し、もう一度押すか Esc で閉じる。
        </p>
        <div
          className="mx-grid"
          ref={gridRef}
          onKeyDown={(e) => {
            if (e.key === 'Escape') setTip(null)
          }}
        >
          {data.roadmap.domains.map((d) => (
            <div className="mx-row" key={d.key}>
              <div className="mx-label">
                <span className="nm">{d.name}</span>
                <span className="cnt num">{d.items.length} 項目</span>
              </div>
              <div className="mx-cells">
                {d.items.map((it) => {
                  const st = states.get(it.key) ?? emptyState(it.key)
                  const active = tip?.key === it.key
                  return (
                    <button
                      key={it.key}
                      type="button"
                      className={cellClass(st, today, cfg, active)}
                      aria-label={`${it.key} ${it.name}`}
                      aria-describedby={active ? 'mx-tip' : undefined}
                      onPointerEnter={(e) => {
                        if (!pinned) place(it.key, e.currentTarget, false)
                      }}
                      onPointerLeave={() => {
                        if (!pinned) setTip(null)
                      }}
                      onFocus={(e) => {
                        // キーボードで別の升目へ移ったら、固定していても吹き出しを付け替える
                        if (!pinned || !active) place(it.key, e.currentTarget, false)
                      }}
                      onBlur={() => {
                        if (!pinned) setTip(null)
                      }}
                      onClick={(e) => {
                        if (pinned && active) setTip(null)
                        else place(it.key, e.currentTarget, true)
                      }}
                    />
                  )
                })}
              </div>
            </div>
          ))}
          {tip && tipItem ? (
            <div
              id="mx-tip"
              role="tooltip"
              className={tip.pinned ? 'mx-tip pinned' : 'mx-tip'}
              style={{ left: tip.left, top: tip.top, width: tip.width }}
            >
              <TipBody
                item={tipItem}
                st={states.get(tipItem.key) ?? emptyState(tipItem.key)}
                today={today}
                cfg={cfg}
                levelName={names}
              />
              {/* 押して固定したときだけ、吹き出しの中を押せるようにして項目詳細へのリンクを出す */}
              {tip.pinned ? (
                <Link className="more" to={`/plan/item/${tipItem.key}`}>
                  項目詳細を開く
                </Link>
              ) : null}
            </div>
          ) : null}
        </div>

        <div className="mx-legend" aria-label="凡例">
          {LEVELS.map((l) => (
            <span className="lg" key={l}>
              <i className={`mx-cell${l >= 1 ? ` l${l}` : ''}`} aria-hidden="true" />
              <span>
                <span className="num">L{l}</span> {names[l]}
              </span>
            </span>
          ))}
          <span className="lg">
            <i className="mx-cell l1 attn" aria-hidden="true" />
            <span>
              枠線：要再確認（不合格の報告あり、または最終根拠が {cfg.agingWithinDays} 日より前）
            </span>
          </span>
          <span className="lg">
            <i className="mx-cell up up-l3" aria-hidden="true" />
            <span>右上の三角：上の段にも根拠あり（三角の色がその段）</span>
          </span>
        </div>
      </section>
    </>
  )
}

/** 升目のクラス。塗り（l1〜l5）・要再確認（attn）・上位の根拠（up up-lN）・吹き出し中（on） */
function cellClass(st: ItemState, today: Date, cfg: StalenessConfig, active: boolean): string {
  const cls = ['mx-cell']
  if (st.verifiedLevel >= 1) cls.push(`l${st.verifiedLevel}`)
  if (needsAttention(st, today, cfg)) cls.push('attn')
  const top = topEvidenced(st)
  if (top > st.verifiedLevel) cls.push('up', `up-l${top}`)
  if (active) cls.push('on')
  return cls.join(' ')
}

/** 項目キーからロードマップの項目を探す */
function findItem(data: AppData, key: string): RoadmapItem | undefined {
  for (const d of data.roadmap.domains) {
    const it = d.items.find((x) => x.key === key)
    if (it) return it
  }
  return undefined
}

/** サマリー帯の 1 行。0 件の欄は薄くして、数のある欄を目立たせる */
function BandRow({ r }: { r: DomainRollup }) {
  const z = (n: number) => (n === 0 ? 'z' : undefined)
  return (
    <tr>
      <th scope="row" className="dom">
        {r.name}
      </th>
      <td className="num">{r.total}</td>
      {r.byLevel.map((n, l) => (
        <td className={['num', z(n)].filter(Boolean).join(' ')} key={l}>
          {n}
        </td>
      ))}
      <td className={['num', z(r.pendingCount)].filter(Boolean).join(' ')}>{r.pendingCount}</td>
      <td className={['num', z(r.staleCount)].filter(Boolean).join(' ')}>{r.staleCount}</td>
      <td className="num">{Math.round(r.progress * 100)}%</td>
    </tr>
  )
}

interface TipProps {
  item: RoadmapItem
  st: ItemState
  today: Date
  cfg: StalenessConfig
  levelName: Record<Level, string>
}

/** 吹き出しの中身。レベルと印、段階前の状態、鮮度、要再確認の理由 */
function TipBody({ item, st, today, cfg, levelName }: TipProps) {
  const fresh = staleness(today, st.lastEvidenceAt, cfg)
  const reasons: string[] = []
  if (st.needsReview) reasons.push('不合格の報告あり')
  if (fresh === 'stale') reasons.push(`最終根拠が ${cfg.agingWithinDays} 日より前`)
  return (
    <>
      <div className="k num">{item.key}</div>
      <div className="nm">{item.name}</div>
      <div className="ln">
        {levelParts(st).map((part, i) => (
          <span key={i}>
            {i > 0 ? ' / ' : null}
            <span className="part">{part}</span>
          </span>
        ))}
      </div>
      <div className="sub">
        L{st.verifiedLevel} {levelName[st.verifiedLevel]}
      </div>
      {st.preState !== 'none' ? (
        <div className="sub">段階前の状態：{PRE_STATE_LABELS[st.preState]}</div>
      ) : null}
      <div className="sub">
        最終根拠：
        {st.lastEvidenceAt ? (
          <>
            <span className="num">{st.lastEvidenceAt}</span>（{daysSince(today, st.lastEvidenceAt)}{' '}
            日前・{STALENESS_LABELS[fresh]}）
          </>
        ) : (
          'なし'
        )}
      </div>
      {reasons.length > 0 ? <div className="warn">要再確認：{reasons.join('／')}</div> : null}
    </>
  )
}
