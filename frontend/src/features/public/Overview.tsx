import type { MouseEvent } from 'react'
import type { DomainOverview, OverviewUnit } from './summary'

// 公開ビューの 1 層目（分野ごとの要約。SPEC.md §7.3）。ステータス帳の形で、分野ごとに
// 一番高い段の大きな数字・段の名前・1 コマ＝1 項目の帯・「m 項目中 x に根拠」・資格を出す。
// 分野を押すと、2 層目のその分野のタイルへ移る（移る処理は親が持つ）

interface Props {
  domains: DomainOverview[]
  /** 分野を押したときに、その分野のタイルへ移る */
  onJump: (domainKey: string) => void
}

/** 1 コマのクラス。塗りは段、段 0 で実装の印だけの項目は枠だけ */
function unitClass(u: OverviewUnit): string {
  if (u.level >= 1) return `u l${u.level}`
  return u.implOnly ? 'u impl' : 'u'
}

export function Overview({ domains, onJump }: Props) {
  /** リンクの既定の動き（HashRouter の経路が変わる）を止めて、同じページの中で移る */
  const jump = (e: MouseEvent<HTMLAnchorElement>, key: string) => {
    e.preventDefault()
    onJump(key)
  }

  return (
    <section className="overview" id="overview" aria-labelledby="overview-title">
      <h2 className="ov-title" id="overview-title" tabIndex={-1}>
        分野ごとに、確認できた一番高い段
      </h2>
      <ul className="lc">
        {domains.map((d) => (
          <li key={d.key}>
            <a className="lc-row" href={`#d-${d.key}`} onClick={(e) => jump(e, d.key)}>
              <span className="lc-lv num">
                <small>Lv</small>
                {d.top}
              </span>
              <span className="lc-body">
                <span className="lc-name">{d.name}</span>
                <span className="lc-tn">{d.topName}</span>
                {/* 帯は見た目の要約。読み上げには下の「m 項目中 x に根拠」と段の名前で足りる */}
                <span className="lc-bar" aria-hidden="true">
                  {d.units.map((u, i) => (
                    <i key={i} className={unitClass(u)} />
                  ))}
                </span>
                <span className="lc-c">
                  <span className="num">{d.total}</span> 項目中{' '}
                  <span className="num">{d.withEvidence}</span> に根拠
                </span>
                {d.certifications.length > 0 ? (
                  <span className="lc-cert">資格：{d.certifications.join('、')}</span>
                ) : null}
              </span>
            </a>
          </li>
        ))}
      </ul>
      <p className="legend">
        Lv は分野で確認できた一番高い段。帯は 1 コマ＝1 項目で、塗りは確認できた段（
        <i className="u impl" aria-hidden="true" />
        は実装の根拠だけがあり、基礎の確認がまだ）。資格は自己申告で、段には数えていません。
      </p>
      {/* 凡例の末尾に足すと、スマホで最後の行が「ます。」だけになった（2026-10-14 の点検）。短い 1 文は別の行にする */}
      <p className="legend">分野を押すと、その分野の項目へ移ります。</p>
    </section>
  )
}
