package domain

import (
	"testing"
	"time"
)

// planRoadmap は依存関係のある少し大きめのロードマップ。
//
//	go-01 ── go-02 ── go-03
//	  └───── go-04
//	react-01（依存なし）
func planRoadmap() Roadmap {
	return Roadmap{
		Name: "テスト用",
		Domains: []Domain{
			{
				Key: "go", Name: "Go", Goal: "Go で Web API を書ける",
				Items: []Item{
					{Key: "go-01", Name: "基本構文", Outcome: "型と値の流れを追える", VerifyBy: "Tour Basics 後にドリル"},
					{Key: "go-02", Name: "インターフェース", DependsOn: []ItemKey{"go-01"}},
					{Key: "go-03", Name: "エラー処理", DependsOn: []ItemKey{"go-02"}},
					{Key: "go-04", Name: "並行処理", DependsOn: []ItemKey{"go-01"}},
				},
			},
			{
				Key: "react", Name: "React",
				Items: []Item{
					{Key: "react-01", Name: "コンポーネント"},
				},
			},
		},
	}
}

func keysOf(actions []Action) []ItemKey {
	out := make([]ItemKey, 0, len(actions))
	for _, a := range actions {
		out = append(out, a.ItemKey)
	}
	return out
}

func pathKeys(p Path) []ItemKey {
	out := make([]ItemKey, 0, len(p.Nodes))
	for _, n := range p.Nodes {
		out = append(out, n.Item.Key)
	}
	return out
}

func TestNextActions(t *testing.T) {
	now := time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)
	cfg := DefaultStalenessConfig()
	w := DefaultWeights()

	t.Run("着手できない項目は着手できる項目より必ず後ろに来る", func(t *testing.T) {
		rm := planRoadmap()
		got := NextActions(rm, nil, now, cfg, w, 10)

		// 未着手の状態では go-01 と react-01 だけが着手可能
		seenBlocked := false
		for _, a := range got {
			if a.Blocked {
				seenBlocked = true
				continue
			}
			if seenBlocked {
				t.Fatalf("着手可能な %q が着手不能な項目より後ろにある: %v", a.ItemKey, keysOf(got))
			}
		}
	})

	t.Run("道を多く開く項目が先に来る", func(t *testing.T) {
		rm := planRoadmap()
		got := NextActions(rm, nil, now, cfg, w, 2)

		// go-01 は go-02 と go-04 を開く。react-01 は何も開かない。
		if len(got) == 0 || got[0].ItemKey != "go-01" {
			t.Errorf("先頭 = %v, want go-01", keysOf(got))
		}
	})

	t.Run("件数を n 件に絞る", func(t *testing.T) {
		rm := planRoadmap()
		if got := NextActions(rm, nil, now, cfg, w, 2); len(got) != 2 {
			t.Errorf("件数 = %d, want 2", len(got))
		}
		if got := NextActions(rm, nil, now, cfg, w, 0); got != nil {
			t.Errorf("n=0 のとき = %v, want nil", got)
		}
	})

	t.Run("到達状態と次の確認方法を必ず載せる", func(t *testing.T) {
		rm := planRoadmap()
		got := NextActions(rm, nil, now, cfg, w, 1)

		if len(got) != 1 {
			t.Fatalf("件数 = %d, want 1", len(got))
		}
		if got[0].Outcome == "" || got[0].VerifyBy == "" {
			t.Errorf("Outcome=%q VerifyBy=%q, want どちらも空でないこと", got[0].Outcome, got[0].VerifyBy)
		}
	})

	t.Run("最大レベルに達した項目は出さない", func(t *testing.T) {
		rm := planRoadmap()
		states := map[ItemKey]ItemState{
			"go-01": {ItemKey: "go-01", VerifiedLevel: MaxLevel, LastEvidenceAt: now},
		}
		for _, a := range NextActions(rm, states, now, cfg, w, 10) {
			if a.ItemKey == "go-01" {
				t.Error("完了済みの go-01 が出ている")
			}
		}
	})

	t.Run("最大レベルでも要再確認なら出す", func(t *testing.T) {
		rm := planRoadmap()
		states := map[ItemKey]ItemState{
			"go-01": {ItemKey: "go-01", VerifiedLevel: MaxLevel, LastEvidenceAt: now.Add(-200 * day)},
		}
		found := false
		for _, a := range NextActions(rm, states, now, cfg, w, 10) {
			if a.ItemKey == "go-01" {
				found = true
				if a.Staleness != StalenessStale {
					t.Errorf("Staleness = %q, want %q", a.Staleness, StalenessStale)
				}
			}
		}
		if !found {
			t.Error("要再確認の go-01 が出ていない")
		}
	})

	t.Run("依存が満たされると着手可能になる", func(t *testing.T) {
		rm := planRoadmap()
		states := map[ItemKey]ItemState{
			"go-01": {ItemKey: "go-01", VerifiedLevel: LevelBasicConfirmed, LastEvidenceAt: now},
		}
		got := NextActions(rm, states, now, cfg, w, 10)

		for _, a := range got {
			if a.ItemKey == "go-02" && a.Blocked {
				t.Error("go-01 が済なのに go-02 が着手不能のまま")
			}
			if a.ItemKey == "go-03" && !a.Blocked {
				t.Error("go-02 が未達なのに go-03 が着手可能になっている")
			}
		}
	})
}

