// Package httpapi は HTTP のハンドラとミドルウェアをまとめる。
//
// 仕様の正本は SPEC.md §6（エンドポイント一覧とエラー形式）。
// 理解度の計算は internal/domain、DB アクセスは internal/store にあり、ここでは行わない。
package httpapi

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/n-yoshida-dev/skill-matrix/internal/config"
	"github.com/n-yoshida-dev/skill-matrix/internal/store"
)

// API はハンドラが必要とする依存をまとめて持つ。
type API struct {
	cfg    *config.Config
	store  *store.Store
	github *githubOAuth
	// roadmaps は store と同じ実体。ロードマップの操作だけインタフェース越しに呼び、
	// テストで DB 無しの偽物に差し替えられるようにしている（roadmaps.go）
	roadmaps roadmapStore
}

// New はルータを組み立てて返す。
func New(cfg *config.Config, st *store.Store) http.Handler {
	api := &API{
		cfg:      cfg,
		store:    st,
		github:   newGitHubOAuth(cfg.GitHub),
		roadmaps: st,
	}

	r := chi.NewRouter()
	r.Use(api.withCORS)

	r.Get("/health", api.handleHealth)

	r.Route("/api", func(r chi.Router) {
		// 認証不要
		r.Get("/auth/github", api.handleGitHubStart)
		r.Get("/auth/github/callback", api.handleGitHubCallback)
		r.Post("/auth/logout", api.handleLogout)

		// 認証が要るもの
		r.Group(func(r chi.Router) {
			r.Use(api.requireAuth)
			r.Get("/me", api.handleMe)
			r.Post("/roadmaps/import", api.handleImportRoadmap)
			r.Get("/roadmaps", api.handleListRoadmaps)
			r.Get("/roadmaps/{id}", api.handleGetRoadmap)
			r.Patch("/roadmaps/{id}", api.handlePatchRoadmap)
			r.Delete("/roadmaps/{id}", api.handleDeleteRoadmap)
		})
	})

	return r
}

// ---------------------------------------------------------------------------
// レスポンスの組み立て
// ---------------------------------------------------------------------------

// errorBody は SPEC.md §6 のエラー形式。
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// エラーコード。フロントが分岐に使うので、文言ではなくこの値で判断させる。
const (
	codeUnauthorized    = "unauthorized"
	codeBadRequest      = "bad_request"
	codeInternal        = "internal_error"
	codeOAuthFailed     = "oauth_failed"
	codeNotFound        = "not_found"         // 存在しない、または自分のものではない（存在の有無は教えない）
	codePayloadTooLarge = "payload_too_large" // リクエストボディが上限を超えた
	codeInvalidRoadmap  = "invalid_roadmap"   // マスタ JSON の検査でエラーがあった（issues を併せて返す）
)

// writeJSON は JSON を書き出す。
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// ヘッダは送信済みでステータスを変えられないため、ログに残すだけにする
		log.Printf("レスポンスの書き込みに失敗しました: %v", err)
	}
}

// writeError は SPEC.md §6 の形式でエラーを返す。
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

// ---------------------------------------------------------------------------
// ミドルウェア
// ---------------------------------------------------------------------------

// withCORS はフロントエンドのオリジンだけを許可する。
// セッション Cookie を送受信するため、許可オリジンはワイルドカードにできない
// （ブラウザは Access-Control-Allow-Origin: * と credentials の併用を認めない）。
func (a *API) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && origin == a.cfg.FrontendOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
		}
		// キャッシュが別オリジン向けの応答を使い回さないようにする
		w.Header().Add("Vary", "Origin")

		// プリフライトはここで終える
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// ハンドラ
// ---------------------------------------------------------------------------

// handleHealth は死活監視用。DB への疎通も含めて確認する。
func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := a.store.Ping(r.Context()); err != nil {
		log.Printf("ヘルスチェックで DB に疎通できません: %v", err)
		writeError(w, http.StatusServiceUnavailable, codeInternal, "データベースに接続できません")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// meResponse は GET /api/me の応答（SPEC.md §6）。
type meResponse struct {
	ID        string `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatarUrl"`
	ApplyMode string `json:"applyMode"`
}

// handleMe はログイン中のユーザーを返す。
func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	u, ok := userFromContext(r.Context())
	if !ok {
		// requireAuth を通っていれば必ず入っている。到達したら組み立ての誤り
		log.Print("handleMe に認証済みユーザーが渡っていません")
		writeError(w, http.StatusInternalServerError, codeInternal, "サーバ内部エラーです")
		return
	}
	writeJSON(w, http.StatusOK, meResponse{
		ID:        u.ID,
		Login:     u.Login,
		AvatarURL: u.AvatarURL,
		ApplyMode: u.ApplyMode,
	})
}
