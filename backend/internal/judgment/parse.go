// Package judgment は判定の JSON（SPEC.md §3.3・§4.3）の形を検査する。
//
// v1 の CLI（cmd/skillmatrix）も、棚上げ中の stub と Claude API クライアント（internal/llm）も、必ずここを通す。
// 偽物だけが通れる近道を作ると、「stub では動いたのに本物では読み取りで落ちた」が起きるため。
//
// このパッケージは DB・HTTP・LLM クライアントを import しない。以前は internal/llm/output.go に置いていたが、
// CLI が internal/llm を import すると、同じパッケージにいる棚上げ中の Claude API の SDK と
// 通信ライブラリまで付いてくるため、ここへ切り出した（KNOWLEDGE.md 2026-09-26）。
//
// ここで見るのは**形**だけ（JSON として読めるか・欄が揃っているか・型が合っているか・
// evidenceRefs の書式・confidence の有無が source と合っているか）。
// **意味**の検査（項目が実在するか・レベルが 0〜5 か・梯子の範囲か）は
// domain.ApplyJudgments の V1〜V8 が担当する。検査を2か所に重複させない。
package judgment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
)

// Proposed は AI（または人）が書いた判定1件。形の検査だけを通った、検証前の提案。
type Proposed struct {
	// Index は judgments 配列の中での位置（0 始まり）。形の検査で弾いた判定があると、
	// Output.Judgments の並びの位置とはずれる。state.json には元の位置を書く（SPEC.md §3.4）。
	Index         int
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

// Rejected は形が崩れていて読み取れなかった判定1件。
//
// **握りつぶさない。** 呼び出し側（CLI やワーカー）が state.json の rejected / llm_responses.violations に記録する。
type Rejected struct {
	// Index は judgments 配列の中での位置（0 始まり）。
	Index int
	// Raw は読み取れなかった要素そのもの。
	Raw json.RawMessage
	// Reason は弾いた理由。人が読むための文。
	Reason string
}

// Output は判定の JSON 全体を読み取ったもの。
type Output struct {
	// Judgments は形の検査を通った判定。書かれた順を保つ（V8 の「超過分」を決めるのに順序が要る）。
	Judgments []Proposed
	// Unmatched はロードマップのどの項目にも対応づけられなかった記述の要約。
	Unmatched []string
	// Rejected は形が崩れていて弾いた判定。
	Rejected []Rejected
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

// EnvelopeError は外枠（judgments の配列そのもの）が読めないことを表す。
//
// 個々の判定の崩れ（Rejected）とは違い、どの判定も取り出せないので呼び出し側は止まる。
// internal/llm はこれを生の出力つきの *llm.InvalidOutputError に包み直す。
type EnvelopeError struct {
	// Reason は読み取れなかった理由。人が読むための文。
	Reason string
}

// Error はエラーの文面を返す。
func (e *EnvelopeError) Error() string {
	return "judgment: " + e.Reason
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

// dateLayout は occurredAt / loggedAt の書式（YYYY-MM-DD）。
const dateLayout = "2006-01-02"

// Parse は判定の JSON を、書いた主体（source）に応じた条件で読み取る。
//
// confidence は source が ai なら必須、manual / migration なら存在してはいけない（SPEC.md §3.3）。
// 外枠が壊れている（JSON でない・judgments が無い）場合は *EnvelopeError を返す。
// 個々の判定の形が崩れている場合はエラーにせず、その1件だけを Rejected に入れて続ける。
func Parse(raw []byte, source domain.JudgmentSource) (Output, error) {
	var w wireOutput
	if err := json.Unmarshal(raw, &w); err != nil {
		return Output{}, &EnvelopeError{Reason: fmt.Sprintf("JSON として解釈できません: %v", err)}
	}
	if w.Judgments == nil {
		return Output{}, &EnvelopeError{Reason: "judgments がありません"}
	}

	out := parseJudgments(*w.Judgments, source)
	out.Unmatched = w.Unmatched
	return out, nil
}

// parseJudgments は judgments 配列の要素を1件ずつ読む。形の崩れた要素は Rejected に入れて続ける。
// AI の出力（Parse）と判定ファイル（ParseFile）の両方がここを通る。
func parseJudgments(elems []json.RawMessage, source domain.JudgmentSource) Output {
	var out Output
	for i, elem := range elems {
		p, reason := parseJudgment(elem, source)
		if reason != "" {
			out.Rejected = append(out.Rejected, Rejected{Index: i, Raw: elem, Reason: reason})
			continue
		}
		p.Index = i
		out.Judgments = append(out.Judgments, p)
	}
	return out
}

// parseJudgment は判定1件を読み取る。読み取れなければ理由を返す（理由が空なら成功）。
func parseJudgment(elem json.RawMessage, source domain.JudgmentSource) (Proposed, string) {
	var w wireJudgment
	// 定義にない欄は弾く。`evidenceRef`（単数）のような打ち間違いで根拠が黙って消えるのを防ぐ（SPEC.md §3.3）
	dec := json.NewDecoder(bytes.NewReader(elem))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&w); err != nil {
		// proposedLevel が 1.5 や "3" のように整数でない場合、未知の欄がある場合もここに来る
		return Proposed{}, fmt.Sprintf("型が合いません: %v", err)
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
		return Proposed{}, "必須の欄がありません: " + strings.Join(missing, ", ")
	}

	// 理由の無い判定は、後から「なぜこのレベルになったか」を説明できないので受け取らない
	if strings.TrimSpace(*w.Rationale) == "" {
		return Proposed{}, "rationale が空です"
	}
	// 根拠の出どころは 1 件以上。書式は `<種別>:<識別子>`。種別の列挙はしない
	if len(*w.EvidenceRefs) == 0 {
		return Proposed{}, "evidenceRefs が空です"
	}
	for _, ref := range *w.EvidenceRefs {
		if !evidenceRefPattern.MatchString(ref) {
			return Proposed{}, fmt.Sprintf("evidenceRefs の書式が違います（<種別>:<識別子>）: %q", ref)
		}
	}
	// 確信度は AI の判定だけが持つ。人の訂正や移行の写しに付いていたら、書き間違いか AI の出力の混入
	if source != domain.SourceAI && w.Confidence != nil {
		return Proposed{}, fmt.Sprintf("source が %q の判定に confidence は書けません", source)
	}
	// 確信度は 0〜1 の約束。範囲外の値は閾値との比較（V7）が意味を失う
	if w.Confidence != nil && (*w.Confidence < 0 || *w.Confidence > 1) {
		return Proposed{}, fmt.Sprintf("confidence が 0〜1 の範囲外です: %g", *w.Confidence)
	}

	p := Proposed{
		ItemKey:       *w.ItemKey,
		ProposedLevel: *w.ProposedLevel,
		EvidenceType:  *w.EvidenceType,
		EvidenceRefs:  append([]string(nil), (*w.EvidenceRefs)...),
		Rationale:     *w.Rationale,
		Confidence:    w.Confidence,
	}
	if w.OccurredAt != nil {
		t, err := time.Parse(dateLayout, *w.OccurredAt)
		if err != nil {
			return Proposed{}, fmt.Sprintf("occurredAt は YYYY-MM-DD で書いてください: %q", *w.OccurredAt)
		}
		p.OccurredAt = t
	}
	return p, ""
}
