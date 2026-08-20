package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SessionTTL はセッションの有効期限。アクセスのたびに延長する（SPEC.md §9）。
const SessionTTL = 30 * 24 * time.Hour

// CreateSession はセッションを1件作る。
// tokenHash は Cookie に入れた乱数の SHA-256。生のトークンは受け取らない。
func (s *Store) CreateSession(ctx context.Context, userID string, tokenHash []byte, now time.Time) error {
	const q = `
INSERT INTO sessions (token_hash, user_id, expires_at, last_used_at, created_at)
VALUES ($1, $2, $3, $4, $4)`

	if _, err := s.db.ExecContext(ctx, q, tokenHash, userID, now.Add(SessionTTL), now); err != nil {
		return fmt.Errorf("セッションの作成に失敗しました: %w", err)
	}
	return nil
}

// FindSessionUser は期限内のセッションの持ち主を返す。
// あわせて last_used_at を返し、期限を延長すべきかを呼び出し側が判断できるようにしている。
// 見つからない・期限切れのときは ErrNotFound を返す。
func (s *Store) FindSessionUser(ctx context.Context, tokenHash []byte, now time.Time) (*User, time.Time, error) {
	const q = `
SELECT u.id, u.github_id, u.login, coalesce(u.avatar_url, ''), u.apply_mode, s.last_used_at
  FROM sessions s
  JOIN users u ON u.id = s.user_id
 WHERE s.token_hash = $1
   AND s.expires_at > $2`

	var u User
	var lastUsedAt time.Time
	err := s.db.QueryRowContext(ctx, q, tokenHash, now).
		Scan(&u.ID, &u.GitHubID, &u.Login, &u.AvatarURL, &u.ApplyMode, &lastUsedAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// 期限切れと存在しないトークンを区別しない。攻撃者に情報を与えないため
		return nil, time.Time{}, ErrNotFound
	case err != nil:
		return nil, time.Time{}, fmt.Errorf("セッションの照会に失敗しました: %w", err)
	}
	return &u, lastUsedAt, nil
}

// TouchSession は最終利用日時を更新し、有効期限をそこから TTL 分だけ延長する。
// リクエストのたびに書き込むのは無駄なので、呼ぶ間隔は呼び出し側で間引く。
func (s *Store) TouchSession(ctx context.Context, tokenHash []byte, now time.Time) error {
	const q = `
UPDATE sessions
   SET last_used_at = $2,
       expires_at   = $3
 WHERE token_hash = $1`

	if _, err := s.db.ExecContext(ctx, q, tokenHash, now, now.Add(SessionTTL)); err != nil {
		return fmt.Errorf("セッションの更新に失敗しました: %w", err)
	}
	return nil
}

// DeleteSession はセッションを1件消す（ログアウト）。
// 存在しないトークンでもエラーにしない。ログアウトは何度呼んでも同じ結果でよい。
func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash); err != nil {
		return fmt.Errorf("セッションの削除に失敗しました: %w", err)
	}
	return nil
}

// DeleteExpiredSessions は期限切れのセッションを掃除し、消した件数を返す。
func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, fmt.Errorf("期限切れセッションの削除に失敗しました: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		// 削除自体は成功しているので、件数が取れないことはエラーにしない
		return 0, nil
	}
	return n, nil
}
