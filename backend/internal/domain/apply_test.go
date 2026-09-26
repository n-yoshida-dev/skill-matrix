package domain

import (
	"slices"
	"testing"
	"time"
)

// このテストは「AI（または人）がこう言ってきたら、こう扱う」を具体例で固定するもの。
// 検証ルールの意図を読み取りたいときは SPEC.md §4.5 より先にここを読むとよい。
//
// 2026-09-25 にレベルの決まり方が「1 本の数直線」から「印（Evidenced）からの導出」に変わった（SPEC.md §1.1）。
// 理解の梯子（1→2）と実装の梯子（3→4→5）は独立で、実装の根拠は基礎理解を含意しない。

var (
	day01 = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	day10 = time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
)

// testRoadmap は検証用の最小限のロードマップ。
func testRoadmap() Roadmap {
	return Roadmap{
		Name: "テスト用",
		Domains: []Domain{{
			Key:  "go",
			Name: "Go",
			Items: []Item{
				{Key: "go-01", DomainKey: "go", Name: "基本構文"},
				{Key: "go-02", DomainKey: "go", Name: "メソッド・インターフェース", DependsOn: []ItemKey{"go-01"}},
			},
		}},
	}
}

// codes は違反の識別子だけを取り出す。テストの比較を読みやすくするため。
func codes(vs []Violation) []ViolationCode {
	out := make([]ViolationCode, 0, len(vs))
	for _, v := range vs {
		out = append(out, v.Code)
	}
	return out
}

// evidenced は印の一覧から Evidenced の配列を作る。テストの入力を短く書くため。
func evidenced(levels ...Level) [LevelCount]bool {
	var ev [LevelCount]bool
	for _, l := range levels {
		ev[l] = true
	}
	return ev
}

// stateWith は印と、そこから導いた VerifiedLevel を持つ状態を作る。
func stateWith(key ItemKey, levels ...Level) ItemState {
	ev := evidenced(levels...)
	return ItemState{ItemKey: key, Evidenced: ev, VerifiedLevel: deriveVerifiedLevel(ev)}
}

// aiJudgment は AI の判定（confidence あり）を短く書くための補助。
func aiJudgment(key ItemKey, ev EvidenceType, level Level, conf float64, at time.Time) Judgment {
	return Judgment{
		ItemKey: key, ProposedLevel: level, EvidenceType: ev,
		Source: SourceAI, Confidence: conf, HasConfidence: true,
		EvidenceRefs: []string{"repo:example/study@0000000/log.md"}, OccurredAt: at,
	}
}

func TestDeriveVerifiedLevel(t *testing.T) {
	tests := []struct {
		marks []Level
		want  Level
	}{
		{nil, 0},
		{[]Level{3}, 0},          // 実装の根拠だけでは表示レベルは 0
		{[]Level{1, 3}, 1},       // 2 が抜けているので 1 で止まる
		{[]Level{1, 2, 3}, 3},    // 途切れずに付いていれば最上段
		{[]Level{1, 2, 3, 4}, 4}, // 4 まで
		{[]Level{2, 3}, 0},       // 1 が無ければ 0
	}
	for _, tt := range tests {
		if got := deriveVerifiedLevel(evidenced(tt.marks...)); got != tt.want {
			t.Errorf("印 %v → VerifiedLevel = %d, want %d", tt.marks, got, tt.want)
		}
	}
}

