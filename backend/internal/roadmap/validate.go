package roadmap

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// このファイルはマスタ JSON の検査を担う。方針は3つ。
//
//   - **1件目で打ち切らない。** 見つかった問題は全部集めて返す。1つ直すたびに投稿し直して
//     次の1つが出る、という直し方をさせないため（internal/config と同じ流儀）
//   - **エラーと警告を分ける。** エラーはインポートを止める。警告は通すが画面に出す。
//     「external なのに outcome が無い」は警告に留める（SPEC.md §2.1）
//   - **場所を必ず示す。** domains[0].items[2].key のような経路を添えて、
//     フロントがどこが悪いかを指せるようにする

// Severity は問題の重さ。
type Severity string

const (
	// SeverityError はインポートを中止させる。
	SeverityError Severity = "error"
	// SeverityWarning はインポートは通すが、利用者に伝える。
	SeverityWarning Severity = "warning"
)

// IssueCode は問題の種類。文言ではなくこの値でフロントが分岐する。
type IssueCode string

const (
	IssueInvalidJSON              IssueCode = "invalid_json"               // JSON として読めない
	IssueUnknownField             IssueCode = "unknown_field"              // 定義にないフィールド
	IssueUnsupportedSchemaVersion IssueCode = "unsupported_schema_version" // 対応していない schemaVersion
	IssueRequired                 IssueCode = "required"                   // 必須なのに空
	IssueInvalidFormat            IssueCode = "invalid_format"             // 形式が不正（key・日付・URL）
	IssueInvalidOrigin            IssueCode = "invalid_origin"             // 既知でない origin
	IssueInvalidLevels            IssueCode = "invalid_levels"             // レベル定義が 1〜5 を満たさない
	IssueDuplicateKey             IssueCode = "duplicate_key"              // key の重複
	IssueUnknownDependency        IssueCode = "unknown_dependency"         // 実在しない項目への依存
	IssueSelfDependency           IssueCode = "self_dependency"            // 自分自身への依存
	IssueCircularDependency       IssueCode = "circular_dependency"        // 循環参照
	IssueTooMany                  IssueCode = "too_many"                   // 件数の上限超え
	IssueTooLong                  IssueCode = "too_long"                   // 1フィールドの文字数の上限超え
	IssueMissingOutcome           IssueCode = "missing_outcome"            // 到達状態が無い
)

// Issue は検査で見つかった問題1件。
type Issue struct {
	Severity Severity  `json:"severity"`
	Code     IssueCode `json:"code"`
	// Path は問題のあった場所（例: "domains[0].items[2].key"）。全体に関わるものは空。
	Path string `json:"path,omitempty"`
	// Message は人が読むための説明。分岐には使わせない。
	Message string `json:"message"`
}

// Result は検査結果。**問題が無いことではなく、問題の一覧を持つ。**
type Result struct {
	Issues []Issue `json:"issues"`
}

// OK はインポートしてよいか（エラーが1件も無いか）を返す。警告だけなら true。
func (r Result) OK() bool {
	for _, is := range r.Issues {
		if is.Severity == SeverityError {
			return false
		}
	}
	return true
}

// Errors はインポートを止める問題だけを返す。
func (r Result) Errors() []Issue { return r.filter(SeverityError) }

// Warnings は通すが伝えるべき問題だけを返す。
func (r Result) Warnings() []Issue { return r.filter(SeverityWarning) }

func (r Result) filter(s Severity) []Issue {
	var out []Issue
	for _, is := range r.Issues {
		if is.Severity == s {
			out = append(out, is)
		}
	}
	return out
}

// add は問題を1件積む。
func (r *Result) add(sev Severity, code IssueCode, path, format string, args ...any) {
	r.Issues = append(r.Issues, Issue{
		Severity: sev,
		Code:     code,
		Path:     path,
		Message:  fmt.Sprintf(format, args...),
	})
}

func (r *Result) errorf(code IssueCode, path, format string, args ...any) {
	r.add(SeverityError, code, path, format, args...)
}

func (r *Result) warnf(code IssueCode, path, format string, args ...any) {
	r.add(SeverityWarning, code, path, format, args...)
}

// keyPattern は key に許す形（SPEC.md §2）。
// URL やファイル名にそのまま使えて、目で見て区別できる範囲に絞る。
var keyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// unknownFieldPattern は encoding/json が返す未知フィールドのエラーからキー名を取り出す。
var unknownFieldPattern = regexp.MustCompile(`unknown field "([^"]*)"`)

// ---------------------------------------------------------------------------
// 読み込み
// ---------------------------------------------------------------------------

