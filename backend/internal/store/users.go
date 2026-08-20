package store

import (
	"context"
	"fmt"
)

// User はアプリ側のユーザー（SPEC.md §3.1 の users）。
type User struct {
	ID        string
	GitHubID  int64
	Login     string
	AvatarURL string
	// ApplyMode は判定結果の反映モード 'auto' | 'confirm'（SPEC.md §4.7）
	ApplyMode string
}

// GitHubAccount は GitHub から取得したアカウント情報。
type GitHubAccount struct {
	ID        int64
	Login     string
	AvatarURL string
}

// UpsertUserByGitHub は github_id をキーにユーザーを作成、または既存ユーザーを更新して返す。
//
// login と avatar_url は GitHub 側で変更されうるので、ログインのたびに追随させる。
// apply_mode は本人の設定なので上書きしない。
func (s *Store) UpsertUserByGitHub(ctx context.Context, acct GitHubAccount) (*User, error) {
	const q = `
INSERT INTO users (github_id, login, avatar_url)
VALUES ($1, $2, $3)
ON CONFLICT (github_id) DO UPDATE
   SET login      = EXCLUDED.login,
       avatar_url = EXCLUDED.avatar_url
RETURNING id, github_id, login, coalesce(avatar_url, ''), apply_mode`

	var u User
	err := s.db.QueryRowContext(ctx, q, acct.ID, acct.Login, nullIfEmpty(acct.AvatarURL)).
		Scan(&u.ID, &u.GitHubID, &u.Login, &u.AvatarURL, &u.ApplyMode)
	if err != nil {
		return nil, fmt.Errorf("ユーザーの登録・更新に失敗しました: %w", err)
	}
	return &u, nil
}

// nullIfEmpty は空文字を SQL の NULL に変換する。
// 「未設定」と「空文字」を DB 上で区別するため。
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
