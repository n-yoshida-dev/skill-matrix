package main

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// このテストは CLI を外から見た約束（SPEC.md §6 のモードと終了コード）を固定する。
// data/ の形をした一時ディレクトリを作り、run を呼んで終了コード・出力・state.json を確かめる。

// update を付けて走らせると、testdata の期待値（state-sample.json）を書き直す。
// 書き直したら git diff で中身を読み、意図した変化かを確かめてからコミットする。
//
//	go -C backend test ./cmd/skillmatrix -run TestRecalc_testdata -update
var update = flag.Bool("update", false, "testdata/state-sample.json を書き直す")

// today は要約の「次にやること」に渡す日付。state.json には影響しない。
var today = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

// testdataDir は backend/testdata の場所（テストはパッケージのディレクトリで走る）。
const testdataDir = "../../testdata"

// newDataDir は data/ の形をした一時ディレクトリを作る。roadmap.json は testdata のサンプル。
// judgments の各要素は「ファイル名 → 中身」。
func newDataDir(t *testing.T, judgments map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	copyFile(t, filepath.Join(testdataDir, "roadmap-sample.json"), filepath.Join(dir, roadmapFile))
	if judgments != nil {
		jdir := filepath.Join(dir, judgmentsDir)
		if err := os.Mkdir(jdir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, raw := range judgments {
			writeFile(t, filepath.Join(jdir, name), raw)
		}
	}
	return dir
}

// copyFile はファイルを写す。
func copyFile(t *testing.T, from, to string) {
	t.Helper()
	raw, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, to, string(raw))
}

// writeFile はファイルを書く。
func writeFile(t *testing.T, path, raw string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runCLI は CLI を走らせ、終了コードと標準出力・標準エラーを返す。
func runCLI(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr, today)
	return code, stdout.String(), stderr.String()
}

// okJudgment は適用される判定1件だけの判定ファイル。
const okJudgment = `{"schemaVersion": 2, "loggedAt": "2026-08-01", "source": "ai", "judge": "claude-code", "judgments": [
	{"itemKey": "go-01", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["log:a.md"], "rationale": "確認問題に答えた", "confidence": 0.9}
], "unmatched": []}`