// Parse はバイト列を Document に読み込む。
//
// **未知のフィールドはエラーにする。** dependsOn を dependOn と打ち間違えたときに、
// 黙って依存関係が消えて学習パスの並び順だけが静かに狂う、という壊れ方を避けるため。
//
// 読み込めなかった場合、返る Document は nil。
func Parse(data []byte) (*Document, Result) {
	var res Result

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var doc Document
	if err := dec.Decode(&doc); err != nil {
		res.errorf(codeForDecodeError(err), "", "%s", messageForDecodeError(err))
		return nil, res
	}

	// 1つの JSON オブジェクトだけを受け付ける。後ろにゴミが続いていたら弾く
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		res.errorf(IssueInvalidJSON, "", "JSON オブジェクトの後ろに余分なデータがあります")
		return nil, res
	}

	return &doc, res
}

// codeForDecodeError は読み込み失敗の種類を分ける。
func codeForDecodeError(err error) IssueCode {
	if unknownFieldPattern.MatchString(err.Error()) {
		return IssueUnknownField
	}
	return IssueInvalidJSON
}

// messageForDecodeError は encoding/json のエラーを日本語にする。
func messageForDecodeError(err error) string {
	if m := unknownFieldPattern.FindStringSubmatch(err.Error()); m != nil {
		return fmt.Sprintf("定義にないフィールド %q があります。綴りを確認してください", m[1])
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		where := typeErr.Field
		if where == "" {
			where = "（最上位）"
		}
		return fmt.Sprintf("%s の型が違います（%s が来ましたが %s を期待しています）", where, typeErr.Value, typeErr.Type)
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return fmt.Sprintf("JSON として読めません（%d バイト目）: %v", syntaxErr.Offset, err)
	}

	return fmt.Sprintf("JSON として読めません: %v", err)
}

// ParseAndValidate は読み込みと検査をまとめて行う。インポート API はこれを呼ぶ。
//
// 読み込みに失敗した場合、検査は行わない（構造が取れていないため検査しても意味が無い）。
func ParseAndValidate(data []byte) (*Document, Result) {
	doc, res := Parse(data)
	if doc == nil {
		return nil, res
	}
	res.Issues = append(res.Issues, Validate(doc).Issues...)
	return doc, res
}

// ---------------------------------------------------------------------------
// 検査
// ---------------------------------------------------------------------------

// Validate は Document が DB に入れてよい形かを検査する。
//
// 検査するのは SPEC.md §2 と §2.1 に書かれているものだけ。
//   - schemaVersion が対応範囲か
//   - origin 別の出典（source / checkedAt）
//   - levels が 1〜5 を全て含むか
//   - key の形式と重複
//   - dependsOn が実在する項目を指しているか、循環していないか
//   - 上限（分野・項目数）
//   - origin 別の到達状態（outcome / goal）
func Validate(doc *Document) Result {
	var res Result
	if doc == nil {
		res.errorf(IssueInvalidJSON, "", "ロードマップが空です")
		return res
	}

	origin := doc.EffectiveOrigin()

	validateMeta(doc, origin, &res)
	validateLevels(doc, &res)
	itemDomains := validateStructure(doc, origin, &res)
	validateDependencies(doc, itemDomains, &res)

	return res
}

// validateMeta はロードマップ全体の属性（名前・出典）を見る。
func validateMeta(doc *Document, origin Origin, res *Result) {
	if doc.SchemaVersion != SchemaVersion {
		res.errorf(IssueUnsupportedSchemaVersion, "schemaVersion",
			"対応している schemaVersion は %d です（%d が指定されました）", SchemaVersion, doc.SchemaVersion)
	}

	if strings.TrimSpace(doc.Name) == "" {
		res.errorf(IssueRequired, "name", "ロードマップの名前は必須です")
	}
	checkLen(doc.Name, "name", MaxNameLen, res)
	checkLen(doc.Description, "description", MaxTextLen, res)
	checkLen(doc.Source, "source", MaxSourceLen, res)

	if doc.Origin != "" && !doc.Origin.Valid() {
		res.errorf(IssueInvalidOrigin, "origin",
			"origin は %q / %q / %q のいずれかです（%q が指定されました）",
			OriginManual, OriginExternal, OriginBuiltin, doc.Origin)
		// 以降の出典チェックは origin が決まらないと判断できないので、ここで打ち切る
		return
	}

	validateSource(doc, origin, res)
}

