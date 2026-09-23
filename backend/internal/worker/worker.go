// Package worker は、順番待ちに積まれた判定の仕事を1件ずつ処理する（SPEC.md §4.1）。
//
// 流れは次のとおり。
//
//	キューから1件取り出す（running にする）
//	  → 材料を読む（ログ本文・ロードマップ・現在の状態）
//	  → Claude に判定させる
//	  → 応答を保存する（読めなかった応答・拒否された応答も残す）
//	  → 検証する（V1〜V8。純粋関数の domain.ApplyJudgments）
//	  → 理解度へ反映する（assessment_events → item_states）
//	  → 仕事を succeeded にする
//
// **DB と LLM は口（インタフェース）越しに使う。** 本物を差せば本番、偽物を差せばテストになり、
// DB も API キーも無しでワーカーの筋道そのものを検証できる。
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
	"github.com/n-yoshida-dev/skill-matrix/internal/llm"
	"github.com/n-yoshida-dev/skill-matrix/internal/store"
)

// Queue はワーカーが必要とする DB 操作の口。実体は *store.Store。
type Queue interface {
	// ClaimJudgmentJob は仕事を1件取り出す。無ければ store.ErrNotFound を返す。
	ClaimJudgmentJob(ctx context.Context, now time.Time) (*store.JudgmentJob, error)
	LoadJudgmentInput(ctx context.Context, job *store.JudgmentJob) (*store.JudgmentInput, error)
	SaveLLMResponse(ctx context.Context, r store.LLMResponse) error
	ApplyJudgmentResult(ctx context.Context, job *store.JudgmentJob, in *store.JudgmentInput, applied []domain.Applied) error
	FinishJob(ctx context.Context, jobID string, now time.Time) error
	FailJob(ctx context.Context, jobID, errText string, now time.Time) error
	RequeueJob(ctx context.Context, jobID, errText string, now time.Time) error
}

// Config はワーカーのふるまいの設定。
type Config struct {
	// MaxAttempts は1つの仕事を試す回数の上限。再試行してよい失敗にだけ効く。
	MaxAttempts int
	// Rules は検証ルールの設定（確信度の閾値・1ログの件数上限）。
	Rules domain.Rules
	// PollInterval は仕事が無かったときに次に見に行くまでの待ち時間。
	PollInterval time.Duration
}

// 既定値。設定が与えられなかったときに使う。
const (
	defaultMaxAttempts  = 3
	defaultPollInterval = 5 * time.Second
)

// withDefaults はゼロ値を既定値で埋める。
func (c Config) withDefaults() Config {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = defaultMaxAttempts
	}
	if c.PollInterval <= 0 {
		c.PollInterval = defaultPollInterval
	}
	return c
}

// Worker は判定ジョブの処理係。
type Worker struct {
	queue    Queue
	provider llm.Provider
	cfg      Config
	log      *slog.Logger
	now      func() time.Time
}

// New はワーカーを作る。
func New(queue Queue, provider llm.Provider, cfg Config, log *slog.Logger) *Worker {
	if log == nil {
		log = slog.Default()
	}
	return &Worker{
		queue:    queue,
		provider: provider,
		cfg:      cfg.withDefaults(),
		log:      log,
		now:      time.Now,
	}
}

// Run は中断されるまで仕事を処理し続ける。
//
// 仕事が無いときは PollInterval だけ待ってから見に行く。
// ctx が終わったら、処理中の1件を終えてから抜ける。
func (w *Worker) Run(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		more, err := w.ProcessOne(ctx)
		switch {
		case err != nil:
			// 1件の失敗でワーカーごと止めない。止めると後続の仕事もすべて滞る
			w.log.Error("判定ジョブの処理に失敗しました", "error", err)
		case more:
			continue // 続けて次の仕事を取りに行く
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(w.cfg.PollInterval):
		}
	}
}

