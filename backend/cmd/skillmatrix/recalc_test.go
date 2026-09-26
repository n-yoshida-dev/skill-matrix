package main

import (
	"slices"
	"testing"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
	"github.com/n-yoshida-dev/skill-matrix/internal/judgment"
)

// このテストは「判定ファイルの並びから state.json の中身がどう組み上がるか」を具体例で固定する。
// 検証ルール（V1〜V8）そのものは internal/domain のテストが担当する。ここで見るのは、
// 処理順・「どのファイルの何件目か」の記録・保留と棄却の振り分け・書き出しの形。

// testRoadmap は 3 項目だけのロードマップ。
func testRoadmap() domain.Roadmap {
	return domain.Roadmap{Domains: []domain.Domain{
		{Key: "go", Name: "Go", Items: []domain.Item{
			{Key: "go-01", DomainKey: "go", Name: "基本構文"},
			{Key: "go-02", DomainKey: "go", Name: "インターフェース", DependsOn: []domain.ItemKey{"go-01"}},
		}},
		{Key: "react", Name: "React", Items: []domain.Item{
			{Key: "react-01", DomainKey: "react", Name: "コンポーネント"},
		}},
	}}
}

// mustFile は判定ファイルの JSON を読む。外枠が崩れていればテストを止める。
func mustFile(t *testing.T, name, raw string) judgment.File {
	t.Helper()
	f, err := judgment.ParseFile(name, []byte(raw))
	if err != nil {
		t.Fatalf("判定ファイルを読めない: %v", err)
	}
	return f
}

// itemOf は state.json の中身から項目1つを探す。
func itemOf(t *testing.T, doc stateDoc, key string) itemDoc {
	t.Helper()
	for _, it := range doc.Items {
		if it.ItemKey == key {
			return it
		}
	}
	t.Fatalf("%s が items に無い", key)
	return itemDoc{}
}

func TestRecalculate_ファイル名の昇順で適用する(t *testing.T) {
	// 8/1 に説明できた（印 1・2）→ 9/10 にドリル（印 1）。ドリルの印 1 は適用前のレベル 2 未満なので、
	// 最終根拠日は 8/1 のまま。逆順に適用すると 9/10 になってしまう（SPEC.md §3.2・§3.4）
	a := mustFile(t, "2026-08-01-explain.json", `{"schemaVersion": 2, "loggedAt": "2026-08-01", "source": "manual", "judgments": [
		{"itemKey": "go-01", "evidenceType": "self_explanation", "proposedLevel": 2, "evidenceRefs": ["log:a.md"], "rationale": "説明できた"}]}`)
	b := mustFile(t, "2026-09-10-drill.json", `{"schemaVersion": 2, "loggedAt": "2026-09-10", "source": "manual", "judgments": [
		{"itemKey": "go-01", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["log:b.md"], "rationale": "ドリルに答えた"}]}`)

	// わざと逆順で渡す
	doc := recalculate(testRoadmap(), []judgment.File{b, a}, domain.DefaultRules()).doc

	it := itemOf(t, doc, "go-01")
	if it.LastEvidenceAt == nil || *it.LastEvidenceAt != "2026-08-01" {
		t.Errorf("lastEvidenceAt = %v, want 2026-08-01", dateOrDash(it.LastEvidenceAt))
	}
	var files []string
	for _, e := range it.Events {
		files = append(files, e.File)
	}
	if want := []string{"2026-08-01-explain.json", "2026-09-10-drill.json"}; !slices.Equal(files, want) {
		t.Errorf("events の順 = %v, want %v", files, want)
	}
}

