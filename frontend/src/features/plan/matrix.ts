import type { ItemState, Level, PreState, Roadmap, RoadmapDomain, Settings } from '../../data/types'
import { topEvidenced } from '../public/summary'

// 作業ビューのマトリクス（SPEC.md §7.1）のための純粋関数。DOM・ファイル読み込みを import しない。
// 「今日」に依存する計算（鮮度・要再確認）はここに置き、今日は引数で受け取る（SPEC.md §5）。
// Go 側の参照実装は backend/internal/domain/summary.go（Staleness / NeedsAttention / RollupDomain）。
// 同じ入力に同じ答えを返すことを matrix.test.ts が Go のテスト例を写して確かめる。

/** レベルの上限 */
export const MAX_LEVEL = 5

/** 根拠の古さ。レベルを減衰させる代わりの指標（SPEC.md §1.3） */
export type Staleness = 'unknown' | 'fresh' | 'aging' | 'stale'

/** 鮮度の境界（日数） */
export interface StalenessConfig {
  freshWithinDays: number
  agingWithinDays: number
}

/** 既定の境界。Go の DefaultStalenessConfig() と同じ 30 日 / 90 日 */
export const DEFAULT_STALENESS: StalenessConfig = { freshWithinDays: 30, agingWithinDays: 90 }

/** 鮮度の日本語 */
export const STALENESS_LABELS: Record<Staleness, string> = {
  unknown: '根拠なし',
  fresh: '新しい',
  aging: '古くなりつつある',
  stale: '古い',
}

/** 段階前の状態の日本語（SPEC.md §1.2 の「意味」の列） */
export const PRE_STATE_LABELS: Record<PreState, string> = {
  none: '',
  learning: '未確認（学習中）',
  explained_only: '説明済み・理解未確認',
  self_reported: '実務経験あり・横断評価未実施',
}

/**
 * settings.json の鮮度の境界を読み、無い値・0 は既定値で埋める。
 * Go の StalenessConfig.withDefaults() と同じく、0 を「未設定」として扱う
 */
export function stalenessConfig(settings: Settings): StalenessConfig {
  const s = settings.staleness
  return {
    freshWithinDays: s?.freshWithinDays || DEFAULT_STALENESS.freshWithinDays,
    agingWithinDays: s?.agingWithinDays || DEFAULT_STALENESS.agingWithinDays,
  }
}

/** 日付を手元の暦の YYYY-MM-DD にする */
export function toDateKey(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

/** YYYY-MM-DD を暦日の通し番号（1970-01-01 からの日数）にする。形が違えば例外 */
function dayNumber(ymd: string): number {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(ymd)
  if (!m) {
    throw new Error(`日付の形が YYYY-MM-DD ではない: ${ymd}`)
  }
  return Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3])) / 86_400_000
}

/**
 * 今日から見て、その日付が何日前か（暦日で数える）。
 * 時刻の差ではなく暦日の差にするのは、同じ日のうちに開き直しても結果が変わらないようにするため
 */
export function daysSince(today: Date, ymd: string): number {
  return dayNumber(toDateKey(today)) - dayNumber(ymd)
}

/** 最終根拠日からの経過で鮮度を返す。根拠がまだ無ければ unknown（Go の Staleness） */
export function staleness(
  today: Date,
  lastEvidenceAt: string | null,
  cfg: StalenessConfig,
): Staleness {
  if (lastEvidenceAt === null) return 'unknown'
  const elapsed = daysSince(today, lastEvidenceAt)
  if (elapsed <= cfg.freshWithinDays) return 'fresh'
  if (elapsed <= cfg.agingWithinDays) return 'aging'
  return 'stale'
}

/** 要再確認か。不合格の報告があった場合（V6）と、根拠が古い場合の両方を拾う（Go の NeedsAttention） */
export function needsAttention(st: ItemState, today: Date, cfg: StalenessConfig): boolean {
  if (st.needsReview) return true
  return staleness(today, st.lastEvidenceAt, cfg) === 'stale'
}

/** 未確認の段。1 から最上段の印までのうち、印が無い段（SPEC.md §7.4）。例：[3] なら [1, 2] */
export function missingLevels(st: ItemState): Level[] {
  const marks = new Set<number>(st.evidencedLevels)
  const out: Level[] = []
  for (let l = 1; l < topEvidenced(st); l++) {
    if (!marks.has(l)) out.push(l as Level)
  }
  return out
}

/**
 * 升目の吹き出しの 1 行目を 3 つの区切りで返す。例：["Verified 0", "根拠の印 3", "未確認 1, 2"]。
 * 画面は区切りの間でだけ折り返す（区切りの途中で「1,」と「2」が泣き別れにならないように）
 */
