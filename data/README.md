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

## 今の状態（2026-09-27）

- `roadmap.json` は実物（8 分野・46 項目）。項目の key・分野・`moduleRefs` は `docs/skill-map-migration.md` §8.3 の対応表から起こした。
  `backend/testdata/roadmap-sample.json`（テスト用のダミー）とは別物
- `judgments/` には段階 F の移行判定 7 本（`2026-09-27-migration-<分野>.json`、計 41 件。`source: "migration"`）が入っている。
  凍結した `skill-map.md`（study `284c0f1`）の写しで、`state.json` は `docs/skill-map-migration.md` §8.4 の期待表と一致する。以後の判定は学習ログから足す
- `verifyBy` は §8.4 の状態から見た「次の印の取り方」で書いてある。印が付いて段が上がったら書き換える

`roadmap.json` の文字列は `CLAUDE.md`「守ること」の roadmap の縛りに従う（技術的な事実だけを書く）。
`verifyBy` は、その項目で**次に付く印**を得るための技術的な検証条件を書く。印が付いて段が上がったら、次の段の条件に書き換える。
