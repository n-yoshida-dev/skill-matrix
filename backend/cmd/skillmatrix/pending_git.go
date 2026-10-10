package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// このファイルは `pending` モードの入口と、git を呼んで材料を集める部分を持つ。
// 未判定かどうかを決める規則は pending.go（git に触らない）にある。

// pendingSynopsis は pending モードの使い方。
const pendingSynopsis = "使い方: skillmatrix pending [--data <data/ の場所>] [--sources <sources.local.json の場所>] [--count]"

// snapshotRepo は学習ログのリポジトリ 1 つから、未判定のログを探す材料を git で集める。
// freeze が空でなければ、そのコミットの時点の情報も集める。
func snapshotRepo(repo, repoPath, logsGlob, freeze string) (repoSnapshot, error) {
	s := repoSnapshot{Repo: repo}
	pathspec := ":(glob)" + logsGlob

	var err error
	if s.Files, err = gitLines(repoPath, "ls-files", "-z", "--", pathspec); err != nil {
		return s, err
	}
	if s.Uncommitted, err = gitStatusPaths(repoPath, pathspec); err != nil {
		return s, err
	}

	if freeze == "" {
		return s, nil
	}
	fz := &freezeSnapshot{Commit: freeze}
	date, err := gitOutput(repoPath, "show", "-s", "--format=%as", freeze)
	if err != nil {
		return s, fmt.Errorf("凍結コミット %s が手元の %s にありません: %w", freeze, repo, err)
	}
	fz.Date = strings.TrimSpace(string(date))
	if fz.Changed, err = gitLines(repoPath, "diff", "--name-only", "-z", freeze, "HEAD", "--", pathspec); err != nil {
		return s, err
	}
	if fz.Existed, err = gitLines(repoPath, "ls-tree", "-r", "--name-only", "-z", freeze); err != nil {
		return s, err
	}
	s.Freeze = fz
	return s, nil
}

