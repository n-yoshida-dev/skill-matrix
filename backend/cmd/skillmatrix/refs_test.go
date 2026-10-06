package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/n-yoshida-dev/skill-matrix/internal/judgment"
)

// このテストは refs（evidenceRefs の指す行を引くコマンド）の約束を固定する。
// 指している行が無い根拠は、黙って飛ばさずエラーにする（突き合わせ役が「書いてあるか」を確かめられなくなるため）。

func TestParseEvidenceRef(t *testing.T) {
	tests := []struct {
		in   string
		want evidenceRef
	}{
		{"repo:o/r@a1b2c3d/go/logs/2026-09-28.md#L10-L30", evidenceRef{Kind: "repo", Repo: "o/r", Commit: "a1b2c3d", Path: "go/logs/2026-09-28.md", Start: 10, End: 30}},
		{"repo:o/r@a1b2c3d/go/logs/2026-09-28.md#L7", evidenceRef{Kind: "repo", Repo: "o/r", Commit: "a1b2c3d", Path: "go/logs/2026-09-28.md", Start: 7, End: 7}},
		{"repo:o/r@a1b2c3d/progress.md", evidenceRef{Kind: "repo", Repo: "o/r", Commit: "a1b2c3d", Path: "progress.md"}},
		{"repo:o/r@6647ba8", evidenceRef{Kind: "repo", Repo: "o/r", Commit: "6647ba8"}},
		{"log:learning-logs/2026-10-01.md", evidenceRef{Kind: "log", Path: "learning-logs/2026-10-01.md"}},
		{"log:learning-logs/2026-10-01.md#L3-L4", evidenceRef{Kind: "log", Path: "learning-logs/2026-10-01.md", Start: 3, End: 4}},
		{"article:https://example.com/x", evidenceRef{Kind: "article"}},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parseEvidenceRef(tt.in)
			if err != nil {
				t.Fatalf("エラーになった: %v", err)
			}
			tt.want.Raw = tt.in
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}

	for _, bad := range []string{
		"repo:o/r",                    // commit が無い
		"repo:o/r@XYZ/a.md",           // commit が 16 進でない
		"repo:o/r@a1b2c3d/a.md#L0",    // 行は 1 から
		"repo:o/r@a1b2c3d/a.md#L5-L3", // 終わりが始まりより前
		"repo:o/r@a1b2c3d/a.md#L3-",   // 範囲の書きかけ
		"log:a.md#L",                  // 行番号が無い
		"no-kind",                     // 種別が無い
	} {
		t.Run("崩れ "+bad, func(t *testing.T) {
			if _, err := parseEvidenceRef(bad); err == nil {
				t.Errorf("エラーにならなかった")
			}
		})
	}
}