// ProcessOne は仕事を1件だけ処理する。
//
// more は「続けてすぐ次の仕事を取りに行ってよいか」。仕事が無かったときと、
// 再試行のために順番待ちへ戻したときは false になる。戻したものをすぐ取り直すと、
// 待ち時間ゼロで試行回数だけを使い切ってしまい、再試行の意味が無くなるため。
//
// 返すエラーは「ワーカー側の不具合」だけ。判定そのものの失敗（LLM が落ちた・
// 出力が読めなかった）は仕事の状態として記録し、エラーにはしない。
func (w *Worker) ProcessOne(ctx context.Context) (more bool, err error) {
	job, err := w.queue.ClaimJudgmentJob(ctx, w.now())
	if errors.Is(err, store.ErrNotFound) {
		return false, nil // 今は仕事が無い。異常ではない
	}
	if err != nil {
		return false, fmt.Errorf("仕事の取り出しに失敗しました: %w", err)
	}

	log := w.log.With("jobId", job.ID, "attempts", job.Attempts)
	requeued, err := w.run(ctx, job, log)
	if err != nil {
		return false, err
	}
	return !requeued, nil
}

// run は取り出した仕事1件を最後まで処理する。
// requeued が true なら、再試行のために順番待ちへ戻したことを表す。
func (w *Worker) run(ctx context.Context, job *store.JudgmentJob, log *slog.Logger) (requeued bool, err error) {
	in, err := w.queue.LoadJudgmentInput(ctx, job)
	if err != nil {
		// 対象が無い（ログが消された等）のはやり直しても直らないので失敗で閉じる。
		// それ以外（DB の一時的な接続断など）は時間を置けば直るので再試行する。
		// 区別しないと、DB が一瞬詰まっただけで判定が永久に失われる
		reason := fmt.Sprintf("判定の材料を読めません: %v", err)
		if errors.Is(err, store.ErrNotFound) {
			return false, w.fail(ctx, job, reason)
		}
		return w.retryOrFail(ctx, job, reason, log)
	}

	res, judgeErr := w.provider.Judge(ctx, llm.JudgmentRequest{
		Roadmap:        in.Roadmap,
		Levels:         in.Levels,
		States:         in.States,
		RecentEvidence: in.RecentEvidence,
		LogBody:        in.LogBody,
	})
	if judgeErr != nil {
		// 失敗していても API を呼んだ分の課金は発生している。消費分を記録に残す
		if err := w.saveFailedResponse(ctx, job, judgeErr); err != nil {
			log.Error("失敗した応答を保存できませんでした", "error", err)
		}
		if errors.Is(judgeErr, llm.ErrTemporary) {
			return w.retryOrFail(ctx, job, judgeErr.Error(), log)
		}
		return false, w.fail(ctx, job, judgeErr.Error())
	}

	// 検証（V1〜V8）。ここは純粋関数で、DB も LLM も知らない
	batch := domain.ApplyJudgments(in.Roadmap, in.States,
		res.Output.DomainJudgments(in.LoggedAt), w.cfg.Rules)

	// **応答は反映の前に保存する。** 反映に失敗しても「AI が何と言ったか」は残る
	if err := w.queue.SaveLLMResponse(ctx, store.LLMResponse{
		JobID:        job.ID,
		Raw:          res.Raw,
		Violations:   violationsOf(batch, res.Output),
		InputTokens:  res.Usage.InputTokens,
		OutputTokens: res.Usage.OutputTokens,
	}); err != nil {
		return false, w.fail(ctx, job, fmt.Sprintf("応答を保存できません: %v", err))
	}

	if in.ApplyMode == applyModeConfirm {
		// 「確認してから反映」の設定。イベントを積まずに保留として残す（SPEC.md §4.7）。
		// 採否を決める API は後のタスクで作る
		log.Info("判定を保留にしました（apply_mode=confirm）", "judgments", len(batch.Results))
		return false, w.finish(ctx, job)
	}

	if err := w.queue.ApplyJudgmentResult(ctx, job, in, batch.Results); err != nil {
		return false, w.fail(ctx, job, fmt.Sprintf("判定を反映できません: %v", err))
	}

	log.Info("判定を反映しました",
		"applied", countApplied(batch.Results),
		"deferred", countDeferred(batch.Results),
		"violations", len(batch.Violations),
		"rejectedByShape", len(res.Output.Rejected),
		"inputTokens", res.Usage.InputTokens,
		"outputTokens", res.Usage.OutputTokens,
		// キャッシュが効いているかはこの値でしか分からない（効かなくてもエラーは出ない）
		"cacheReadTokens", res.Usage.CacheReadTokens,
	)
	return false, w.finish(ctx, job)
}

// applyModeConfirm は「確認してから反映」の設定値（users.apply_mode）。
const applyModeConfirm = "confirm"