func TestBuildPath(t *testing.T) {
	now := time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)

	t.Run("定義順がばらばらでも依存関係の順に並べる", func(t *testing.T) {
		d := Domain{
			Key: "go", Name: "Go",
			Items: []Item{
				{Key: "go-03", DependsOn: []ItemKey{"go-02"}},
				{Key: "go-01"},
				{Key: "go-02", DependsOn: []ItemKey{"go-01"}},
			},
		}
		got := pathKeys(BuildPath(d, nil))
		want := []ItemKey{"go-01", "go-02", "go-03"}

		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("並び = %v, want %v", got, want)
			}
		}
	})

	t.Run("済・今ここ・この先に分類する", func(t *testing.T) {
		d := planRoadmap().Domains[0]
		states := map[ItemKey]ItemState{
			"go-01": {ItemKey: "go-01", VerifiedLevel: LevelCanExplain, LastEvidenceAt: now},
		}
		got := BuildPath(d, states)

		status := map[ItemKey]PathStatus{}
		for _, n := range got.Nodes {
			status[n.Item.Key] = n.Status
		}

		if status["go-01"] != PathDone {
			t.Errorf("go-01 = %q, want %q", status["go-01"], PathDone)
		}
		if status["go-03"] != PathUpcoming {
			t.Errorf("go-03 = %q, want %q（go-02 が未達なので着手できない）", status["go-03"], PathUpcoming)
		}

		// 今ここは1件だけ
		current := 0
		for _, s := range status {
			if s == PathCurrent {
				current++
			}
		}
		if current != 1 {
			t.Errorf("今ここ = %d 件, want 1 件", current)
		}
		if got.DoneCount != 1 || got.TotalCount != 4 {
			t.Errorf("DoneCount=%d TotalCount=%d, want 1 と 4", got.DoneCount, got.TotalCount)
		}
	})

	t.Run("全部済なら今ここは無い", func(t *testing.T) {
		d := planRoadmap().Domains[0]
		states := map[ItemKey]ItemState{}
		for _, it := range d.Items {
			states[it.Key] = ItemState{ItemKey: it.Key, VerifiedLevel: LevelBasicConfirmed, LastEvidenceAt: now}
		}
		got := BuildPath(d, states)

		for _, n := range got.Nodes {
			if n.Status != PathDone {
				t.Errorf("%q = %q, want %q", n.Item.Key, n.Status, PathDone)
			}
		}
		if got.DoneCount != 4 {
			t.Errorf("DoneCount = %d, want 4", got.DoneCount)
		}
	})

	t.Run("分野の目標を持ち回る", func(t *testing.T) {
		d := planRoadmap().Domains[0]
		if got := BuildPath(d, nil); got.Goal != d.Goal {
			t.Errorf("Goal = %q, want %q", got.Goal, d.Goal)
		}
	})

	t.Run("分野の外を指す依存は並び順に影響しない", func(t *testing.T) {
		d := Domain{
			Key: "go", Name: "Go",
			Items: []Item{
				{Key: "go-01", DependsOn: []ItemKey{"react-01"}}, // 分野外
				{Key: "go-02", DependsOn: []ItemKey{"go-01"}},
			},
		}
		got := pathKeys(BuildPath(d, nil))
		if len(got) != 2 || got[0] != "go-01" || got[1] != "go-02" {
			t.Errorf("並び = %v, want [go-01 go-02]", got)
		}
	})

	t.Run("循環参照があっても全項目を返して落ちない", func(t *testing.T) {
		d := Domain{
			Key: "go", Name: "Go",
			Items: []Item{
				{Key: "a", DependsOn: []ItemKey{"b"}},
				{Key: "b", DependsOn: []ItemKey{"a"}},
				{Key: "c"},
			},
		}
		got := BuildPath(d, nil)

		if len(got.Nodes) != 3 {
			t.Errorf("件数 = %d, want 3（循環していても全部返すこと）", len(got.Nodes))
		}
	})
}

