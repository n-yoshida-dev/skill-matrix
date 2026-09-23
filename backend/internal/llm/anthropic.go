package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/n-yoshida-dev/skill-matrix/internal/config"
)

// このファイルは本物の Claude API を呼ぶ Provider。
//
// 文面の組み立ては prompt.go、返事の読み取りは output.go にある。ここが持つのは
// 「API をどう呼ぶか」と「返ってきたエラーを再試行してよいか」の2つだけ。
//
// 仕様の正本は SPEC.md §4.2（プロンプト構成）・§4.3（出力スキーマ）・§4.6（モデルとコスト）。

const (
	// anthropicMaxTokens は1回の応答で生成を許す最大トークン数。
	//
	// SPEC.md §4.6 の見積もりでは出力は約500トークン。V8 の上限（既定20件）まで判定が並び、
	// 1件ずつ rationale が付いても収まる値として余裕を持たせた。
	// ここで足りずに途中で切れると JSON が壊れ、判定が丸ごと無駄になる（課金は発生する）。
	anthropicMaxTokens = 8192

	// anthropicRequestTimeout は API 呼び出し1回の待ち時間の上限。
	//
	// 判定は数秒〜十数秒で返る想定（SPEC.md §4.1）。SDK の既定は10分で、
	// 応答が返らないときにワーカーが1件に10分占有されてしまうため短くしている。
	anthropicRequestTimeout = 2 * time.Minute
)

// Anthropic は Claude API を呼ぶ Provider。
type Anthropic struct {
	client anthropic.Client
	model  string
}

// NewAnthropic は設定から Claude API クライアントを作る。
//
// API キーが無いまま作れてしまうと、判定を投げた時点で初めて失敗が分かる。
// 起動時に気づけるよう、ここで確認する。
func NewAnthropic(cfg config.LLMConfig) (*Anthropic, error) {
	return newAnthropic(cfg)
}

// newAnthropic はテストから接続先を差し替えられるようにした内部の入口。
// テストでは httptest の偽サーバへ向けて、課金もネットワークも無しで呼び出しを確かめる。
func newAnthropic(cfg config.LLMConfig, opts ...option.RequestOption) (*Anthropic, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("llm: ANTHROPIC_API_KEY が設定されていません")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("llm: ANTHROPIC_MODEL が設定されていません")
	}

	base := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
		option.WithRequestTimeout(anthropicRequestTimeout),
	}
	return &Anthropic{
		client: anthropic.NewClient(append(base, opts...)...),
		model:  cfg.Model,
	}, nil
}

// Judge は Provider の実装。学習ログ1件を Claude に読ませ、判定の提案を受け取る。
//
// 返る値は**検証前の提案**。V1〜V8（domain.ApplyJudgments）を通すまで DB に反映してはいけない。
func (a *Anthropic) Judge(ctx context.Context, req JudgmentRequest) (*JudgmentResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("llm: 判定を開始する前に中断されました: %w", err)
	}
	if err := req.validate(); err != nil {
		return nil, err
	}

	resp, err := a.client.Messages.New(ctx, a.params(req))
	if err != nil {
		return nil, classifyAPIError(err)
	}

	// 生成の打ち切られ方を先に見る。中身を読む前に確かめないと、
	// 途中で切れた JSON を「壊れた出力」と取り違えて原因を見失う。
	switch resp.StopReason {
	case anthropic.StopReasonRefusal:
		return nil, fmt.Errorf("llm: Claude が判定を拒否しました（理由: %s）", resp.StopDetails.Category)
	case anthropic.StopReasonMaxTokens:
		return nil, newInvalidOutputError(nil, "出力が上限（max_tokens=%d）に達して途中で切れました", anthropicMaxTokens)
	}

	raw := textOf(resp)
	if len(raw) == 0 {
		return nil, newInvalidOutputError(nil, "応答に本文がありません")
	}

	out, err := ParseOutput(raw)
	if err != nil {
		return nil, err
	}

	return &JudgmentResult{
		Raw:    raw,
		Output: out,
		Usage: Usage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
		},
		Model: string(resp.Model),
	}, nil
}

// params は API へ送るリクエストを組み立てる。
//
// 並びは SPEC.md §4.2 のとおり。先頭に全ユーザー共通の system を置き、そこにだけ
// キャッシュの印（CacheControl）を付ける。印から後ろ（ロードマップ・現在の状態・ログ本文）は
// 利用者ごとに違うので、キャッシュの対象にしても当たらない。
//
// 注意：共通部が約1,024トークンに満たないモデルでは、**エラーにならず黙ってキャッシュされない**
// （SPEC.md §4.2）。効いているかは resp.Usage.CacheReadInputTokens で確かめる。
func (a *Anthropic) params(req JudgmentRequest) anthropic.MessageNewParams {
	return anthropic.MessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: anthropicMaxTokens,
		System: []anthropic.TextBlockParam{{
			Text:         buildSystemPrompt(req.Levels),
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock(buildRoadmapBlock(req.Roadmap)),
				anthropic.NewTextBlock(buildStateBlock(req)),
				anthropic.NewTextBlock(buildLogBlock(req.LogBody)),
			),
		},
		// 返事の形を JSON Schema で縛る（SPEC.md §4.3）。
		// 前置きの文章が混ざらなくなるだけで、中身の正しさは保証されない。
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: JudgmentOutputSchema()},
		},
		// Thinking は指定しない。モデルは ANTHROPIC_MODEL で差し替えられる想定で、
		// 指定の作法がモデルごとに違う（新しいモデルは既定で適応的に思考する）。
		// 指定しなければどのモデルでも通る。
	}
}

// textOf は応答から本文（JSON のはずの文字列）を取り出す。
// 本文が複数のブロックに分かれて返ることがあるので、順につなげる。
func textOf(resp *anthropic.Message) []byte {
	var b strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			b.WriteString(t.Text)
		}
	}
	return []byte(b.String())
}

// classifyAPIError は API のエラーを「再試行する価値があるか」で仕分ける。
//
// 仕分けないと、ワーカーは鍵の間違い（何度やっても失敗する）にも
// 混雑（時間を置けば成功する）にも同じ対応をすることになる。
// 再試行してよいものにだけ ErrTemporary を付ける。
func classifyAPIError(err error) error {
	// 呼び出し側が中断した場合。ワーカーの停止中などで、やり直す相手がもういない
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("llm: 判定が中断されました: %w", err)
	}
	// 待ち時間の上限に達した場合。相手の混雑の可能性があるので再試行してよい
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: Claude API の応答が %s 以内に返りませんでした: %v",
			ErrTemporary, anthropicRequestTimeout, err)
	}

	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode >= 500:
			// 混雑・相手側の不調。時間を置けば直る
			return fmt.Errorf("%w: Claude API が %d を返しました: %v", ErrTemporary, apiErr.StatusCode, err)
		case apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden:
			// 鍵が違う・権限が無い。再試行しても同じなので、設定を直すよう伝える
			return fmt.Errorf("llm: Claude API の認証に失敗しました（ANTHROPIC_API_KEY を確認してください）: %w", err)
		default:
			// 400 など。こちらの送り方が悪いので再試行しない
			return fmt.Errorf("llm: Claude API の呼び出しに失敗しました: %w", err)
		}
	}

	// 名前解決の失敗や接続断など。相手に届いていないので再試行してよい
	return fmt.Errorf("%w: Claude API に接続できませんでした: %v", ErrTemporary, err)
}
