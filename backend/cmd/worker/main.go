// skill-matrix の判定ワーカー。
//
// 順番待ちに積まれた判定の仕事を1件ずつ処理する（SPEC.md §4.1）。HTTP サーバとは別プロセス。
//
// 別プロセスにする理由は2つ。判定は数秒〜十数秒かかるので、リクエストの処理と同じ
// プロセスに置くと互いの邪魔になる。もう1つは Cloud Run へ載せるとき、
// リクエスト処理の外では CPU が絞られるため（SPEC.md §4.1）。
//
// 起動には HTTP サーバと同じ環境変数を使う（backend/.env）。
// LLM_PROVIDER=stub なら Claude API を呼ばずに動くので、課金なしで通しの確認ができる。
package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/config"
	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
	"github.com/n-yoshida-dev/skill-matrix/internal/llm"
	"github.com/n-yoshida-dev/skill-matrix/internal/store"
	"github.com/n-yoshida-dev/skill-matrix/internal/worker"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("ワーカーの起動に失敗しました: %v", err)
	}
}

// run はワーカーを組み立てて動かす。main と分けて、エラーを戻り値で扱えるようにしている。
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	// String() が秘密情報を伏せるので、そのままログに出してよい
	logger.Info("設定を読み込みました", "config", cfg.String())

	// Ctrl+C や docker stop を受け取ったらコンテキストが閉じる
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() {
		if err := st.Close(); err != nil {
			logger.Error("DB 接続のクローズに失敗しました", "error", err)
		}
	}()

	provider, err := llm.New(cfg.LLM)
	if err != nil {
		return err
	}

	w := worker.New(st, provider, worker.Config{
		MaxAttempts:  cfg.Judgment.MaxAttempts,
		PollInterval: time.Duration(cfg.Judgment.PollIntervalSeconds) * time.Second,
		Rules: domain.Rules{
			ConfidenceThreshold: cfg.Judgment.ConfidenceThreshold,
			MaxItemsPerLog:      cfg.Judgment.MaxItemsPerLog,
		},
	}, logger)

	logger.Info("判定ワーカーを開始します", "provider", string(cfg.LLM.Provider), "model", cfg.LLM.Model)
	// 停止シグナルを受けたら、処理中の1件を終えてから戻る
	return w.Run(ctx)
}
