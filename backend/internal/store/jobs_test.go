package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
)

// このテストは順番待ち（llm_jobs）と応答の保存（llm_responses）を本物の PostgreSQL で確かめる。
// SQL の綴り・制約違反・取り合いの回避は、DB が無いと確かめようがない。

var jobNow = time.Date(2026, 9, 23, 9, 0, 0, 0, time.UTC)

// createTestLog はダミーの学習ログを1件作って id を返す。
func createTestLog(t *testing.T, st *Store, userID, roadmapID, body string) string {
	t.Helper()
	const q = `
INSERT INTO learning_logs (user_id, roadmap_id, body, logged_at)
VALUES ($1, $2, $3, $4) RETURNING id`

	var id string
	err := st.db.QueryRowContext(context.Background(), q, userID, roadmapID, body, jobNow).Scan(&id)
	if err != nil {
		t.Fatalf("テスト用の学習ログの作成に失敗: %v", err)
	}
	return id
}

// enqueueTestJob はユーザー・ロードマップ・ログ・仕事を一式作る。
func enqueueTestJob(t *testing.T, st *Store) (userID, roadmapID, logID, jobID string) {
	t.Helper()
	u := createTestUser(t, st)
	roadmapID = importSample(t, st, u.ID)
	logID = createTestLog(t, st, u.ID, roadmapID, "Go の基本構文の確認問題に答えた。")

	jobID, err := st.EnqueueJudgmentJob(context.Background(), u.ID, logID, "claude-sonnet-5")
	if err != nil {
		t.Fatalf("判定ジョブの登録に失敗: %v", err)
	}
	return u.ID, roadmapID, logID, jobID
}

func TestEnqueueJudgmentJob_自分のログだけ積める(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	owner := createTestUser(t, st)
	rmID := importSample(t, st, owner.ID)
	logID := createTestLog(t, st, owner.ID, rmID, "ログ本文")

	t.Run("持ち主なら積める", func(t *testing.T) {
		jobID, err := st.EnqueueJudgmentJob(ctx, owner.ID, logID, "claude-sonnet-5")
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if n := countRows(t, st, "llm_jobs", "id = $1 AND status = 'queued' AND type = 'judgment'", jobID); n != 1 {
			t.Errorf("queued の判定ジョブが %d 件", n)
		}
	})

	t.Run("他人のログは存在しない扱いにする", func(t *testing.T) {
		// 他人のログを判定させられると、書いた覚えのない理解度が他人の画面に増える
		other := createTestUser(t, st)
		if _, err := st.EnqueueJudgmentJob(ctx, other.ID, logID, "claude-sonnet-5"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("ErrNotFound のはずが %v", err)
		}
	})
}

func TestClaimJudgmentJob_同じ仕事を二度取らない(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	_, _, _, jobID := enqueueTestJob(t, st)

	got, err := st.ClaimJudgmentJob(ctx, jobNow)
	if err != nil {
		t.Fatalf("取り出しに失敗: %v", err)
	}
	if got.ID != jobID {
		t.Errorf("別の仕事を取った: %s", got.ID)
	}
	if got.Attempts != 1 {
		t.Errorf("試行回数が %d（初回なので 1 のはず）", got.Attempts)
	}

	// 取り出した仕事は running なので、もう取れない。
	// ここが崩れると、同じログを二重に判定して二重に課金される
	if _, err := st.ClaimJudgmentJob(ctx, jobNow); !errors.Is(err, ErrNotFound) {
		t.Fatalf("2回目は ErrNotFound のはずが %v", err)
	}
}

func TestClaimJudgmentJob_古い順に取り出す(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	u := createTestUser(t, st)
	rmID := importSample(t, st, u.ID)

	var want []string
	for i := range 2 {
		logID := createTestLog(t, st, u.ID, rmID, "ログ")
		jobID, err := st.EnqueueJudgmentJob(ctx, u.ID, logID, "m")
		if err != nil {
			t.Fatalf("%d 件目の登録に失敗: %v", i, err)
		}
		want = append(want, jobID)
		// created_at の既定値は now() なので、順序を確実にするため間隔を空ける
		if _, err := st.db.ExecContext(ctx,
			"UPDATE llm_jobs SET created_at = $1 WHERE id = $2",
			jobNow.Add(time.Duration(i)*time.Minute), jobID); err != nil {
			t.Fatalf("created_at の調整に失敗: %v", err)
		}
	}

	for i, wantID := range want {
		got, err := st.ClaimJudgmentJob(ctx, jobNow)
		if err != nil {
			t.Fatalf("%d 件目の取り出しに失敗: %v", i, err)
		}
		if got.ID != wantID {
			t.Errorf("%d 件目が %s（期待 %s）", i, got.ID, wantID)
		}
	}
}

func TestClaimJudgmentJob_到達状態の下書きは取らない(t *testing.T) {
	// 同じキューに別の種類の仕事が乗る（SPEC.md §4.8）。判定のワーカーが拾ってはいけない
	st := testStore(t)
	ctx := context.Background()
	u := createTestUser(t, st)
	rmID := importSample(t, st, u.ID)

	if _, err := st.db.ExecContext(ctx,
		`INSERT INTO llm_jobs (type, user_id, roadmap_id, model) VALUES ('outcome_draft', $1, $2, 'm')`,
		u.ID, rmID); err != nil {
		t.Fatalf("下書きジョブの登録に失敗: %v", err)
	}

	if _, err := st.ClaimJudgmentJob(ctx, jobNow); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ErrNotFound のはずが %v", err)
	}
}