// validateSource は出典（source / checkedAt）を origin 別に見る（SPEC.md §2）。
//
// 「外部由来の定義は出典と確認日を併記する」という規約は、外部由来のものだけに掛かる。
// 利用者が自分の頭で組んだロードマップにまで出典を要求しない。
func validateSource(doc *Document, origin Origin, res *Result) {
	sourceRequired := origin == OriginExternal
	// builtin は「元ネタがあるなら source、オリジナルなら checkedAt のみ」（SPEC.md §2）
	checkedAtRequired := origin == OriginExternal || origin == OriginBuiltin

	source := strings.TrimSpace(doc.Source)
	if source == "" {
		if sourceRequired {
			res.errorf(IssueRequired, "source", "origin が %q のときは出典（source）が必須です", origin)
		}
	} else if !looksLikeURL(source) {
		// 書籍名などを書きたい場合もあるので、URL でないことは止めるほどではない
		res.warnf(IssueInvalidFormat, "source",
			"出典が URL の形をしていません。参照できる URL を書くと後から確認しやすくなります")
	}

	checkedAt := strings.TrimSpace(doc.CheckedAt)
	switch {
	case checkedAt == "":
		if checkedAtRequired {
			res.errorf(IssueRequired, "checkedAt", "origin が %q のときは確認日（checkedAt）が必須です", origin)
		}
	default:
		if _, err := time.Parse(time.DateOnly, checkedAt); err != nil {
			res.errorf(IssueInvalidFormat, "checkedAt",
				"確認日は YYYY-MM-DD の形式で書いてください（%q が指定されました）", checkedAt)
		}
	}
}

// looksLikeURL は http(s) の絶対 URL に見えるかを返す。
func looksLikeURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// validateLevels はレベル定義が 1〜5 を過不足なく持つかを見る（SPEC.md §2）。
//
// criteria はそのまま LLM のプロンプトに入る判定基準なので、欠けていると
// 「何をもってレベル3とするか」が誰にも決められなくなる。
func validateLevels(doc *Document, res *Result) {
	if len(doc.Levels) == 0 {
		res.errorf(IssueRequired, "levels", "レベル定義（levels）は必須です。1〜5 をすべて書いてください")
		return
	}

	seen := make(map[int]int, len(doc.Levels)) // レベル値 -> 出現回数
	for i, lv := range doc.Levels {
		path := fmt.Sprintf("levels[%d]", i)

		if lv.Level < 1 || lv.Level > 5 {
			res.errorf(IssueInvalidLevels, path+".level",
				"レベルは 1〜5 の整数です（%d が指定されました）", lv.Level)
		} else {
			seen[lv.Level]++
			if seen[lv.Level] == 2 {
				res.errorf(IssueInvalidLevels, path+".level", "レベル %d が重複しています", lv.Level)
			}
		}

		if strings.TrimSpace(lv.Name) == "" {
			res.errorf(IssueRequired, path+".name", "レベルの名前は必須です")
		}
		if strings.TrimSpace(lv.Criteria) == "" {
			res.errorf(IssueRequired, path+".criteria",
				"判定基準（criteria）は必須です。この文言がそのまま AI の判定基準になります")
		}
		checkLen(lv.Name, path+".name", MaxNameLen, res)
		checkLen(lv.Criteria, path+".criteria", MaxTextLen, res)
	}

	var missing []int
	for lv := 1; lv <= 5; lv++ {
		if seen[lv] == 0 {
			missing = append(missing, lv)
		}
	}
	if len(missing) > 0 {
		res.errorf(IssueInvalidLevels, "levels", "レベル %s の定義がありません", joinInts(missing))
	}
}

