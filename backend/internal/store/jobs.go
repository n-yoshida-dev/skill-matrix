package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
)

// このファイルは LLM を使う仕事の順番待ち（llm_jobs）を扱う（SPEC.md §4.1）。
//
// 専用のキュー製品を使わず DB のテーブルで待たせている。個人〜小規模なら十分で、
// 仕事とその結果（llm_responses・assessment_events）を同じトランザクションで
// 扱えるという利点がある。
//
// **取り出しは FOR UPDATE SKIP LOCKED で行う。** ワーカーを複数動かしたときに
// 同じ仕事を2人が掴んで、同じログを二重に判定し二重に課金されるのを防ぐ。

// JobType は仕事の種類。
type JobType string

const (
	// JobJudgment は学習ログの判定。
	JobJudgment JobType = "judgment"
	// JobOutcomeDraft は到達状態の下書き生成（SPEC.md §4.8。実装は後のタスク）。
	JobOutcomeDraft JobType = "outcome_draft"
)

// JobStatus は仕事の状態。
type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
)

// JudgmentJob はキューから取り出した判定の仕事。
type JudgmentJob struct {
	ID     string
	UserID string
	LogID  string
	// Attempts は取り出した回数（今回を含む）。再試行の上限判定に使う。
	Attempts int
	// Model は積んだ時点で決まっていたモデル ID。
	Model string
}

// JobState は仕事の現在の状態（ジョブ状態 API の材料）。
type JobState struct {
	ID         string
	Type       JobType
	Status     JobStatus
	Attempts   int
	Error      string
	StartedAt  *time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
}

// EnqueueJudgmentJob は学習ログ1件の判定をキューに積む。
//
// ログの持ち主だけが積めるよう、user_id はログから引く。呼び出し側が渡した値を信じない。
func (s *Store) EnqueueJudgmentJob(ctx context.Context, ownerUserID, logID, model string) (string, error) {
	const q = `
INSERT INTO llm_jobs (type, user_id, log_id, model)
SELECT $1, l.user_id, l.id, $2
  FROM learning_logs l
 WHERE l.id = $3 AND l.user_id = $4
RETURNING id`

	var jobID string
	err := s.db.QueryRowContext(ctx, q, string(JobJudgment), model, logID, ownerUserID).Scan(&jobID)
	if errors.Is(err, sql.ErrNoRows) {
		// 他人のログ・存在しないログはどちらも「無い」として扱う（存在を教えない）
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("判定ジョブの登録に失敗しました: %w", err)
	}
	return jobID, nil
}

// ClaimJudgmentJob は順番待ちの先頭から判定の仕事を1件取り、running にして返す。
//
// 仕事が無ければ ErrNotFound を返す。これは異常ではなく「今は暇」という意味なので、
// 呼び出し側はログに出さずに待つ。
func (s *Store) ClaimJudgmentJob(ctx context.Context, now time.Time) (*JudgmentJob, error) {
	// 内側の SELECT で1件を予約し、外側の UPDATE で running にする。
	// SKIP LOCKED があるので、別のワーカーが掴んでいる行は飛ばして次の行を見る
	const q = `
UPDATE llm_jobs
   SET status = $1, attempts = attempts + 1, started_at = $2
 WHERE id = (
   SELECT id
     FROM llm_jobs
    WHERE status = $3 AND type = $4
    ORDER BY created_at
    FOR UPDATE SKIP LOCKED
    LIMIT 1
 )
RETURNING id, user_id, log_id, attempts, model`

	var j JudgmentJob
	err := s.db.QueryRowContext(ctx, q,
		string(JobRunning), now.UTC(), string(JobQueued), string(JobJudgment),
	).Scan(&j.ID, &j.UserID, &j.LogID, &j.Attempts, &j.Model)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("判定ジョブの取り出しに失敗しました: %w", err)
	}
	return &j, nil
}

// FinishJob は仕事を成功として閉じる。
func (s *Store) FinishJob(ctx context.Context, jobID string, now time.Time) error {
	return s.setJobStatus(ctx, jobID, JobSucceeded, "", now)
}

// FailJob は仕事を失敗として閉じる。errText には原因を残す。
//
// **原因を空にしない。** 失敗した事実だけが残っても、後から何も調べられない。
func (s *Store) FailJob(ctx context.Context, jobID, errText string, now time.Time) error {
	if errText == "" {
		errText = "原因不明の失敗"
	}
	return s.setJobStatus(ctx, jobID, JobFailed, errText, now)
}

// RequeueJob は仕事を順番待ちへ戻す。一時的な失敗のあとに再試行するために使う。
//
// errText は「今回はなぜ失敗したか」。次の試行が成功すれば消える。
func (s *Store) RequeueJob(ctx context.Context, jobID, errText string, now time.Time) error {
	const q = `
UPDATE llm_jobs
   SET status = $1, error = $2, started_at = NULL, finished_at = NULL
 WHERE id = $3`

	res, err := s.db.ExecContext(ctx, q, string(JobQueued), nullIfEmpty(errText), jobID)
	if err != nil {
		return fmt.Errorf("判定ジョブの再登録に失敗しました: %w", err)
	}
	return checkOneRow(res, "判定ジョブ")
}

