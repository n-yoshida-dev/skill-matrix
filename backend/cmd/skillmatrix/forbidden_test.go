package main

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/n-yoshida-dev/skill-matrix/internal/judgment"
)

// このテストは禁止語の検査（SPEC.md §6・§9）の約束を固定する。
// 公開される文（各判定の rationale・evidenceRefs と unmatched）のどこに入っていても見つけ、場所を全部返す。

// parseForTest は判定ファイルの中身を読む。読めなければテストを止める。
func parseForTest(t *testing.T, name, raw string) judgment.File {
	t.Helper()
	f, err := judgment.ParseFile(name, []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestFindForbidden(t *testing.T) {
	const name = "2026-08-01-tour.json"
	raw := `{"schemaVersion": 2, "loggedAt": "2026-08-01", "source": "ai", "judge": "claude-code", "judgments": [
		{"itemKey": "go-01", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["log:a.md"], "rationale": "確認問題に答えた", "confidence": 0.9},
		{"itemKey": "go-02", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["log:a.md", "repo:Example-Corp/app@abc1234"], "rationale": "伏せる語の想定問題に答えた", "confidence": 0.9}
	], "unmatched": ["説明を受けた", "禁止の例の話をした"]}`
	f := parseForTest(t, name, raw)

	t.Run("理由の文・出どころ・付かなかった記述のどれでも見つけ、場所を全部返す", func(t *testing.T) {
		got := findForbidden([]judgment.File{f}, []string{"伏せる語", "禁止の例", "example-corp"})
		want := []forbiddenHit{
			{File: name, Field: "judgments[1].rationale", Word: "伏せる語"},
			{File: name, Field: "judgments[1].evidenceRefs[1]", Word: "example-corp"},
			{File: name, Field: "unmatched[1]", Word: "禁止の例"},
		}
		if !slices.Equal(got, want) {
			t.Errorf("got  %v\nwant %v", got, want)
		}
	})

	t.Run("英字は大文字と小文字を区別しない", func(t *testing.T) {
		got := findForbidden([]judgment.File{f}, []string{"EXAMPLE-CORP"})
		if len(got) != 1 || got[0].Word != "EXAMPLE-CORP" {
			t.Errorf("got %v（リストに書かれたままの綴りで 1 件のはず）", got)
		}
	})

	t.Run("語が無ければ何も返さない", func(t *testing.T) {
		if got := findForbidden([]judgment.File{f}, nil); len(got) != 0 {
			t.Errorf("got %v", got)
		}
	})

	t.Run("当たらなければ何も返さない", func(t *testing.T) {
		if got := findForbidden([]judgment.File{f}, []string{"無い語"}); len(got) != 0 {
			t.Errorf("got %v", got)
		}
	})
}

// コミットされている実データ（data/judgments/ と data/settings.json）が禁止語 0 件で通ることを確かめる。
// 手元に forbidden-words.local.json があれば、その語も含めて調べる。
func TestFindForbidden_コミットされた判定は0件(t *testing.T) {
	in, err := load(filepath.Join("..", "..", "..", "data"))
	if err != nil {
		t.Fatalf("data/ を読めません: %v", err)
	}
	if len(in.settings.forbiddenWords) == 0 {
		t.Fatal("data/settings.json に forbiddenWords が無い（検査が空のまま通ってしまう）")
	}
	if hits := findForbidden(in.files, in.forbiddenWords); len(hits) != 0 {
		t.Errorf("禁止語が見つかった: %v", hits)
	}
}
