package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/config"
)

// testAPI は DB を使わない経路のテスト用に API を組み立てる。
// store は nil のまま。DB に触る経路はここでは検査しない。
func testAPI() *API {
	cfg := &config.Config{
		AppEnv:         config.EnvDevelopment,
		Port:           "8080",
		FrontendOrigin: "http://localhost:5173",
		SessionSecret:  testSecret,
		GitHub: config.GitHubConfig{
			ClientID:     "Iv1.dummy",
			ClientSecret: "dummy-secret",
			CallbackURL:  "http://localhost:8080/api/auth/github/callback",
		},
	}
	return &API{cfg: cfg, github: newGitHubOAuth(cfg.GitHub)}
}

// findCookie はレスポンスから指定名の Cookie を探す。
func findCookie(res *http.Response, name string) *http.Cookie {
	for _, c := range res.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestWithCORS_許可したオリジンにだけヘッダを付ける(t *testing.T) {
	api := testAPI()
	handler := api.withCORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		origin     string
		wantOrigin string
	}{
		{"フロントのオリジン", "http://localhost:5173", "http://localhost:5173"},
		{"別のオリジン", "https://evil.example.com", ""},
		{"Origin なし", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			got := rec.Header().Get("Access-Control-Allow-Origin")
			if got != tt.wantOrigin {
				t.Errorf("Allow-Origin = %q, 期待値 %q", got, tt.wantOrigin)
			}
			// キャッシュがオリジンをまたいで応答を使い回さないこと
			if rec.Header().Get("Vary") != "Origin" {
				t.Errorf("Vary = %q, 期待値 Origin", rec.Header().Get("Vary"))
			}
		})
	}
}

func TestWithCORS_プリフライトは後続を呼ばずに終える(t *testing.T) {
	api := testAPI()
	called := false
	handler := api.withCORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/me", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Error("プリフライトで後続のハンドラが呼ばれている")
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("ステータス = %d, 期待値 %d", rec.Code, http.StatusNoContent)
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("Cookie を送るには Allow-Credentials: true が要る")
	}
}

func TestAuthorizeURL_必要なパラメータだけを載せる(t *testing.T) {
	api := testAPI()

	raw := api.github.authorizeURL("nonce-value")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("URL として解釈できない: %v", err)
	}
	if u.Host != "github.com" {
		t.Errorf("ホスト = %q, 期待値 github.com", u.Host)
	}

	q := u.Query()
	if q.Get("client_id") != "Iv1.dummy" {
		t.Errorf("client_id = %q", q.Get("client_id"))
	}
	if q.Get("state") != "nonce-value" {
		t.Errorf("state = %q", q.Get("state"))
	}
	if q.Get("redirect_uri") != api.cfg.GitHub.CallbackURL {
		t.Errorf("redirect_uri = %q", q.Get("redirect_uri"))
	}
	// 必要以上の権限を要求しないこと（公開プロフィールの読み取りだけで足りる）
	if q.Has("scope") {
		t.Errorf("scope を要求している: %q", q.Get("scope"))
	}
	// client_secret は認可 URL に載せてはいけない（ブラウザに渡るため）
	if q.Has("client_secret") {
		t.Error("認可 URL に client_secret が含まれている")
	}
}

func TestHandleGitHubStart_stateをCookieとURLの両方に入れる(t *testing.T) {
	api := testAPI()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/github", nil)
	rec := httptest.NewRecorder()
	api.handleGitHubStart(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("ステータス = %d, 期待値 %d", rec.Code, http.StatusFound)
	}

	res := rec.Result()
	c := findCookie(res, stateCookieName)
	if c == nil {
		t.Fatal("state Cookie が発行されていない")
	}
	if !c.HttpOnly {
		t.Error("state Cookie は HttpOnly であるべき")
	}
	if c.Secure {
		t.Error("development では Secure を落とすべき（ローカルは http のため）")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Error("GitHub からの戻りで送られる必要があるので SameSite=Lax であるべき")
	}

	// Cookie 側の nonce と、リダイレクト先の state が一致すること
	nonce, err := verifyState(api.cfg.SessionSecret, c.Value, time.Now())
	if err != nil {
		t.Fatalf("発行した Cookie を検証できない: %v", err)
	}
	loc, err := url.Parse(res.Header.Get("Location"))
	if err != nil {
		t.Fatalf("Location を解釈できない: %v", err)
	}
	if loc.Query().Get("state") != nonce {
		t.Errorf("URL の state (%q) と Cookie の nonce (%q) が一致しない",
			loc.Query().Get("state"), nonce)
	}
}

func TestHandleGitHubCallback_stateがないと弾く(t *testing.T) {
	api := testAPI()

	// Cookie を付けずにコールバックを叩く＝自分で始めていないログイン
	req := httptest.NewRequest(http.MethodGet, "/api/auth/github/callback?code=abc&state=xyz", nil)
	rec := httptest.NewRecorder()
	api.handleGitHubCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("ステータス = %d, 期待値 %d", rec.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, rec, codeOAuthFailed)
}

func TestHandleGitHubCallback_stateが一致しないと弾く(t *testing.T) {
	api := testAPI()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/github/callback?code=abc&state=attacker", nil)
	req.AddCookie(&http.Cookie{
		Name:  stateCookieName,
		Value: signState(api.cfg.SessionSecret, "mine", time.Now().Add(stateTTL)),
	})
	rec := httptest.NewRecorder()
	api.handleGitHubCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("ステータス = %d, 期待値 %d", rec.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, rec, codeOAuthFailed)
}

func TestHandleGitHubCallback_利用者が拒否したときは400(t *testing.T) {
	api := testAPI()

	req := httptest.NewRequest(http.MethodGet,
		"/api/auth/github/callback?error=access_denied&error_description=denied", nil)
	rec := httptest.NewRecorder()
	api.handleGitHubCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("ステータス = %d, 期待値 %d", rec.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, rec, codeOAuthFailed)
}

func TestRequireAuth_Cookieがなければ401(t *testing.T) {
	api := testAPI()
	called := false
	handler := api.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Error("未認証で後続のハンドラが呼ばれている")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("ステータス = %d, 期待値 %d", rec.Code, http.StatusUnauthorized)
	}
	assertErrorCode(t, rec, codeUnauthorized)
}

func TestClearSessionCookie_即時に失効させる(t *testing.T) {
	api := testAPI()
	rec := httptest.NewRecorder()
	api.clearSessionCookie(rec)

	c := findCookie(rec.Result(), sessionCookieName)
	if c == nil {
		t.Fatal("削除用の Cookie が発行されていない")
	}
	if c.Value != "" {
		t.Errorf("値 = %q, 空であるべき", c.Value)
	}
	if c.MaxAge >= 0 {
		t.Errorf("MaxAge = %d, 負の値で即時削除すべき", c.MaxAge)
	}
}

// assertErrorCode はエラー応答が SPEC.md §6 の形式で、期待したコードを持つことを確かめる。
func assertErrorCode(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, JSON であるべき", ct)
	}
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("応答を JSON として解釈できない: %v (%s)", err, rec.Body.String())
	}
	if body.Error.Code != want {
		t.Errorf("error.code = %q, 期待値 %q", body.Error.Code, want)
	}
	if body.Error.Message == "" {
		t.Error("error.message が空")
	}
}
