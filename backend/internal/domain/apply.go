package domain

import "fmt"

// このファイルは LLM が返した判定を検証し、現在の状態に適用する処理を持つ。
// 検証ルールの一覧は SPEC.md §4.5（V1〜V8）。人間向けの解説は docs/spec-guide.md §4.4。
//
// 基本方針は3つ。
//   - LLM の出力は「提案」であって確定値ではない。必ず検証を通してから適用する
//   - 弾いた／切り詰めた内容は握りつぶさず、すべて Violation として返す
//   - レベルは自動で下げない。降格の提案は「要再確認」の印に変換する

// withDefaults はゼロ値のフィールドを既定値で埋める。
//
// Rules{} をうっかり渡したときに検証が丸ごと無効化されるのを防ぐため、
// ゼロ値は「未設定」とみなす。閾値を意図的に無効化したい場合は負の値を渡す。
func (r Rules) withDefaults() Rules {
	d := DefaultRules()
	if r.ConfidenceThreshold == 0 {
		r.ConfidenceThreshold = d.ConfidenceThreshold
	}
	if r.MaxItemsPerLog == 0 {
		r.MaxItemsPerLog = d.MaxItemsPerLog
	}
	if r.MaxLevelStep == 0 {
		r.MaxLevelStep = d.MaxLevelStep
	}
	return r
}

// decision は検証を通したあとの「こう適用する」という結論。
// 保留（V7）になった場合でも、何が適用されるはずだったかを示すために使う。
type decision struct {
	level       Level
	preState    PreState
	needsReview bool
	// refreshEvidence が true のとき LastEvidenceAt を更新する。
	refreshEvidence bool
}

// ApplyJudgment は判定1件を現在の状態に適用する。
//
// 検証ルール V2〜V7 を担当する。V1（実在しない項目）と V8（件数の上限）は
// ロードマップ全体と判定の並びが必要なため ApplyJudgments 側で見る。
//
// cur がゼロ値（まだ一度も判定されていない項目）でも安全に呼べる。
func ApplyJudgment(cur ItemState, j Judgment, rules Rules) Applied {
	rules = rules.withDefaults()
	res := Applied{Judgment: j, State: cur}

	// V2: レベルが 0〜5 の範囲にあるか
	if !j.ProposedLevel.Valid() {
		res.Rejected = true
		res.Violations = append(res.Violations, Violation{
			Code:     ViolationLevelOutOfRange,
			ItemKey:  j.ItemKey,
			Proposed: j.ProposedLevel,
			Rejected: true,
			Detail:   fmt.Sprintf("レベルは 0〜%d の整数でなければならない", MaxLevel),
		})
		return res
	}

	// V3: 根拠の種類が許可リストにあるか
	if !j.EvidenceType.Valid() {
		res.Rejected = true
		res.Violations = append(res.Violations, Violation{
			Code:     ViolationUnknownEvidence,
			ItemKey:  j.ItemKey,
			Proposed: j.ProposedLevel,
			Rejected: true,
			Detail:   fmt.Sprintf("許可されていない根拠の種類: %q", j.EvidenceType),
		})
		return res
	}

	d := decide(cur, j, rules, &res.Violations)

	// V7: 確信度が閾値未満のものは適用せず保留にする。
	// 保留分は assessment_events に書かず、人が承認してから積む（SPEC.md §4.7）。
	if j.Confidence < rules.ConfidenceThreshold {
		res.Deferred = true
		res.Violations = append(res.Violations, Violation{
			Code:     ViolationLowConfidence,
			ItemKey:  j.ItemKey,
			Proposed: j.ProposedLevel,
			Applied:  d.level,
			Detail: fmt.Sprintf("確信度 %.2f が閾値 %.2f 未満のため保留",
				j.Confidence, rules.ConfidenceThreshold),
		})
		return res
	}

	next := cur
	next.ItemKey = j.ItemKey
	next.Level = d.level
	if d.preState != PreStateNone {
		next.PreState = d.preState
	}
	if d.needsReview {
		next.NeedsReview = true
	}
	// 過去のログを後から投稿しても鮮度が巻き戻らないようにする。
	if d.refreshEvidence && j.OccurredAt.After(next.LastEvidenceAt) {
		next.LastEvidenceAt = j.OccurredAt
		next.NeedsReview = false
	}

	res.State = next
	res.Changed = next != cur
	return res
}

