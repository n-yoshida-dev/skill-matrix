package store

import (
	"context"
	"testing"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
)

// このテストは判定の材料の読み出しと、結果の反映を本物の PostgreSQL で確かめる。
//
// 反映は「イベントを積む」と「現在の状態を書き直す」の2段構えで、
// 途中で落ちたときに片方だけ残らないことが要。そこを重点的に見る。

func TestLoadJudgmentInput_判定に必要な材料が揃う(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	userID, _, logID, _ := enqueueTestJob(t, st)

	job, err := st.ClaimJudgmentJob(ctx, jobNow)
	if err != nil {
		t.Fatalf("取り出しに失敗: %v", err)
	}

	in, err := st.LoadJudgmentInput(ctx, job)
	if err != nil {
		t.Fatalf("材料の読み出しに失敗: %v", err)
	}

	t.Run("ログ本文と学習日と反映モードが読める", func(t *testing.T) {
		if in.LogBody == "" {
			t.Error("ログ本文が空")
		}
		if in.LoggedAt.IsZero() {
			t.Error("学習日が入っていない")
		}
		if in.ApplyMode != "auto" {
			t.Errorf("反映モードが %q（既定は auto）", in.ApplyMode)
		}
	})

	t.Run("ロードマップが純粋関数の扱える形で読める", func(t *testing.T) {
		items := in.Roadmap.AllItems()
		if len(items) == 0 {
			t.Fatal("項目が1件も無い")
		}
		if items[0].Key != "go-01" {
			t.Errorf("並び順が崩れている: 先頭が %q", items[0].Key)
		}
		if len(in.Levels) != 5 {
			t.Errorf("レベル定義が %d 件（5 件のはず）", len(in.Levels))
		}
		if in.Levels[0].Criteria == "" {
			t.Error("判定基準（criteria）が空")
		}
	})

	t.Run("項目の key から items.id を引ける", func(t *testing.T) {
		// 判定は key でやり取りし、保存は id で行う。その対応表がここで要る
		if id, ok := in.ItemIDs["go-01"]; !ok || id == "" {
			t.Errorf("go-01 の id が引けない: %q", id)
		}
	})

	t.Run("まだ判定されていない項目は状態に入らない", func(t *testing.T) {
		if len(in.States) != 0 {
			t.Errorf("状態が %d 件（まだ判定していないので 0 件のはず）", len(in.States))
		}
	})

	_ = userID
	_ = logID
}

func TestLoadJudgmentInput_ログが消えていればErrNotFound(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	_, _, logID, _ := enqueueTestJob(t, st)

	job, err := st.ClaimJudgmentJob(ctx, jobNow)
	if err != nil {
		t.Fatalf("取り出しに失敗: %v", err)
	}
	if _, err := st.db.ExecContext(ctx, "DELETE FROM learning_logs WHERE id = $1", logID); err != nil {
		t.Fatalf("ログの削除に失敗: %v", err)
	}

	if _, err := st.LoadJudgmentInput(ctx, job); err == nil {
		t.Fatal("エラーになるはずが通った")
	}
}

// applyOne は判定1件ぶんの適用結果を組み立てる。
func applyOne(key domain.ItemKey, level domain.Level, ev domain.EvidenceType, occurredAt time.Time) domain.Applied {
	return domain.Applied{
		Judgment: domain.Judgment{
			ItemKey:       key,
			ProposedLevel: level,
			EvidenceType:  ev,
			Rationale:     "確認問題に答えている",
			Confidence:    0.8,
			OccurredAt:    occurredAt,
		},
		State: domain.ItemState{
			ItemKey:        key,
			Level:          level,
			PreState:       domain.PreStateNone,
			LastEvidenceAt: occurredAt,
		},
		Changed: true,
	}
}