// 学習パスの「今ここ」と「次にやること」の1位は、別の問いに答えているので一致しない。
//
//	学習パスの「今ここ」 : 道のりのどこまで来たか（レベル1に達した項目の先端）
//	次にやること の1位   : 今いちばん時間を使うべき項目（済んだ項目の深掘りも含む）
func TestBuildPathとNextActionsは別の問いに答える(t *testing.T) {
	now := time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)
	cfg := DefaultStalenessConfig()
	w := DefaultWeights()

	rm := planRoadmap()
	states := map[ItemKey]ItemState{
		"go-01": {ItemKey: "go-01", VerifiedLevel: LevelBasicConfirmed, LastEvidenceAt: now},
	}

	actions := NextActions(rm, states, now, cfg, w, 10)
	var topInGo ItemKey
	for _, a := range actions {
		if a.DomainKey == "go" && !a.Blocked {
			topInGo = a.ItemKey
			break
		}
	}

	path := BuildPath(rm.Domains[0], states)
	var current ItemKey
	for _, n := range path.Nodes {
		if n.Status == PathCurrent {
			current = n.Item.Key
		}
	}

	// go-01 はレベル1に達したので、パス上の先端は go-02 へ進む。
	if current != "go-02" {
		t.Errorf("今ここ = %q, want go-02（go-01 は済なので先端が進む）", current)
	}
	// 一方で「次にやること」は深掘りを含むので、済んだ go-01 を1位に挙げうる。
	if topInGo != "go-01" {
		t.Errorf("次にやること1位 = %q, want go-01（レベル1→2 の深掘り）", topInGo)
	}
	if topInGo == current {
		t.Error("2つが一致してしまった。別の問いに答えているので一致しないはず")
	}
}

// 「今ここ」は常に「まだ済んでいない」かつ「着手できる」項目であること。
func TestBuildPathの今ここは着手可能な未達項目(t *testing.T) {
	rm := planRoadmap()
	states := map[ItemKey]ItemState{
		"go-01": {ItemKey: "go-01", VerifiedLevel: LevelBasicConfirmed},
		"go-02": {ItemKey: "go-02", VerifiedLevel: LevelBasicConfirmed},
	}

	path := BuildPath(rm.Domains[0], states)
	for _, n := range path.Nodes {
		if n.Status != PathCurrent {
			continue
		}
		if isDone(states, n.Item) {
			t.Errorf("今ここ %q が済になっている", n.Item.Key)
		}
		if !isReady(states, n.Item) {
			t.Errorf("今ここ %q が着手できない", n.Item.Key)
		}
	}
}
