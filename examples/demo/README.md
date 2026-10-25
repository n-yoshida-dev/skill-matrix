# 見本のデータ（架空の学習者）

見本の画面のためのデータ。**架空の学習者のもので、作者の実データではない。** 形は本物の `data/` と同じ（[`data/README.md`](../../data/README.md)）。

画面の見どころが全部出るように組んである。

| 見どころ | 項目の例 |
|---|---|
| 段の違う分野（分野の最高段が 2〜5） | CI とデプロイ（5）、React・Go の API（4）、TypeScript（3）、データベース（2） |
| 段の違う項目（Lv 0〜5） | `delivery-git-flow`（5）、`react-components`（4）、`react-state-hooks`（3）、`db-schema-design`（2）、`db-sql-basics`（1） |
| 実装したが基礎が未確認（印が 3 だけ） | `react-routing`、`go-testing`、`delivery-docker` |
| 要再確認（不合格の報告） | `react-data-fetching` |
| 根拠が古い | `db-sql-basics`、`db-schema-design`（2026-05 の根拠） |
| 段階前の状態 | 学習中 `go-concurrency`、説明を受けただけ `ts-generics`、自己申告 `delivery-deploy` |
| 保留（確信度が低い） | `db-index` |
| 資格（自己申告） | データベース、CI とデプロイ |

`evidenceRefs` は架空の `log:learning-logs/...` を指していて、ログの本文は無い。

## 更新のしかた

本物の `data/` と同じく、判定を足したら `recalc` で `state.json` を作り直す。CI の「data/ の検証（verify）」がこのディレクトリも見る。

```bash
go -C backend run ./cmd/skillmatrix recalc --data ../examples/demo/data
go -C backend run ./cmd/skillmatrix verify --data ../examples/demo/data
```

本物と違い、見本なので判定ファイルを書き換えてよい（`data/judgments/` の「追記のみ」は本物の記録を守る決まり）。
