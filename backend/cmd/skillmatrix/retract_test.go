package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
	"github.com/n-yoshida-dev/skill-matrix/internal/judgment"
)

// このテストは取り消しの記録（SPEC.md §4.7）が再計算にどう効くかを具体例で固定する。
// 取り消された判定は「最初から無かったもの」になり、別の有効な判定が付けた印は残る。

// explainedGo01 は go-01 に自分の言葉の説明（印 1・2）を付ける AI の判定ファイル。これを誤りとして取り消す。
const explainedGo01 = `{"schemaVersion": 2, "loggedAt": "2026-10-01", "source": "ai", "judgments": [
	{"itemKey": "go-01", "evidenceType": "self_explanation", "proposedLevel": 2, "evidenceRefs": ["log:a.md"], "rationale": "説明した", "confidence": 0.9},
	{"itemKey": "react-01", "evidenceType": "implementation", "proposedLevel": 3, "evidenceRefs": ["log:a.md"], "rationale": "書いた", "confidence": 0.9}]}`

// drillGo01 は go-01 にドリル（印 1）を付ける AI の判定ファイル。取り消さない、別の有効な根拠。
const drillGo01 = `{"schemaVersion": 2, "loggedAt": "2026-10-02", "source": "ai", "judgments": [
	{"itemKey": "go-01", "evidenceType": "drill", "proposedLevel": 1, "occurredAt": "2026-09-30", "evidenceRefs": ["log:b.md"], "rationale": "答えた", "confidence": 0.9}]}`

// retractFirst は 2026-10-01-ai.json の 0 件目（go-01 の説明）を取り消す訂正ファイル。
const retractFirst = `{"schemaVersion": 2, "loggedAt": "2026-10-05", "source": "manual", "judgments": [], "retractions": [
	{"file": "2026-10-01-ai.json", "index": 0, "reason": "説明したのは別の日の別の項目だった"}]}`

func TestRecalculate_取り消した判定を外し別の根拠の印は残す(t *testing.T) {
	files := []judgment.File{
		mustFile(t, "2026-10-05-fix.json", retractFirst), // 並び順に依存しないことも見る
		mustFile(t, "2026-10-01-ai.json", explainedGo01),
		mustFile(t, "2026-10-02-ai.json", drillGo01),
	}
	doc := recalculate(testRoadmap(), files, domain.DefaultRules()).doc

	g := itemOf(t, doc, "go-01")
	// 説明の印 2 は消え、ドリルの印 1 だけが残る
	if !slices.Equal(g.EvidencedLevels, []int{1}) || g.VerifiedLevel != 1 {
		t.Errorf("go-01: evidencedLevels = %v, verifiedLevel = %d, want [1], 1", g.EvidencedLevels, g.VerifiedLevel)
	}
	// 履歴にも取り消した判定は残らない。鮮度は残ったドリルの日付
	if len(g.Events) != 1 || g.Events[0].File != "2026-10-02-ai.json" {
		t.Errorf("go-01 の events = %+v", g.Events)
	}
	if g.LastEvidenceAt == nil || *g.LastEvidenceAt != "2026-09-30" {
		t.Errorf("go-01 の lastEvidenceAt = %v, want 2026-09-30", g.LastEvidenceAt)
	}

	// 同じファイルの取り消していない判定はそのまま適用される
	if r := itemOf(t, doc, "react-01"); !slices.Equal(r.EvidencedLevels, []int{3}) {
		t.Errorf("react-01: evidencedLevels = %v, want [3]", r.EvidencedLevels)
	}

	want := []retractedDoc{{File: "2026-10-01-ai.json", Index: 0, ItemKey: "go-01", RetractedBy: "2026-10-05-fix.json", Reason: "説明したのは別の日の別の項目だった"}}
	if !slices.Equal(doc.Retracted, want) {
		t.Errorf("retracted = %+v, want %+v", doc.Retracted, want)
	}
}