// validateStructure は分野と項目の key・名前・上限・到達状態を見る。
// 戻り値は「項目 key -> それが属する分野 key」。依存関係の検査で使う。
func validateStructure(doc *Document, origin Origin, res *Result) map[string]string {
	itemDomains := make(map[string]string)

	if len(doc.Domains) == 0 {
		res.errorf(IssueRequired, "domains", "分野（domains）が1つもありません")
		return itemDomains
	}
	if len(doc.Domains) > MaxDomains {
		res.errorf(IssueTooMany, "domains",
			"分野は %d 件までです（%d 件あります）", MaxDomains, len(doc.Domains))
	}
	if total := doc.TotalItems(); total > MaxItems {
		res.errorf(IssueTooMany, "domains",
			"詳細項目はロードマップ全体で %d 件までです（%d 件あります）", MaxItems, total)
	}

	// 分野と項目で key の名前空間は分ける（DB も domains / items で別々に一意）
	domainKeys := make(map[string]int)
	itemKeys := make(map[string]string) // 項目 key -> 最初に現れた場所

	for di, dom := range doc.Domains {
		domPath := fmt.Sprintf("domains[%d]", di)

		if checkKey(dom.Key, domPath+".key", "分野", res) {
			if first, dup := domainKeys[dom.Key]; dup {
				res.errorf(IssueDuplicateKey, domPath+".key",
					"分野の key %q が domains[%d] と重複しています", dom.Key, first)
			} else {
				domainKeys[dom.Key] = di
			}
		}

		if strings.TrimSpace(dom.Name) == "" {
			res.errorf(IssueRequired, domPath+".name", "分野の名前は必須です")
		}
		checkLen(dom.Name, domPath+".name", MaxNameLen, res)
		checkLen(dom.Goal, domPath+".goal", MaxTextLen, res)
		checkGoal(dom, origin, domPath, res)

		if len(dom.Items) == 0 {
			res.errorf(IssueRequired, domPath+".items", "分野 %q に詳細項目がありません", dom.Key)
		}
		if len(dom.Items) > MaxItemsPerDomain {
			res.errorf(IssueTooMany, domPath+".items",
				"1分野あたりの詳細項目は %d 件までです（%d 件あります）", MaxItemsPerDomain, len(dom.Items))
		}

		for ii, it := range dom.Items {
			itemPath := fmt.Sprintf("%s.items[%d]", domPath, ii)

			if checkKey(it.Key, itemPath+".key", "詳細項目", res) {
				if first, dup := itemKeys[it.Key]; dup {
					res.errorf(IssueDuplicateKey, itemPath+".key",
						"詳細項目の key %q が %s と重複しています", it.Key, first)
				} else {
					itemKeys[it.Key] = itemPath
					itemDomains[it.Key] = dom.Key
				}
			}

			if strings.TrimSpace(it.Name) == "" {
				res.errorf(IssueRequired, itemPath+".name", "詳細項目の名前は必須です")
			}
			checkLen(it.Name, itemPath+".name", MaxNameLen, res)
			checkLen(it.Description, itemPath+".description", MaxTextLen, res)
			checkLen(it.Outcome, itemPath+".outcome", MaxTextLen, res)
			checkLen(it.VerifyBy, itemPath+".verifyBy", MaxTextLen, res)
			for k, ref := range it.ModuleRefs {
				checkLen(ref, fmt.Sprintf("%s.moduleRefs[%d]", itemPath, k), MaxModuleRefLen, res)
			}
			checkOutcome(it, origin, itemPath, res)
		}
	}

	return itemDomains
}

// checkKey は key の形式を見る。形式が正しければ true を返す（重複判定に進めてよい合図）。
func checkKey(key, path, what string, res *Result) bool {
	if key == "" {
		res.errorf(IssueRequired, path, "%sの key は必須です", what)
		return false
	}
	if !keyPattern.MatchString(key) {
		res.errorf(IssueInvalidFormat, path,
			"%sの key %q は使えません。英小文字・数字・ハイフンで、先頭は英数字、%d 文字以内にしてください",
			what, key, MaxKeyLen)
		return false
	}
	return true
}

// checkLen は1フィールドの文字数が上限内かを見る。空は「未設定」なので通す。
//
// バイト数（len）ではなく文字数（utf8.RuneCountInString）で数える。
// 日本語は1文字3バイトなので、バイト数で見ると英語との間で不公平になる。
func checkLen(s, path string, maxRunes int, res *Result) {
	if n := utf8.RuneCountInString(s); n > maxRunes {
		res.errorf(IssueTooLong, path, "%d 文字までです（%d 文字あります）", maxRunes, n)
	}
}

// checkOutcome は到達状態を origin 別に見る（SPEC.md §2.1）。
//
// **初学者は自分のロードマップの到達状態を自分では書けない。**
// 「これを学ぶと何ができるようになるか」が分かっているなら、そもそも先が見えていないという
// 課題が存在しない。だから manual では求めず、external では警告に留め、
// アプリ同梱の builtin だけ必須にする。
func checkOutcome(it ItemDef, origin Origin, path string, res *Result) {
	if strings.TrimSpace(it.Outcome) != "" {
		return
	}
	switch origin {
	case OriginBuiltin:
		res.errorf(IssueMissingOutcome, path+".outcome",
			"アプリ同梱（builtin）のロードマップでは到達状態（outcome）が必須です")
	case OriginExternal:
		res.warnf(IssueMissingOutcome, path+".outcome",
			"詳細項目 %q に到達状態（outcome）がありません。AI の下書きを作れます", it.Key)
	}
}