func TestApplyJudgment(t *testing.T) {
	tests := []struct {
		name string
		cur  ItemState
		j    Judgment

		wantEvidenced   []Level
		wantVerified    Level
		wantMarked      []Level
		wantPreState    PreState
		wantNeedsReview bool
		wantEvidenceAt  time.Time
		wantDeferred    bool
		wantChanged     bool
		wantViolations  []ViolationCode
	}{
		{
			name:          "未着手にドリルの根拠で印 1 が付き、表示レベル 1 になる",
			cur:           ItemState{ItemKey: "go-01"},
			j:             aiJudgment("go-01", EvidenceDrill, LevelBasicConfirmed, 0.9, day10),
			wantEvidenced: []Level{1}, wantVerified: 1, wantMarked: []Level{1},
			wantEvidenceAt: day10, wantChanged: true,
		},
		{
			name:          "自分の言葉での説明は 1 と 2 の印を同時に付ける",
			cur:           ItemState{ItemKey: "go-01"},
			j:             aiJudgment("go-01", EvidenceSelfExplanation, LevelCanExplain, 0.9, day10),
			wantEvidenced: []Level{1, 2}, wantVerified: 2, wantMarked: []Level{1, 2},
			wantEvidenceAt: day10, wantChanged: true,
		},
		{
			name:          "実装の根拠は 3 の印だけを付け、1・2 を含意しないので表示レベルは 0 のまま",
			cur:           ItemState{ItemKey: "go-01"},
			j:             aiJudgment("go-01", EvidenceImplementation, LevelGuidedImpl, 0.9, day10),
			wantEvidenced: []Level{3}, wantVerified: 0, wantMarked: []Level{3},
			wantEvidenceAt: day10, wantChanged: true,
		},
		{
			name:          "印 [3] の項目に説明の根拠が付くと 1・2 が埋まり、表示レベルが 3 に跳ぶ（V4 は廃止）",
			cur:           stateWith("go-01", 3),
			j:             aiJudgment("go-01", EvidenceSelfExplanation, LevelCanExplain, 0.9, day10),
			wantEvidenced: []Level{1, 2, 3}, wantVerified: 3, wantMarked: []Level{1, 2},
			wantEvidenceAt: day10, wantChanged: true,
		},
		{
			name:          "ガイドなしの実装は 3 と 4 を同時に付ける",
			cur:           stateWith("go-01", 1, 2),
			j:             aiJudgment("go-01", EvidenceUnaidedImplementation, LevelIndependentImpl, 0.9, day10),
			wantEvidenced: []Level{1, 2, 3, 4}, wantVerified: 4, wantMarked: []Level{3, 4},
			wantEvidenceAt: day10, wantChanged: true,
		},
		{
			name:          "proposedLevel を梯子の途中にすれば、そこまでしか印を付けない",
			cur:           ItemState{ItemKey: "go-01"},
			j:             aiJudgment("go-01", EvidenceUnaidedImplementation, LevelGuidedImpl, 0.9, day10),
			wantEvidenced: []Level{3}, wantVerified: 0, wantMarked: []Level{3},
			wantEvidenceAt: day10, wantChanged: true,
		},
		{
			name:           "V2 レベルが範囲外なら丸ごと棄却する",
			cur:            ItemState{ItemKey: "go-01"},
			j:              aiJudgment("go-01", EvidenceDrill, MaxLevel+1, 0.9, day10),
			wantViolations: []ViolationCode{ViolationLevelOutOfRange},
		},
		{
			name:           "V2 負のレベルも棄却する",
			cur:            ItemState{ItemKey: "go-01"},
			j:              aiJudgment("go-01", EvidenceDrill, -1, 0.9, day10),
			wantViolations: []ViolationCode{ViolationLevelOutOfRange},
		},
		{
			name:           "V3 許可リストに無い根拠は棄却する",
			cur:            ItemState{ItemKey: "go-01"},
			j:              aiJudgment("go-01", "vibes", LevelBasicConfirmed, 0.9, day10),
			wantViolations: []ViolationCode{ViolationUnknownEvidence},
		},
		{
			name:          "V5 ドリルの根拠でレベル 3 は主張できないので上限 1 に切り詰める",
			cur:           ItemState{ItemKey: "go-01"},
			j:             aiJudgment("go-01", EvidenceDrill, LevelGuidedImpl, 0.9, day10),
			wantEvidenced: []Level{1}, wantVerified: 1, wantMarked: []Level{1},
			wantEvidenceAt: day10, wantChanged: true,
			wantViolations: []ViolationCode{ViolationEvidenceTooWeak},
		},
		{
			name:          "V5 実装の根拠でレベル 2 は梯子の下端未満なので 3 に切り上げる",
			cur:           ItemState{ItemKey: "go-01"},
			j:             aiJudgment("go-01", EvidenceImplementation, LevelCanExplain, 0.9, day10),
			wantEvidenced: []Level{3}, wantVerified: 0, wantMarked: []Level{3},
			wantEvidenceAt: day10, wantChanged: true,
			wantViolations: []ViolationCode{ViolationEvidenceTooWeak},
		},
		{
			name:          "V6 不合格の報告（proposedLevel 0）は印を付けず要再確認にする",
			cur:           stateWith("go-01", 1, 2, 3),
			j:             aiJudgment("go-01", EvidenceDrill, LevelNone, 0.9, day10),
			wantEvidenced: []Level{1, 2, 3}, wantVerified: 3, wantNeedsReview: true,
			wantChanged:    true,
			wantViolations: []ViolationCode{ViolationFailedCheck},
		},
		{
			name:           "V7 確信度が低い AI の判定は適用せず保留にする",
			cur:            ItemState{ItemKey: "go-01"},
			j:              aiJudgment("go-01", EvidenceDrill, LevelBasicConfirmed, 0.3, day10),
			wantDeferred:   true,
			wantViolations: []ViolationCode{ViolationLowConfidence},
		},
		{
			name: "V7 AI の判定に確信度が無ければ 0 とみなして保留にする（安全側）",
			cur:  ItemState{ItemKey: "go-01"},
			j: Judgment{
				ItemKey: "go-01", ProposedLevel: LevelBasicConfirmed, EvidenceType: EvidenceDrill,
				Source: SourceAI, EvidenceRefs: []string{"log:a.md"}, OccurredAt: day10,
			},
			wantDeferred:   true,
			wantViolations: []ViolationCode{ViolationLowConfidence},
		},
		{
			name: "V7 は manual の判定には掛からない（確信度は書かれない）",
			cur:  ItemState{ItemKey: "go-01"},
			j: Judgment{
				ItemKey: "go-01", ProposedLevel: LevelBasicConfirmed, EvidenceType: EvidenceDrill,
				Source: SourceManual, EvidenceRefs: []string{"repo:example/study@0000000/log.md"}, OccurredAt: day10,
			},
			wantEvidenced: []Level{1}, wantVerified: 1, wantMarked: []Level{1},
			wantEvidenceAt: day10, wantChanged: true,
		},
		{
			name: "V7 は migration の判定にも掛からない",
			cur:  ItemState{ItemKey: "go-01"},
			j: Judgment{
				ItemKey: "go-01", ProposedLevel: LevelGuidedImpl, EvidenceType: EvidenceImplementation,
				Source: SourceMigration, EvidenceRefs: []string{"repo:example/study@0000000/skill-map.md#L1"}, OccurredAt: day01,
			},
			wantEvidenced: []Level{3}, wantVerified: 0, wantMarked: []Level{3},
			wantEvidenceAt: day01, wantChanged: true,
		},
		{
			name:         "説明を受けただけでは印が付かず、段階前の状態だけ立ち、鮮度も更新されない",
			cur:          ItemState{ItemKey: "go-01"},
			j:            aiJudgment("go-01", EvidenceExplainedTo, LevelGuidedImpl, 0.9, day10),
			wantPreState: PreStateExplainedOnly, wantChanged: true,
		},
		{
			name:         "学習を始めただけなら「学習中」の印だけ立つ",
			cur:          ItemState{ItemKey: "go-01"},
			j:            aiJudgment("go-01", EvidenceLearningActivity, LevelBasicConfirmed, 0.9, day10),
			wantPreState: PreStateLearning, wantChanged: true,
		},
		{
			name:         "自己申告も印を付けず段階前の状態だけ立てる",
			cur:          ItemState{ItemKey: "go-01"},
			j:            aiJudgment("go-01", EvidenceSelfReport, LevelIndependentImpl, 0.9, day10),
			wantPreState: PreStateSelfReported, wantChanged: true,
		},
		{
			name:          "印がある項目に「説明を受けた」が来ても段階前の状態は none のまま（印が優先）",
			cur:           stateWith("go-01", 3),
			j:             aiJudgment("go-01", EvidenceExplainedTo, LevelGuidedImpl, 0.9, day10),
			wantEvidenced: []Level{3}, wantVerified: 0,
			wantChanged: false,
		},
		{
			name:          "印が付くと、それまでの段階前の状態は消える",
			cur:           ItemState{ItemKey: "go-01", PreState: PreStateExplainedOnly},
			j:             aiJudgment("go-01", EvidenceDrill, LevelBasicConfirmed, 0.9, day10),
			wantEvidenced: []Level{1}, wantVerified: 1, wantMarked: []Level{1},
			wantPreState: PreStateNone, wantEvidenceAt: day10, wantChanged: true,
		},
		{
			name: "過去のログを後から投稿しても鮮度は巻き戻らない",
			cur: func() ItemState {
				s := stateWith("go-01", 1)
				s.LastEvidenceAt = day10
				return s
			}(),
			j:             aiJudgment("go-01", EvidenceSelfExplanation, LevelCanExplain, 0.9, day01),
			wantEvidenced: []Level{1, 2}, wantVerified: 2, wantMarked: []Level{1, 2},
			wantEvidenceAt: day10, wantChanged: true,
		},
		{
			name: "表示レベル 3 の項目にドリル（印 1）が付いても、鮮度は更新しない",
			cur: func() ItemState {
				s := stateWith("go-01", 1, 2, 3)
				s.LastEvidenceAt = day01
				return s
			}(),
			j:             aiJudgment("go-01", EvidenceDrill, LevelBasicConfirmed, 0.9, day10),
			wantEvidenced: []Level{1, 2, 3}, wantVerified: 3, wantMarked: []Level{1},
			wantEvidenceAt: day01, wantChanged: false,
		},
		{
			name: "印 [3] で表示レベル 0 の項目にドリルが付けば、1 ≥ 0 なので鮮度は更新する",
			cur: func() ItemState {
				s := stateWith("go-01", 3)
				s.LastEvidenceAt = day01
				return s
			}(),
			j:             aiJudgment("go-01", EvidenceDrill, LevelBasicConfirmed, 0.9, day10),
			wantEvidenced: []Level{1, 3}, wantVerified: 1, wantMarked: []Level{1},
			wantEvidenceAt: day10, wantChanged: true,
		},
		{
			name: "要再確認は十分な根拠が付けば解除される",
			cur: func() ItemState {
				s := stateWith("go-01", 1, 2)
				s.NeedsReview = true
				s.LastEvidenceAt = day01
				return s
			}(),
			j:             aiJudgment("go-01", EvidenceImplementation, LevelGuidedImpl, 0.9, day10),
			wantEvidenced: []Level{1, 2, 3}, wantVerified: 3, wantMarked: []Level{3},
			wantNeedsReview: false, wantEvidenceAt: day10, wantChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyJudgment(tt.cur, tt.j, DefaultRules())

			if !slices.Equal(got.State.EvidencedLevels(), tt.wantEvidenced) {
				t.Errorf("Evidenced = %v, want %v", got.State.EvidencedLevels(), tt.wantEvidenced)
			}
			if got.State.VerifiedLevel != tt.wantVerified {
				t.Errorf("VerifiedLevel = %d, want %d", got.State.VerifiedLevel, tt.wantVerified)
			}
			if !slices.Equal(got.Marked, tt.wantMarked) {
				t.Errorf("Marked = %v, want %v", got.Marked, tt.wantMarked)
			}
			wantPre := tt.wantPreState
			if wantPre == "" {
				wantPre = tt.cur.PreState
			}
			if got.State.PreState != wantPre {
				t.Errorf("PreState = %q, want %q", got.State.PreState, wantPre)
			}
			if got.State.NeedsReview != tt.wantNeedsReview {
				t.Errorf("NeedsReview = %v, want %v", got.State.NeedsReview, tt.wantNeedsReview)
			}
			if !got.State.LastEvidenceAt.Equal(tt.wantEvidenceAt) {
				t.Errorf("LastEvidenceAt = %v, want %v", got.State.LastEvidenceAt, tt.wantEvidenceAt)
			}
			if got.Deferred != tt.wantDeferred {
				t.Errorf("Deferred = %v, want %v", got.Deferred, tt.wantDeferred)
			}
			if got.Changed != tt.wantChanged {
				t.Errorf("Changed = %v, want %v", got.Changed, tt.wantChanged)
			}
			if gotCodes := codes(got.Violations); !slices.Equal(gotCodes, tt.wantViolations) {
				t.Errorf("Violations = %v, want %v", gotCodes, tt.wantViolations)
			}
		})
	}
}