// retryOrFail は一時的な失敗の後始末をする。
//
// 試行回数が上限に達していなければ順番待ちへ戻し、達していれば失敗で閉じる。
//
// **再試行するのは時間を置けば直る失敗だけ。** 出力が読めない・拒否された・鍵が違う失敗は
// 何度やっても同じ結果になり、課金だけが増える（SPEC.md §4.5 も「判定ジョブの失敗として扱う」）。
func (w *Worker) retryOrFail(ctx context.Context, job *store.JudgmentJob, reason string, log *slog.Logger) (bool, error) {
	if job.Attempts >= w.cfg.MaxAttempts {
		log.Warn("再試行の上限に達しました", "maxAttempts", w.cfg.MaxAttempts)
		return false, w.fail(ctx, job, reason)
	}
	log.Warn("一時的な失敗のため順番待ちへ戻します", "reason", reason)
	if err := w.queue.RequeueJob(ctx, job.ID, reason, w.now()); err != nil {
		return false, fmt.Errorf("仕事を順番待ちへ戻せません: %w", err)
	}
	return true, nil
}

// saveFailedResponse は失敗した応答を残す。
//
// 読み取れなかった生の出力（*llm.InvalidOutputError の Raw）と、消費したトークン数を保存する。
// **握りつぶさない。** 何と返ってきたかが残らないと、プロンプトの直しようがない。
func (w *Worker) saveFailedResponse(ctx context.Context, job *store.JudgmentJob, judgeErr error) error {
	usage, billed := llm.UsageOf(judgeErr)
	var outErr *llm.InvalidOutputError
	hasRaw := errors.As(judgeErr, &outErr) && len(outErr.Raw) > 0

	if !billed && !hasRaw {
		// API に届く前の失敗（接続できない・依頼が不正）。残すものが無い
		return nil
	}

	rec := store.LLMResponse{
		JobID:        job.ID,
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		Violations: []store.RecordedViolation{{
			Code:     store.ShapeRejectedCode,
			Detail:   judgeErr.Error(),
			Rejected: true,
		}},
	}
	if hasRaw {
		// JSON として読めるならそのまま raw へ、読めないなら raw_text へ入れる。
		// jsonb の列に壊れた文字列は入らないため、ここで振り分ける
		if json.Valid(outErr.Raw) {
			rec.Raw = append(json.RawMessage(nil), outErr.Raw...)
		} else {
			rec.RawText = string(outErr.Raw)
		}
	} else {
		rec.RawText = judgeErr.Error()
	}
	return w.queue.SaveLLMResponse(ctx, rec)
}

// finish は仕事を成功として閉じる。
func (w *Worker) finish(ctx context.Context, job *store.JudgmentJob) error {
	if err := w.queue.FinishJob(ctx, job.ID, w.now()); err != nil {
		return fmt.Errorf("仕事を完了にできません: %w", err)
	}
	return nil
}

// fail は仕事を失敗として閉じる。原因は必ず残す。
func (w *Worker) fail(ctx context.Context, job *store.JudgmentJob, reason string) error {
	if err := w.queue.FailJob(ctx, job.ID, reason, w.now()); err != nil {
		return fmt.Errorf("仕事を失敗にできません: %w", err)
	}
	w.log.Warn("判定ジョブを失敗として閉じました", "jobId", job.ID, "reason", reason)
	return nil
}

// violationsOf は「意味の検査で弾いたもの」と「形の検査で弾いたもの」を1つの記録にまとめる。
//
// 別々に持つと、後から「AI が何を間違えたか」を数えるたびに2か所を足し合わせることになる。
func violationsOf(batch domain.BatchResult, out llm.Output) []store.RecordedViolation {
	recorded := store.ViolationsOf(batch.Violations)
	for _, r := range out.Rejected {
		index := r.Index
		recorded = append(recorded, store.RecordedViolation{
			Code:     store.ShapeRejectedCode,
			Detail:   r.Reason,
			Rejected: true,
			RawIndex: &index,
			Raw:      r.Raw,
		})
	}
	return recorded
}

// countApplied は実際に反映した件数を数える。
// 棄却（V2・V3）は domain 側で Results から外れているが、数え方をそろえるためここでも除く。
func countApplied(rs []domain.Applied) int {
	n := 0
	for _, r := range rs {
		if !r.Deferred && !r.Rejected {
			n++
		}
	}
	return n
}

// countDeferred は保留にした件数を数える。
func countDeferred(rs []domain.Applied) int {
	return len(rs) - countApplied(rs)
}
