package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/n-yoshida-dev/skill-matrix/internal/config"
)

// このテストは Claude API を呼ぶ実装を、**本物の API を呼ばずに**確かめる。
//
// httptest で「Claude API のふりをするローカルのサーバ」を立て、SDK の接続先をそこへ向ける。
// 課金もネットワークも発生せず、結果は毎回同じになる。
// ここで確かめるのは2つ。「送っている中身が SPEC.md §4.2〜§4.3 どおりか」
// 「返ってきたものを正しく読み、エラーを正しく仕分けるか」。

// capturedRequest は偽サーバが受け取ったリクエストを読み取った形。
type capturedRequest struct {
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
	System    []struct {
		Text         string          `json:"text"`
		CacheControl json.RawMessage `json:"cache_control"`
	} `json:"system"`
	Messages []struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"messages"`
	OutputConfig struct {
		Format struct {
			Type   string         `json:"type"`
			Schema map[string]any `json:"schema"`
		} `json:"format"`
	} `json:"output_config"`
}

// userTexts はユーザー側メッセージの本文を順に返す。
func (c capturedRequest) userTexts(t *testing.T) []string {
	t.Helper()
	if len(c.Messages) != 1 {
		t.Fatalf("メッセージは1件のはずが %d 件だった", len(c.Messages))
	}
	out := make([]string, 0, len(c.Messages[0].Content))
	for _, block := range c.Messages[0].Content {
		out = append(out, block.Text)
	}
	return out
}

// successBody は判定が1件返ってきたときの応答。
const successBody = `{
  "id": "msg_01",
  "type": "message",
  "role": "assistant",
  "model": "claude-sonnet-5",
  "content": [{"type": "text", "text": "{\"judgments\":[{\"itemKey\":\"go-01\",\"proposedLevel\":1,\"evidenceType\":\"drill\",\"rationale\":\"確認問題に答えている\",\"confidence\":0.8}],\"unmatched\":[\"型パラメータの話\"]}"}],
  "stop_reason": "end_turn",
  "usage": {"input_tokens": 7500, "output_tokens": 500, "cache_read_input_tokens": 1200, "cache_creation_input_tokens": 0}
}`

// newTestProvider は偽サーバに向いた Provider と、受け取ったリクエストの取り出し口を返す。
// status が 200 以外なら body をそのままエラー応答として返す。
func newTestProvider(t *testing.T, status int, body string) (*Anthropic, func() capturedRequest) {
	t.Helper()

	var raw []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var err error
		if raw, err = io.ReadAll(r.Body); err != nil {
			t.Errorf("リクエストを読めません: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)

	p, err := newAnthropic(
		config.LLMConfig{Provider: config.ProviderAnthropic, APIKey: "dummy", Model: "claude-sonnet-5"},
		option.WithBaseURL(srv.URL+"/"),
		// SDK は 429 や 5xx を既定で2回まで自動再試行する。テストでは待ち時間が無駄なので止める
		option.WithMaxRetries(0),
	)
	if err != nil {
		t.Fatalf("クライアントを作れません: %v", err)
	}

	return p, func() capturedRequest {
		t.Helper()
		var c capturedRequest
		if err := json.Unmarshal(raw, &c); err != nil {
			t.Fatalf("リクエストを読み取れません: %v（%s）", err, raw)
		}
		return c
	}
}

func TestAnthropicJudgeRequest(t *testing.T) {
	p, captured := newTestProvider(t, http.StatusOK, successBody)
	req := sampleRequest(t)

	if _, err := p.Judge(context.Background(), req); err != nil {
		t.Fatalf("判定に失敗した: %v", err)
	}
	got := captured()

	t.Run("モデルは設定から来る（コードに直書きしない）", func(t *testing.T) {
		if got.Model != "claude-sonnet-5" {
			t.Errorf("model が %q だった", got.Model)
		}
	})

	t.Run("共通部は system に置き、キャッシュの印を付ける", func(t *testing.T) {
		if len(got.System) != 1 {
			t.Fatalf("system は1ブロックのはずが %d ブロックだった", len(got.System))
		}
		if len(got.System[0].CacheControl) == 0 {
			t.Error("system にキャッシュの印が付いていない（SPEC.md §4.2）")
		}
		if !strings.Contains(got.System[0].Text, "レベルの定義") {
			t.Error("system に判定の基準が入っていない")
		}
	})

	t.Run("利用者ごとに違う部分は system の後ろに3ブロックで置く", func(t *testing.T) {
		texts := got.userTexts(t)
		if len(texts) != 3 {
			t.Fatalf("ユーザー側は3ブロックのはずが %d ブロックだった", len(texts))
		}
		if !strings.Contains(texts[0], "分野と詳細項目") {
			t.Error("1ブロック目がロードマップ一覧でない")
		}
		if !strings.Contains(texts[1], "現在の状態") {
			t.Error("2ブロック目が現在の状態でない")
		}
		if !strings.Contains(texts[2], logBodyOpen) || !strings.Contains(texts[2], req.LogBody) {
			t.Error("3ブロック目が学習ログ本文でない")
		}
	})

	t.Run("返事の形を JSON Schema で指定する", func(t *testing.T) {
		if got.OutputConfig.Format.Type != "json_schema" {
			t.Errorf("出力形式が %q だった", got.OutputConfig.Format.Type)
		}
		if _, ok := got.OutputConfig.Format.Schema["properties"]; !ok {
			t.Error("スキーマが送られていない")
		}
	})
}

func TestAnthropicJudgeResponse(t *testing.T) {
	p, _ := newTestProvider(t, http.StatusOK, successBody)

	res, err := p.Judge(context.Background(), sampleRequest(t))
	if err != nil {
		t.Fatalf("判定に失敗した: %v", err)
	}

	t.Run("判定を読み取る", func(t *testing.T) {
		if len(res.Output.Judgments) != 1 {
			t.Fatalf("判定は1件のはずが %d 件だった", len(res.Output.Judgments))
		}
		j := res.Output.Judgments[0]
		if j.ItemKey != "go-01" || j.ProposedLevel != 1 || j.EvidenceType != "drill" {
			t.Errorf("判定の中身が違う: %+v", j)
		}
		if len(res.Output.Unmatched) != 1 {
			t.Errorf("unmatched が読めていない: %v", res.Output.Unmatched)
		}
	})

	t.Run("生の出力・モデル名・消費トークンを残す", func(t *testing.T) {
		// あとから「AI が実際に何と言ったか」と、いくら掛かったかを確かめられるようにする
		if !strings.Contains(string(res.Raw), "go-01") {
			t.Errorf("生の出力が残っていない: %s", res.Raw)
		}
		if res.Model != "claude-sonnet-5" {
			t.Errorf("モデル名が %q だった", res.Model)
		}
		if res.Usage.InputTokens != 7500 || res.Usage.OutputTokens != 500 {
			t.Errorf("消費トークンが違う: %+v", res.Usage)
		}
	})

	t.Run("キャッシュが効いたかを確かめられる値を運ぶ", func(t *testing.T) {
		// キャッシュは効かなくてもエラーにならないので、この値でしか確かめようがない
		if res.Usage.CacheReadTokens != 1200 {
			t.Errorf("キャッシュから読めたトークン数が %d だった", res.Usage.CacheReadTokens)
		}
	})
}

func TestAnthropicJudgeInvalidOutput(t *testing.T) {
	t.Run("JSON でない出力は、生の出力ごとエラーに載せる", func(t *testing.T) {
		body := `{"id":"msg_01","type":"message","role":"assistant","model":"claude-sonnet-5",
			"content":[{"type":"text","text":"承知しました。判定は次のとおりです。"}],
			"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`
		p, _ := newTestProvider(t, http.StatusOK, body)

		_, err := p.Judge(context.Background(), sampleRequest(t))
		if !errors.Is(err, ErrInvalidOutput) {
			t.Fatalf("ErrInvalidOutput のはずが %v だった", err)
		}
		var outErr *InvalidOutputError
		if !errors.As(err, &outErr) {
			t.Fatalf("InvalidOutputError を取り出せない: %v", err)
		}
		// 握りつぶさない。何と返ってきたのかが残る（SPEC.md §4.5）
		if !strings.Contains(string(outErr.Raw), "承知しました") {
			t.Errorf("生の出力が運ばれていない: %s", outErr.Raw)
		}
	})

	t.Run("出力が上限で切れたときは、壊れた JSON と取り違えずに報告する", func(t *testing.T) {
		body := `{"id":"msg_01","type":"message","role":"assistant","model":"claude-sonnet-5",
			"content":[{"type":"text","text":"{\"judgments\":[{\"itemKey\":\"go-01\""}],
			"stop_reason":"max_tokens","usage":{"input_tokens":10,"output_tokens":8192}}`
		p, _ := newTestProvider(t, http.StatusOK, body)

		_, err := p.Judge(context.Background(), sampleRequest(t))
		if !errors.Is(err, ErrInvalidOutput) {
			t.Fatalf("ErrInvalidOutput のはずが %v だった", err)
		}
		if !strings.Contains(err.Error(), "max_tokens") {
			t.Errorf("上限で切れたことが分からない文面だった: %v", err)
		}
		// 途中まで生成された本文も捨てない（握りつぶし禁止）
		var outErr *InvalidOutputError
		if !errors.As(err, &outErr) || !strings.Contains(string(outErr.Raw), "go-01") {
			t.Errorf("切れた本文が運ばれていない: %v", err)
		}
	})
}

