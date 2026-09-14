package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/n-yoshida-dev/skill-matrix/internal/roadmap"
	"github.com/n-yoshida-dev/skill-matrix/internal/store"
)

// fakeRoadmapStore は DB を使わない偽物の store（CRUD 用）。
// 受け取った引数を覚え、決めておいた結果を返す。
type fakeRoadmapStore struct {
	gotOwner string
	gotID    string
	gotUpd   store.RoadmapUpdate

	list      []store.RoadmapSummary
	roadmap   *store.Roadmap
	summary   *store.RoadmapSummary
	returnErr error
}

func (f *fakeRoadmapStore) ImportRoadmap(context.Context, string, string, *roadmap.Document) (string, error) {
	return "", errors.New("このテストでは使わない")
}

func (f *fakeRoadmapStore) ListRoadmaps(_ context.Context, ownerUserID string) ([]store.RoadmapSummary, error) {
	f.gotOwner = ownerUserID
	return f.list, f.returnErr
}

func (f *fakeRoadmapStore) GetRoadmap(_ context.Context, ownerUserID, roadmapID string) (*store.Roadmap, error) {
	f.gotOwner, f.gotID = ownerUserID, roadmapID
	return f.roadmap, f.returnErr
}

func (f *fakeRoadmapStore) UpdateRoadmap(_ context.Context, ownerUserID, roadmapID string, upd store.RoadmapUpdate) (*store.RoadmapSummary, error) {
	f.gotOwner, f.gotID, f.gotUpd = ownerUserID, roadmapID, upd
	return f.summary, f.returnErr
}

func (f *fakeRoadmapStore) DeleteRoadmap(_ context.Context, ownerUserID, roadmapID string) error {
	f.gotOwner, f.gotID = ownerUserID, roadmapID
	return f.returnErr
}

const testRoadmapID = "0f5b7a3e-9c1d-4e2f-8a6b-1c2d3e4f5a6b"

// crudRequest は認証済みユーザーとパスの :id を載せたリクエストを作る。
// ルータを通さずハンドラを直接呼ぶので、chi の URL パラメータは手で入れる。
func crudRequest(method, id, body string) *http.Request {
	req := httptest.NewRequest(method, "/api/roadmaps/"+id, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), userContextKey, &store.User{ID: "user-1", Login: "dummy"})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}

func sampleSummary() store.RoadmapSummary {
	checked := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	return store.RoadmapSummary{
		ID: testRoadmapID, Kind: "personal", Name: "サンプル", Origin: "external",
		Source: "https://example.com", CheckedAt: &checked, ItemCount: 11,
		CreatedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
	}
}

// ---------------------------------------------------------------------------
// 一覧
// ---------------------------------------------------------------------------