func TestRecalculate_唯一の根拠を取り消すと未着手に戻る(t *testing.T) {
	files := []judgment.File{mustFile(t, "2026-10-01-ai.json", explainedGo01), mustFile(t, "2026-10-05-fix.json", retractFirst)}
	g := itemOf(t, recalculate(testRoadmap(), files, domain.DefaultRules()).doc, "go-01")
	if len(g.EvidencedLevels) != 0 || g.VerifiedLevel != 0 || g.LastEvidenceAt != nil || len(g.Events) != 0 {
		t.Errorf("go-01 = %+v, want 判定が無いときと同じ", g)
	}
}

func TestRecalculate_取り消しと正しい判定を1つのファイルで足せる(t *testing.T) {
	// 訂正の典型：誤った説明（印 1・2）を取り消し、実際にあったドリル（印 1）を足し直す
	fix := `{"schemaVersion": 2, "loggedAt": "2026-10-05", "source": "manual", "judgments": [
		{"itemKey": "go-01", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["log:a.md"], "rationale": "答えたのは確認問題だけ"}
	], "retractions": [{"file": "2026-10-01-ai.json", "index": 0, "reason": "説明ではなく確認問題への回答だった"}]}`
	files := []judgment.File{mustFile(t, "2026-10-01-ai.json", explainedGo01), mustFile(t, "2026-10-05-fix.json", fix)}
	g := itemOf(t, recalculate(testRoadmap(), files, domain.DefaultRules()).doc, "go-01")
	if !slices.Equal(g.EvidencedLevels, []int{1}) || len(g.Events) != 1 || g.Events[0].Source != "manual" {
		t.Errorf("go-01 = %+v", g)
	}
}

func TestRecalculate_取り消すと要再確認も保留も棄却も消える(t *testing.T) {
	ai := `{"schemaVersion": 2, "loggedAt": "2026-10-01", "source": "ai", "judgments": [
		{"itemKey": "go-01", "evidenceType": "drill", "proposedLevel": 0, "evidenceRefs": ["log:a.md"], "rationale": "落ちた", "confidence": 0.9},
		{"itemKey": "go-02", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["log:a.md"], "rationale": "低い", "confidence": 0.1},
		{"itemKey": "go-99", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["log:a.md"], "rationale": "無い項目", "confidence": 0.9},
		{"itemKey": "react-01", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["log:a.md"], "rationale": "確信度が無い"}]}`
	fix := `{"schemaVersion": 2, "loggedAt": "2026-10-05", "source": "manual", "judgments": [], "retractions": [
		{"file": "2026-10-01-ai.json", "index": 3, "reason": "d"},
		{"file": "2026-10-01-ai.json", "index": 0, "reason": "a"},
		{"file": "2026-10-01-ai.json", "index": 1, "reason": "b"},
		{"file": "2026-10-01-ai.json", "index": 2, "reason": "c"}]}`

	before := recalculate(testRoadmap(), []judgment.File{mustFile(t, "2026-10-01-ai.json", ai)}, domain.DefaultRules()).doc
	// 取り消す前は、要再確認（V6）・保留（V7）・棄却（V1 と形の検査）がそれぞれ起きている
	if !itemOf(t, before, "go-01").NeedsReview || len(before.Deferred) != 1 || len(before.Rejected) != 2 {
		t.Fatalf("前提が崩れている: deferred = %+v, rejected = %+v", before.Deferred, before.Rejected)
	}

	doc := recalculate(testRoadmap(), []judgment.File{mustFile(t, "2026-10-01-ai.json", ai), mustFile(t, "2026-10-05-fix.json", fix)}, domain.DefaultRules()).doc
	if itemOf(t, doc, "go-01").NeedsReview || len(doc.Deferred) != 0 || len(doc.Rejected) != 0 {
		t.Errorf("needsReview = %v, deferred = %+v, rejected = %+v", itemOf(t, doc, "go-01").NeedsReview, doc.Deferred, doc.Rejected)
	}
	// 取り消しの記録は、取り消しを書いた順ではなく判定の場所の順。形の崩れた判定も itemKey が読めれば書く
	var got []string
	for _, r := range doc.Retracted {
		got = append(got, r.ItemKey+":"+r.Reason)
	}
	if want := []string{"go-01:a", "go-02:b", "go-99:c", "react-01:d"}; !slices.Equal(got, want) {
		t.Errorf("retracted = %v, want %v", got, want)
	}
}

