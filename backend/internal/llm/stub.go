package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
)

// このファイルは LLM を呼ばない Provider（stub）を持つ（SPEC.md §4.6）。
//
// 目的は2つ。
//   - 開発中の課金をゼロにする（開発・テスト中の呼び出しは本番より多くなりやすい）
//   - 同じ入力に必ず同じ判定を返すことで、検証ルール V1〜V8 と集計のテストを安定させる

// StubModel は stub が応答したことを表すモデル名。
// 本物のモデル名と並べて保存されたときに、偽物の判定だと見分けられるようにする。
const StubModel = "stub"

const (
	// stubMaxJudgments は既定の stub が1回の判定で返す件数の上限。
	// 1件だと複数項目にまたがるログの動きを画面で確かめられず、多すぎると1回の投稿で
	// マトリクスが埋まってしまうため、その中間に置いた。
	stubMaxJudgments = 3
	// stubConfidence は既定の stub が返す確信度。
	// 既定の閾値（0.5）を上回る値にして、保留（V7）にならず反映まで進むようにしている。
	stubConfidence = 0.9
)

// promotingEvidence は昇格できる根拠を、到達できるレベルの低い順に並べたもの。
// 各根拠がどのレベルまで届くかは domain 側の表（EvidenceType.MaxLevel）が正本で、ここには書かない。
var promotingEvidence = []domain.EvidenceType{
	domain.EvidenceDrill,
	domain.EvidenceSelfExplanation,
	domain.EvidenceImplementation,
	domain.EvidenceUnaidedImplementation,
	domain.EvidenceCrossContext,
}

// Stub は LLM を呼ばずに判定を返す Provider。
type Stub struct {
	// fixed が空でなければ、入力に関係なくこの JSON をそのまま LLM の出力として返す。
	fixed json.RawMessage
}

// NewStub は既定の stub を返す。
//
// 渡されたロードマップの先頭から、まだ最大レベルに達していない項目を stubMaxJudgments 件まで選び、
// それぞれ「現在のレベル + 1」を提案する。文面を完全に固定しないのは、項目の key が
// ロードマップごとに違い、固定の key では全件が V1（実在しない項目）で弾かれてしまうため。
// 入力から機械的に決まるので、同じ入力には必ず同じ結果を返す。
func NewStub() *Stub {
	return &Stub{}
}

// NewStubWithOutput は、入力に関係なく raw をそのまま LLM の出力として返す stub を作る。
//
// テストで「実在しない項目を返す LLM」「レベル 9 を返す LLM」「壊れた JSON を返す LLM」を
// 演じさせるために使う。raw は本物の出力と同じ読み取り処理（ParseOutput）を通る。
func NewStubWithOutput(raw []byte) *Stub {
	// 呼び出し側が後から raw を書き換えても影響を受けないよう、写しを持つ
	return &Stub{fixed: append(json.RawMessage(nil), raw...)}
}

// Judge は Provider の実装。LLM を呼ばずに判定を組み立てて返す。
func (s *Stub) Judge(ctx context.Context, req JudgmentRequest) (*JudgmentResult, error) {
	// 本物の実装は中断されたコンテキストでエラーを返す。stub も同じふるまいにしておく
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("llm: 判定を開始する前に中断されました: %w", err)
	}
	if err := req.validate(); err != nil {
		return nil, err
	}

	raw := s.fixed
	if len(raw) == 0 {
		generated, err := generateStubOutput(req)
		if err != nil {
			return nil, err
		}
		raw = generated
	}

	out, err := ParseOutput(raw)
	if err != nil {
		return nil, err
	}
	return &JudgmentResult{
		Raw:    raw,
		Output: out,
		Model:  StubModel,
	}, nil
}

// stubWireOutput は stub が組み立てる JSON の形。SPEC.md §4.3 と同じ欄を持つ。
type stubWireOutput struct {
	Judgments []stubWireJudgment `json:"judgments"`
	Unmatched []string           `json:"unmatched"`
}

// stubWireJudgment は stub が組み立てる判定1件。
type stubWireJudgment struct {
	ItemKey       string   `json:"itemKey"`
	ProposedLevel int      `json:"proposedLevel"`
	EvidenceType  string   `json:"evidenceType"`
	Rationale     string   `json:"rationale"`
	EvidenceRefs  []string `json:"evidenceRefs"`
	Confidence    float64  `json:"confidence"`
}

// generateStubOutput は既定の stub の出力を JSON として組み立てる。
// Go の値を直接返さず JSON にするのは、本物と同じ読み取り処理を通すため。
func generateStubOutput(req JudgmentRequest) (json.RawMessage, error) {
	w := stubWireOutput{
		Judgments: []stubWireJudgment{},
		Unmatched: []string{},
	}

	// AllItems は分野の並び順・項目の並び順で返るので、選ばれる項目は毎回同じになる
	for _, it := range req.Roadmap.AllItems() {
		if len(w.Judgments) >= stubMaxJudgments {
			break
		}
		// States に無い項目はゼロ値（レベル 0）として扱う
		cur := req.States[it.Key].VerifiedLevel
		if cur >= domain.MaxLevel {
			continue
		}
		target := cur + 1
		ev, err := evidenceReaching(target)
		if err != nil {
			return nil, err
		}
		w.Judgments = append(w.Judgments, stubWireJudgment{
			ItemKey:       string(it.Key),
			ProposedLevel: int(target),
			EvidenceType:  string(ev),
			Rationale:     fmt.Sprintf("stub による固定の判定（レベル %d → %d）。LLM は呼んでいない", cur, target),
			EvidenceRefs:  []string{"log:stub"},
			Confidence:    stubConfidence,
		})
	}

	raw, err := json.Marshal(w)
	if err != nil {
		return nil, fmt.Errorf("llm: stub の出力を JSON にできません: %w", err)
	}
	return raw, nil
}

// evidenceReaching は target のレベルまで昇格できる根拠のうち、いちばん弱いものを返す。
// いちばん弱いものを選ぶのは、本物の LLM が返しそうな自然な組み合わせ
// （レベル 1 ならドリル、レベル 3 なら実装）に寄せるため。
func evidenceReaching(target domain.Level) (domain.EvidenceType, error) {
	for _, ev := range promotingEvidence {
		if limit, promotes := ev.MaxLevel(); promotes && limit >= target {
			return ev, nil
		}
	}
	return "", fmt.Errorf("llm: レベル %d に届く根拠の種類がありません", target)
}
