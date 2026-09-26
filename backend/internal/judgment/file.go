package judgment

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
)

// このファイルは判定ファイル `data/judgments/YYYY-MM-DD-<短い名前>.json`（SPEC.md §3.3）の外枠を読む。
//
// 外枠（schemaVersion / loggedAt / source / judge / judgments / unmatched）の崩れはファイル全体のエラーにして
// CLI を止める。判定1件の中の崩れは Parse と同じく、その1件だけを Rejected に入れて続ける。
// 外枠が崩れていると source も loggedAt も信用できず、どの判定をどう扱うかが決まらないため。

// FileSchemaVersion は判定ファイルの形の版。形を変えたら上げる。
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
	Unmatched     []string           `json:"unmatched"`
}

// validSources は判定を書く主体として認める値（SPEC.md §3.3）。
var validSources = map[domain.JudgmentSource]bool{
	domain.SourceAI:        true,
	domain.SourceManual:    true,
	domain.SourceMigration: true,
}

// ParseFile は判定ファイル1本を読む。name はファイル名（ディレクトリを含まない）、raw は中身。
//
// 外枠の崩れ（ファイル名・JSON・未知のキー・schemaVersion・loggedAt・source・judgments）は *FileError を返す。
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

	f := File{
		Name:     name,
		LoggedAt: loggedAt,
		Source:   source,
		Output:   parseJudgments(*w.Judgments, source),
	}
	f.Output.Unmatched = w.Unmatched
	if w.Judge != nil {
		f.Judge = *w.Judge
	}
	return f, nil
}
