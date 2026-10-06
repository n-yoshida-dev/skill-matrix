package judgment

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
)

// このテストは判定ファイル（SPEC.md §3.3）の外枠のうち、どこまでをファイル全体のエラーにするかを固定する。

// validFile は外枠が正しい判定ファイル。各ケースはここから 1 か所だけ崩す。
const validFile = `{
	"schemaVersion": 2,
	"loggedAt": "2026-10-01",
	"source": "ai",
	"judge": "claude-code",
	"judgments": [
		{"itemKey": "go-01", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["log:a.md"], "rationale": "x", "confidence": 0.9}
	],
	"unmatched": ["どの項目にも対応しない記述"]
}`

func TestParseFile(t *testing.T) {
	t.Run("正しいファイルを読める", func(t *testing.T) {
		f, err := ParseFile("2026-10-01-tour.json", []byte(validFile))
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if f.Name != "2026-10-01-tour.json" || f.Source != domain.SourceAI || f.Judge != "claude-code" {
			t.Errorf("Name = %q, Source = %q, Judge = %q", f.Name, f.Source, f.Judge)
		}
		if want := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC); !f.LoggedAt.Equal(want) {
			t.Errorf("LoggedAt = %v, want %v", f.LoggedAt, want)
		}
		if len(f.Output.Judgments) != 1 || len(f.Output.Unmatched) != 1 {
			t.Errorf("Output = %+v", f.Output)
		}
	})

	t.Run("judge と unmatched は省略できる", func(t *testing.T) {
		raw := `{"schemaVersion": 2, "loggedAt": "2026-10-01", "source": "manual", "judgments": []}`
		f, err := ParseFile("2026-10-01-fix.json", []byte(raw))
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if f.Judge != "" || len(f.Output.Unmatched) != 0 {
			t.Errorf("Judge = %q, Unmatched = %v", f.Judge, f.Output.Unmatched)
		}
	})

	t.Run("判定1件の崩れはファイルのエラーにせず、その1件だけを弾く", func(t *testing.T) {
		raw := `{"schemaVersion": 2, "loggedAt": "2026-10-01", "source": "migration", "judgments": [
			{"itemKey": "go-01", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["log:a.md"], "rationale": "x", "confidence": 0.9},
			{"itemKey": "go-02", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["log:a.md"], "rationale": "y"}
		]}`
		f, err := ParseFile("2026-10-01-migration-go.json", []byte(raw))
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		// migration に confidence は書けない（source が判定1件の検査まで届いている）
		if len(f.Output.Rejected) != 1 || f.Output.Rejected[0].Index != 0 {
			t.Errorf("Rejected = %+v", f.Output.Rejected)
		}
		if len(f.Output.Judgments) != 1 || f.Output.Judgments[0].Index != 1 {
			t.Errorf("Judgments = %+v", f.Output.Judgments)
		}
	})
}

// retractionFile は取り消しの記録だけを持つ 2026-10-02 の判定ファイルを作る。
func retractionFile(source string, elems ...string) string {
	return `{"schemaVersion": 2, "loggedAt": "2026-10-02", "source": "` + source + `", "judgments": [], "retractions": [` +
		strings.Join(elems, ", ") + `]}`
}

func TestParseFile_取り消しの記録(t *testing.T) {
	t.Run("取り消しを書いた順に読める", func(t *testing.T) {
		raw := retractionFile("manual",
			`{"file": "2026-10-01-tour.json", "index": 2, "reason": "翌日の出来事だった"}`,
			`{"file": "2026-09-30-intro.json", "index": 0, "reason": "別の項目の話だった"}`)
		f, err := ParseFile("2026-10-02-fix.json", []byte(raw))
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		want := []Retraction{
			{File: "2026-10-01-tour.json", Index: 2, Reason: "翌日の出来事だった"},
			{File: "2026-09-30-intro.json", Index: 0, Reason: "別の項目の話だった"},
		}
		if len(f.Retractions) != len(want) {
			t.Fatalf("Retractions = %+v", f.Retractions)
		}
		for i := range want {
			if f.Retractions[i] != want[i] {
				t.Errorf("Retractions[%d] = %+v, want %+v", i, f.Retractions[i], want[i])
			}
		}
	})

	t.Run("retractions は省略でき、空の配列なら AI のファイルにも書ける", func(t *testing.T) {
		for _, raw := range []string{validFile, strings.Replace(validFile, `"unmatched"`, `"retractions": [], "unmatched"`, 1)} {
			f, err := ParseFile("2026-10-01-tour.json", []byte(raw))
			if err != nil {
				t.Fatalf("エラーになった: %v", err)
			}
			if len(f.Retractions) != 0 {
				t.Errorf("Retractions = %+v", f.Retractions)
			}
		}
	})

	t.Run("同じファイルの別の判定なら並べて取り消せる", func(t *testing.T) {
		raw := retractionFile("manual",
			`{"file": "2026-10-01-tour.json", "index": 0, "reason": "x"}`, `{"file": "2026-10-01-tour.json", "index": 1, "reason": "y"}`)
		if _, err := ParseFile("2026-10-02-fix.json", []byte(raw)); err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
	})
}

