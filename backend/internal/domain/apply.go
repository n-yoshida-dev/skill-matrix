package domain

import "fmt"

// このファイルは判定を検証し、現在の状態に適用する処理を持つ。
// 検証ルールの一覧は SPEC.md §4.5（V1〜V8。V4 は欠番）。人間向けの解説は docs/spec-guide.md §4.4。
//
// 基本方針は3つ。
//   - 判定は「提案」であって確定値ではない。必ず検証を通してから適用する
//   - 弾いた／切り詰めた内容は握りつぶさず、すべて Violation として返す
//   - レベルは印から導く。1 件の根拠は、その根拠の梯子の範囲にだけ印を付ける（SPEC.md §1.1）

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
	return r
}

// decision は検証を通したあとの「こう適用する」という結論。
// 保留（V7）になった場合でも、何が適用されるはずだったかを示すために使う。
type decision struct {
	// marks はこの判定が付ける印（梯子の下端から top まで）。空なら印を付けない。
	marks []Level
	// top は marks の最上段。印を付けないときは 0。
	top         Level
	preState    PreState
	needsReview bool
	// refreshEvidence が true のとき LastEvidenceAt を更新する。
	refreshEvidence bool
}

// ApplyJudgment は判定1件を現在の状態に適用する。
//
// 検証ルール V2・V3・V5〜V7 を担当する。V1（実在しない項目）と V8（件数の上限）は
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

	d := decide(cur, j, &res.Violations)

	// V7: 確信度が閾値未満のものは適用せず保留にする。AI の判定だけが対象で、
	// 人の訂正（manual）や skill-map からの写し（migration）には確信度が無い（SPEC.md §3.3）。
	// 保留分は state.json の deferred に残し、採用するなら人が manual の判定を足す（SPEC.md §4.7）。
	// AI の判定に確信度が無ければ 0 とみなして保留にする（安全側）。形の検査が通していれば起きない
	if j.isAI() {
		conf := 0.0
		if j.HasConfidence {
			conf = j.Confidence
		}
		if conf < rules.ConfidenceThreshold {
			res.Deferred = true
			res.Violations = append(res.Violations, Violation{
				Code:     ViolationLowConfidence,
				ItemKey:  j.ItemKey,
				Proposed: j.ProposedLevel,
				Applied:  d.top,
				Detail: fmt.Sprintf("確信度 %.2f が閾値 %.2f 未満のため保留",
					conf, rules.ConfidenceThreshold),
			})
			return res
		}
	}

	next := cur
	next.ItemKey = j.ItemKey

	// 印を付け、表示レベルを導き直す（SPEC.md §4.5「適用の規則」の (1)(2)）
	for _, l := range d.marks {
		next.Evidenced[l] = true
	}
	next.VerifiedLevel = deriveVerifiedLevel(next.Evidenced)

	// (3) 段階前の状態は印が無いときだけ意味を持つ。印があれば常に none。
	// ゼロ値（""）は none と同じ意味なので、印が付いたときに "" を "none" に書き換えて
	// 「変わった」扱いにはしない
	if next.HasEvidence() {
		if next.PreState != "" {
			next.PreState = PreStateNone
		}
	} else if d.preState != PreStateNone {
		next.PreState = d.preState
	}

	// (5) 不合格の報告は印を付けず要再確認にする
	if d.needsReview {
		next.NeedsReview = true
	}

	// (4) 過去のログを後から投稿しても鮮度が巻き戻らないようにする
	if d.refreshEvidence && j.OccurredAt.After(next.LastEvidenceAt) {
		next.LastEvidenceAt = j.OccurredAt
		next.NeedsReview = false
	}

	res.State = next
	res.Marked = d.marks
	res.Changed = next != cur
	return res
}

