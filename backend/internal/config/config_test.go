package config

import (
	"strings"
	"testing"
)

// validEnv は検証を通る最小限の環境変数一式。テストごとに必要な分だけ上書きして使う。
func validEnv() map[string]string {
	return map[string]string{
		"APP_ENV":                       "",
		"PORT":                          "",
		"FRONTEND_ORIGIN":               "http://localhost:5173",
		"DATABASE_URL":                  "postgres://skillmatrix:skillmatrix@localhost:5432/skillmatrix?sslmode=disable",
		"SESSION_SECRET":                strings.Repeat("a", minSessionSecretLen),
		"GITHUB_OAUTH_CLIENT_ID":        "Iv1.dummy",
		"GITHUB_OAUTH_CLIENT_SECRET":    "dummy-client-secret",
		"GITHUB_OAUTH_CALLBACK_URL":     "http://localhost:8080/api/auth/github/callback",
		"LLM_PROVIDER":                  "stub",
		"ANTHROPIC_API_KEY":             "",
		"ANTHROPIC_MODEL":               "",
		"JUDGMENT_MONTHLY_QUOTA":        "",
		"JUDGMENT_CONFIDENCE_THRESHOLD": "",
		"JUDGMENT_MAX_ITEMS_PER_LOG":    "",
	}
}

// setEnv は環境変数をテストの間だけ差し替える（t.Setenv がテスト終了時に元へ戻す）。
// 空文字は「未設定」として扱われるので、消したい変数は "" を渡す。
func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func TestLoad_既定値が入る(t *testing.T) {
	setEnv(t, validEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("エラーは想定していない: %v", err)
	}

	if cfg.Port != defaultPort {
		t.Errorf("Port = %q, 期待値 %q", cfg.Port, defaultPort)
	}
	// APP_ENV を書き忘れたときは安全側（Secure Cookie が付く側）に倒れること
	if cfg.AppEnv != EnvProduction {
		t.Errorf("AppEnv = %q, 期待値 %q", cfg.AppEnv, EnvProduction)
	}
	if !cfg.CookieSecure() {
		t.Error("APP_ENV 未設定なら CookieSecure() は true であるべき")
	}
	if cfg.LLM.Model != defaultModel {
		t.Errorf("LLM.Model = %q, 期待値 %q", cfg.LLM.Model, defaultModel)
	}
	if cfg.Judgment.MonthlyQuota != defaultMonthlyQuota {
		t.Errorf("MonthlyQuota = %d, 期待値 %d", cfg.Judgment.MonthlyQuota, defaultMonthlyQuota)
	}
	if cfg.Judgment.ConfidenceThreshold != defaultConfidenceThreshold {
		t.Errorf("ConfidenceThreshold = %g, 期待値 %g", cfg.Judgment.ConfidenceThreshold, defaultConfidenceThreshold)
	}
	if cfg.Judgment.MaxItemsPerLog != defaultMaxItemsPerLog {
		t.Errorf("MaxItemsPerLog = %d, 期待値 %d", cfg.Judgment.MaxItemsPerLog, defaultMaxItemsPerLog)
	}
	if cfg.Judgment.MaxAttempts != defaultMaxAttempts {
		t.Errorf("MaxAttempts = %d, 期待値 %d", cfg.Judgment.MaxAttempts, defaultMaxAttempts)
	}
	if cfg.Judgment.PollIntervalSeconds != defaultPollIntervalSeconds {
		t.Errorf("PollIntervalSeconds = %d, 期待値 %d", cfg.Judgment.PollIntervalSeconds, defaultPollIntervalSeconds)
	}
}

