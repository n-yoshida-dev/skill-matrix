// skill-matrix のバックエンド。現時点ではヘルスチェックのみを提供する骨組み。
package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"
)

// defaultPort は PORT が未設定のときに使う待ち受けポート
const defaultPort = "8080"

func main() {
	if err := run(); err != nil {
		log.Fatalf("サーバの起動に失敗しました: %v", err)
	}
}

// run はサーバを組み立てて起動する。main と分けて、エラーを戻り値で扱えるようにしている
func run() error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)

	srv := &http.Server{
		Addr:              ":" + port(),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("listening on %s", srv.Addr)
	// 正常なシャットダウンで返る ErrServerClosed はエラー扱いしない
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// port は待ち受けポートを環境変数から取得する。Cloud Run 等が PORT を注入する前提
func port() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return defaultPort
}

// handleHealth は死活監視用のエンドポイント
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		// レスポンス書き込み済みでステータスは変えられないため、ログに残すだけにする
		log.Printf("ヘルスチェックの応答に失敗しました: %v", err)
	}
}
