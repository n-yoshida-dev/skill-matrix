// skill-matrix のバックエンド（HTTP サーバ）。
//
// エンドポイントの一覧は SPEC.md §6。判定ジョブのワーカーは別プロセス（cmd/worker）。
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/config"
	"github.com/n-yoshida-dev/skill-matrix/internal/httpapi"
	"github.com/n-yoshida-dev/skill-matrix/internal/store"
)

// shutdownTimeout は停止シグナルを受けてから処理中のリクエストを待つ上限。
const shutdownTimeout = 15 * time.Second

func main() {
	if err := run(); err != nil {
		log.Fatalf("サーバの起動に失敗しました: %v", err)
	}
}

// run はサーバを組み立てて起動する。main と分けて、エラーを戻り値で扱えるようにしている
func run() error {
	// 設定は起動時に読み切って検証する。設定ミスはリクエストが来る前に気づきたい
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// String() が秘密情報を伏せるので、そのままログに出してよい
	log.Printf("設定を読み込みました: %v", cfg)

	// Ctrl+C や docker stop を受け取ったらコンテキストが閉じる
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() {
		if err := st.Close(); err != nil {
			log.Printf("DB 接続のクローズに失敗しました: %v", err)
		}
	}()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.New(cfg, st),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// ListenAndServe は停止するまで戻らないので、別ゴルーチンで動かして
	// メインは停止シグナルを待つ
	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", srv.Addr)
		// 正常なシャットダウンで返る ErrServerClosed はエラー扱いしない
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Print("停止シグナルを受け取りました。処理中のリクエストの完了を待ちます")
	}

	// 処理中のリクエストを打ち切らずに閉じる
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return <-errCh
}
