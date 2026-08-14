package roadmap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// このテストは「壊れた JSON をどう扱うか」を具体例で固定するもの。
// 検査ルールの意図を読み取りたいときは SPEC.md §2 より先にここを読むとよい。

// testdataPath は backend/testdata/ のファイルを指す。
// 共有のダミーデータは SPEC.md §8.1 に従い backend/testdata/ に置いている。
func testdataPath(name string) string {
	return filepath.Join("..", "..", "testdata", name)
}

// validDoc は検査を通る最小限のロードマップ。各テストで一部だけ壊して使う。
func validDoc() *Document {
	return &Document{
		SchemaVersion: SchemaVersion,
		Name:          "テスト用",
		Origin:        OriginManual,
		Levels:        validLevels(),
		Domains: []DomainDef{{
			Key:  "go",
			Name: "Go",
			Items: []ItemDef{
				{Key: "go-01", Name: "基本構文"},
				{Key: "go-02", Name: "インターフェース", DependsOn: []string{"go-01"}},
			},
		}},
	}
}

func validLevels() []LevelDef {
	return []LevelDef{
		{Level: 1, Name: "基礎理解を確認", Criteria: "確認質問に答えられた"},
		{Level: 2, Name: "自分で説明可能", Criteria: "自分の言葉で説明できる"},
		{Level: 3, Name: "ガイド付き実装", Criteria: "助けがあれば実装できる"},
		{Level: 4, Name: "自力実装・レビュー", Criteria: "独力で実装できる"},
		{Level: 5, Name: "別文脈へ応用", Criteria: "別の場面で使える"},
	}
}

// issueCodes は結果から、指定した重さの問題の種類だけを取り出す。
func issueCodes(res Result, sev Severity) []IssueCode {
	var out []IssueCode
	for _, is := range res.Issues {
		if is.Severity == sev {
			out = append(out, is.Code)
		}
	}
	return out
}

// hasIssue は指定した種類・場所の問題があるかを返す。path が空なら場所は問わない。
func hasIssue(res Result, code IssueCode, path string) bool {
	for _, is := range res.Issues {
		if is.Code == code && (path == "" || is.Path == path) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 正常系
// ---------------------------------------------------------------------------

func TestValidate_ValidDocumentHasNoIssues(t *testing.T) {
	res := Validate(validDoc())
	if !res.OK() {
		t.Fatalf("エラーが出ないはずだが出た: %+v", res.Errors())
	}
	if len(res.Warnings()) != 0 {
		t.Errorf("警告が出ないはずだが出た: %+v", res.Warnings())
	}
}

func TestValidate_NilDocument(t *testing.T) {
	// Parse に失敗した戻り値をそのまま渡されても落ちないこと
	res := Validate(nil)
	if res.OK() {
		t.Error("nil はエラーにするべき")
	}
}

func TestParseAndValidate_SampleFile(t *testing.T) {
	data, err := os.ReadFile(testdataPath("roadmap-sample.json"))
	if err != nil {
		t.Fatalf("サンプルを読めない: %v", err)
	}

	doc, res := ParseAndValidate(data)
	if doc == nil {
		t.Fatalf("読み込みに失敗した: %+v", res.Errors())
	}
	if !res.OK() {
		t.Fatalf("サンプルはエラーなしで通るべき: %+v", res.Errors())
	}
	if len(res.Warnings()) != 0 {
		t.Errorf("サンプルは警告も出ないようにしてある: %+v", res.Warnings())
	}
	if doc.TotalItems() != 11 {
		t.Errorf("項目数 = %d, 期待 11", doc.TotalItems())
	}
}

// ---------------------------------------------------------------------------
// 読み込み（Parse）
// ---------------------------------------------------------------------------

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode IssueCode
	}{
		{
			name:     "未知のフィールドは弾く（dependsOn の打ち間違い）",
			input:    `{"schemaVersion":1,"name":"x","levels":[],"domains":[{"key":"go","name":"Go","items":[{"key":"go-01","name":"基本","dependOn":["go-00"]}]}]}`,
			wantCode: IssueUnknownField,
		},
		{
			name:     "JSON として壊れている",
			input:    `{"schemaVersion":1,`,
			wantCode: IssueInvalidJSON,
		},
		{
			name:     "型が違う",
			input:    `{"schemaVersion":"いち","name":"x"}`,
			wantCode: IssueInvalidJSON,
		},
		{
			name:     "オブジェクトの後ろに余分なデータがある",
			input:    `{"schemaVersion":1,"name":"x"} {"schemaVersion":1}`,
			wantCode: IssueInvalidJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, res := Parse([]byte(tt.input))
			if doc != nil {
				t.Fatal("読み込みに失敗したときは Document を返さない")
			}
			if !hasIssue(res, tt.wantCode, "") {
				t.Errorf("%s が出るべき: %+v", tt.wantCode, res.Issues)
			}
		})
	}
}

