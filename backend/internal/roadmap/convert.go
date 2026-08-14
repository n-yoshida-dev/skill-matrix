package roadmap

import (
	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
)

// このファイルは「インポート用の形」と「計算用の形」をつなぐ。
//
// 2つの型を分けているのは、責務が違うため。
//   - roadmap.Document は JSON の形そのまま。文字列の key を持ち、検査の対象になる
//   - domain.Roadmap は計算のための形。集計・優先度・学習パスの純粋関数が受け取る
//
// 分けずに 1 つの型で兼ねると、domain 側の関数が「未検査の JSON かもしれない値」を
// 受け取ることになり、純粋関数の前提（値は検証済み）が崩れる。

// ToDomain は Document を domain.Roadmap に変換する。
//
// **検査を通った Document に対して呼ぶこと。** Validate がエラーを返した Document を
// 渡した場合の結果は保証しない（存在しない key への依存がそのまま残る）。
//
// 並び順（OrderIndex）は JSON に書かれた順をそのまま採用する。
// マスタ JSON を書く人が並べた順が、そのまま画面の並びになる。
// TargetDate は JSON に含まれない（自分用ロードマップの属性なので API 側で設定する）。
func (d *Document) ToDomain() domain.Roadmap {
	domains := make([]domain.Domain, 0, len(d.Domains))

	for di, dom := range d.Domains {
		items := make([]domain.Item, 0, len(dom.Items))
		for ii, it := range dom.Items {
			items = append(items, domain.Item{
				Key:         domain.ItemKey(it.Key),
				DomainKey:   domain.DomainKey(dom.Key),
				Name:        it.Name,
				Description: it.Description,
				Outcome:     it.Outcome,
				VerifyBy:    it.VerifyBy,
				DependsOn:   toItemKeys(it.DependsOn),
				OrderIndex:  ii,
			})
		}

		domains = append(domains, domain.Domain{
			Key:        domain.DomainKey(dom.Key),
			Name:       dom.Name,
			Goal:       dom.Goal,
			Items:      items,
			OrderIndex: di,
		})
	}

	return domain.Roadmap{
		Name:    d.Name,
		Domains: domains,
	}
}

// toItemKeys は文字列の並びを ItemKey の並びに変える。
// 依存が無い項目では nil を返す（空スライスと区別する必要はない）。
func toItemKeys(keys []string) []domain.ItemKey {
	if len(keys) == 0 {
		return nil
	}
	out := make([]domain.ItemKey, len(keys))
	for i, k := range keys {
		out[i] = domain.ItemKey(k)
	}
	return out
}
