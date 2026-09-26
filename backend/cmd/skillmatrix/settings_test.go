package main

import (
	"strings"
	"testing"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
)

// このテストは data/settings.json（SPEC.md §8.3）の読み方を固定する。
// 書かれたキーだけを上書きし、書かれていないキーは domain の既定値のまま。

func TestParseSettings(t *testing.T) {
	t.Run("全部のキーを読む", func(t *testing.T) {
		s, err := parseSettings([]byte(`{
			"rules": {"confidenceThreshold": 0.6, "maxItemsPerLog": 10},
			"staleness": {"freshWithinDays": 14, "agingWithinDays": 60},
			"weights": {"readiness": 2, "gap": 1, "staleness": 0.5, "unlocks": 0.1},
			"nextActions": {"limit": 3},
			"site": {"repoUrl": "https://example.com/repo"}
		}`))
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		if s.rules != (domain.Rules{ConfidenceThreshold: 0.6, MaxItemsPerLog: 10}) {
			t.Errorf("rules = %+v", s.rules)
		}
		const day = 24 * time.Hour
		if s.staleness != (domain.StalenessConfig{FreshWithin: 14 * day, AgingWithin: 60 * day}) {
			t.Errorf("staleness = %+v", s.staleness)
		}
		if s.weights != (domain.Weights{Readiness: 2, Gap: 1, Staleness: 0.5, Unlocks: 0.1}) {
			t.Errorf("weights = %+v", s.weights)
		}
		if s.nextActionsLimit != 3 {
			t.Errorf("nextActionsLimit = %d", s.nextActionsLimit)
		}
	})

	t.Run("書かれていないキーは既定値", func(t *testing.T) {
		s, err := parseSettings([]byte(`{"rules": {"maxItemsPerLog": 5}}`))
		if err != nil {
			t.Fatalf("エラーになった: %v", err)
		}
		want := defaultSettings()
		want.rules.MaxItemsPerLog = 5
		if s != want {
			t.Errorf("settings = %+v, want %+v", s, want)
		}
	})

	t.Run("空のオブジェクトは既定値そのもの", func(t *testing.T) {
		s, err := parseSettings([]byte(`{}`))
		if err != nil || s != defaultSettings() {
			t.Errorf("settings = %+v, err = %v", s, err)
		}
	})

	errs := []struct {
		name, raw, want string
	}{
		{"未知のキー", `{"nextAction": {"limit": 3}}`, "nextAction"},
		{"型が違う", `{"rules": {"maxItemsPerLog": "20"}}`, "maxItemsPerLog"},
		{"JSON の後ろに続きがある", `{} {}`, "余分なデータ"},
		{"limit が負", `{"nextActions": {"limit": -1}}`, "0 以上"},
	}
	for _, tt := range errs {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSettings([]byte(tt.raw))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v（%q を含むはず）", err, tt.want)
			}
		})
	}
}
