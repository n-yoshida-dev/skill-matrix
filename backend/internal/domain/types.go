// Package domain は理解度モデルの中核を担う。
//
// このパッケージは DB・HTTP・LLM クライアントを import しない。
// LLM の判定結果は「入力」として受け取るだけで、集計・検証・優先度計算だけをここに置く。
// そうすることで、外部サービスを立てずに単体テストだけで挙動を保証できる。
//
// 仕様の正本は SPEC.md。人間向けの解説は docs/spec-guide.md にある。
package domain

import "time"

// ItemKey は詳細項目の識別子。ロードマップ内で一意（例: "go-01"）。
type ItemKey string

// DomainKey は分野の識別子。ロードマップ内で一意（例: "go"）。
type DomainKey string

// ---------------------------------------------------------------------------
// レベルと段階前の状態（SPEC.md §1）
// ---------------------------------------------------------------------------

// Level は理解度の段階。0（未着手）から 5（別文脈へ応用）まで。
type Level int

const (
	LevelNone            Level = 0 // 未着手
	LevelBasicConfirmed  Level = 1 // 基礎理解を確認
	LevelCanExplain      Level = 2 // 自分で説明可能
	LevelGuidedImpl      Level = 3 // ガイド付き実装
	LevelIndependentImpl Level = 4 // 自力実装・レビュー
	LevelCrossContext    Level = 5 // 別文脈へ応用
)

// MaxLevel は到達しうる最大のレベル。
const MaxLevel = LevelCrossContext

// LevelCount はレベルの取りうる値の数（0〜5 の 6 通り）。集計用の配列長に使う。
const LevelCount = int(MaxLevel) + 1

// Valid はレベルが 0〜5 の範囲内かを返す（検証ルール V2）。
func (l Level) Valid() bool {
	return l >= LevelNone && l <= MaxLevel
}

// PreState はレベルとは別軸の「段階前の状態」。
//
// レベルに混ぜて数値化しない。「説明を受けた」と「自分で説明できた」は
// 連続した量ではなく質の違うできごとなので、別の軸として持つ。
type PreState string

const (
	PreStateNone          PreState = "none"           // 特記なし
	PreStateLearning      PreState = "learning"       // 未確認（学習中）
	PreStateExplainedOnly PreState = "explained_only" // 説明済み・理解未確認
	PreStateSelfReported  PreState = "self_reported"  // 実務経験あり・横断評価未実施
)

// ---------------------------------------------------------------------------
// 根拠の種類と昇格の上限（SPEC.md §4.4）
// ---------------------------------------------------------------------------

// EvidenceType は判定の根拠の種類。
type EvidenceType string

const (
	EvidenceDrill                 EvidenceType = "drill"                  // ドリル・確認質問に答えた
	EvidenceSelfExplanation       EvidenceType = "self_explanation"       // 自分の言葉で説明した
	EvidenceImplementation        EvidenceType = "implementation"         // 実装した
	EvidenceUnaidedImplementation EvidenceType = "unaided_implementation" // ガイドなしで実装した／レビューした
	EvidenceCrossContext          EvidenceType = "cross_context"          // 別の場面で使った
	EvidenceExplainedTo           EvidenceType = "explained_to"           // 説明を受けた（昇格しない）
	EvidenceSelfReport            EvidenceType = "self_report"            // 自己申告（昇格しない）
)

// evidenceCaps は根拠の種類ごとに到達できるレベルの上限。
//
// 「説明を受けただけ」「自己申告しただけ」でレベルが上がらないようにするための表。
// skill-map.md の「説明済みと習熟度の昇格は別物」「根拠のない昇格をしない」を機械化したもの。
var evidenceCaps = map[EvidenceType]Level{
	EvidenceDrill:                 LevelBasicConfirmed,
	EvidenceSelfExplanation:       LevelCanExplain,
	EvidenceImplementation:        LevelGuidedImpl,
	EvidenceUnaidedImplementation: LevelIndependentImpl,
	EvidenceCrossContext:          LevelCrossContext,
}

// evidencePreStates は昇格しない根拠が立てる段階前の状態。
var evidencePreStates = map[EvidenceType]PreState{
	EvidenceExplainedTo: PreStateExplainedOnly,
	EvidenceSelfReport:  PreStateSelfReported,
}

