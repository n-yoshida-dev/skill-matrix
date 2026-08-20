package domain

import (
	"sort"
	"time"
)

// このファイルは「この先どう進むか」を組み立てる処理を持つ。
//   - NextActions: 次にやること Top N（SPEC.md §5.1）
//   - BuildPath  : 学習パス。依存関係の順に一列で並べる（SPEC.md §7.2）
//
// どちらも同じ優先度の計算を使う。2つの画面で「今やるべきこと」が
// 食い違わないようにするため。

// staleBonus は鮮度から優先度の加点を返す。古いものほど再確認を促す。
func staleBonus(s StalenessLevel) float64 {
	switch s {
	case StalenessStale:
		return 1.0
	case StalenessAging:
		return 0.5
	default:
		return 0
	}
}

// dependentCounts は「その項目を前提として挙げている項目」の数を数える。
// これが多いほど、終わらせたときに開く道が多い。
func dependentCounts(items []Item) map[ItemKey]int {
	counts := make(map[ItemKey]int, len(items))
	for _, it := range items {
		for _, dep := range it.DependsOn {
			counts[dep]++
		}
	}
	return counts
}

// stateOf は項目の現在の状態を返す。まだ判定されていない項目は未着手として扱う。
// 保存値が範囲外でも後続の計算が壊れないように丸める。
func stateOf(states map[ItemKey]ItemState, k ItemKey) ItemState {
	st, ok := states[k]
	if !ok {
		st = ItemState{ItemKey: k}
	}
	if st.Level < LevelNone {
		st.Level = LevelNone
	} else if st.Level > MaxLevel {
		st.Level = MaxLevel
	}
	return st
}

// isReady は依存項目がすべてレベル1以上に達しているか（＝着手できるか）を返す。
func isReady(states map[ItemKey]ItemState, it Item) bool {
	for _, dep := range it.DependsOn {
		if stateOf(states, dep).Level < LevelBasicConfirmed {
			return false
		}
	}
	return true
}

// isDone は「済」とみなせるか。学習パスの ✓ と同じ基準（レベル1以上＝一度は根拠が付いた）。
func isDone(states map[ItemKey]ItemState, it Item) bool {
	return stateOf(states, it.Key).Level >= LevelBasicConfirmed
}

// scorer は優先度の計算に必要なものをまとめた内部の入れ物。
type scorer struct {
	states  map[ItemKey]ItemState
	now     time.Time
	cfg     StalenessConfig
	w       Weights
	deps    map[ItemKey]int
	maxDeps int
}

func newScorer(items []Item, states map[ItemKey]ItemState, now time.Time, cfg StalenessConfig, w Weights) scorer {
	deps := dependentCounts(items)
	maxDeps := 0
	for _, c := range deps {
		if c > maxDeps {
			maxDeps = c
		}
	}
	return scorer{
		states:  states,
		now:     now,
		cfg:     cfg.withDefaults(),
		w:       w,
		deps:    deps,
		maxDeps: maxDeps,
	}
}

func (s scorer) state(k ItemKey) ItemState { return stateOf(s.states, k) }
func (s scorer) ready(it Item) bool        { return isReady(s.states, it) }

// score は優先度を計算する（SPEC.md §5.1）。
//
// 目標日までの逼迫度は入れない。ロードマップ全体で同じ値になり、
// 全項目に同じ数を足すだけで並び順が変わらないため。逼迫度は BuildSchedule が別に返す。
func (s scorer) score(it Item) float64 {
	st := s.state(it.Key)

	var readiness float64
	if s.ready(it) {
		readiness = 1
	}

	gap := float64(MaxLevel-st.Level) / float64(MaxLevel)

	bonus := staleBonus(Staleness(s.now, st.LastEvidenceAt, s.cfg))
	if st.NeedsReview {
		bonus = 1.0
	}

	var unlocks float64
	if s.maxDeps > 0 {
		unlocks = float64(s.deps[it.Key]) / float64(s.maxDeps)
	}

	return s.w.Readiness*readiness +
		s.w.Gap*gap +
		s.w.Staleness*bonus +
		s.w.Unlocks*unlocks
}

