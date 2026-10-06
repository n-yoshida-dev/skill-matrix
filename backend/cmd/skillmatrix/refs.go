package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/n-yoshida-dev/skill-matrix/internal/judgment"
)

// このファイルは `refs` モード（SPEC.md §6）を持つ。判定ファイルの evidenceRefs を手元のリポジトリで解決し、
// 指している行を判定の rationale と並べて表示する。突き合わせ役の AI（prompts/check-judgment.md）が
// 「rationale の出来事は、本当にその行に書いてあるか」を確かめる材料にする。
//
// 学習ログの本文はリポジトリに置かない（CLAUDE.md）。refs は端末に出すだけで、ファイルには書かない。
// 手元専用で、CI では使わない（CI のチェックアウトには sources.local.json も学習ログのリポジトリも無い）。

// defaultSourcesFile は --sources を省略したときの sources.local.json の場所。`go -C backend run` で動かす前提。
const defaultSourcesFile = "../sources.local.json"

// refsSynopsis は refs モードの使い方。
const refsSynopsis = "使い方: skillmatrix refs [--data <data/ の場所>] [--sources <sources.local.json の場所>] <判定ファイル名>..."

// evidenceRef は evidenceRefs の1件を読み取ったもの（SPEC.md §3.3）。
type evidenceRef struct {
	// Raw は書かれたままの文字列。
	Raw string
	// Kind は種別（repo / log / それ以外）。
	Kind string
	// Repo は repo: のときの `<owner>/<repo>`。
	Repo string
	// Commit は repo: のときのコミット。
	Commit string
	// Path はファイルの場所。repo: ならリポジトリ内、log: ならこのリポジトリのルートから。空ならコミットだけを指す。
	Path string
	// Start と End は行の範囲（1 始まり、両端を含む）。Start が 0 なら行の指定なし（ファイル全体）。
	Start, End int
}

// repoRefPattern は repo: の文法（`repo:<owner>/<repo>@<commit>[/<path>[#L<n>[-L<m>]]]`）。
var repoRefPattern = regexp.MustCompile(`^repo:([^/@\s]+/[^/@\s]+)@([0-9a-f]{4,40})(?:/([^#\s]+?)(?:#L(\d+)(?:-L(\d+))?)?)?$`)

// logRefPattern は log: の文法（`log:<path>[#L<n>[-L<m>]]`）。
var logRefPattern = regexp.MustCompile(`^log:([^#\s]+?)(?:#L(\d+)(?:-L(\d+))?)?$`)

// parseEvidenceRef は evidenceRefs の1件を読む。repo: と log: 以外の種別は Kind だけを埋めて返す（refs では引けない）。
// repo: / log: なのに文法が崩れていればエラー（形の検査は種別の書式しか見ないので、ここで初めて分かる）。
func parseEvidenceRef(s string) (evidenceRef, error) {
	kind, _, ok := strings.Cut(s, ":")
	if !ok {
		return evidenceRef{}, fmt.Errorf("%q は <種別>:<識別子> の形ではありません", s)
	}
	r := evidenceRef{Raw: s, Kind: kind}
	var start, end string
	switch kind {
	case "repo":
		m := repoRefPattern.FindStringSubmatch(s)
		if m == nil {
			return evidenceRef{}, fmt.Errorf("%q は repo:<owner>/<repo>@<commit>[/<path>[#L<n>[-L<m>]]] の形ではありません", s)
		}
		r.Repo, r.Commit, r.Path, start, end = m[1], m[2], m[3], m[4], m[5]
	case "log":
		m := logRefPattern.FindStringSubmatch(s)
		if m == nil {
			return evidenceRef{}, fmt.Errorf("%q は log:<path>[#L<n>[-L<m>]] の形ではありません", s)
		}
		r.Path, start, end = m[1], m[2], m[3]
	default:
		return r, nil
	}

	if start == "" {
		return r, nil
	}
	// 正規表現で数字だけに絞っているので、変換に失敗するのは桁あふれのときだけ
	var err error
	if r.Start, err = strconv.Atoi(start); err != nil {
		return evidenceRef{}, fmt.Errorf("%q の行番号を読めません: %w", s, err)
	}
	r.End = r.Start
	if end != "" {
		if r.End, err = strconv.Atoi(end); err != nil {
			return evidenceRef{}, fmt.Errorf("%q の行番号を読めません: %w", s, err)
		}
	}
	if r.Start < 1 || r.End < r.Start {
		return evidenceRef{}, fmt.Errorf("%q の行の範囲が正しくありません（1 以上で、終わりが始まり以上）", s)
	}
	return r, nil
}

