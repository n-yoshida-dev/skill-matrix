package main

import (
	"fmt"
	"sort"

	"github.com/n-yoshida-dev/skill-matrix/internal/judgment"
)

// このファイルは取り消しの記録（判定ファイルの retractions。SPEC.md §3.3・§4.7）を扱う。
// ファイルを読まず「今日」にも依存しない純粋な関数だけを置く。
//
// 取り消された判定は「最初から無かったもの」として再計算から外す。理解度は毎回すべての判定から
// 計算し直すので、外すだけで、その判定が付けた印は消え、別の有効な判定が付けた印は残る。

// judgmentRef は判定1件の場所（ファイル名と judgments 配列での位置）。
type judgmentRef struct {
	File  string
	Index int
}

// retractionOf は取り消された判定1件について、どのファイルがなぜ取り消したか。
type retractionOf struct {
	// By は取り消しを書いたファイル名。
	By string
	// Reason は取り消す理由。
	Reason string
}

// checkRetractions は取り消しの指し先が実在することと、同じ判定を2回取り消していないことを確かめ、
// 見つかった不備を全部返す（空なら問題なし）。
//
// 1ファイルの中で分かること（書式・source・自分より前のファイルか）は judgment.ParseFile が見ている。
// 指し先が無い取り消しを黙って捨てると、取り消したつもりの印が残るので、データの不備として止める。
func checkRetractions(files []judgment.File) []string {
	// sizes はファイルごとの judgments 配列の長さ（形の検査で弾いた要素も数える）
	sizes := make(map[string]int, len(files))
	for _, f := range files {
		sizes[f.Name] = len(f.Output.Judgments) + len(f.Output.Rejected)
	}

	var problems []string
	firstBy := make(map[judgmentRef]string)
	for _, f := range sortedByName(files) {
		for i, r := range f.Retractions {
			where := fmt.Sprintf("%s/%s: retractions[%d]", judgmentsDir, f.Name, i)
			size, ok := sizes[r.File]
			switch {
			case !ok:
				problems = append(problems, fmt.Sprintf("%s: 取り消す判定のファイル %s がありません", where, r.File))
				continue
			case r.Index >= size:
				problems = append(problems, fmt.Sprintf("%s: %s の判定は %d 件なので、index %d の判定はありません", where, r.File, size, r.Index))
				continue
			}
			ref := judgmentRef{File: r.File, Index: r.Index}
			if by, dup := firstBy[ref]; dup {
				problems = append(problems, fmt.Sprintf("%s: %s の %d 件目は %s ですでに取り消しています", where, r.File, r.Index, by))
				continue
			}
			firstBy[ref] = f.Name
		}
	}
	return problems
}

// retractedJudgments は取り消された判定の一覧を、判定の場所から引ける形で返す。
// 同じ判定を2回取り消していれば先のファイルを採る（checkRetractions が止めるので、通常は起きない）。
func retractedJudgments(files []judgment.File) map[judgmentRef]retractionOf {
	out := make(map[judgmentRef]retractionOf)
	for _, f := range sortedByName(files) {
		for _, r := range f.Retractions {
			ref := judgmentRef{File: r.File, Index: r.Index}
			if _, dup := out[ref]; dup {
				continue
			}
			out[ref] = retractionOf{By: f.Name, Reason: r.Reason}
		}
	}
	return out
}

// withoutRetracted は判定ファイルから取り消された判定を外した写しと、外した判定の記録を返す。
//
// 形の検査で弾いた判定も外す（取り消せば rejected にも残らない）。元の File は変更しない。
func withoutRetracted(f judgment.File, retracted map[judgmentRef]retractionOf) (judgment.File, []retractedDoc) {
	var docs []retractedDoc
	record := func(index int, itemKey string) bool {
		r, ok := retracted[judgmentRef{File: f.Name, Index: index}]
		if ok {
			docs = append(docs, retractedDoc{File: f.Name, Index: index, ItemKey: itemKey, RetractedBy: r.By, Reason: r.Reason})
		}
		return ok
	}

	out := f
	out.Output.Judgments = nil
	for _, p := range f.Output.Judgments {
		if !record(p.Index, p.ItemKey) {
			out.Output.Judgments = append(out.Output.Judgments, p)
		}
	}
	out.Output.Rejected = nil
	for _, r := range f.Output.Rejected {
		if !record(r.Index, itemKeyOf(r.Raw)) {
			out.Output.Rejected = append(out.Output.Rejected, r)
		}
	}

	// 判定と形の崩れた判定を、ファイル内の位置の順にそろえる
	sort.SliceStable(docs, func(a, b int) bool { return docs[a].Index < docs[b].Index })
	return out, docs
}

// sortedByName はファイル名の昇順（＝処理順）に並べた写しを返す。
func sortedByName(files []judgment.File) []judgment.File {
	sorted := append([]judgment.File(nil), files...)
	sort.SliceStable(sorted, func(a, b int) bool { return sorted[a].Name < sorted[b].Name })
	return sorted
}