// decide は検証ルール V5・V6 を適用して、どの印を付けるかを決める。
// 切り詰めた内容は vs に追記する。
func decide(cur ItemState, j Judgment, vs *[]Violation) decision {
	base, top, marks := j.EvidenceType.Ladder()

	// 印を付けない根拠（学習中／説明を受けた／自己申告）は、段階前の状態だけを立てる。
	// 「説明済み ≠ 理解」を機械が守るための分岐（SPEC.md §4.4）。
	// 理解を確認したわけではないので、根拠の鮮度も更新しない。
	if !marks {
		return decision{preState: j.EvidenceType.MarksPreState()}
	}

	// V6: 不合格の報告（ドリルに落ちた・説明できなかった）。
	// 印は付けず「要再確認」を立てるだけにする。下げるかどうかは人間の判断であって、
	// AI の一存で下げない（SPEC.md §1.3）。
	if j.ProposedLevel == LevelNone {
		*vs = append(*vs, Violation{
			Code:     ViolationFailedCheck,
			ItemKey:  j.ItemKey,
			Proposed: LevelNone,
			Applied:  cur.VerifiedLevel,
			Detail: fmt.Sprintf("不合格の報告（根拠 %q）。印は付けず、レベル %d を維持して要再確認とした",
				j.EvidenceType, cur.VerifiedLevel),
		})
		return decision{needsReview: true}
	}

	// V5: 提案した段が、その根拠の梯子の範囲内か。上限超えは上限へ、下端未満は下端へ。
	target := j.ProposedLevel
	if target > top {
		*vs = append(*vs, Violation{
			Code:     ViolationEvidenceTooWeak,
			ItemKey:  j.ItemKey,
			Proposed: target,
			Applied:  top,
			Detail: fmt.Sprintf("根拠 %q で印を付けられるのはレベル %d〜%d。%d を %d に切り詰めた",
				j.EvidenceType, base, top, target, top),
		})
		target = top
	} else if target < base {
		*vs = append(*vs, Violation{
			Code:     ViolationEvidenceTooWeak,
			ItemKey:  j.ItemKey,
			Proposed: target,
			Applied:  base,
			Detail: fmt.Sprintf("根拠 %q で印を付けられるのはレベル %d〜%d。%d を %d に切り上げた",
				j.EvidenceType, base, top, target, base),
		})
		target = base
	}

	var levels []Level
	for l := base; l <= target; l++ {
		levels = append(levels, l)
	}

	// 根拠の鮮度を更新するのは、この判定が付けた最上段の印が適用前の表示レベル以上のときだけ。
	// VerifiedLevel 3 の項目にドリル（印 1）が付いても「3 をまだ保持している」証明にはならない。
	// 印が [3] で VerifiedLevel 0 の項目にドリルが付いた場合は 1 ≥ 0 なので更新する（SPEC.md §3.4）。
	return decision{marks: levels, top: target, refreshEvidence: target >= cur.VerifiedLevel}
}

// BatchResult は1つの判定ファイルから得た判定をまとめて適用した結果。
type BatchResult struct {
	// States は適用後の状態。入力の map は変更せず、新しい map を返す。
	States map[ItemKey]ItemState
	// Results は受理された判定ごとの適用結果。入力の並び順を保つ。
	// **丸ごと捨てた判定はここに含まれない**（V1・V2・V3・V8）。捨てた事実は Violations に残る。
	// 保留（V7）は含まれる。反映はしないが、人が採否を決める対象として残すため。
	Results []Applied
	// Violations は判定単位・バッチ単位のすべての違反。
	// これをそのまま state.json（v2 では llm_responses.violations）に保存する。
	Violations []Violation
}

// ApplyJudgments は1つの判定ファイルから得た判定をまとめて適用する。
//
// V1（ロードマップに実在しない項目）と V8（1ファイルあたりの件数上限）をここで見る。
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
		// V8: 上限を超えた分は捨てる。移行の判定も免除しない
		if accepted >= rules.MaxItemsPerLog {
			out.Violations = append(out.Violations, Violation{
				Code:     ViolationTooManyItems,
				ItemKey:  j.ItemKey,
				Proposed: j.ProposedLevel,
				Rejected: true,
				Detail: fmt.Sprintf("1ファイルの判定は %d 件までのため棄却",
					rules.MaxItemsPerLog),
			})
			continue
		}

		// V1: ロードマップに実在しない項目。AI が項目を捏造することがある。
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
