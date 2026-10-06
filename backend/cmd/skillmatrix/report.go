package main

import (
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
)

// このファイルは端末に出す文面を組み立てる。state.json には影響しない。
// 「次にやること」と鮮度は「今日」に依存するので、ここ（端末の表示）でだけ使う。

// printViolations は棄却・保留・記録（V5・V6）を「どのファイルの何件目の、どの項目の、どのルールか」の形で出す（SPEC.md §6）。
func printViolations(w io.Writer, doc stateDoc) {
	fmt.Fprintf(w, "棄却（適用していない。verify を失敗にする）: %d 件\n", len(doc.Rejected))
	for _, p := range doc.Rejected {
		fmt.Fprintf(w, "  %s\n", pendingLine(p))
	}
	fmt.Fprintf(w, "保留（V7。適用していない。採用するなら manual の判定を足す）: %d 件\n", len(doc.Deferred))
	for _, p := range doc.Deferred {
		fmt.Fprintf(w, "  %s\n", pendingLine(p))
	}
	fmt.Fprintf(w, "取り消し（適用していない。retractions で外した）: %d 件\n", len(doc.Retracted))
	for _, r := range doc.Retracted {
		key := r.ItemKey
		if key == "" {
			key = "(itemKey 不明)"
		}
		fmt.Fprintf(w, "  %s judgments[%d] %s: %s で取り消し（%s）\n", r.File, r.Index, key, r.RetractedBy, r.Reason)
	}

	// V5（梯子の範囲への切り詰め）と V6（不合格の報告）は適用済みの記録。モデルの想定どおりの動きなので失敗にはしない
	type recorded struct {
		file  string
		index int
		line  string
	}
	var rs []recorded
	for _, it := range doc.Items {
		for _, e := range it.Events {
			for _, v := range e.Violations {
				rs = append(rs, recorded{e.File, e.Index, fmt.Sprintf("%s judgments[%d] %s %s: %s", e.File, e.Index, it.ItemKey, v.Code, v.Detail)})
			}
		}
	}
	// 項目の順ではなく、処理の順（ファイル名・配列の位置）で並べる
	sort.SliceStable(rs, func(a, b int) bool {
		if rs[a].file != rs[b].file {
			return rs[a].file < rs[b].file
		}
		return rs[a].index < rs[b].index
	})
	fmt.Fprintf(w, "記録（V5・V6。適用済み）: %d 件\n", len(rs))
	for _, r := range rs {
		fmt.Fprintf(w, "  %s\n", r.line)
	}
}

// pendingLine は保留・棄却1件を1行にする。
func pendingLine(p pendingDoc) string {
	key := p.ItemKey
	if key == "" {
		key = "（itemKey 不明）"
	}
	return fmt.Sprintf("%s judgments[%d] %s %s: %s", p.File, p.Index, key, p.Code, p.Detail)
}

// countEvents は適用した判定の件数を数える。
func countEvents(doc stateDoc) int {
	n := 0
	for _, it := range doc.Items {
		n += len(it.Events)
	}
	return n
}

// printSummary は recalc の要約（分野ごとの進捗・変わった項目・次にやること）を出す（SPEC.md §6）。
// prev は書き直す前の state.json。無い・読めないときは nil で、変わった項目は出さない。
func printSummary(w io.Writer, rm domain.Roadmap, res recalcResult, prev *stateDoc, fileCount int, s settings, now time.Time) {
	doc := res.doc
	fmt.Fprintf(w, "判定ファイル %d 本 / 適用 %d 件・保留 %d 件・棄却 %d 件\n\n",
		fileCount, countEvents(doc), len(doc.Deferred), len(doc.Rejected))

	fmt.Fprintln(w, "分野ごとの進捗（レベル別の項目数は L0〜L5）")
	for _, sum := range domain.RollupAll(rm, res.states, now, s.staleness) {
		fmt.Fprintf(w, "  %s（%s）: %3.0f%%  %d 項目  %v  要再確認 %d  基礎の確認待ち %d\n",
			sum.DomainName, sum.DomainKey, sum.Progress*100, sum.TotalItems, sum.ByLevel, sum.StaleCount, sum.PendingCount)
	}
	fmt.Fprintln(w)

	if prev == nil {
		fmt.Fprintln(w, "変わった項目: （前回の state.json が無い・読めないので出さない）")
	} else {
		changed := changedItems(*prev, doc)
		fmt.Fprintf(w, "変わった項目: %d 件\n", len(changed))
		for _, c := range changed {
			fmt.Fprintf(w, "  %s\n", c)
		}
	}
	fmt.Fprintln(w)

	actions := domain.NextActions(rm, res.states, now, s.staleness, s.weights, s.nextActionsLimit)
	fmt.Fprintf(w, "次にやること（上位 %d 件。今日 %s 時点）\n", s.nextActionsLimit, now.Format(dateLayout))
	for i, a := range actions {
		note := ""
		if a.Blocked {
			note = "（依存する項目が未達）"
		}
		if len(a.PendingLevels) > 0 {
			note += fmt.Sprintf("（印 %v あり。既存の実装について基礎 L1/L2 を確認する）", a.PendingLevels)
		}
		fmt.Fprintf(w, "  %d. %s %s（レベル %d）%s\n", i+1, a.ItemKey, a.Name, a.Level, note)
		if a.VerifyBy != "" {
			fmt.Fprintf(w, "     確認方法: %s\n", a.VerifyBy)
		}
	}
}

// changedItems は書き直す前と後の state.json を比べ、変わった項目を1行ずつ返す。
// 前回に無かった項目は「判定がまだ無い状態」から変わったものとして比べる。
func changedItems(prev, cur stateDoc) []string {
	before := make(map[string]itemDoc, len(prev.Items))
	for _, it := range prev.Items {
		before[it.ItemKey] = it
	}

	var out []string
	for _, it := range cur.Items {
		old, ok := before[it.ItemKey]
		if !ok {
			old = itemDoc{ItemKey: it.ItemKey, PreState: string(domain.PreStateNone)}
		}
		var parts []string
		if old.VerifiedLevel != it.VerifiedLevel {
			parts = append(parts, fmt.Sprintf("レベル %d → %d", old.VerifiedLevel, it.VerifiedLevel))
		}
		if !slices.Equal(old.EvidencedLevels, it.EvidencedLevels) {
			parts = append(parts, fmt.Sprintf("印 %v → %v", old.EvidencedLevels, it.EvidencedLevels))
		}
		if old.PreState != it.PreState {
			parts = append(parts, fmt.Sprintf("段階前 %s → %s", old.PreState, it.PreState))
		}
		if old.NeedsReview != it.NeedsReview {
			parts = append(parts, fmt.Sprintf("要再確認 %v → %v", old.NeedsReview, it.NeedsReview))
		}
		if dateOrDash(old.LastEvidenceAt) != dateOrDash(it.LastEvidenceAt) {
			parts = append(parts, fmt.Sprintf("最終根拠 %s → %s", dateOrDash(old.LastEvidenceAt), dateOrDash(it.LastEvidenceAt)))
		}
		if len(parts) > 0 {
			out = append(out, it.ItemKey+": "+strings.Join(parts, "、"))
		}
	}
	return out
}

// dateOrDash は null の日付を「-」にする。
func dateOrDash(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}