func TestJobLifecycle_完了と失敗と再登録(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	t.Run("完了にすると状態と終了時刻が入る", func(t *testing.T) {
		_, _, _, jobID := enqueueTestJob(t, st)
		if err := st.FinishJob(ctx, jobID, jobNow); err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if n := countRows(t, st, "llm_jobs",
			"id = $1 AND status = 'succeeded' AND finished_at IS NOT NULL AND error IS NULL", jobID); n != 1 {
			t.Error("完了になっていない")
		}
	})

	t.Run("失敗には必ず原因を残す", func(t *testing.T) {
		_, _, _, jobID := enqueueTestJob(t, st)
		if err := st.FailJob(ctx, jobID, "", jobNow); err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		// 空文字で呼ばれても「原因なし」で保存しない。何も調べられなくなるため
		if n := countRows(t, st, "llm_jobs", "id = $1 AND status = 'failed' AND error <> ''", jobID); n != 1 {
			t.Error("失敗の原因が残っていない")
		}
	})

	t.Run("再登録すると順番待ちへ戻り、開始時刻が消える", func(t *testing.T) {
		_, _, _, jobID := enqueueTestJob(t, st)
		if _, err := st.ClaimJudgmentJob(ctx, jobNow); err != nil {
			t.Fatalf("取り出しに失敗: %v", err)
		}
		if err := st.RequeueJob(ctx, jobID, "429 が返った", jobNow); err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if n := countRows(t, st, "llm_jobs",
			"id = $1 AND status = 'queued' AND started_at IS NULL AND error = '429 が返った'", jobID); n != 1 {
			t.Error("順番待ちへ戻っていない")
		}

		// 戻したので、もう一度取り出せる。試行回数は増えている
		got, err := st.ClaimJudgmentJob(ctx, jobNow)
		if err != nil {
			t.Fatalf("再取り出しに失敗: %v", err)
		}
		if got.Attempts != 2 {
			t.Errorf("試行回数が %d（2 のはず）", got.Attempts)
		}
	})

	t.Run("存在しない仕事の更新は ErrNotFound", func(t *testing.T) {
		// 黙って成功にすると、更新できていない仕事を成功として扱ってしまう
		err := st.FinishJob(ctx, "00000000-0000-0000-0000-000000000000", jobNow)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("ErrNotFound のはずが %v", err)
		}
	})
}

func TestGetJobState_他人の仕事は見えない(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	userID, _, _, jobID := enqueueTestJob(t, st)

	got, err := st.GetJobState(ctx, userID, jobID)
	if err != nil {
		t.Fatalf("取得に失敗: %v", err)
	}
	if got.Status != JobQueued || got.Type != JobJudgment {
		t.Errorf("状態が %+v", got)
	}

	other := createTestUser(t, st)
	if _, err := st.GetJobState(ctx, other.ID, jobID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ErrNotFound のはずが %v", err)
	}
}

func TestSaveLLMResponse_読めた応答と読めなかった応答を別の列に入れる(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	t.Run("読めた応答は raw へ", func(t *testing.T) {
		_, _, _, jobID := enqueueTestJob(t, st)
		err := st.SaveLLMResponse(ctx, LLMResponse{
			JobID:        jobID,
			Raw:          json.RawMessage(`{"judgments":[]}`),
			InputTokens:  7500,
			OutputTokens: 500,
		})
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if n := countRows(t, st, "llm_responses",
			"job_id = $1 AND raw IS NOT NULL AND raw_text IS NULL AND input_tokens = 7500", jobID); n != 1 {
			t.Error("raw に入っていない")
		}
	})

	t.Run("読めなかった応答は raw_text へ", func(t *testing.T) {
		// jsonb の列には壊れた文字列が入らない。捨てると原因を調べようがなくなる
		_, _, _, jobID := enqueueTestJob(t, st)
		err := st.SaveLLMResponse(ctx, LLMResponse{
			JobID:   jobID,
			RawText: "承知しました。判定結果は次のとおりです。",
		})
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if n := countRows(t, st, "llm_responses",
			"job_id = $1 AND raw IS NULL AND raw_text LIKE '承知%'", jobID); n != 1 {
			t.Error("raw_text に入っていない")
		}
	})

	t.Run("どちらも空の応答は DB が弾く", func(t *testing.T) {
		_, _, _, jobID := enqueueTestJob(t, st)
		if err := st.SaveLLMResponse(ctx, LLMResponse{JobID: jobID}); err == nil {
			t.Fatal("制約で弾かれるはずが通った")
		}
	})

	t.Run("違反は空でも null ではなく空配列で入る", func(t *testing.T) {
		_, _, _, jobID := enqueueTestJob(t, st)
		if err := st.SaveLLMResponse(ctx, LLMResponse{JobID: jobID, Raw: json.RawMessage(`{}`)}); err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if n := countRows(t, st, "llm_responses", "job_id = $1 AND violations = '[]'::jsonb", jobID); n != 1 {
			t.Error("violations が空配列になっていない")
		}
	})
}

func TestViolationsOf_domainの違反を保存用に移す(t *testing.T) {
	got := ViolationsOf([]domain.Violation{{
		Code:     domain.ViolationLevelJump,
		ItemKey:  "go-01",
		Detail:   "2段階の昇格を1段に切り詰めた",
		Proposed: 3,
		Applied:  1,
	}})

	if len(got) != 1 {
		t.Fatalf("%d 件になった", len(got))
	}
	if got[0].Code != string(domain.ViolationLevelJump) || got[0].ItemKey != "go-01" {
		t.Errorf("中身が違う: %+v", got[0])
	}
	if got[0].Proposed == nil || *got[0].Proposed != 3 || got[0].Applied == nil || *got[0].Applied != 1 {
		t.Errorf("レベルが移っていない: %+v", got[0])
	}
}
