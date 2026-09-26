package judgment

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
)

// このテストは「判定 JSON のどこまでを形の崩れとして弾くか」を固定する。
// 意味の検査（項目が実在するか・レベルが 0〜5 か・梯子の範囲か）はここでは見ない。domain の V1〜V8 が担当する。

// parseAI は source が ai（confidence 必須）として読む。AI の判定がいちばん多い形なので既定にする。
func parseAI(raw []byte) (Output, error) {
	return Parse(raw, domain.SourceAI)
}

func TestParse(t *testing.T) {
	t.Run("SPEC §4.3 の例をそのまま読める", func(t *testing.T) {
		raw := []byte(`{
			"judgments": [{
				"itemKey": "go-syntax-basics", "evidenceType": "self_explanation", "proposedLevel": 2,
				"occurredAt": "2026-09-28",
				"evidenceRefs": ["repo:n-yoshida-dev/study@a1b2c3d/go-react/logs/2026-09-28.md#L10-L30"],
				"rationale": "for/if/switch の挙動を自分の言葉で説明している", "confidence": 0.8
			}],
			"unmatched": ["どの項目にも対応しない記述"]
		}`)
		out, err := parseAI(raw)
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		conf := 0.8
		want := Proposed{
			ItemKey: "go-syntax-basics", ProposedLevel: 2, EvidenceType: "self_explanation",
			EvidenceRefs: []string{"repo:n-yoshida-dev/study@a1b2c3d/go-react/logs/2026-09-28.md#L10-L30"},
			Rationale:    "for/if/switch の挙動を自分の言葉で説明している", Confidence: &conf,
			OccurredAt: time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC),
		}
		if len(out.Judgments) != 1 || !reflect.DeepEqual(out.Judgments[0], want) {
			t.Errorf("Judgments = %+v, want [%+v]", out.Judgments, want)
		}
		if len(out.Unmatched) != 1 || len(out.Rejected) != 0 {
			t.Errorf("Unmatched = %v, Rejected = %+v", out.Unmatched, out.Rejected)
		}
	})

	t.Run("レベル0（不合格の報告）と確信度0は欠落ではなく正当な値として読む", func(t *testing.T) {
		raw := []byte(`{"judgments": [{"itemKey": "go-01", "proposedLevel": 0, "evidenceType": "drill", "evidenceRefs": ["log:learning-logs/x.md"], "rationale": "確認質問に答えられなかった", "confidence": 0}]}`)
		out, err := parseAI(raw)
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if len(out.Judgments) != 1 || len(out.Rejected) != 0 {
			t.Errorf("Judgments = %+v, Rejected = %+v", out.Judgments, out.Rejected)
		}
	})

	t.Run("occurredAt が無ければゼロ値（呼び出し側が loggedAt で補う）", func(t *testing.T) {
		raw := []byte(`{"judgments": [{"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "drill", "evidenceRefs": ["log:a.md"], "rationale": "x", "confidence": 0.9}]}`)
		out, err := parseAI(raw)
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if !out.Judgments[0].OccurredAt.IsZero() {
			t.Errorf("OccurredAt = %v, want ゼロ値", out.Judgments[0].OccurredAt)
		}
	})

	t.Run("範囲外のレベルや知らない根拠は形としては通す（弾くのは domain の役目）", func(t *testing.T) {
		raw := []byte(`{"judgments": [{"itemKey": "zzz", "proposedLevel": 9, "evidenceType": "guess", "evidenceRefs": ["cert:x"], "rationale": "x", "confidence": 0.9}]}`)
		out, err := parseAI(raw)
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if len(out.Judgments) != 1 || len(out.Rejected) != 0 {
			t.Errorf("Judgments = %+v, Rejected = %+v", out.Judgments, out.Rejected)
		}
	})

	t.Run("判定が0件でも正常", func(t *testing.T) {
		out, err := parseAI([]byte(`{"judgments": [], "unmatched": []}`))
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if len(out.Judgments) != 0 || len(out.Rejected) != 0 {
			t.Errorf("Judgments = %+v, Rejected = %+v", out.Judgments, out.Rejected)
		}
	})
}

func TestParse_SourceによるConfidenceの条件(t *testing.T) {
	const withConf = `{"judgments": [{"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "drill", "evidenceRefs": ["log:a.md"], "rationale": "x", "confidence": 0.9}]}`
	const withoutConf = `{"judgments": [{"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "drill", "evidenceRefs": ["log:a.md"], "rationale": "x"}]}`

	tests := []struct {
		name       string
		source     domain.JudgmentSource
		raw        string
		wantOK     bool
		wantReason string
	}{
		{"ai は confidence 必須", domain.SourceAI, withoutConf, false, "confidence"},
		{"ai は confidence があれば通る", domain.SourceAI, withConf, true, ""},
		{"manual に confidence は書けない", domain.SourceManual, withConf, false, "confidence は書けません"},
		{"manual は confidence 無しで通る", domain.SourceManual, withoutConf, true, ""},
		{"migration に confidence は書けない", domain.SourceMigration, withConf, false, "confidence は書けません"},
		{"migration は confidence 無しで通る", domain.SourceMigration, withoutConf, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Parse([]byte(tt.raw), tt.source)
			if err != nil {
				t.Fatalf("外枠のエラーになった: %v", err)
			}
			if tt.wantOK {
				if len(out.Judgments) != 1 || len(out.Rejected) != 0 {
					t.Fatalf("通るはずが Judgments = %+v, Rejected = %+v", out.Judgments, out.Rejected)
				}
				if tt.source != domain.SourceAI && out.Judgments[0].Confidence != nil {
					t.Errorf("Confidence = %v, want nil", *out.Judgments[0].Confidence)
				}
				return
			}
			if len(out.Rejected) != 1 || !strings.Contains(out.Rejected[0].Reason, tt.wantReason) {
				t.Fatalf("弾かれるはずが Judgments = %+v, Rejected = %+v", out.Judgments, out.Rejected)
			}
		})
	}
}