func TestApplyJudgmentResult_イベントを積んで現在の状態を書き直す(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	userID, _, logID, _ := enqueueTestJob(t, st)

	job, err := st.ClaimJudgmentJob(ctx, jobNow)
	if err != nil {
		t.Fatalf("取り出しに失敗: %v", err)
	}
	in, err := st.LoadJudgmentInput(ctx, job)
	if err != nil {
		t.Fatalf("材料の読み出しに失敗: %v", err)
	}

	applied := []domain.Applied{
		applyOne("go-01", domain.LevelBasicConfirmed, domain.EvidenceDrill, jobNow),
	}
	if err := st.ApplyJudgmentResult(ctx, job, in, applied); err != nil {
		t.Fatalf("反映に失敗: %v", err)
	}

	t.Run("イベントが1件積まれる", func(t *testing.T) {
		n := countRows(t, st, "assessment_events",
			"user_id = $1 AND log_id = $2 AND source = 'ai' AND applied_level = 1 AND evidence_type = 'drill'",
			userID, logID)
		if n != 1 {
			t.Errorf("イベントが %d 件", n)
		}
	})

	t.Run("現在の状態が書き直される", func(t *testing.T) {
		n := countRows(t, st, "item_states",
			"item_id = $1 AND user_id = $2 AND level = 1 AND last_event_id IS NOT NULL",
			in.ItemIDs["go-01"], userID)
		if n != 1 {
			t.Errorf("状態が %d 件", n)
		}
	})

	t.Run("次の判定では現在の状態が読み出せる", func(t *testing.T) {
		again, err := st.LoadJudgmentInput(ctx, job)
		if err != nil {
			t.Fatalf("材料の再読み出しに失敗: %v", err)
		}
		got, ok := again.States["go-01"]
		if !ok {
			t.Fatal("go-01 の状態が読めない")
		}
		if got.Level != domain.LevelBasicConfirmed {
			t.Errorf("レベルが %d", got.Level)
		}
		if got.LastEvidenceAt.IsZero() {
			t.Error("最終根拠日が入っていない")
		}
	})

	t.Run("直近の根拠が読み出せる", func(t *testing.T) {
		// 過去に何を根拠に上がったかを次のプロンプトへ渡す（SPEC.md §4.2）
		again, err := st.LoadJudgmentInput(ctx, job)
		if err != nil {
			t.Fatalf("材料の再読み出しに失敗: %v", err)
		}
		if got := again.RecentEvidence["go-01"]; len(got) != 1 || got[0] != "確認問題に答えている" {
			t.Errorf("直近の根拠が読めない: %v", got)
		}
	})
}

func TestApplyJudgmentResult_純粋関数が出したそのままの形で保存できる(t *testing.T) {
	// 手で組み立てた Applied ではなく、domain.ApplyJudgments の出力をそのまま渡す。
	//
	// まだ一度も判定されていない項目の ItemState は PreState が空（ゼロ値）になる。
	// DB は 'none' などの決まった値しか受け付けないため、そのまま入れると制約違反で落ちる。
	// 手で組み立てたテストデータでは PreState を明示していたので気づけなかった
	// （2026-09-23 に実機の通し確認で判明）。
	st := testStore(t)
	ctx := context.Background()
	userID, _, _, _ := enqueueTestJob(t, st)

	job, err := st.ClaimJudgmentJob(ctx, jobNow)
	if err != nil {
		t.Fatalf("取り出しに失敗: %v", err)
	}
	in, err := st.LoadJudgmentInput(ctx, job)
	if err != nil {
		t.Fatalf("材料の読み出しに失敗: %v", err)
	}

	batch := domain.ApplyJudgments(in.Roadmap, in.States, []domain.Judgment{{
		ItemKey:       "go-01",
		ProposedLevel: domain.LevelBasicConfirmed,
		EvidenceType:  domain.EvidenceDrill,
		Rationale:     "確認問題に答えている",
		Confidence:    0.8,
		OccurredAt:    jobNow,
	}}, domain.DefaultRules())

	if err := st.ApplyJudgmentResult(ctx, job, in, batch.Results); err != nil {
		t.Fatalf("反映に失敗: %v", err)
	}
	if n := countRows(t, st, "assessment_events", "user_id = $1 AND pre_state = 'none'", userID); n != 1 {
		t.Errorf("イベントが %d 件（段階前の状態が none で 1 件のはず）", n)
	}
	if n := countRows(t, st, "item_states", "user_id = $1 AND level = 1 AND pre_state = 'none'", userID); n != 1 {
		t.Errorf("状態が %d 件", n)
	}
}

