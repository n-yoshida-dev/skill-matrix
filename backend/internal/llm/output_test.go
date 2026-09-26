package llm

import (
	"errors"
	"strings"
	"testing"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
)

// 形の検査そのもののテストは internal/judgment/parse_test.go にある。
// ここでは入口（ParseOutput / ParseOutputFor）が、棚上げ中の呼び出し側（stub・Claude API クライアント・ワーカー）の
// 期待する形でエラーを返すことだけを固定する。

func TestParseOutput_形の検査をjudgmentに任せる(t *testing.T) {
	raw := []byte(`{"judgments": [
		{"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "drill", "evidenceRefs": ["log:a.md"], "rationale": "x", "confidence": 0.9},
		{"itemKey": "go-02", "rationale": "欄が足りない"}
	]}`)
	out, err := ParseOutput(raw)
	if err != nil {
		t.Fatalf("エラーになった: %v", err)
	}
	if len(out.Judgments) != 1 || out.Judgments[0].ItemKey != "go-01" {
		t.Errorf("Judgments = %+v", out.Judgments)
	}
	if len(out.Rejected) != 1 || out.Rejected[0].Index != 1 {
		t.Errorf("Rejected = %+v", out.Rejected)
	}
}

func TestParseOutputFor_SourceをJudgmentへ渡す(t *testing.T) {
	// manual の判定に confidence があれば弾かれる＝ source が judgment.Parse まで届いている
	raw := []byte(`{"judgments": [{"itemKey": "go-01", "proposedLevel": 1, "evidenceType": "drill", "evidenceRefs": ["log:a.md"], "rationale": "x", "confidence": 0.9}]}`)
	out, err := ParseOutputFor(raw, domain.SourceManual)
	if err != nil {
		t.Fatalf("エラーになった: %v", err)
	}
	if len(out.Rejected) != 1 || !strings.Contains(out.Rejected[0].Reason, "confidence は書けません") {
		t.Errorf("Rejected = %+v", out.Rejected)
	}
}

func TestParseOutput_外枠の崩れは生の出力つきのエラーにする(t *testing.T) {
	raw := []byte(`判定できませんでした`)
	_, err := ParseOutput(raw)
	if !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("ErrInvalidOutput のはずが %v だった", err)
	}
	var outErr *InvalidOutputError
	if !errors.As(err, &outErr) {
		t.Fatalf("*InvalidOutputError を取り出せない: %v", err)
	}
	if string(outErr.Raw) != string(raw) {
		t.Errorf("Raw = %q（生の出力がそのまま残るはず）", outErr.Raw)
	}
	if !strings.Contains(outErr.Reason, "JSON として解釈できません") {
		t.Errorf("Reason = %q", outErr.Reason)
	}
}
