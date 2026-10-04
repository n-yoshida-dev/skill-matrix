package main

import (
	"reflect"
	"slices"
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
		if !reflect.DeepEqual(s, want) {
			t.Errorf("settings = %+v, want %+v", s, want)
		}
	})

	t.Run("空のオブジェクトは既定値そのもの", func(t *testing.T) {
		s, err := parseSettings([]byte(`{}`))
		if err != nil || !reflect.DeepEqual(s, defaultSettings()) {
			t.Errorf("settings = %+v, err = %v", s, err)
		}
	})

	t.Run("禁止語を読む。書かなければ空（コードに既定の語を持たない）", func(t *testing.T) {
		s, err := parseSettings([]byte(`{"forbiddenWords": ["伏せる語", "Example"]}`))
		if err != nil || !slices.Equal(s.forbiddenWords, []string{"伏せる語", "Example"}) {
			t.Errorf("forbiddenWords = %v, err = %v", s.forbiddenWords, err)
		}
		if len(defaultSettings().forbiddenWords) != 0 {
			t.Errorf("既定の禁止語 = %v（設定ファイルにだけ書く）", defaultSettings().forbiddenWords)
		}
	})

	errs := []struct {
		name, raw, want string
	}{
		{"未知のキー", `{"nextAction": {"limit": 3}}`, "nextAction"},
		{"型が違う", `{"rules": {"maxItemsPerLog": "20"}}`, "maxItemsPerLog"},
		{"JSON の後ろに続きがある", `{} {}`, "余分なデータ"},
		{"limit が負", `{"nextActions": {"limit": -1}}`, "0 以上"},
		{"禁止語に空の語がある", `{"forbiddenWords": ["伏せる語", " "]}`, "forbiddenWords[1] が空"},
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

// このテストは forbidden-words.local.json（本人しか知らない禁止語。コミットしない）の読み方を固定する。
func TestParseLocalForbidden(t *testing.T) {
	t.Run("語を読む", func(t *testing.T) {
		words, err := parseLocalForbidden([]byte(`{"forbiddenWords": ["ダミー社"]}`))
		if err != nil || !slices.Equal(words, []string{"ダミー社"}) {
			t.Errorf("words = %v, err = %v", words, err)
		}
	})

	errs := []struct {
		name, raw, want string
	}{
		{"未知のキー", `{"forbiddenWord": ["ダミー社"]}`, "forbiddenWord"},
		{"forbiddenWords が無い", `{}`, "forbiddenWords がありません"},
		{"空の語がある", `{"forbiddenWords": [""]}`, "forbiddenWords[0] が空"},
		{"JSON の後ろに続きがある", `{"forbiddenWords": []} {}`, "余分なデータ"},
	}
	for _, tt := range errs {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseLocalForbidden([]byte(tt.raw))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v（%q を含むはず）", err, tt.want)
			}
		})
	}
}
