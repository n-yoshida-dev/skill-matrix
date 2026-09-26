# data

ロードマップ・判定・理解度の JSON。形の正本は `SPEC.md` §2（roadmap）・§3.3（judgments）・§3.4（state）・§8.3（settings）。

| ファイル | 役割 | 誰が書くか |
|---|---|---|
| `roadmap.json` | ロードマップ（マスタ） | 人（AI に手伝わせてよい） |
| `judgments/` | 判定。学習ログ 1 件 = 1 ファイル。追記のみ | AI（訂正は人が `source: "manual"` のファイルを足す） |
| `state.json` | 理解度。判定を順に適用した結果 | CLI の `recalc` だけ。手で編集しない |
| `settings.json` | 検証ルール・鮮度・重みの設定 | 人 |

## 更新のしかた

判定ファイルを `judgments/` に足したら、`recalc` で `state.json` を作り直して一緒にコミットする。
CI の「data/ の検証（verify）」が、`state.json` が再計算結果と一致しないと落ちる（`SPEC.md` §6）。

```bash
go -C backend run ./cmd/skillmatrix recalc --data ../data   # state.json を書き直す
go -C backend run ./cmd/skillmatrix verify --data ../data   # CI と同じ検査（書き出さない）
```

## 今はダミーのロードマップ（2026-09-26）

画面の土台（TODO 2-5）を実物のロードマップ（TODO 2-1 段階 D）より先に作ったため、
`roadmap.json` は `backend/testdata/roadmap-sample.json` に `moduleRefs` を足しただけのダミー。**実際の学習計画ではない。**

- `judgments/` は空。`state.json` は `recalc` の出力で、全項目がレベル 0（未着手）
- `roadmap.json` は段階 D で実物に差し替える。判定は段階 F（`skill-map.md` からの移行）で初めて入る
