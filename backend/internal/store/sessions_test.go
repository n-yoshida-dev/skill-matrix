package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

// sessionEpoch はセッションのテストで使う基準時刻。
// 時刻は引数で渡す設計なので、固定値から「31日後」などを作れば実際に待たずに期限切れを再現できる。
// timestamptz の精度（マイクロ秒）で丸められないよう、秒未満は 0 にしている。
var sessionEpoch = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

// newTokenHash は本番と同じ形（乱数の SHA-256 = 32バイト）のダミーのトークンハッシュを返す。
func newTokenHash(t *testing.T) []byte {
	t.Helper()
	sum := sha256.Sum256([]byte(randomHex(t, 32)))
	return sum[:]
}

// createTestSession は user のセッションを now 時点で1件作り、そのトークンハッシュを返す。
func createTestSession(t *testing.T, st *Store, userID string, now time.Time) []byte {
	t.Helper()
	hash := newTokenHash(t)
	if err := st.CreateSession(context.Background(), userID, hash, now); err != nil {
		t.Fatalf("テスト用セッションの作成に失敗: %v", err)
	}
	return hash
}

func TestFindSessionUser_期限内なら持ち主と最終利用日時を返す(t *testing.T) {
	st := testStore(t)
	me := createTestUser(t, st)
	hash := createTestSession(t, st, me.ID, sessionEpoch)

	// 期限（30日後）の1秒前でもまだ引ける
	got, lastUsedAt, err := st.FindSessionUser(context.Background(), hash, sessionEpoch.Add(SessionTTL-time.Second))
	if err != nil {
		t.Fatalf("FindSessionUser が失敗: %v", err)
	}
	if got.ID != me.ID || got.GitHubID != me.GitHubID || got.Login != me.Login {
		t.Errorf("持ち主 = %+v, want %+v", got, me)
	}
	if !lastUsedAt.Equal(sessionEpoch) {
		t.Errorf("lastUsedAt = %v, want %v（作成時刻）", lastUsedAt, sessionEpoch)
	}
}

func TestFindSessionUser_期限切れと存在しないトークンはどちらもErrNotFound(t *testing.T) {
	st := testStore(t)
	me := createTestUser(t, st)
	hash := createTestSession(t, st, me.ID, sessionEpoch)

	cases := []struct {
		name string
		hash []byte
		now  time.Time
	}{
		// expires_at > now で引くので、ちょうど期限の瞬間はもう使えない
		{"期限ちょうど", hash, sessionEpoch.Add(SessionTTL)},
		{"期限の1日後", hash, sessionEpoch.Add(SessionTTL + 24*time.Hour)},
		{"存在しないトークン", newTokenHash(t), sessionEpoch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := st.FindSessionUser(context.Background(), tc.hash, tc.now)
			// 期限切れと存在しないトークンを区別できないこと（攻撃者に情報を与えない）
			if !errors.Is(err, ErrNotFound) {
				t.Errorf("err = %v, want ErrNotFound", err)
			}
			if got != nil {
				t.Errorf("user = %+v, want nil", got)
			}
		})
	}
}

func TestTouchSession_期限が延びて元の期限を過ぎても引ける(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	me := createTestUser(t, st)
	hash := createTestSession(t, st, me.ID, sessionEpoch)

	// 10日後にアクセス → 期限は「10日後 + 30日」になる
	touchedAt := sessionEpoch.Add(10 * 24 * time.Hour)
	if err := st.TouchSession(ctx, hash, touchedAt); err != nil {
		t.Fatalf("TouchSession が失敗: %v", err)
	}

	// 元の期限（30日後）を過ぎた35日後でも引ける
	_, lastUsedAt, err := st.FindSessionUser(ctx, hash, sessionEpoch.Add(35*24*time.Hour))
	if err != nil {
		t.Fatalf("延長後のセッションが引けない: %v", err)
	}
	if !lastUsedAt.Equal(touchedAt) {
		t.Errorf("lastUsedAt = %v, want %v（Touch した時刻）", lastUsedAt, touchedAt)
	}

	// 延長後の期限（40日後）に達したら引けない。無期限になっていないこと
	if _, _, err := st.FindSessionUser(ctx, hash, touchedAt.Add(SessionTTL)); !errors.Is(err, ErrNotFound) {
		t.Errorf("延長後の期限ちょうど: err = %v, want ErrNotFound", err)
	}
}

