package llm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/config"
	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
	"github.com/n-yoshida-dev/skill-matrix/internal/roadmap"
)

// このテストは「stub がどんな判定を返すか」と「それが domain の検証にどうつながるか」を固定する。

var logDay = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

// sampleRequest は backend/testdata のダミーのロードマップから判定の依頼を組み立てる。
// 項目は go-01〜05 / react-01〜04 / infra-01〜02 の順に並んでいる。
func sampleRequest(t *testing.T) JudgmentRequest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "roadmap-sample.json"))
	if err != nil {
		t.Fatalf("サンプルのロードマップを読めません: %v", err)
	}
	doc, res := roadmap.ParseAndValidate(data)
	if doc == nil || !res.OK() {
		t.Fatalf("サンプルのロードマップが検査を通りません: %+v", res.Errors())
	}
	return JudgmentRequest{
		Roadmap: doc.ToDomain(),
		Levels:  doc.Levels,
		States:  map[domain.ItemKey]domain.ItemState{},
		LogBody: "ダミーの学習ログ。Go の基本構文の確認問題に答えた。",
	}
}

// itemKeys は判定から項目の key だけを取り出す。
func itemKeys(js []ProposedJudgment) []string {
	out := make([]string, 0, len(js))
	for _, j := range js {
		out = append(out, j.ItemKey)
	}
	return out
}

// violationCodes は違反の識別子だけを取り出す。
func violationCodes(vs []domain.Violation) []domain.ViolationCode {
	out := make([]domain.ViolationCode, 0, len(vs))
	for _, v := range vs {
		out = append(out, v.Code)
	}
	return out
}

func TestNew(t *testing.T) {
	t.Run("stub を選ぶと LLM を呼ばない実装が返る", func(t *testing.T) {
		p, err := New(config.LLMConfig{Provider: config.ProviderStub})
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if _, ok := p.(*Stub); !ok {
			t.Fatalf("*Stub が返るはずが %T だった", p)
		}
	})

	t.Run("anthropic を選ぶと Claude API を呼ぶ実装が返る", func(t *testing.T) {
		p, err := New(config.LLMConfig{
			Provider: config.ProviderAnthropic,
			APIKey:   "dummy",
			Model:    "claude-sonnet-5",
		})
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if _, ok := p.(*Anthropic); !ok {
			t.Fatalf("*Anthropic が返るはずが %T だった", p)
		}
	})

	t.Run("anthropic で API キーが無ければ、黙って stub に落とさずエラーにする", func(t *testing.T) {
		p, err := New(config.LLMConfig{Provider: config.ProviderAnthropic, Model: "claude-sonnet-5"})
		if err == nil {
			t.Fatal("エラーになるはずが通った")
		}
		if p != nil {
			t.Fatalf("エラーのときは Provider を返さないはずが %T だった", p)
		}
	})

	t.Run("知らないプロバイダはエラーにする", func(t *testing.T) {
		if _, err := New(config.LLMConfig{Provider: "openai"}); err == nil {
			t.Fatal("エラーになるはずが通った")
		}
	})
}

func TestStubJudge_Default(t *testing.T) {
	ctx := context.Background()

	t.Run("未着手のロードマップでは先頭3項目にレベル1を提案する", func(t *testing.T) {
		res, err := NewStub().Judge(ctx, sampleRequest(t))
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if got, want := itemKeys(res.Output.Judgments), []string{"go-01", "go-02", "go-03"}; !slices.Equal(got, want) {
			t.Errorf("項目 = %v, want %v", got, want)
		}
		for _, j := range res.Output.Judgments {
			if j.ProposedLevel != 1 || j.EvidenceType != string(domain.EvidenceDrill) || j.Confidence == nil || *j.Confidence != stubConfidence {
				t.Errorf("%s: level=%d evidence=%s confidence=%v（レベル1・drill・%g のはず）",
					j.ItemKey, j.ProposedLevel, j.EvidenceType, j.Confidence, stubConfidence)
			}
		}
		if len(res.Output.Rejected) != 0 {
			t.Errorf("stub 自身の出力が形の検査で弾かれている: %+v", res.Output.Rejected)
		}
		if res.Model != StubModel {
			t.Errorf("Model = %q, want %q", res.Model, StubModel)
		}
		if res.Usage != (Usage{}) {
			t.Errorf("stub はトークンを消費しないはずが %+v だった", res.Usage)
		}
	})

	t.Run("同じ入力には必ず同じ出力を返す", func(t *testing.T) {
		req := sampleRequest(t)
		first, err := NewStub().Judge(ctx, req)
		if err != nil {
			t.Fatalf("1回目がエラーになった: %v", err)
		}
		for i := 0; i < 5; i++ {
			again, err := NewStub().Judge(ctx, req)
			if err != nil {
				t.Fatalf("%d 回目がエラーになった: %v", i+2, err)
			}
			if string(again.Raw) != string(first.Raw) {
				t.Fatalf("%d 回目の出力が変わった:\n1回目: %s\n今回:   %s", i+2, first.Raw, again.Raw)
			}
		}
	})

	t.Run("現在のレベルに応じて次のレベルと、それに届く根拠を選ぶ", func(t *testing.T) {
		req := sampleRequest(t)
		req.States = map[domain.ItemKey]domain.ItemState{
			"go-01": {ItemKey: "go-01", VerifiedLevel: domain.MaxLevel},        // 最大レベルなので選ばれない
			"go-02": {ItemKey: "go-02", VerifiedLevel: domain.LevelCanExplain}, // 2 → 3 は「実装」が要る
		}
		res, err := NewStub().Judge(ctx, req)
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if got, want := itemKeys(res.Output.Judgments), []string{"go-02", "go-03", "go-04"}; !slices.Equal(got, want) {
			t.Fatalf("項目 = %v, want %v", got, want)
		}
		first := res.Output.Judgments[0]
		if first.ProposedLevel != 3 || first.EvidenceType != string(domain.EvidenceImplementation) {
			t.Errorf("go-02: level=%d evidence=%s（レベル3・implementation のはず）",
				first.ProposedLevel, first.EvidenceType)
		}
	})

	t.Run("全項目が最大レベルなら判定は0件で、エラーにはしない", func(t *testing.T) {
		req := sampleRequest(t)
		for _, it := range req.Roadmap.AllItems() {
			req.States[it.Key] = domain.ItemState{ItemKey: it.Key, VerifiedLevel: domain.MaxLevel}
		}
		res, err := NewStub().Judge(ctx, req)
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if len(res.Output.Judgments) != 0 {
			t.Errorf("判定は0件のはずが %v だった", itemKeys(res.Output.Judgments))
		}
	})

	t.Run("stub の判定は domain の検証を違反なしで通り、レベルが上がる", func(t *testing.T) {
		req := sampleRequest(t)
		res, err := NewStub().Judge(ctx, req)
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		batch := domain.ApplyJudgments(req.Roadmap, req.States, res.Output.DomainJudgments(domain.SourceAI, logDay), domain.DefaultRules())
		if len(batch.Violations) != 0 {
			t.Fatalf("違反が出た: %+v", batch.Violations)
		}
		for _, key := range []domain.ItemKey{"go-01", "go-02", "go-03"} {
			st := batch.States[key]
			if st.VerifiedLevel != domain.LevelBasicConfirmed || !st.LastEvidenceAt.Equal(logDay) {
				t.Errorf("%s: level=%d lastEvidenceAt=%v（レベル1・%v のはず）", key, st.VerifiedLevel, st.LastEvidenceAt, logDay)
			}
		}
	})
}

