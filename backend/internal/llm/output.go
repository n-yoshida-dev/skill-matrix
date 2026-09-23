package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
)

// このファイルは LLM の出力（SPEC.md §4.3 の JSON）を読み取る。
//
// stub も本物の Claude API も、必ずここを通す。偽物だけが通れる近道を作ると、
// 「stub では動いたのに本物では読み取りで落ちた」が起きるため。
//
// ここで見るのは**形**だけ（JSON として読めるか・欄が揃っているか・型が合っているか）。
// **意味**の検査（項目が実在するか・レベルが 0〜5 か・昇格幅は妥当か）は
// domain.ApplyJudgments の V1〜V8 が担当する。検査を2か所に重複させない。

// ProposedJudgment は LLM が返した判定1件。形の検査だけを通った、検証前の提案。
type ProposedJudgment struct {
	ItemKey       string
	ProposedLevel int
	EvidenceType  string
	Rationale     string
	Confidence    float64
}

// RejectedJudgment は形が崩れていて読み取れなかった判定1件。
//
// **握りつぶさない。** 呼び出し側（ワーカー）が llm_responses.violations に記録する。
type RejectedJudgment struct {
	// Index は judgments 配列の中での位置（0 始まり）。
	Index int
	// Raw は読み取れなかった要素そのもの。
	Raw json.RawMessage
	// Reason は弾いた理由。人が読むための文。
	Reason string
}

// Output は LLM の出力全体を読み取ったもの。
type Output struct {
	// Judgments は形の検査を通った判定。LLM が返した順を保つ（V8 の「超過分」を決めるのに順序が要る）。
	Judgments []ProposedJudgment
	// Unmatched はロードマップのどの項目にも対応づけられなかった記述の要約。
	Unmatched []string
	// Rejected は形が崩れていて弾いた判定。
	Rejected []RejectedJudgment
}

// DomainJudgments は domain の検証関数に渡せる形へ変換する。
// occurredAt は根拠が生じた日時（学習ログの日付）。LLM の出力には含まれないので外から与える。
func (o Output) DomainJudgments(occurredAt time.Time) []domain.Judgment {
	js := make([]domain.Judgment, 0, len(o.Judgments))
	for _, p := range o.Judgments {
		js = append(js, domain.Judgment{
			ItemKey:       domain.ItemKey(p.ItemKey),
			ProposedLevel: domain.Level(p.ProposedLevel),
			EvidenceType:  domain.EvidenceType(p.EvidenceType),
			Rationale:     p.Rationale,
			Confidence:    p.Confidence,
			OccurredAt:    occurredAt,
		})
	}
	return js
}

// wireOutput は JSON の外枠。judgments の各要素は1件ずつ読みたいので、まだ解釈せずに持つ。
// 1件の形が崩れているだけで、同じログの他の判定まで巻き添えにしないため。
type wireOutput struct {
	Judgments *[]json.RawMessage `json:"judgments"`
	Unmatched []string           `json:"unmatched"`
}

// wireJudgment は判定1件の JSON。欄が欠けているのか、ゼロ値が入っているのかを
// 区別するためにポインタで受ける（proposedLevel の 0 は正当な値）。
type wireJudgment struct {
	ItemKey       *string  `json:"itemKey"`
	ProposedLevel *int     `json:"proposedLevel"`
	EvidenceType  *string  `json:"evidenceType"`
	Rationale     *string  `json:"rationale"`
	Confidence    *float64 `json:"confidence"`
}

// InvalidOutputError は読み取れなかった出力そのものを運ぶエラー。
//
// errors.Is(err, ErrInvalidOutput) で見分けられ、errors.As で中の Raw を取り出せる。
// 生の出力を運ぶのは、**握りつぶさない**ため。何と返ってきたのかが残らないと、
// プロンプトのどこが悪かったのかを後から調べようがない（SPEC.md §4.5）。
// どこへ保存するかは呼び出し側（ワーカー）が決める。
type InvalidOutputError struct {
	// Raw は LLM が返した生のテキスト。JSON として壊れていることもあるので []byte で持つ。
	Raw []byte
	// Reason は読み取れなかった理由。人が読むための文。
	Reason string
	// Usage はこの失敗で消費したトークン数。
	// **読み取れなくても課金は発生している。** 記録しないと利用量の実績が実際より少なく見える。
	Usage Usage
}

// Error はエラーの文面を返す。生の出力は長くなりうるので、ここには含めない。
func (e *InvalidOutputError) Error() string {
	return fmt.Sprintf("%s: %s", ErrInvalidOutput.Error(), e.Reason)
}

// Unwrap は errors.Is(err, ErrInvalidOutput) を成り立たせる。
func (e *InvalidOutputError) Unwrap() error { return ErrInvalidOutput }

// TokenUsage は失敗したときの消費トークン数を返す（usageCarrier の実装）。
func (e *InvalidOutputError) TokenUsage() Usage { return e.Usage }