func TestRecalculate_棄却はファイル内の元の位置で記録する(t *testing.T) {
	// 0 件目は形の崩れ（evidenceRefs が無い）、1 件目は実在しない項目（V1）、2 件目だけが適用される。
	// domain には形の検査を通った 2 件だけが渡るので、位置を引き直さないと V1 が 0 件目と記録されてしまう
	f := mustFile(t, "2026-10-01-mixed.json", `{"schemaVersion": 2, "loggedAt": "2026-10-01", "source": "ai", "judgments": [
		{"itemKey": "go-02", "evidenceType": "drill", "proposedLevel": 1, "rationale": "x", "confidence": 0.9},
		{"itemKey": "go-99", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["log:a.md"], "rationale": "x", "confidence": 0.9},
		{"itemKey": "go-01", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["log:a.md"], "rationale": "x", "confidence": 0.9}]}`)

	doc := recalculate(testRoadmap(), []judgment.File{f}, domain.DefaultRules()).doc

	want := []pendingDoc{
		{File: "2026-10-01-mixed.json", Index: 0, ItemKey: "go-02", Code: shapeRejectedCode},
		{File: "2026-10-01-mixed.json", Index: 1, ItemKey: "go-99", Code: string(domain.ViolationUnknownItem)},
	}
	if len(doc.Rejected) != len(want) {
		t.Fatalf("rejected = %+v", doc.Rejected)
	}
	for i, w := range want {
		got := doc.Rejected[i]
		got.Detail = "" // 文面は domain と judgment のテストが見る
		if got != w {
			t.Errorf("rejected[%d] = %+v, want %+v", i, got, w)
		}
	}
	if evs := itemOf(t, doc, "go-01").Events; len(evs) != 1 || evs[0].Index != 2 {
		t.Errorf("go-01 の events = %+v（index 2 のはず）", evs)
	}
}

func TestRecalculate_形の崩れでitemKeyが読めなければ空にする(t *testing.T) {
	f := mustFile(t, "2026-10-01-broken.json", `{"schemaVersion": 2, "loggedAt": "2026-10-01", "source": "ai", "judgments": ["go-01", {"itemKey": 1}]}`)
	doc := recalculate(testRoadmap(), []judgment.File{f}, domain.DefaultRules()).doc
	if len(doc.Rejected) != 2 || doc.Rejected[0].ItemKey != "" || doc.Rejected[1].ItemKey != "" {
		t.Errorf("rejected = %+v", doc.Rejected)
	}
}

func TestRecalculate_保留は適用せずdeferredに残す(t *testing.T) {
	f := mustFile(t, "2026-10-01-unsure.json", `{"schemaVersion": 2, "loggedAt": "2026-10-01", "source": "ai", "judgments": [
		{"itemKey": "go-01", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["log:a.md"], "rationale": "x", "confidence": 0.9},
		{"itemKey": "go-02", "evidenceType": "implementation", "proposedLevel": 3, "evidenceRefs": ["log:a.md"], "rationale": "y", "confidence": 0.3}]}`)

	doc := recalculate(testRoadmap(), []judgment.File{f}, domain.DefaultRules()).doc

	if len(doc.Deferred) != 1 {
		t.Fatalf("deferred = %+v", doc.Deferred)
	}
	d := doc.Deferred[0]
	if d.Index != 1 || d.ItemKey != "go-02" || d.Code != string(domain.ViolationLowConfidence) || d.Detail == "" {
		t.Errorf("deferred[0] = %+v", d)
	}
	it := itemOf(t, doc, "go-02")
	if len(it.Events) != 0 || len(it.EvidencedLevels) != 0 {
		t.Errorf("保留した判定が適用されている: %+v", it)
	}
	if len(doc.Rejected) != 0 {
		t.Errorf("保留は棄却ではない: %+v", doc.Rejected)
	}
}

func TestRecalculate_閾値は設定から受け取る(t *testing.T) {
	f := mustFile(t, "2026-10-01-unsure.json", `{"schemaVersion": 2, "loggedAt": "2026-10-01", "source": "ai", "judgments": [
		{"itemKey": "go-02", "evidenceType": "implementation", "proposedLevel": 3, "evidenceRefs": ["log:a.md"], "rationale": "y", "confidence": 0.3}]}`)
	rules := domain.DefaultRules()
	rules.ConfidenceThreshold = 0.2

	doc := recalculate(testRoadmap(), []judgment.File{f}, rules).doc
	if len(doc.Deferred) != 0 || len(itemOf(t, doc, "go-02").Events) != 1 {
		t.Errorf("閾値 0.2 なら 0.3 は適用されるはず: deferred = %+v", doc.Deferred)
	}
}

