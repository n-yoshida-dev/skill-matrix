package llm

import (
	"errors"
	"strings"
	"testing"
)

// このテストは「LLM の出力のどこまでを形の崩れとして弾くか」を固定する。
// 意味の検査（項目が実在するか・レベルが 0〜5 か）はここでは見ない。domain の V1〜V8 が担当する。

func TestParseOutput(t *testing.T) {
	t.Run("SPEC §4.3 の例をそのまま読める", func(t *testing.T) {
		raw := []byte(`{
			"judgments": [{
				"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "self_explanation",
				"rationale": "for/if/switch の挙動を自分の言葉で説明している", "confidence": 0.8
			}],
			"unmatched": ["どの項目にも対応しない記述"]
		}`)
		out, err := ParseOutput(raw)
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		want := ProposedJudgment{
			ItemKey: "go-01", ProposedLevel: 1, EvidenceType: "self_explanation",
			Rationale: "for/if/switch の挙動を自分の言葉で説明している", Confidence: 0.8,
		}
		if len(out.Judgments) != 1 || out.Judgments[0] != want {
			t.Errorf("Judgments = %+v, want [%+v]", out.Judgments, want)
		}
		if len(out.Unmatched) != 1 || len(out.Rejected) != 0 {
			t.Errorf("Unmatched = %v, Rejected = %+v", out.Unmatched, out.Rejected)
		}
	})

	t.Run("レベル0と確信度0は欠落ではなく正当な値として読む", func(t *testing.T) {
		raw := []byte(`{"judgments": [{"itemKey": "go-01", "proposedLevel": 0, "evidenceType": "explained_to", "rationale": "説明を受けただけ", "confidence": 0}]}`)
		out, err := ParseOutput(raw)
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if len(out.Judgments) != 1 || len(out.Rejected) != 0 {
			t.Errorf("Judgments = %+v, Rejected = %+v", out.Judgments, out.Rejected)
		}
	})

	t.Run("範囲外のレベルや知らない根拠は形としては通す（弾くのは domain の役目）", func(t *testing.T) {
		raw := []byte(`{"judgments": [{"itemKey": "zzz", "proposedLevel": 9, "evidenceType": "guess", "rationale": "x", "confidence": 0.9}]}`)
		out, err := ParseOutput(raw)
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if len(out.Judgments) != 1 || len(out.Rejected) != 0 {
			t.Errorf("Judgments = %+v, Rejected = %+v", out.Judgments, out.Rejected)
		}
	})

	t.Run("判定が0件でも正常", func(t *testing.T) {
		out, err := ParseOutput([]byte(`{"judgments": [], "unmatched": []}`))
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if len(out.Judgments) != 0 || len(out.Rejected) != 0 {
			t.Errorf("Judgments = %+v, Rejected = %+v", out.Judgments, out.Rejected)
		}
	})
}

func TestParseOutput_RejectsMalformedJudgment(t *testing.T) {
	// 形の崩れた1件だけを弾き、同じログの正常な判定は生かす
	const good = `{"itemKey": "go-02", "proposedLevel": 1, "evidenceType": "drill", "rationale": "正常な判定", "confidence": 0.9}`

	tests := []struct {
		name       string
		bad        string
		wantReason string
	}{
		{
			name:       "レベルが整数でない",
			bad:        `{"itemKey": "go-01", "proposedLevel": 1.5, "evidenceType": "drill", "rationale": "x", "confidence": 0.9}`,
			wantReason: "型が合いません",
		},
		{
			name:       "レベルが文字列",
			bad:        `{"itemKey": "go-01", "proposedLevel": "3", "evidenceType": "drill", "rationale": "x", "confidence": 0.9}`,
			wantReason: "型が合いません",
		},
		{
			name:       "要素がオブジェクトでない",
			bad:        `"go-01"`,
			wantReason: "型が合いません",
		},
		{
			name:       "必須の欄が欠けている",
			bad:        `{"itemKey": "go-01", "rationale": "x"}`,
			wantReason: "proposedLevel, evidenceType, confidence",
		},
		{
			name:       "理由が空白だけ",
			bad:        `{"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "drill", "rationale": "  ", "confidence": 0.9}`,
			wantReason: "rationale が空",
		},
		{
			name:       "確信度が1を超える",
			bad:        `{"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "drill", "rationale": "x", "confidence": 1.2}`,
			wantReason: "confidence が 0〜1 の範囲外",
		},
		{
			name:       "確信度が負",
			bad:        `{"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "drill", "rationale": "x", "confidence": -0.1}`,
			wantReason: "confidence が 0〜1 の範囲外",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := ParseOutput([]byte(`{"judgments": [` + tt.bad + `, ` + good + `]}`))
			if err != nil {
				t.Fatalf("1件の崩れで全体がエラーになった: %v", err)
			}
			if len(out.Judgments) != 1 || out.Judgments[0].ItemKey != "go-02" {
				t.Errorf("正常な判定が残っていない: %+v", out.Judgments)
			}
			if len(out.Rejected) != 1 {
				t.Fatalf("弾いた判定は1件のはずが %+v だった", out.Rejected)
			}
			rej := out.Rejected[0]
			if rej.Index != 0 {
				t.Errorf("Index = %d, want 0", rej.Index)
			}
			if !strings.Contains(rej.Reason, tt.wantReason) {
				t.Errorf("Reason = %q（%q を含むはず）", rej.Reason, tt.wantReason)
			}
			if string(rej.Raw) != tt.bad {
				t.Errorf("Raw = %s（弾いた要素そのものが残るはず）", rej.Raw)
			}
		})
	}
}

func TestParseOutput_BrokenEnvelope(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "JSON でない", raw: `判定できませんでした`},
		{name: "途中で切れている", raw: `{"judgments": [`},
		{name: "judgments が無い", raw: `{"unmatched": []}`},
		{name: "judgments が null", raw: `{"judgments": null}`},
		{name: "judgments が配列でない", raw: `{"judgments": {}}`},
		{name: "JSON の後ろに余計な文字がある", raw: `{"judgments": []} 以上です`},
		{name: "空", raw: ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseOutput([]byte(tt.raw)); !errors.Is(err, ErrInvalidOutput) {
				t.Fatalf("ErrInvalidOutput のはずが %v だった", err)
			}
		})
	}
}
