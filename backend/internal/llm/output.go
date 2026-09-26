package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
)

// このファイルは AI の出力（SPEC.md §4.3 の JSON）を読み取る。
//
// v1 の CLI（判定ファイル）も、棚上げ中の stub と Claude API クライアントも、必ずここを通す。
// 偽物だけが通れる近道を作ると、「stub では動いたのに本物では読み取りで落ちた」が起きるため。
//
// ここで見るのは**形**だけ（JSON として読めるか・欄が揃っているか・型が合っているか・
// evidenceRefs の書式・confidence の有無が source と合っているか）。
// **意味**の検査（項目が実在するか・レベルが 0〜5 か・梯子の範囲か）は
// domain.ApplyJudgments の V1〜V8 が担当する。検査を2か所に重複させない。

// ProposedJudgment は AI（または人）が書いた判定1件。形の検査だけを通った、検証前の提案。
type ProposedJudgment struct {
	ItemKey       string
	ProposedLevel int
	EvidenceType  string
	// EvidenceRefs は根拠の出どころ（`<種別>:<識別子>`）。1 件以上。
	EvidenceRefs []string
	Rationale    string
	// Confidence は source が ai のときだけ入る。それ以外は nil。
	Confidence *float64
	// OccurredAt は根拠が生じた日。省略されていればゼロ値（呼び出し側が loggedAt で補う）。
	OccurredAt time.Time
}

// RejectedJudgment は形が崩れていて読み取れなかった判定1件。
//
// **握りつぶさない。** 呼び出し側（CLI やワーカー）が state.json の rejected / llm_responses.violations に記録する。
type RejectedJudgment struct {
	// Index は judgments 配列の中での位置（0 始まり）。
	Index int
	// Raw は読み取れなかった要素そのもの。
	Raw json.RawMessage
	// Reason は弾いた理由。人が読むための文。
	Reason string
}

// Output は AI の出力全体を読み取ったもの。
type Output struct {
	// Judgments は形の検査を通った判定。返した順を保つ（V8 の「超過分」を決めるのに順序が要る）。
	Judgments []ProposedJudgment
	// Unmatched はロードマップのどの項目にも対応づけられなかった記述の要約。
	Unmatched []string
	// Rejected は形が崩れていて弾いた判定。
	Rejected []RejectedJudgment
}