func TestDeleteSession_消したあとは引けず2回呼んでもエラーにならない(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	me := createTestUser(t, st)
	hash := createTestSession(t, st, me.ID, sessionEpoch)
	other := createTestSession(t, st, me.ID, sessionEpoch) // 別の端末のセッション。巻き込まれてはいけない

	if err := st.DeleteSession(ctx, hash); err != nil {
		t.Fatalf("DeleteSession が失敗: %v", err)
	}
	if _, _, err := st.FindSessionUser(ctx, hash, sessionEpoch); !errors.Is(err, ErrNotFound) {
		t.Errorf("削除後: err = %v, want ErrNotFound", err)
	}
	if _, _, err := st.FindSessionUser(ctx, other, sessionEpoch); err != nil {
		t.Errorf("別のセッションまで消えている: %v", err)
	}

	// ログアウトは何度呼んでも同じ結果でよい
	if err := st.DeleteSession(ctx, hash); err != nil {
		t.Errorf("2回目の DeleteSession: err = %v, want nil", err)
	}
}

func TestDeleteExpiredSessions_期限切れだけを消して件数を返す(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	me := createTestUser(t, st)

	// 期限は作成の30日後。掃除の時刻（基準の30日後）から見て：
	expiredLongAgo := createTestSession(t, st, me.ID, sessionEpoch.Add(-24*time.Hour)) // 1日前に期限切れ
	expiredJustNow := createTestSession(t, st, me.ID, sessionEpoch)                    // ちょうど期限（expires_at <= now で消える）
	alive := createTestSession(t, st, me.ID, sessionEpoch.Add(time.Second))            // あと1秒残っている
	sweepAt := sessionEpoch.Add(SessionTTL)

	n, err := st.DeleteExpiredSessions(ctx, sweepAt)
	if err != nil {
		t.Fatalf("DeleteExpiredSessions が失敗: %v", err)
	}
	if n != 2 {
		t.Errorf("消した件数 = %d, want 2", n)
	}
	for name, hash := range map[string][]byte{"1日前に期限切れ": expiredLongAgo, "ちょうど期限": expiredJustNow} {
		if got := countRows(t, st, "sessions", "token_hash = $1", hash); got != 0 {
			t.Errorf("%s の行が残っている: %d 件", name, got)
		}
	}
	if got := countRows(t, st, "sessions", "token_hash = $1", alive); got != 1 {
		t.Errorf("期限内のセッションの行数 = %d, want 1（消してはいけない）", got)
	}

	// もう消すものが無ければ 0 件
	n, err = st.DeleteExpiredSessions(ctx, sweepAt)
	if err != nil {
		t.Fatalf("2回目の DeleteExpiredSessions が失敗: %v", err)
	}
	if n != 0 {
		t.Errorf("2回目に消した件数 = %d, want 0", n)
	}
}

func TestSessions_ユーザーを消すとそのセッションも消える(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	me := createTestUser(t, st)
	other := createTestUser(t, st)
	mine1 := createTestSession(t, st, me.ID, sessionEpoch)
	mine2 := createTestSession(t, st, me.ID, sessionEpoch)
	others := createTestSession(t, st, other.ID, sessionEpoch)

	// 退会の API はまだ無いので、テストから直接ユーザーを消して ON DELETE CASCADE を確かめる
	if _, err := st.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, me.ID); err != nil {
		t.Fatalf("ユーザーの削除に失敗: %v", err)
	}

	if got := countRows(t, st, "sessions", "user_id = $1", me.ID); got != 0 {
		t.Errorf("消したユーザーのセッションが %d 件残っている", got)
	}
	for _, hash := range [][]byte{mine1, mine2} {
		if _, _, err := st.FindSessionUser(ctx, hash, sessionEpoch); !errors.Is(err, ErrNotFound) {
			t.Errorf("消したユーザーのセッション: err = %v, want ErrNotFound", err)
		}
	}
	// 他人のセッションは巻き込まれない
	got, _, err := st.FindSessionUser(ctx, others, sessionEpoch)
	if err != nil {
		t.Fatalf("他人のセッションが引けない: %v", err)
	}
	if got.ID != other.ID {
		t.Errorf("持ち主 = %s, want %s", got.ID, other.ID)
	}
}

func TestCreateSession_不正な入力は制約で弾かれる(t *testing.T) {
	st := testStore(t)
	me := createTestUser(t, st)

	cases := []struct {
		name   string
		userID string
		hash   []byte
	}{
		// token_hash は CHECK (length = 32)。SHA-256 以外の長さが紛れ込んだら保存させない
		{"ハッシュが32バイトでない", me.ID, newTokenHash(t)[:16]},
		// user_id は users への外部キー
		{"存在しないユーザー", "00000000-0000-0000-0000-000000000000", newTokenHash(t)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := st.CreateSession(context.Background(), tc.userID, tc.hash, sessionEpoch); err == nil {
				t.Error("err = nil, want 制約違反のエラー")
			}
			if got := countRows(t, st, "sessions", "token_hash = $1", tc.hash); got != 0 {
				t.Errorf("弾かれたはずの行が %d 件入っている", got)
			}
		})
	}
}
