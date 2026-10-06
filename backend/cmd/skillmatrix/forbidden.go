package main

import (
	"fmt"
	"strings"

	"github.com/n-yoshida-dev/skill-matrix/internal/judgment"
)

// このファイルは、公開される文に禁止語が入っていないかを調べる（SPEC.md §6・§9）。
// ファイルを読まず「今日」にも依存しない純粋な関数だけを置く（語のリストの読み込みは main.go と settings.go）。

// forbiddenHit は禁止語が見つかった場所1つ。
type forbiddenHit struct {
	// File は判定ファイル名。
	File string
	// Field は見つかった欄。`judgments[2].rationale` のようにファイルの中での位置を書く。
	Field string
	// Word は見つかった禁止語（リストに書かれたままの綴り）。
	Word string
}

// String は見つかった場所を1行にする。
func (h forbiddenHit) String() string {
	return fmt.Sprintf("%s/%s: %s に「%s」があります", judgmentsDir, h.File, h.Field, h.Word)
}

// findForbidden は判定ファイルの公開される文（各判定の rationale・evidenceRefs と unmatched、取り消しの reason）に禁止語があるかを調べ、見つかった場所を全部返す。
//
// 大文字と小文字は区別しない（社名などの英字の書き方の揺れを拾うため）。
// 形の検査で弾いた判定（Output.Rejected）は見ない。棄却があれば verify はそれだけで失敗し、直したあとの判定がここで調べられる。
func findForbidden(files []judgment.File, words []string) []forbiddenHit {
	var hits []forbiddenHit
	if len(words) == 0 {
		return hits
	}
	lowered := make([]string, len(words))
	for i, w := range words {
		lowered[i] = strings.ToLower(w)
	}
	// check は文1つを全部の語と照らし、見つかった語ごとに1件足す
	check := func(file, field, text string) {
		t := strings.ToLower(text)
		for i, w := range lowered {
			if strings.Contains(t, w) {
				hits = append(hits, forbiddenHit{File: file, Field: field, Word: words[i]})
			}
		}
	}

	for _, f := range files {
		for _, p := range f.Output.Judgments {
			check(f.Name, fmt.Sprintf("judgments[%d].rationale", p.Index), p.Rationale)
			for k, ref := range p.EvidenceRefs {
				check(f.Name, fmt.Sprintf("judgments[%d].evidenceRefs[%d]", p.Index, k), ref)
			}
		}
		for k, u := range f.Output.Unmatched {
			check(f.Name, fmt.Sprintf("unmatched[%d]", k), u)
		}
		// 取り消しの理由も state.json に写って公開される
		for k, r := range f.Retractions {
			check(f.Name, fmt.Sprintf("retractions[%d].reason", k), r.Reason)
		}
	}
	return hits
}