// 保留になった判定は状態を一切変えない。採用されるまで反映しないため。
func TestApplyJudgment_保留は状態を変えない(t *testing.T) {
	cur := stateWith("go-01", 1, 2)
	cur.LastEvidenceAt = day01
	got := ApplyJudgment(cur, aiJudgment("go-01", EvidenceImplementation, LevelGuidedImpl, 0.1, day10), DefaultRules())

	if !got.Deferred {
		t.Fatal("Deferred = false, want true")
	}
	if got.State != cur {
		t.Errorf("State = %+v, want 変更なし %+v", got.State, cur)
	}
	if got.Marked != nil {
		t.Errorf("Marked = %v, want 空（保留では印を付けない）", got.Marked)
	}
	// 保留でも「何が適用されるはずだったか」は記録に残す
	if len(got.Violations) != 1 || got.Violations[0].Applied != LevelGuidedImpl {
		t.Errorf("Violations = %+v, want Applied=%d の記録", got.Violations, LevelGuidedImpl)
	}
}

// Rules のゼロ値を渡しても検証が無効化されないこと。
func TestApplyJudgment_ゼロ値のRulesは既定値で埋まる(t *testing.T) {
	got := ApplyJudgment(ItemState{ItemKey: "go-01"},
		aiJudgment("go-01", EvidenceDrill, LevelBasicConfirmed, 0.3, day10), Rules{})

	if !got.Deferred {
		t.Error("確信度 0.3 なのに保留にならない（V7 の閾値が既定値ではたらくこと）")
	}
}

