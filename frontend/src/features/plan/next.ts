import type { ItemState, Level, Roadmap, RoadmapItem, Settings } from '../../data/types'
import { MAX_LEVEL, emptyState, needsAttention, staleness } from './matrix'
import type { Staleness, StalenessConfig } from './matrix'

// 「次にやること」Top N（SPEC.md §5.1・§7.4）のための純粋関数。DOM・ファイル読み込みを import しない。
// 鮮度の加点が「今日」に依存するので TypeScript 側に置く（SPEC.md §5）。
// Go の参照実装は backend/internal/domain/plan.go の NextActions。
// 同じ入力に同じ答えを返すことを next.test.ts が Go の plan_test.go の例を写して確かめる。

/** 優先度の重み（SPEC.md §5.1）。settings.json の weights */
export interface Weights {
  /** 着手できるか（依存項目がすべてレベル 1 以上か） */
  readiness: number
  /** 伸びしろ（上限との差） */
  gap: number
  /** 鮮度の悪さ（再確認を促す） */
  staleness: number
  /** これを終えると何項目の前提が満たされるか */
  unlocks: number
}

/** 既定の重み。Go の DefaultWeights() と同じ（実データで調整する前の仮置き。TODO 2-6） */
export const DEFAULT_WEIGHTS: Weights = { readiness: 1.0, gap: 0.8, staleness: 0.3, unlocks: 0.5 }

/** 「次にやること」の既定の件数。Go の defaultNextActionsLimit と同じ */
export const DEFAULT_NEXT_LIMIT = 5

/**
 * settings.json の重みを読む。書かれていない重みだけ既定値で埋める。
 * Go の CLI（cmd/skillmatrix/settings.go）と同じく、0 と書かれていれば 0 として使う
 */
export function weightsFrom(settings: Settings): Weights {
  const w = settings.weights
  return {
    readiness: w?.readiness ?? DEFAULT_WEIGHTS.readiness,
    gap: w?.gap ?? DEFAULT_WEIGHTS.gap,
    staleness: w?.staleness ?? DEFAULT_WEIGHTS.staleness,
    unlocks: w?.unlocks ?? DEFAULT_WEIGHTS.unlocks,
  }
}

/** settings.json の件数を読む。無ければ既定の 5 件 */
export function nextLimitFrom(settings: Settings): number {
  return settings.nextActions?.limit ?? DEFAULT_NEXT_LIMIT
}

/** 「次にやること」1 件。到達状態（outcome）と次の確認方法（verifyBy）を必ず一緒に載せる（SPEC.md §5.1） */
export interface Action {
  itemKey: string
  domainKey: string
  domainName: string
  name: string
  outcome?: string
  verifyBy?: string
  /** 表示レベル（verifiedLevel） */
  level: Level
  staleness: Staleness
  priority: number
  /** 前提が済んでいないので、まだ着手できない */
  blocked: boolean
  /** 印はあるが表示レベルに届いていない段。例：印 [3] でレベル 0 なら [3]（SPEC.md §7.4） */
  pendingLevels: Level[]
}

/**
 * 何をする項目かの短い分類。学習パスの「今ここ」と違うことが分かるよう、深掘りは深掘りと書く（SPEC.md §7.2）。
 * 例：印 [3]・レベル 0 は「基礎の確認」、レベル 1 は「深掘り（L1 → L2）」
 */
export function actionKind(a: Action): string {
  if (a.blocked) return '前提が未達'
  if (a.pendingLevels.length > 0) return '基礎の確認'
  if (a.level === 0) return '着手（L0 → L1）'
  if (a.level >= MAX_LEVEL) return '再確認'
  return `深掘り（L${a.level} → L${a.level + 1}）`
}

/**
 * 「何をするか」の 1 行。印が表示レベルより上にある項目は
 * 「既存の実装について L1 / L2 を…確認する」にして、基礎からやり直しと見せない（SPEC.md §7.4）。
 * missing は未確認の段（matrix.ts の missingLevels）
 */
export function actionText(a: Action, missing: Level[]): string {
  if (a.pendingLevels.length > 0 && missing.length > 0) {
    const levels = missing.map((l) => `L${l}`).join(' / ')
    return `既存の実装について ${levels} を短いドリル・自己説明で確認する`
  }
  return a.verifyBy ?? '次の確認方法が未記入（roadmap.json の verifyBy）'
}

