// Command skillmatrix は判定ファイルを検証し、理解度 data/state.json を再計算する（SPEC.md §6）。
//
//	go -C backend run ./cmd/skillmatrix recalc --data ../data   # 検証して state.json を書き直す（手元で使う）
//	go -C backend run ./cmd/skillmatrix verify --data ../data   # 検証だけ。state.json と再計算結果を比べる（CI で使う）
//	go -C backend run ./cmd/skillmatrix refs <判定ファイル名>     # 根拠の指す学習ログの行を、rationale と並べて出す（手元専用）
//
// DB・HTTP・LLM クライアントを import しない。読むのは internal/domain / internal/roadmap / internal/judgment だけ
// （internal/llm は棚上げ中の Claude API クライアントと同居しているので import しない。KNOWLEDGE.md 2026-09-26）。
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
	"strings"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
	"github.com/n-yoshida-dev/skill-matrix/internal/judgment"
	"github.com/n-yoshida-dev/skill-matrix/internal/roadmap"
)

// 終了コード（SPEC.md §6）。
const (
	exitOK     = 0 // 問題なし
	exitFailed = 1 // データの不備（ロードマップのエラー・判定ファイルの外枠の崩れ・禁止語・verify の棄却や不一致）
	exitUsage  = 2 // 使い方の誤り・ファイルの読み書きの失敗
)

// defaultDataDir は --data を省略したときの data/ の場所。`go -C backend run` で動かす前提（SPEC.md §6）。
const defaultDataDir = "../data"

// data/ の中のファイル名（SPEC.md §3.1）。
const (
	roadmapFile   = "roadmap.json"
	settingsFile  = "settings.json"
	stateFile     = "state.json"
	judgmentsDir  = "judgments"
	usageSynopsis = "使い方: skillmatrix <recalc|verify> [--data <data/ の場所>]\n       skillmatrix refs [--data <data/ の場所>] [--sources <sources.local.json の場所>] <判定ファイル名>..."
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, time.Now()))
}

// run は CLI の本体。テストから呼べるよう、引数・出力先・今日の日時を外から受け取り、終了コードを返す。
func run(args []string, stdout, stderr io.Writer, now time.Time) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usageSynopsis)
		return exitUsage
	}
	mode := args[0]
	// refs はロードマップも state.json も使わない、手元専用の別の入口（refs.go）
	if mode == "refs" {
		return runRefs(args[1:], stdout, stderr)
	}
	if mode != "recalc" && mode != "verify" {
		fmt.Fprintf(stderr, "知らないモードです: %q\n%s\n", mode, usageSynopsis)
		return exitUsage
	}

	flags := flag.NewFlagSet(mode, flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataDir := flags.String("data", defaultDataDir, "data/ ディレクトリの場所（roadmap.json・judgments/・state.json・settings.json を置く）")
	if err := flags.Parse(args[1:]); err != nil {
		return exitUsage
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "余分な引数があります: %v\n%s\n", flags.Args(), usageSynopsis)
		return exitUsage
	}

	in, err := load(*dataDir)
	for _, w := range in.warnings {
		fmt.Fprintf(stderr, "警告: %s\n", w)
	}
	if err != nil {
		return reportError(stderr, err)
	}
	// 指し先の無い取り消しは、取り消したつもりの印が残るので止める。state.json は書かない
	if problems := checkRetractions(in.files); len(problems) > 0 {
		return reportError(stderr, &dataError{problems: problems})
	}

	res := recalculate(in.roadmap, in.files, in.settings.rules)
	encoded, err := encodeState(res.doc)
	if err != nil {
		fmt.Fprintf(stderr, "エラー: %v\n", err)
		return exitUsage
	}
	hits := findForbidden(in.files, in.forbiddenWords)

	statePath := filepath.Join(*dataDir, stateFile)
	if mode == "recalc" {
		return doRecalc(statePath, encoded, in, res, hits, stdout, stderr, now)
	}
	return doVerify(statePath, encoded, res, hits, stdout, stderr)
}

// dataError はデータの不備を表す（終了コード 1）。これ以外のエラーはファイルの読み書きの失敗（終了コード 2）。
type dataError struct {
	// problems は見つかった不備。1 件で止めず全部集める（直すたびに走らせ直さずに済むように）。
	problems []string
}