func TestPendingLevels(t *testing.T) {
	st := stateWith("go-01", 1, 3, 4)
	if got, want := st.PendingLevels(), []Level{3, 4}; !slices.Equal(got, want) {
		t.Errorf("PendingLevels = %v, want %v", got, want)
	}
	if got := stateWith("go-01", 1, 2).PendingLevels(); got != nil {
		t.Errorf("届いている項目の PendingLevels = %v, want 空", got)
	}
	if got, want := st.TopEvidenced(), LevelIndependentImpl; got != want {
		t.Errorf("TopEvidenced = %d, want %d", got, want)
	}
}

func TestApplyJudgments_V1_実在しない項目は棄却する(t *testing.T) {
	rm := testRoadmap()
	got := ApplyJudgments(rm, nil, []Judgment{
		aiJudgment("go-99", EvidenceDrill, LevelBasicConfirmed, 0.9, day10),
		aiJudgment("go-01", EvidenceDrill, LevelBasicConfirmed, 0.9, day10),
	}, DefaultRules())

	if _, ok := got.States["go-99"]; ok {
		t.Error("実在しない項目の状態が作られている")
	}
	if got.States["go-01"].VerifiedLevel != LevelBasicConfirmed {
		t.Errorf("go-01 の VerifiedLevel = %d, want %d", got.States["go-01"].VerifiedLevel, LevelBasicConfirmed)
	}
	if want := []ViolationCode{ViolationUnknownItem}; !slices.Equal(codes(got.Violations), want) {
		t.Errorf("Violations = %v, want %v", codes(got.Violations), want)
	}
	if len(got.Results) != 1 {
		t.Errorf("Results = %d 件, want 1 件（棄却分は含めない）", len(got.Results))
	}
}

