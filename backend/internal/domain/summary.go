package domain

import "time"

// このファイルは「今どこまで来たか」を集計する処理を持つ。
//   - Staleness  : 根拠の古さ（レベルを減衰させる代わりの指標）
//   - RollupDomain: 分野1つの進捗まとめ（マトリクス上部のサマリー帯に使う）
//   - BuildSchedule: 目標日に対する逼迫度
//
// 仕様は SPEC.md §1.3・§5、人間向けの解説は docs/spec-guide.md §2.3。

// withDefaults はゼロ値のフィールドを既定値で埋める。
func (c StalenessConfig) withDefaults() StalenessConfig {
	d := DefaultStalenessConfig()
	if c.FreshWithin == 0 {
		c.FreshWithin = d.FreshWithin
	}
	if c.AgingWithin == 0 {
		c.AgingWithin = d.AgingWithin
	}
	return c
}

// Staleness は最終根拠日からの経過で鮮度を返す。
//
// 時間が経ってもレベルは下げない。「なぜ昨日より薄くなったのか」を説明できない値を
// 作らないため（SPEC.md §1.3）。代わりにこの指標を別の軸として持ち、
// 画面では升目の枠線で表す。
//
// 経過は暦日で数える（今日の日付 − 最終根拠日。SPEC.md §1.3）。時刻の差で数えると、
// 境界の日（30 日目・90 日目）に朝と夜で答えが変わり、画面（TypeScript の staleness）と 1 日ずれるため。
// 境界の設定（FreshWithin / AgingWithin）は日数に切り捨てて使う。
//
// lastEvidenceAt がゼロ値（まだ一度も根拠が付いていない）なら StalenessUnknown。
func Staleness(now, lastEvidenceAt time.Time, cfg StalenessConfig) StalenessLevel {
	if lastEvidenceAt.IsZero() {
		return StalenessUnknown
	}
	cfg = cfg.withDefaults()

	elapsed := calendarDaysSince(now, lastEvidenceAt)
	switch {
	case elapsed <= wholeDays(cfg.FreshWithin):
		return StalenessFresh
	case elapsed <= wholeDays(cfg.AgingWithin):
		return StalenessAging
	default:
		return StalenessStale
	}
}

// calendarDaysSince は、then の日付から now の日付までの暦日の差を返す。
// 時刻は見ず、それぞれが持つタイムゾーンでの日付（年・月・日）だけで数える。
// CLI は now を手元の時刻で渡し、最終根拠日は日付だけ（UTC の 0 時）なので、どちらも「その日」として比べられる。
func calendarDaysSince(now, then time.Time) int {
	y1, m1, d1 := now.Date()
	y2, m2, d2 := then.Date()
	a := time.Date(y1, m1, d1, 0, 0, 0, 0, time.UTC)
	b := time.Date(y2, m2, d2, 0, 0, 0, 0, time.UTC)
	return int(a.Sub(b) / (24 * time.Hour))
}

// wholeDays は期間を日数にする（端数は切り捨て）。
func wholeDays(d time.Duration) int {
	return int(d / (24 * time.Hour))
}

// NeedsAttention はその項目が「要再確認」として目立たせるべきかを返す。
//
// 根拠が古くなった場合と、LLM が降格を提案した場合（検証ルール V6）の
// どちらもここで拾う。画面ではどちらも同じ枠線で表す。
func NeedsAttention(st ItemState, now time.Time, cfg StalenessConfig) bool {
	if st.NeedsReview {
		return true
	}
	return Staleness(now, st.LastEvidenceAt, cfg) == StalenessStale
}

// RollupDomain は分野1つの進捗を集計する。
//
// states に無い項目は「未着手（レベル0・根拠なし）」として数える。
// 状態はまだ判定されていないだけで、項目自体は存在するため。
func RollupDomain(d Domain, states map[ItemKey]ItemState, now time.Time, cfg StalenessConfig) DomainSummary {
	cfg = cfg.withDefaults()

	sum := DomainSummary{
		DomainKey:  d.Key,
		DomainName: d.Name,
		TotalItems: len(d.Items),
	}

	levelTotal := 0
	for _, it := range d.Items {
		st, ok := states[it.Key]
		if !ok {
			st = ItemState{ItemKey: it.Key}
		}

		// 保存された値が壊れていても配列外参照で落ちないようにする。
		lv := st.VerifiedLevel
		if lv < LevelNone {
			lv = LevelNone
		} else if lv > MaxLevel {
			lv = MaxLevel
		}

		sum.ByLevel[lv]++
		levelTotal += int(lv)
		if NeedsAttention(st, now, cfg) {
			sum.StaleCount++
		}
		// 実装の根拠はあるが基礎の確認が未了（印が表示レベルより上にある）。サマリー帯に 1 列足す（SPEC.md §7.1）
		if st.TopEvidenced() > st.VerifiedLevel {
			sum.PendingCount++
		}
	}

	if sum.TotalItems > 0 {
		sum.Progress = float64(levelTotal) / float64(sum.TotalItems*int(MaxLevel))
	}
	return sum
}

// RollupAll はロードマップ全体の分野を、定義された並び順で集計する。
func RollupAll(rm Roadmap, states map[ItemKey]ItemState, now time.Time, cfg StalenessConfig) []DomainSummary {
	out := make([]DomainSummary, 0, len(rm.Domains))
	for _, d := range rm.Domains {
		out = append(out, RollupDomain(d, states, now, cfg))
	}
	return out
}

// BuildSchedule は目標日に対する逼迫度を組み立てる。
//
// 「残り」は**レベル1未満の項目数**とする。全項目をレベル5にするのは現実的でないため、
// 学習パス画面の「済」の定義（レベル1以上）と揃えている。
//
// この値は項目の優先順位には使わない。ロードマップ全体で同じ値になるので、
// 足しても順位が変わらないため（SPEC.md §5.1）。画面に単独で表示する。
func BuildSchedule(rm Roadmap, states map[ItemKey]ItemState, now time.Time) Schedule {
	var s Schedule

	for _, it := range rm.AllItems() {
		if states[it.Key].VerifiedLevel < LevelBasicConfirmed {
			s.ItemsRemaining++
		}
	}

	if rm.TargetDate.IsZero() {
		return s
	}
	s.HasTarget = true

	// 日付単位で数える。時刻の差ではなく暦日の差を見たいため、日の始まりに丸める。
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	to := time.Date(rm.TargetDate.Year(), rm.TargetDate.Month(), rm.TargetDate.Day(), 0, 0, 0, 0, now.Location())
	s.DaysRemaining = int(to.Sub(from).Hours() / 24)

	if s.DaysRemaining > 0 {
		s.ItemsPerDay = float64(s.ItemsRemaining) / float64(s.DaysRemaining)
	}
	return s
}
