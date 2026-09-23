// Package llm は学習ログの判定を LLM に依頼する窓口を持つ。
//
// 呼び出し側（判定ジョブのワーカー）は Provider インタフェースだけを相手にする。
// 裏にいるのが本物の Claude API か、LLM を呼ばない stub かは環境変数 LLM_PROVIDER で決まり、
// 呼び出し側のコードは変わらない（SPEC.md §4.6）。
//
// このパッケージが返すのは**検証前の提案**まで。レベルの範囲や昇格幅の検査（V1〜V8）は
// internal/domain の純粋関数が担当する。ここで見るのは「JSON として読めるか」「必要な欄が
// 揃っているか」という形の検査だけ。
//
// 仕様の正本は SPEC.md §4。
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/n-yoshida-dev/skill-matrix/internal/config"
	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
	"github.com/n-yoshida-dev/skill-matrix/internal/roadmap"
)

// 呼び出し側がエラーの種類で扱いを変えられるようにするための目印。
// errors.Is で見分ける。通信エラー（再試行する価値がある）とは区別したい。
var (
	// ErrInvalidRequest は依頼の中身に不備があることを表す。再試行しても直らない。
	ErrInvalidRequest = errors.New("llm: 判定の依頼に不備があります")
	// ErrInvalidOutput は LLM の出力が約束した形（SPEC.md §4.3）になっていないことを表す。
	ErrInvalidOutput = errors.New("llm: LLM の出力を読み取れません")
	// ErrTemporary は一時的な失敗（混雑・通信断・サーバ側の不調）であることを表す。
	// これが付いているエラーだけを、ワーカーが時間を置いて再試行する。
	// 付いていないエラー（依頼の不備・認証の失敗）は何度やっても同じ結果になる。
	ErrTemporary = errors.New("llm: 一時的な失敗です")
)

// Provider は学習ログの判定を引き受ける相手。
//
// 実装は stub（LLM を呼ばない）と Claude API クライアントの2つになる。
// メソッドは必要になった時点で足す（到達状態の下書き＝SPEC.md §4.8 は、そのタスクで追加する）。
type Provider interface {
	// Judge は学習ログ1件を読み、どの項目がどのレベルに達したかの提案を返す。
	// 返る値は検証前の提案であり、そのまま DB に反映してはいけない。
	Judge(ctx context.Context, req JudgmentRequest) (*JudgmentResult, error)
}

// JudgmentRequest は判定1回ぶんの入力（SPEC.md §4.2 のプロンプトの材料）。
type JudgmentRequest struct {
	// Roadmap は判定対象のロードマップ。LLM に項目一覧（key / name / description）として渡す。
	Roadmap domain.Roadmap
	// Levels はレベルの定義。Criteria が判定基準の正本で、コード側には基準を書かない。
	Levels []roadmap.LevelDef
	// States は項目ごとの現在の状態。まだ判定されたことのない項目は入っていなくてよい。
	States map[domain.ItemKey]domain.ItemState
	// RecentEvidence は項目ごとの直近の根拠（新しい順、SPEC.md §4.2 の3番目）。
	// 過去に何を根拠にレベルが上がったかを見せると、同じ根拠での二重の昇格を避けやすい。
	// 埋めるのは呼び出し側（ワーカー）。空でも判定は成立する。
	RecentEvidence map[domain.ItemKey][]string
	// LogBody は学習ログの本文。
	LogBody string
}

// validate は依頼として成り立っているかを確認する。
// 空の本文や項目の無いロードマップで LLM を呼ぶと、課金だけ発生して何も得られないため。
func (r JudgmentRequest) validate() error {
	var problems []string
	if strings.TrimSpace(r.LogBody) == "" {
		problems = append(problems, "学習ログの本文が空です")
	}
	if len(r.Roadmap.AllItems()) == 0 {
		problems = append(problems, "ロードマップに詳細項目がありません")
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalidRequest, strings.Join(problems, " / "))
	}
	return nil
}

// Usage は判定1回で消費したトークン数。llm_responses に保存してコストの実績を追う。
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// JudgmentResult は判定1回ぶんの結果。
type JudgmentResult struct {
	// Raw は LLM が返した JSON そのもの。llm_responses.raw に保存する。
	// 後から「AI が実際に何と言ったか」を確かめられるよう、加工せずに残す。
	Raw json.RawMessage
	// Output は Raw を読み取ったもの。
	Output Output
	// Usage は消費トークン数。stub では常に 0。
	Usage Usage
	// Model は実際に応答したモデルの名前。stub のときは StubModel。
	Model string
}

// New は設定に応じた Provider を返す。
func New(cfg config.LLMConfig) (Provider, error) {
	switch cfg.Provider {
	case config.ProviderStub:
		return NewStub(), nil
	case config.ProviderAnthropic:
		// 戻り値をそのまま返さないのは Go の落とし穴を避けるため。
		// nil の *Anthropic をインタフェースに入れると「nil でないインタフェース」になり、
		// 呼び出し側の `if p != nil` がすり抜ける。
		p, err := NewAnthropic(cfg)
		if err != nil {
			return nil, err
		}
		return p, nil
	default:
		return nil, fmt.Errorf("llm: 未対応のプロバイダです: %q", cfg.Provider)
	}
}