func TestRun_recalcしてからverifyすると通る(t *testing.T) {
	dir := newDataDir(t, map[string]string{"2026-08-01-tour.json": okJudgment})

	code, out, errOut := runCLI("recalc", "--data", dir)
	if code != exitOK {
		t.Fatalf("recalc の終了コード = %d, stderr = %s", code, errOut)
	}
	if !strings.Contains(out, "変わった項目") || !strings.Contains(out, "次にやること") {
		t.Errorf("要約が出ていない: %s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, stateFile)); err != nil {
		t.Fatalf("state.json が書かれていない: %v", err)
	}

	code, out, errOut = runCLI("verify", "--data", dir)
	if code != exitOK {
		t.Fatalf("verify の終了コード = %d, stderr = %s", code, errOut)
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("通ったことが出ていない: %s", out)
	}
}

func TestRun_verifyが失敗する場合(t *testing.T) {
	t.Run("state.json が無い", func(t *testing.T) {
		dir := newDataDir(t, map[string]string{"2026-08-01-tour.json": okJudgment})
		code, _, errOut := runCLI("verify", "--data", dir)
		if code != exitFailed || !strings.Contains(errOut, "state.json がありません") {
			t.Errorf("終了コード = %d, stderr = %s", code, errOut)
		}
	})

	t.Run("state.json を手で書き換えた", func(t *testing.T) {
		dir := newDataDir(t, map[string]string{"2026-08-01-tour.json": okJudgment})
		runCLI("recalc", "--data", dir)
		path := filepath.Join(dir, stateFile)
		raw, _ := os.ReadFile(path)
		writeFile(t, path, strings.Replace(string(raw), `"verifiedLevel": 1`, `"verifiedLevel": 3`, 1))

		code, _, errOut := runCLI("verify", "--data", dir)
		if code != exitFailed || !strings.Contains(errOut, "一致しません") {
			t.Errorf("終了コード = %d, stderr = %s", code, errOut)
		}
	})

	t.Run("判定ファイルを足したのに recalc していない", func(t *testing.T) {
		dir := newDataDir(t, map[string]string{"2026-08-01-tour.json": okJudgment})
		runCLI("recalc", "--data", dir)
		writeFile(t, filepath.Join(dir, judgmentsDir, "2026-08-02-more.json"),
			strings.ReplaceAll(okJudgment, "2026-08-01", "2026-08-02"))

		code, _, errOut := runCLI("verify", "--data", dir)
		if code != exitFailed || !strings.Contains(errOut, "一致しません") {
			t.Errorf("終了コード = %d, stderr = %s", code, errOut)
		}
	})

	t.Run("棄却された判定が残っている（state.json は一致していても失敗）", func(t *testing.T) {
		unknown := strings.Replace(okJudgment, `"go-01"`, `"go-99"`, 1)
		dir := newDataDir(t, map[string]string{"2026-08-01-tour.json": unknown})
		code, out, _ := runCLI("recalc", "--data", dir)
		if code != exitOK || !strings.Contains(out, "verify が失敗します") {
			t.Fatalf("recalc は書き出して知らせるだけ: 終了コード = %d, stdout = %s", code, out)
		}

		code, out, errOut := runCLI("verify", "--data", dir)
		if code != exitFailed || !strings.Contains(errOut, "棄却された判定が 1 件") {
			t.Errorf("終了コード = %d, stderr = %s", code, errOut)
		}
		// どのファイルの何件目の、どの項目の、どのルールか
		if !strings.Contains(out, "2026-08-01-tour.json judgments[0] go-99 V1_unknown_item") {
			t.Errorf("違反の一覧が出ていない: %s", out)
		}
	})
}

func TestRun_判定ファイルの外枠が崩れていたら止まる(t *testing.T) {
	broken := map[string]string{
		"2026-08-01-tour.json":   okJudgment,
		"2026-08-02-broken.json": `{"schemaVersion": 2, "loggedAt": "2026-08-02", "source": "ai", "judgment": []}`,
		"2026-08-03-wrong.json":  strings.ReplaceAll(okJudgment, "2026-08-01", "2026-08-04"),
	}
	for _, mode := range []string{"recalc", "verify"} {
		t.Run(mode, func(t *testing.T) {
			dir := newDataDir(t, broken)
			code, _, errOut := runCLI(mode, "--data", dir)
			if code != exitFailed {
				t.Errorf("終了コード = %d, want %d", code, exitFailed)
			}
			// 1 件目で止めず、崩れたファイルを全部挙げる
			for _, want := range []string{"judgments/2026-08-02-broken.json", "judgments/2026-08-03-wrong.json"} {
				if !strings.Contains(errOut, want) {
					t.Errorf("%s が挙がっていない: %s", want, errOut)
				}
			}
			if _, err := os.Stat(filepath.Join(dir, stateFile)); err == nil {
				t.Error("止まったのに state.json が書かれている")
			}
		})
	}
}

func TestRun_取り消し(t *testing.T) {
	fix := func(file string, index int) string {
		return `{"schemaVersion": 2, "loggedAt": "2026-08-05", "source": "manual", "judgments": [], "retractions": [
			{"file": "` + file + `", "index": ` + strconv.Itoa(index) + `, "reason": "別の日の出来事だった"}], "unmatched": []}`
	}

	t.Run("取り消すと印が消え、recalc してから verify すると通る", func(t *testing.T) {
		dir := newDataDir(t, map[string]string{"2026-08-01-tour.json": okJudgment, "2026-08-05-fix.json": fix("2026-08-01-tour.json", 0)})
		if code, _, errOut := runCLI("recalc", "--data", dir); code != exitOK {
			t.Fatalf("recalc の終了コード = %d, stderr = %s", code, errOut)
		}
		raw, err := os.ReadFile(filepath.Join(dir, stateFile))
		if err != nil {
			t.Fatal(err)
		}
		doc, err := decodeState(raw)
		if err != nil {
			t.Fatal(err)
		}
		if it := itemOf(t, doc, "go-01"); len(it.EvidencedLevels) != 0 || len(doc.Retracted) != 1 {
			t.Errorf("go-01 = %+v, retracted = %+v", it, doc.Retracted)
		}
		if code, _, errOut := runCLI("verify", "--data", dir); code != exitOK {
			t.Errorf("verify の終了コード = %d, stderr = %s", code, errOut)
		}
	})

	// 指し先の無い取り消しを黙って捨てると、取り消したつもりの印が残る
	for _, mode := range []string{"recalc", "verify"} {
		t.Run(mode+" は指し先の無い取り消しで止まり、state.json を書かない", func(t *testing.T) {
			dir := newDataDir(t, map[string]string{"2026-08-01-tour.json": okJudgment, "2026-08-05-fix.json": fix("2026-08-01-tour.json", 1)})
			code, _, errOut := runCLI(mode, "--data", dir)
			if code != exitFailed || !strings.Contains(errOut, "judgments/2026-08-05-fix.json: retractions[0]: 2026-08-01-tour.json の判定は 1 件") {
				t.Errorf("終了コード = %d, stderr = %s", code, errOut)
			}
			if _, err := os.Stat(filepath.Join(dir, stateFile)); err == nil {
				t.Error("止まったのに state.json が書かれている")
			}
		})
	}
}

func TestRun_judgmentsディレクトリが無ければ判定0件(t *testing.T) {
	// git は空のディレクトリを記録しないので、判定をコミットする前の CI には judgments/ が無い
	dir := newDataDir(t, nil)
	if code, _, errOut := runCLI("recalc", "--data", dir); code != exitOK {
		t.Fatalf("終了コード = %d, stderr = %s", code, errOut)
	}
	if code, _, errOut := runCLI("verify", "--data", dir); code != exitOK {
		t.Errorf("終了コード = %d, stderr = %s", code, errOut)
	}
}

func TestRun_judgmentsの中の扱い(t *testing.T) {
	dir := newDataDir(t, map[string]string{
		"2026-08-01-tour.json": okJudgment,
		".gitkeep":             "",
	})
	if code, _, errOut := runCLI("recalc", "--data", dir); code != exitOK {
		t.Fatalf("`.` で始まるファイルは読まない: 終了コード = %d, stderr = %s", code, errOut)
	}

	if err := os.Mkdir(filepath.Join(dir, judgmentsDir, "old"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := runCLI("recalc", "--data", dir)
	if code != exitFailed || !strings.Contains(errOut, "ディレクトリは置けません") {
		t.Errorf("終了コード = %d, stderr = %s", code, errOut)
	}
}

func TestRun_ロードマップ(t *testing.T) {
	t.Run("エラーがあれば止まる", func(t *testing.T) {
		dir := newDataDir(t, nil)
		copyFile(t, filepath.Join(testdataDir, "roadmap-broken.json"), filepath.Join(dir, roadmapFile))
		code, _, errOut := runCLI("verify", "--data", dir)
		if code != exitFailed || !strings.Contains(errOut, "roadmap.json") {
			t.Errorf("終了コード = %d, stderr = %s", code, errOut)
		}
	})

	t.Run("無ければファイルの読み込みの失敗", func(t *testing.T) {
		code, _, errOut := runCLI("verify", "--data", t.TempDir())
		if code != exitUsage || !strings.Contains(errOut, "roadmap.json を読めません") {
			t.Errorf("終了コード = %d, stderr = %s", code, errOut)
		}
	})
}

func TestRun_設定(t *testing.T) {
	unsure := strings.Replace(okJudgment, `"confidence": 0.9`, `"confidence": 0.4`, 1)

	t.Run("閾値を settings.json から読む", func(t *testing.T) {
		dir := newDataDir(t, map[string]string{"2026-08-01-tour.json": unsure})
		writeFile(t, filepath.Join(dir, settingsFile), `{"rules": {"confidenceThreshold": 0.3}}`)
		runCLI("recalc", "--data", dir)
		raw, _ := os.ReadFile(filepath.Join(dir, stateFile))
		if !strings.Contains(string(raw), `"deferred": []`) {
			t.Errorf("閾値 0.3 なら 0.4 は保留にならない: %s", raw)
		}
	})

	t.Run("無ければ既定値（0.5）", func(t *testing.T) {
		dir := newDataDir(t, map[string]string{"2026-08-01-tour.json": unsure})
		_, out, _ := runCLI("recalc", "--data", dir)
		if !strings.Contains(out, "V7_low_confidence") {
			t.Errorf("既定の閾値 0.5 なら 0.4 は保留: %s", out)
		}
	})

	t.Run("未知のキーは止まる", func(t *testing.T) {
		dir := newDataDir(t, nil)
		writeFile(t, filepath.Join(dir, settingsFile), `{"rules": {"confidenceThreshhold": 0.3}}`)
		code, _, errOut := runCLI("verify", "--data", dir)
		if code != exitFailed || !strings.Contains(errOut, "confidenceThreshhold") {
			t.Errorf("終了コード = %d, stderr = %s", code, errOut)
		}
	})
}

func TestRun_禁止語(t *testing.T) {
	// わざと語を入れた判定（理由の文に「伏せる語」、付かなかった記述に「ダミー社」）
	tainted := strings.Replace(okJudgment, `"rationale": "確認問題に答えた"`, `"rationale": "伏せる語の想定問題に答えた"`, 1)
	tainted = strings.Replace(tainted, `"unmatched": []`, `"unmatched": ["ダミー社の話をした"]`, 1)

	t.Run("settings.json の語があれば verify は終了コード 1 で、場所を出す", func(t *testing.T) {
		dir := newDataDir(t, map[string]string{"2026-08-01-tour.json": tainted})
		writeFile(t, filepath.Join(dir, settingsFile), `{"forbiddenWords": ["伏せる語"]}`)
		runCLI("recalc", "--data", dir)
		code, _, errOut := runCLI("verify", "--data", dir)
		if code != exitFailed || !strings.Contains(errOut, "2026-08-01-tour.json: judgments[0].rationale に「伏せる語」") {
			t.Errorf("終了コード = %d, stderr = %s", code, errOut)
		}
	})

	t.Run("recalc も終了コード 1。state.json は書く（棄却と同じく、何が起きたかを見られるように）", func(t *testing.T) {
		dir := newDataDir(t, map[string]string{"2026-08-01-tour.json": tainted})
		writeFile(t, filepath.Join(dir, settingsFile), `{"forbiddenWords": ["伏せる語"]}`)
		code, _, errOut := runCLI("recalc", "--data", dir)
		if code != exitFailed || !strings.Contains(errOut, "禁止語が 1 件") {
			t.Errorf("終了コード = %d, stderr = %s", code, errOut)
		}
		if _, err := os.Stat(filepath.Join(dir, stateFile)); err != nil {
			t.Errorf("state.json が書かれていない: %v", err)
		}
	})

	t.Run("forbidden-words.local.json の語も調べる", func(t *testing.T) {
		dir := newDataDir(t, map[string]string{"2026-08-01-tour.json": tainted})
		writeFile(t, filepath.Join(dir, localForbiddenFile), `{"forbiddenWords": ["ダミー社"]}`)
		runCLI("recalc", "--data", dir)
		code, _, errOut := runCLI("verify", "--data", dir)
		if code != exitFailed || !strings.Contains(errOut, "unmatched[0] に「ダミー社」") {
			t.Errorf("終了コード = %d, stderr = %s", code, errOut)
		}
	})

	t.Run("語のファイルがどちらも無ければ調べずに通る（CI には手元のファイルが無い）", func(t *testing.T) {
		dir := newDataDir(t, map[string]string{"2026-08-01-tour.json": tainted})
		runCLI("recalc", "--data", dir)
		if code, _, errOut := runCLI("verify", "--data", dir); code != exitOK {
			t.Errorf("終了コード = %d, stderr = %s", code, errOut)
		}
	})

	t.Run("語の無い判定は通る", func(t *testing.T) {
		dir := newDataDir(t, map[string]string{"2026-08-01-tour.json": okJudgment})
		writeFile(t, filepath.Join(dir, settingsFile), `{"forbiddenWords": ["伏せる語"]}`)
		writeFile(t, filepath.Join(dir, localForbiddenFile), `{"forbiddenWords": ["ダミー社"]}`)
		if code, _, errOut := runCLI("recalc", "--data", dir); code != exitOK {
			t.Fatalf("recalc の終了コード = %d, stderr = %s", code, errOut)
		}
		if code, _, errOut := runCLI("verify", "--data", dir); code != exitOK {
			t.Errorf("verify の終了コード = %d, stderr = %s", code, errOut)
		}
	})

	t.Run("forbidden-words.local.json が崩れていたら止まる", func(t *testing.T) {
		dir := newDataDir(t, map[string]string{"2026-08-01-tour.json": okJudgment})
		writeFile(t, filepath.Join(dir, localForbiddenFile), `{"forbiddenWord": ["ダミー社"]}`)
		code, _, errOut := runCLI("verify", "--data", dir)
		if code != exitFailed || !strings.Contains(errOut, localForbiddenFile) {
			t.Errorf("終了コード = %d, stderr = %s", code, errOut)
		}
	})
}

func TestRun_使い方の誤り(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"モードが無い", nil},
		{"知らないモード", []string{"check"}},
		{"知らないフラグ", []string{"verify", "--date", "x"}},
		{"余分な引数", []string{"verify", "extra"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code, _, _ := runCLI(tt.args...); code != exitUsage {
				t.Errorf("終了コード = %d, want %d", code, exitUsage)
			}
		})
	}
}

// CLI は DB・HTTP・LLM クライアントを import しない（SPEC.md §6）。間接的な依存まで含めて確かめる。
// internal/llm を import すると、棚上げ中の Claude API の SDK と net/http が付いてくる（KNOWLEDGE.md 2026-09-26）。
func TestCLIはDBとHTTPとLLMクライアントに依存しない(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go コマンドが見つからない")
	}
	out, err := exec.Command(goBin, "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list に失敗: %v", err)
	}
	deps := strings.Fields(string(out))

	const module = "github.com/n-yoshida-dev/skill-matrix/"
	forbidden := []string{
		"net/http",
		"database/sql",
		"github.com/anthropics/anthropic-sdk-go",
		"github.com/jackc/pgx/v5",
		module + "internal/llm",
		module + "internal/store",
		module + "internal/httpapi",
		module + "internal/worker",
		module + "internal/config",
	}
	for _, bad := range forbidden {
		if slices.Contains(deps, bad) {
			t.Errorf("%s に依存している", bad)
		}
	}
}