func TestApplyJudgments_V8_件数の上限を超えた分は棄却する(t *testing.T) {
	rm := testRoadmap()
	rules := DefaultRules()
	rules.MaxItemsPerLog = 1

	got := ApplyJudgments(rm, nil, []Judgment{
		aiJudgment("go-01", EvidenceDrill, LevelBasicConfirmed, 0.9, day10),
		aiJudgment("go-02", EvidenceDrill, LevelBasicConfirmed, 0.9, day10),
	}, rules)

	if got.States["go-01"].VerifiedLevel != LevelBasicConfirmed {
		t.Error("1件目が適用されていない")
	}
	if _, ok := got.States["go-02"]; ok {
		t.Error("上限を超えた2件目が適用されている")
	}
	if want := []ViolationCode{ViolationTooManyItems}; !slices.Equal(codes(got.Violations), want) {
		t.Errorf("Violations = %v, want %v", codes(got.Violations), want)
	}
}

// 1つのログが複数項目にまたがっても、項目ごとに独立して判定する（配点の分配はしない）。
func TestApplyJudgments_項目ごとに独立して適用する(t *testing.T) {
	rm := testRoadmap()
	got := ApplyJudgments(rm, nil, []Judgment{
		aiJudgment("go-01", EvidenceSelfExplanation, LevelCanExplain, 0.9, day10),
		aiJudgment("go-02", EvidenceDrill, LevelBasicConfirmed, 0.9, day10),
	}, DefaultRules())

	if got.States["go-01"].VerifiedLevel != LevelCanExplain {
		t.Errorf("go-01 の VerifiedLevel = %d, want %d", got.States["go-01"].VerifiedLevel, LevelCanExplain)
	}
	if got.States["go-02"].VerifiedLevel != LevelBasicConfirmed {
		t.Errorf("go-02 の VerifiedLevel = %d, want %d", got.States["go-02"].VerifiedLevel, LevelBasicConfirmed)
	}
	if len(got.Results) != 2 {
		t.Errorf("Results = %d 件, want 2 件", len(got.Results))
	}
}

