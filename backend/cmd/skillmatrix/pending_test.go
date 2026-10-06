package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/n-yoshida-dev/skill-matrix/internal/judgment"
)

// このテストは未判定のログを探す規則（prompts/judge.md 手順 2。SPEC.md §3.5）を具体例で固定する。
// git に触らない findPending と findFreeze を、材料を手で組み立てて確かめる。git を呼ぶ部分は TestRun_pending が一時リポジトリで確かめる。

// pendingPaths は未判定のログを「パス（差分の基準・書き起こし）」の並びにする。比べやすくするため。
func pendingPaths(ps []pendingLog) []string {
	var out []string
	for _, p := range ps {
		s := p.Repo + ":" + p.Path
		if p.DiffBase != "" {
			s += " 差分@" + p.DiffBase
		}
		if p.Backfilled {
			s += " 書き起こし"
		}
		out = append(out, s)
	}
	return out
}

// judgedFile は study のログを根拠に持つ AI の判定ファイル。
func judgedFile(t *testing.T, name string, refs ...string) judgment.File {
	t.Helper()
	return mustFile(t, name, `{"schemaVersion": 2, "loggedAt": "`+name[:10]+`", "source": "ai", "judgments": [
		{"itemKey": "go-01", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["`+strings.Join(refs, `", "`)+`"], "rationale": "x", "confidence": 0.9}]}`)
}

// migrationFile は移行の判定ファイル。各判定の evidenceRefs の 1 件目が凍結コミットの台帳を指す。
func migrationFile(t *testing.T, name string, ledgerRefs ...string) judgment.File {
	t.Helper()
	var js []string
	for _, r := range ledgerRefs {
		js = append(js, `{"itemKey": "go-01", "evidenceType": "implementation", "proposedLevel": 3, "evidenceRefs": ["`+r+`"], "rationale": "x"}`)
	}
	return mustFile(t, name, `{"schemaVersion": 2, "loggedAt": "`+name[:10]+`", "source": "migration", "judgments": [`+strings.Join(js, ",")+`]}`)
}

func TestFindPending(t *testing.T) {
	study := repoSnapshot{
		Repo: "o/study",
		Head: "fff0000",
		Files: []string{
			"go/logs/2026-09-20.md",      // 凍結前からあり、凍結後に変わっていない → ② で外す
			"go/logs/2026-09-25.md",      // 凍結前からあり、凍結後に追記された → 凍結からの差分
			"go/logs/2026-09-24.md",      // 凍結後に作られたが、日付は凍結より前 → 書き起こし（全文）
			"go/logs/2026-10-02.md",      // 凍結後の新しいログ → 全文
			"go/logs/2026-10-01.md",      // 判定済み（commit は違う）→ ① で外す
			"go/logs/2026-10-03.md",      // コミットされていない変更がある → ③ で外して理由を出す
			"go/logs/notes.md",           // 日付の無いログ → 最後に並ぶ
			"orgflow/logs/2026-09-30.md", // 凍結後の新しいログ。日付順で go の 10-02 より前
		},
		Uncommitted: []string{"go/logs/2026-10-03.md", "go/logs/2026-10-04.md"},
		Freeze: &freezeSnapshot{
			Commit:  "abc1234",
			Date:    "2026-09-27",
			Changed: []string{"go/logs/2026-09-25.md", "go/logs/2026-09-24.md", "go/logs/2026-10-02.md", "go/logs/2026-10-01.md", "go/logs/2026-10-03.md", "go/logs/notes.md", "orgflow/logs/2026-09-30.md"},
			Existed: []string{"go/logs/2026-09-20.md", "go/logs/2026-09-25.md"},
		},
	}
	judged := judgedLogs([]judgment.File{judgedFile(t, "2026-10-02-ai.json", "repo:o/study@0001111/go/logs/2026-10-01.md#L3")})

	pending, skipped := findPending([]repoSnapshot{study}, nil, judged)

	want := []string{
		"o/study:go/logs/2026-09-24.md 書き起こし",
		"o/study:go/logs/2026-09-25.md 差分@abc1234",
		"o/study:orgflow/logs/2026-09-30.md",
		"o/study:go/logs/2026-10-02.md",
		"o/study:go/logs/notes.md",
	}
	if got := pendingPaths(pending); !slices.Equal(got, want) {
		t.Errorf("pending =\n%v\nwant\n%v", got, want)
	}
	for _, p := range pending {
		if p.Head != "fff0000" {
			t.Errorf("%s の Head = %q（evidenceRefs に書く今のコミット）", p.Path, p.Head)
		}
	}

	wantSkipped := []skippedLog{
		{Repo: "o/study", Path: "go/logs/2026-10-03.md", Reason: "コミットされていない変更がある"},
		{Repo: "o/study", Path: "go/logs/2026-10-04.md", Reason: "まだコミットされていない"},
	}
	if !slices.Equal(skipped, wantSkipped) {
		t.Errorf("skipped = %+v, want %+v", skipped, wantSkipped)
	}
}

func TestFindPending_凍結の無いリポジトリは全文(t *testing.T) {
	other := repoSnapshot{Repo: "o/other", Head: "aaa", Files: []string{"logs/2026-09-01.md"}}
	pending, _ := findPending([]repoSnapshot{other}, nil, map[string]bool{})
	if got := pendingPaths(pending); !slices.Equal(got, []string{"o/other:logs/2026-09-01.md"}) {
		t.Errorf("pending = %v", got)
	}
}