func TestLoad_明示した値が優先される(t *testing.T) {
	env := validEnv()
	env["PORT"] = "9000"
	env["LLM_PROVIDER"] = "anthropic"
	env["ANTHROPIC_API_KEY"] = "sk-ant-dummy"
	env["ANTHROPIC_MODEL"] = "claude-haiku-4-5-20251001"
	env["JUDGMENT_MONTHLY_QUOTA"] = "30"
	env["JUDGMENT_CONFIDENCE_THRESHOLD"] = "0.75"
	env["JUDGMENT_MAX_ITEMS_PER_LOG"] = "5"
	env["JUDGMENT_MAX_ATTEMPTS"] = "2"
	env["JUDGMENT_POLL_INTERVAL_SECONDS"] = "30"
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("エラーは想定していない: %v", err)
	}

	if cfg.Port != "9000" {
		t.Errorf("Port = %q, 期待値 %q", cfg.Port, "9000")
	}
	if cfg.LLM.Provider != ProviderAnthropic {
		t.Errorf("Provider = %q, 期待値 %q", cfg.LLM.Provider, ProviderAnthropic)
	}
	if cfg.LLM.APIKey != "sk-ant-dummy" {
		t.Errorf("APIKey が読めていない: %q", cfg.LLM.APIKey)
	}
	if cfg.LLM.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("Model = %q", cfg.LLM.Model)
	}
	if cfg.Judgment.MonthlyQuota != 30 {
		t.Errorf("MonthlyQuota = %d, 期待値 30", cfg.Judgment.MonthlyQuota)
	}
	if cfg.Judgment.ConfidenceThreshold != 0.75 {
		t.Errorf("ConfidenceThreshold = %g, 期待値 0.75", cfg.Judgment.ConfidenceThreshold)
	}
	if cfg.Judgment.MaxItemsPerLog != 5 {
		t.Errorf("MaxItemsPerLog = %d, 期待値 5", cfg.Judgment.MaxItemsPerLog)
	}
	if cfg.Judgment.MaxAttempts != 2 {
		t.Errorf("MaxAttempts = %d, 期待値 2", cfg.Judgment.MaxAttempts)
	}
	if cfg.Judgment.PollIntervalSeconds != 30 {
		t.Errorf("PollIntervalSeconds = %d, 期待値 30", cfg.Judgment.PollIntervalSeconds)
	}
}

func TestLoad_developmentならSecureCookieを落とす(t *testing.T) {
	env := validEnv()
	env["APP_ENV"] = "development"
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("エラーは想定していない: %v", err)
	}
	if cfg.AppEnv != EnvDevelopment {
		t.Errorf("AppEnv = %q, 期待値 %q", cfg.AppEnv, EnvDevelopment)
	}
	if cfg.CookieSecure() {
		t.Error("development では CookieSecure() は false であるべき（ローカルは http のため）")
	}
}

func TestLoad_前後の空白は落とす(t *testing.T) {
	env := validEnv()
	env["GITHUB_OAUTH_CLIENT_ID"] = "  Iv1.dummy  "
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("エラーは想定していない: %v", err)
	}
	if cfg.GitHub.ClientID != "Iv1.dummy" {
		t.Errorf("ClientID = %q, 空白が落ちていない", cfg.GitHub.ClientID)
	}
}

func TestLoad_必須が欠けていると全部まとめて報告する(t *testing.T) {
	env := validEnv()
	// 必須の4つを同時に消す。1件目で打ち切らないことを確認する。
	env["DATABASE_URL"] = ""
	env["SESSION_SECRET"] = ""
	env["GITHUB_OAUTH_CLIENT_ID"] = ""
	env["GITHUB_OAUTH_CALLBACK_URL"] = ""
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("エラーになるはず")
	}
	for _, key := range []string{
		"DATABASE_URL", "SESSION_SECRET", "GITHUB_OAUTH_CLIENT_ID", "GITHUB_OAUTH_CALLBACK_URL",
	} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("エラーに %s が含まれていない:\n%v", key, err)
		}
	}
}

func TestLoad_セッション鍵が短いと弾く(t *testing.T) {
	env := validEnv()
	env["SESSION_SECRET"] = strings.Repeat("a", minSessionSecretLen-1)
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("短い鍵はエラーになるはず")
	}
	if !strings.Contains(err.Error(), "SESSION_SECRET") {
		t.Errorf("エラーに SESSION_SECRET が含まれていない:\n%v", err)
	}
}