// newInvalidOutputError は生の出力を写して持つエラーを作る。
func newInvalidOutputError(raw []byte, format string, args ...any) *InvalidOutputError {
	return &InvalidOutputError{
		Raw:    append([]byte(nil), raw...),
		Reason: fmt.Sprintf(format, args...),
	}
}

// RefusalError は Claude が判定そのものを断ったことを表す。
//
// 形の問題（ErrInvalidOutput）とは別に扱う。直し方が違うため。
// 読み取れないのはプロンプトや上限の問題だが、拒否は依頼の内容の問題。
type RefusalError struct {
	// Category は拒否の分類。API が返す値をそのまま持つ。
	Category string
	// Usage は拒否までに消費したトークン数。**拒否されても課金は発生している。**
	Usage Usage
}

// Error はエラーの文面を返す。
func (e *RefusalError) Error() string {
	if e.Category == "" {
		return "llm: Claude が判定を拒否しました"
	}
	return fmt.Sprintf("llm: Claude が判定を拒否しました（理由: %s）", e.Category)
}

// TokenUsage は拒否までの消費トークン数を返す（usageCarrier の実装）。
func (e *RefusalError) TokenUsage() Usage { return e.Usage }

// usageCarrier は「失敗したが課金は発生した」エラーが満たす約束。
type usageCarrier interface {
	TokenUsage() Usage
}

// UsageOf は失敗したエラーから消費トークン数を取り出す。
//
// 判定が失敗しても API の呼び出し自体は課金されている。呼び出し側（ワーカー）は
// これで消費分を拾って記録する。取り出せない（API に届く前に失敗した）ときは ok が false。
func UsageOf(err error) (Usage, bool) {
	var carrier usageCarrier
	if errors.As(err, &carrier) {
		return carrier.TokenUsage(), true
	}
	return Usage{}, false
}

// ParseOutput は LLM の出力を読み取る。
//
// 外枠が壊れている（JSON でない・judgments が無い）場合は *InvalidOutputError を返す。
// これは errors.Is(err, ErrInvalidOutput) で見分けられる。
// 個々の判定の形が崩れている場合はエラーにせず、その1件だけを Rejected に入れて続ける。
func ParseOutput(raw []byte) (Output, error) {
	var w wireOutput
	if err := json.Unmarshal(raw, &w); err != nil {
		return Output{}, newInvalidOutputError(raw, "JSON として解釈できません: %v", err)
	}
	if w.Judgments == nil {
		return Output{}, newInvalidOutputError(raw, "judgments がありません")
	}

	out := Output{Unmatched: w.Unmatched}
	for i, elem := range *w.Judgments {
		p, reason := parseJudgment(elem)
		if reason != "" {
			out.Rejected = append(out.Rejected, RejectedJudgment{Index: i, Raw: elem, Reason: reason})
			continue
		}
		out.Judgments = append(out.Judgments, p)
	}
	return out, nil
}

// parseJudgment は判定1件を読み取る。読み取れなければ理由を返す（理由が空なら成功）。
func parseJudgment(elem json.RawMessage) (ProposedJudgment, string) {
	var w wireJudgment
	if err := json.Unmarshal(elem, &w); err != nil {
		// proposedLevel が 1.5 や "3" のように整数でない場合もここに来る
		return ProposedJudgment{}, fmt.Sprintf("型が合いません: %v", err)
	}

	var missing []string
	if w.ItemKey == nil {
		missing = append(missing, "itemKey")
	}
	if w.ProposedLevel == nil {
		missing = append(missing, "proposedLevel")
	}
	if w.EvidenceType == nil {
		missing = append(missing, "evidenceType")
	}
	if w.Rationale == nil {
		missing = append(missing, "rationale")
	}
	if w.Confidence == nil {
		missing = append(missing, "confidence")
	}
	if len(missing) > 0 {
		return ProposedJudgment{}, "必須の欄がありません: " + strings.Join(missing, ", ")
	}

	// 理由の無い判定は、後から「なぜこのレベルになったか」を説明できないので受け取らない
	if strings.TrimSpace(*w.Rationale) == "" {
		return ProposedJudgment{}, "rationale が空です"
	}
	// 確信度は 0〜1 の約束。範囲外の値は閾値との比較（V7）が意味を失ううえ、
	// assessment_events.confidence の CHECK 制約にも違反して保存に失敗する
	if *w.Confidence < 0 || *w.Confidence > 1 {
		return ProposedJudgment{}, fmt.Sprintf("confidence が 0〜1 の範囲外です: %g", *w.Confidence)
	}

	return ProposedJudgment{
		ItemKey:       *w.ItemKey,
		ProposedLevel: *w.ProposedLevel,
		EvidenceType:  *w.EvidenceType,
		Rationale:     *w.Rationale,
		Confidence:    *w.Confidence,
	}, ""
}
