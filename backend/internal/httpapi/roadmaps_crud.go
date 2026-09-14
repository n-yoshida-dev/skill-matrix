package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/n-yoshida-dev/skill-matrix/internal/roadmap"
	"github.com/n-yoshida-dev/skill-matrix/internal/store"
)

// このファイルは自分の personal ロードマップの一覧・取得・更新・削除（SPEC.md §6）。
//
// 他人のロードマップは 403 ではなく **404** にする。403 は「その id は存在するが権限が無い」と
// 教えてしまう。所有者の絞り込みは store の SQL に入っていて、ここでは ErrNotFound を 404 に写すだけ。

// maxPatchBodyBytes は PATCH のボディ上限。名前と日付しか来ないので小さくてよい。
const maxPatchBodyBytes = 16 << 10 // 16 KiB

// uuidPattern は パスの :id が uuid の形かを見る。
// 形が違う値を DB に渡すと「invalid input syntax for type uuid」で 500 になるので、
// 先に 404 に寄せる（存在しない id と同じ扱い）。
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ---------------------------------------------------------------------------
// 応答の形（SPEC.md §6）
// ---------------------------------------------------------------------------

// roadmapSummaryResponse は一覧の1件と PATCH の応答。
// 日付（checkedAt / targetDate）は YYYY-MM-DD の文字列、未設定なら null。
type roadmapSummaryResponse struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Origin      string    `json:"origin"`
	Source      string    `json:"source"`
	CheckedAt   *string   `json:"checkedAt"`
	TargetDate  *string   `json:"targetDate"`
	ItemCount   int       `json:"itemCount"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// roadmapListResponse は GET /api/roadmaps の応答。
type roadmapListResponse struct {
	Roadmaps []roadmapSummaryResponse `json:"roadmaps"`
}

// roadmapResponse は GET /api/roadmaps/:id の応答。見出しに levels と domains を足したもの。
type roadmapResponse struct {
	roadmapSummaryResponse
	Levels  []roadmap.LevelDef `json:"levels"`
	Domains []domainResponse   `json:"domains"`
}

type domainResponse struct {
	ID    string         `json:"id"`
	Key   string         `json:"key"`
	Name  string         `json:"name"`
	Goal  string         `json:"goal"`
	Items []itemResponse `json:"items"`
}

type itemResponse struct {
	ID            string   `json:"id"`
	Key           string   `json:"key"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Outcome       string   `json:"outcome"`
	OutcomeSource string   `json:"outcomeSource"`
	VerifyBy      string   `json:"verifyBy"`
	DependsOn     []string `json:"dependsOn"`
}

// toSummaryResponse は store の型を応答の形に写す。
func toSummaryResponse(s store.RoadmapSummary) roadmapSummaryResponse {
	return roadmapSummaryResponse{
		ID:          s.ID,
		Kind:        s.Kind,
		Name:        s.Name,
		Description: s.Description,
		Origin:      s.Origin,
		Source:      s.Source,
		CheckedAt:   dateString(s.CheckedAt),
		TargetDate:  dateString(s.TargetDate),
		ItemCount:   s.ItemCount,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

// toRoadmapResponse は本体（分野・項目つき）を応答の形に写す。
func toRoadmapResponse(rm *store.Roadmap) roadmapResponse {
	out := roadmapResponse{
		roadmapSummaryResponse: toSummaryResponse(rm.RoadmapSummary),
		Levels:                 rm.Levels,
		Domains:                make([]domainResponse, 0, len(rm.Domains)),
	}
	for _, d := range rm.Domains {
		dr := domainResponse{ID: d.ID, Key: d.Key, Name: d.Name, Goal: d.Goal, Items: make([]itemResponse, 0, len(d.Items))}
		for _, it := range d.Items {
			dr.Items = append(dr.Items, itemResponse{
				ID:            it.ID,
				Key:           it.Key,
				Name:          it.Name,
				Description:   it.Description,
				Outcome:       it.Outcome,
				OutcomeSource: it.OutcomeSource,
				VerifyBy:      it.VerifyBy,
				DependsOn:     it.DependsOn,
			})
		}
		out.Domains = append(out.Domains, dr)
	}
	return out
}

// dateString は日付を YYYY-MM-DD にする。nil なら nil（JSON では null）。
func dateString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.DateOnly)
	return &s
}

// ---------------------------------------------------------------------------
// ハンドラ
// ---------------------------------------------------------------------------

// handleListRoadmaps は自分の personal ロードマップの一覧を返す。
func (a *API) handleListRoadmaps(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	list, err := a.roadmaps.ListRoadmaps(r.Context(), u.ID)
	if err != nil {
		log.Printf("ロードマップ一覧の取得に失敗しました: %v", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "ロードマップ一覧を取得できませんでした")
		return
	}
	res := roadmapListResponse{Roadmaps: make([]roadmapSummaryResponse, 0, len(list))}
	for _, s := range list {
		res.Roadmaps = append(res.Roadmaps, toSummaryResponse(s))
	}
	writeJSON(w, http.StatusOK, res)
}

// handleGetRoadmap はロードマップ本体（分野・項目・レベル定義）を返す。
func (a *API) handleGetRoadmap(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, ok := roadmapIDFromPath(w, r)
	if !ok {
		return
	}
	rm, err := a.roadmaps.GetRoadmap(r.Context(), u.ID, id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, codeNotFound, "ロードマップが見つかりません")
		return
	case err != nil:
		log.Printf("ロードマップの取得に失敗しました: %v", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "ロードマップを取得できませんでした")
		return
	}
	writeJSON(w, http.StatusOK, toRoadmapResponse(rm))
}

