package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
	"github.com/n-yoshida-dev/skill-matrix/internal/llm"
	"github.com/n-yoshida-dev/skill-matrix/internal/store"
)

// このテストは DB も Claude API も使わない。
// 偽の Queue と、LLM を呼ばない Provider を差して、ワーカーの筋道だけを確かめる。
//
// 確かめたいのは「どんな失敗のとき、仕事をどう閉じるか」。
// 再試行してよい失敗だけが順番待ちへ戻り、それ以外は失敗で閉じること。
// そして**どの経路でも、AI が何と言ったかが記録に残ること**（握りつぶし禁止）。

var testNow = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

// fakeQueue は Queue の偽物。呼ばれた順番と中身を記録する。
type fakeQueue struct {
	job   *store.JudgmentJob
	input *store.JudgmentInput

	// claimErr / loadErr を立てるとその段階で失敗する。
	claimErr error
	loadErr  error
	// saveErr / applyErr は保存・反映の失敗を演じる。
	saveErr  error
	applyErr error

	calls     []string
	responses []store.LLMResponse
	applied   []domain.Applied
	failed    string
	requeued  string
}

func (q *fakeQueue) ClaimJudgmentJob(context.Context, time.Time) (*store.JudgmentJob, error) {
	q.calls = append(q.calls, "claim")
	if q.claimErr != nil {
		return nil, q.claimErr
	}
	return q.job, nil
}

func (q *fakeQueue) LoadJudgmentInput(context.Context, *store.JudgmentJob) (*store.JudgmentInput, error) {
	q.calls = append(q.calls, "load")
	if q.loadErr != nil {
		return nil, q.loadErr
	}
	return q.input, nil
}

func (q *fakeQueue) SaveLLMResponse(_ context.Context, r store.LLMResponse) error {
	q.calls = append(q.calls, "save")
	if q.saveErr != nil {
		return q.saveErr
	}
	q.responses = append(q.responses, r)
	return nil
}

func (q *fakeQueue) ApplyJudgmentResult(_ context.Context, _ *store.JudgmentJob, _ *store.JudgmentInput, applied []domain.Applied) error {
	q.calls = append(q.calls, "apply")
	if q.applyErr != nil {
		return q.applyErr
	}
	q.applied = applied
	return nil
}

func (q *fakeQueue) FinishJob(context.Context, string, time.Time) error {
	q.calls = append(q.calls, "finish")
	return nil
}

func (q *fakeQueue) FailJob(_ context.Context, _, errText string, _ time.Time) error {
	q.calls = append(q.calls, "fail")
	q.failed = errText
	return nil
}

func (q *fakeQueue) RequeueJob(_ context.Context, _, errText string, _ time.Time) error {
	q.calls = append(q.calls, "requeue")
	q.requeued = errText
	return nil
}

// failingProvider は必ず決まったエラーを返す Provider。
type failingProvider struct{ err error }

func (p failingProvider) Judge(context.Context, llm.JudgmentRequest) (*llm.JudgmentResult, error) {
	return nil, p.err
}

// testRoadmap は2項目だけの小さなロードマップ。
func testRoadmap() domain.Roadmap {
	return domain.Roadmap{
		Name: "テスト",
		Domains: []domain.Domain{{
			Key:  "go",
			Name: "Go",
			Items: []domain.Item{
				{Key: "go-01", DomainKey: "go", Name: "基本構文"},
				{Key: "go-02", DomainKey: "go", Name: "エラー処理"},
			},
		}},
	}
}

