package store

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/roadmap"
)

// loadSampleDocument は testdata/roadmap-sample.json を読み込んで検査を通す。
func loadSampleDocument(t *testing.T) *roadmap.Document {
	t.Helper()
	data, err := os.ReadFile("../../testdata/roadmap-sample.json")
	if err != nil {
		t.Fatalf("サンプルの読み込みに失敗: %v", err)
	}
	doc, res := roadmap.ParseAndValidate(data)
	if !res.OK() {
		t.Fatalf("サンプルが検査を通りません: %+v", res.Errors())
	}
	return doc
}

// minimalDocument は検査を通る最小のロードマップ（manual・出典なし・outcome なし）を返す。
func minimalDocument() *roadmap.Document {
	return &roadmap.Document{
		SchemaVersion: roadmap.SchemaVersion,
		Name:          "最小のロードマップ",
		Levels: []roadmap.LevelDef{
			{Level: 1, Name: "L1", Criteria: "c1"},
			{Level: 2, Name: "L2", Criteria: "c2"},
			{Level: 3, Name: "L3", Criteria: "c3"},
			{Level: 4, Name: "L4", Criteria: "c4"},
			{Level: 5, Name: "L5", Criteria: "c5"},
		},
		Domains: []roadmap.DomainDef{
			{Key: "a", Name: "A", Items: []roadmap.ItemDef{
				{Key: "a-01", Name: "A-01"},
			}},
		},
	}
}

func TestImportRoadmap_サンプルを全行保存する(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	user := createTestUser(t, st)
	doc := loadSampleDocument(t)

	id, err := st.ImportRoadmap(ctx, user.ID, RoadmapKindPersonal, doc)
	if err != nil {
		t.Fatalf("ImportRoadmap が失敗: %v", err)
	}
	if id == "" {
		t.Fatal("roadmaps.id が返っていません")
	}

	// 件数：roadmaps 1、domains 3、items 11（サンプルの内容と一致）
	if n := countRows(t, st, "roadmaps", "id = $1", id); n != 1 {
		t.Errorf("roadmaps の件数 = %d, want 1", n)
	}
	if n := countRows(t, st, "domains", "roadmap_id = $1", id); n != len(doc.Domains) {
		t.Errorf("domains の件数 = %d, want %d", n, len(doc.Domains))
	}
	if n := countRows(t, st, "items", "roadmap_id = $1", id); n != doc.TotalItems() {
		t.Errorf("items の件数 = %d, want %d", n, doc.TotalItems())
	}

	// roadmaps の列：JSON の値がそのまま入り、visibility / version は既定値になる
	var (
		kind, owner, name, origin, source, visibility, levelsJSON string
		checkedAt                                                 time.Time
		version                                                   int
	)
	err = st.db.QueryRowContext(ctx, `
SELECT kind, owner_user_id, name, origin, source, checked_at, visibility, version, levels::text
  FROM roadmaps WHERE id = $1`, id).
		Scan(&kind, &owner, &name, &origin, &source, &checkedAt, &visibility, &version, &levelsJSON)
	if err != nil {
		t.Fatalf("roadmaps の読み出しに失敗: %v", err)
	}
	if kind != RoadmapKindPersonal || owner != user.ID || name != doc.Name {
		t.Errorf("roadmaps = (%s, %s, %s), want (%s, %s, %s)", kind, owner, name, RoadmapKindPersonal, user.ID, doc.Name)
	}
	if origin != string(doc.Origin) || source != doc.Source {
		t.Errorf("出典 = (%s, %s), want (%s, %s)", origin, source, doc.Origin, doc.Source)
	}
	if got := checkedAt.Format(time.DateOnly); got != doc.CheckedAt {
		t.Errorf("checked_at = %s, want %s", got, doc.CheckedAt)
	}
	if visibility != "private" || version != 1 {
		t.Errorf("visibility/version = (%s, %d), want (private, 1)", visibility, version)
	}

	// levels は jsonb に丸ごと入り、読み戻すと元の LevelDef と一致する
	var levels []roadmap.LevelDef
	if err := json.Unmarshal([]byte(levelsJSON), &levels); err != nil {
		t.Fatalf("levels の JSON が読めません: %v", err)
	}
	if !reflect.DeepEqual(levels, doc.Levels) {
		t.Errorf("levels = %+v, want %+v", levels, doc.Levels)
	}

	// domains は JSON の並び順が order_index に入る
	rows, err := st.db.QueryContext(ctx, `SELECT key FROM domains WHERE roadmap_id = $1 ORDER BY order_index`, id)
	if err != nil {
		t.Fatalf("domains の読み出しに失敗: %v", err)
	}
	defer rows.Close()
	var gotKeys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatalf("domains の Scan に失敗: %v", err)
		}
		gotKeys = append(gotKeys, k)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("domains の走査に失敗: %v", err)
	}
	wantKeys := make([]string, 0, len(doc.Domains))
	for _, d := range doc.Domains {
		wantKeys = append(wantKeys, d.Key)
	}
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Errorf("domains の並び = %v, want %v", gotKeys, wantKeys)
	}

	// items：依存先は key の文字列のまま、outcome があれば outcome_source='authored'、
	// 分野内の並び順が order_index に入る
	var (
		deps          []string
		outcomeSource string
		orderIndex    int
		domainKey     string
	)
	err = st.db.QueryRowContext(ctx, `
SELECT i.depends_on_keys, i.outcome_source, i.order_index, d.key
  FROM items i JOIN domains d ON d.id = i.domain_id
 WHERE i.roadmap_id = $1 AND i.key = 'go-04'`, id).
		Scan(textArrayScanner(&deps), &outcomeSource, &orderIndex, &domainKey)
	if err != nil {
		t.Fatalf("items の読み出しに失敗: %v", err)
	}
	if want := []string{"go-02", "go-03"}; !reflect.DeepEqual(deps, want) {
		t.Errorf("go-04 の depends_on_keys = %v, want %v", deps, want)
	}
	if outcomeSource != "authored" {
		t.Errorf("go-04 の outcome_source = %q, want authored", outcomeSource)
	}
	if orderIndex != 3 || domainKey != "go" {
		t.Errorf("go-04 の位置 = (%s, %d), want (go, 3)", domainKey, orderIndex)
	}
}