func TestApplyJudgmentResult_2回目の判定で状態が上書きされる(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	userID, _, _, _ := enqueueTestJob(t, st)

	job, err := st.ClaimJudgmentJob(ctx, jobNow)
	if err != nil {
		t.Fatalf("取り出しに失敗: %v", err)
	}
	in, err := st.LoadJudgmentInput(ctx, job)
	if err != nil {
		t.Fatalf("材料の読み出しに失敗: %v", err)
	}

	for i, lv := range []domain.Level{domain.LevelBasicConfirmed, domain.LevelCanExplain} {
		ev := domain.EvidenceDrill
		if lv == domain.LevelCanExplain {
			ev = domain.EvidenceSelfExplanation
		}
		applied := []domain.Applied{applyOne("go-01", lv, ev, jobNow.AddDate(0, 0, i))}
		if err := st.ApplyJudgmentResult(ctx, job, in, applied); err != nil {
			t.Fatalf("%d 回目の反映に失敗: %v", i+1, err)
		}
	}

	// イベントは追記され、状態は1行のまま最新に書き換わる
	if n := countRows(t, st, "assessment_events", "user_id = $1", userID); n != 2 {
		t.Errorf("イベントが %d 件（2 件のはず）", n)
	}
	if n := countRows(t, st, "item_states", "item_id = $1 AND level = 2", in.ItemIDs["go-01"]); n != 1 {
		t.Error("状態が最新になっていない")
	}
}

func TestApplyJudgmentResult_保留はイベントを積まない(t *testing.T) {
	// 保留は assessment_events を作らないことで表す（SPEC.md §4.7）
	st := testStore(t)
	ctx := context.Background()
	userID, _, _, _ := enqueueTestJob(t, st)

	job, err := st.ClaimJudgmentJob(ctx, jobNow)
	if err != nil {
		t.Fatalf("取り出しに失敗: %v", err)
	}
	in, err := st.LoadJudgmentInput(ctx, job)
	if err != nil {
		t.Fatalf("材料の読み出しに失敗: %v", err)
	}

	deferred := applyOne("go-01", domain.LevelBasicConfirmed, domain.EvidenceDrill, jobNow)
	deferred.Deferred = true
	if err := st.ApplyJudgmentResult(ctx, job, in, []domain.Applied{deferred}); err != nil {
		t.Fatalf("反映に失敗: %v", err)
	}

	if n := countRows(t, st, "assessment_events", "user_id = $1", userID); n != 0 {
		t.Errorf("保留なのにイベントが %d 件積まれた", n)
	}
}

func TestApplyJudgmentResult_途中で失敗したら何も残らない(t *testing.T) {
	// イベントだけ残って状態が古い、という食い違いを作らない
	st := testStore(t)
	ctx := context.Background()
	userID, _, _, _ := enqueueTestJob(t, st)

	job, err := st.ClaimJudgmentJob(ctx, jobNow)
	if err != nil {
		t.Fatalf("取り出しに失敗: %v", err)
	}
	in, err := st.LoadJudgmentInput(ctx, job)
	if err != nil {
		t.Fatalf("材料の読み出しに失敗: %v", err)
	}

	ok := applyOne("go-01", domain.LevelBasicConfirmed, domain.EvidenceDrill, jobNow)
	// 2件目は存在しない項目。ItemIDs に無いので反映の途中で止まる
	ng := applyOne("存在しない項目", domain.LevelBasicConfirmed, domain.EvidenceDrill, jobNow)

	if err := st.ApplyJudgmentResult(ctx, job, in, []domain.Applied{ok, ng}); err == nil {
		t.Fatal("エラーになるはずが通った")
	}
	if n := countRows(t, st, "assessment_events", "user_id = $1", userID); n != 0 {
		t.Errorf("巻き戻っていない: イベントが %d 件残った", n)
	}
	if n := countRows(t, st, "item_states", "user_id = $1", userID); n != 0 {
		t.Errorf("巻き戻っていない: 状態が %d 件残った", n)
	}
}