// Valid は許可リストにある根拠の種類かを返す（検証ルール V3）。
func (e EvidenceType) Valid() bool {
	if _, ok := evidenceCaps[e]; ok {
		return true
	}
	_, ok := evidencePreStates[e]
	return ok
}

// MaxLevel はこの根拠で到達できるレベルの上限を返す（検証ルール V5）。
// promotes が false のときは昇格させない種類（explained_to / self_report）。
func (e EvidenceType) MaxLevel() (limit Level, promotes bool) {
	limit, promotes = evidenceCaps[e]
	return limit, promotes
}

// MarksPreState はこの根拠が立てる段階前の状態を返す。
// 昇格する種類なら PreStateNone を返す。
func (e EvidenceType) MarksPreState() PreState {
	if ps, ok := evidencePreStates[e]; ok {
		return ps
	}
	return PreStateNone
}

// evidenceOrder は許可リストの並び。昇格できる種類を到達レベルの低い順に、
// そのあと昇格しない種類を置く。
//
// map から組み立てず固定の並びを持つのは、この並びがそのまま LLM へ渡る
// プロンプトと出力スキーマに載るため。map の反復順は実行のたびに変わり、
// 並びが変わるとプロンプトキャッシュが毎回外れる（SPEC.md §4.2）。
var evidenceOrder = []EvidenceType{
	EvidenceDrill,
	EvidenceSelfExplanation,
	EvidenceImplementation,
	EvidenceUnaidedImplementation,
	EvidenceCrossContext,
	EvidenceExplainedTo,
	EvidenceSelfReport,
}

// EvidenceTypes は許可リスト（SPEC.md §4.4）の根拠の種類を、常に同じ並びで返す。
//
// 呼び出し側が書き換えても影響が出ないよう、毎回写しを返す。
func EvidenceTypes() []EvidenceType {
	return append([]EvidenceType(nil), evidenceOrder...)
}

// ---------------------------------------------------------------------------
// ロードマップ（純粋関数が必要とする最小限の形）
// ---------------------------------------------------------------------------

// Item は詳細項目1つ。
type Item struct {
	Key         ItemKey
	DomainKey   DomainKey
	Name        string
	Description string
	// Outcome は「身につくと何ができるようになるか」。空でもよい（SPEC.md §2.1）。
	Outcome string
	// VerifyBy は「次に何をすればレベルが上がるか」。
	VerifyBy string
	// DependsOn は先に着手すべき項目。学習パスの並び順に使う。
	DependsOn  []ItemKey
	OrderIndex int
}

// Domain は分野1つ。
type Domain struct {
	Key  DomainKey
	Name string
	// Goal は「この分野を修めると何ができるようになるか」。空でもよい。
	Goal       string
	Items      []Item
	OrderIndex int
}

// Roadmap は分野と詳細項目の集合。
type Roadmap struct {
	Name    string
	Domains []Domain
	// TargetDate は学習の目標日。ゼロ値なら目標日なし。
	TargetDate time.Time
}

// AllItems はロードマップ全体の詳細項目を、分野の並び順・項目の並び順で返す。
func (r Roadmap) AllItems() []Item {
	var items []Item
	for _, d := range r.Domains {
		items = append(items, d.Items...)
	}
	return items
}

// ItemKeySet は詳細項目のキー集合を返す。
// LLM が実在しない項目を返してこないかの確認に使う（検証ルール V1）。
func (r Roadmap) ItemKeySet() map[ItemKey]struct{} {
	set := make(map[ItemKey]struct{})
	for _, d := range r.Domains {
		for _, it := range d.Items {
			set[it.Key] = struct{}{}
		}
	}
	return set
}

// ---------------------------------------------------------------------------
// 現在の状態と判定
// ---------------------------------------------------------------------------

// ItemState は詳細項目1つの現在の理解度。
//
// これは導出値であって一次データではない。assessment_events を順に適用すれば
// いつでも再計算できる（SPEC.md §3.2）。
type ItemState struct {
	ItemKey  ItemKey
	Level    Level
	PreState PreState
	// NeedsReview は「要再確認」。AI が降格を提案したときに立つ（検証ルール V6）。
	// レベル自体は下げない。
	NeedsReview bool
	// LastEvidenceAt は最後に根拠が付いた日時。ゼロ値なら根拠がまだ無い。
	// 時間経過でレベルを減衰させる代わりに、この値から鮮度を導出する（SPEC.md §1.3）。
	LastEvidenceAt time.Time
}

