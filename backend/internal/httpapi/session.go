package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/store"
)

// sessionCookieName はセッション Cookie の名前。
const sessionCookieName = "sm_session"

// sessionTokenBytes は Cookie に入れる乱数の長さ。
// 32バイト（256ビット）あれば総当たりで当てられない。
const sessionTokenBytes = 32

// touchInterval は最終利用日時を書き戻す最小間隔。
// リクエストのたびに UPDATE すると無駄な書き込みが増えるので間引く。
const touchInterval = time.Hour

// contextKey はコンテキストのキー衝突を避けるための専用型。
type contextKey struct{ name string }

var userContextKey = &contextKey{name: "user"}

// newSessionToken は Cookie に入れる乱数トークンを作る。
func newSessionToken() (string, error) {
	buf := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("セッショントークンの生成に失敗しました: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// hashToken はトークンのハッシュを返す。DB にはこの値だけを保存する（SPEC.md §3.2）。
func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// setSessionCookie はセッション Cookie を発行する。
//
//   - HttpOnly: JavaScript から読めない。XSS でトークンを持ち出されるのを防ぐ
//   - SameSite=Lax: 他サイトからの POST に Cookie が付かない。CSRF を防ぐ。
//     OAuth のコールバック（他サイトからの GET 遷移）では送られるので Strict にはしない
//   - Secure: https のときだけ送る。ローカル開発は http なので development では外す
func (a *API) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie はセッション Cookie を消す。
func (a *API) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

// requireAuth はセッション Cookie からユーザーを特定し、コンテキストに載せる。
// 特定できないときは 401 を返して後続のハンドラを呼ばない。
func (a *API) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := a.authenticate(w, r)
		if !ok {
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, u)))
	})
}

// authenticate はセッションを検証してユーザーを返す。
// 失敗時はこの中でレスポンスを書き終える（戻り値の ok が false）。
func (a *API) authenticate(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil || c.Value == "" {
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "ログインが必要です")
		return nil, false
	}

	now := time.Now()
	hash := hashToken(c.Value)
	u, lastUsedAt, err := a.store.FindSessionUser(r.Context(), hash, now)
	switch {
	case errors.Is(err, store.ErrNotFound):
		// 期限切れか無効なトークン。残っている Cookie を消してから返す
		a.clearSessionCookie(w)
		writeError(w, http.StatusUnauthorized, codeUnauthorized, "ログインの有効期限が切れました")
		return nil, false
	case err != nil:
		log.Printf("セッションの照会に失敗しました: %v", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "サーバ内部エラーです")
		return nil, false
	}

	// 有効期限のスライド延長。書き込みを減らすため間隔をあけて実行する
	if now.Sub(lastUsedAt) >= touchInterval {
		if err := a.store.TouchSession(r.Context(), hash, now); err != nil {
			// 延長に失敗しても認証自体は成立している。記録だけ残して処理は続ける
			log.Printf("セッションの有効期限の延長に失敗しました: %v", err)
		}
	}
	return u, true
}

// userFromContext はコンテキストから認証済みユーザーを取り出す。
func userFromContext(ctx context.Context) (*store.User, bool) {
	u, ok := ctx.Value(userContextKey).(*store.User)
	return u, ok
}
