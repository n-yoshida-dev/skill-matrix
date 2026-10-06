package judgment

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
)

// このファイルは判定ファイル `data/judgments/YYYY-MM-DD-<短い名前>.json`（SPEC.md §3.3）の外枠を読む。
//
// 外枠（schemaVersion / loggedAt / source / judge / judgments / retractions / unmatched）の崩れはファイル全体のエラーにして
// CLI を止める。判定1件の中の崩れは Parse と同じく、その1件だけを Rejected に入れて続ける。
// 外枠が崩れていると source も loggedAt も信用できず、どの判定をどう扱うかが決まらないため。
// 取り消しの記録（retractions）の崩れも外枠の崩れとして扱う。取り消しは別のファイルの判定に効くので、
// 1 件だけ弾いて続けると「取り消したつもりの印が残る」が黙って起きる。

// FileSchemaVersion は判定ファイルの形の版。既存の判定ファイルが読めなくなる変え方をしたら上げる。
// 省略できる欄を足すだけ（2026-10-06 の retractions）なら上げない。上げると既存の判定ファイルを全部書き換えることになり、
// 「追記のみ」に反する（古い CLI は未知のキーで止まるので、黙って読み違えることも無い）。
const FileSchemaVersion = 2

// fileNamePattern は判定ファイルの名前（SPEC.md §3.3）。先頭の日付が loggedAt と一致する。
var fileNamePattern = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-[a-z0-9][a-z0-9-]{0,63}\.json$`)

// File は判定ファイル1本を読み取ったもの。
type File struct {
	// Name はファイル名（ディレクトリを含まない）。処理順と state.json の記録に使う。
	Name string
	// LoggedAt は判定を書いた日。判定に occurredAt が無いときの既定になる。
	LoggedAt time.Time
	// Source は誰が書いた判定か。confidence の要否と V7 の対象かどうかが決まる。
	Source domain.JudgmentSource
	// Judge は判定した AI ツール名（任意）。
	Judge string
	// Output は judgments と unmatched。形の崩れた判定は Output.Rejected に入る。
	Output Output
	// Retractions は取り消しの記録。書かれた順を保つ。source が manual のファイルだけが持てる（SPEC.md §3.3）。
	Retractions []Retraction
}

// Retraction は取り消しの記録1件。誤ってコミットした判定を、元のファイルを触らずに適用から外す（SPEC.md §4.7）。
//
// ここで確かめるのは1ファイルの中で分かることだけ。指し先のファイルと判定が実在するか、
// 同じ判定を2回取り消していないかは、全ファイルを見られる呼び出し側（CLI）が確かめる。
type Retraction struct {
	// File は取り消す判定のあるファイル名（ディレクトリを含まない）。
	File string
	// Index は取り消す判定の、そのファイルの judgments 配列での位置（0 始まり）。
	Index int
	// Reason は取り消す理由。公開される文なので、技術的な事実だけを書く。
	Reason string
}

// FileError は判定ファイルの外枠が読めないことを表す。どの判定も適用できないので、呼び出し側は止まる。
type FileError struct {
	// Name はファイル名。
	Name string
	// Reason は読めなかった理由。人が読むための文。
	Reason string
}

// Error はエラーの文面を返す。
func (e *FileError) Error() string {
	return fmt.Sprintf("%s: %s", e.Name, e.Reason)
}

// wireFile は判定ファイルの外枠。欄の欠落とゼロ値を区別するためにポインタで受ける。
type wireFile struct {
	SchemaVersion *int               `json:"schemaVersion"`
	LoggedAt      *string            `json:"loggedAt"`
	Source        *string            `json:"source"`
	Judge         *string            `json:"judge"`
	Judgments     *[]json.RawMessage `json:"judgments"`
	Retractions   []wireRetraction   `json:"retractions"`
	Unmatched     []string           `json:"unmatched"`
}

// wireRetraction は取り消しの記録1件。欄の欠落とゼロ値（index の 0）を区別するためにポインタで受ける。
// 外枠の decoder に DisallowUnknownFields を掛けているので、ここの未知のキーもエラーになる。
type wireRetraction struct {
	File   *string `json:"file"`
	Index  *int    `json:"index"`
	Reason *string `json:"reason"`
}

// validSources は判定を書く主体として認める値（SPEC.md §3.3）。
var validSources = map[domain.JudgmentSource]bool{
	domain.SourceAI:        true,
	domain.SourceManual:    true,
	domain.SourceMigration: true,
}

// ParseFile は判定ファイル1本を読む。name はファイル名（ディレクトリを含まない）、raw は中身。
//
// 外枠の崩れ（ファイル名・JSON・未知のキー・schemaVersion・loggedAt・source・judgments・retractions）は *FileError を返す。
// 判定1件の中の崩れはエラーにせず Output.Rejected に入れる（SPEC.md §4.5）。
func ParseFile(name string, raw []byte) (File, error) {
	fail := func(format string, args ...any) (File, error) {
		return File{}, &FileError{Name: name, Reason: fmt.Sprintf(format, args...)}
	}

	m := fileNamePattern.FindStringSubmatch(name)
	if m == nil {
		return fail("ファイル名は YYYY-MM-DD-<英小文字・数字・ハイフン>.json にしてください")
	}

	// 外枠の未知のキーはファイル全体のエラー。`judgment`（単数）のような打ち間違いで判定が黙って消えるのを防ぐ
	var w wireFile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&w); err != nil {
		return fail("外枠を読めません: %v", err)
	}
	// 1つの JSON オブジェクトだけを受け付ける。後ろに続きがあれば、書きかけか2つを連結したファイル
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return fail("JSON オブジェクトの後ろに余分なデータがあります")
	}

	if w.SchemaVersion == nil {
		return fail("schemaVersion がありません")
	}
	if *w.SchemaVersion != FileSchemaVersion {
		return fail("schemaVersion は %d にしてください（%d が書かれています）", FileSchemaVersion, *w.SchemaVersion)
	}

	if w.LoggedAt == nil {
		return fail("loggedAt がありません")
	}
	loggedAt, err := time.Parse(dateLayout, *w.LoggedAt)
	if err != nil {
		return fail("loggedAt は YYYY-MM-DD で書いてください: %q", *w.LoggedAt)
	}
	// ファイル名の日付と loggedAt がずれると、処理順（ファイル名の昇順）と日付の順が食い違う
	if *w.LoggedAt != m[1] {
		return fail("loggedAt（%s）がファイル名の日付（%s）と一致しません", *w.LoggedAt, m[1])
	}

	if w.Source == nil {
		return fail("source がありません（ai / manual / migration）")
	}
	source := domain.JudgmentSource(*w.Source)
	if !validSources[source] {
		return fail("source は ai / manual / migration のどれかにしてください: %q", *w.Source)
	}

	if w.Judgments == nil {
		return fail("judgments がありません")
	}

	retractions, reason := parseRetractions(name, source, w.Retractions)
	if reason != "" {
		return fail("%s", reason)
	}

	f := File{
		Name:        name,
		LoggedAt:    loggedAt,
		Source:      source,
		Output:      parseJudgments(*w.Judgments, source),
		Retractions: retractions,
	}
	f.Output.Unmatched = w.Unmatched
	if w.Judge != nil {
		f.Judge = *w.Judge
	}
	return f, nil
}

// parseRetractions は取り消しの記録を読む。崩れていれば理由を返す（理由が空なら成功）。
//
// 取り消しは人の訂正なので、書けるのは source が manual のファイルだけ。AI の判定ファイルに混ぜると、
// 判定した AI が過去の判定を自分の一存で外せてしまう。
// 指し先は自分より前に処理されるファイル（ファイル名の昇順で前）に限る。処理順はファイル名だけで決まる（SPEC.md §3.2）ので、
// 「後から足した訂正が、前の判定を外す」の向きをファイル名で守る。
func parseRetractions(name string, source domain.JudgmentSource, ws []wireRetraction) ([]Retraction, string) {
	if len(ws) == 0 {
		return nil, ""
	}
	if source != domain.SourceManual {
		return nil, fmt.Sprintf("retractions を書けるのは source が manual のファイルだけです（%q が書かれています）", source)
	}

	out := make([]Retraction, 0, len(ws))
	seen := make(map[Retraction]bool, len(ws))
	for i, w := range ws {
		switch {
		case w.File == nil:
			return nil, fmt.Sprintf("retractions[%d] に file がありません", i)
		case w.Index == nil:
			return nil, fmt.Sprintf("retractions[%d] に index がありません", i)
		case w.Reason == nil || strings.TrimSpace(*w.Reason) == "":
			return nil, fmt.Sprintf("retractions[%d] に reason がありません（取り消す理由を技術的な事実で書いてください）", i)
		}
		if !fileNamePattern.MatchString(*w.File) {
			return nil, fmt.Sprintf("retractions[%d].file は判定ファイルの名前（YYYY-MM-DD-<短い名前>.json）にしてください: %q", i, *w.File)
		}
		if *w.File >= name {
			return nil, fmt.Sprintf("retractions[%d].file（%s）は、このファイルより前に処理されるファイル（ファイル名の昇順で前）にしてください", i, *w.File)
		}
		if *w.Index < 0 {
			return nil, fmt.Sprintf("retractions[%d].index は 0 以上にしてください: %d", i, *w.Index)
		}
		key := Retraction{File: *w.File, Index: *w.Index}
		if seen[key] {
			return nil, fmt.Sprintf("retractions[%d] は同じ判定（%s の %d 件目）を2回取り消しています", i, *w.File, *w.Index)
		}
		seen[key] = true
		out = append(out, Retraction{File: *w.File, Index: *w.Index, Reason: *w.Reason})
	}
	return out, ""
}
