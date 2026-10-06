package main

import (
	"encoding/json"
	"sort"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
	"github.com/n-yoshida-dev/skill-matrix/internal/judgment"
)

// このファイルは判定ファイルの並びから state.json の中身を組み立てる。
// ファイルを読まず「今日」にも依存しない純粋な関数だけを置く（読み書きは main.go）。

// shapeRejectedCode は形の検査（judgment.Parse）で弾いた判定に付ける識別子。
// V1〜V8 とは別の層の検査なので、コードも別にして後から区別できるようにする
// （棚上げ中の store.ShapeRejectedCode と同じ値）。
const shapeRejectedCode = "shape_rejected"

// dateLayout は state.json に書く日付の書式（YYYY-MM-DD）。
const dateLayout = "2006-01-02"

// recalcResult は再計算の結果。
type recalcResult struct {
	// doc は state.json に書く中身。
	doc stateDoc
	// states は項目ごとの理解度。要約（分野ごとの進捗・次にやること）の計算に使う。
	states map[domain.ItemKey]domain.ItemState
}

// recalculate は判定ファイルをファイル名の昇順・ファイル内は配列の順に適用し、state.json の中身を組み立てる（SPEC.md §3.2）。
//
// 入力の並び順には依存しない（ここで並べ直す）。順序が変わると lastEvidenceAt の更新や
// preState の結果が変わるため、順序はファイル名だけで決まるようにする。
//
// 取り消された判定（retractions。SPEC.md §4.7）は、最初から無かったものとして外してから適用する。
// 指し先の実在は checkRetractions が先に確かめている前提で、指し先の無い取り消しはここでは何もしない。
func recalculate(rm domain.Roadmap, files []judgment.File, rules domain.Rules) recalcResult {
	sorted := sortedByName(files)
	retractions := retractedJudgments(sorted)

	states := make(map[domain.ItemKey]domain.ItemState)
	events := make(map[domain.ItemKey][]eventDoc)
	deferred := []pendingDoc{}
	rejected := []pendingDoc{}
	retracted := []retractedDoc{}

	for _, f := range sorted {
		f, fileRetracted := withoutRetracted(f, retractions)
		retracted = append(retracted, fileRetracted...)
		fileRejected := shapeRejections(f)

		js := f.Output.DomainJudgments(f.Source, f.LoggedAt)
		batch := domain.ApplyJudgments(rm, states, js, rules)
		states = batch.States

		// domain が見る判定の並び（js）は、形の検査で弾いた分が抜けている。
		// state.json にはファイルの judgments 配列での位置を書くので、Proposed.Index で引き直す
		fileIndex := func(i int) int { return f.Output.Judgments[i].Index }

		// 棄却（V1・V2・V3・V8）。domain.BatchResult.Results には入らないので違反から拾う
		for _, v := range batch.Violations {
			if !v.Rejected {
				continue
			}
			fileRejected = append(fileRejected, pendingDoc{
				File: f.Name, Index: fileIndex(v.Index), ItemKey: string(v.ItemKey), Code: string(v.Code), Detail: v.Detail,
			})
		}

		for _, res := range batch.Results {
			idx := fileIndex(res.Index)
			if res.Deferred {
				// 保留（V7）。適用していないので events には入れない
				v := findViolation(res.Violations, domain.ViolationLowConfidence)
				deferred = append(deferred, pendingDoc{
					File: f.Name, Index: idx, ItemKey: string(res.Judgment.ItemKey), Code: string(v.Code), Detail: v.Detail,
				})
				continue
			}
			events[res.Judgment.ItemKey] = append(events[res.Judgment.ItemKey], newEventDoc(f.Name, idx, res))
		}

		// 形の検査の棄却と意味の検査の棄却を、ファイル内の位置の順にそろえる（1 件の判定が棄却されるのは 1 回だけ）
		sort.SliceStable(fileRejected, func(a, b int) bool { return fileRejected[a].Index < fileRejected[b].Index })
		rejected = append(rejected, fileRejected...)
	}

	// 項目はロードマップの定義順。判定が1件も無い項目も入れる（画面は全項目を並べる）
	items := []itemDoc{}
	for _, it := range rm.AllItems() {
		st, ok := states[it.Key]
		if !ok {
			st = domain.ItemState{ItemKey: it.Key}
			states[it.Key] = st
		}
		items = append(items, newItemDoc(st, events[it.Key]))
	}

	return recalcResult{
		doc: stateDoc{
			SchemaVersion: stateSchemaVersion,
			Items:         items,
			Deferred:      deferred,
			Rejected:      rejected,
			Retracted:     retracted,
		},
		states: states,
	}
}