// checkGoal は分野の到達状態を origin 別に見る（SPEC.md §2.1）。
func checkGoal(dom DomainDef, origin Origin, path string, res *Result) {
	if strings.TrimSpace(dom.Goal) != "" {
		return
	}
	switch origin {
	case OriginBuiltin:
		res.errorf(IssueMissingOutcome, path+".goal",
			"アプリ同梱（builtin）のロードマップでは分野の到達状態（goal）が必須です")
	case OriginExternal:
		res.warnf(IssueMissingOutcome, path+".goal",
			"分野 %q に到達状態（goal）がありません。AI の下書きを作れます", dom.Key)
	}
}

// ---------------------------------------------------------------------------
// 依存関係
// ---------------------------------------------------------------------------

// validateDependencies は dependsOn が指す先と循環参照を見る。
//
// 循環を通してしまうと、学習パス（依存順のトポロジカルソート）が並び順を決められなくなる。
// 画面が壊れてから気づくのではなく、インポートの時点で弾く。
func validateDependencies(doc *Document, itemDomains map[string]string, res *Result) {
	// 依存グラフ。key の形式が不正な項目は itemDomains に入っていないので自然に除外される
	graph := make(map[string][]string, len(itemDomains))
	// 出現順。検査結果を実行のたびに同じ順序にするため、map の反復には頼らない
	var order []string

	for di, dom := range doc.Domains {
		for ii, it := range dom.Items {
			if _, ok := itemDomains[it.Key]; !ok {
				continue
			}
			path := fmt.Sprintf("domains[%d].items[%d].dependsOn", di, ii)

			var deps []string
			for k, dep := range it.DependsOn {
				depPath := fmt.Sprintf("%s[%d]", path, k)
				switch {
				case dep == it.Key:
					res.errorf(IssueSelfDependency, depPath,
						"詳細項目 %q が自分自身に依存しています", it.Key)
				case itemDomains[dep] == "":
					res.errorf(IssueUnknownDependency, depPath,
						"詳細項目 %q が依存している %q は、このロードマップに存在しません", it.Key, dep)
				default:
					deps = append(deps, dep)
				}
			}
			graph[it.Key] = deps
			order = append(order, it.Key)
		}
	}

	for _, cycle := range findCycles(graph, order) {
		res.errorf(IssueCircularDependency, "",
			"依存関係が循環しています: %s", strings.Join(cycle, " -> "))
	}
}

// findCycles は依存グラフの循環を探す。
//
// 深さ優先探索で、探索中の道に戻ってきたら循環とみなす典型的なやり方。
// 同じ循環を何度も報告しないよう、一度報告した経路上の項目には印を付ける。
func findCycles(graph map[string][]string, order []string) [][]string {
	const (
		white = 0 // 未訪問
		gray  = 1 // 探索中（いま辿っている道の上にある）
		black = 2 // 探索済み（ここから先に循環は無いと分かっている）
	)

	color := make(map[string]int, len(graph))
	var stack []string
	var cycles [][]string
	// 同じ循環を重複して報告しないための集合
	reported := make(map[string]struct{})

	var visit func(node string)
	visit = func(node string) {
		color[node] = gray
		stack = append(stack, node)

		for _, dep := range graph[node] {
			switch color[dep] {
			case white:
				visit(dep)
			case gray:
				// いま辿っている道の上に戻ってきた ＝ 循環
				if cycle := cycleFrom(stack, dep); cycle != nil {
					if _, dup := reported[cycleID(cycle)]; !dup {
						reported[cycleID(cycle)] = struct{}{}
						cycles = append(cycles, cycle)
					}
				}
			}
		}

		stack = stack[:len(stack)-1]
		color[node] = black
	}

	for _, node := range order {
		if color[node] == white {
			visit(node)
		}
	}
	return cycles
}

// cycleFrom は探索中の経路から、循環している部分だけを切り出す。
// 末尾に始点を再掲して "a -> b -> a" の形にする。
func cycleFrom(stack []string, start string) []string {
	for i, n := range stack {
		if n == start {
			cycle := append([]string(nil), stack[i:]...)
			return append(cycle, start)
		}
	}
	return nil
}

// cycleID は循環の同一性を判定するための鍵。
// 経路の始点が違っても同じ循環なので、含まれる項目の集合で見る。
func cycleID(cycle []string) string {
	keys := append([]string(nil), cycle[:len(cycle)-1]...)
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

// joinInts は数値の並びを "1, 2" の形にする。
func joinInts(ns []int) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = fmt.Sprint(n)
	}
	return strings.Join(parts, ", ")
}