func TestParse_UnknownFieldMessageNamesTheField(t *testing.T) {
	// 「どこが悪いか」が分からないと直せないので、綴りを間違えたキー名を必ず含める
	_, res := Parse([]byte(`{"schemaVersion":1,"nmae":"x"}`))
	if len(res.Errors()) == 0 {
		t.Fatal("エラーが出るべき")
	}
	if !strings.Contains(res.Errors()[0].Message, `"nmae"`) {
		t.Errorf("メッセージに間違えたキー名が含まれるべき: %q", res.Errors()[0].Message)
	}
}

// ---------------------------------------------------------------------------
// メタ情報と出典
// ---------------------------------------------------------------------------

func TestValidate_Meta(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*Document)
		wantCode IssueCode
		wantPath string
	}{
		{
			name:     "対応していない schemaVersion",
			mutate:   func(d *Document) { d.SchemaVersion = 99 },
			wantCode: IssueUnsupportedSchemaVersion, wantPath: "schemaVersion",
		},
		{
			name:     "名前が空",
			mutate:   func(d *Document) { d.Name = "   " },
			wantCode: IssueRequired, wantPath: "name",
		},
		{
			name:     "既知でない origin",
			mutate:   func(d *Document) { d.Origin = "imported" },
			wantCode: IssueInvalidOrigin, wantPath: "origin",
		},
		{
			name:     "確認日の形式が違う",
			mutate:   func(d *Document) { d.CheckedAt = "2026/08/14" },
			wantCode: IssueInvalidFormat, wantPath: "checkedAt",
		},
		{
			name: "external なのに出典が無い",
			mutate: func(d *Document) {
				d.Origin = OriginExternal
				d.CheckedAt = "2026-08-14"
			},
			wantCode: IssueRequired, wantPath: "source",
		},
		{
			name: "external なのに確認日が無い",
			mutate: func(d *Document) {
				d.Origin = OriginExternal
				d.Source = "https://example.com/roadmap"
			},
			wantCode: IssueRequired, wantPath: "checkedAt",
		},
		{
			name: "builtin は確認日が必須",
			mutate: func(d *Document) {
				d.Origin = OriginBuiltin
				withOutcomes(d)
			},
			wantCode: IssueRequired, wantPath: "checkedAt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := validDoc()
			tt.mutate(doc)
			res := Validate(doc)
			if !hasIssue(res, tt.wantCode, tt.wantPath) {
				t.Errorf("%s（%s）が出るべき: %+v", tt.wantCode, tt.wantPath, res.Issues)
			}
		})
	}
}

func TestValidate_ManualNeedsNoSource(t *testing.T) {
	// 自分で組んだロードマップに出典を要求すると、作成のハードルが無意味に上がる（SPEC.md §2）
	doc := validDoc()
	doc.Origin = ""
	doc.Source = ""
	doc.CheckedAt = ""

	res := Validate(doc)
	if !res.OK() {
		t.Errorf("origin 省略（= manual）は出典なしで通るべき: %+v", res.Errors())
	}
}