func TestListRoadmaps_自分のIDで問い合わせて一覧を返す(t *testing.T) {
	fake := &fakeRoadmapStore{list: []store.RoadmapSummary{sampleSummary()}}
	api := testAPI()
	api.roadmaps = fake
	rec := httptest.NewRecorder()
	api.handleListRoadmaps(rec, crudRequest(http.MethodGet, "", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータス = %d, 期待値 200\n%s", rec.Code, rec.Body.String())
	}
	if fake.gotOwner != "user-1" {
		t.Errorf("owner = %q, 期待値 user-1（ログイン中のユーザー）", fake.gotOwner)
	}
	body := decodeBody(t, rec)
	list, ok := body["roadmaps"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("roadmaps が1件の配列になっていない: %s", rec.Body.String())
	}
	first := list[0].(map[string]any)
	// 日付は YYYY-MM-DD の文字列、未設定は null
	if first["checkedAt"] != "2026-08-14" {
		t.Errorf("checkedAt = %v, 期待値 2026-08-14", first["checkedAt"])
	}
	if v, present := first["targetDate"]; !present || v != nil {
		t.Errorf("targetDate = %v, 期待値 null（キーは残す）", v)
	}
	if first["itemCount"] != float64(11) {
		t.Errorf("itemCount = %v, 期待値 11", first["itemCount"])
	}
}

func TestListRoadmaps_0件なら空配列(t *testing.T) {
	api := testAPI()
	api.roadmaps = &fakeRoadmapStore{list: []store.RoadmapSummary{}}
	rec := httptest.NewRecorder()
	api.handleListRoadmaps(rec, crudRequest(http.MethodGet, "", ""))

	if !strings.Contains(rec.Body.String(), `"roadmaps":[]`) {
		t.Errorf("空配列になっていない: %s", rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// 取得
// ---------------------------------------------------------------------------

func TestGetRoadmap_分野と項目を含めて返す(t *testing.T) {
	fake := &fakeRoadmapStore{roadmap: &store.Roadmap{
		RoadmapSummary: sampleSummary(),
		Levels:         []roadmap.LevelDef{{Level: 1, Name: "L1", Criteria: "c1"}},
		Domains: []store.Domain{{
			ID: "d-1", Key: "go", Name: "Go", Goal: "API を作れる",
			Items: []store.Item{
				{ID: "i-1", Key: "go-01", Name: "基本構文", DependsOn: []string{}},
				{ID: "i-2", Key: "go-02", Name: "インターフェース", Outcome: "差し替えられる", OutcomeSource: "authored", DependsOn: []string{"go-01"}},
			},
		}},
	}}
	api := testAPI()
	api.roadmaps = fake
	rec := httptest.NewRecorder()
	api.handleGetRoadmap(rec, crudRequest(http.MethodGet, testRoadmapID, ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータス = %d, 期待値 200\n%s", rec.Code, rec.Body.String())
	}
	if fake.gotOwner != "user-1" || fake.gotID != testRoadmapID {
		t.Errorf("store に渡った (owner, id) = (%q, %q)", fake.gotOwner, fake.gotID)
	}
	var got roadmapResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("応答を読めない: %v", err)
	}
	if len(got.Domains) != 1 || len(got.Domains[0].Items) != 2 {
		t.Fatalf("分野・項目の数が合わない: %s", rec.Body.String())
	}
	if got.Domains[0].Items[1].DependsOn[0] != "go-01" {
		t.Errorf("dependsOn が写っていない: %+v", got.Domains[0].Items[1])
	}
	// 依存が無い項目は null ではなく []
	if !strings.Contains(rec.Body.String(), `"dependsOn":[]`) {
		t.Errorf("依存なしの dependsOn が空配列になっていない: %s", rec.Body.String())
	}
	if len(got.Levels) != 1 {
		t.Errorf("levels が写っていない: %+v", got.Levels)
	}
}

func TestGetRoadmap_見つからなければ404(t *testing.T) {
	tests := []struct {
		name string
		id   string
		err  error
	}{
		{"store が NotFound（他人のもの・存在しない）", testRoadmapID, store.ErrNotFound},
		{"uuid の形でない id", "not-a-uuid", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeRoadmapStore{returnErr: tt.err}
			api := testAPI()
			api.roadmaps = fake
			rec := httptest.NewRecorder()
			api.handleGetRoadmap(rec, crudRequest(http.MethodGet, tt.id, ""))

			if rec.Code != http.StatusNotFound {
				t.Errorf("ステータス = %d, 期待値 404\n%s", rec.Code, rec.Body.String())
			}
			var got errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("応答を読めない: %v", err)
			}
			if got.Error.Code != codeNotFound {
				t.Errorf("error.code = %q, 期待値 %q", got.Error.Code, codeNotFound)
			}
			if tt.err == nil && fake.gotID != "" {
				t.Error("uuid の形でない id が store まで渡っている")
			}
		})
	}
}

func TestGetRoadmap_storeの失敗は500(t *testing.T) {
	api := testAPI()
	api.roadmaps = &fakeRoadmapStore{returnErr: errors.New("DB が落ちている")}
	rec := httptest.NewRecorder()
	api.handleGetRoadmap(rec, crudRequest(http.MethodGet, testRoadmapID, ""))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータス = %d, 期待値 500", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// 更新
// ---------------------------------------------------------------------------

func TestPatchRoadmap_名前と目標日の指定を正しくstoreに渡す(t *testing.T) {
	sum := sampleSummary()
	tests := []struct {
		name    string
		body    string
		want    func(t *testing.T, upd store.RoadmapUpdate)
		wantErr bool
	}{
		{
			name: "名前だけ",
			body: `{"name": "  新しい名前  "}`,
			want: func(t *testing.T, upd store.RoadmapUpdate) {
				if upd.Name == nil || *upd.Name != "新しい名前" {
					t.Errorf("Name = %v, 期待値 新しい名前（前後の空白は除く）", upd.Name)
				}
				if upd.SetTargetDate {
					t.Error("targetDate を送っていないのに SetTargetDate が true")
				}
			},
		},
		{
			name: "目標日を設定",
			body: `{"targetDate": "2026-12-31"}`,
			want: func(t *testing.T, upd store.RoadmapUpdate) {
				if upd.Name != nil {
					t.Errorf("name を送っていないのに Name = %q", *upd.Name)
				}
				if !upd.SetTargetDate || upd.TargetDate == nil || upd.TargetDate.Format(time.DateOnly) != "2026-12-31" {
					t.Errorf("TargetDate = (%v, set=%v), 期待値 2026-12-31", upd.TargetDate, upd.SetTargetDate)
				}
			},
		},
		{
			name: "目標日を消す（null）",
			body: `{"targetDate": null}`,
			want: func(t *testing.T, upd store.RoadmapUpdate) {
				if !upd.SetTargetDate || upd.TargetDate != nil {
					t.Errorf("null は「消す」。(set=%v, date=%v)", upd.SetTargetDate, upd.TargetDate)
				}
			},
		},
		{
			name: "両方",
			body: `{"name": "x", "targetDate": "2027-01-01"}`,
			want: func(t *testing.T, upd store.RoadmapUpdate) {
				if upd.Name == nil || !upd.SetTargetDate || upd.TargetDate == nil {
					t.Errorf("両方が渡っていない: %+v", upd)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeRoadmapStore{summary: &sum}
			api := testAPI()
			api.roadmaps = fake
			rec := httptest.NewRecorder()
			api.handlePatchRoadmap(rec, crudRequest(http.MethodPatch, testRoadmapID, tt.body))

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータス = %d, 期待値 200\n%s", rec.Code, rec.Body.String())
			}
			if fake.gotOwner != "user-1" || fake.gotID != testRoadmapID {
				t.Errorf("store に渡った (owner, id) = (%q, %q)", fake.gotOwner, fake.gotID)
			}
			tt.want(t, fake.gotUpd)
		})
	}
}

func TestPatchRoadmap_不正なボディは400でstoreを呼ばない(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"空のオブジェクト（更新する項目がない）", `{}`},
		{"空のボディ", ``},
		{"壊れた JSON", `{"name": `},
		{"名前が空", `{"name": "   "}`},
		{"名前が長すぎる", `{"name": "` + strings.Repeat("あ", roadmap.MaxNameLen+1) + `"}`},
		{"目標日の形が違う", `{"targetDate": "2026/12/31"}`},
		{"目標日が文字列でない", `{"targetDate": 20261231}`},
		{"定義にないフィールド", `{"nmae": "x"}`},
		{"後ろに余分なデータ", `{"name": "x"} {}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeRoadmapStore{}
			api := testAPI()
			api.roadmaps = fake
			rec := httptest.NewRecorder()
			api.handlePatchRoadmap(rec, crudRequest(http.MethodPatch, testRoadmapID, tt.body))

			if rec.Code != http.StatusBadRequest {
				t.Errorf("ステータス = %d, 期待値 400\n%s", rec.Code, rec.Body.String())
			}
			if fake.gotID != "" {
				t.Error("不正なボディなのに store が呼ばれている")
			}
		})
	}
}

func TestPatchRoadmap_見つからなければ404(t *testing.T) {
	api := testAPI()
	api.roadmaps = &fakeRoadmapStore{returnErr: store.ErrNotFound}
	rec := httptest.NewRecorder()
	api.handlePatchRoadmap(rec, crudRequest(http.MethodPatch, testRoadmapID, `{"name": "x"}`))
	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータス = %d, 期待値 404\n%s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// 削除
// ---------------------------------------------------------------------------

func TestDeleteRoadmap_成功なら204(t *testing.T) {
	fake := &fakeRoadmapStore{}
	api := testAPI()
	api.roadmaps = fake
	rec := httptest.NewRecorder()
	api.handleDeleteRoadmap(rec, crudRequest(http.MethodDelete, testRoadmapID, ""))

	if rec.Code != http.StatusNoContent {
		t.Errorf("ステータス = %d, 期待値 204\n%s", rec.Code, rec.Body.String())
	}
	if fake.gotOwner != "user-1" || fake.gotID != testRoadmapID {
		t.Errorf("store に渡った (owner, id) = (%q, %q)", fake.gotOwner, fake.gotID)
	}
}

func TestDeleteRoadmap_見つからなければ404(t *testing.T) {
	api := testAPI()
	api.roadmaps = &fakeRoadmapStore{returnErr: store.ErrNotFound}
	rec := httptest.NewRecorder()
	api.handleDeleteRoadmap(rec, crudRequest(http.MethodDelete, testRoadmapID, ""))
	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータス = %d, 期待値 404\n%s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// 経路の登録
// ---------------------------------------------------------------------------

func TestRoadmapRoutes_未ログインなら401(t *testing.T) {
	// ルータ経由で呼び、4本とも requireAuth の内側に登録されていることを確かめる
	api := testAPI()
	handler := New(api.cfg, nil)

	tests := []struct{ method, path string }{
		{http.MethodGet, "/api/roadmaps"},
		{http.MethodGet, "/api/roadmaps/" + testRoadmapID},
		{http.MethodPatch, "/api/roadmaps/" + testRoadmapID},
		{http.MethodDelete, "/api/roadmaps/" + testRoadmapID},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("ステータス = %d, 期待値 401\n%s", rec.Code, rec.Body.String())
			}
		})
	}
}
