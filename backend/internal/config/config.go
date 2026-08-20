// Package config は環境変数からアプリ設定を読み込み、検証する。
//
// 変数の一覧は SPEC.md §8.3 が正本。雛形は backend/.env.example にある。
// 「ハードコードを禁止し、外部由来の値は設定に切り出す」という規約（../CLAUDE.md）の受け皿。
//
// 起動時にまとめて検証し、間違いは全部まとめて報告する。1つ直すたびに再起動して
// 次の1つが出る、という直し方をさせないため。
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// AppEnv は実行環境。Cookie の Secure 属性の有無を分けるために使う。
type AppEnv string

const (
	// EnvDevelopment はローカル開発。http で動かすため Secure Cookie を落とす。
	EnvDevelopment AppEnv = "development"
	// EnvProduction は本番。https 前提。
	EnvProduction AppEnv = "production"
)

// LLMProvider は判定に使う LLM 実装の切り替え。
type LLMProvider string

const (
	// ProviderAnthropic は Claude API を実際に呼ぶ。
	ProviderAnthropic LLMProvider = "anthropic"
	// ProviderStub は LLM を呼ばず固定レスポンスを返す。
	// 開発中の課金をゼロにし、テストの結果を決定的にするために使う（SPEC.md §8.3）。
	ProviderStub LLMProvider = "stub"
)

// 既定値。環境変数が未設定のときに使う。
const (
	defaultPort                = "8080"
	defaultModel               = "claude-sonnet-5"
	defaultMonthlyQuota        = 100
	defaultConfidenceThreshold = 0.5
	defaultMaxItemsPerLog      = 20
)

// minSessionSecretLen はセッション Cookie の署名鍵に要求する最低長。
// 短い鍵は総当たりで破られ、他人のセッションを偽造できてしまう。
const minSessionSecretLen = 32

// Config はアプリ全体の設定。起動時に1度だけ組み立て、以降は読み取り専用で扱う。
type Config struct {
	AppEnv AppEnv
	Port   string
	// FrontendOrigin は CORS の許可と、ログイン後の戻り先に使うフロントの配信元。
	FrontendOrigin string
	DatabaseURL    string
	SessionSecret  string
	GitHub         GitHubConfig
	LLM            LLMConfig
	Judgment       JudgmentConfig
}

// CookieSecure はセッション Cookie に Secure 属性を付けるかを返す。
// ローカル開発は http なので、Secure を付けるとブラウザが Cookie を送らずログインできない。
func (c *Config) CookieSecure() bool {
	return c.AppEnv != EnvDevelopment
}

// GitHubConfig は GitHub OAuth の設定（SPEC.md §6）。
type GitHubConfig struct {
	ClientID     string
	ClientSecret string
	CallbackURL  string
}

// LLMConfig は判定に使う LLM の設定。
type LLMConfig struct {
	Provider LLMProvider
	// APIKey は Provider が anthropic のときだけ必須。
	// この値はバックエンドの外（フロント・ログ・共有機能）へ出さない（SPEC.md §9）。
	APIKey string
	Model  string
}

// JudgmentConfig は AI 判定の上限値。制度・料金と同じく直書きせず設定に出す。
type JudgmentConfig struct {
	// MonthlyQuota は1ユーザーあたりの月間判定回数の上限（SPEC.md §4.6）。
	MonthlyQuota int
	// ConfidenceThreshold はこれを下回る確信度の判定を保留にするしきい値（検証ルール V7）。
	ConfidenceThreshold float64
	// MaxItemsPerLog は1件のログで更新を認める項目数の上限（検証ルール V8）。
	MaxItemsPerLog int
}