func TestPickLines(t *testing.T) {
	content := []byte("1 行目\n2 行目\r\n3 行目\n")

	t.Run("範囲の行を行番号付きで返す", func(t *testing.T) {
		got, err := pickLines(content, 2, 3)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0] != (numberedLine{2, "2 行目"}) || got[1] != (numberedLine{3, "3 行目"}) {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("行の指定が無ければファイル全体", func(t *testing.T) {
		if got, err := pickLines(content, 0, 0); err != nil || len(got) != 3 {
			t.Errorf("got %+v, err %v", got, err)
		}
	})

	t.Run("ファイルの行数を超える範囲はエラー", func(t *testing.T) {
		_, err := pickLines(content, 3, 4)
		if err == nil || !strings.Contains(err.Error(), "3 行しかない") {
			t.Errorf("err = %v", err)
		}
	})
}

// fakeReader はファイルの中身を map から返す refReader。キーは "<repoPath>@<commit>:<path>"。
type fakeReader struct {
	files   map[string]string
	commits map[string]bool
}

func (f fakeReader) show(repoPath, commit, path string) ([]byte, error) {
	c, ok := f.files[repoPath+"@"+commit+":"+path]
	if !ok {
		return nil, errors.New("そのコミットにファイルが無い")
	}
	return []byte(c), nil
}

func (f fakeReader) hasCommit(repoPath, commit string) error {
	if !f.commits[repoPath+"@"+commit] {
		return errors.New("コミットが無い")
	}
	return nil
}

func TestPrintRefs(t *testing.T) {
	rr := refResolver{
		sources: sourcesDoc{Repos: map[string]struct {
			Path     string `json:"path"`
			LogsGlob string `json:"logsGlob"`
		}{"o/study": {Path: "/study"}}},
		reader: fakeReader{
			files:   map[string]string{"/study@abc1234:logs/a.md": "見出し\nドリルに答えた\n説明した\n"},
			commits: map[string]bool{"/study@abc1234": true},
		},
	}
	file := func(refs ...string) judgment.File {
		return mustFile(t, "2026-10-01-ai.json", `{"schemaVersion": 2, "loggedAt": "2026-10-01", "source": "ai", "judgments": [
			{"itemKey": "go-01", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["`+strings.Join(refs, `", "`)+`"], "rationale": "ドリルに答えた", "confidence": 0.9}]}`)
	}

	t.Run("rationale と指している行を並べる", func(t *testing.T) {
		var out strings.Builder
		if n := printRefs(&out, file("repo:o/study@abc1234/logs/a.md#L2"), rr); n != 0 {
			t.Fatalf("失敗 %d 件: %s", n, out.String())
		}
		for _, want := range []string{"judgments[0] go-01 drill 1", "rationale: ドリルに答えた", "    2| ドリルに答えた"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("%q が無い:\n%s", want, out.String())
			}
		}
		if strings.Contains(out.String(), "説明した") {
			t.Errorf("指していない行まで出ている:\n%s", out.String())
		}
	})

	t.Run("引けない根拠はエラーとして数える", func(t *testing.T) {
		cases := map[string]string{
			"repo:o/study@abc1234/logs/a.md#L3-L9": "3 行しかない",                           // 行が無い
			"repo:o/study@abc1234/logs/b.md#L1":    "ファイルが無い",                           // ファイルが無い
			"repo:o/other@abc1234/logs/a.md#L1":    "sources.local.json の repos にありません", // 手元の場所が分からない
			"repo:o/study@fff9999":                 "コミット fff9999 が手元の o/study にありません",
			"repo:o/study@abc1234/a.md#L0":         "行の範囲が正しくありません",
		}
		for ref, want := range cases {
			var out strings.Builder
			if n := printRefs(&out, file(ref), rr); n != 1 || !strings.Contains(out.String(), want) {
				t.Errorf("%s: 失敗 %d 件, 出力に %q が無い:\n%s", ref, n, want, out.String())
			}
		}
	})

	t.Run("コミットだけを指す根拠は行を出さずに通す", func(t *testing.T) {
		for _, ref := range []string{"repo:o/study@abc1234", "repo:o/impl@abc1234"} {
			var out strings.Builder
			if n := printRefs(&out, file(ref), rr); n != 0 || !strings.Contains(out.String(), "コミットだけを指す") {
				t.Errorf("%s: 失敗 %d 件:\n%s", ref, n, out.String())
			}
		}
	})
}

func TestRun_refs(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git が無い")
	}
	// 学習ログのリポジトリの代わりに、一時ディレクトリに git のリポジトリを作る
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := gitOutput(repo, args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	writeFile(t, filepath.Join(repo, "log.md"), "見出し\nドリルに答えた\n")
	git("add", "log.md")
	git("-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "log")
	commit := git("rev-parse", "--short", "HEAD")

	judgmentWith := func(lines string) string {
		return `{"schemaVersion": 2, "loggedAt": "2026-10-01", "source": "ai", "judgments": [
			{"itemKey": "go-01", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["repo:o/study@` + commit + `/log.md#` + lines + `"], "rationale": "ドリルに答えた", "confidence": 0.9}]}`
	}
	setup := func(lines string) (string, string) {
		dir := newDataDir(t, map[string]string{"2026-10-01-ai.json": judgmentWith(lines)})
		sources := filepath.Join(dir, "sources.local.json")
		writeFile(t, sources, `{"repos": {"o/study": {"path": "`+filepath.ToSlash(repo)+`", "logsGlob": "*.md"}}}`)
		return dir, sources
	}

	t.Run("指している行を出して終了コード 0", func(t *testing.T) {
		dir, sources := setup("L2")
		code, out, errOut := runCLI("refs", "--data", dir, "--sources", sources, "2026-10-01-ai.json")
		if code != exitOK || !strings.Contains(out, "    2| ドリルに答えた") {
			t.Errorf("終了コード = %d, stdout = %s, stderr = %s", code, out, errOut)
		}
	})

	t.Run("無い行を指していれば終了コード 1", func(t *testing.T) {
		dir, sources := setup("L2-L5")
		code, out, errOut := runCLI("refs", "--data", dir, "--sources", sources, "2026-10-01-ai.json")
		if code != exitFailed || !strings.Contains(out, "2 行しかない") || !strings.Contains(errOut, "引けなかった根拠が 1 件") {
			t.Errorf("終了コード = %d, stdout = %s, stderr = %s", code, out, errOut)
		}
	})

	t.Run("sources.local.json が無ければ終了コード 2", func(t *testing.T) {
		dir, _ := setup("L2")
		code, _, errOut := runCLI("refs", "--data", dir, "--sources", filepath.Join(dir, "none.json"), "2026-10-01-ai.json")
		if code != exitUsage || !strings.Contains(errOut, "sources.local.json.example") {
			t.Errorf("終了コード = %d, stderr = %s", code, errOut)
		}
	})

	t.Run("判定ファイルの指定が無い・見つからなければ終了コード 2", func(t *testing.T) {
		dir, sources := setup("L2")
		for _, args := range [][]string{{"refs", "--data", dir, "--sources", sources}, {"refs", "--data", dir, "--sources", sources, "2026-10-09-none.json"}} {
			if code, _, errOut := runCLI(args...); code != exitUsage {
				t.Errorf("%v: 終了コード = %d, stderr = %s", args, code, errOut)
			}
		}
	})
}

func TestRun_refsはsourcesの未知のキーで止まる(t *testing.T) {
	dir := newDataDir(t, map[string]string{"2026-08-01-tour.json": okJudgment})
	sources := filepath.Join(dir, "sources.local.json")
	writeFile(t, sources, `{"repo": {}}`)
	if code, _, errOut := runCLI("refs", "--data", dir, "--sources", sources, "2026-08-01-tour.json"); code != exitUsage {
		t.Errorf("終了コード = %d, stderr = %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, stateFile)); err == nil {
		t.Error("refs が state.json を書いている")
	}
}
