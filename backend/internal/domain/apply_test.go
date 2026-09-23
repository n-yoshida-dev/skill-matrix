package domain

import (
	"slices"
	"testing"
	"time"
)

// このテストは「LLM がこう言ってきたら、こう扱う」を具体例で固定するもの。
// 検証ルールの意図を読み取りたいときは SPEC.md §4.5 より先にここを読むとよい。

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

func TestApplyJudgment(t *testing.T) {
	tests := []struct {
		name string
		cur  ItemState
		j    Judgment

		wantLevel       Level
		wantPreState    PreState
		wantNeedsReview bool
		wantEvidenceAt  time.Time
		wantDeferred    bool
		wantChanged     bool
		wantViolations  []ViolationCode
	}{
		{
			name: "未着手からドリルの根拠でレベル1へ上がる",
			cur:  ItemState{ItemKey: "go-01"},
			j: Judgment{
				ItemKey: "go-01", ProposedLevel: LevelBasicConfirmed,
				EvidenceType: EvidenceDrill, Confidence: 0.9, OccurredAt: day10,
			},
			wantLevel: LevelBasicConfirmed, wantEvidenceAt: day10, wantChanged: true,
		},
		{
			name: "V2 レベルが範囲外なら丸ごと棄却する",
			cur:  ItemState{ItemKey: "go-01"},
			j: Judgment{
				ItemKey: "go-01", ProposedLevel: MaxLevel + 1,
				EvidenceType: EvidenceDrill, Confidence: 0.9, OccurredAt: day10,
			},
			wantViolations: []ViolationCode{ViolationLevelOutOfRange},
		},
		{
			name: "V2 負のレベルも棄却する",
			cur:  ItemState{ItemKey: "go-01"},
			j: Judgment{
				ItemKey: "go-01", ProposedLevel: -1,
				EvidenceType: EvidenceDrill, Confidence: 0.9, OccurredAt: day10,
			},
			wantViolations: []ViolationCode{ViolationLevelOutOfRange},
		},
		{
			name: "V3 許可リストに無い根拠は棄却する",
			cur:  ItemState{ItemKey: "go-01"},
			j: Judgment{
				ItemKey: "go-01", ProposedLevel: LevelBasicConfirmed,
				EvidenceType: "vibes", Confidence: 0.9, OccurredAt: day10,
			},
			wantViolations: []ViolationCode{ViolationUnknownEvidence},
		},
		{
			name: "V4 未着手からレベル4への一気飛びは1段に切り詰める",
			cur:  ItemState{ItemKey: "go-01"},
			j: Judgment{
				ItemKey: "go-01", ProposedLevel: LevelIndependentImpl,
				EvidenceType: EvidenceCrossContext, Confidence: 0.9, OccurredAt: day10,
			},
			wantLevel: LevelBasicConfirmed, wantEvidenceAt: day10, wantChanged: true,
			wantViolations: []ViolationCode{ViolationLevelJump},
		},
		{
			name: "V5 ドリルの根拠でレベル3は主張できないので上限まで下げる",
			cur:  ItemState{ItemKey: "go-01"},
			j: Judgment{
				ItemKey: "go-01", ProposedLevel: LevelGuidedImpl,
				EvidenceType: EvidenceDrill, Confidence: 0.9, OccurredAt: day10,
			},
			wantLevel: LevelBasicConfirmed, wantEvidenceAt: day10, wantChanged: true,
			wantViolations: []ViolationCode{ViolationEvidenceTooWeak},
		},
		{
			name: "V5 と V4 が両方はたらく",
			cur:  ItemState{ItemKey: "go-01"},
			j: Judgment{
				ItemKey: "go-01", ProposedLevel: LevelCrossContext,
				EvidenceType: EvidenceSelfExplanation, Confidence: 0.9, OccurredAt: day10,
			},
			wantLevel: LevelBasicConfirmed, wantEvidenceAt: day10, wantChanged: true,
			wantViolations: []ViolationCode{ViolationEvidenceTooWeak, ViolationLevelJump},
		},
		{
			name: "V6 降格の提案ではレベルを下げず要再確認にする",
			cur:  ItemState{ItemKey: "go-01", Level: LevelGuidedImpl, LastEvidenceAt: day01},
			j: Judgment{
				ItemKey: "go-01", ProposedLevel: LevelBasicConfirmed,
				EvidenceType: EvidenceSelfExplanation, Confidence: 0.9, OccurredAt: day10,
			},
			wantLevel: LevelGuidedImpl, wantNeedsReview: true,
			wantEvidenceAt: day01, wantChanged: true,
			wantViolations: []ViolationCode{ViolationDowngrade},
		},
		{
			name: "V7 確信度が低いものは適用せず保留にする",
			cur:  ItemState{ItemKey: "go-01"},
			j: Judgment{
				ItemKey: "go-01", ProposedLevel: LevelBasicConfirmed,
				EvidenceType: EvidenceDrill, Confidence: 0.3, OccurredAt: day10,
			},
			wantDeferred:   true,
			wantViolations: []ViolationCode{ViolationLowConfidence},
		},
		{
			name: "説明を受けただけではレベルが上がらず、鮮度も更新されない",
			cur:  ItemState{ItemKey: "go-01", Level: LevelBasicConfirmed, LastEvidenceAt: day01},
			j: Judgment{
				ItemKey: "go-01", ProposedLevel: LevelGuidedImpl,
				EvidenceType: EvidenceExplainedTo, Confidence: 0.9, OccurredAt: day10,
			},
			wantLevel: LevelBasicConfirmed, wantPreState: PreStateExplainedOnly,
			wantEvidenceAt: day01, wantChanged: true,
		},
		{
			name: "自己申告もレベルを動かさず印だけ立てる",
			cur:  ItemState{ItemKey: "go-01"},
			j: Judgment{
				ItemKey: "go-01", ProposedLevel: LevelIndependentImpl,
				EvidenceType: EvidenceSelfReport, Confidence: 0.9, OccurredAt: day10,
			},
			wantLevel: LevelNone, wantPreState: PreStateSelfReported, wantChanged: true,
		},
		{
			name: "過去のログを後から投稿しても鮮度は巻き戻らない",
			cur:  ItemState{ItemKey: "go-01", Level: LevelBasicConfirmed, LastEvidenceAt: day10},
			j: Judgment{
				ItemKey: "go-01", ProposedLevel: LevelCanExplain,
				EvidenceType: EvidenceSelfExplanation, Confidence: 0.9, OccurredAt: day01,
			},
			wantLevel: LevelCanExplain, wantEvidenceAt: day10, wantChanged: true,
		},
		{
			name: "高いレベルに弱い根拠が付いても鮮度は更新しない",
			cur:  ItemState{ItemKey: "go-01", Level: LevelGuidedImpl, LastEvidenceAt: day01},
			j: Judgment{
				ItemKey: "go-01", ProposedLevel: LevelGuidedImpl,
				EvidenceType: EvidenceDrill, Confidence: 0.9, OccurredAt: day10,
			},
			wantLevel: LevelGuidedImpl, wantEvidenceAt: day01, wantChanged: false,
			wantViolations: []ViolationCode{ViolationEvidenceTooWeak},
		},
		{
			name: "要再確認は十分な根拠が付けば解除される",
			cur: ItemState{
				ItemKey: "go-01", Level: LevelCanExplain,
				NeedsReview: true, LastEvidenceAt: day01,
			},
			j: Judgment{
				ItemKey: "go-01", ProposedLevel: LevelGuidedImpl,
				EvidenceType: EvidenceImplementation, Confidence: 0.9, OccurredAt: day10,
			},
			wantLevel: LevelGuidedImpl, wantNeedsReview: false,
			wantEvidenceAt: day10, wantChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyJudgment(tt.cur, tt.j, DefaultRules())

			if got.State.Level != tt.wantLevel {
				t.Errorf("Level = %d, want %d", got.State.Level, tt.wantLevel)
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

// 保留になった判定は状態を一切変えない。承認されるまで反映しないため。
func TestApplyJudgment_保留は状態を変えない(t *testing.T) {
	cur := ItemState{ItemKey: "go-01", Level: LevelCanExplain, LastEvidenceAt: day01}
	got := ApplyJudgment(cur, Judgment{
		ItemKey: "go-01", ProposedLevel: LevelGuidedImpl,
		EvidenceType: EvidenceImplementation, Confidence: 0.1, OccurredAt: day10,
	}, DefaultRules())

	if !got.Deferred {
		t.Fatal("Deferred = false, want true")
	}
	if got.State != cur {
		t.Errorf("State = %+v, want 変更なし %+v", got.State, cur)
	}
	// 保留でも「何が適用されるはずだったか」は記録に残す
	if len(got.Violations) != 1 || got.Violations[0].Applied != LevelGuidedImpl {
		t.Errorf("Violations = %+v, want Applied=%d の記録", got.Violations, LevelGuidedImpl)
	}
}

// Rules のゼロ値を渡しても検証が無効化されないこと。
func TestApplyJudgment_ゼロ値のRulesは既定値で埋まる(t *testing.T) {
	got := ApplyJudgment(ItemState{ItemKey: "go-01"}, Judgment{
		ItemKey: "go-01", ProposedLevel: LevelCrossContext,
		EvidenceType: EvidenceCrossContext, Confidence: 0.9, OccurredAt: day10,
	}, Rules{})

	if got.State.Level != LevelBasicConfirmed {
		t.Errorf("Level = %d, want %d（V4 が既定値ではたらくこと）",
			got.State.Level, LevelBasicConfirmed)
	}
}

func TestApplyJudgments_V1_実在しない項目は棄却する(t *testing.T) {
	rm := testRoadmap()
	got := ApplyJudgments(rm, nil, []Judgment{
		{ItemKey: "go-99", ProposedLevel: LevelBasicConfirmed,
			EvidenceType: EvidenceDrill, Confidence: 0.9, OccurredAt: day10},
		{ItemKey: "go-01", ProposedLevel: LevelBasicConfirmed,
			EvidenceType: EvidenceDrill, Confidence: 0.9, OccurredAt: day10},
	}, DefaultRules())

	if _, ok := got.States["go-99"]; ok {
		t.Error("実在しない項目の状態が作られている")
	}
	if got.States["go-01"].Level != LevelBasicConfirmed {
		t.Errorf("go-01 の Level = %d, want %d", got.States["go-01"].Level, LevelBasicConfirmed)
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
		{ItemKey: "go-01", ProposedLevel: LevelBasicConfirmed,
			EvidenceType: EvidenceDrill, Confidence: 0.9, OccurredAt: day10},
		{ItemKey: "go-02", ProposedLevel: LevelBasicConfirmed,
			EvidenceType: EvidenceDrill, Confidence: 0.9, OccurredAt: day10},
	}, rules)

	if got.States["go-01"].Level != LevelBasicConfirmed {
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
		{ItemKey: "go-01", ProposedLevel: LevelCanExplain,
			EvidenceType: EvidenceSelfExplanation, Confidence: 0.9, OccurredAt: day10},
		{ItemKey: "go-02", ProposedLevel: LevelBasicConfirmed,
			EvidenceType: EvidenceDrill, Confidence: 0.9, OccurredAt: day10},
	}, DefaultRules())

	// go-01 は未着手からなので V4 で1段に切り詰められる
	if got.States["go-01"].Level != LevelBasicConfirmed {
		t.Errorf("go-01 の Level = %d, want %d", got.States["go-01"].Level, LevelBasicConfirmed)
	}
	if got.States["go-02"].Level != LevelBasicConfirmed {
		t.Errorf("go-02 の Level = %d, want %d", got.States["go-02"].Level, LevelBasicConfirmed)
	}
	if len(got.Results) != 2 {
		t.Errorf("Results = %d 件, want 2 件", len(got.Results))
	}
}

// 入力として渡した states を書き換えないこと（呼び出し側の値が壊れないように）。
func TestApplyJudgments_入力のstatesを破壊しない(t *testing.T) {
	rm := testRoadmap()
	in := map[ItemKey]ItemState{
		"go-01": {ItemKey: "go-01", Level: LevelNone},
	}

	got := ApplyJudgments(rm, in, []Judgment{
		{ItemKey: "go-01", ProposedLevel: LevelBasicConfirmed,
			EvidenceType: EvidenceDrill, Confidence: 0.9, OccurredAt: day10},
	}, DefaultRules())

	if in["go-01"].Level != LevelNone {
		t.Errorf("入力の states が書き換えられた: Level = %d", in["go-01"].Level)
	}
	if got.States["go-01"].Level != LevelBasicConfirmed {
		t.Errorf("返り値の Level = %d, want %d", got.States["go-01"].Level, LevelBasicConfirmed)
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
		{ItemKey: "go-01", ProposedLevel: 9, EvidenceType: EvidenceDrill, Confidence: 0.9, OccurredAt: day10},
		// V3: 許可リストに無い根拠
		{ItemKey: "go-01", ProposedLevel: 1, EvidenceType: "vibes", Confidence: 0.9, OccurredAt: day10},
		// これだけが受理される
		{ItemKey: "go-02", ProposedLevel: 1, EvidenceType: EvidenceDrill, Confidence: 0.9, OccurredAt: day10},
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
	if got.States["go-02"].Level != LevelBasicConfirmed {
		t.Errorf("go-02 のレベル = %d", got.States["go-02"].Level)
	}
}

// 単体の ApplyJudgment では、棄却したことが Rejected で分かる。
func TestApplyJudgment_棄却にはRejectedが立つ(t *testing.T) {
	tests := []struct {
		name string
		j    Judgment
	}{
		{name: "レベルが範囲外（V2）", j: Judgment{ItemKey: "go-01", ProposedLevel: 9, EvidenceType: EvidenceDrill, Confidence: 0.9}},
		{name: "根拠が許可リストに無い（V3）", j: Judgment{ItemKey: "go-01", ProposedLevel: 1, EvidenceType: "vibes", Confidence: 0.9}},
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

// EvidenceTypes は許可リスト（SPEC.md §4.4）をプロンプトと出力スキーマへ渡すための一覧。
// 並びが揺れるとプロンプトキャッシュが毎回外れるため、並びまで固定する。
func TestEvidenceTypes(t *testing.T) {
	got := EvidenceTypes()

	t.Run("許可リストの7種類を返す", func(t *testing.T) {
		if len(got) != 7 {
			t.Fatalf("7 件のはずが %d 件だった: %v", len(got), got)
		}
		for _, ev := range got {
			if !ev.Valid() {
				t.Errorf("許可リストにない種類が混ざっている: %q", ev)
			}
		}
	})

	t.Run("昇格できる種類が到達レベルの低い順に並び、昇格しない種類が最後に来る", func(t *testing.T) {
		var last Level
		seenNonPromoting := false
		for _, ev := range got {
			limit, promotes := ev.MaxLevel()
			if !promotes {
				seenNonPromoting = true
				continue
			}
			if seenNonPromoting {
				t.Fatalf("昇格しない種類のあとに %q が来ている", ev)
			}
			if limit < last {
				t.Fatalf("%q の上限 %d が直前の %d より低い", ev, limit, last)
			}
			last = limit
		}
	})

	t.Run("呼び出し側が書き換えても次の呼び出しに影響しない", func(t *testing.T) {
		got[0] = "broken"
		if EvidenceTypes()[0] == "broken" {
			t.Error("内部の並びが書き換えられた")
		}
	})
}