func TestRecalculate_取り消しが無ければretractedは空の配列(t *testing.T) {
	doc := recalculate(testRoadmap(), []judgment.File{mustFile(t, "2026-10-01-ai.json", explainedGo01)}, domain.DefaultRules()).doc
	raw, err := encodeState(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"retracted": []`) {
		t.Errorf("retracted が [] で書かれていない:\n%s", raw)
	}
}

func TestCheckRetractions(t *testing.T) {
	base := mustFile(t, "2026-10-01-ai.json", explainedGo01) // 判定 2 件
	fix := func(name string, elems ...string) judgment.File {
		return mustFile(t, name, `{"schemaVersion": 2, "loggedAt": "`+name[:10]+`", "source": "manual", "judgments": [], "retractions": [`+strings.Join(elems, ",")+`]}`)
	}

	t.Run("指し先があれば問題なし", func(t *testing.T) {
		if p := checkRetractions([]judgment.File{base, fix("2026-10-05-fix.json", `{"file": "2026-10-01-ai.json", "index": 1, "reason": "x"}`)}); len(p) != 0 {
			t.Errorf("problems = %v", p)
		}
	})

	t.Run("形の検査で弾いた判定も、配列の位置があれば指せる", func(t *testing.T) {
		broken := mustFile(t, "2026-10-01-ai.json", `{"schemaVersion": 2, "loggedAt": "2026-10-01", "source": "ai", "judgments": [{"itemKey": 1}]}`)
		if p := checkRetractions([]judgment.File{broken, fix("2026-10-05-fix.json", `{"file": "2026-10-01-ai.json", "index": 0, "reason": "x"}`)}); len(p) != 0 {
			t.Errorf("problems = %v", p)
		}
	})

	t.Run("指し先が無い取り消しと二重の取り消しを全部挙げる", func(t *testing.T) {
		files := []judgment.File{
			base,
			fix("2026-10-05-fix.json",
				`{"file": "2026-09-01-none.json", "index": 0, "reason": "x"}`,
				`{"file": "2026-10-01-ai.json", "index": 2, "reason": "x"}`,
				`{"file": "2026-10-01-ai.json", "index": 0, "reason": "x"}`),
			fix("2026-10-06-again.json", `{"file": "2026-10-01-ai.json", "index": 0, "reason": "y"}`),
		}
		p := checkRetractions(files)
		wants := []string{
			"2026-10-05-fix.json: retractions[0]: 取り消す判定のファイル 2026-09-01-none.json がありません",
			"2026-10-05-fix.json: retractions[1]: 2026-10-01-ai.json の判定は 2 件なので、index 2 の判定はありません",
			"2026-10-06-again.json: retractions[0]: 2026-10-01-ai.json の 0 件目は 2026-10-05-fix.json ですでに取り消しています",
		}
		if len(p) != len(wants) {
			t.Fatalf("problems = %v", p)
		}
		for i, want := range wants {
			if !strings.Contains(p[i], want) {
				t.Errorf("problems[%d] = %q, want %q を含む", i, p[i], want)
			}
		}
	})
}

func TestFindForbidden_取り消しの理由も調べる(t *testing.T) {
	f := mustFile(t, "2026-10-05-fix.json", `{"schemaVersion": 2, "loggedAt": "2026-10-05", "source": "manual", "judgments": [], "retractions": [
		{"file": "2026-10-01-ai.json", "index": 0, "reason": "ダミー社の話だった"}]}`)
	hits := findForbidden([]judgment.File{f}, []string{"ダミー社"})
	if len(hits) != 1 || hits[0].Field != "retractions[0].reason" {
		t.Errorf("hits = %+v", hits)
	}
}
