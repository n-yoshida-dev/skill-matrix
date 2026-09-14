// Package roadmap はロードマップ・マスタ JSON（SPEC.md §2）の受け皿と検査を持つ。
//
// このパッケージも internal/domain と同じく DB・HTTP・LLM クライアントを import しない。
// 「バイト列を受け取り、これを DB に入れてよいかを判定する」だけの純粋な処理に閉じる。
//
// インポートされる JSON は、利用者が手で書いたものか外部から持ってきたもので、
// **壊れている前提で扱う。** ここで弾かなければ、たとえば存在しない項目に依存する項目が
// そのまま DB に入り、学習パス（依存関係のトポロジカルソート）が破綻する。
//
// 仕様の正本は SPEC.md §2 と §2.1。
package roadmap

// SchemaVersion はこのパッケージが読める JSON のバージョン。
// 将来フォーマットを変えたときに、古いアプリが新しい JSON を黙って取りこぼさないための番号。
const SchemaVersion = 1

// 上限（SPEC.md §2）。
// 画面が読めなくなる規模を防ぐためと、LLM のプロンプトに載る量を抑えるための両方の意味がある。
const (
	MaxDomains        = 50  // 1ロードマップあたりの分野数
	MaxItemsPerDomain = 100 // 1分野あたりの詳細項目数
	MaxItems          = 500 // 1ロードマップあたりの詳細項目の総数
	MaxKeyLen         = 64  // key の最大長。^[a-z0-9][a-z0-9-]{0,63}$ に対応する
)

// 1フィールドの文字数の上限（SPEC.md §2）。バイト数ではなく文字数（rune）で数える。
// 日本語は1文字が3バイトなので、バイト数で制限すると英語の3分の1しか書けなくなる。
//
// criteria / outcome / goal / verifyBy は LLM のプロンプトにそのまま載る。
// 上限が無いと、巨大な文字列を1つ入れるだけで判定1回あたりの課金が跳ね上がる。
const (
	MaxNameLen   = 200  // name（ロードマップ・分野・項目・レベル）
	MaxTextLen   = 2000 // description / outcome / goal / criteria / verifyBy
	MaxSourceLen = 2048 // source（URL。一般的なブラウザが扱える上限に合わせた）
)

// Origin はロードマップの出所（SPEC.md §2）。
//
// 出典（source / checkedAt）と到達状態（outcome / goal）をどこまで求めるかが、これで変わる。
// 自分の頭で組んだロードマップにまで出典を要求すると、作成のハードルが無意味に上がるため。
type Origin string

const (
	// OriginManual は利用者が自分で書いたもの。出典も到達状態も任意。
	OriginManual Origin = "manual"
	// OriginExternal は外部の記事・カリキュラム等を元にしたもの。出典は必須。
	OriginExternal Origin = "external"
	// OriginBuiltin はアプリ同梱のサンプル。用意する側が責任を持つので到達状態まで必須。
	OriginBuiltin Origin = "builtin"
)

// Valid は既知の origin かを返す。
func (o Origin) Valid() bool {
	switch o {
	case OriginManual, OriginExternal, OriginBuiltin:
		return true
	default:
		return false
	}
}

// LevelDef はレベル1つの定義。
//
// Criteria は**そのまま LLM のプロンプトに埋め込まれる**判定基準の正本（SPEC.md §2）。
// 判定の基準をコード側とプロンプト側の2か所に書くと必ずずれるため、ここ1か所に集める。
type LevelDef struct {
	Level    int    `json:"level"`
	Name     string `json:"name"`
	Criteria string `json:"criteria"`
}

// ItemDef は詳細項目1つ。
type ItemDef struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Outcome は「身につくと何ができるようになるか」。origin により必須かどうかが変わる（SPEC.md §2.1）。
	Outcome string `json:"outcome,omitempty"`
	// DependsOn は先に着手すべき項目の key。同一ロードマップ内の項目しか指せない。
	DependsOn []string `json:"dependsOn,omitempty"`
	// VerifyBy は「次に何をすればレベルが上がるか」。
	VerifyBy string `json:"verifyBy,omitempty"`
}

// DomainDef は分野1つ。
type DomainDef struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	// Goal は「この分野を修めると何ができるようになるか」（SPEC.md §2.1）。
	Goal  string    `json:"goal,omitempty"`
	Items []ItemDef `json:"items"`
}

// Document はマスタ JSON 全体。インポート・エクスポートの正本フォーマット。
type Document struct {
	SchemaVersion int    `json:"schemaVersion"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	// Origin は省略時 manual とみなす（SPEC.md §2）。
	Origin Origin `json:"origin,omitempty"`
	// Source は出典 URL。origin='external' なら必須。
	Source string `json:"source,omitempty"`
	// CheckedAt は出典の確認日（YYYY-MM-DD）。origin='external' / 'builtin' なら必須。
	CheckedAt string      `json:"checkedAt,omitempty"`
	Levels    []LevelDef  `json:"levels"`
	Domains   []DomainDef `json:"domains"`
}

// EffectiveOrigin は origin を省略したときの既定（manual）を補って返す。
func (d *Document) EffectiveOrigin() Origin {
	if d.Origin == "" {
		return OriginManual
	}
	return d.Origin
}

// TotalItems はロードマップ全体の詳細項目数を返す。
func (d *Document) TotalItems() int {
	n := 0
	for _, dom := range d.Domains {
		n += len(dom.Items)
	}
	return n
}
