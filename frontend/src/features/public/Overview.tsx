import { useEffect, useState, type CSSProperties, type MouseEvent } from 'react'
import type { DomainOverview, OverviewUnit } from './summary'
import { RadarChart } from './RadarChart'
import { prefersReducedMotion } from './motion'

// 公開ビューの 1 層目（分野ごとの要約。SPEC.md §7.3）。最初は「一覧で見る」（ステータス帳）で、
// 分野ごとに一番高い段の大きな数字・段の名前・1 コマ＝1 項目の帯・資格を出す。
// 補足のラベルは画面に出さず、並びと形に読ませる（2026-10-24 の試作）：小さな見出し・「m 項目中 x に根拠」は読み上げだけに残し、
// 帯の読み方は公開ビューの末尾の「色の読み方」に寄せた。
// 「図で見る」で能力値チャート（RadarChart）に切り替える。切り替えたときだけ動く（2026-10-12 決定）：
// 図は多角形が中心から広がり、一覧に戻すと帯が左から 1 コマずつ埋まって Lv の数字が 0 から数え上がる。
// 分野を押すと、2 層目のその分野のタイルへ移る（移る処理は親が持つ）

interface Props {
  domains: DomainOverview[]
  /** 分野を押したときに、その分野のタイルへ移る */
  onJump: (domainKey: string) => void
}

type Mode = 'list' | 'chart'

/** 1 コマのクラス。塗りは段、段 0 で実装の印だけの項目は枠だけ */
function unitClass(u: OverviewUnit): string {
  if (u.level >= 1) return `u l${u.level}`
  return u.implOnly ? 'u impl' : 'u'
}

/** 帯の i 番目のコマが埋まるまでの遅れ（ミリ秒）。行 r ごとに少しずらす（試作と同じ） */
function unitDelay(r: number, i: number): number {
  return 60 + r * 40 + i * 35
}

/** 一番高い段の数字。run なら 0 から to まで 1 ずつ数え上げる（行 r ごとに少しずらす） */
function CountUp({ to, r, run }: { to: number; r: number; run: boolean }) {
  const [v, setV] = useState(run ? 0 : to)
  useEffect(() => {
    if (!run) return
    const timers = Array.from({ length: to + 1 }, (_, k) =>
      window.setTimeout(() => setV(k), r * 40 + k * 90),
    )
    return () => timers.forEach((t) => window.clearTimeout(t))
  }, [run, to, r])
  return <>{v}</>
}

export function Overview({ domains, onJump }: Props) {
  const [mode, setMode] = useState<Mode>('list')
  // 切り替えたあとだけ動かす。最初に開いたときは動かさない（ページは止まった状態で完結させる）
  const [switched, setSwitched] = useState(false)
  const animate = switched && !prefersReducedMotion()

  /** 一覧と図を切り替える。同じ側を押したときは何もしない */
  const show = (next: Mode) => {
    if (next === mode) return
    setMode(next)
    setSwitched(true)
  }

  /** リンクの既定の動き（HashRouter の経路が変わる）を止めて、同じページの中で移る */
  const jump = (e: MouseEvent<HTMLAnchorElement>, key: string) => {
    e.preventDefault()
    onJump(key)
  }

  return (
    <section className="overview" id="overview" aria-labelledby="overview-title">
      <div className="ov-head">
        {/* 見出しは読み上げと「↑ 要約へ」の移り先にだけ使う。画面では大きな Lv の数字と帯が何の一覧かを示す */}
        <h2 className="ov-title sr-only" id="overview-title" tabIndex={-1}>
          分野ごとに、確認できた一番高い段
        </h2>
        <div className="ov-mode" role="group" aria-label="表示の切り替え">
          <button type="button" aria-pressed={mode === 'list'} onClick={() => show('list')}>
            一覧で見る
          </button>
          <button type="button" aria-pressed={mode === 'chart'} onClick={() => show('chart')}>
            図で見る
          </button>
        </div>
      </div>

      {mode === 'list' ? (
        <>
          <ul className={`lc${animate ? ' filling' : ''}`}>
            {domains.map((d, r) => (
              <li key={d.key}>
                <a className="lc-row" href={`#d-${d.key}`} onClick={(e) => jump(e, d.key)}>
                  <span className="lc-lv num">
                    <small>Lv</small>
                    <CountUp to={d.top} r={r} run={animate} />
                  </span>
                  <span className="lc-body">
                    <span className="lc-name">{d.name}</span>
                    <span className="lc-tn">{d.topName}</span>
                    {/* 帯は見た目の要約。読み上げには下の「m 項目中 x に根拠」と段の名前で足りる。数は帯のコマが示すので画面には出さない */}
                    <span className="lc-bar" aria-hidden="true">
                      {d.units.map((u, i) => (
                        <i
                          key={i}
                          className={unitClass(u)}
                          style={{ '--delay': `${unitDelay(r, i)}ms` } as CSSProperties}
                        />
                      ))}
                    </span>
                    <span className="sr-only">
                      <span className="num">{d.total}</span> 項目中{' '}
                      <span className="num">{d.withEvidence}</span> に根拠
                    </span>
                    {d.certifications.length > 0 ? (
                      <span className="lc-cert">
                        資格：{d.certifications.join('、')}
                        <span className="nb">（自己申告）</span>
                      </span>
                    ) : null}
                  </span>
                </a>
              </li>
            ))}
          </ul>
        </>
      ) : (
        <>
          <RadarChart domains={domains} onJump={onJump} animate={animate} />
          <p className="legend">
            中心から外へ
            L0〜L5。各分野で確認できた一番高い段を結んだ形です。一覧の「x/m」は根拠のある項目の数。
          </p>
        </>
      )}
    </section>
  )
}
