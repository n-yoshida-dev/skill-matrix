package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
	"github.com/n-yoshida-dev/skill-matrix/internal/roadmap"
)

// このファイルは判定の「材料の読み出し」と「結果の反映」を持つ（SPEC.md §4.1）。
//
// 検証そのもの（V1〜V8）はここではやらない。純粋関数の domain.ApplyJudgments が担当し、
// このファイルはその入力を用意して、出てきた結果を書くだけにする。
// DB を触る処理と判断する処理を混ぜないことで、判断の側を DB 無しでテストできる。

// JudgmentInput は判定1件に必要な材料（SPEC.md §4.2 のプロンプトの材料と、反映先の情報）。
type JudgmentInput struct {
	RoadmapID string
	// LogBody は学習ログの本文。
	LogBody string
	// LoggedAt は学習した日。判定の根拠が「いつのことか」を表す。投稿日とは別。
	LoggedAt time.Time
	// ApplyMode は 'auto' か 'confirm'（SPEC.md §4.7）。
	ApplyMode string

	// Roadmap / Levels / States は llm.JudgmentRequest にそのまま渡す。
	Roadmap domain.Roadmap
	Levels  []roadmap.LevelDef
	States  map[domain.ItemKey]domain.ItemState
	// RecentEvidence は項目ごとの直近の根拠（SPEC.md §4.2 の3番目）。
	RecentEvidence map[domain.ItemKey][]string

	// ItemIDs は項目の key から DB の items.id を引くための対応表。
	// 判定は key でやり取りし、保存は id で行うため、境目のここで持つ。
	ItemIDs map[domain.ItemKey]string
}

// recentEvidencePerItem は1項目につき何件の「直近の根拠」をプロンプトに載せるか。
// 増やすほど入力トークンが増え、全項目ぶん積み上がる。1件でも二重の昇格は避けられる。
const recentEvidencePerItem = 1

// LoadJudgmentInput は判定に必要な材料をまとめて読む。
func (s *Store) LoadJudgmentInput(ctx context.Context, job *JudgmentJob) (*JudgmentInput, error) {
	in := &JudgmentInput{}

	const logQ = `
SELECT l.body, l.logged_at, l.roadmap_id, u.apply_mode
  FROM learning_logs l
  JOIN users u ON u.id = l.user_id
 WHERE l.id = $1 AND l.user_id = $2`

	err := s.db.QueryRowContext(ctx, logQ, job.LogID, job.UserID).
		Scan(&in.LogBody, &in.LoggedAt, &in.RoadmapID, &in.ApplyMode)
	if errors.Is(err, sql.ErrNoRows) {
		// ログが消えた（ユーザーが削除した）あとに仕事だけ残っている場合
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("学習ログの読み出しに失敗しました: %w", err)
	}

	rm, err := s.loadRoadmapForJudgment(ctx, in.RoadmapID)
	if err != nil {
		return nil, err
	}
	in.Roadmap = rm.roadmap
	in.Levels = rm.levels
	in.ItemIDs = rm.itemIDs

	if in.States, err = s.loadItemStates(ctx, job.UserID, in.RoadmapID); err != nil {
		return nil, err
	}
	if in.RecentEvidence, err = s.loadRecentEvidence(ctx, job.UserID, in.RoadmapID); err != nil {
		return nil, err
	}
	return in, nil
}

// judgmentRoadmap は判定に使う形に直したロードマップ。
type judgmentRoadmap struct {
	roadmap domain.Roadmap
	levels  []roadmap.LevelDef
	itemIDs map[domain.ItemKey]string
}

