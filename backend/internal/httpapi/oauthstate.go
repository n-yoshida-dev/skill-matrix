package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// OAuth の state（CSRF 対策）。
//
// 攻撃の形はこうなる。攻撃者が自分の GitHub アカウントで認可コードを取得し、
// そのコードを踏ませる URL を被害者に開かせる。何も対策しないと、被害者のブラウザが
// 攻撃者のアカウントでログインした状態になり、以後の学習ログが攻撃者の手元に残る。
//
// そこでログイン開始時にランダム値を作り、
//   - 認可 URL の state パラメータ
//   - 短命の HttpOnly Cookie
// の両方に入れる。コールバックで両者の一致を確認すれば、
// 自分で始めていないログインを弾ける。
//
// Cookie 側は「乱数 . 有効期限 . 署名」の形にして SESSION_SECRET で署名する。
// 署名があるので、期限を書き換えて古い state を使い回すことができない。

// stateCookieName は state を保存する Cookie の名前。
const stateCookieName = "sm_oauth_state"

// stateTTL は認可画面を往復するのに認める時間。
const stateTTL = 10 * time.Minute

// stateNonceBytes は state の乱数部分の長さ。
const stateNonceBytes = 16

var (
	// errStateMalformed は Cookie の形式が壊れていることを表す。
	errStateMalformed = errors.New("state Cookie の形式が不正です")
	// errStateBadSignature は署名が合わないことを表す（改ざんの疑い）。
	errStateBadSignature = errors.New("state Cookie の署名が一致しません")
	// errStateExpired は有効期限切れを表す。
	errStateExpired = errors.New("state Cookie の有効期限が切れました")
)

// newStateNonce は state の乱数部分を作る。
func newStateNonce() (string, error) {
	buf := make([]byte, stateNonceBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("state の生成に失敗しました: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// signState は「乱数.有効期限.署名」の文字列を組み立てる。
func signState(secret, nonce string, expiresAt time.Time) string {
	payload := nonce + "." + strconv.FormatInt(expiresAt.Unix(), 10)
	return payload + "." + stateSignature(secret, payload)
}

// verifyState は署名と有効期限を検証し、乱数部分を返す。
func verifyState(secret, cookieValue string, now time.Time) (string, error) {
	// 乱数と有効期限には "." が含まれないので、後ろから2つに区切れば足りる
	parts := strings.Split(cookieValue, ".")
	if len(parts) != 3 {
		return "", errStateMalformed
	}
	nonce, expStr, sig := parts[0], parts[1], parts[2]
	if nonce == "" {
		return "", errStateMalformed
	}

	// 署名の比較は hmac.Equal を使う。== は先頭から順に比べるため、
	// 一致する長さが応答時間に出て総当たりの手掛かりになる
	payload := nonce + "." + expStr
	if !hmac.Equal([]byte(sig), []byte(stateSignature(secret, payload))) {
		return "", errStateBadSignature
	}

	// 署名を確かめてから期限を読む。順番を逆にすると、改ざんされた値を解釈することになる
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return "", errStateMalformed
	}
	if !now.Before(time.Unix(exp, 0)) {
		return "", errStateExpired
	}
	return nonce, nil
}

// stateSignature は payload の HMAC-SHA256 を返す。
func stateSignature(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	// hash.Hash の Write は仕様上エラーを返さないため、戻り値を確認しない
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