func TestValidate_NonURLSourceIsOnlyAWarning(t *testing.T) {
	// 書籍名を出典に書きたい場合もあるので、止めるほどではない
	doc := validDoc()
	doc.Origin = OriginExternal
	doc.Source = "書籍『実用 Go 言語』"
	doc.CheckedAt = "2026-08-14"
	withOutcomes(doc)

	res := Validate(doc)
	if !res.OK() {
		t.Errorf("URL でない出典はインポートを止めない: %+v", res.Errors())
	}
	if !hasIssue(res, IssueInvalidFormat, "source") {
		t.Errorf("警告は出すべき: %+v", res.Issues)
	}
}

// ---------------------------------------------------------------------------
// レベル定義
// ---------------------------------------------------------------------------

func TestValidate_Levels(t *testing.T) {
	tests := []struct {
		name     string
		levels   []LevelDef
		wantCode IssueCode
	}{
		{
			name:     "levels が空",
			levels:   nil,
			wantCode: IssueRequired,
		},
		{
			name:     "レベル5が欠けている",
			levels:   validLevels()[:4],
			wantCode: IssueInvalidLevels,
		},
		{
			name: "レベルが重複している",
			levels: func() []LevelDef {
				l := validLevels()
				l[4].Level = 4
				return l
			}(),
			wantCode: IssueInvalidLevels,
		},
		{
			name: "レベルが範囲外",
			levels: func() []LevelDef {
				l := validLevels()
				l[4].Level = 9
				return l
			}(),
			wantCode: IssueInvalidLevels,
		},
		{
			name: "判定基準が空",
			levels: func() []LevelDef {
				l := validLevels()
				l[2].Criteria = ""
				return l
			}(),
			wantCode: IssueRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := validDoc()
			doc.Levels = tt.levels
			res := Validate(doc)
			if !hasIssue(res, tt.wantCode, "") {
				t.Errorf("%s が出るべき: %+v", tt.wantCode, res.Issues)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// key・上限・構造
// ---------------------------------------------------------------------------

func TestValidate_Keys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		wantCode IssueCode
	}{
		{name: "英小文字・数字・ハイフンは通る", key: "go-01"},
		{name: "空", key: "", wantCode: IssueRequired},
		{name: "大文字を含む", key: "Go-01", wantCode: IssueInvalidFormat},
		{name: "先頭がハイフン", key: "-go", wantCode: IssueInvalidFormat},
		{name: "空白を含む", key: "go 01", wantCode: IssueInvalidFormat},
		{name: "アンダースコアを含む", key: "go_01", wantCode: IssueInvalidFormat},
		{name: "長すぎる", key: strings.Repeat("a", MaxKeyLen+1), wantCode: IssueInvalidFormat},
		{name: "上限ちょうど", key: strings.Repeat("a", MaxKeyLen)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := validDoc()
			doc.Domains[0].Items[0].Key = tt.key
			// 依存元の key が変わるので、参照側も合わせる（依存の検査に巻き込まれないように）
			doc.Domains[0].Items[1].DependsOn = nil

			res := Validate(doc)
			got := hasIssue(res, IssueRequired, "domains[0].items[0].key") ||
				hasIssue(res, IssueInvalidFormat, "domains[0].items[0].key")

			if tt.wantCode == "" {
				if got {
					t.Errorf("key %q は通るべき: %+v", tt.key, res.Issues)
				}
				return
			}
			if !hasIssue(res, tt.wantCode, "domains[0].items[0].key") {
				t.Errorf("%s が出るべき: %+v", tt.wantCode, res.Issues)
			}
		})
	}
}

func TestValidate_DuplicateKeys(t *testing.T) {
	t.Run("項目の key は分野をまたいで一意", func(t *testing.T) {
		doc := validDoc()
		doc.Domains = append(doc.Domains, DomainDef{
			Key:   "react",
			Name:  "React",
			Items: []ItemDef{{Key: "go-01", Name: "別分野に同じ key"}},
		})

		res := Validate(doc)
		if !hasIssue(res, IssueDuplicateKey, "domains[1].items[0].key") {
			t.Errorf("重複が検出されるべき: %+v", res.Issues)
		}
	})

	t.Run("分野の key の重複", func(t *testing.T) {
		doc := validDoc()
		doc.Domains = append(doc.Domains, DomainDef{
			Key:   "go",
			Name:  "重複した分野",
			Items: []ItemDef{{Key: "go-99", Name: "項目"}},
		})

		res := Validate(doc)
		if !hasIssue(res, IssueDuplicateKey, "domains[1].key") {
			t.Errorf("重複が検出されるべき: %+v", res.Issues)
		}
	})

	t.Run("分野と項目で key が同じでも構わない", func(t *testing.T) {
		// DB も domains / items で別々に一意（migrations 000001）
		doc := validDoc()
		doc.Domains[0].Items[0].Key = "go"
		doc.Domains[0].Items[1].DependsOn = []string{"go"}

		res := Validate(doc)
		if !res.OK() {
			t.Errorf("名前空間が違うので通るべき: %+v", res.Errors())
		}
	})
}

func TestValidate_Limits(t *testing.T) {
	t.Run("分野の上限", func(t *testing.T) {
		doc := validDoc()
		doc.Domains = makeDomains(MaxDomains + 1)

		if !hasIssue(Validate(doc), IssueTooMany, "domains") {
			t.Error("分野数の上限超えが検出されるべき")
		}
	})

	t.Run("1分野あたりの項目の上限", func(t *testing.T) {
		doc := validDoc()
		doc.Domains[0].Items = makeItems("go", MaxItemsPerDomain+1)

		if !hasIssue(Validate(doc), IssueTooMany, "domains[0].items") {
			t.Error("1分野あたりの項目数の上限超えが検出されるべき")
		}
	})

	t.Run("ロードマップ全体の項目の上限", func(t *testing.T) {
		// 1分野あたりの上限には触れずに、全体の上限だけを超えさせる
		doc := validDoc()
		doc.Domains = makeDomains(MaxItems/MaxItemsPerDomain + 1)
		for i := range doc.Domains {
			doc.Domains[i].Items = makeItems(doc.Domains[i].Key, MaxItemsPerDomain)
		}

		res := Validate(doc)
		if doc.TotalItems() <= MaxItems {
			t.Fatalf("テストの前提が壊れている: 項目数 %d", doc.TotalItems())
		}
		if !hasIssue(res, IssueTooMany, "domains") {
			t.Error("全体の項目数の上限超えが検出されるべき")
		}
	})

	t.Run("項目が空の分野", func(t *testing.T) {
		doc := validDoc()
		doc.Domains[0].Items = nil

		if !hasIssue(Validate(doc), IssueRequired, "domains[0].items") {
			t.Error("空の分野が検出されるべき")
		}
	})
}

// ---------------------------------------------------------------------------
// 依存関係
// ---------------------------------------------------------------------------

func TestValidate_Dependencies(t *testing.T) {
	t.Run("実在しない項目への依存", func(t *testing.T) {
		doc := validDoc()
		doc.Domains[0].Items[1].DependsOn = []string{"go-99"}

		if !hasIssue(Validate(doc), IssueUnknownDependency, "domains[0].items[1].dependsOn[0]") {
			t.Error("実在しない依存が検出されるべき")
		}
	})

	t.Run("自分自身への依存", func(t *testing.T) {
		doc := validDoc()
		doc.Domains[0].Items[1].DependsOn = []string{"go-02"}

		if !hasIssue(Validate(doc), IssueSelfDependency, "domains[0].items[1].dependsOn[0]") {
			t.Error("自己依存が検出されるべき")
		}
	})

	t.Run("分野をまたいだ依存は許す", func(t *testing.T) {
		doc := validDoc()
		doc.Domains = append(doc.Domains, DomainDef{
			Key:   "react",
			Name:  "React",
			Items: []ItemDef{{Key: "react-01", Name: "基礎", DependsOn: []string{"go-01"}}},
		})

		if res := Validate(doc); !res.OK() {
			t.Errorf("同一ロードマップ内なら分野をまたいでよい: %+v", res.Errors())
		}
	})

	t.Run("2項目の相互依存", func(t *testing.T) {
		doc := validDoc()
		doc.Domains[0].Items[0].DependsOn = []string{"go-02"}

		res := Validate(doc)
		if !hasIssue(res, IssueCircularDependency, "") {
			t.Fatalf("循環が検出されるべき: %+v", res.Issues)
		}
	})

	t.Run("3項目の循環はメッセージに経路が出る", func(t *testing.T) {
		doc := validDoc()
		doc.Domains[0].Items = []ItemDef{
			{Key: "a", Name: "A", DependsOn: []string{"c"}},
			{Key: "b", Name: "B", DependsOn: []string{"a"}},
			{Key: "c", Name: "C", DependsOn: []string{"b"}},
		}

		res := Validate(doc)
		var msg string
		for _, is := range res.Issues {
			if is.Code == IssueCircularDependency {
				msg = is.Message
			}
		}
		if msg == "" {
			t.Fatalf("循環が検出されるべき: %+v", res.Issues)
		}
		for _, k := range []string{"a", "b", "c"} {
			if !strings.Contains(msg, k) {
				t.Errorf("メッセージ %q に %q が含まれるべき", msg, k)
			}
		}
	})

	t.Run("長い直線の依存は循環ではない", func(t *testing.T) {
		doc := validDoc()
		doc.Domains[0].Items = makeChain("go", 30)

		if res := Validate(doc); !res.OK() {
			t.Errorf("一直線の依存は通るべき: %+v", res.Errors())
		}
	})

	t.Run("菱形の依存（合流）は循環ではない", func(t *testing.T) {
		doc := validDoc()
		doc.Domains[0].Items = []ItemDef{
			{Key: "a", Name: "A"},
			{Key: "b", Name: "B", DependsOn: []string{"a"}},
			{Key: "c", Name: "C", DependsOn: []string{"a"}},
			{Key: "d", Name: "D", DependsOn: []string{"b", "c"}},
		}

		if res := Validate(doc); !res.OK() {
			t.Errorf("合流は循環ではない: %+v", res.Errors())
		}
	})

	t.Run("同じ循環を重複して報告しない", func(t *testing.T) {
		doc := validDoc()
		doc.Domains[0].Items = []ItemDef{
			{Key: "a", Name: "A", DependsOn: []string{"b"}},
			{Key: "b", Name: "B", DependsOn: []string{"a"}},
			// 循環の外から両方を指す。探索の入口が増えても報告は1件であってほしい
			{Key: "c", Name: "C", DependsOn: []string{"a", "b"}},
		}

		res := Validate(doc)
		n := 0
		for _, is := range res.Issues {
			if is.Code == IssueCircularDependency {
				n++
			}
		}
		if n != 1 {
			t.Errorf("循環の報告件数 = %d, 期待 1: %+v", n, res.Issues)
		}
	})
}

// ---------------------------------------------------------------------------
// 到達状態（origin 別）
// ---------------------------------------------------------------------------

func TestValidate_Outcome(t *testing.T) {
	t.Run("manual は outcome が無くても何も言わない", func(t *testing.T) {
		doc := validDoc()
		doc.Origin = OriginManual

		res := Validate(doc)
		if len(res.Issues) != 0 {
			t.Errorf("manual では到達状態を求めない: %+v", res.Issues)
		}
	})

	t.Run("external は警告に留めてインポートは通す", func(t *testing.T) {
		doc := validDoc()
		doc.Origin = OriginExternal
		doc.Source = "https://example.com/roadmap"
		doc.CheckedAt = "2026-08-14"

		res := Validate(doc)
		if !res.OK() {
			t.Errorf("external の outcome 欠落でインポートを止めない: %+v", res.Errors())
		}
		if !hasIssue(res, IssueMissingOutcome, "domains[0].items[0].outcome") {
			t.Errorf("項目の警告が出るべき: %+v", res.Issues)
		}
		if !hasIssue(res, IssueMissingOutcome, "domains[0].goal") {
			t.Errorf("分野の警告が出るべき: %+v", res.Issues)
		}
	})

	t.Run("builtin は必須", func(t *testing.T) {
		doc := validDoc()
		doc.Origin = OriginBuiltin
		doc.CheckedAt = "2026-08-14"

		res := Validate(doc)
		if res.OK() {
			t.Error("builtin では到達状態の欠落をエラーにする")
		}
		if !slices.Contains(issueCodes(res, SeverityError), IssueMissingOutcome) {
			t.Errorf("missing_outcome がエラーとして出るべき: %+v", res.Issues)
		}
	})
}

// ---------------------------------------------------------------------------
// 問題をまとめて返すこと
// ---------------------------------------------------------------------------

func TestParseAndValidate_BrokenFileReportsEverythingAtOnce(t *testing.T) {
	// 1つ直すたびに投稿し直して次の1つが出る、という直し方をさせないための確認
	data, err := os.ReadFile(testdataPath("roadmap-broken.json"))
	if err != nil {
		t.Fatalf("テストデータを読めない: %v", err)
	}

	_, res := ParseAndValidate(data)
	if res.OK() {
		t.Fatal("壊れたロードマップが通ってしまった")
	}

	want := []IssueCode{
		IssueUnsupportedSchemaVersion, // schemaVersion が 2
		IssueRequired,                 // name が空 / source と checkedAt が無い
		IssueInvalidLevels,            // レベル5が無い、レベル9が範囲外
		IssueInvalidFormat,            // 分野の key "Go Basics"
		IssueDuplicateKey,             // x-01 の重複
		IssueUnknownDependency,        // nope-99
		IssueSelfDependency,           // c-04
		IssueCircularDependency,       // c-01 -> c-03 -> c-02 -> c-01
	}
	got := issueCodes(res, SeverityError)
	for _, code := range want {
		if !slices.Contains(got, code) {
			t.Errorf("%s が出るべき。出た問題: %+v", code, res.Issues)
		}
	}
}

func TestResult_JSONShapeIsStable(t *testing.T) {
	// フロントはこの形をそのまま受け取る。キー名が変わると画面が壊れる
	res := Result{Issues: []Issue{{
		Severity: SeverityWarning,
		Code:     IssueMissingOutcome,
		Path:     "domains[0].items[1].outcome",
		Message:  "到達状態がありません",
	}}}

	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("JSON 化に失敗: %v", err)
	}
	want := `{"issues":[{"severity":"warning","code":"missing_outcome","path":"domains[0].items[1].outcome","message":"到達状態がありません"}]}`
	if string(b) != want {
		t.Errorf("JSON = %s\n期待 = %s", b, want)
	}
}

