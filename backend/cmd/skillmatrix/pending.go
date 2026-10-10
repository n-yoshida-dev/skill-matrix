package main

import (
	"fmt"
	"path"
	"regexp"
	"sort"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
	"github.com/n-yoshida-dev/skill-matrix/internal/judgment"
)

// このファイルは `pending` モード（SPEC.md §6）の中身のうち、git に触らない部分を持つ。
// 未判定の学習ログを探す規則（SPEC.md §3.5。prompts/judge.md 手順 1 が使う）をここに集め、単体テストで固定する。
// git を呼んで材料（repoSnapshot）を集める部分は pending_git.go にある。

// repoSnapshot は学習ログのリポジトリ 1 つについて、git から集めた材料。
type repoSnapshot struct {
	// Repo は `<owner>/<repo>`（sources.local.json の repos のキー。evidenceRefs の repo: と同じ名前）。
	Repo string
	// Files は logsGlob に合う、git が追跡しているファイル（リポジトリ内のパス。git add だけしてまだコミットしていないものも入る）。
	Files []string
	// Uncommitted は logsGlob に合うファイルのうち、コミットされていない変更があるもの（add だけしたもの・まだ追跡していないものを含む）。
	Uncommitted []string
	// Freeze はこのリポジトリが移行の凍結コミットを持つときだけ埋める。
	Freeze *freezeSnapshot
}

// freezeSnapshot は移行の凍結コミット（移行元の台帳を凍結したコミット）の時点の情報。
type freezeSnapshot struct {
	// Commit は凍結コミット。
	Commit string
	// Date は凍結コミットの日付（YYYY-MM-DD。author date）。書き起こしのログを見分けるのに使う。
	Date string
	// Changed は凍結コミットのあと（HEAD まで）に変更されたファイル。
	Changed []string
	// Existed は凍結コミットの時点にあったファイル。
	Existed []string
}

// freezePoint は移行の判定が指す凍結コミット。
type freezePoint struct {
	Repo, Commit string
}

// pendingLog は未判定のログ 1 本。
type pendingLog struct {
	// Repo は `<owner>/<repo>`。log:（このリポジトリ内のコミットしない置き場）なら ""。
	Repo string
	// Path はリポジトリ内のパス。log: ならこのリポジトリのルートからのパス。
	Path string
	// Commit は判定ファイルの evidenceRefs に書くコミット（そのログを最後に変更したコミット。prompts/judge.md「出力の形」）。
	// findPending は埋めない（git が要る）。呼び出し側（pending_git.go）が埋める。log: なら ""。
	Commit string
	// DiffBase は差分だけを判定するときの基準コミット（凍結コミット）。空なら全文を判定する。
	DiffBase string
	// Backfilled は、凍結コミットより前の日付なのに凍結のあとで作られたログ（前の記録から書き起こしたもの）。
	// 全文を読むが、判定するのは台帳に写っていない出来事だけだと判定する AI に知らせる（prompts/judge.md 手順 1。logs/decisions.md 2026-10-10）。
	Backfilled bool
}

// skippedLog は候補だったが今は判定しないログ 1 本と、その理由。
type skippedLog struct {
	Repo, Path, Reason string
}