func TestStubJudge_InvalidRequest(t *testing.T) {
	ctx := context.Background()

	t.Run("本文が空の依頼は断る", func(t *testing.T) {
		req := sampleRequest(t)
		req.LogBody = "  \n"
		if _, err := NewStub().Judge(ctx, req); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("ErrInvalidRequest のはずが %v だった", err)
		}
	})

	t.Run("項目の無いロードマップの依頼は断る", func(t *testing.T) {
		req := sampleRequest(t)
		req.Roadmap = domain.Roadmap{Name: "空"}
		if _, err := NewStub().Judge(ctx, req); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("ErrInvalidRequest のはずが %v だった", err)
		}
	})

	t.Run("中断済みのコンテキストでは判定しない", func(t *testing.T) {
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := NewStub().Judge(cancelled, sampleRequest(t)); !errors.Is(err, context.Canceled) {
			t.Fatalf("context.Canceled のはずが %v だった", err)
		}
	})
}

func TestStubJudge_FixedOutput(t *testing.T) {
	ctx := context.Background()

	t.Run("行儀の悪い LLM を演じさせると、domain の検証が弾いて記録する", func(t *testing.T) {
		// 1件目: 実在しない項目（V1）／2件目: レベル 9（V2）／3件目: 未着手から実装の根拠でレベル3（印 [3] が付き、表示レベルは 0 のまま。V4 は廃止）
		raw := []byte(`{
			"judgments": [
				{"itemKey": "rust-01", "proposedLevel": 1, "evidenceType": "drill", "evidenceRefs": ["log:x.md"], "rationale": "捏造された項目", "confidence": 0.9},
				{"itemKey": "go-01", "proposedLevel": 9, "evidenceType": "drill", "evidenceRefs": ["log:x.md"], "rationale": "範囲外のレベル", "confidence": 0.9},
				{"itemKey": "go-02", "proposedLevel": 3, "evidenceType": "implementation", "evidenceRefs": ["log:x.md"], "rationale": "実装の根拠", "confidence": 0.9}
			],
			"unmatched": ["Rust の所有権について読んだ"]
		}`)
		req := sampleRequest(t)
		res, err := NewStubWithOutput(raw).Judge(ctx, req)
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if got, want := res.Output.Unmatched, []string{"Rust の所有権について読んだ"}; !slices.Equal(got, want) {
			t.Errorf("Unmatched = %v, want %v", got, want)
		}

		batch := domain.ApplyJudgments(req.Roadmap, req.States, res.Output.DomainJudgments(domain.SourceAI, logDay), domain.DefaultRules())
		want := []domain.ViolationCode{
			domain.ViolationUnknownItem, domain.ViolationLevelOutOfRange,
		}
		if got := violationCodes(batch.Violations); !slices.Equal(got, want) {
			t.Errorf("違反 = %v, want %v", got, want)
		}
		if st := batch.States["go-02"]; st.VerifiedLevel != domain.LevelNone || !slices.Equal(st.EvidencedLevels(), []domain.Level{domain.LevelGuidedImpl}) {
			t.Errorf("go-02 は印 [3]・表示レベル 0 のはずが evidenced=%v verified=%d だった", st.EvidencedLevels(), st.VerifiedLevel)
		}
	})

	t.Run("壊れた JSON を返す LLM はエラーになる", func(t *testing.T) {
		_, err := NewStubWithOutput([]byte(`{"judgments": [`)).Judge(ctx, sampleRequest(t))
		if !errors.Is(err, ErrInvalidOutput) {
			t.Fatalf("ErrInvalidOutput のはずが %v だった", err)
		}
	})

	t.Run("渡した JSON を後から書き換えても stub の出力は変わらない", func(t *testing.T) {
		raw := []byte(`{"judgments": [], "unmatched": []}`)
		stub := NewStubWithOutput(raw)
		raw[0] = 'X'
		if _, err := stub.Judge(ctx, sampleRequest(t)); err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
	})
}