// Judgment は LLM が返した判定1件。**検証前の提案**であって確定値ではない。
type Judgment struct {
	ItemKey       ItemKey
	ProposedLevel Level
	EvidenceType  EvidenceType
	Rationale     string
	// Confidence は LLM の自己申告する確信度（0.0〜1.0）。
	Confidence float64
	OccurredAt time.Time
}

// ---------------------------------------------------------------------------
// 検証（SPEC.md §4.5）
// ---------------------------------------------------------------------------

// ViolationCode は検証ルールの識別子。SPEC.md §4.5 の V1〜V8 に対応する。
type ViolationCode string

const (
	ViolationUnknownItem     ViolationCode = "V1_unknown_item"       // ロードマップに無い項目
	ViolationLevelOutOfRange ViolationCode = "V2_level_out_of_range" // レベルが 0〜5 の外
	ViolationUnknownEvidence ViolationCode = "V3_unknown_evidence"   // 許可リストに無い根拠
	ViolationLevelJump       ViolationCode = "V4_level_jump"         // 2段階以上の昇格
	ViolationEvidenceTooWeak ViolationCode = "V5_evidence_too_weak"  // 根拠がそのレベルに足りない
	ViolationDowngrade       ViolationCode = "V6_downgrade"          // 降格の提案
	ViolationLowConfidence   ViolationCode = "V7_low_confidence"     // 確信度が閾値未満
	ViolationTooManyItems    ViolationCode = "V8_too_many_items"     // 1ログの判定件数が上限超え
)

// Violation は検証で弾いた、または切り詰めた内容。
//
// **握りつぶさない。** すべて llm_responses.violations に保存する。
// 「AI が変なことを言った」という事実自体が、後でプロンプトを改善するときの材料になる。
type Violation struct {
	Code    ViolationCode
	ItemKey ItemKey
	// Detail は人が読むための補足。
	Detail string
	// Proposed は LLM が提案した値。
	Proposed Level
	// Applied は実際に適用した値。Rejected が true のときは意味を持たない。
	Applied Level
	// Rejected が true なら、適用せず丸ごと捨てたことを表す。
	Rejected bool
}

// Rules は検証のふるまいを外から与える設定。ハードコードしない。
type Rules struct {
	// ConfidenceThreshold はこれ未満の確信度を保留にする（V7）。既定 0.5。
	ConfidenceThreshold float64
	// MaxItemsPerLog は1ログで判定できる項目数の上限（V8）。既定 20。
	MaxItemsPerLog int
	// MaxLevelStep は1回の判定で上がれる段数（V4）。既定 1。
	MaxLevelStep int
}

// DefaultRules は設定が与えられなかったときの既定値。
func DefaultRules() Rules {
	return Rules{
		ConfidenceThreshold: 0.5,
		MaxItemsPerLog:      20,
		MaxLevelStep:        1,
	}
}

// Applied は判定1件を適用した結果。
type Applied struct {
	// Judgment は元になった判定（LLM の提案そのもの）。
	// 適用結果を assessment_events に保存するとき、提案値・根拠・確信度が要る。
	// State から逆算できないので、どの判定から来たかを持ち歩く。
	Judgment Judgment
	// State は適用後の状態。Deferred が true のときは適用前のまま。
	State ItemState
	// Changed は状態が実際に変わったか。
	Changed bool
	// Deferred は保留にしたか。確信度が低い場合（V7）に立つ。
	// 保留分は assessment_events に書かず、人が承認してから積む（SPEC.md §4.7）。
	Deferred bool
	// Rejected は丸ごと棄却したか。レベルが範囲外（V2）・根拠が許可リストに無い（V3）場合に立つ。
	// **棄却した判定は反映してはいけない。** State は適用前のままで、判定は無かったものとして扱う。
	Rejected bool
	// Violations は検証で弾いた／切り詰めた内容。空でないことは異常を意味しない。
	Violations []Violation
}

// ---------------------------------------------------------------------------
// 鮮度（SPEC.md §1.3）
// ---------------------------------------------------------------------------

// StalenessLevel は根拠の古さ。レベルを減衰させる代わりにこちらで表す。
type StalenessLevel string

