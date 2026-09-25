import type { AppData, ItemState, Level, Roadmap, RoadmapItem, StateEvent } from '../../data/types'

// 公開ビュー（SPEC.md §7.3）のための純粋関数。DOM を触らない。
// 「何をどこまでできるか」「それは信用できるか」に答えるための集計と文言だけを置く。

/** レベルの名前。採用担当者向けに、基準を短い動詞で言い換えている */
export const LEVEL_NAMES: Record<Level, string> = {
  0: '未着手',
  1: '基礎を確認した',
  2: '自分の言葉で説明できる',
  3: '資料を見れば実装できる',
  4: '自力で実装しレビューできる',
  5: '別の場面でも使える',
}

/** 根拠の種類の日本語 */
export const EVIDENCE_LABELS: Record<StateEvent['evidenceType'], string> = {
  drill: '確認質問に答えた',
  self_explanation: '自分の言葉で説明した',
  implementation: '実装した',
  unaided_implementation: 'ガイドなしで実装した',
  cross_context: '別の場面で使った',
  learning_activity: '読んだ・写経した',
  explained_to: '説明を受けた',
  self_report: '自己申告',
}

/** 括弧の補足を外した短い名前。タイルに出す（正式名は根拠の見出しに出る） */
export function shortName(name: string): string {
  return name.replace(/（[^）]*）/g, '').trim()
}

/** 項目キー → 状態の対応表 */
export function stateByKey(data: AppData): Map<string, ItemState> {
  return new Map(data.state.items.map((s) => [s.itemKey, s]))
}

/** 印が 1 つでもある（何らかの根拠がある）か */
export function hasEvidence(st: ItemState): boolean {
  return st.evidencedLevels.length > 0
}

/** 付いている印の最上段。印が無ければ 0 */
export function topEvidenced(st: ItemState): Level {
  return st.evidencedLevels.length > 0 ? st.evidencedLevels[st.evidencedLevels.length - 1] : 0
}

/** 実装の根拠（3 以上の印）はあるが、基礎（1・2）の確認が未了か（SPEC.md §1.1 の [3] のケース） */
export function isImplementedButUnverified(st: ItemState): boolean {
  return topEvidenced(st) > st.verifiedLevel
}

export interface DomainSummary {
  key: string
  name: string
  goal: string
  total: number
  /** 根拠のある項目数 */
  withEvidence: number
}

/** 分野ごとの「M 項目中 X に根拠」 */
export function summarizeDomains(data: AppData): DomainSummary[] {
  const states = stateByKey(data)
  return data.roadmap.domains.map((d) => ({
    key: d.key,
    name: d.name,
    goal: d.goal ?? '',
    total: d.items.length,
    withEvidence: d.items.filter((it) => {
      const st = states.get(it.key)
      return st !== undefined && hasEvidence(st)
    }).length,
  }))
}

export interface OverallSummary {
  domains: number
  items: number
  withEvidence: number
  /** verifiedLevel が 3 以上（実装まで届いている） */
  canImplement: number
}

/** 見出し下の 2 文目に使う全体の数 */
export function summarizeOverall(data: AppData): OverallSummary {
  const states = stateByKey(data)
  const all = allItems(data.roadmap)
  const sts = all.map((it) => states.get(it.key)).filter((s): s is ItemState => s !== undefined)
  return {
    domains: data.roadmap.domains.length,
    items: all.length,
    withEvidence: sts.filter(hasEvidence).length,
    canImplement: sts.filter((s) => s.verifiedLevel >= 3).length,
  }
}

export function allItems(roadmap: Roadmap): RoadmapItem[] {
  return roadmap.domains.flatMap((d) => d.items)
}

/** 印を付けた判定（新しい順）と、印を付けなかった判定に分ける */
export function splitEvents(st: ItemState): { marked: StateEvent[]; unmarked: StateEvent[] } {
  const marked = st.events
    .filter((ev) => ev.marked.length > 0)
    .slice()
    .reverse()
  const unmarked = st.events.filter((ev) => ev.marked.length === 0)
  return { marked, unmarked }
}