// 同じファイルの中で同じ項目に 2 件の判定があれば、順に積み上がる（処理順は配列の順。SPEC.md §3.2）。
func TestApplyJudgments_同じ項目への判定は順に積み上がる(t *testing.T) {
	rm := testRoadmap()
	got := ApplyJudgments(rm, nil, []Judgment{
		aiJudgment("go-01", EvidenceImplementation, LevelGuidedImpl, 0.9, day01),
		aiJudgment("go-01", EvidenceSelfExplanation, LevelCanExplain, 0.9, day10),
	}, DefaultRules())

	st := got.States["go-01"]
	if got, want := st.EvidencedLevels(), []Level{1, 2, 3}; !slices.Equal(got, want) {
		t.Errorf("Evidenced = %v, want %v", got, want)
	}
	if st.VerifiedLevel != LevelGuidedImpl {
		t.Errorf("VerifiedLevel = %d, want 3", st.VerifiedLevel)
	}
	if !st.LastEvidenceAt.Equal(day10) {
		t.Errorf("LastEvidenceAt = %v, want %v", st.LastEvidenceAt, day10)
	}
}

// 入力として渡した states を書き換えないこと（呼び出し側の値が壊れないように）。
func TestApplyJudgments_入力のstatesを破壊しない(t *testing.T) {
	rm := testRoadmap()
	in := map[ItemKey]ItemState{
		"go-01": {ItemKey: "go-01"},
	}

	got := ApplyJudgments(rm, in, []Judgment{
		aiJudgment("go-01", EvidenceDrill, LevelBasicConfirmed, 0.9, day10),
	}, DefaultRules())

	if in["go-01"].VerifiedLevel != LevelNone {
		t.Errorf("入力の states が書き換えられた: VerifiedLevel = %d", in["go-01"].VerifiedLevel)
	}
	if got.States["go-01"].VerifiedLevel != LevelBasicConfirmed {
		t.Errorf("返り値の VerifiedLevel = %d, want %d", got.States["go-01"].VerifiedLevel, LevelBasicConfirmed)
	}
}