func TestDomainJudgments(t *testing.T) {
	logged := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	out, err := Parse([]byte(`{"judgments": [
		{"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "drill", "evidenceRefs": ["log:a.md"], "rationale": "x"},
		{"itemKey": "go-02", "proposedLevel": 3, "evidenceType": "implementation", "evidenceRefs": ["repo:a/b@c"], "rationale": "y", "occurredAt": "2026-09-01"}
	]}`), domain.SourceManual)
	if err != nil {
		t.Fatalf("エラーになった: %v", err)
	}
	js := out.DomainJudgments(domain.SourceManual, logged)
	if len(js) != 2 {
		t.Fatalf("判定 = %d 件, want 2", len(js))
	}
	if js[0].Source != domain.SourceManual || js[0].HasConfidence {
		t.Errorf("1 件目: Source = %q, HasConfidence = %v", js[0].Source, js[0].HasConfidence)
	}
	if !js[0].OccurredAt.Equal(logged) {
		t.Errorf("occurredAt が無い判定は loggedAt で補う: %v", js[0].OccurredAt)
	}
	if want := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC); !js[1].OccurredAt.Equal(want) {
		t.Errorf("occurredAt がある判定はそれを使う: %v", js[1].OccurredAt)
	}
	if want := []string{"repo:a/b@c"}; !reflect.DeepEqual(js[1].EvidenceRefs, want) {
		t.Errorf("EvidenceRefs = %v, want %v", js[1].EvidenceRefs, want)
	}
}

func TestParse_RejectsMalformedJudgment(t *testing.T) {
	// 形の崩れた1件だけを弾き、同じログの正常な判定は生かす
	const good = `{"itemKey": "go-02", "proposedLevel": 1, "evidenceType": "drill", "evidenceRefs": ["log:a.md"], "rationale": "正常な判定", "confidence": 0.9}`

	tests := []struct {
		name       string
		bad        string
		wantReason string
	}{
		{
			name:       "レベルが整数でない",
			bad:        `{"itemKey": "go-01", "proposedLevel": 1.5, "evidenceType": "drill", "evidenceRefs": ["log:a.md"], "rationale": "x", "confidence": 0.9}`,
			wantReason: "型が合いません",
		},
		{
			name:       "レベルが文字列",
			bad:        `{"itemKey": "go-01", "proposedLevel": "3", "evidenceType": "drill", "evidenceRefs": ["log:a.md"], "rationale": "x", "confidence": 0.9}`,
			wantReason: "型が合いません",
		},
		{
			name:       "要素がオブジェクトでない",
			bad:        `"go-01"`,
			wantReason: "型が合いません",
		},
		{
			name:       "定義にない欄がある（evidenceRef の打ち間違い）",
			bad:        `{"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "drill", "evidenceRef": "log:a.md", "evidenceRefs": ["log:a.md"], "rationale": "x", "confidence": 0.9}`,
			wantReason: "型が合いません",
		},
		{
			name:       "必須の欄が欠けている",
			bad:        `{"itemKey": "go-01", "rationale": "x"}`,
			wantReason: "proposedLevel, evidenceType, evidenceRefs, confidence",
		},
		{
			name:       "理由が空白だけ",
			bad:        `{"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "drill", "evidenceRefs": ["log:a.md"], "rationale": "  ", "confidence": 0.9}`,
			wantReason: "rationale が空",
		},
		{
			name:       "evidenceRefs が空の配列",
			bad:        `{"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "drill", "evidenceRefs": [], "rationale": "x", "confidence": 0.9}`,
			wantReason: "evidenceRefs が空",
		},
		{
			name:       "evidenceRefs の書式が違う（種別が無い）",
			bad:        `{"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "drill", "evidenceRefs": ["logs/a.md"], "rationale": "x", "confidence": 0.9}`,
			wantReason: "evidenceRefs の書式",
		},
		{
			name:       "evidenceRefs の書式が違う（識別子が空）",
			bad:        `{"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "drill", "evidenceRefs": ["repo:"], "rationale": "x", "confidence": 0.9}`,
			wantReason: "evidenceRefs の書式",
		},
		{
			name:       "確信度が1を超える",
			bad:        `{"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "drill", "evidenceRefs": ["log:a.md"], "rationale": "x", "confidence": 1.2}`,
			wantReason: "confidence が 0〜1 の範囲外",
		},
		{
			name:       "確信度が負",
			bad:        `{"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "drill", "evidenceRefs": ["log:a.md"], "rationale": "x", "confidence": -0.1}`,
			wantReason: "confidence が 0〜1 の範囲外",
		},
		{
			name:       "occurredAt の書式が違う",
			bad:        `{"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "drill", "evidenceRefs": ["log:a.md"], "rationale": "x", "confidence": 0.9, "occurredAt": "2026/09/01"}`,
			wantReason: "occurredAt は YYYY-MM-DD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := parseAI([]byte(`{"judgments": [` + tt.bad + `, ` + good + `]}`))
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

func TestParse_BrokenEnvelope(t *testing.T) {
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
			var envErr *EnvelopeError
			if _, err := parseAI([]byte(tt.raw)); !errors.As(err, &envErr) {
				t.Fatalf("*EnvelopeError のはずが %v だった", err)
			}
		})
	}
}
