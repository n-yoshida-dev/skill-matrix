package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

// importSample はサンプルのロードマップを user の personal として保存し、id を返す。
func importSample(t *testing.T, st *Store, userID string) string {
	t.Helper()
	id, err := st.ImportRoadmap(context.Background(), userID, RoadmapKindPersonal, loadSampleDocument(t))
	if err != nil {
		t.Fatalf("サンプルの保存に失敗: %v", err)
	}
	return id
}

func TestListRoadmaps_自分のpersonalだけを返す(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	me := createTestUser(t, st)
	other := createTestUser(t, st)

	mine := importSample(t, st, me.ID)
	importSample(t, st, other.ID) // 他人のもの。一覧に出てはいけない
	// 自分のものでも template は出ない（v2 の公開側。進捗は紐づかない）
	if _, err := st.ImportRoadmap(ctx, me.ID, RoadmapKindTemplate, minimalDocument()); err != nil {
		t.Fatalf("template の保存に失敗: %v", err)
	}

	got, err := st.ListRoadmaps(ctx, me.ID)
	if err != nil {
		t.Fatalf("ListRoadmaps が失敗: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("件数 = %d, want 1: %+v", len(got), got)
	}
	if got[0].ID != mine || got[0].Kind != RoadmapKindPersonal {
		t.Errorf("一覧 = %+v, want id=%s kind=personal", got[0], mine)
	}
	if got[0].ItemCount != 11 {
		t.Errorf("itemCount = %d, want 11（サンプルの項目数）", got[0].ItemCount)
	}
	if got[0].CheckedAt == nil || got[0].CheckedAt.Format(time.DateOnly) != "2026-08-14" {
		t.Errorf("checkedAt = %v, want 2026-08-14", got[0].CheckedAt)
	}
	if got[0].TargetDate != nil {
		t.Errorf("targetDate = %v, want nil（未設定）", got[0].TargetDate)
	}
}

func TestListRoadmaps_1件も無ければ空スライス(t *testing.T) {
	st := testStore(t)
	me := createTestUser(t, st)

	got, err := st.ListRoadmaps(context.Background(), me.ID)
	if err != nil {
		t.Fatalf("ListRoadmaps が失敗: %v", err)
	}
	// JSON にしたとき null ではなく [] になるように、nil ではない空スライスであること
	if got == nil || len(got) != 0 {
		t.Errorf("got = %#v, want 空スライス", got)
	}
}

func TestGetRoadmap_分野と項目を順序どおりに返す(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	me := createTestUser(t, st)
	doc := loadSampleDocument(t)
	id := importSample(t, st, me.ID)

	got, err := st.GetRoadmap(ctx, me.ID, id)
	if err != nil {
		t.Fatalf("GetRoadmap が失敗: %v", err)
	}
	if got.Name != doc.Name || got.Origin != string(doc.Origin) || got.Source != doc.Source {
		t.Errorf("見出し = (%s, %s, %s), want (%s, %s, %s)", got.Name, got.Origin, got.Source, doc.Name, doc.Origin, doc.Source)
	}
	if !reflect.DeepEqual(got.Levels, doc.Levels) {
		t.Errorf("levels = %+v, want %+v", got.Levels, doc.Levels)
	}

	// 分野は JSON の順、各分野の項目も JSON の順
	if len(got.Domains) != len(doc.Domains) {
		t.Fatalf("分野数 = %d, want %d", len(got.Domains), len(doc.Domains))
	}
	for di, dom := range doc.Domains {
		g := got.Domains[di]
		if g.Key != dom.Key || g.Name != dom.Name || g.Goal != dom.Goal {
			t.Errorf("domains[%d] = (%s, %s, %s), want (%s, %s, %s)", di, g.Key, g.Name, g.Goal, dom.Key, dom.Name, dom.Goal)
		}
		if len(g.Items) != len(dom.Items) {
			t.Fatalf("domains[%d] の項目数 = %d, want %d", di, len(g.Items), len(dom.Items))
		}
		for ii, it := range dom.Items {
			gi := g.Items[ii]
			if gi.Key != it.Key || gi.Name != it.Name || gi.Outcome != it.Outcome || gi.VerifyBy != it.VerifyBy {
				t.Errorf("domains[%d].items[%d] = %+v, want %+v", di, ii, gi, it)
			}
			// 依存は key のまま。無ければ空スライス（nil ではない）
			wantDeps := it.DependsOn
			if wantDeps == nil {
				wantDeps = []string{}
			}
			if !reflect.DeepEqual(gi.DependsOn, wantDeps) {
				t.Errorf("%s の dependsOn = %#v, want %#v", it.Key, gi.DependsOn, wantDeps)
			}
			if it.Outcome != "" && gi.OutcomeSource != "authored" {
				t.Errorf("%s の outcomeSource = %q, want authored", it.Key, gi.OutcomeSource)
			}
		}
	}
}

func TestGetRoadmap_他人のものはNotFound(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	me := createTestUser(t, st)
	other := createTestUser(t, st)
	theirs := importSample(t, st, other.ID)

	if _, err := st.GetRoadmap(ctx, me.ID, theirs); !errors.Is(err, ErrNotFound) {
		t.Errorf("他人のロードマップを取得: err = %v, want ErrNotFound", err)
	}
	// 存在しない id も同じ扱い。存在の有無を漏らさない
	if _, err := st.GetRoadmap(ctx, me.ID, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, ErrNotFound) {
		t.Errorf("存在しない id: err = %v, want ErrNotFound", err)
	}
}

func TestUpdateRoadmap_名前と目標日を個別に更新できる(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	me := createTestUser(t, st)
	id := importSample(t, st, me.ID)

	// 名前だけ変える。目標日は触らない（未設定のまま）
	name := "新しい名前"
	got, err := st.UpdateRoadmap(ctx, me.ID, id, RoadmapUpdate{Name: &name})
	if err != nil {
		t.Fatalf("名前の更新に失敗: %v", err)
	}
	if got.Name != name || got.TargetDate != nil {
		t.Errorf("更新後 = (%s, %v), want (%s, nil)", got.Name, got.TargetDate, name)
	}

	// 目標日だけ設定する。名前は変わらない
	target := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	got, err = st.UpdateRoadmap(ctx, me.ID, id, RoadmapUpdate{TargetDate: &target, SetTargetDate: true})
	if err != nil {
		t.Fatalf("目標日の設定に失敗: %v", err)
	}
	if got.Name != name {
		t.Errorf("目標日だけ変えたのに名前が %q になっている", got.Name)
	}
	if got.TargetDate == nil || got.TargetDate.Format(time.DateOnly) != "2026-12-31" {
		t.Errorf("targetDate = %v, want 2026-12-31", got.TargetDate)
	}

	// 目標日を消す（SetTargetDate=true, TargetDate=nil）
	got, err = st.UpdateRoadmap(ctx, me.ID, id, RoadmapUpdate{SetTargetDate: true})
	if err != nil {
		t.Fatalf("目標日の削除に失敗: %v", err)
	}
	if got.TargetDate != nil {
		t.Errorf("targetDate = %v, want nil（消したはず）", got.TargetDate)
	}

	// updated_at は DB のトリガで進む
	if !got.UpdatedAt.After(got.CreatedAt) {
		t.Errorf("updated_at (%v) が created_at (%v) より後になっていない", got.UpdatedAt, got.CreatedAt)
	}
}

func TestUpdateRoadmap_他人のものはNotFound(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	me := createTestUser(t, st)
	other := createTestUser(t, st)
	theirs := importSample(t, st, other.ID)

	name := "乗っ取り"
	if _, err := st.UpdateRoadmap(ctx, me.ID, theirs, RoadmapUpdate{Name: &name}); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	// 変わっていないこと
	rm, err := st.GetRoadmap(ctx, other.ID, theirs)
	if err != nil {
		t.Fatalf("持ち主として取得できない: %v", err)
	}
	if rm.Name == name {
		t.Error("他人の更新が通っている")
	}
}

func TestDeleteRoadmap_分野と項目ごと消える(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	me := createTestUser(t, st)
	id := importSample(t, st, me.ID)

	if err := st.DeleteRoadmap(ctx, me.ID, id); err != nil {
		t.Fatalf("DeleteRoadmap が失敗: %v", err)
	}
	if n := countRows(t, st, "roadmaps", "id = $1", id); n != 0 {
		t.Errorf("roadmaps が %d 行残っている", n)
	}
	if n := countRows(t, st, "domains", "roadmap_id = $1", id); n != 0 {
		t.Errorf("domains が %d 行残っている（cascade が効いていない）", n)
	}
	if n := countRows(t, st, "items", "roadmap_id = $1", id); n != 0 {
		t.Errorf("items が %d 行残っている（cascade が効いていない）", n)
	}
	// 2回目は無いので NotFound
	if err := st.DeleteRoadmap(ctx, me.ID, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("2回目の削除: err = %v, want ErrNotFound", err)
	}
}

func TestDeleteRoadmap_他人のものはNotFound(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	me := createTestUser(t, st)
	other := createTestUser(t, st)
	theirs := importSample(t, st, other.ID)

	if err := st.DeleteRoadmap(ctx, me.ID, theirs); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if n := countRows(t, st, "roadmaps", "id = $1", theirs); n != 1 {
		t.Errorf("他人のロードマップが消えている（%d 行）", n)
	}
}
