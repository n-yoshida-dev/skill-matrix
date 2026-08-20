package roadmap

import (
	"os"
	"testing"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
)

// このテストは「インポートした JSON が、計算に使える形にそのまま渡せるか」を確かめる。
// ここが噛み合っていないと、インポートは成功したのに学習パスや集計が空になる。

func TestToDomain(t *testing.T) {
	data, err := os.ReadFile(testdataPath("roadmap-sample.json"))
	if err != nil {
		t.Fatalf("サンプルを読めない: %v", err)
	}
	doc, res := ParseAndValidate(data)
	if doc == nil || !res.OK() {
		t.Fatalf("サンプルの読み込みに失敗した: %+v", res.Errors())
	}

	rm := doc.ToDomain()

	t.Run("名前と件数が保たれる", func(t *testing.T) {
		if rm.Name != doc.Name {
			t.Errorf("名前 = %q, 期待 %q", rm.Name, doc.Name)
		}
		if len(rm.Domains) != len(doc.Domains) {
			t.Errorf("分野数 = %d, 期待 %d", len(rm.Domains), len(doc.Domains))
		}
		if got := len(rm.AllItems()); got != doc.TotalItems() {
			t.Errorf("項目数 = %d, 期待 %d", got, doc.TotalItems())
		}
	})

	t.Run("並び順は JSON に書かれた順", func(t *testing.T) {
		for i, d := range rm.Domains {
			if d.OrderIndex != i {
				t.Errorf("分野 %q の OrderIndex = %d, 期待 %d", d.Key, d.OrderIndex, i)
			}
			for j, it := range d.Items {
				if it.OrderIndex != j {
					t.Errorf("項目 %q の OrderIndex = %d, 期待 %d", it.Key, it.OrderIndex, j)
				}
			}
		}
	})

	t.Run("項目が所属する分野の key を持つ", func(t *testing.T) {
		// domain 側の集計（RollupDomain）はこの値で分野をまとめる
		for _, d := range rm.Domains {
			for _, it := range d.Items {
				if it.DomainKey != d.Key {
					t.Errorf("項目 %q の DomainKey = %q, 期待 %q", it.Key, it.DomainKey, d.Key)
				}
			}
		}
	})

	t.Run("依存関係が引き継がれる", func(t *testing.T) {
		item, ok := findItem(rm, "go-04")
		if !ok {
			t.Fatal("go-04 が見つからない")
		}
		want := []domain.ItemKey{"go-02", "go-03"}
		if len(item.DependsOn) != len(want) {
			t.Fatalf("依存 = %v, 期待 %v", item.DependsOn, want)
		}
		for i, k := range want {
			if item.DependsOn[i] != k {
				t.Errorf("依存[%d] = %q, 期待 %q", i, item.DependsOn[i], k)
			}
		}
	})

	t.Run("依存が無い項目は nil", func(t *testing.T) {
		item, ok := findItem(rm, "go-01")
		if !ok {
			t.Fatal("go-01 が見つからない")
		}
		if item.DependsOn != nil {
			t.Errorf("依存 = %v, 期待 nil", item.DependsOn)
		}
	})

	t.Run("到達状態と確認方法が引き継がれる", func(t *testing.T) {
		item, ok := findItem(rm, "go-01")
		if !ok {
			t.Fatal("go-01 が見つからない")
		}
		if item.Outcome == "" {
			t.Error("Outcome が落ちている")
		}
		if item.VerifyBy == "" {
			t.Error("VerifyBy が落ちている")
		}
		if rm.Domains[0].Goal == "" {
			t.Error("Goal が落ちている")
		}
	})

	t.Run("変換後のロードマップで学習パスを組める", func(t *testing.T) {
		// 変換の目的はこれ。依存関係が壊れていればここで並べられない
		states := map[domain.ItemKey]domain.ItemState{}
		path := domain.BuildPath(rm.Domains[0], states)
		if len(path.Nodes) != 5 {
			t.Fatalf("パスの項目数 = %d, 期待 5", len(path.Nodes))
		}
		// 依存先が必ず先に並ぶ
		seen := map[domain.ItemKey]bool{}
		for _, n := range path.Nodes {
			for _, dep := range n.Item.DependsOn {
				if !seen[dep] {
					t.Errorf("項目 %q が依存先 %q より先に並んでいる", n.Item.Key, dep)
				}
			}
			seen[n.Item.Key] = true
		}
	})
}

// findItem は変換後のロードマップから項目を1つ探す。
func findItem(rm domain.Roadmap, key domain.ItemKey) (domain.Item, bool) {
	for _, it := range rm.AllItems() {
		if it.Key == key {
			return it, true
		}
	}
	return domain.Item{}, false
}