func TestLoad_anthropicならAPIキーが必須(t *testing.T) {
	env := validEnv()
	env["LLM_PROVIDER"] = "anthropic"
	env["ANTHROPIC_API_KEY"] = ""
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("API キーなしはエラーになるはず")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Errorf("エラーに ANTHROPIC_API_KEY が含まれていない:\n%v", err)
	}
}

func TestLoad_stubならAPIキーは不要(t *testing.T) {
	env := validEnv()
	env["LLM_PROVIDER"] = "stub"
	env["ANTHROPIC_API_KEY"] = ""
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("stub では API キーなしでも通るはず: %v", err)
	}
	if cfg.LLM.Provider != ProviderStub {
		t.Errorf("Provider = %q, 期待値 %q", cfg.LLM.Provider, ProviderStub)
	}
}

func TestLoad_不正な値を弾く(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantMsg string
	}{
		{"未対応の実行環境", "APP_ENV", "staging", "APP_ENV"},
		{"フロントのオリジンが URL でない", "FRONTEND_ORIGIN", "localhost:5173", "FRONTEND_ORIGIN"},
		{"未対応のプロバイダ", "LLM_PROVIDER", "openai", "LLM_PROVIDER"},
		{"DB の scheme が違う", "DATABASE_URL", "mysql://localhost:3306/db", "DATABASE_URL"},
		{"DB にホストがない", "DATABASE_URL", "postgres://", "DATABASE_URL"},
		{"コールバックが URL でない", "GITHUB_OAUTH_CALLBACK_URL", "localhost/callback", "GITHUB_OAUTH_CALLBACK_URL"},
		{"クォータが整数でない", "JUDGMENT_MONTHLY_QUOTA", "たくさん", "JUDGMENT_MONTHLY_QUOTA"},
		{"クォータが 0", "JUDGMENT_MONTHLY_QUOTA", "0", "JUDGMENT_MONTHLY_QUOTA"},
		{"しきい値が範囲外", "JUDGMENT_CONFIDENCE_THRESHOLD", "1.5", "JUDGMENT_CONFIDENCE_THRESHOLD"},
		{"しきい値が小数でない", "JUDGMENT_CONFIDENCE_THRESHOLD", "high", "JUDGMENT_CONFIDENCE_THRESHOLD"},
		{"項目数の上限が負", "JUDGMENT_MAX_ITEMS_PER_LOG", "-1", "JUDGMENT_MAX_ITEMS_PER_LOG"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := validEnv()
			env[tt.key] = tt.value
			setEnv(t, env)

			_, err := Load()
			if err == nil {
				t.Fatalf("%s=%q はエラーになるはず", tt.key, tt.value)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("エラーに %s が含まれていない:\n%v", tt.wantMsg, err)
			}
		})
	}
}

func TestConfig_String_秘密情報を伏せる(t *testing.T) {
	env := validEnv()
	env["LLM_PROVIDER"] = "anthropic"
	env["ANTHROPIC_API_KEY"] = "sk-ant-super-secret"
	env["DATABASE_URL"] = "postgres://skillmatrix:p4ssw0rd@localhost:5432/skillmatrix"
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("エラーは想定していない: %v", err)
	}

	got := cfg.String()
	// ログに流し込んでも漏れてはいけない値
	for _, secret := range []string{
		"sk-ant-super-secret",
		"dummy-client-secret",
		"p4ssw0rd",
		strings.Repeat("a", minSessionSecretLen),
	} {
		if strings.Contains(got, secret) {
			t.Errorf("秘密情報が伏せられていない: %q が出力に含まれる\n%s", secret, got)
		}
	}
	// 障害調査のために残すべき値
	for _, visible := range []string{"localhost:5432", "skillmatrix", "8080"} {
		if !strings.Contains(got, visible) {
			t.Errorf("%q は出力に残ってほしい\n%s", visible, got)
		}
	}
}