// gitLines は git の -z 付きの出力（NUL 区切り）をパスの並びにする。-z にするのは、日本語のパスが引用符と 8 進数に化けないようにするため。
func gitLines(repoPath string, args ...string) ([]string, error) {
	out, err := gitOutput(repoPath, args...)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// gitStatusPaths はコミットされていない変更があるファイル（まだ追跡していないものを含む）のパスを返す。
// `git status --porcelain=v1 -z` の各要素は "XY <path>"。名前の変更（R・C）は次の要素に元の名前が続くので読み飛ばす。
func gitStatusPaths(repoPath, pathspec string) ([]string, error) {
	out, err := gitOutput(repoPath, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--", pathspec)
	if err != nil {
		return nil, err
	}
	var paths []string
	entries := strings.Split(string(out), "\x00")
	for i := 0; i < len(entries); i++ {
		e := entries[i]
		if len(e) < 4 {
			continue
		}
		paths = append(paths, e[3:])
		if e[0] == 'R' || e[0] == 'C' {
			i++ // 元の名前
		}
	}
	return paths, nil
}

// listLocalLogs はこのリポジトリ内のコミットしない置き場（sources.local.json の logs.path）のファイルを、ルートからのパスで返す。
// 置き場が無ければ 0 件。`.` で始まるファイル（.gitkeep 等）は読まない。
func listLocalLogs(rootDir, logsPath string) ([]string, error) {
	if logsPath == "" {
		return nil, nil
	}
	base := filepath.Join(rootDir, filepath.FromSlash(logsPath))
	var out []string
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") && p != base {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(rootDir, p)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return out, err
}

// runPending は pending モードの本体。終了コードを返す。
func runPending(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("pending", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataDir := flags.String("data", defaultDataDir, "data/ ディレクトリの場所（judgments/ を読む）")
	sourcesPath := flags.String("sources", defaultSourcesFile, "sources.local.json の場所（学習ログのリポジトリの手元の場所）")
	countOnly := flags.Bool("count", false, "未判定の本数だけを出す（起動時の表示・ダッシュボード用）")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "余分な引数があります: %v\n%s\n", flags.Args(), pendingSynopsis)
		return exitUsage
	}

	sources, code := readSources(*sourcesPath, stderr)
	if code != exitOK {
		return code
	}
	files, err := loadJudgments(filepath.Join(*dataDir, judgmentsDir))
	if err != nil {
		return reportError(stderr, err)
	}
	freeze, hasFreeze, err := findFreeze(files)
	if err != nil {
		fmt.Fprintf(stderr, "エラー: %v\n", err)
		return exitFailed
	}

	// map の反復順は毎回変わるので、リポジトリ名の順に集める
	repos := make([]string, 0, len(sources.Repos))
	for name := range sources.Repos {
		repos = append(repos, name)
	}
	sort.Strings(repos)
	var snaps []repoSnapshot
	for _, name := range repos {
		r := sources.Repos[name]
		if r.LogsGlob == "" {
			fmt.Fprintf(stderr, "警告: sources.local.json の %s に logsGlob が無いので、学習ログを探しません\n", name)
			continue
		}
		fz := ""
		if hasFreeze && freeze.Repo == name {
			fz = freeze.Commit
		}
		s, err := snapshotRepo(name, r.Path, r.LogsGlob, fz)
		if err != nil {
			fmt.Fprintf(stderr, "エラー: %s（%s）: %v\n", name, r.Path, err)
			return exitUsage
		}
		snaps = append(snaps, s)
	}

	rootDir := filepath.Dir(filepath.Clean(*dataDir))
	local, err := listLocalLogs(rootDir, sources.Logs.Path)
	if err != nil {
		fmt.Fprintf(stderr, "エラー: %s を読めません: %v\n", sources.Logs.Path, err)
		return exitUsage
	}

	judged, warnings := judgedLogs(files)
	for _, w := range warnings {
		fmt.Fprintf(stderr, "警告: %s\n", w)
	}
	pending, skipped := findPending(snaps, local, judged)
	if *countOnly {
		fmt.Fprintln(stdout, len(pending))
		return exitOK
	}
	// evidenceRefs に書くコミットは、そのログを最後に変更したコミット（prompts/judge.md「出力の形」）。リポジトリの HEAD ではない
	for i, p := range pending {
		if p.Repo == "" {
			continue
		}
		out, err := gitOutput(sources.Repos[p.Repo].Path, "log", "-1", "--format=%h", "--", p.Path)
		if err != nil {
			fmt.Fprintf(stderr, "エラー: %s の %s を最後に変更したコミットを調べられません: %v\n", p.Repo, p.Path, err)
			return exitUsage
		}
		pending[i].Commit = strings.TrimSpace(string(out))
	}
	printPending(stdout, pending, skipped)
	return exitOK
}

// readSources は sources.local.json を読む。読めなければ理由を出して終了コード 2 を返す。
func readSources(sourcesPath string, stderr io.Writer) (sourcesDoc, int) {
	raw, err := os.ReadFile(sourcesPath)
	if errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(stderr, "エラー: %s がありません。学習ログのリポジトリの手元の場所を書いてください（雛形は sources.local.json.example）\n", sourcesPath)
		return sourcesDoc{}, exitUsage
	}
	if err != nil {
		fmt.Fprintf(stderr, "エラー: %s を読めません: %v\n", sourcesPath, err)
		return sourcesDoc{}, exitUsage
	}
	sources, err := parseSources(raw)
	if err != nil {
		fmt.Fprintf(stderr, "エラー: %v\n", err)
		return sourcesDoc{}, exitUsage
	}
	return sources, exitOK
}

// printPending は未判定のログと、外したログを出す。判定する AI がこのまま使える形にする（prompts/judge.md 手順 1・3）。
func printPending(w io.Writer, pending []pendingLog, skipped []skippedLog) {
	fmt.Fprintf(w, "未判定のログ %d 本（古い順。この順に判定する）\n", len(pending))
	for i, p := range pending {
		fmt.Fprintf(w, "  %d. %s\n", i+1, pendingLogLine(p))
	}
	if len(skipped) > 0 {
		fmt.Fprintf(w, "\n今は判定しないログ %d 本（先にそのリポジトリでコミットする）\n", len(skipped))
		for _, s := range skipped {
			fmt.Fprintf(w, "  - %s %s：%s\n", s.Repo, s.Path, s.Reason)
		}
	}
}

// pendingLogLine は未判定のログ 1 本を 1 行にする。evidenceRefs に書く形（repo:<owner>/<repo>@<commit>/<path>）と、何を判定するかを並べる。
func pendingLogLine(p pendingLog) string {
	var b bytes.Buffer
	if p.Repo == "" {
		fmt.Fprintf(&b, "log:%s（全文）", p.Path)
		return b.String()
	}
	fmt.Fprintf(&b, "repo:%s@%s/%s", p.Repo, p.Commit, p.Path)
	switch {
	case p.DiffBase != "":
		fmt.Fprintf(&b, "（凍結コミット %s からの差分だけ）", p.DiffBase)
	case p.Backfilled:
		b.WriteString("（凍結より前の日付のログ。台帳に写っていない出来事だけ）")
	default:
		b.WriteString("（全文）")
	}
	return b.String()
}
