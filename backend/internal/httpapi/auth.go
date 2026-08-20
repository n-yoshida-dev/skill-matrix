package httpapi

import (
	"log"
	"net/http"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/store"
)

// handleGitHubStart は GitHub の認可画面へリダイレクトする（SPEC.md §6）。
// このとき state を作り、URL と Cookie の両方に入れる（oauthstate.go 参照）。
func (a *API) handleGitHubStart(w http.ResponseWriter, r *http.Request) {
	nonce, err := newStateNonce()
	if err != nil {
		log.Printf("state の生成に失敗しました: %v", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "ログインを開始できませんでした")
		return
	}

	expiresAt := time.Now().Add(stateTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    signState(a.cfg.SessionSecret, nonce, expiresAt),
		Path:     "/api/auth",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure(),
		// GitHub からの戻り（他サイトからの GET 遷移）でも送られる必要があるため Lax
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, a.github.authorizeURL(nonce), http.StatusFound)
}

// handleGitHubCallback は GitHub からの戻りを受け、セッションを発行する（SPEC.md §6）。
func (a *API) handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	// 使い終わったかどうかにかかわらず state Cookie は必ず消す。使い回させない
	defer a.clearStateCookie(w)

	q := r.URL.Query()

	// 利用者が認可画面で拒否した場合もここに戻ってくる
	if e := q.Get("error"); e != "" {
		log.Printf("GitHub の認可が完了しませんでした: %s (%s)", e, q.Get("error_description"))
		writeError(w, http.StatusBadRequest, codeOAuthFailed, "GitHub の認可が完了しませんでした")
		return
	}

	code := q.Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, codeBadRequest, "認可コードがありません")
		return
	}

	if !a.verifyStateFromRequest(w, r) {
		return
	}

	ctx := r.Context()

	token, err := a.github.exchangeCode(ctx, code)
	if err != nil {
		log.Printf("トークン交換に失敗しました: %v", err)
		writeError(w, http.StatusBadGateway, codeOAuthFailed, "GitHub との認証に失敗しました")
		return
	}

	acct, err := a.github.fetchAccount(ctx, token)
	if err != nil {
		log.Printf("GitHub のユーザー取得に失敗しました: %v", err)
		writeError(w, http.StatusBadGateway, codeOAuthFailed, "GitHub からユーザー情報を取得できませんでした")
		return
	}

	user, err := a.store.UpsertUserByGitHub(ctx, store.GitHubAccount{
		ID:        acct.ID,
		Login:     acct.Login,
		AvatarURL: acct.AvatarURL,
	})
	if err != nil {
		log.Printf("ユーザーの登録・更新に失敗しました: %v", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "ログイン処理に失敗しました")
		return
	}

	if err := a.issueSession(w, r, user.ID); err != nil {
		log.Printf("セッションの発行に失敗しました: %v", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "ログイン処理に失敗しました")
		return
	}

	// ログインが済んだらフロントエンドへ戻す
	http.Redirect(w, r, a.cfg.FrontendOrigin, http.StatusFound)
}

// handleLogout はセッションを破棄する（SPEC.md §6）。
// Cookie が無い・すでに無効な場合も 204 を返す。何度呼んでも結果は同じでよい。
func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		if err := a.store.DeleteSession(r.Context(), hashToken(c.Value)); err != nil {
			log.Printf("セッションの削除に失敗しました: %v", err)
			writeError(w, http.StatusInternalServerError, codeInternal, "ログアウトに失敗しました")
			return
		}
	}
	a.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// 補助
// ---------------------------------------------------------------------------

// verifyStateFromRequest は state Cookie とクエリの state が一致することを確認する。
// 失敗時はこの中でレスポンスを書き終える。
func (a *API) verifyStateFromRequest(w http.ResponseWriter, r *http.Request) bool {
	c, err := r.Cookie(stateCookieName)
	if err != nil || c.Value == "" {
		log.Print("state Cookie がありません")
		writeError(w, http.StatusBadRequest, codeOAuthFailed, "ログインをやり直してください")
		return false
	}

	nonce, err := verifyState(a.cfg.SessionSecret, c.Value, time.Now())
	if err != nil {
		log.Printf("state の検証に失敗しました: %v", err)
		writeError(w, http.StatusBadRequest, codeOAuthFailed, "ログインをやり直してください")
		return false
	}

	// ここが CSRF 対策の要。自分が始めたログインでなければ一致しない
	if r.URL.Query().Get("state") != nonce {
		log.Print("state が一致しません（CSRF の可能性）")
		writeError(w, http.StatusBadRequest, codeOAuthFailed, "ログインをやり直してください")
		return false
	}
	return true
}

// issueSession はセッションを1件作り、Cookie を発行する。
func (a *API) issueSession(w http.ResponseWriter, r *http.Request, userID string) error {
	token, err := newSessionToken()
	if err != nil {
		return err
	}
	now := time.Now()
	if err := a.store.CreateSession(r.Context(), userID, hashToken(token), now); err != nil {
		return err
	}
	a.setSessionCookie(w, token, now.Add(store.SessionTTL))
	return nil
}

// clearStateCookie は state Cookie を消す。
func (a *API) clearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     "/api/auth",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

// logCloseFailure は後始末の失敗を記録する。処理結果には影響しないが握りつぶさない。
func logCloseFailure(err error) {
	log.Printf("レスポンスの後始末に失敗しました: %v", err)
}