func TestFindPending_コミットしない置き場のログ(t *testing.T) {
	judged := judgedLogs([]judgment.File{judgedFile(t, "2026-10-02-ai.json", "log:learning-logs/2026-10-01.md")})
	pending, _ := findPending(nil, []string{"learning-logs/2026-10-01.md", "learning-logs/2026-10-02.md"}, judged)
	if got := pendingPaths(pending); !slices.Equal(got, []string{":learning-logs/2026-10-02.md"}) {
		t.Errorf("pending = %v", got)
	}
}

func TestFindFreeze(t *testing.T) {
	t.Run("移行の判定の 1 件目の台帳から凍結コミットを読む", func(t *testing.T) {
		files := []judgment.File{
			judgedFile(t, "2026-10-02-ai.json", "repo:o/study@0001111/go/logs/2026-10-01.md"),
			migrationFile(t, "2026-09-27-migration-go.json", "repo:o/study@abc1234/ledger.md#L3", "repo:o/study@abc1234/ledger.md#L9"),
		}
		fp, ok, err := findFreeze(files)
		if err != nil || !ok || fp != (freezePoint{Repo: "o/study", Commit: "abc1234"}) {
			t.Errorf("fp = %+v, ok = %v, err = %v", fp, ok, err)
		}
	})

	t.Run("移行の判定が無ければ凍結は無い", func(t *testing.T) {
		if _, ok, err := findFreeze([]judgment.File{judgedFile(t, "2026-10-02-ai.json", "log:a.md")}); ok || err != nil {
			t.Errorf("ok = %v, err = %v", ok, err)
		}
	})

	t.Run("凍結コミットが揃っていなければエラー", func(t *testing.T) {
		files := []judgment.File{
			migrationFile(t, "2026-09-27-migration-go.json", "repo:o/study@abc1234/ledger.md#L3"),
			migrationFile(t, "2026-09-27-migration-react.json", "repo:o/study@def5678/ledger.md#L3"),
		}
		if _, _, err := findFreeze(files); err == nil || !strings.Contains(err.Error(), "揃っていません") {
			t.Errorf("err = %v", err)
		}
	})
}

func TestRun_pending(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git が無い")
	}
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := gitOutput(repo, append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	// 凍結前のログ 2 本と台帳 → 凍結 → 1 本に追記・新しいログ（日本語のパス）・判定済みのログ・未コミットのログ
	git("init", "-q")
	writeFile(t, filepath.Join(repo, "ledger.md"), "台帳\n")
	if err := os.MkdirAll(filepath.Join(repo, "go", "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "go", "logs", "2026-09-20.md"), "古い\n")
	writeFile(t, filepath.Join(repo, "go", "logs", "2026-09-25.md"), "凍結前\n")
	git("add", ".")
	git("commit", "-q", "--date=2026-09-27T12:00:00", "-m", "凍結")
	freeze := git("rev-parse", "--short", "HEAD")

	writeFile(t, filepath.Join(repo, "go", "logs", "2026-09-25.md"), "凍結前\n追記\n")
	writeFile(t, filepath.Join(repo, "go", "logs", "2026-10-01-判定済み.md"), "済み\n")
	writeFile(t, filepath.Join(repo, "go", "logs", "2026-10-02-新しい.md"), "新しい\n")
	git("add", ".")
	git("commit", "-q", "-m", "続き")
	writeFile(t, filepath.Join(repo, "go", "logs", "2026-10-03.md"), "書きかけ\n")
	head := git("rev-parse", "--short", "HEAD")

	dir := newDataDir(t, map[string]string{
		"2026-09-27-migration-go.json": `{"schemaVersion": 2, "loggedAt": "2026-09-27", "source": "migration", "judgments": [
			{"itemKey": "go-01", "evidenceType": "implementation", "proposedLevel": 3, "evidenceRefs": ["repo:o/study@` + freeze + `/ledger.md#L1"], "rationale": "x"}]}`,
		"2026-10-02-ai.json": `{"schemaVersion": 2, "loggedAt": "2026-10-02", "source": "ai", "judgments": [
			{"itemKey": "go-01", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["repo:o/study@` + head + `/go/logs/2026-10-01-判定済み.md#L1"], "rationale": "x", "confidence": 0.9}]}`,
	})
	sources := filepath.Join(dir, "sources.local.json")
	writeFile(t, sources, `{"repos": {"o/study": {"path": "`+filepath.ToSlash(repo)+`", "logsGlob": "*/logs/*.md"}}}`)

	code, out, errOut := runCLI("pending", "--data", dir, "--sources", sources)
	if code != exitOK {
		t.Fatalf("終了コード = %d, stderr = %s", code, errOut)
	}
	wants := []string{
		"未判定のログ 2 本",
		"1. repo:o/study@" + head + "/go/logs/2026-09-25.md（凍結コミット " + freeze + " からの差分だけ）",
		"2. repo:o/study@" + head + "/go/logs/2026-10-02-新しい.md（全文）",
		"go/logs/2026-10-03.md：まだコミットされていない",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("%q が無い:\n%s", want, out)
		}
	}
	if strings.Contains(out, "2026-09-20.md") || strings.Contains(out, "判定済み.md") {
		t.Errorf("外すはずのログが出ている:\n%s", out)
	}

	code, out, _ = runCLI("pending", "--data", dir, "--sources", sources, "--count")
	if code != exitOK || out != "2\n" {
		t.Errorf("--count: 終了コード = %d, stdout = %q", code, out)
	}
}