// Load は環境変数から設定を読み込む。
// 不備があっても最初の1件で打ち切らず、全部集めてから1つのエラーにして返す。
func Load() (*Config, error) {
	var errs []error

	// エラーを集めるためのヘルパ群。エラーは errs に積み、値はゼロ値を返す。
	fail := func(err error) {
		errs = append(errs, err)
	}

	cfg := &Config{
		AppEnv:         appEnv(fail),
		Port:           optional("PORT", defaultPort),
		FrontendOrigin: requiredURL("FRONTEND_ORIGIN", []string{"http", "https"}, fail),
		DatabaseURL:    requiredURL("DATABASE_URL", []string{"postgres", "postgresql"}, fail),
		SessionSecret:  requiredSecret("SESSION_SECRET", minSessionSecretLen, fail),
		GitHub: GitHubConfig{
			ClientID:     required("GITHUB_OAUTH_CLIENT_ID", fail),
			ClientSecret: required("GITHUB_OAUTH_CLIENT_SECRET", fail),
			CallbackURL:  requiredURL("GITHUB_OAUTH_CALLBACK_URL", []string{"http", "https"}, fail),
		},
		LLM: LLMConfig{
			Provider: provider(fail),
			Model:    optional("ANTHROPIC_MODEL", defaultModel),
		},
		Judgment: JudgmentConfig{
			MonthlyQuota:        intInRange("JUDGMENT_MONTHLY_QUOTA", defaultMonthlyQuota, 1, 100000, fail),
			ConfidenceThreshold: floatInRange("JUDGMENT_CONFIDENCE_THRESHOLD", defaultConfidenceThreshold, 0, 1, fail),
			MaxItemsPerLog:      intInRange("JUDGMENT_MAX_ITEMS_PER_LOG", defaultMaxItemsPerLog, 1, 200, fail),
		},
	}

	// API キーは実際に Claude を呼ぶときだけ必須。stub 運用で鍵を用意させない。
	if cfg.LLM.Provider == ProviderAnthropic {
		cfg.LLM.APIKey = required("ANTHROPIC_API_KEY", fail)
	} else {
		cfg.LLM.APIKey = strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("環境変数の設定に不備があります（backend/.env.example を参照）:\n%w", errors.Join(errs...))
	}
	return cfg, nil
}

// String は設定を1行で表す。秘密情報は伏せる。
// ログに %v で流し込んでも鍵が漏れないようにするために定義している。
func (c *Config) String() string {
	return fmt.Sprintf(
		"Config{AppEnv:%s Port:%s FrontendOrigin:%s DatabaseURL:%s SessionSecret:%s "+
			"GitHub:{ClientID:%s ClientSecret:%s CallbackURL:%s} "+
			"LLM:{Provider:%s APIKey:%s Model:%s} Judgment:{MonthlyQuota:%d ConfidenceThreshold:%g MaxItemsPerLog:%d}}",
		c.AppEnv, c.Port, c.FrontendOrigin, redactURL(c.DatabaseURL), redact(c.SessionSecret),
		c.GitHub.ClientID, redact(c.GitHub.ClientSecret), c.GitHub.CallbackURL,
		c.LLM.Provider, redact(c.LLM.APIKey), c.LLM.Model,
		c.Judgment.MonthlyQuota, c.Judgment.ConfidenceThreshold, c.Judgment.MaxItemsPerLog,
	)
}

// ---------------------------------------------------------------------------
// 読み取りヘルパ
// ---------------------------------------------------------------------------

// lookup は環境変数を前後の空白を落として返す。空文字は未設定として扱う。
func lookup(key string) (string, bool) {
	v := strings.TrimSpace(os.Getenv(key))
	return v, v != ""
}

// optional は未設定なら既定値を返す。
func optional(key, fallback string) string {
	if v, ok := lookup(key); ok {
		return v
	}
	return fallback
}

// required は必須の文字列を読む。未設定ならエラーを積む。
func required(key string, fail func(error)) string {
	v, ok := lookup(key)
	if !ok {
		fail(fmt.Errorf("%s: 必須ですが設定されていません", key))
	}
	return v
}

// requiredSecret は必須かつ最低長を満たす秘密情報を読む。
func requiredSecret(key string, minLen int, fail func(error)) string {
	v, ok := lookup(key)
	if !ok {
		fail(fmt.Errorf("%s: 必須ですが設定されていません", key))
		return ""
	}
	if len(v) < minLen {
		fail(fmt.Errorf("%s: %d 文字以上にしてください（現在 %d 文字）", key, minLen, len(v)))
	}
	return v
}