// loadRoadmapForJudgment はロードマップを純粋関数が扱える形（domain.Roadmap）で読む。
func (s *Store) loadRoadmapForJudgment(ctx context.Context, roadmapID string) (*judgmentRoadmap, error) {
	const q = `SELECT name, target_date, levels::text FROM roadmaps WHERE id = $1`

	var name, levelsJSON string
	var targetDate sql.NullTime
	err := s.db.QueryRowContext(ctx, q, roadmapID).Scan(&name, &targetDate, &levelsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ロードマップの読み出しに失敗しました: %w", err)
	}

	out := &judgmentRoadmap{itemIDs: map[domain.ItemKey]string{}}
	if err := json.Unmarshal([]byte(levelsJSON), &out.levels); err != nil {
		return nil, fmt.Errorf("レベル定義（levels）の読み出しに失敗しました: %w", err)
	}

	domains, err := s.loadDomains(ctx, roadmapID)
	if err != nil {
		return nil, err
	}

	out.roadmap = domain.Roadmap{Name: name}
	if targetDate.Valid {
		out.roadmap.TargetDate = targetDate.Time
	}
	for di, d := range domains {
		dd := domain.Domain{
			Key:        domain.DomainKey(d.Key),
			Name:       d.Name,
			Goal:       d.Goal,
			OrderIndex: di,
		}
		for ii, it := range d.Items {
			deps := make([]domain.ItemKey, 0, len(it.DependsOn))
			for _, k := range it.DependsOn {
				deps = append(deps, domain.ItemKey(k))
			}
			dd.Items = append(dd.Items, domain.Item{
				Key:         domain.ItemKey(it.Key),
				DomainKey:   domain.DomainKey(d.Key),
				Name:        it.Name,
				Description: it.Description,
				Outcome:     it.Outcome,
				VerifyBy:    it.VerifyBy,
				DependsOn:   deps,
				OrderIndex:  ii,
			})
			out.itemIDs[domain.ItemKey(it.Key)] = it.ID
		}
		out.roadmap.Domains = append(out.roadmap.Domains, dd)
	}
	return out, nil
}

// loadItemStates は現在の理解度を項目の key 引きで返す。
// まだ一度も判定されていない項目は入らない（ゼロ値＝レベル0として扱えばよい）。
func (s *Store) loadItemStates(ctx context.Context, userID, roadmapID string) (map[domain.ItemKey]domain.ItemState, error) {
	const q = `
SELECT i.key, st.level, st.pre_state, st.needs_review, st.last_evidence_at
  FROM item_states st
  JOIN items i ON i.id = st.item_id
 WHERE st.user_id = $1 AND i.roadmap_id = $2`

	rows, err := s.db.QueryContext(ctx, q, userID, roadmapID)
	if err != nil {
		return nil, fmt.Errorf("現在の理解度の取得に失敗しました: %w", err)
	}
	defer rows.Close()

	states := map[domain.ItemKey]domain.ItemState{}
	for rows.Next() {
		var st domain.ItemState
		var key string
		var lastEvidenceAt sql.NullTime
		if err := rows.Scan(&key, &st.Level, &st.PreState, &st.NeedsReview, &lastEvidenceAt); err != nil {
			return nil, fmt.Errorf("現在の理解度の読み出しに失敗しました: %w", err)
		}
		st.ItemKey = domain.ItemKey(key)
		if lastEvidenceAt.Valid {
			st.LastEvidenceAt = lastEvidenceAt.Time
		}
		states[st.ItemKey] = st
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("現在の理解度の走査に失敗しました: %w", err)
	}
	return states, nil
}

// loadRecentEvidence は項目ごとの直近の根拠を新しい順に返す。
//
// 過去に何を根拠にレベルが上がったかを LLM に見せると、同じ根拠での二重の昇格を避けやすい。
// 全部を渡すと入力が膨らむので、項目あたり recentEvidencePerItem 件に絞る。
func (s *Store) loadRecentEvidence(ctx context.Context, userID, roadmapID string) (map[domain.ItemKey][]string, error) {
	const q = `
SELECT key, rationale FROM (
  SELECT i.key,
         e.rationale,
         row_number() OVER (PARTITION BY e.item_id ORDER BY e.occurred_at DESC, e.created_at DESC) AS rn
    FROM assessment_events e
    JOIN items i ON i.id = e.item_id
   WHERE e.user_id = $1 AND i.roadmap_id = $2
) ranked
 WHERE rn <= $3
 ORDER BY key, rn`

	rows, err := s.db.QueryContext(ctx, q, userID, roadmapID, recentEvidencePerItem)
	if err != nil {
		return nil, fmt.Errorf("直近の根拠の取得に失敗しました: %w", err)
	}
	defer rows.Close()

	out := map[domain.ItemKey][]string{}
	for rows.Next() {
		var key, rationale string
		if err := rows.Scan(&key, &rationale); err != nil {
			return nil, fmt.Errorf("直近の根拠の読み出しに失敗しました: %w", err)
		}
		k := domain.ItemKey(key)
		out[k] = append(out[k], rationale)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("直近の根拠の走査に失敗しました: %w", err)
	}
	return out, nil
}