// setJobStatus は仕事を終了状態にする。
func (s *Store) setJobStatus(ctx context.Context, jobID string, status JobStatus, errText string, now time.Time) error {
	const q = `
UPDATE llm_jobs
   SET status = $1, error = $2, finished_at = $3
 WHERE id = $4`

	res, err := s.db.ExecContext(ctx, q, string(status), nullIfEmpty(errText), now.UTC(), jobID)
	if err != nil {
		return fmt.Errorf("判定ジョブの更新に失敗しました: %w", err)
	}
	return checkOneRow(res, "判定ジョブ")
}

// GetJobState は仕事の現在の状態を返す。他人の仕事は ErrNotFound。
func (s *Store) GetJobState(ctx context.Context, ownerUserID, jobID string) (*JobState, error) {
	const q = `
SELECT id, type, status, attempts, coalesce(error, ''), started_at, finished_at, created_at
  FROM llm_jobs
 WHERE id = $1 AND user_id = $2`

	var js JobState
	var startedAt, finishedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, q, jobID, ownerUserID).Scan(
		&js.ID, &js.Type, &js.Status, &js.Attempts, &js.Error, &startedAt, &finishedAt, &js.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("判定ジョブの取得に失敗しました: %w", err)
	}
	js.StartedAt = nullTimePtr(startedAt)
	js.FinishedAt = nullTimePtr(finishedAt)
	return &js, nil
}

// LLMResponse は LLM の応答1件の保存内容（llm_responses）。
//
// 成功した応答も、読み取れなかった応答も、拒否された応答もここに残す。
// **握りつぶさない**（SPEC.md §4.5）。
type LLMResponse struct {
	JobID string
	// Raw は読み取れた判定 JSON。読み取れなかったときは空。
	Raw json.RawMessage
	// RawText は JSON として読み取れなかった生の出力。正常時は空。
	RawText string
	// Violations は検証で弾いた・切り詰めた内容と、形の検査で弾いた判定。
	Violations []RecordedViolation
	// Usage は消費トークン数。失敗した応答でも課金は発生しているので記録する。
	InputTokens  int
	OutputTokens int
}

// RecordedViolation は llm_responses.violations に残す1件。
//
// domain.Violation（意味の検査 V1〜V8）と llm.RejectedJudgment（形の検査）の
// どちらもここに集める。後から「AI が何を間違えたか」を1か所で数えられるようにするため。
type RecordedViolation struct {
	// Code は違反の識別子。形の検査で弾いたものは "shape_rejected"。
	Code string `json:"code"`
	// ItemKey は対象の項目。形の検査で弾いたものは空のことがある。
	ItemKey string `json:"itemKey,omitempty"`
	// Detail は人が読むための説明。
	Detail string `json:"detail"`
	// Proposed / Applied はレベル。形の検査で弾いたものには入らない。
	Proposed *int `json:"proposed,omitempty"`
	Applied  *int `json:"applied,omitempty"`
	// Rejected は丸ごと捨てたか（切り詰めた場合は false）。
	Rejected bool `json:"rejected"`
	// RawIndex は形の検査で弾いた判定の、出力の中での位置。
	RawIndex *int `json:"rawIndex,omitempty"`
	// Raw は形の検査で弾いた判定そのもの。
	Raw json.RawMessage `json:"raw,omitempty"`
}

// ShapeRejectedCode は形の検査（llm.ParseOutput）で弾いた判定に付ける識別子。
// V1〜V8 とは別の層の検査なので、コードも別にして後から区別できるようにする。
const ShapeRejectedCode = "shape_rejected"

// SaveLLMResponse は LLM の応答を保存する。
func (s *Store) SaveLLMResponse(ctx context.Context, r LLMResponse) error {
	violations, err := json.Marshal(nonNilViolations(r.Violations))
	if err != nil {
		return fmt.Errorf("違反の記録を JSON にできません: %w", err)
	}

	const q = `
INSERT INTO llm_responses (job_id, raw, raw_text, violations, input_tokens, output_tokens)
VALUES ($1, $2, $3, $4, $5, $6)`

	var raw any
	if len(r.Raw) > 0 {
		raw = []byte(r.Raw)
	}
	if _, err := s.db.ExecContext(ctx, q,
		r.JobID, raw, nullIfEmpty(r.RawText), violations, r.InputTokens, r.OutputTokens,
	); err != nil {
		return fmt.Errorf("LLM の応答の保存に失敗しました: %w", err)
	}
	return nil
}

// nonNilViolations は nil を空配列にする。JSON にしたとき null ではなく [] にするため。
func nonNilViolations(vs []RecordedViolation) []RecordedViolation {
	if vs == nil {
		return []RecordedViolation{}
	}
	return vs
}

// ViolationsOf は domain の違反を保存用の形に移す。
func ViolationsOf(vs []domain.Violation) []RecordedViolation {
	out := make([]RecordedViolation, 0, len(vs))
	for _, v := range vs {
		proposed, applied := int(v.Proposed), int(v.Applied)
		out = append(out, RecordedViolation{
			Code:     string(v.Code),
			ItemKey:  string(v.ItemKey),
			Detail:   v.Detail,
			Proposed: &proposed,
			Applied:  &applied,
			Rejected: v.Rejected,
		})
	}
	return out
}

// checkOneRow は UPDATE が1行に当たったことを確かめる。
// 0行なら対象が無い（ErrNotFound）。黙って成功にすると、更新できていない仕事を
// 成功したものとして扱ってしまう。
func checkOneRow(res sql.Result, what string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%sの更新件数を取得できません: %w", what, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
