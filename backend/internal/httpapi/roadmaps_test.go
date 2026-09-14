package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/n-yoshida-dev/skill-matrix/internal/roadmap"
	"github.com/n-yoshida-dev/skill-matrix/internal/store"
)

// fakeImporter は DB を使わない偽物の store。
// 呼ばれたときの引数を覚えておき、決めておいた結果を返す。
type fakeImporter struct {
	called  bool
	ownerID string
	kind    string
	doc     *roadmap.Document

	returnID  string
	returnErr error
}

func (f *fakeImporter) ImportRoadmap(_ context.Context, ownerUserID, kind string, doc *roadmap.Document) (string, error) {
	f.called = true
	f.ownerID = ownerUserID
	f.kind = kind
	f.doc = doc
	return f.returnID, f.returnErr
}

// importRequest は認証済みユーザーを載せたインポートのリクエストを作る。
// requireAuth（DB が要る）を通さず、ハンドラを直接呼ぶためのもの。
func importRequest(body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/roadmaps/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	u := &store.User{ID: "user-1", Login: "dummy"}
	return req.WithContext(context.WithValue(req.Context(), userContextKey, u))
}

// callImport はハンドラを直接呼び、応答と偽物の store を返す。
func callImport(t *testing.T, body []byte, imp *fakeImporter) *httptest.ResponseRecorder {
	t.Helper()
	api := testAPI()
	api.importer = imp
	rec := httptest.NewRecorder()
	api.handleImportRoadmap(rec, importRequest(body))
	return rec
}

// readSample は backend/testdata のサンプルを読む。
func readSample(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("../../testdata/" + name)
	if err != nil {
		t.Fatalf("サンプルの読み込みに失敗: %v", err)
	}
	return data
}

// decodeBody は応答の JSON を汎用の map に読む。キー名の有無を確かめたいときに使う。
func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("応答が JSON として読めない: %v\n%s", err, rec.Body.String())
	}
	return body
}

func TestImportRoadmap_検査を通れば保存して201を返す(t *testing.T) {
	imp := &fakeImporter{returnID: "roadmap-1"}
	// external で outcome の無い項目がある。警告は出るがエラーは無い（SPEC.md §2.1）
	rec := callImport(t, []byte(`{
	  "schemaVersion": 1, "name": "外部由来", "origin": "external",
	  "source": "https://example.com/curriculum", "checkedAt": "2026-09-14",
	  "levels": [
	    {"level":1,"name":"L1","criteria":"c1"},{"level":2,"name":"L2","criteria":"c2"},
	    {"level":3,"name":"L3","criteria":"c3"},{"level":4,"name":"L4","criteria":"c4"},
	    {"level":5,"name":"L5","criteria":"c5"}
	  ],
	  "domains": [{"key":"a","name":"A","goal":"A ができる","items":[
	    {"key":"a-01","name":"A-01","outcome":"a-01 ができる"},
	    {"key":"a-02","name":"A-02","dependsOn":["a-01"]}
	  ]}]
	}`), imp)

	if rec.Code != http.StatusCreated {
		t.Fatalf("ステータス = %d, 期待値 %d\n%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if !imp.called {
		t.Fatal("store.ImportRoadmap が呼ばれていない")
	}
	if imp.ownerID != "user-1" {
		t.Errorf("owner = %q, 期待値 user-1（ログイン中のユーザー）", imp.ownerID)
	}
	if imp.kind != store.RoadmapKindPersonal {
		t.Errorf("kind = %q, 期待値 %q（インポートは常に personal）", imp.kind, store.RoadmapKindPersonal)
	}
	if imp.doc == nil || imp.doc.Name == "" {
		t.Error("検査済みの Document が store に渡っていない")
	}

	var got importResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("応答を読めない: %v", err)
	}
	if got.ID != "roadmap-1" {
		t.Errorf("id = %q, 期待値 roadmap-1", got.ID)
	}
	// 警告は応答に載る。エラーは載らない（載っていたら 400 になっているはず）
	if len(got.Issues) == 0 {
		t.Error("外部由来で outcome の無い項目があるのに警告が応答に載っていない")
	}
	for _, is := range got.Issues {
		if is.Severity != roadmap.SeverityWarning {
			t.Errorf("201 の issues に警告以外が入っている: %+v", is)
		}
	}
}