func TestRecalculate_適用した判定の記録(t *testing.T) {
	ai := mustFile(t, "2026-10-01-ai.json", `{"schemaVersion": 2, "loggedAt": "2026-10-01", "source": "ai", "judgments": [
		{"itemKey": "go-01", "evidenceType": "drill", "proposedLevel": 2, "occurredAt": "2026-09-28", "evidenceRefs": ["repo:a/b@c/x.md#L1"], "rationale": "ドリル", "confidence": 0.8}]}`)
	manual := mustFile(t, "2026-10-02-fix.json", `{"schemaVersion": 2, "loggedAt": "2026-10-02", "source": "manual", "judgments": [
		{"itemKey": "react-01", "evidenceType": "learning_activity", "proposedLevel": 0, "evidenceRefs": ["log:r.md"], "rationale": "読み始めた"}]}`)

	doc := recalculate(testRoadmap(), []judgment.File{ai, manual}, domain.DefaultRules()).doc

	e := itemOf(t, doc, "go-01").Events[0]
	if e.OccurredAt != "2026-09-28" || e.Source != "ai" || e.EvidenceType != "drill" || e.ProposedLevel != 2 {
		t.Errorf("event = %+v", e)
	}
	// ドリルで付けられる印は 1 だけ。2 は V5 で切り詰めて記録する
	if !slices.Equal(e.Marked, []int{1}) {
		t.Errorf("marked = %v, want [1]", e.Marked)
	}
	if len(e.Violations) != 1 || e.Violations[0].Code != string(domain.ViolationEvidenceTooWeak) {
		t.Errorf("violations = %+v", e.Violations)
	}
	if e.Confidence == nil || *e.Confidence != 0.8 {
		t.Errorf("ai の confidence = %v", e.Confidence)
	}

	r := itemOf(t, doc, "react-01")
	if r.PreState != "learning" || len(r.Events[0].Marked) != 0 || r.Events[0].Confidence != nil {
		t.Errorf("manual の学習開始: preState = %q, event = %+v", r.PreState, r.Events[0])
	}
	if r.Events[0].OccurredAt != "2026-10-02" {
		t.Errorf("occurredAt が無ければ loggedAt: %q", r.Events[0].OccurredAt)
	}
	// 印の付かない根拠は鮮度を更新しない
	if r.LastEvidenceAt != nil {
		t.Errorf("lastEvidenceAt = %s, want null", *r.LastEvidenceAt)
	}
}

func TestRecalculate_全項目をロードマップの定義順に並べる(t *testing.T) {
	doc := recalculate(testRoadmap(), nil, domain.DefaultRules()).doc

	var keys []string
	for _, it := range doc.Items {
		keys = append(keys, it.ItemKey)
		// 判定が1件も無い項目も、画面が扱える形（none・空の配列・null）で出す
		if it.VerifiedLevel != 0 || it.PreState != "none" || it.NeedsReview || it.LastEvidenceAt != nil {
			t.Errorf("%s = %+v", it.ItemKey, it)
		}
		if it.EvidencedLevels == nil || it.Events == nil {
			t.Errorf("%s の配列が nil（state.json では [] にする）", it.ItemKey)
		}
	}
	if want := []string{"go-01", "go-02", "react-01"}; !slices.Equal(keys, want) {
		t.Errorf("items の順 = %v, want %v", keys, want)
	}
	if doc.Deferred == nil || doc.Rejected == nil {
		t.Error("deferred / rejected が nil（state.json では [] にする）")
	}
}

func TestRecalculate_印が付いた項目のpreStateはnone(t *testing.T) {
	// 段階前の状態のゼロ値 "" は、印が付いても domain では書き換えない（Changed を立てないため）。
	// state.json では none にそろえる（PR #41 のレビューで判明）
	f := mustFile(t, "2026-10-01-drill.json", `{"schemaVersion": 2, "loggedAt": "2026-10-01", "source": "manual", "judgments": [
		{"itemKey": "go-01", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["log:a.md"], "rationale": "x"}]}`)
	doc := recalculate(testRoadmap(), []judgment.File{f}, domain.DefaultRules()).doc
	if got := itemOf(t, doc, "go-01").PreState; got != "none" {
		t.Errorf("preState = %q, want none", got)
	}
}