// logDatePattern はログのファイル名の先頭の日付。並べ替えと書き起こしの見分けに使う。
var logDatePattern = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})`)

// logDate はログのファイル名の先頭の日付を返す。無ければ ""。
func logDate(p string) string {
	m := logDatePattern.FindStringSubmatch(path.Base(p))
	if m == nil {
		return ""
	}
	return m[1]
}

// judgedLogs は判定ファイルの evidenceRefs から、判定済みのログを集める。キーは "<repo>:<path>"（log: は ":<path>"）。
// commit の違いは見ない（追記型のログは、同じ path なら判定済みとみなす。SPEC.md §3.5）。
// 取り消された判定（retractions）の根拠も数える（取り消しは訂正で、ログを読み直すきっかけではない。SPEC.md §3.5）。
// 読めない根拠（repo: / log: の文法の崩れ）は warnings に返す。黙って飛ばすと、判定済みのログが「未判定」に戻って見える。
func judgedLogs(files []judgment.File) (map[string]bool, []string) {
	out := make(map[string]bool)
	var warnings []string
	for _, f := range files {
		for _, p := range f.Output.Judgments {
			for k, raw := range p.EvidenceRefs {
				ref, err := parseEvidenceRef(raw)
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("%s の judgments[%d].evidenceRefs[%d] を読めないので、判定済みかどうかの判断に使っていません: %v", f.Name, p.Index, k, err))
					continue
				}
				if ref.Path == "" {
					continue
				}
				switch ref.Kind {
				case "repo":
					out[ref.Repo+":"+ref.Path] = true
				case "log":
					out[":"+ref.Path] = true
				}
			}
		}
	}
	return out, warnings
}

// findFreeze は移行の判定ファイル（source: migration）から凍結コミットを読む。
// 各判定の evidenceRefs の 1 件目が移行元の台帳を指す。移行の判定が無ければ ok は false。
// 違うコミットが混ざっていればエラー（どこで凍結したかが決まらない。prompts/judge.md 手順 1 は止まって報告する）。
func findFreeze(files []judgment.File) (freezePoint, bool, error) {
	var fp freezePoint
	found := false
	for _, f := range files {
		if f.Source != domain.SourceMigration {
			continue
		}
		for _, p := range f.Output.Judgments {
			if len(p.EvidenceRefs) == 0 {
				continue
			}
			ref, err := parseEvidenceRef(p.EvidenceRefs[0])
			if err != nil || ref.Kind != "repo" {
				return freezePoint{}, false, fmt.Errorf("%s の judgments[%d] の evidenceRefs の 1 件目が移行元の台帳（repo:）を指していません: %q", f.Name, p.Index, p.EvidenceRefs[0])
			}
			got := freezePoint{Repo: ref.Repo, Commit: ref.Commit}
			if found && got != fp {
				return freezePoint{}, false, fmt.Errorf("移行の判定の凍結コミットが揃っていません（%s@%s と %s@%s。%s の judgments[%d]）",
					fp.Repo, fp.Commit, got.Repo, got.Commit, f.Name, p.Index)
			}
			fp, found = got, true
		}
	}
	return fp, found, nil
}

// findPending は未判定のログを、古い順に並べて返す（SPEC.md §3.5 の自動の探索。prompts/judge.md 手順 1）。
//
// 候補は各リポジトリの Files と、このリポジトリ内のコミットしない置き場のログ（localLogs）。そこから次の順に外す。
//   - ① 判定済み：判定ファイルの evidenceRefs に同じリポジトリの同じ path がある（commit の違いは見ない）
//   - ③ 未コミット：commit を書けないので判定しない。skipped に理由を付けて返す（先にそのリポジトリでコミットしてもらう）。
//     ② より先に当てる。git add だけした新しいログは、凍結後の変更（コミットどうしの比較）に出てこないので、② を先にすると黙って消える
//   - ② 移行で写し済み：凍結コミットのあと一度も変更されていない（台帳を経由して写し済み。古いログを判定し直すと、台帳が意図して上げなかった段を付け直してしまう）
//
// 凍結前からあって凍結後に変わったログは、凍結コミットからの差分だけを判定する（DiffBase）。
// 並びはファイル名の日付の古い順（新しいログを先に判定すると「要再確認」の付き方が変わる）。日付の無いものは最後。
func findPending(snaps []repoSnapshot, localLogs []string, judged map[string]bool) ([]pendingLog, []skippedLog) {
	var pending []pendingLog
	var skipped []skippedLog

	for _, s := range snaps {
		uncommitted := toSet(s.Uncommitted)
		var changed, existed map[string]bool
		if s.Freeze != nil {
			changed, existed = toSet(s.Freeze.Changed), toSet(s.Freeze.Existed)
		}
		for _, p := range s.Files {
			if judged[s.Repo+":"+p] {
				continue // ①
			}
			if uncommitted[p] {
				skipped = append(skipped, skippedLog{Repo: s.Repo, Path: p, Reason: "コミットされていない変更がある"})
				continue // ③
			}
			if s.Freeze != nil && !changed[p] {
				continue // ②
			}
			pl := pendingLog{Repo: s.Repo, Path: p}
			if s.Freeze != nil {
				if existed[p] {
					pl.DiffBase = s.Freeze.Commit
				} else if d := logDate(p); d != "" && d < s.Freeze.Date {
					pl.Backfilled = true
				}
			}
			pending = append(pending, pl)
		}
		// まだ git が追跡していないログは Files に無い。気づけるように skipped に出す
		files := toSet(s.Files)
		for _, p := range s.Uncommitted {
			if !files[p] && !judged[s.Repo+":"+p] {
				skipped = append(skipped, skippedLog{Repo: s.Repo, Path: p, Reason: "まだコミットされていない"})
			}
		}
	}

	// コミットしない置き場のログは ① だけを当てる（commit が無いので ② ③ は意味を持たない）
	for _, p := range localLogs {
		if !judged[":"+p] {
			pending = append(pending, pendingLog{Path: p})
		}
	}

	sort.SliceStable(pending, func(a, b int) bool {
		return logLess(pending[a].Repo, pending[a].Path, pending[b].Repo, pending[b].Path)
	})
	sort.SliceStable(skipped, func(a, b int) bool {
		return logLess(skipped[a].Repo, skipped[a].Path, skipped[b].Repo, skipped[b].Path)
	})
	return pending, skipped
}

// logLess はログの並び順。ファイル名の日付の古い順、日付の無いものは最後。同じならリポジトリ・パスの順。
func logLess(repoA, pathA, repoB, pathB string) bool {
	da, db := logDate(pathA), logDate(pathB)
	if (da == "") != (db == "") {
		return da != ""
	}
	if da != db {
		return da < db
	}
	if repoA != repoB {
		return repoA < repoB
	}
	return pathA < pathB
}

// toSet は文字列の並びを集合にする。
func toSet(ss []string) map[string]bool {
	out := make(map[string]bool, len(ss))
	for _, s := range ss {
		out[s] = true
	}
	return out
}