// requiredURL は必須の URL を読む。scheme が許可リストにあることまで確認する。
// 接続先を間違えたまま起動して、実行時になって初めて気づく事故を防ぐ。
func requiredURL(key string, schemes []string, fail func(error)) string {
	v, ok := lookup(key)
	if !ok {
		fail(fmt.Errorf("%s: 必須ですが設定されていません", key))
		return ""
	}
	u, err := url.Parse(v)
	if err != nil {
		fail(fmt.Errorf("%s: URL として解釈できません: %w", key, err))
		return v
	}
	if u.Host == "" {
		fail(fmt.Errorf("%s: ホスト名を含む URL にしてください", key))
		return v
	}
	for _, s := range schemes {
		if u.Scheme == s {
			return v
		}
	}
	fail(fmt.Errorf("%s: scheme は %s のいずれかにしてください（実際は %q）",
		key, strings.Join(schemes, " / "), u.Scheme))
	return v
}

// appEnv は APP_ENV を読む。未設定なら production。
// 「設定し忘れたら Secure Cookie が付く」側に倒す。逆にすると、本番で書き忘れたときに
// 平文で Cookie が飛ぶ事故になる。
func appEnv(fail func(error)) AppEnv {
	v := optional("APP_ENV", string(EnvProduction))
	switch AppEnv(v) {
	case EnvDevelopment, EnvProduction:
		return AppEnv(v)
	default:
		fail(fmt.Errorf("APP_ENV: %q は未対応です（%s / %s のいずれか）",
			v, EnvDevelopment, EnvProduction))
		return ""
	}
}

// provider は LLM_PROVIDER を読む。未設定なら anthropic。
func provider(fail func(error)) LLMProvider {
	v := optional("LLM_PROVIDER", string(ProviderAnthropic))
	switch LLMProvider(v) {
	case ProviderAnthropic, ProviderStub:
		return LLMProvider(v)
	default:
		fail(fmt.Errorf("LLM_PROVIDER: %q は未対応です（%s / %s のいずれか）",
			v, ProviderAnthropic, ProviderStub))
		return ""
	}
}

// intInRange は整数を読み、min〜max の範囲に収まっているかを確認する。
func intInRange(key string, fallback, minV, maxV int, fail func(error)) int {
	v, ok := lookup(key)
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		fail(fmt.Errorf("%s: 整数として解釈できません（実際は %q）", key, v))
		return fallback
	}
	if n < minV || n > maxV {
		fail(fmt.Errorf("%s: %d〜%d の範囲にしてください（実際は %d）", key, minV, maxV, n))
		return fallback
	}
	return n
}

// floatInRange は小数を読み、min〜max の範囲に収まっているかを確認する。
func floatInRange(key string, fallback, minV, maxV float64, fail func(error)) float64 {
	v, ok := lookup(key)
	if !ok {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		fail(fmt.Errorf("%s: 小数として解釈できません（実際は %q）", key, v))
		return fallback
	}
	if f < minV || f > maxV {
		fail(fmt.Errorf("%s: %g〜%g の範囲にしてください（実際は %g）", key, minV, maxV, f))
		return fallback
	}
	return f
}

// ---------------------------------------------------------------------------
// 伏せ字
// ---------------------------------------------------------------------------

// redact は秘密情報を伏せる。設定されているかどうかだけ分かればよい。
func redact(s string) string {
	if s == "" {
		return "(未設定)"
	}
	return "***"
}

// redactURL は接続文字列からパスワードだけを伏せる。
// 接続先ホストとデータベース名は障害調査で必要なので残す。
func redactURL(raw string) string {
	if raw == "" {
		return "(未設定)"
	}
	u, err := url.Parse(raw)
	if err != nil {
		// 解釈できない値は丸ごと伏せる。中身に何が入っているか分からないため。
		return "***"
	}
	if u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			// 記号を使うと URL エンコードされて読みにくくなるので英字だけの目印にする
			u.User = url.UserPassword(u.User.Username(), "redacted")
		}
	}
	return u.String()
}