func TestImportRoadmap_任意項目が無ければNULLになる(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	user := createTestUser(t, st)

	id, err := st.ImportRoadmap(ctx, user.ID, RoadmapKindTemplate, minimalDocument())
	if err != nil {
		t.Fatalf("ImportRoadmap が失敗: %v", err)
	}

	// origin を省略したら manual。出典・確認日・説明は NULL
	var origin string
	var description, source, checkedAt any
	err = st.db.QueryRowContext(ctx,
		`SELECT origin, description, source, checked_at FROM roadmaps WHERE id = $1`, id).
		Scan(&origin, &description, &source, &checkedAt)
	if err != nil {
		t.Fatalf("roadmaps の読み出しに失敗: %v", err)
	}
	if origin != string(roadmap.OriginManual) {
		t.Errorf("origin = %q, want manual", origin)
	}
	if description != nil || source != nil || checkedAt != nil {
		t.Errorf("description/source/checked_at = (%v, %v, %v), want すべて NULL", description, source, checkedAt)
	}

	// outcome が無い項目は outcome も outcome_source も NULL。依存が無ければ空配列（NULL ではない）
	var outcome, outcomeSource any
	var deps []string
	err = st.db.QueryRowContext(ctx,
		`SELECT outcome, outcome_source, depends_on_keys FROM items WHERE roadmap_id = $1 AND key = 'a-01'`, id).
		Scan(&outcome, &outcomeSource, textArrayScanner(&deps))
	if err != nil {
		t.Fatalf("items の読み出しに失敗: %v", err)
	}
	if outcome != nil || outcomeSource != nil {
		t.Errorf("outcome/outcome_source = (%v, %v), want 両方 NULL", outcome, outcomeSource)
	}
	if deps == nil || len(deps) != 0 {
		t.Errorf("depends_on_keys = %v, want 空配列", deps)
	}
}

func TestImportRoadmap_途中で失敗したら何も残らない(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	user := createTestUser(t, st)

	// 2つ目の分野で item の key を重複させる。検査を通さずに渡すと items の UNIQUE 制約で落ちる。
	// そのとき、先に入れた roadmaps / domains / 1つ目の item まで巻き戻ることを確かめる
	doc := minimalDocument()
	doc.Domains = append(doc.Domains, roadmap.DomainDef{
		Key: "b", Name: "B", Items: []roadmap.ItemDef{{Key: "a-01", Name: "重複"}},
	})

	_, err := st.ImportRoadmap(ctx, user.ID, RoadmapKindPersonal, doc)
	if err == nil {
		t.Fatal("重複 key なのに成功しています")
	}
	for _, table := range []string{"roadmaps", "domains", "items"} {
		where := "roadmap_id IN (SELECT id FROM roadmaps WHERE owner_user_id = $1)"
		if table == "roadmaps" {
			where = "owner_user_id = $1"
		}
		if n := countRows(t, st, table, where, user.ID); n != 0 {
			t.Errorf("%s に %d 行残っています。巻き戻っていません", table, n)
		}
	}
}

func TestImportRoadmap_不正な入力はDBに触らず弾く(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	user := createTestUser(t, st)

	if _, err := st.ImportRoadmap(ctx, user.ID, RoadmapKindPersonal, nil); err == nil {
		t.Error("nil の Document が通っています")
	}
	if _, err := st.ImportRoadmap(ctx, user.ID, "public", minimalDocument()); err == nil {
		t.Error("不正な kind が通っています")
	}
	if n := countRows(t, st, "roadmaps", "owner_user_id = $1", user.ID); n != 0 {
		t.Errorf("roadmaps に %d 行あります。何も作られないはず", n)
	}
}

func TestImportRoadmap_ユーザー削除で木ごと消える(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	user := createTestUser(t, st)

	id, err := st.ImportRoadmap(ctx, user.ID, RoadmapKindPersonal, loadSampleDocument(t))
	if err != nil {
		t.Fatalf("ImportRoadmap が失敗: %v", err)
	}
	if _, err := st.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, user.ID); err != nil {
		t.Fatalf("ユーザーの削除に失敗: %v", err)
	}
	for _, table := range []string{"roadmaps", "domains", "items"} {
		where := "roadmap_id = $1"
		if table == "roadmaps" {
			where = "id = $1"
		}
		if n := countRows(t, st, table, where, id); n != 0 {
			t.Errorf("%s に %d 行残っています。cascade していません", table, n)
		}
	}
}