// 棄却した判定（V2・V3）は Results に入れない。
// 入れてしまうと、呼び出し側が棄却を確かめずに保存へ流し、範囲外のレベルが
// DB の制約違反を起こして同じログの正常な判定まで巻き戻る（2026-09-23 に PR #29 のレビューで判明）。
func TestApplyJudgments_棄却した判定はResultsに入れない(t *testing.T) {
	rm := testRoadmap()
	states := map[ItemKey]ItemState{}

	got := ApplyJudgments(rm, states, []Judgment{
		// V2: レベルが 0〜5 の外
		aiJudgment("go-01", EvidenceDrill, 9, 0.9, day10),
		// V3: 許可リストに無い根拠
		aiJudgment("go-01", "vibes", 1, 0.9, day10),
		// これだけが受理される
		aiJudgment("go-02", EvidenceDrill, 1, 0.9, day10),
	}, DefaultRules())

	if len(got.Results) != 1 {
		t.Fatalf("Results = %d 件, want 1 件（棄却分は含めない）", len(got.Results))
	}
	if got.Results[0].Judgment.ItemKey != "go-02" {
		t.Errorf("残ったのが %q（go-02 のはず）", got.Results[0].Judgment.ItemKey)
	}
	// 捨てた事実は Violations に残る（握りつぶさない）
	gotCodes := codes(got.Violations)
	for _, want := range []ViolationCode{ViolationLevelOutOfRange, ViolationUnknownEvidence} {
		if !slices.Contains(gotCodes, want) {
			t.Errorf("%s が記録されていない: %v", want, gotCodes)
		}
	}
	// 棄却された項目のレベルは動かず、正常な判定だけが生きている
	if _, ok := got.States["go-01"]; ok {
		t.Error("棄却したのに go-01 の状態が作られている")
	}
	if got.States["go-02"].VerifiedLevel != LevelBasicConfirmed {
		t.Errorf("go-02 のレベル = %d", got.States["go-02"].VerifiedLevel)
	}
}

// 呼び出し側（CLI）は、違反と適用結果が「渡した並びの何件目か」を state.json に書く（SPEC.md §3.4）。
// 棄却・保留・切り詰めのどれでも、元の位置を指していることを確かめる。
func TestApplyJudgments_違反と適用結果に並びの位置が付く(t *testing.T) {
	rm := testRoadmap()
	rules := DefaultRules()
	rules.MaxItemsPerLog = 4

	got := ApplyJudgments(rm, nil, []Judgment{
		aiJudgment("go-99", EvidenceDrill, 1, 0.9, day10),          // 0: V1 棄却（上限の数に入らない）
		aiJudgment("go-01", EvidenceDrill, 9, 0.9, day10),          // 1: V2 棄却
		aiJudgment("go-01", EvidenceDrill, 2, 0.9, day10),          // 2: V5 切り詰めて適用
		aiJudgment("go-02", EvidenceImplementation, 3, 0.4, day10), // 3: V7 保留
		aiJudgment("go-02", EvidenceDrill, 1, 0.9, day10),          // 4: 適用（違反なし）
		aiJudgment("go-01", EvidenceDrill, 1, 0.9, day10),          // 5: V8 棄却（受理済み 4 件で上限）
	}, rules)

	type at struct {
		code  ViolationCode
		index int
	}
	var gotViolations []at
	for _, v := range got.Violations {
		gotViolations = append(gotViolations, at{v.Code, v.Index})
	}
	wantViolations := []at{
		{ViolationUnknownItem, 0},
		{ViolationLevelOutOfRange, 1},
		{ViolationEvidenceTooWeak, 2},
		{ViolationLowConfidence, 3},
		{ViolationTooManyItems, 5},
	}
	if !slices.Equal(gotViolations, wantViolations) {
		t.Errorf("Violations = %v, want %v", gotViolations, wantViolations)
	}

	var gotResults []int
	for _, r := range got.Results {
		gotResults = append(gotResults, r.Index)
	}
	// 棄却（0・1・5）は Results に入らない。保留（3）は入る
	if want := []int{2, 3, 4}; !slices.Equal(gotResults, want) {
		t.Errorf("Results の位置 = %v, want %v", gotResults, want)
	}
}

