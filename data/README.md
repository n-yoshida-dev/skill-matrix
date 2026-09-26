# data

ロードマップ・判定・理解度の JSON。形の正本は `SPEC.md` §2（roadmap）・§3.3（judgments）・§3.4（state）・§8.3（settings）。

| ファイル | 役割 | 誰が書くか |
|---|---|---|
| `roadmap.json` | ロードマップ（マスタ） | 人（AI に手伝わせてよい） |
| `judgments/` | 判定。学習ログ 1 件 = 1 ファイル。追記のみ | AI（訂正は人が `source: "manual"` のファイルを足す） |
| `state.json` | 理解度。判定を順に適用した結果 | CLI の `recalc` だけ。手で編集しない |
| `settings.json` | 検証ルール・鮮度・重みの設定 | 人 |

## 今はダミー（2026-09-25）

画面の土台（TODO 2-5）を実物のロードマップ（TODO 2-1 段階 D）より先に作ったため、
`roadmap.json` は `backend/testdata/roadmap-sample.json` と同じ内容、`state.json` は手書きのダミー。
`judgments/` は空。**実際の学習記録ではない。**

- `roadmap.json` は段階 D で実物に差し替える
- `state.json` は手書き。CLI（段階 C-2）はできたので、CI に `verify` を入れるタスクで `recalc` から作り直す
  （`go -C backend run ./cmd/skillmatrix recalc --data ../data`。`judgments/` が空なので全項目 0 になる）