func TestImportRoadmap_警告が無ければissuesは空配列(t *testing.T) {
	// サンプルは出典も到達状態も全部埋まっていて、警告すら出ない
	rec := callImport(t, readSample(t, "roadmap-sample.json"), &fakeImporter{returnID: "roadmap-2"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("ステータス = %d, 期待値 %d\n%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	// フロントが issues.length を読んで落ちないよう、null ではなく [] であること
	if !strings.Contains(rec.Body.String(), `"issues":[]`) {
		t.Errorf("issues が空配列になっていない: %s", rec.Body.String())
	}
}

func TestImportRoadmap_検査エラーがあれば400で全問題を返し保存しない(t *testing.T) {
	imp := &fakeImporter{returnID: "must-not-be-used"}
	rec := callImport(t, readSample(t, "roadmap-broken.json"), imp)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ステータス = %d, 期待値 %d\n%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if imp.called {
		t.Error("検査エラーがあるのに store.ImportRoadmap が呼ばれている")
	}

	var got importErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("応答を読めない: %v", err)
	}
	if got.Error.Code != codeInvalidRoadmap {
		t.Errorf("error.code = %q, 期待値 %q", got.Error.Code, codeInvalidRoadmap)
	}
	// 1件目で打ち切らず、壊れたサンプルの問題をまとめて返す
	if len(got.Issues) < 5 {
		t.Errorf("問題が %d 件しか返っていない。全部集めて返すべき: %+v", len(got.Issues), got.Issues)
	}
	hasError := false
	for _, is := range got.Issues {
		if is.Severity == roadmap.SeverityError {
			hasError = true
		}
	}
	if !hasError {
		t.Error("400 なのに issues にエラーが1件も入っていない")
	}
}

func TestImportRoadmap_JSONとして読めなければ400(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"壊れた JSON", `{"name": `},
		{"空のボディ", ``},
		{"配列", `[]`},
		{"定義にないフィールド", `{"schemaVersion":1,"name":"x","dependOn":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imp := &fakeImporter{}
			rec := callImport(t, []byte(tt.body), imp)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("ステータス = %d, 期待値 %d\n%s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			if imp.called {
				t.Error("読み込みに失敗したのに store が呼ばれている")
			}
			body := decodeBody(t, rec)
			if _, ok := body["issues"]; !ok {
				t.Errorf("400 の応答に issues が無い: %s", rec.Body.String())
			}
		})
	}
}

func TestImportRoadmap_ボディが上限を超えると413(t *testing.T) {
	// 上限 +1 バイト。中身は JSON として妥当でなくてよい（解釈する前に弾かれる）
	body := bytes.Repeat([]byte("a"), maxImportBodyBytes+1)
	imp := &fakeImporter{}
	rec := callImport(t, body, imp)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("ステータス = %d, 期待値 %d\n%s", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
	}
	if imp.called {
		t.Error("上限超えなのに store が呼ばれている")
	}
	var got errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("応答を読めない: %v", err)
	}
	if got.Error.Code != codePayloadTooLarge {
		t.Errorf("error.code = %q, 期待値 %q", got.Error.Code, codePayloadTooLarge)
	}
}

func TestImportRoadmap_保存に失敗すれば500(t *testing.T) {
	imp := &fakeImporter{returnErr: errors.New("DB が落ちている")}
	rec := callImport(t, readSample(t, "roadmap-sample.json"), imp)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("ステータス = %d, 期待値 %d\n%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	var got errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("応答を読めない: %v", err)
	}
	if got.Error.Code != codeInternal {
		t.Errorf("error.code = %q, 期待値 %q", got.Error.Code, codeInternal)
	}
}

func TestImportRoadmap_未ログインなら401(t *testing.T) {
	// ルータ経由で呼び、経路が requireAuth の内側に登録されていることを確かめる。
	// Cookie が無ければ store に触る前に 401 が返るので DB は要らない
	api := testAPI()
	handler := New(api.cfg, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/roadmaps/import", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("ステータス = %d, 期待値 %d\n%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}