// ---------------------------------------------------------------------------
// テスト用の組み立て
// ---------------------------------------------------------------------------

// withOutcomes はすべての項目と分野に到達状態を埋める。
// builtin の検査で「到達状態以外」を確かめたいときに使う。
func withOutcomes(d *Document) {
	for i := range d.Domains {
		d.Domains[i].Goal = "この分野の到達状態"
		for j := range d.Domains[i].Items {
			d.Domains[i].Items[j].Outcome = "この項目の到達状態"
		}
	}
}

// makeDomains は n 個の分野を作る。各分野は項目を1つ持つ。
func makeDomains(n int) []DomainDef {
	out := make([]DomainDef, n)
	for i := range out {
		key := "d" + itoa(i)
		out[i] = DomainDef{
			Key:   key,
			Name:  "分野" + itoa(i),
			Items: makeItems(key, 1),
		}
	}
	return out
}

// makeItems は分野 domainKey に属する n 個の項目を作る（依存関係なし）。
func makeItems(domainKey string, n int) []ItemDef {
	out := make([]ItemDef, n)
	for i := range out {
		out[i] = ItemDef{Key: domainKey + "-" + itoa(i), Name: "項目" + itoa(i)}
	}
	return out
}

// makeChain は一直線に依存する n 個の項目を作る。
func makeChain(prefix string, n int) []ItemDef {
	out := make([]ItemDef, n)
	for i := range out {
		out[i] = ItemDef{Key: prefix + "-" + itoa(i), Name: "項目" + itoa(i)}
		if i > 0 {
			out[i].DependsOn = []string{prefix + "-" + itoa(i-1)}
		}
	}
	return out
}

func itoa(n int) string { return strconv.Itoa(n) }