// decide は検証ルール V4〜V6 を適用して、最終的にどう反映するかを決める。
// 切り詰めた内容は vs に追記する。
func decide(cur ItemState, j Judgment, rules Rules, vs *[]Violation) decision {
	limit, promotes := j.EvidenceType.MaxLevel()

	// 昇格しない根拠（説明を受けた／自己申告）は、レベルを動かさず段階前の状態だけを立てる。
	// 「説明済み ≠ 理解」を機械が守るための分岐（SPEC.md §4.4）。
	// 理解を確認したわけではないので、根拠の鮮度も更新しない。
	if !promotes {
		return decision{
			level:    cur.Level,
			preState: j.EvidenceType.MarksPreState(),
		}
	}

	target := j.ProposedLevel

	// V6: 降格の提案。レベルは下げず「要再確認」の印を立てるだけにする。
	// 下げるかどうかは人間の判断であって、LLM の一存で下げない（SPEC.md §1.3）。
	if target < cur.Level {
		*vs = append(*vs, Violation{
			Code:     ViolationDowngrade,
			ItemKey:  j.ItemKey,
			Proposed: target,
			Applied:  cur.Level,
			Detail: fmt.Sprintf("レベル %d への降格が提案されたが、%d を維持して要再確認とした",
				target, cur.Level),
		})
		return decision{level: cur.Level, needsReview: true}
	}

	// V5: その根拠で到達できる上限に切り詰める。
	if target > limit {
		*vs = append(*vs, Violation{
			Code:     ViolationEvidenceTooWeak,
			ItemKey:  j.ItemKey,
			Proposed: target,
			Applied:  limit,
			Detail: fmt.Sprintf("根拠 %q で到達できるのはレベル %d まで",
				j.EvidenceType, limit),
		})
		target = limit
	}

	// 上限で切り詰めた結果が現在レベルを下回ることがある。
	// 例：レベル3の項目にドリル（上限レベル1）の根拠が付いた場合。据え置く。
	if target < cur.Level {
		target = cur.Level
	}

	// V4: 一気飛びの禁止。1回の判定で上がれるのは MaxLevelStep 段まで。
	// 1つのログに4段階分の根拠が揃うことはまずないため。
	if maxStep := cur.Level + Level(rules.MaxLevelStep); target > maxStep {
		*vs = append(*vs, Violation{
			Code:     ViolationLevelJump,
			ItemKey:  j.ItemKey,
			Proposed: target,
			Applied:  maxStep,
			Detail: fmt.Sprintf("1回に上がれるのは %d 段まで（レベル %d → %d に切り詰め）",
				rules.MaxLevelStep, cur.Level, maxStep),
		})
		target = maxStep
	}

	// 根拠の鮮度を更新するのは、その根拠が現在のレベルを支えられる強さのときだけ。
	// レベル3の項目にドリル（上限レベル1）の根拠が付いても
	// 「レベル3をまだ保持している」証明にはならないので、古いままにしておく。
	return decision{level: target, refreshEvidence: limit >= cur.Level}
}

// BatchResult は1つの学習ログから得た判定をまとめて適用した結果。
type BatchResult struct {
	// States は適用後の状態。入力の map は変更せず、新しい map を返す。
	States map[ItemKey]ItemState
	// Results は受理された判定ごとの適用結果。入力の並び順を保つ。
	// **丸ごと捨てた判定はここに含まれない**（V1・V2・V3・V8）。捨てた事実は Violations に残る。
	// 保留（V7）は含まれる。反映はしないが、人が採否を決める対象として残すため。
	Results []Applied
	// Violations は判定単位・バッチ単位のすべての違反。
	// これをそのまま llm_responses.violations に保存する。
	Violations []Violation
}

// ApplyJudgments は1つの学習ログから得た判定をまとめて適用する。
//
// V1（ロードマップに実在しない項目）と V8（1ログあたりの件数上限）をここで見る。
// 判定は項目ごとに独立して扱い、合計や配点の制約は設けない（SPEC.md §4.5）。
func ApplyJudgments(rm Roadmap, states map[ItemKey]ItemState, js []Judgment, rules Rules) BatchResult {
	rules = rules.withDefaults()
	known := rm.ItemKeySet()

	out := BatchResult{States: make(map[ItemKey]ItemState, len(states))}
	for k, v := range states {
		out.States[k] = v
	}

	accepted := 0
	for _, j := range js {
		// V8: 上限を超えた分は捨てる。
		if accepted >= rules.MaxItemsPerLog {
			out.Violations = append(out.Violations, Violation{
				Code:     ViolationTooManyItems,
				ItemKey:  j.ItemKey,
				Proposed: j.ProposedLevel,
				Rejected: true,
				Detail: fmt.Sprintf("1ログの判定は %d 件までのため棄却",
					rules.MaxItemsPerLog),
			})
			continue
		}

		// V1: ロードマップに実在しない項目。LLM が項目を捏造することがある。
		if _, ok := known[j.ItemKey]; !ok {
			out.Violations = append(out.Violations, Violation{
				Code:     ViolationUnknownItem,
				ItemKey:  j.ItemKey,
				Proposed: j.ProposedLevel,
				Rejected: true,
				Detail:   fmt.Sprintf("ロードマップに存在しない項目: %q", j.ItemKey),
			})
			continue
		}
		accepted++

		cur, ok := out.States[j.ItemKey]
		if !ok {
			cur = ItemState{ItemKey: j.ItemKey}
		}

		res := ApplyJudgment(cur, j, rules)
		out.Violations = append(out.Violations, res.Violations...)

		// V2・V3 で棄却したものは Results に入れない。判定が無かったのと同じ扱いにする。
		// 入れてしまうと、呼び出し側が「棄却されたこと」を確かめずに保存へ流し、
		// 範囲外のレベルが DB の制約違反を起こす（2026-09-23 に PR #29 のレビューで判明）
		if res.Rejected {
			continue
		}
		if !res.Deferred {
			out.States[j.ItemKey] = res.State
		}
		out.Results = append(out.Results, res)
	}

	return out
}