// shapeRejections は形の検査で弾いた判定を rejected の記録に変える。
func shapeRejections(f judgment.File) []pendingDoc {
	out := []pendingDoc{}
	for _, r := range f.Output.Rejected {
		out = append(out, pendingDoc{
			File: f.Name, Index: r.Index, ItemKey: itemKeyOf(r.Raw), Code: shapeRejectedCode, Detail: r.Reason,
		})
	}
	return out
}

// itemKeyOf は形の崩れた判定から、読めるなら itemKey だけを拾う。読めなければ空文字。
// どの項目の判定が弾かれたのかが分かると、直す箇所を探しやすい。
func itemKeyOf(raw json.RawMessage) string {
	var w struct {
		ItemKey any `json:"itemKey"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return ""
	}
	if s, ok := w.ItemKey.(string); ok {
		return s
	}
	return ""
}

// findViolation は違反の一覧から指定の識別子のものを探す。無ければ識別子だけを埋めて返す。
func findViolation(vs []domain.Violation, code domain.ViolationCode) domain.Violation {
	for _, v := range vs {
		if v.Code == code {
			return v
		}
	}
	return domain.Violation{Code: code}
}

// newEventDoc は適用した判定1件を events[] の記録に変える。
func newEventDoc(file string, index int, res domain.Applied) eventDoc {
	j := res.Judgment
	e := eventDoc{
		File:          file,
		Index:         index,
		OccurredAt:    j.OccurredAt.Format(dateLayout),
		Source:        string(j.Source), // judgment.ParseFile が ai / manual / migration のどれかであることを確かめている
		EvidenceType:  string(j.EvidenceType),
		ProposedLevel: int(j.ProposedLevel),
		Marked:        levelsToInts(res.Marked),
		EvidenceRefs:  append([]string{}, j.EvidenceRefs...),
		Rationale:     j.Rationale,
		Violations:    []violationDoc{},
	}
	// 確信度は AI の判定だけが持つ。manual / migration は null（SPEC.md §3.4）
	if j.HasConfidence {
		c := j.Confidence
		e.Confidence = &c
	}
	for _, v := range res.Violations {
		e.Violations = append(e.Violations, violationDoc{Code: string(v.Code), Detail: v.Detail})
	}
	return e
}

// newItemDoc は項目1つの状態を items[] の記録に変える。
func newItemDoc(st domain.ItemState, evs []eventDoc) itemDoc {
	d := itemDoc{
		ItemKey:         string(st.ItemKey),
		VerifiedLevel:   int(st.VerifiedLevel),
		EvidencedLevels: levelsToInts(st.EvidencedLevels()),
		PreState:        string(st.PreState),
		NeedsReview:     st.NeedsReview,
		Events:          evs,
	}
	// ゼロ値の "" は none と同じ意味。state.json は none を要求する（PR #41 のレビューで判明）
	if st.PreState == "" {
		d.PreState = string(domain.PreStateNone)
	}
	if !st.LastEvidenceAt.IsZero() {
		s := st.LastEvidenceAt.Format(dateLayout)
		d.LastEvidenceAt = &s
	}
	if d.Events == nil {
		d.Events = []eventDoc{}
	}
	return d
}

// levelsToInts はレベルの並びを数値の並びにする。空でも null ではなく [] にする。
func levelsToInts(ls []domain.Level) []int {
	out := make([]int, 0, len(ls))
	for _, l := range ls {
		out = append(out, int(l))
	}
	return out
}