// NextActions は「次にやること」を優先度順に最大 n 件返す。
//
// 並び順は「着手できないものを常に後ろ」→「優先度の降順」→「ロードマップの定義順」。
// 着手できないものを数値だけで後ろに送ると重み次第で前に出てしまうため、
// 順序の条件として明示的に分けている。
//
// これ以上やることが無い項目（最大レベル）は出さない。ただし要再確認のものは、
// 再確認を促すために出す。
func NextActions(rm Roadmap, states map[ItemKey]ItemState, now time.Time, cfg StalenessConfig, w Weights, n int) []Action {
	if n <= 0 {
		return nil
	}
	sc := newScorer(rm.AllItems(), states, now, cfg, w)

	// seq はロードマップ上の定義順。優先度が並んだときの安定した順序に使う。
	type entry struct {
		action Action
		seq    int
	}
	var list []entry

	seq := 0
	for _, d := range rm.Domains {
		for _, it := range d.Items {
			seq++
			st := sc.state(it.Key)
			if st.Level >= MaxLevel && !NeedsAttention(st, now, sc.cfg) {
				continue
			}
			list = append(list, entry{
				action: Action{
					ItemKey:   it.Key,
					DomainKey: d.Key,
					Name:      it.Name,
					Outcome:   it.Outcome,
					VerifyBy:  it.VerifyBy,
					Level:     st.Level,
					Staleness: Staleness(now, st.LastEvidenceAt, sc.cfg),
					Priority:  sc.score(it),
					Blocked:   !sc.ready(it),
				},
				seq: seq,
			})
		}
	}

	sort.SliceStable(list, func(i, j int) bool {
		if list[i].action.Blocked != list[j].action.Blocked {
			return !list[i].action.Blocked
		}
		if list[i].action.Priority != list[j].action.Priority {
			return list[i].action.Priority > list[j].action.Priority
		}
		return list[i].seq < list[j].seq
	})

	if len(list) > n {
		list = list[:n]
	}
	out := make([]Action, 0, len(list))
	for _, e := range list {
		out = append(out, e.action)
	}
	return out
}

// BuildPath は分野1つの学習パスを組み立てる（SPEC.md §7.2）。
//
// 依存関係は有向グラフだが、図としては描かずトポロジカルソートで一列に潰す。
// 項目が増えても読めることを優先するため。
//
// 「今ここ」は**依存順に見て最初に現れる、まだ済んでいない着手可能な項目**。
// 道のりのどこまで到達したかを示す先端であって、優先度とは無関係に決まる。
//
// そのため NextActions の1位とは一致しないことがある。両者は別の問いに答えている。
//
//	学習パスの「今ここ」 : 道のりのどこまで来たか（レベル1に達した項目の先端）
//	次にやること の1位   : 今いちばん時間を使うべき項目（済んだ項目の深掘りも含む）
//
// 例：go-01 がレベル1に達していれば、パス上では「済」で先端は go-02 に進む。
// 一方 NextActions は go-01 をレベル2へ引き上げることを1位に挙げうる。
// どちらも正しい。画面ではこの違いが分かるように出すこと。
func BuildPath(d Domain, states map[ItemKey]ItemState) Path {
	ordered := topoSort(d.Items)

	p := Path{
		DomainKey:  d.Key,
		DomainName: d.Name,
		Goal:       d.Goal,
		TotalCount: len(d.Items),
	}

	var currentKey ItemKey
	for _, it := range ordered {
		if !isDone(states, it) && isReady(states, it) {
			currentKey = it.Key
			break
		}
	}

	p.Nodes = make([]PathNode, 0, len(ordered))
	for _, it := range ordered {
		status := PathUpcoming
		switch {
		case isDone(states, it):
			status = PathDone
			p.DoneCount++
		case it.Key == currentKey:
			status = PathCurrent
		}
		p.Nodes = append(p.Nodes, PathNode{
			Item:   it,
			State:  stateOf(states, it.Key),
			Status: status,
		})
	}
	return p
}

// topoSort は依存関係の順に項目を並べる（トポロジカルソート）。
//
//   - 同じ順位のものは定義順を保つ
//   - 分野の外を指す依存は順序付けに使えないので無視する
//   - 循環参照が残っていた場合は、残りを定義順で末尾に付ける（無限ループを避ける）。
//     循環はインポート時に弾く決まりだが、ここで落ちないようにしておく
func topoSort(items []Item) []Item {
	index := make(map[ItemKey]int, len(items))
	for i, it := range items {
		index[it.Key] = i
	}

	inDegree := make([]int, len(items))
	children := make([][]int, len(items))
	for i, it := range items {
		for _, dep := range it.DependsOn {
			j, ok := index[dep]
			if !ok {
				continue // 分野外への依存
			}
			inDegree[i]++
			children[j] = append(children[j], i)
		}
	}

	var ready []int
	for i := range items {
		if inDegree[i] == 0 {
			ready = append(ready, i)
		}
	}

	out := make([]Item, 0, len(items))
	placed := make([]bool, len(items))
	for len(ready) > 0 {
		// 定義順で最も早いものから取り出して並びを安定させる。
		sort.Ints(ready)
		i := ready[0]
		ready = ready[1:]

		out = append(out, items[i])
		placed[i] = true
		for _, c := range children[i] {
			inDegree[c]--
			if inDegree[c] == 0 {
				ready = append(ready, c)
			}
		}
	}

	for i := range items {
		if !placed[i] {
			out = append(out, items[i])
		}
	}
	return out
}