const (
	StalenessUnknown StalenessLevel = "unknown" // 根拠がまだ無い
	StalenessFresh   StalenessLevel = "fresh"   // 新しい
	StalenessAging   StalenessLevel = "aging"   // 古くなりつつある
	StalenessStale   StalenessLevel = "stale"   // 要再確認
)

// StalenessConfig は鮮度の境界。設定ファイルから与える。
type StalenessConfig struct {
	FreshWithin time.Duration // これ以内なら fresh（既定 30日）
	AgingWithin time.Duration // これ以内なら aging、超えたら stale（既定 90日）
}

// DefaultStalenessConfig は既定の境界（30日 / 90日）。
func DefaultStalenessConfig() StalenessConfig {
	const day = 24 * time.Hour
	return StalenessConfig{
		FreshWithin: 30 * day,
		AgingWithin: 90 * day,
	}
}

// ---------------------------------------------------------------------------
// 集計と表示（SPEC.md §5, §7）
// ---------------------------------------------------------------------------

// DomainSummary は分野1つの集計結果。マトリクス上部のサマリー帯に使う。
type DomainSummary struct {
	DomainKey  DomainKey
	DomainName string
	TotalItems int
	// ByLevel はレベルごとの項目数。添字がレベル（0〜5）。
	ByLevel [LevelCount]int
	// Progress は進捗率（0.0〜1.0）。達成レベルの合計 ÷（項目数 × MaxLevel）。
	Progress float64
	// StaleCount は要再確認の項目数。
	StaleCount int
}

// Weights は「次にやること」の優先度の重み（SPEC.md §5.1）。設定ファイルから与える。
type Weights struct {
	Readiness float64 // 着手できるか（依存項目が終わっているか）
	Gap       float64 // 伸びしろ（現在レベルと上限の差）
	Staleness float64 // 鮮度の悪さ（再確認を促す）
	Unlocks   float64 // これを終えると何項目が着手可能になるか
}

// DefaultWeights は既定の重み。実運用で調整する前提の初期値。
func DefaultWeights() Weights {
	return Weights{
		Readiness: 1.0,
		Gap:       0.8,
		Staleness: 0.3,
		Unlocks:   0.5,
	}
}

// Schedule は目標日に対する進捗の逼迫度。
//
// これはロードマップ全体で1つの値なので、項目の並び順には使えない
// （全項目に同じ値が乗るだけで順位が変わらない）。画面に別途表示するための情報。
type Schedule struct {
	// HasTarget は目標日が設定されているか。false なら他のフィールドは意味を持たない。
	HasTarget bool
	// DaysRemaining は目標日までの残り日数。過ぎていれば負。
	DaysRemaining int
	// ItemsRemaining はまだ一度も根拠が付いていない項目数（レベル1未満）。
	ItemsRemaining int
	// ItemsPerDay は目標日に間に合わせるために必要な1日あたりの項目数。
	// 残り日数が0以下のときは0を返す。
	ItemsPerDay float64
}

// Action は「次にやること」1件。
//
// Outcome と VerifyBy を必ず同時に載せる。「次に何をやるか」だけだと作業リストになり、
// なぜやるのかが見えないため（SPEC.md §5.1）。
type Action struct {
	ItemKey   ItemKey
	DomainKey DomainKey
	Name      string
	Outcome   string
	VerifyBy  string
	Level     Level
	Staleness StalenessLevel
	Priority  float64
	// Blocked は依存項目が未達で着手できないことを表す。
	Blocked bool
}

// PathStatus は学習パス上での位置づけ（SPEC.md §7.2）。
type PathStatus string

const (
	PathDone     PathStatus = "done"     // レベル1以上
	PathCurrent  PathStatus = "current"  // 今ここ（着手可能で最優先の1件）
	PathUpcoming PathStatus = "upcoming" // この先
)

// PathNode は学習パスの1行。
type PathNode struct {
	Item   Item
	State  ItemState
	Status PathStatus
}

// Path は分野1つの学習パス。依存関係の順に並べた一列（SPEC.md §7.2）。
//
// 依存関係は有向グラフだが、図としては描かずトポロジカルソートで一列に潰す。
// 項目が増えても読めることを優先する。
type Path struct {
	DomainKey  DomainKey
	DomainName string
	Goal       string
	Nodes      []PathNode
	DoneCount  int
	TotalCount int
}
