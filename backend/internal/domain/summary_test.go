package domain

import (
	"testing"
	"time"
)

const day = 24 * time.Hour

func TestStaleness(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		lastEvidenceAt time.Time
		want           StalenessLevel
	}{
		{"根拠がまだ無い", time.Time{}, StalenessUnknown},
		{"今日", now, StalenessFresh},
		{"30日前（境界。fresh に含む）", now.Add(-30 * day), StalenessFresh},
		{"31日前", now.Add(-31 * day), StalenessAging},
		{"90日前（境界。aging に含む）", now.Add(-90 * day), StalenessAging},
		{"91日前", now.Add(-91 * day), StalenessStale},
		{"1年前", now.Add(-365 * day), StalenessStale},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Staleness(now, tt.lastEvidenceAt, DefaultStalenessConfig()); got != tt.want {
				t.Errorf("Staleness() = %q, want %q", got, tt.want)
			}
		})
	}
}

// StalenessConfig のゼロ値を渡しても既定の境界ではたらくこと。
func TestStaleness_ゼロ値の設定は既定値で埋まる(t *testing.T) {
	now := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	if got := Staleness(now, now.Add(-100*day), StalenessConfig{}); got != StalenessStale {
		t.Errorf("Staleness() = %q, want %q", got, StalenessStale)
	}
}

func TestNeedsAttention(t *testing.T) {
	now := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		st   ItemState
		want bool
	}{
		{"新しい根拠がある", ItemState{LastEvidenceAt: now.Add(-1 * day)}, false},
		{"根拠が古い", ItemState{LastEvidenceAt: now.Add(-100 * day)}, true},
		{"降格の提案があった", ItemState{LastEvidenceAt: now, NeedsReview: true}, true},
		{"未着手（根拠なし）は要再確認ではない", ItemState{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsAttention(tt.st, now, DefaultStalenessConfig()); got != tt.want {
				t.Errorf("NeedsAttention() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRollupDomain(t *testing.T) {
	now := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	d := testRoadmap().Domains[0] // go-01, go-02 の2項目

	t.Run("状態が無い項目は未着手として数える", func(t *testing.T) {
		got := RollupDomain(d, nil, now, DefaultStalenessConfig())

		if got.TotalItems != 2 {
			t.Errorf("TotalItems = %d, want 2", got.TotalItems)
		}
		if got.ByLevel[LevelNone] != 2 {
			t.Errorf("ByLevel[0] = %d, want 2", got.ByLevel[LevelNone])
		}
		if got.Progress != 0 {
			t.Errorf("Progress = %v, want 0", got.Progress)
		}
		if got.StaleCount != 0 {
			t.Errorf("StaleCount = %d, want 0", got.StaleCount)
		}
	})

	t.Run("レベルごとの内訳と進捗率を出す", func(t *testing.T) {
		states := map[ItemKey]ItemState{
			"go-01": {ItemKey: "go-01", VerifiedLevel: LevelCanExplain, LastEvidenceAt: now},
			"go-02": {ItemKey: "go-02", VerifiedLevel: LevelBasicConfirmed, LastEvidenceAt: now},
		}
		got := RollupDomain(d, states, now, DefaultStalenessConfig())

		if got.ByLevel[LevelCanExplain] != 1 || got.ByLevel[LevelBasicConfirmed] != 1 {
			t.Errorf("ByLevel = %v, want L1:1 L2:1", got.ByLevel)
		}
		// (2 + 1) / (2項目 × 最大レベル5) = 0.3
		if want := 0.3; got.Progress != want {
			t.Errorf("Progress = %v, want %v", got.Progress, want)
		}
	})

	t.Run("古い根拠と降格提案を要再確認として数える", func(t *testing.T) {
		states := map[ItemKey]ItemState{
			"go-01": {ItemKey: "go-01", VerifiedLevel: LevelCanExplain, LastEvidenceAt: now.Add(-100 * day)},
			"go-02": {ItemKey: "go-02", VerifiedLevel: LevelBasicConfirmed, LastEvidenceAt: now, NeedsReview: true},
		}
		got := RollupDomain(d, states, now, DefaultStalenessConfig())

		if got.StaleCount != 2 {
			t.Errorf("StaleCount = %d, want 2", got.StaleCount)
		}
	})

	t.Run("保存値が範囲外でも落ちない", func(t *testing.T) {
		states := map[ItemKey]ItemState{
			"go-01": {ItemKey: "go-01", VerifiedLevel: 99},
			"go-02": {ItemKey: "go-02", VerifiedLevel: -5},
		}
		got := RollupDomain(d, states, now, DefaultStalenessConfig())

		if got.ByLevel[MaxLevel] != 1 || got.ByLevel[LevelNone] != 1 {
			t.Errorf("ByLevel = %v, want 範囲内に丸められること", got.ByLevel)
		}
	})
}

func TestBuildSchedule(t *testing.T) {
	now := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)

	t.Run("目標日が無ければ残り項目数だけ数える", func(t *testing.T) {
		rm := testRoadmap()
		got := BuildSchedule(rm, nil, now)

		if got.HasTarget {
			t.Error("HasTarget = true, want false")
		}
		if got.ItemsRemaining != 2 {
			t.Errorf("ItemsRemaining = %d, want 2", got.ItemsRemaining)
		}
	})

	t.Run("必要なペースを出す", func(t *testing.T) {
		rm := testRoadmap()
		rm.TargetDate = time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)

		got := BuildSchedule(rm, nil, now)

		if !got.HasTarget {
			t.Fatal("HasTarget = false, want true")
		}
		if got.DaysRemaining != 10 {
			t.Errorf("DaysRemaining = %d, want 10", got.DaysRemaining)
		}
		// 2項目 ÷ 10日 = 0.2
		if want := 0.2; got.ItemsPerDay != want {
			t.Errorf("ItemsPerDay = %v, want %v", got.ItemsPerDay, want)
		}
	})

	t.Run("レベル1以上の項目は残りに数えない", func(t *testing.T) {
		rm := testRoadmap()
		states := map[ItemKey]ItemState{
			"go-01": {ItemKey: "go-01", VerifiedLevel: LevelBasicConfirmed},
		}
		got := BuildSchedule(rm, states, now)

		if got.ItemsRemaining != 1 {
			t.Errorf("ItemsRemaining = %d, want 1", got.ItemsRemaining)
		}
	})

	t.Run("目標日を過ぎていてもゼロ除算しない", func(t *testing.T) {
		rm := testRoadmap()
		rm.TargetDate = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

		got := BuildSchedule(rm, nil, now)

		if got.DaysRemaining != -9 {
			t.Errorf("DaysRemaining = %d, want -9", got.DaysRemaining)
		}
		if got.ItemsPerDay != 0 {
			t.Errorf("ItemsPerDay = %v, want 0", got.ItemsPerDay)
		}
	})
}