// patchRoadmapRequest は PATCH /api/roadmaps/:id のボディ。
//
// TargetDate を json.RawMessage で受けるのは、「キーが無い（触らない）」と「null（消す）」を
// 区別するため。*string で受けるとどちらも nil になって見分けがつかない。
type patchRoadmapRequest struct {
	Name       *string         `json:"name"`
	TargetDate json.RawMessage `json:"targetDate"`
}

// handlePatchRoadmap は名前と目標日を更新する。
func (a *API) handlePatchRoadmap(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, ok := roadmapIDFromPath(w, r)
	if !ok {
		return
	}

	upd, ok := decodePatchRoadmap(w, r)
	if !ok {
		return
	}

	sum, err := a.roadmaps.UpdateRoadmap(r.Context(), u.ID, id, upd)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, codeNotFound, "ロードマップが見つかりません")
		return
	case err != nil:
		log.Printf("ロードマップの更新に失敗しました: %v", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "ロードマップを更新できませんでした")
		return
	}
	writeJSON(w, http.StatusOK, toSummaryResponse(*sum))
}

// decodePatchRoadmap はボディを読んで検査し、store に渡す形にする。
// 失敗時はこの中でレスポンスを書き終える（戻り値の ok が false）。
func decodePatchRoadmap(w http.ResponseWriter, r *http.Request) (store.RoadmapUpdate, bool) {
	body, err := readLimitedBody(w, r, maxPatchBodyBytes)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, codePayloadTooLarge,
				fmt.Sprintf("リクエストボディは %d バイトまでです", maxPatchBodyBytes))
			return store.RoadmapUpdate{}, false
		}
		log.Printf("リクエストボディの読み込みに失敗しました: %v", err)
		writeError(w, http.StatusBadRequest, codeBadRequest, "リクエストボディを読み込めませんでした")
		return store.RoadmapUpdate{}, false
	}

	var req patchRoadmapRequest
	dec := json.NewDecoder(strings.NewReader(string(body)))
	// 未知のフィールドは弾く。targetDate を targetdate と打ち間違えて「変わらない」で済ませない
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "リクエストボディが JSON として読めません: "+err.Error())
		return store.RoadmapUpdate{}, false
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, codeBadRequest, "JSON オブジェクトの後ろに余分なデータがあります")
		return store.RoadmapUpdate{}, false
	}

	var upd store.RoadmapUpdate

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, codeBadRequest, "名前を空にはできません")
			return store.RoadmapUpdate{}, false
		}
		if n := utf8.RuneCountInString(name); n > roadmap.MaxNameLen {
			writeError(w, http.StatusBadRequest, codeBadRequest,
				fmt.Sprintf("名前は %d 文字までです（%d 文字あります）", roadmap.MaxNameLen, n))
			return store.RoadmapUpdate{}, false
		}
		upd.Name = &name
	}

	if len(req.TargetDate) > 0 {
		upd.SetTargetDate = true
		// null なら「消す」。TargetDate は nil のまま
		if string(req.TargetDate) != "null" {
			var s string
			if err := json.Unmarshal(req.TargetDate, &s); err != nil {
				writeError(w, http.StatusBadRequest, codeBadRequest, "目標日（targetDate）は YYYY-MM-DD の文字列か null です")
				return store.RoadmapUpdate{}, false
			}
			t, err := time.Parse(time.DateOnly, s)
			if err != nil {
				writeError(w, http.StatusBadRequest, codeBadRequest,
					fmt.Sprintf("目標日（targetDate）%q は YYYY-MM-DD の形ではありません", s))
				return store.RoadmapUpdate{}, false
			}
			upd.TargetDate = &t
		}
	}

	if upd.Name == nil && !upd.SetTargetDate {
		writeError(w, http.StatusBadRequest, codeBadRequest, "更新する項目がありません（name / targetDate）")
		return store.RoadmapUpdate{}, false
	}
	return upd, true
}

// handleDeleteRoadmap はロードマップを削除する。分野・項目・学習ログも一緒に消える。
func (a *API) handleDeleteRoadmap(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, ok := roadmapIDFromPath(w, r)
	if !ok {
		return
	}
	err := a.roadmaps.DeleteRoadmap(r.Context(), u.ID, id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, codeNotFound, "ロードマップが見つかりません")
		return
	case err != nil:
		log.Printf("ロードマップの削除に失敗しました: %v", err)
		writeError(w, http.StatusInternalServerError, codeInternal, "ロードマップを削除できませんでした")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// 補助
// ---------------------------------------------------------------------------

// requireUser はコンテキストから認証済みユーザーを取り出す。
// requireAuth を通っていれば必ず入っている。無ければ組み立ての誤りなので 500。
func requireUser(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	u, ok := userFromContext(r.Context())
	if !ok {
		log.Printf("%s %s に認証済みユーザーが渡っていません", r.Method, r.URL.Path)
		writeError(w, http.StatusInternalServerError, codeInternal, "サーバ内部エラーです")
		return nil, false
	}
	return u, true
}

// roadmapIDFromPath はパスの :id を取り出す。uuid の形でなければ 404 を書いて false を返す。
func roadmapIDFromPath(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := chi.URLParam(r, "id")
	if !uuidPattern.MatchString(id) {
		writeError(w, http.StatusNotFound, codeNotFound, "ロードマップが見つかりません")
		return "", false
	}
	return id, true
}