// 単体の ApplyJudgment では、棄却したことが Rejected で分かる。
func TestApplyJudgment_棄却にはRejectedが立つ(t *testing.T) {
	tests := []struct {
		name string
		j    Judgment
	}{
		{name: "レベルが範囲外（V2）", j: aiJudgment("go-01", EvidenceDrill, 9, 0.9, day10)},
		{name: "根拠が許可リストに無い（V3）", j: aiJudgment("go-01", "vibes", 1, 0.9, day10)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyJudgment(ItemState{ItemKey: "go-01"}, tt.j, DefaultRules())
			if !got.Rejected {
				t.Error("Rejected が立っていない")
			}
			if got.Changed {
				t.Error("棄却したのに状態が変わっている")
			}
		})
	}
}

// EvidenceTypes は許可リスト（SPEC.md §4.4）を指示書と出力スキーマへ渡すための一覧。
// 並びが揺れると指示書の文面が毎回変わるため、並びまで固定する。
func TestEvidenceTypes(t *testing.T) {
	got := EvidenceTypes()

	t.Run("許可リストの8種類を返す", func(t *testing.T) {
		if len(got) != 8 {
			t.Fatalf("8 件のはずが %d 件だった: %v", len(got), got)
		}
		for _, ev := range got {
			if !ev.Valid() {
				t.Errorf("許可リストにない種類が混ざっている: %q", ev)
			}
		}
	})

	t.Run("印を付けられる種類が上限の低い順に並び、印を付けない種類が最後に来る", func(t *testing.T) {
		var last Level
		seenNonMarking := false
		for _, ev := range got {
			_, top, marks := ev.Ladder()
			if !marks {
				seenNonMarking = true
				continue
			}
			if seenNonMarking {
				t.Fatalf("印を付けない種類のあとに %q が来ている", ev)
			}
			if top < last {
				t.Fatalf("%q の上限 %d が直前の %d より低い", ev, top, last)
			}
			last = top
		}
	})

	t.Run("梯子の範囲は SPEC.md §1.1 の表のとおり", func(t *testing.T) {
		want := map[EvidenceType][2]Level{
			EvidenceDrill:                 {1, 1},
			EvidenceSelfExplanation:       {1, 2},
			EvidenceImplementation:        {3, 3},
			EvidenceUnaidedImplementation: {3, 4},
			EvidenceCrossContext:          {3, 5},
		}
		for ev, w := range want {
			base, top, marks := ev.Ladder()
			if !marks || base != w[0] || top != w[1] {
				t.Errorf("%q の梯子 = %d〜%d（marks=%v）, want %d〜%d", ev, base, top, marks, w[0], w[1])
			}
		}
		for _, ev := range []EvidenceType{EvidenceLearningActivity, EvidenceExplainedTo, EvidenceSelfReport} {
			if _, _, marks := ev.Ladder(); marks {
				t.Errorf("%q は印を付けない種類のはず", ev)
			}
		}
	})

	t.Run("呼び出し側が書き換えても次の呼び出しに影響しない", func(t *testing.T) {
		got[0] = "broken"
		if EvidenceTypes()[0] == "broken" {
			t.Error("内部の並びが書き換えられた")
		}
	})
}
