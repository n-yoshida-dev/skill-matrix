package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/config"
)

// GitHub の OAuth と REST のエンドポイント。
// デプロイ先や利用者によって変わる値ではなく GitHub 側が固定で決めているものなので、
// 環境変数ではなく定数として置く（可変なのは client_id / secret / callback URL の3つ）。
const (
	githubAuthorizeURL = "https://github.com/login/oauth/authorize"
	githubTokenURL     = "https://github.com/login/oauth/access_token"
	githubUserURL      = "https://api.github.com/user"
)

// githubTimeout は GitHub への1リクエストの制限時間。
// 相手が遅いときにこちらのリクエストを道連れにしないよう、必ず上限を付ける。
const githubTimeout = 10 * time.Second

// maxGitHubResponseBytes は読み込むレスポンスの上限。
// 想定外に巨大な応答でメモリを食い潰さないための歯止め。
const maxGitHubResponseBytes = 1 << 20 // 1MiB

// githubOAuth は GitHub OAuth の呼び出しをまとめる。
type githubOAuth struct {
	cfg    config.GitHubConfig
	client *http.Client
}

// newGitHubOAuth はクライアントを作る。
func newGitHubOAuth(cfg config.GitHubConfig) *githubOAuth {
	return &githubOAuth{
		cfg:    cfg,
		client: &http.Client{Timeout: githubTimeout},
	}
}

// authorizeURL は認可画面の URL を組み立てる。
//
// scope は空にしている。空のときに得られるのは公開プロフィールの読み取りだけで、
// このアプリに必要な id / login / avatar_url はそれで足りる。
// 必要以上の権限を求めないほうが、利用者が許可しやすく、漏れたときの被害も小さい。
func (g *githubOAuth) authorizeURL(state string) string {
	q := url.Values{}
	q.Set("client_id", g.cfg.ClientID)
	q.Set("redirect_uri", g.cfg.CallbackURL)
	q.Set("state", state)
	return githubAuthorizeURL + "?" + q.Encode()
}

// tokenResponse は トークン交換の応答。エラーも同じ 200 で返ってくるため error 系も受ける。
type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// exchangeCode は認可コードをアクセストークンに交換する。
// このトークンは保存しない。直後に fetchAccount を1回呼ぶためだけに使う（SPEC.md §9）。
func (g *githubOAuth) exchangeCode(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", g.cfg.ClientID)
	form.Set("client_secret", g.cfg.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", g.cfg.CallbackURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("トークン交換のリクエスト組み立てに失敗しました: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// 指定しないと GitHub はクエリ文字列形式で返してくる
	req.Header.Set("Accept", "application/json")

	var tr tokenResponse
	if err := g.doJSON(req, &tr); err != nil {
		return "", err
	}
	// GitHub はコードが無効でも HTTP 200 で error を返すので、本文を見て判断する
	if tr.Error != "" {
		return "", fmt.Errorf("GitHub がトークン交換を拒否しました: %s (%s)", tr.Error, tr.ErrorDescription)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("GitHub の応答にアクセストークンが含まれていません")
	}
	return tr.AccessToken, nil
}

// githubUser は GET /user の応答のうち、このアプリが使う項目。
type githubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

// fetchAccount はアクセストークンでログイン中のアカウント情報を取得する。
func (g *githubOAuth) fetchAccount(ctx context.Context, accessToken string) (*githubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserURL, nil)
	if err != nil {
		return nil, fmt.Errorf("ユーザー取得のリクエスト組み立てに失敗しました: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	var u githubUser
	if err := g.doJSON(req, &u); err != nil {
		return nil, err
	}
	if u.ID == 0 || u.Login == "" {
		return nil, fmt.Errorf("GitHub の応答にユーザー情報が含まれていません")
	}
	return &u, nil
}

// doJSON はリクエストを送り、応答の JSON を out に読み込む。
func (g *githubOAuth) doJSON(req *http.Request, out any) error {
	res, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub への通信に失敗しました: %w", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			// 接続の後始末の失敗。処理結果には影響しないので記録だけ残す
			logCloseFailure(err)
		}
	}()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxGitHubResponseBytes))
	if err != nil {
		return fmt.Errorf("GitHub の応答の読み込みに失敗しました: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		// 本文には認可情報が含まれうるので、そのままは載せずステータスだけ返す
		return fmt.Errorf("GitHub が %d を返しました", res.StatusCode)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("GitHub の応答を JSON として解釈できませんでした: %w", err)
	}
	return nil
}
