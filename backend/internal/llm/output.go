package llm

import (
	"errors"
	"fmt"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
	"github.com/n-yoshida-dev/skill-matrix/internal/judgment"
)

// このファイルは AI の出力（SPEC.md §4.3 の JSON）を読み取る入口。
//
// 形の検査そのものは internal/judgment にある（v1 の CLI が LLM クライアントを import せずに使えるよう、
// 2026-09-26 に切り出した。KNOWLEDGE.md 2026-09-26）。ここに残るのは、棚上げ中の stub と Claude API
// クライアントのための「生の出力と消費トークン数を運ぶエラー」だけ。
// stub も本物のクライアントも ParseOutput を通るので、判定ファイルを読む CLI と同じ検査を受ける。

// ProposedJudgment は形の検査だけを通った、検証前の判定1件（judgment.Proposed の別名）。
type ProposedJudgment = judgment.Proposed

// RejectedJudgment は形が崩れていて読み取れなかった判定1件（judgment.Rejected の別名）。
type RejectedJudgment = judgment.Rejected

// Output は AI の出力全体を読み取ったもの（judgment.Output の別名）。
type Output = judgment.Output

// InvalidOutputError は読み取れなかった出力そのものを運ぶエラー。
//
// errors.Is(err, ErrInvalidOutput) で見分けられ、errors.As で中の Raw を取り出せる。
// 生の出力を運ぶのは、**握りつぶさない**ため。何と返ってきたのかが残らないと、
// 指示書のどこが悪かったのかを後から調べようがない（SPEC.md §4.5）。
// どこへ保存するかは呼び出し側が決める。
type InvalidOutputError struct {
	// Raw は AI が返した生のテキスト。JSON として壊れていることもあるので []byte で持つ。
	Raw []byte
	// Reason は読み取れなかった理由。人が読むための文。
	Reason string
	// Usage はこの失敗で消費したトークン数（Claude API 経由のときだけ意味を持つ）。
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

// RefusalError は Claude が判定そのものを断ったことを表す（棚上げ中の Claude API クライアント用）。
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

// ParseOutput は AI の出力を読み取る。source は ai として扱う（confidence 必須）。
func ParseOutput(raw []byte) (Output, error) {
	return ParseOutputFor(raw, domain.SourceAI)
}

// ParseOutputFor は判定の JSON を、書いた主体（source）に応じた条件で読み取る（judgment.Parse への入口）。
//
// 外枠が壊れている（JSON でない・judgments が無い）場合は、生の出力つきの *InvalidOutputError を返す。
// これは errors.Is(err, ErrInvalidOutput) で見分けられる。
// 個々の判定の形が崩れている場合はエラーにせず、その1件だけを Rejected に入れて続ける。
func ParseOutputFor(raw []byte, source domain.JudgmentSource) (Output, error) {
	out, err := judgment.Parse(raw, source)
	if err != nil {
		var envErr *judgment.EnvelopeError
		if errors.As(err, &envErr) {
			return Output{}, newInvalidOutputError(raw, "%s", envErr.Reason)
		}
		return Output{}, newInvalidOutputError(raw, "%v", err)
	}
	return out, nil
}