// Error はエラーの文面を返す。
func (e *dataError) Error() string {
	return strings.Join(e.problems, "\n")
}

// reportError はエラーを出し、種類に応じた終了コードを返す。
func reportError(stderr io.Writer, err error) int {
	var de *dataError
	if errors.As(err, &de) {
		for _, p := range de.problems {
			fmt.Fprintf(stderr, "エラー: %s\n", p)
		}
		return exitFailed
	}
	fmt.Fprintf(stderr, "エラー: %v\n", err)
	return exitUsage
}

// inputs は data/ から読んだもの一式。
type inputs struct {
	roadmap  domain.Roadmap
	settings settings
	files    []judgment.File
	// forbiddenWords は公開される文に入れてはいけない語。settings.json の語に forbidden-words.local.json の語を足したもの。
	forbiddenWords []string
	// warnings はロードマップの警告。止めずに表示だけする。
	warnings []string
}

// load は data/ からロードマップ・設定・判定ファイルを読む。
func load(dataDir string) (inputs, error) {
	in := inputs{settings: defaultSettings()}

	raw, err := os.ReadFile(filepath.Join(dataDir, roadmapFile))
	if err != nil {
		return in, fmt.Errorf("%s を読めません: %w", roadmapFile, err)
	}
	doc, res := roadmap.ParseAndValidate(raw)
	for _, is := range res.Warnings() {
		in.warnings = append(in.warnings, issueLine(is))
	}
	if !res.OK() {
		de := &dataError{}
		for _, is := range res.Errors() {
			de.problems = append(de.problems, issueLine(is))
		}
		return in, de
	}
	in.roadmap = doc.ToDomain()

	// settings.json は省略できる（SPEC.md §8.3）
	raw, err = os.ReadFile(filepath.Join(dataDir, settingsFile))
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return in, fmt.Errorf("%s を読めません: %w", settingsFile, err)
	default:
		s, err := parseSettings(raw)
		if err != nil {
			return in, &dataError{problems: []string{err.Error()}}
		}
		in.settings = s
	}
	in.forbiddenWords = append([]string(nil), in.settings.forbiddenWords...)

	// 本人しか知らない禁止語は省略できる（コミットしないので、CI のチェックアウトには無い。SPEC.md §9）
	raw, err = os.ReadFile(filepath.Join(dataDir, localForbiddenFile))
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return in, fmt.Errorf("%s を読めません: %w", localForbiddenFile, err)
	default:
		words, err := parseLocalForbidden(raw)
		if err != nil {
			return in, &dataError{problems: []string{err.Error()}}
		}
		in.forbiddenWords = append(in.forbiddenWords, words...)
	}

	in.files, err = loadJudgments(filepath.Join(dataDir, judgmentsDir))
	return in, err
}

// issueLine はロードマップの検査結果1件を1行にする。
func issueLine(is roadmap.Issue) string {
	if is.Path == "" {
		return fmt.Sprintf("%s: %s（%s）", roadmapFile, is.Message, is.Code)
	}
	return fmt.Sprintf("%s: %s: %s（%s）", roadmapFile, is.Path, is.Message, is.Code)
}

// loadJudgments は judgments/ の判定ファイルを全部読む。外枠の崩れたファイルがあれば、全部集めてから dataError を返す。
//
// ディレクトリが無いのは「判定 0 件」として扱う。git は空のディレクトリを記録しないので、
// 判定を1件もコミットしていないうちは CI のチェックアウトに judgments/ が無い。
// `.` で始まるファイル（.gitkeep 等）は読まない。
func loadJudgments(dir string) ([]judgment.File, error) {
	entries, err := os.ReadDir(dir) // ファイル名の昇順で返る。処理順は recalculate が改めて並べる
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s/ を読めません: %w", judgmentsDir, err)
	}

	var files []judgment.File
	de := &dataError{}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			de.problems = append(de.problems, fmt.Sprintf("%s/%s: judgments/ の中にディレクトリは置けません", judgmentsDir, name))
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("%s/%s を読めません: %w", judgmentsDir, name, err)
		}
		f, err := judgment.ParseFile(name, raw)
		if err != nil {
			de.problems = append(de.problems, fmt.Sprintf("%s/%v", judgmentsDir, err))
			continue
		}
		files = append(files, f)
	}
	if len(de.problems) > 0 {
		return nil, de
	}
	return files, nil
}