// DomainJudgments は domain の検証関数に渡せる形へ変換する。
// source は誰が書いた判定か。occurredAt は判定に occurredAt が無いときの既定（判定ファイルの loggedAt）。
func (o Output) DomainJudgments(source domain.JudgmentSource, occurredAt time.Time) []domain.Judgment {
	js := make([]domain.Judgment, 0, len(o.Judgments))
	for _, p := range o.Judgments {
		j := domain.Judgment{
			ItemKey:       domain.ItemKey(p.ItemKey),
			ProposedLevel: domain.Level(p.ProposedLevel),
			EvidenceType:  domain.EvidenceType(p.EvidenceType),
			EvidenceRefs:  append([]string(nil), p.EvidenceRefs...),
			Rationale:     p.Rationale,
			Source:        source,
			OccurredAt:    occurredAt,
		}
		if p.Confidence != nil {
			j.Confidence = *p.Confidence
			j.HasConfidence = true
		}
		if !p.OccurredAt.IsZero() {
			j.OccurredAt = p.OccurredAt
		}
		js = append(js, j)
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
// 区別するためにポインタで受ける（proposedLevel の 0 は正当な値＝不合格の報告）。
type wireJudgment struct {
	ItemKey       *string   `json:"itemKey"`
	ProposedLevel *int      `json:"proposedLevel"`
	EvidenceType  *string   `json:"evidenceType"`
	EvidenceRefs  *[]string `json:"evidenceRefs"`
	Rationale     *string   `json:"rationale"`
	Confidence    *float64  `json:"confidence"`
	OccurredAt    *string   `json:"occurredAt"`
}

// evidenceRefPattern は evidenceRefs の書式（`<種別>:<識別子>`。SPEC.md §3.3）。
// 種別は列挙しない（2026-09-23 決定）。識別子は空でなければよい。
var evidenceRefPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*:.+$`)

// occurredAtLayout は occurredAt の書式（YYYY-MM-DD）。
const occurredAtLayout = "2006-01-02"

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

// ParseOutputFor は判定の JSON を、書いた主体（source）に応じた条件で読み取る。
//
// confidence は source が ai なら必須、manual / migration なら存在してはいけない（SPEC.md §3.3）。
// 外枠が壊れている（JSON でない・judgments が無い）場合は *InvalidOutputError を返す。
// これは errors.Is(err, ErrInvalidOutput) で見分けられる。
// 個々の判定の形が崩れている場合はエラーにせず、その1件だけを Rejected に入れて続ける。
func ParseOutputFor(raw []byte, source domain.JudgmentSource) (Output, error) {
	var w wireOutput
	if err := json.Unmarshal(raw, &w); err != nil {
		return Output{}, newInvalidOutputError(raw, "JSON として解釈できません: %v", err)
	}
	if w.Judgments == nil {
		return Output{}, newInvalidOutputError(raw, "judgments がありません")
	}

	out := Output{Unmatched: w.Unmatched}
	for i, elem := range *w.Judgments {
		p, reason := parseJudgment(elem, source)
		if reason != "" {
			out.Rejected = append(out.Rejected, RejectedJudgment{Index: i, Raw: elem, Reason: reason})
			continue
		}
		out.Judgments = append(out.Judgments, p)
	}
	return out, nil
}

// parseJudgment は判定1件を読み取る。読み取れなければ理由を返す（理由が空なら成功）。
func parseJudgment(elem json.RawMessage, source domain.JudgmentSource) (ProposedJudgment, string) {
	var w wireJudgment
	// 定義にない欄は弾く。`evidenceRef`（単数）のような打ち間違いで根拠が黙って消えるのを防ぐ（SPEC.md §3.3）
	dec := json.NewDecoder(bytes.NewReader(elem))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&w); err != nil {
		// proposedLevel が 1.5 や "3" のように整数でない場合、未知の欄がある場合もここに来る
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
	if w.EvidenceRefs == nil {
		missing = append(missing, "evidenceRefs")
	}
	if w.Rationale == nil {
		missing = append(missing, "rationale")
	}
	if source == domain.SourceAI && w.Confidence == nil {
		missing = append(missing, "confidence")
	}
	if len(missing) > 0 {
		return ProposedJudgment{}, "必須の欄がありません: " + strings.Join(missing, ", ")
	}

	// 理由の無い判定は、後から「なぜこのレベルになったか」を説明できないので受け取らない
	if strings.TrimSpace(*w.Rationale) == "" {
		return ProposedJudgment{}, "rationale が空です"
	}
	// 根拠の出どころは 1 件以上。書式は `<種別>:<識別子>`。種別の列挙はしない
	if len(*w.EvidenceRefs) == 0 {
		return ProposedJudgment{}, "evidenceRefs が空です"
	}
	for _, ref := range *w.EvidenceRefs {
		if !evidenceRefPattern.MatchString(ref) {
			return ProposedJudgment{}, fmt.Sprintf("evidenceRefs の書式が違います（<種別>:<識別子>）: %q", ref)
		}
	}
	// 確信度は AI の判定だけが持つ。人の訂正や移行の写しに付いていたら、書き間違いか AI の出力の混入
	if source != domain.SourceAI && w.Confidence != nil {
		return ProposedJudgment{}, fmt.Sprintf("source が %q の判定に confidence は書けません", source)
	}
	// 確信度は 0〜1 の約束。範囲外の値は閾値との比較（V7）が意味を失う
	if w.Confidence != nil && (*w.Confidence < 0 || *w.Confidence > 1) {
		return ProposedJudgment{}, fmt.Sprintf("confidence が 0〜1 の範囲外です: %g", *w.Confidence)
	}

	p := ProposedJudgment{
		ItemKey:       *w.ItemKey,
		ProposedLevel: *w.ProposedLevel,
		EvidenceType:  *w.EvidenceType,
		EvidenceRefs:  append([]string(nil), (*w.EvidenceRefs)...),
		Rationale:     *w.Rationale,
		Confidence:    w.Confidence,
	}
	if w.OccurredAt != nil {
		t, err := time.Parse(occurredAtLayout, *w.OccurredAt)
		if err != nil {
			return ProposedJudgment{}, fmt.Sprintf("occurredAt は YYYY-MM-DD で書いてください: %q", *w.OccurredAt)
		}
		p.OccurredAt = t
	}
	return p, ""
}