/** 項目の状態。state.json に無ければ未着手。範囲外のレベルは 0〜5 に丸める（Go の stateOf） */
export function stateOf(states: Map<string, ItemState>, key: string): ItemState {
  const st = states.get(key) ?? emptyState(key)
  const lv = Math.min(Math.max(st.verifiedLevel, 0), MAX_LEVEL) as Level
  return lv === st.verifiedLevel ? st : { ...st, verifiedLevel: lv }
}

/** 依存項目がすべてレベル 1 以上か（＝着手できるか）。分野の外の項目も見る（Go の isReady） */
export function isReady(states: Map<string, ItemState>, item: RoadmapItem): boolean {
  return (item.dependsOn ?? []).every((dep) => stateOf(states, dep).verifiedLevel >= 1)
}

/** 「済」とみなせるか。一度は根拠が付いた（レベル 1 以上）（Go の isDone） */
export function isDone(states: Map<string, ItemState>, item: RoadmapItem): boolean {
  return stateOf(states, item.key).verifiedLevel >= 1
}

/** 印はあるが表示レベルに届いていない段（Go の ItemState.PendingLevels） */
export function pendingLevels(st: ItemState): Level[] {
  return st.evidencedLevels.filter((l) => l > st.verifiedLevel)
}

/** 鮮度から優先度の加点を返す。古いものほど再確認を促す（Go の staleBonus） */
function staleBonus(s: Staleness): number {
  if (s === 'stale') return 1.0
  if (s === 'aging') return 0.5
  return 0
}

/** 「その項目を前提に挙げている項目」の数。多いほど、終えたときに開く道が多い（Go の dependentCounts） */
function dependentCounts(items: RoadmapItem[]): Map<string, number> {
  const counts = new Map<string, number>()
  for (const it of items) {
    for (const dep of it.dependsOn ?? []) {
      counts.set(dep, (counts.get(dep) ?? 0) + 1)
    }
  }
  return counts
}

/**
 * 「次にやること」を優先度順に最大 n 件返す（Go の NextActions）。
 * 並び順は「着手できないものを常に後ろ」→「優先度の降順」→「ロードマップの定義順」。
 * これ以上やることが無い項目（レベル 5）は出さない。ただし要再確認なら再確認を促すために出す
 */
export function nextActions(
  roadmap: Roadmap,
  states: Map<string, ItemState>,
  today: Date,
  cfg: StalenessConfig,
  w: Weights,
  n: number,
): Action[] {
  if (n <= 0) return []
  const all = roadmap.domains.flatMap((d) => d.items)
  const deps = dependentCounts(all)
  const maxDeps = Math.max(0, ...deps.values())

  // 優先度の式は SPEC.md §5.1。目標日の逼迫度は全項目に同じ値が乗るだけなので入れない
  const score = (it: RoadmapItem, st: ItemState, ready: boolean): number => {
    const readiness = ready ? 1 : 0
    const gap = (MAX_LEVEL - st.verifiedLevel) / MAX_LEVEL
    const bonus = st.needsReview ? 1.0 : staleBonus(staleness(today, st.lastEvidenceAt, cfg))
    const unlocks = maxDeps > 0 ? (deps.get(it.key) ?? 0) / maxDeps : 0
    return w.readiness * readiness + w.gap * gap + w.staleness * bonus + w.unlocks * unlocks
  }

  const list: { action: Action; seq: number }[] = []
  let seq = 0
  for (const d of roadmap.domains) {
    for (const it of d.items) {
      seq++
      const st = stateOf(states, it.key)
      if (st.verifiedLevel >= MAX_LEVEL && !needsAttention(st, today, cfg)) continue
      const ready = isReady(states, it)
      list.push({
        action: {
          itemKey: it.key,
          domainKey: d.key,
          domainName: d.name,
          name: it.name,
          outcome: it.outcome,
          verifyBy: it.verifyBy,
          level: st.verifiedLevel,
          staleness: staleness(today, st.lastEvidenceAt, cfg),
          priority: score(it, st, ready),
          blocked: !ready,
          pendingLevels: pendingLevels(st),
        },
        seq,
      })
    }
  }

  list.sort((a, b) => {
    if (a.action.blocked !== b.action.blocked) return a.action.blocked ? 1 : -1
    if (a.action.priority !== b.action.priority) return b.action.priority - a.action.priority
    return a.seq - b.seq
  })
  return list.slice(0, n).map((e) => e.action)
}