export function levelParts(st: ItemState): [string, string, string] {
  const marks = st.evidencedLevels.length > 0 ? st.evidencedLevels.join(', ') : 'なし'
  const missing = missingLevels(st)
  const unverified = missing.length > 0 ? missing.join(', ') : 'なし'
  return [`Verified ${st.verifiedLevel}`, `根拠の印 ${marks}`, `未確認 ${unverified}`]
}

/** 升目の吹き出しの 1 行目。例：「Verified 0 / 根拠の印 3 / 未確認 1, 2」（SPEC.md §7.1） */
export function levelLine(st: ItemState): string {
  return levelParts(st).join(' / ')
}

/** state.json に項目が無いときの状態（未着手・根拠なし）。Go の RollupDomain と同じ扱い */
export function emptyState(itemKey: string): ItemState {
  return {
    itemKey,
    verifiedLevel: 0,
    evidencedLevels: [],
    preState: 'none',
    needsReview: false,
    lastEvidenceAt: null,
    events: [],
  }
}

/** レベル 0〜5 の名前。1〜5 はロードマップの levels（判定基準の正本）から取る */
export function levelNames(roadmap: Roadmap): Record<Level, string> {
  const byLevel = new Map(roadmap.levels.map((l) => [l.level, l.name]))
  const name = (l: 1 | 2 | 3 | 4 | 5) => byLevel.get(l) ?? `レベル ${l}`
  return { 0: '未着手', 1: name(1), 2: name(2), 3: name(3), 4: name(4), 5: name(5) }
}

/** 分野 1 つの集計。サマリー帯の 1 行（Go の DomainSummary） */
export interface DomainRollup {
  key: string
  name: string
  total: number
  /** verifiedLevel ごとの項目数。添字がレベル（0〜5） */
  byLevel: number[]
  /** 進捗率（0〜1）。達成レベルの合計 ÷（項目数 × 5） */
  progress: number
  /** 要再確認の項目数 */
  staleCount: number
  /** 実装根拠あり・理解未確認（印が verifiedLevel より上にある）の項目数 */
  pendingCount: number
}

/** 保存値が範囲外でも配列の外を指さないよう、レベルを 0〜5 に丸める */
function clampLevel(n: number): Level {
  return Math.min(Math.max(Math.trunc(n), 0), MAX_LEVEL) as Level
}

/**
 * 分野 1 つを集計する（Go の RollupDomain）。
 * states に無い項目は未着手（レベル 0・根拠なし）として数える
 */
export function rollupDomain(
  domain: RoadmapDomain,
  states: Map<string, ItemState>,
  today: Date,
  cfg: StalenessConfig,
): DomainRollup {
  const sum: DomainRollup = {
    key: domain.key,
    name: domain.name,
    total: domain.items.length,
    byLevel: [0, 0, 0, 0, 0, 0],
    progress: 0,
    staleCount: 0,
    pendingCount: 0,
  }
  let levelTotal = 0
  for (const it of domain.items) {
    const st = states.get(it.key) ?? emptyState(it.key)
    const lv = clampLevel(st.verifiedLevel)
    sum.byLevel[lv]++
    levelTotal += lv
    if (needsAttention(st, today, cfg)) sum.staleCount++
    if (topEvidenced(st) > st.verifiedLevel) sum.pendingCount++
  }
  if (sum.total > 0) {
    sum.progress = levelTotal / (sum.total * MAX_LEVEL)
  }
  return sum
}

/** ロードマップ全体の分野を、定義された並び順で集計する（Go の RollupAll） */
export function rollupAll(
  roadmap: Roadmap,
  states: Map<string, ItemState>,
  today: Date,
  cfg: StalenessConfig,
): DomainRollup[] {
  return roadmap.domains.map((d) => rollupDomain(d, states, today, cfg))
}

/** サマリー帯の合計行。進捗率は分野の平均ではなく、全項目で割り直す */
export function totalRollup(rollups: DomainRollup[]): DomainRollup {
  const sum: DomainRollup = {
    key: '',
    name: '合計',
    total: 0,
    byLevel: [0, 0, 0, 0, 0, 0],
    progress: 0,
    staleCount: 0,
    pendingCount: 0,
  }
  let levelTotal = 0
  for (const r of rollups) {
    sum.total += r.total
    sum.staleCount += r.staleCount
    sum.pendingCount += r.pendingCount
    r.byLevel.forEach((n, lv) => {
      sum.byLevel[lv] += n
      levelTotal += n * lv
    })
  }
  if (sum.total > 0) {
    sum.progress = levelTotal / (sum.total * MAX_LEVEL)
  }
  return sum
}
