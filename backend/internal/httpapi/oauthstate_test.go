package httpapi

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const testSecret = "test-secret-must-be-long-enough-32"

func TestSignState_署名した値を検証できる(t *testing.T) {
	now := time.Now()
	got, err := verifyState(testSecret, signState(testSecret, "abc123", now.Add(stateTTL)), now)
	if err != nil {
		t.Fatalf("検証に失敗した: %v", err)
	}
	if got != "abc123" {
		t.Errorf("nonce = %q, 期待値 %q", got, "abc123")
	}
}

func TestVerifyState_期限が切れていると弾く(t *testing.T) {
	now := time.Now()
	value := signState(testSecret, "abc123", now.Add(-time.Second))

	_, err := verifyState(testSecret, value, now)
	if !errors.Is(err, errStateExpired) {
		t.Errorf("err = %v, 期限切れとして弾いてほしい", err)
	}
}

func TestVerifyState_別の鍵で作られた値を弾く(t *testing.T) {
	now := time.Now()
	value := signState("another-secret-another-secret-32", "abc123", now.Add(stateTTL))

	_, err := verifyState(testSecret, value, now)
	if !errors.Is(err, errStateBadSignature) {
		t.Errorf("err = %v, 署名不一致として弾いてほしい", err)
	}
}

func TestVerifyState_期限を書き換えた値を弾く(t *testing.T) {
	now := time.Now()
	// 期限切れの値を作り、期限部分だけを未来に書き換える（署名はそのまま）
	original := signState(testSecret, "abc123", now.Add(-time.Hour))
	parts := strings.Split(original, ".")
	if len(parts) != 3 {
		t.Fatalf("組み立ての前提が崩れている: %q", original)
	}
	tampered := parts[0] + "." + "99999999999" + "." + parts[2]

	_, err := verifyState(testSecret, tampered, now)
	if !errors.Is(err, errStateBadSignature) {
		t.Errorf("err = %v, 改ざんとして弾いてほしい", err)
	}
}

func TestVerifyState_形式が壊れた値を弾く(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name  string
		value string
	}{
		{"空", ""},
		{"区切りが足りない", "abc123.9999999999"},
		{"区切りが多い", "abc.123.9999999999.sig"},
		{"乱数部分が空", signStateWithEmptyNonce(now.Add(stateTTL))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := verifyState(testSecret, tt.value, now); err == nil {
				t.Errorf("%q はエラーになるはず", tt.value)
			}
		})
	}
}

// signStateWithEmptyNonce は乱数部分が空の値を作る。壊れた入力の検査用。
func signStateWithEmptyNonce(expiresAt time.Time) string {
	return signState(testSecret, "", expiresAt)
}

func TestNewStateNonce_毎回異なる値を返す(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		n, err := newStateNonce()
		if err != nil {
			t.Fatalf("生成に失敗した: %v", err)
		}
		if n == "" {
			t.Fatal("空の nonce が返った")
		}
		if seen[n] {
			t.Fatalf("同じ nonce が2回返った: %q", n)
		}
		seen[n] = true
	}
}

func TestNewSessionToken_毎回異なる値を返す(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		tok, err := newSessionToken()
		if err != nil {
			t.Fatalf("生成に失敗した: %v", err)
		}
		if seen[tok] {
			t.Fatalf("同じトークンが2回返った: %q", tok)
		}
		seen[tok] = true
	}
}

func TestHashToken_同じ入力から同じ32バイトを返す(t *testing.T) {
	h1 := hashToken("token-value")
	h2 := hashToken("token-value")

	if len(h1) != 32 {
		t.Errorf("ハッシュ長 = %d, 期待値 32（migrations の CHECK 制約と一致させる）", len(h1))
	}
	if string(h1) != string(h2) {
		t.Error("同じ入力から同じハッシュが得られていない")
	}
	if string(h1) == string(hashToken("token-value2")) {
		t.Error("異なる入力から同じハッシュが得られている")
	}
	// 生のトークンがハッシュにそのまま残っていないこと
	if strings.Contains(string(h1), "token-value") {
		t.Error("ハッシュに元のトークンが含まれている")
	}
}