// newTestWorker は偽の Queue と与えた Provider でワーカーを組む。
func newTestWorker(t *testing.T, q *fakeQueue, p llm.Provider, cfg Config) *Worker {
	t.Helper()
	if q.job == nil {
		q.job = &store.JudgmentJob{ID: "job-1", UserID: "user-1", LogID: "log-1", Attempts: 1, Model: "test"}
	}
	if q.input == nil {
		rm := testRoadmap()
		q.input = &store.JudgmentInput{
			RoadmapID: "rm-1",
			LogBody:   "Go の基本構文の確認問題に答えた。",
			LoggedAt:  testNow,
			ApplyMode: "auto",
			Roadmap:   rm,
			States:    map[domain.ItemKey]domain.ItemState{},
			ItemIDs:   map[domain.ItemKey]string{"go-01": "item-1", "go-02": "item-2"},
		}
	}
	// テストの出力にログを混ぜない
	w := New(q, p, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w.now = func() time.Time { return testNow }
	return w
}

func TestProcessOne_仕事が無ければ何もしない(t *testing.T) {
	q := &fakeQueue{claimErr: store.ErrNotFound}
	w := newTestWorker(t, q, llm.NewStub(), Config{})

	worked, err := w.ProcessOne(context.Background())
	if err != nil {
		t.Fatalf("エラーになった: %v", err)
	}
	if worked {
		t.Error("仕事が無いのに処理したことになっている")
	}
	if len(q.calls) != 1 {
		t.Errorf("claim 以外も呼ばれた: %v", q.calls)
	}
}

func TestProcessOne_成功したら応答を保存してから反映する(t *testing.T) {
	q := &fakeQueue{}
	w := newTestWorker(t, q, llm.NewStub(), Config{})

	if _, err := w.ProcessOne(context.Background()); err != nil {
		t.Fatalf("エラーになった: %v", err)
	}

	t.Run("順番は 取り出す→材料→保存→反映→完了", func(t *testing.T) {
		want := []string{"claim", "load", "save", "apply", "finish"}
		if !slices.Equal(q.calls, want) {
			t.Errorf("呼ばれた順が %v（期待 %v）", q.calls, want)
		}
	})

	t.Run("生の出力と消費トークンを残す", func(t *testing.T) {
		if len(q.responses) != 1 {
			t.Fatalf("応答が %d 件保存された", len(q.responses))
		}
		if !strings.Contains(string(q.responses[0].Raw), "go-01") {
			t.Errorf("生の出力が残っていない: %s", q.responses[0].Raw)
		}
	})

	t.Run("検証を通った判定が反映される", func(t *testing.T) {
		if len(q.applied) == 0 {
			t.Fatal("1件も反映されていない")
		}
		for _, a := range q.applied {
			if a.Judgment.ItemKey == "" {
				t.Error("どの判定から来たかが残っていない")
			}
		}
	})
}

func TestProcessOne_確認してから反映する設定では反映しない(t *testing.T) {
	q := &fakeQueue{}
	w := newTestWorker(t, q, llm.NewStub(), Config{})
	q.input.ApplyMode = "confirm"

	if _, err := w.ProcessOne(context.Background()); err != nil {
		t.Fatalf("エラーになった: %v", err)
	}

	if slices.Contains(q.calls, "apply") {
		t.Errorf("保留のはずが反映された: %v", q.calls)
	}
	// 応答自体は残す。あとで採否を決めるときの材料になる（SPEC.md §4.7）
	if len(q.responses) != 1 {
		t.Errorf("応答が保存されていない: %v", q.calls)
	}
	if !slices.Contains(q.calls, "finish") {
		t.Errorf("仕事が閉じられていない: %v", q.calls)
	}
}

func TestProcessOne_一時的な失敗は上限まで順番待ちへ戻す(t *testing.T) {
	temporary := fmt.Errorf("%w: Claude API が 429 を返しました", llm.ErrTemporary)

	t.Run("上限に達していなければ戻す", func(t *testing.T) {
		q := &fakeQueue{}
		w := newTestWorker(t, q, failingProvider{err: temporary}, Config{MaxAttempts: 3})
		q.job.Attempts = 2

		more, err := w.ProcessOne(context.Background())
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if !slices.Contains(q.calls, "requeue") {
			t.Errorf("順番待ちへ戻っていない: %v", q.calls)
		}
		if !strings.Contains(q.requeued, "429") {
			t.Errorf("今回の失敗の原因が残っていない: %q", q.requeued)
		}
		// 戻した直後にすぐ取り直すと、待ち時間ゼロで試行回数を使い切ってしまう
		if more {
			t.Error("戻した仕事をすぐ取りに行こうとしている")
		}
	})

	t.Run("上限に達したら失敗で閉じる", func(t *testing.T) {
		q := &fakeQueue{}
		w := newTestWorker(t, q, failingProvider{err: temporary}, Config{MaxAttempts: 3})
		q.job.Attempts = 3

		if _, err := w.ProcessOne(context.Background()); err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if !slices.Contains(q.calls, "fail") {
			t.Errorf("失敗で閉じていない: %v", q.calls)
		}
		if slices.Contains(q.calls, "requeue") {
			t.Errorf("上限を超えて再試行している: %v", q.calls)
		}
	})
}

func TestProcessOne_出力が読めない失敗は再試行しない(t *testing.T) {
	// 構造化出力で形を縛ってもなお読めないなら、投げ直しても同じ結果になり課金だけ増える
	outErr := &llm.InvalidOutputError{
		Raw:    []byte("承知しました。判定結果は次のとおりです。"),
		Reason: "JSON として解釈できません",
		Usage:  llm.Usage{InputTokens: 7000, OutputTokens: 20},
	}
	q := &fakeQueue{}
	w := newTestWorker(t, q, failingProvider{err: outErr}, Config{MaxAttempts: 3})
	q.job.Attempts = 1

	if _, err := w.ProcessOne(context.Background()); err != nil {
		t.Fatalf("エラーになった: %v", err)
	}

	t.Run("順番待ちへ戻さず失敗で閉じる", func(t *testing.T) {
		if slices.Contains(q.calls, "requeue") {
			t.Errorf("再試行している: %v", q.calls)
		}
		if !slices.Contains(q.calls, "fail") {
			t.Errorf("失敗で閉じていない: %v", q.calls)
		}
	})

	t.Run("読めなかった生の出力を raw_text に残す", func(t *testing.T) {
		if len(q.responses) != 1 {
			t.Fatalf("応答が %d 件保存された", len(q.responses))
		}
		got := q.responses[0]
		if !strings.Contains(got.RawText, "承知しました") {
			t.Errorf("生の出力が残っていない: %q", got.RawText)
		}
		if len(got.Raw) != 0 {
			t.Errorf("JSON でないのに raw へ入れている: %s", got.Raw)
		}
	})

	t.Run("失敗しても課金された分のトークンを残す", func(t *testing.T) {
		got := q.responses[0]
		if got.InputTokens != 7000 || got.OutputTokens != 20 {
			t.Errorf("消費トークンが残っていない: in=%d out=%d", got.InputTokens, got.OutputTokens)
		}
	})
}

func TestProcessOne_拒否された応答も記録する(t *testing.T) {
	refusal := &llm.RefusalError{Category: "cyber", Usage: llm.Usage{InputTokens: 6000, OutputTokens: 5}}
	q := &fakeQueue{}
	w := newTestWorker(t, q, failingProvider{err: refusal}, Config{})

	if _, err := w.ProcessOne(context.Background()); err != nil {
		t.Fatalf("エラーになった: %v", err)
	}
	if len(q.responses) != 1 || q.responses[0].InputTokens != 6000 {
		t.Fatalf("拒否のトークン消費が残っていない: %+v", q.responses)
	}
	if !strings.Contains(q.failed, "拒否") {
		t.Errorf("失敗の原因が残っていない: %q", q.failed)
	}
}

func TestProcessOne_API_に届く前の失敗は応答を残さない(t *testing.T) {
	// 依頼が不正・接続できないときは、消費トークンも生の出力も無い
	q := &fakeQueue{}
	w := newTestWorker(t, q, failingProvider{err: llm.ErrInvalidRequest}, Config{})

	if _, err := w.ProcessOne(context.Background()); err != nil {
		t.Fatalf("エラーになった: %v", err)
	}
	if len(q.responses) != 0 {
		t.Errorf("残すものが無いのに応答を保存した: %+v", q.responses)
	}
	if !slices.Contains(q.calls, "fail") {
		t.Errorf("失敗で閉じていない: %v", q.calls)
	}
}

func TestProcessOne_形の崩れた判定も違反として記録する(t *testing.T) {
	// 1件目は正常、2件目は rationale が空。2件目だけが弾かれ、1件目は生きる
	raw := `{"judgments":[
	  {"itemKey":"go-01","proposedLevel":1,"evidenceType":"drill","rationale":"確認問題に答えた","confidence":0.8},
	  {"itemKey":"go-02","proposedLevel":1,"evidenceType":"drill","rationale":"","confidence":0.8}
	],"unmatched":[]}`
	q := &fakeQueue{}
	w := newTestWorker(t, q, llm.NewStubWithOutput([]byte(raw)), Config{})

	if _, err := w.ProcessOne(context.Background()); err != nil {
		t.Fatalf("エラーになった: %v", err)
	}
	if len(q.responses) != 1 {
		t.Fatalf("応答が %d 件保存された", len(q.responses))
	}

	var shapeRejected int
	for _, v := range q.responses[0].Violations {
		if v.Code == store.ShapeRejectedCode {
			shapeRejected++
			if v.RawIndex == nil || *v.RawIndex != 1 {
				t.Errorf("弾いた判定の位置が残っていない: %+v", v)
			}
		}
	}
	if shapeRejected != 1 {
		t.Errorf("形の検査で弾いた記録が %d 件（期待 1 件）", shapeRejected)
	}
	if len(q.applied) != 1 {
		t.Errorf("正常な判定まで巻き添えになっている: %d 件", len(q.applied))
	}
}

func TestProcessOne_材料が読めないときは原因で扱いを分ける(t *testing.T) {
	t.Run("対象が無いなら再試行せず失敗にする", func(t *testing.T) {
		// ログが消されたあとに仕事だけ残っている場合。やり直しても直らない
		q := &fakeQueue{loadErr: store.ErrNotFound}
		w := newTestWorker(t, q, llm.NewStub(), Config{})

		if _, err := w.ProcessOne(context.Background()); err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if slices.Contains(q.calls, "requeue") {
			t.Errorf("再試行している: %v", q.calls)
		}
		if !strings.Contains(q.failed, "材料") {
			t.Errorf("失敗の原因が残っていない: %q", q.failed)
		}
	})

	t.Run("DB が一時的に落ちているだけなら再試行する", func(t *testing.T) {
		// 区別しないと、DB が一瞬詰まっただけで判定が永久に失われる
		q := &fakeQueue{loadErr: errors.New("connection refused")}
		w := newTestWorker(t, q, llm.NewStub(), Config{MaxAttempts: 3})
		q.job.Attempts = 1

		if _, err := w.ProcessOne(context.Background()); err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if !slices.Contains(q.calls, "requeue") {
			t.Errorf("順番待ちへ戻っていない: %v", q.calls)
		}
	})
}

func TestProcessOne_棄却された判定は反映へ渡さない(t *testing.T) {
	// レベルが範囲外（V2）の判定をそのまま保存へ流すと、DB の制約違反で
	// 同じログの正常な判定まで巻き戻る（2026-09-23 に PR #29 のレビューで判明）
	raw := `{"judgments":[
	  {"itemKey":"go-01","proposedLevel":9,"evidenceType":"drill","rationale":"範囲外のレベル","confidence":0.9},
	  {"itemKey":"go-02","proposedLevel":1,"evidenceType":"drill","rationale":"確認問題に答えた","confidence":0.9}
	],"unmatched":[]}`
	q := &fakeQueue{}
	w := newTestWorker(t, q, llm.NewStubWithOutput([]byte(raw)), Config{})

	if _, err := w.ProcessOne(context.Background()); err != nil {
		t.Fatalf("エラーになった: %v", err)
	}

	if len(q.applied) != 1 {
		t.Fatalf("反映へ %d 件渡った（正常な 1 件だけのはず）", len(q.applied))
	}
	if q.applied[0].Judgment.ItemKey != "go-02" {
		t.Errorf("渡ったのが %q（go-02 のはず）", q.applied[0].Judgment.ItemKey)
	}
	// 棄却した事実は記録に残る（握りつぶさない）
	var found bool
	for _, v := range q.responses[0].Violations {
		if v.Code == string(domain.ViolationLevelOutOfRange) {
			found = true
		}
	}
	if !found {
		t.Errorf("棄却の記録が残っていない: %+v", q.responses[0].Violations)
	}
}

func TestProcessOne_反映に失敗しても応答は残っている(t *testing.T) {
	q := &fakeQueue{applyErr: errors.New("DB が落ちている")}
	w := newTestWorker(t, q, llm.NewStub(), Config{})

	if _, err := w.ProcessOne(context.Background()); err != nil {
		t.Fatalf("エラーになった: %v", err)
	}
	if len(q.responses) != 1 {
		t.Error("反映より先に応答を保存していない")
	}
	if !strings.Contains(q.failed, "反映") {
		t.Errorf("失敗の原因が残っていない: %q", q.failed)
	}
}

func TestConfig_ゼロ値は既定値で埋まる(t *testing.T) {
	got := Config{}.withDefaults()
	if got.MaxAttempts != defaultMaxAttempts {
		t.Errorf("MaxAttempts = %d", got.MaxAttempts)
	}
	if got.PollInterval != defaultPollInterval {
		t.Errorf("PollInterval = %v", got.PollInterval)
	}
}