// numberedLine は行番号付きの1行。
type numberedLine struct {
	No   int
	Text string
}

// pickLines はファイルの中身から start〜end 行目を取り出す（1 始まり、両端を含む）。start が 0 ならファイル全体。
// 範囲がファイルの行数を超えていればエラー（指している行が無い＝根拠が崩れている）。
func pickLines(content []byte, start, end int) ([]numberedLine, error) {
	text := strings.TrimSuffix(string(content), "\n")
	var lines []string
	if text != "" {
		lines = strings.Split(text, "\n")
	}
	if start == 0 {
		start, end = 1, len(lines)
	}
	if end > len(lines) {
		return nil, fmt.Errorf("%d 行しかないファイルの %d〜%d 行目を指しています", len(lines), start, end)
	}
	out := make([]numberedLine, 0, end-start+1)
	for i := start; i <= end; i++ {
		out = append(out, numberedLine{No: i, Text: strings.TrimSuffix(lines[i-1], "\r")})
	}
	return out, nil
}

// sourcesDoc は sources.local.json（SPEC.md §3.5）。refs が使うのは repos の path と logs の path だけ。
type sourcesDoc struct {
	Logs struct {
		Path string `json:"path"`
	} `json:"logs"`
	Repos map[string]struct {
		Path     string `json:"path"`
		LogsGlob string `json:"logsGlob"`
	} `json:"repos"`
}

// parseSources は sources.local.json を読む。未知のキーはエラー（打ち間違いで場所が黙って空になるのを防ぐ）。
func parseSources(raw []byte) (sourcesDoc, error) {
	var doc sourcesDoc
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return sourcesDoc{}, fmt.Errorf("sources.local.json を読めません: %w", err)
	}
	return doc, nil
}

// refReader は evidenceRef の指すファイルを読む。テストでは偽物に差し替える。
type refReader interface {
	// show は repoPath のリポジトリの commit 時点の path の中身を返す。
	show(repoPath, commit, path string) ([]byte, error)
	// hasCommit は repoPath のリポジトリに commit があるかを返す。
	hasCommit(repoPath, commit string) error
}

// gitReader は git コマンドで読む refReader。
type gitReader struct{}

// show は `git -C <repoPath> show <commit>:<path>` の出力を返す。
func (gitReader) show(repoPath, commit, path string) ([]byte, error) {
	return gitOutput(repoPath, "show", commit+":"+path)
}

// hasCommit は `git -C <repoPath> cat-file -e <commit>^{commit}` でコミットの有無を確かめる。
func (gitReader) hasCommit(repoPath, commit string) error {
	_, err := gitOutput(repoPath, "cat-file", "-e", commit+"^{commit}")
	return err
}

// gitOutput は git を走らせて標準出力を返す。失敗したら git のエラー出力を添える。
func gitOutput(repoPath string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(firstNonEmpty(stderr.String(), err.Error())))
	}
	return out, nil
}

// firstNonEmpty は空でない最初の文字列を返す。
func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// refResolver は evidenceRef を手元の場所に解決して、指している行を返す。
type refResolver struct {
	sources sourcesDoc
	// rootDir はこのリポジトリのルート。log: の path と、相対で書かれた logs.path の基準。
	rootDir string
	reader  refReader
}

// resolved は解決した結果1件。
type resolved struct {
	// Lines は指している行。コミットだけを指す ref では空。
	Lines []numberedLine
	// Note は行を出さないときの説明（コミットだけを指す、など）。
	Note string
}