func TestParseFile_外枠の崩れはファイル全体のエラー(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		raw        string
		wantReason string
	}{
		{"ファイル名に日付が無い", "tour.json", validFile, "ファイル名"},
		{"ファイル名に大文字がある", "2026-10-01-Tour.json", validFile, "ファイル名"},
		{"拡張子が違う", "2026-10-01-tour.jsonc", validFile, "ファイル名"},
		{"JSON でない", "2026-10-01-tour.json", `判定`, "外枠を読めません"},
		{"外枠に未知のキーがある", "2026-10-01-tour.json", strings.Replace(validFile, `"judge"`, `"judgeName"`, 1), "外枠を読めません"},
		{"JSON の後ろに続きがある", "2026-10-01-tour.json", validFile + `{}`, "余分なデータ"},
		{"schemaVersion が無い", "2026-10-01-tour.json", strings.Replace(validFile, `"schemaVersion": 2,`, ``, 1), "schemaVersion がありません"},
		{"schemaVersion が古い", "2026-10-01-tour.json", strings.Replace(validFile, `"schemaVersion": 2`, `"schemaVersion": 1`, 1), "schemaVersion は 2"},
		{"loggedAt が無い", "2026-10-01-tour.json", strings.Replace(validFile, `"loggedAt": "2026-10-01",`, ``, 1), "loggedAt がありません"},
		{"loggedAt の書式が違う", "2026-10-01-tour.json", strings.Replace(validFile, `"2026-10-01"`, `"2026/10/01"`, 1), "YYYY-MM-DD"},
		{"loggedAt がファイル名の日付と違う", "2026-10-02-tour.json", validFile, "一致しません"},
		{"存在しない日付", "2026-02-30-tour.json", strings.Replace(validFile, `"2026-10-01"`, `"2026-02-30"`, 1), "YYYY-MM-DD"},
		{"source が無い", "2026-10-01-tour.json", strings.Replace(validFile, `"source": "ai",`, ``, 1), "source がありません"},
		{"source が知らない値", "2026-10-01-tour.json", strings.Replace(validFile, `"source": "ai"`, `"source": "gpt"`, 1), "source は ai / manual / migration"},
		{"judgments が無い", "2026-10-01-tour.json", `{"schemaVersion": 2, "loggedAt": "2026-10-01", "source": "ai"}`, "judgments がありません"},
		{"judgments が配列でない", "2026-10-01-tour.json", `{"schemaVersion": 2, "loggedAt": "2026-10-01", "source": "ai", "judgments": {}}`, "外枠を読めません"},
		{"unmatched が文字列の配列でない", "2026-10-01-tour.json", strings.Replace(validFile, `["どの項目にも対応しない記述"]`, `[1]`, 1), "外枠を読めません"},
		{"AI の判定ファイルに取り消しがある", "2026-10-02-fix.json", retractionFile("ai", `{"file": "2026-10-01-tour.json", "index": 0, "reason": "x"}`), "manual のファイルだけ"},
		{"取り消しに file が無い", "2026-10-02-fix.json", retractionFile("manual", `{"index": 0, "reason": "x"}`), "file がありません"},
		{"取り消しに index が無い", "2026-10-02-fix.json", retractionFile("manual", `{"file": "2026-10-01-tour.json", "reason": "x"}`), "index がありません"},
		{"取り消しに reason が無い", "2026-10-02-fix.json", retractionFile("manual", `{"file": "2026-10-01-tour.json", "index": 0}`), "reason がありません"},
		{"取り消しの reason が空白だけ", "2026-10-02-fix.json", retractionFile("manual", `{"file": "2026-10-01-tour.json", "index": 0, "reason": "  "}`), "reason がありません"},
		{"取り消しの file が判定ファイルの名前でない", "2026-10-02-fix.json", retractionFile("manual", `{"file": "tour.json", "index": 0, "reason": "x"}`), "判定ファイルの名前"},
		{"取り消しの file が自分自身", "2026-10-02-fix.json", retractionFile("manual", `{"file": "2026-10-02-fix.json", "index": 0, "reason": "x"}`), "前に処理される"},
		{"取り消しの file が自分より後", "2026-10-02-fix.json", retractionFile("manual", `{"file": "2026-10-03-tour.json", "index": 0, "reason": "x"}`), "前に処理される"},
		{"取り消しの index が負", "2026-10-02-fix.json", retractionFile("manual", `{"file": "2026-10-01-tour.json", "index": -1, "reason": "x"}`), "0 以上"},
		{"取り消しの index が整数でない", "2026-10-02-fix.json", retractionFile("manual", `{"file": "2026-10-01-tour.json", "index": 1.5, "reason": "x"}`), "外枠を読めません"},
		{"取り消しに未知のキーがある", "2026-10-02-fix.json", retractionFile("manual", `{"file": "2026-10-01-tour.json", "index": 0, "reason": "x", "by": "me"}`), "外枠を読めません"},
		{"同じ判定を2回取り消す", "2026-10-02-fix.json", retractionFile("manual",
			`{"file": "2026-10-01-tour.json", "index": 0, "reason": "x"}`, `{"file": "2026-10-01-tour.json", "index": 0, "reason": "y"}`), "2回取り消して"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseFile(tt.file, []byte(tt.raw))
			var fe *FileError
			if !errors.As(err, &fe) {
				t.Fatalf("*FileError のはずが %v だった", err)
			}
			if fe.Name != tt.file || !strings.Contains(fe.Reason, tt.wantReason) {
				t.Errorf("Name = %q, Reason = %q（%q を含むはず）", fe.Name, fe.Reason, tt.wantReason)
			}
		})
	}
}