// doRecalc は state.json を書き直し、要約を出す。
// 棄却があっても書き出す（何が弾かれたかを state.json で見られるように）が、verify は失敗することを知らせる。
// 禁止語があっても書き出すが、終了コード 1 を返す（公開される文なので、コミット前に必ず直させる。SPEC.md §6）。
func doRecalc(statePath string, encoded []byte, in inputs, res recalcResult, hits []forbiddenHit, stdout, stderr io.Writer, now time.Time) int {
	// 変わった項目を出すために、書き直す前の state.json を読む。無い・読めないときは比べない
	var prev *stateDoc
	raw, err := os.ReadFile(statePath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		fmt.Fprintf(stderr, "エラー: %s を読めません: %v\n", stateFile, err)
		return exitUsage
	default:
		if doc, err := decodeState(raw); err == nil {
			prev = &doc
		} else {
			fmt.Fprintf(stderr, "警告: 前回の %v\n", err)
		}
	}

	if err := os.WriteFile(statePath, encoded, 0o644); err != nil {
		fmt.Fprintf(stderr, "エラー: %s を書けません: %v\n", stateFile, err)
		return exitUsage
	}

	fmt.Fprintf(stdout, "%s を書き直しました\n\n", statePath)
	printSummary(stdout, in.roadmap, res, prev, len(in.files), in.settings, now)
	fmt.Fprintln(stdout)
	printViolations(stdout, res.doc)
	if len(res.doc.Rejected) > 0 {
		fmt.Fprintf(stdout, "\n棄却された判定が %d 件あります。このままコミットすると CI の verify が失敗します。判定ファイルを直してください\n", len(res.doc.Rejected))
	}
	if len(hits) > 0 {
		printForbidden(stderr, hits)
		fmt.Fprintln(stderr, "失敗: 公開される文に禁止語があります。判定ファイルの文を言い換えてから、もう一度 recalc してください")
		return exitFailed
	}
	return exitOK
}

// printForbidden は禁止語が見つかった場所を1行ずつ出す。
func printForbidden(w io.Writer, hits []forbiddenHit) {
	fmt.Fprintf(w, "禁止語が %d 件あります\n", len(hits))
	for _, h := range hits {
		fmt.Fprintf(w, "  %s\n", h)
	}
}

// doVerify は state.json を書かずに、棄却と禁止語が無いことと、コミットされた state.json が再計算結果とバイト単位で一致することを確かめる。
func doVerify(statePath string, encoded []byte, res recalcResult, hits []forbiddenHit, stdout, stderr io.Writer) int {
	printViolations(stdout, res.doc)

	failed := false
	if len(res.doc.Rejected) > 0 {
		fmt.Fprintf(stderr, "失敗: 棄却された判定が %d 件あります。判定ファイルを直して recalc してください\n", len(res.doc.Rejected))
		failed = true
	}
	if len(hits) > 0 {
		printForbidden(stderr, hits)
		fmt.Fprintln(stderr, "失敗: 公開される文に禁止語があります。判定ファイルの文を言い換えてください")
		failed = true
	}

	committed, err := os.ReadFile(statePath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		fmt.Fprintf(stderr, "失敗: %s がありません。recalc を実行して、出力をコミットしてください\n", stateFile)
		return exitFailed
	case err != nil:
		fmt.Fprintf(stderr, "エラー: %s を読めません: %v\n", stateFile, err)
		return exitUsage
	}
	if !bytes.Equal(committed, encoded) {
		fmt.Fprintf(stderr, "失敗: %s が再計算結果と一致しません（最初の違いは %d 行目）。recalc を実行して、出力をコミットしてください\n",
			stateFile, firstDiffLine(committed, encoded))
		failed = true
	}

	if failed {
		return exitFailed
	}
	fmt.Fprintf(stdout, "\nOK: 棄却と禁止語は 0 件で、%s は再計算結果と一致しています\n", stateFile)
	return exitOK
}

// firstDiffLine は2つのバイト列が最初に食い違う行番号（1 始まり）を返す。どこを見ればよいかを示すため。
func firstDiffLine(a, b []byte) int {
	line := 1
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return line
		}
		if a[i] == '\n' {
			line++
		}
	}
	return line
}