// testdata のダミー判定を再計算した結果を、期待値ファイルと突き合わせる。
// 書き出しの形（字下げ・キーの順・空の配列・null）が変わったらここで気づく。
func TestRecalc_testdataのダミー判定(t *testing.T) {
	dir := t.TempDir()
	copyFile(t, filepath.Join(testdataDir, "roadmap-sample.json"), filepath.Join(dir, roadmapFile))
	jdir := filepath.Join(dir, judgmentsDir)
	if err := os.Mkdir(jdir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(testdataDir, judgmentsDir))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		copyFile(t, filepath.Join(testdataDir, judgmentsDir, e.Name()), filepath.Join(jdir, e.Name()))
	}

	if code, _, errOut := runCLI("recalc", "--data", dir); code != exitOK {
		t.Fatalf("recalc の終了コード = %d, stderr = %s", code, errOut)
	}
	got, err := os.ReadFile(filepath.Join(dir, stateFile))
	if err != nil {
		t.Fatal(err)
	}

	golden := filepath.Join(testdataDir, "state-sample.json")
	if *update {
		writeFile(t, golden, string(got))
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("期待値ファイルが無い（-update で作る）: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("再計算結果が %s と違う（最初の違いは %d 行目）。意図した変化なら -update で書き直す", golden, firstDiffLine(want, got))
	}

	// 期待値ファイルをそのまま state.json に置けば verify が通る（ダミー判定に棄却は無い）
	if code, _, errOut := runCLI("verify", "--data", dir); code != exitOK {
		t.Errorf("verify の終了コード = %d, stderr = %s", code, errOut)
	}
}
