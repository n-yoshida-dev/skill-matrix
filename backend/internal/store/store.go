// Package store は PostgreSQL へのアクセスをまとめる。
//
// SQL はこのパッケージの外に出さない。ハンドラや理解度モデルが DB の都合を知らずに済むよう、
// 入出力はアプリ側の型（User など）で受け渡す。
//
// 仕様の正本は SPEC.md §3。
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	// database/sql に pgx ドライバを "pgx" という名前で登録する（SPEC.md §8.2）
	_ "github.com/jackc/pgx/v5/stdlib"
)

// 接続プールの設定。個人利用〜小規模を想定した控えめな値。
const (
	maxOpenConns    = 10
	maxIdleConns    = 5
	connMaxLifetime = 30 * time.Minute
	pingTimeout     = 5 * time.Second
)

// ErrNotFound は対象の行が見つからなかったことを表す。
// 呼び出し側が database/sql を import せずに分岐できるよう、このパッケージで定義する。
var ErrNotFound = errors.New("対象が見つかりません")

// Store は DB 接続を保持する。
type Store struct {
	db *sql.DB
}

// Open は接続プールを用意し、実際につながることを確認してから返す。
// sql.Open は接続を張らないため、設定ミスに起動時点で気づけるようここで疎通確認する。
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("DB 接続の初期化に失敗しました: %w", err)
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		// 疎通できないときは開いたプールを閉じてから返す。閉じる側の失敗も握りつぶさない
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("DB へ接続できず、後始末にも失敗しました: %w (close: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("DB へ接続できませんでした: %w", err)
	}
	return &Store{db: db}, nil
}

// Close は接続プールを閉じる。
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("DB 接続のクローズに失敗しました: %w", err)
	}
	return nil
}

// Ping は疎通を確認する。ヘルスチェックから使う。
func (s *Store) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("DB へ疎通できません: %w", err)
	}
	return nil
}