// ApplyJudgmentResult は検証を通った判定を理解度へ反映する。
//
// イベント（assessment_events）を積んでから、そこから導ける現在の状態（item_states）を更新する。
// **1つのトランザクションで行う。** 途中で落ちると「イベントはあるのに状態が古い」
// 「状態だけ進んでイベントが無い」という、どちらも説明のつかない食い違いが残る。
//
// 保留（確信度が閾値未満＝V7）の判定は渡さないこと。保留はイベントを積まずに表す（SPEC.md §4.7）。
func (s *Store) ApplyJudgmentResult(ctx context.Context, job *JudgmentJob, in *JudgmentInput, applied []domain.Applied) (err error) {
	if len(applied) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("判定の反映を開始できません: %w", err)
	}
	defer func() {
		if err != nil {
			// 巻き戻しの失敗も握りつぶさない。元のエラーは残したまま書き足す
			if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
				err = fmt.Errorf("%w（巻き戻しにも失敗: %v）", err, rbErr)
			}
		}
	}()

	for _, a := range applied {
		if a.Deferred {
			// 保留はイベントを積まない。呼び出し側の絞り込み漏れをここでも止める
			continue
		}
		itemID, ok := in.ItemIDs[a.State.ItemKey]
		if !ok {
			// V1 を通っているはずなので起きない。起きたらデータ不整合として止める
			return fmt.Errorf("項目 %q が見つかりません", a.State.ItemKey)
		}
		eventID, insErr := insertAssessmentEvent(ctx, tx, job, itemID, a)
		if insErr != nil {
			return insErr
		}
		if err := upsertItemState(ctx, tx, job.UserID, itemID, eventID, a.State); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("判定の反映を確定できません: %w", err)
	}
	return nil
}

// dbPreState は段階前の状態を DB に入れられる値に直す。
//
// domain 側では「まだ一度も判定されていない項目」の PreState が空文字（ゼロ値）になる。
// DB は 'none' / 'learning' / 'explained_only' / 'self_reported' しか受け付けないので、
// 空文字はここで 'none'（特記なし）に読み替える。
// domain 側を直さないのは、ゼロ値が使える（ItemState{} がそのまま「未判定」を表す）ことに
// 意味があるため。DB の都合は DB に近いこの層で吸収する。
func dbPreState(ps domain.PreState) string {
	if ps == "" {
		return string(domain.PreStateNone)
	}
	return string(ps)
}

// insertAssessmentEvent は判定1件を「確定したできごと」として積む。
func insertAssessmentEvent(ctx context.Context, tx *sql.Tx, job *JudgmentJob, itemID string, a domain.Applied) (string, error) {
	const q = `
INSERT INTO assessment_events
  (user_id, item_id, log_id, source, proposed_level, applied_level,
   pre_state, evidence_type, rationale, confidence, occurred_at)
VALUES ($1, $2, $3, 'ai', $4, $5, $6, $7, $8, $9, $10)
RETURNING id`

	var eventID string
	err := tx.QueryRowContext(ctx, q,
		job.UserID, itemID, job.LogID,
		int(a.Judgment.ProposedLevel), int(a.State.Level),
		dbPreState(a.State.PreState), string(a.Judgment.EvidenceType),
		a.Judgment.Rationale, a.Judgment.Confidence, a.Judgment.OccurredAt.UTC(),
	).Scan(&eventID)
	if err != nil {
		return "", fmt.Errorf("判定イベントの保存に失敗しました: %w", err)
	}
	return eventID, nil
}

// upsertItemState は現在の理解度を書き直す。
//
// item_states は assessment_events から再計算できる導出値なので、
// ここでの値は「いま分かっている最新」を写しただけのもの。
func upsertItemState(ctx context.Context, tx *sql.Tx, userID, itemID, eventID string, st domain.ItemState) error {
	const q = `
INSERT INTO item_states
  (item_id, user_id, level, pre_state, needs_review, last_evidence_at, last_event_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (item_id) DO UPDATE SET
  level            = EXCLUDED.level,
  pre_state        = EXCLUDED.pre_state,
  needs_review     = EXCLUDED.needs_review,
  last_evidence_at = EXCLUDED.last_evidence_at,
  last_event_id    = EXCLUDED.last_event_id`

	var lastEvidenceAt any
	if !st.LastEvidenceAt.IsZero() {
		lastEvidenceAt = st.LastEvidenceAt.UTC()
	}
	_, err := tx.ExecContext(ctx, q,
		itemID, userID, int(st.Level), dbPreState(st.PreState), st.NeedsReview, lastEvidenceAt, eventID,
	)
	if err != nil {
		return fmt.Errorf("現在の理解度の更新に失敗しました: %w", err)
	}
	return nil
}