// resolve は ref の指す行を返す。解決できなければエラー（ファイルが無い・行が無い・場所が分からない）。
//
// コミットだけを指す repo:（実装先のリポジトリの記録など）は行が無いので、手元にあればコミットの有無だけを確かめる。
// 手元の場所が分からないときはエラーにせず、確かめていないことを Note に書く（移行の判定は実装先のリポジトリを指すが、
// それらは sources.local.json に載っていないことが多い）。
func (rr refResolver) resolve(ref evidenceRef) (resolved, error) {
	switch ref.Kind {
	case "repo":
		repo, ok := rr.sources.Repos[ref.Repo]
		if ref.Path == "" {
			if !ok {
				return resolved{Note: "コミットだけを指す。手元の場所が sources.local.json に無いので、コミットの有無は確かめていない"}, nil
			}
			if err := rr.reader.hasCommit(repo.Path, ref.Commit); err != nil {
				return resolved{}, fmt.Errorf("コミット %s が手元の %s にありません: %w", ref.Commit, ref.Repo, err)
			}
			return resolved{Note: "コミットだけを指す（行は無い）。手元にコミットがあることは確かめた"}, nil
		}
		if !ok {
			return resolved{}, fmt.Errorf("%s の手元の場所が sources.local.json の repos にありません", ref.Repo)
		}
		content, err := rr.reader.show(repo.Path, ref.Commit, ref.Path)
		if err != nil {
			return resolved{}, err
		}
		lines, err := pickLines(content, ref.Start, ref.End)
		return resolved{Lines: lines}, err
	case "log":
		content, err := os.ReadFile(filepath.Join(rr.rootDir, filepath.FromSlash(ref.Path)))
		if err != nil {
			return resolved{}, fmt.Errorf("%s を読めません: %w", ref.Path, err)
		}
		lines, err := pickLines(content, ref.Start, ref.End)
		return resolved{Lines: lines}, err
	default:
		return resolved{Note: fmt.Sprintf("種別 %q は refs では引けない", ref.Kind)}, nil
	}
}

// printRefs は判定ファイル1本について、判定ごとに rationale と evidenceRefs の指す行を出す。
// 解決できなかった ref の数を返す（呼び出し側が終了コードを決める）。
func printRefs(w io.Writer, f judgment.File, rr refResolver) int {
	failures := 0
	fmt.Fprintf(w, "== %s\n", f.Name)
	for _, p := range f.Output.Judgments {
		fmt.Fprintf(w, "\njudgments[%d] %s %s %d\n", p.Index, p.ItemKey, p.EvidenceType, p.ProposedLevel)
		fmt.Fprintf(w, "  rationale: %s\n", p.Rationale)
		for _, raw := range p.EvidenceRefs {
			fmt.Fprintf(w, "  %s\n", raw)
			ref, err := parseEvidenceRef(raw)
			if err == nil {
				var res resolved
				res, err = rr.resolve(ref)
				if err == nil {
					if res.Note != "" {
						fmt.Fprintf(w, "    （%s）\n", res.Note)
					}
					for _, l := range res.Lines {
						fmt.Fprintf(w, "    %5d| %s\n", l.No, l.Text)
					}
					continue
				}
			}
			failures++
			fmt.Fprintf(w, "    エラー: %v\n", err)
		}
	}
	// 形の検査で弾いた判定は rationale も ref も信用できないので、弾かれたことだけを出す
	for _, r := range f.Output.Rejected {
		fmt.Fprintf(w, "\njudgments[%d] は形の検査で弾かれています（%s）。recalc で直してから refs を使ってください\n", r.Index, r.Reason)
	}
	return failures
}

// runRefs は refs モードの本体。終了コードを返す。
func runRefs(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("refs", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataDir := flags.String("data", defaultDataDir, "data/ ディレクトリの場所（judgments/ を読む）")
	sourcesPath := flags.String("sources", defaultSourcesFile, "sources.local.json の場所（学習ログのリポジトリの手元の場所）")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() == 0 {
		fmt.Fprintf(stderr, "判定ファイル名を 1 つ以上指定してください\n%s\n", refsSynopsis)
		return exitUsage
	}

	sources, code := readSources(*sourcesPath, stderr)
	if code != exitOK {
		return code
	}
	rr := refResolver{sources: sources, rootDir: filepath.Dir(filepath.Clean(*dataDir)), reader: gitReader{}}

	failures := 0
	for i, name := range flags.Args() {
		// 名前だけでも、data/judgments/ からのパスでもよい
		base := filepath.Base(name)
		raw, err := os.ReadFile(filepath.Join(*dataDir, judgmentsDir, base))
		if err != nil {
			fmt.Fprintf(stderr, "エラー: %s/%s を読めません: %v\n", judgmentsDir, base, err)
			return exitUsage
		}
		f, err := judgment.ParseFile(base, raw)
		if err != nil {
			fmt.Fprintf(stderr, "エラー: %s/%v\n", judgmentsDir, err)
			return exitFailed
		}
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		failures += printRefs(stdout, f, rr)
	}

	if failures > 0 {
		fmt.Fprintf(stderr, "失敗: 引けなかった根拠が %d 件あります（上の「エラー:」の行）。evidenceRefs のファイル・行番号を直してください\n", failures)
		return exitFailed
	}
	return exitOK
}
