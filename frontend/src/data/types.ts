// data/*.json の型。正本は SPEC.md §2（roadmap）・§3.4（state）・§8.3（settings）。
// Go 側（backend/internal/domain/types.go）と同じ語を使う。

/** 理解度の段階。0（未着手）〜 5（別文脈へ応用） */
export type Level = 0 | 1 | 2 | 3 | 4 | 5

/** 段階前の状態。印（evidencedLevels）が無いときだけ意味を持つ（SPEC.md §1.2） */
export type PreState = 'none' | 'learning' | 'explained_only' | 'self_reported'

/** 根拠の種類（SPEC.md §4.4） */
export type EvidenceType =
  | 'drill'
  | 'self_explanation'
  | 'implementation'
  | 'unaided_implementation'
  | 'cross_context'
  | 'learning_activity'
  | 'explained_to'
  | 'self_report'

/** 判定を書いた主体（SPEC.md §3.3） */
export type JudgmentSource = 'ai' | 'manual' | 'migration'

export interface LevelDef {
  level: 1 | 2 | 3 | 4 | 5
  name: string
  criteria: string
}

export interface RoadmapItem {
  key: string
  name: string
  description?: string
  /** 身につくと何ができるか。無いこともある（SPEC.md §2.1） */
  outcome?: string
  /** 次の確認方法 */
  verifyBy?: string
  dependsOn?: string[]
  moduleRefs?: string[]
}

export interface RoadmapDomain {
  key: string
  name: string
  goal?: string
  items: RoadmapItem[]
}

export interface Roadmap {
  schemaVersion: 1
  name: string
  description?: string
  origin?: 'manual' | 'external' | 'builtin'
  source?: string
  checkedAt?: string
  levels: LevelDef[]
  domains: RoadmapDomain[]
}

/** 項目に適用した判定 1 件の記録（state.json の events[]） */
export interface StateEvent {
  file: string
  index: number
  occurredAt: string
  source: JudgmentSource
  evidenceType: EvidenceType
  proposedLevel: Level
  /** この判定が付けた印 */
  marked: Level[]
  evidenceRefs: string[]
  rationale: string
  confidence: number | null
  violations: { code: string; detail: string }[]
}

export interface ItemState {
  itemKey: string
  /** 導出値。1 から途切れずに印が付いている最上段（SPEC.md §1.1） */
  verifiedLevel: Level
  /** 印。昇順 */
  evidencedLevels: Level[]
  preState: PreState
  needsReview: boolean
  lastEvidenceAt: string | null
  events: StateEvent[]
}

export interface PendingJudgment {
  file: string
  index: number
  itemKey: string
  code: string
  detail: string
}

export interface State {
  schemaVersion: 2
  items: ItemState[]
  deferred: PendingJudgment[]
  rejected: PendingJudgment[]
}

export interface Settings {
  rules?: { confidenceThreshold?: number; maxItemsPerLog?: number }
  staleness?: { freshWithinDays?: number; agingWithinDays?: number }
  weights?: { readiness?: number; gap?: number; staleness?: number; unlocks?: number }
  nextActions?: { limit?: number }
  /** 画面のフッターとヘッダが指すリポジトリ（v1.5 のテンプレート利用者は自分のものに変える） */
  site?: { repoUrl?: string }
}

/** 画面に渡す一式 */
export interface AppData {
  roadmap: Roadmap
  state: State
  settings: Settings
}
