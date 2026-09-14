package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/n-yoshida-dev/skill-matrix/internal/roadmap"
	"github.com/n-yoshida-dev/skill-matrix/internal/store"
)

// maxImportBodyBytes はインポートのリクエストボディの上限（SPEC.md §6）。
//
// 1フィールドの文字数上限（internal/roadmap）とは役割が違う。こちらは「巨大なボディを
// 丸ごとメモリに読み込ませない」ための粗いガードで、JSON として解釈する前に効く。
// 500 項目 × 数 KB でも収まる大きさにしてある。
const maxImportBodyBytes = 2 << 20 // 2 MiB

// roadmapStore はロードマップのハンドラが store に求める操作。
//
// *store.Store をそのまま持たず、必要なメソッドだけをインタフェースにしておくと、
// テストでは DB を使わない偽物に差し替えられる（Spring で Repository をモックするのと同じ考え方）。
type roadmapStore interface {
	ImportRoadmap(ctx context.Context, ownerUserID, kind string, doc *roadmap.Document) (string, error)
	ListRoadmaps(ctx context.Context, ownerUserID string) ([]store.RoadmapSummary, error)
	GetRoadmap(ctx context.Context, ownerUserID, roadmapID string) (*store.Roadmap, error)
	UpdateRoadmap(ctx context.Context, ownerUserID, roadmapID string, upd store.RoadmapUpdate) (*store.RoadmapSummary, error)
	DeleteRoadmap(ctx context.Context, ownerUserID, roadmapID string) error
}

// importResponse は POST /api/roadmaps/import の 201 応答（SPEC.md §6）。
// Issues にはインポートを止めなかった警告だけが入る。
type importResponse struct {
	ID     string          `json:"id"`
	Issues []roadmap.Issue `json:"issues"`
}

// importErrorBody は検査エラーで止めたときの 400 応答（SPEC.md §6）。
// 共通のエラー形式（error）に加え、問題の一覧（issues）をそのまま載せる。
// フロントは issues[].path で JSON のどこが悪いかを指せる。
type importErrorBody struct {
	Error  errorDetail     `json:"error"`
	Issues []roadmap.Issue `json:"issues"`
}

// handleImportRoadmap はマスタ JSON を受け取り、自分の personal ロードマップを作る（SPEC.md §6）。
//
// リクエストボディはマスタ JSON そのもの（何かで包まない）。流れは3段。
//
//  1. ボディを上限までで読む。超えていれば 413
//  2. 読み込みと検査（roadmap.ParseAndValidate）。エラーが1件でもあれば 400 で全問題を返す
//  3. 保存（store.ImportRoadmap）。警告があっても止めず、201 の応答に載せる
func (a *API) handleImportRoadmap(w http.ResponseWriter, r *http.Request) {
	u, ok := userFromContext(r.Context())
	if !ok {
		log.Print("handleImportRoadmap に認証済みユーザーが渡っていません")
		writeError(w, http.StatusInternalServerError, codeInternal, "サーバ内部エラーです")
		return
	}

	body, err := readLimitedBody(w, r, maxImportBodyBytes)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, codePayloadTooLarge,
				fmt.Sprintf("リクエストボディは %d バイトまでです", maxImportBodyBytes))
			return
		}
		log.Printf("リクエストボディの読み込みに失敗しました: %v", err)
		writeError(w, http.StatusBadRequest, codeBadRequest, "リクエストボディを読み込めませんでした")
		return
	}

	doc, res := roadmap.ParseAndValidate(body)
	if !res.OK() {
		writeJSON(w, http.StatusBadRequest, importErrorBody{
			Error: errorDetail{
				Code:    codeInvalidRoadmap,
				Message: fmt.Sprintf("ロードマップに %d 件のエラーがあります", len(res.Errors())),
			},
			Issues: nonNilIssues(res.Issues),
		})
		return
	}

	id, err := a.roadmaps.ImportRoadmap(r.Context(), u.ID, store.RoadmapKindPersonal, doc)
	if err != nil {
		log.Printf("ロードマップの保存に失敗しました: %v", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "ロードマップを保存できませんでした")
		return
	}

	writeJSON(w, http.StatusCreated, importResponse{
		ID:     id,
		Issues: nonNilIssues(res.Warnings()),
	})
}

// readLimitedBody はボディを上限バイト数までで読む。
// 上限を超えると *http.MaxBytesError が返る。あわせて接続を閉じる印を付け、
// 残りを読み捨てずに済むようにしている（MaxBytesReader の標準動作）。
func readLimitedBody(w http.ResponseWriter, r *http.Request, limit int64) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	return io.ReadAll(r.Body)
}

// nonNilIssues は nil のスライスを空のスライスに寄せる。
// JSON にしたとき null ではなく [] にするため（フロントが length を読んで落ちないように）。
func nonNilIssues(issues []roadmap.Issue) []roadmap.Issue {
	if issues == nil {
		return []roadmap.Issue{}
	}
	return issues
}