func TestAnthropicJudgeErrorClassification(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		wantTemporary bool
	}{
		{name: "混雑（429）は時間を置けば直るので再試行してよい", status: http.StatusTooManyRequests, wantTemporary: true},
		{name: "相手側の不調（500）も再試行してよい", status: http.StatusInternalServerError, wantTemporary: true},
		{name: "認証の失敗（401）は何度やっても同じなので再試行しない", status: http.StatusUnauthorized},
		{name: "送り方の誤り（400）も再試行しない", status: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := newTestProvider(t, tt.status, `{"type":"error","error":{"type":"error","message":"boom"}}`)

			_, err := p.Judge(context.Background(), sampleRequest(t))
			if err == nil {
				t.Fatal("エラーになるはずが通った")
			}
			if got := errors.Is(err, ErrTemporary); got != tt.wantTemporary {
				t.Errorf("再試行してよいか＝%v のはずが %v だった: %v", tt.wantTemporary, got, err)
			}
		})
	}
}

func TestAnthropicJudgeRejectsBadRequest(t *testing.T) {
	// 空のログや項目の無いロードマップで API を呼ぶと、課金だけ発生して何も得られない
	p, _ := newTestProvider(t, http.StatusOK, successBody)
	req := sampleRequest(t)
	req.LogBody = "   "

	_, err := p.Judge(context.Background(), req)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("ErrInvalidRequest のはずが %v だった", err)
	}
}

func TestNewAnthropicRequiresSettings(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.LLMConfig
	}{
		{name: "API キーが無い", cfg: config.LLMConfig{Model: "claude-sonnet-5"}},
		{name: "モデルが無い", cfg: config.LLMConfig{APIKey: "dummy"}},
	}

	for _, tt := range tests {
		t.Run(tt.name+"と起動時に気づけるようエラーにする", func(t *testing.T) {
			if _, err := NewAnthropic(tt.cfg); err == nil {
				t.Fatal("エラーになるはずが通った")
			}
		})
	}
}
