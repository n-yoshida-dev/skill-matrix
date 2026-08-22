package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// このファイルは DB を使うテストの土台。
//
// 純粋関数と違い、SQL の綴り・制約違反・トランザクションの巻き戻しは本物の PostgreSQL が
// 無いと確かめられない。そこで次の形にしている。
//
//   - TEST_DATABASE_URL が無ければスキップする。`go test ./...` が DB 無しでも赤くならない
//   - あれば、テストごとに専用スキーマ（test_xxxx）を作り、migrations/*.up.sql を順に流す。
//     終わったらスキーマごと捨てる。手元の DB に何が入っていても結果が変わらない
//
// ローカルでの実行方法（docker compose up -d postgres のあと）：
//
//	TEST_DATABASE_URL='postgres://skillmatrix:skillmatrix@localhost:5432/skillmatrix?sslmode=disable' \
//	  go -C backend test ./internal/store/ -v

// migrationsDir はテストから見たマイグレーションの置き場所。
const migrationsDir = "../../migrations"

// testStore は専用スキーマにマイグレーションを流した Store を返す。
// 後始末（接続のクローズとスキーマの削除）は t.Cleanup で行う。
func testStore(t *testing.T) *Store {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL が未設定のため DB テストをスキップします")
	}
	ctx := context.Background()

	// スキーマの作成・削除には search_path を触らない管理用の接続を使う
	admin, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatalf("管理用の DB 接続に失敗: %v", err)
	}
	schema := "test_" + randomHex(t, 8)
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("テスト用スキーマの作成に失敗: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("テスト用スキーマの削除に失敗: %v", err)
		}
		if err := admin.Close(); err != nil {
			t.Errorf("管理用の DB 接続のクローズに失敗: %v", err)
		}
	})

	// テスト対象の Store は search_path を専用スキーマに向けて開く。
	// 接続文字列に書く代わりに pgx の設定を登録し、その名前で Open する
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatalf("TEST_DATABASE_URL の解釈に失敗: %v", err)
	}
	cfg.RuntimeParams["search_path"] = schema
	dsn := stdlib.RegisterConnConfig(cfg)
	t.Cleanup(func() { stdlib.UnregisterConnConfig(dsn) })

	st, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("テスト用の Store を開けません: %v", err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("Store のクローズに失敗: %v", err)
		}
	})

	applyMigrations(t, st)
	return st
}

// applyMigrations は migrations/*.up.sql をファイル名順に流す。
// 本番と同じ SQL を使うことで、テストが「別のスキーマ定義」で通ってしまうのを防ぐ。
func applyMigrations(t *testing.T, st *Store) {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		t.Fatalf("マイグレーションの一覧取得に失敗: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("マイグレーションが見つかりません: %s", migrationsDir)
	}
	// 000001, 000002, ... の連番で並ぶようにする
	sort.Strings(files)

	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("マイグレーションの読み込みに失敗 %s: %v", f, err)
		}
		// 引数なしの Exec は複数文をまとめて実行できる（pgx の simple protocol）
		if _, err := st.db.ExecContext(context.Background(), string(body)); err != nil {
			t.Fatalf("マイグレーションの適用に失敗 %s: %v", filepath.Base(f), err)
		}
	}
}

// randomHex は n バイトの乱数を16進文字列で返す。スキーマ名や github_id の衝突を避けるため。
func randomHex(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("乱数の生成に失敗: %v", err)
	}
	return hex.EncodeToString(b)
}

// createTestUser はダミーのユーザーを1人作って返す。
func createTestUser(t *testing.T, st *Store) *User {
	t.Helper()
	var id int64
	for _, b := range []byte(randomHex(t, 4)) {
		id = id*31 + int64(b)
	}
	u, err := st.UpsertUserByGitHub(context.Background(), GitHubAccount{
		ID:    id,
		Login: fmt.Sprintf("dummy-%d", id),
	})
	if err != nil {
		t.Fatalf("テスト用ユーザーの作成に失敗: %v", err)
	}
	return u
}

// countRows は where 条件に合う行数を返す。検証用。
func countRows(t *testing.T, st *Store, table, where string, args ...any) int {
	t.Helper()
	var n int
	q := "SELECT count(*) FROM " + table + " WHERE " + where
	if err := st.db.QueryRowContext(context.Background(), q, args...).Scan(&n); err != nil {
		t.Fatalf("%s の件数取得に失敗: %v", table, err)
	}
	return n
}
