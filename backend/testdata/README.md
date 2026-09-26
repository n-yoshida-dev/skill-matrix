# testdata

テストと開発で使うダミーデータ。**実データは一切置かない**（`../../CLAUDE.md`）。

| ファイル | 中身 |
|---|---|
| `roadmap-sample.json` | 正常なロードマップ。検査でエラーも警告も出ない状態 |
| `roadmap-broken.json` | 壊れたロードマップ。検査が問題を全部集めて返すことの確認用 |
| `learning-logs.json` | ダミーの学習ログ。AI 判定（SPEC.md §4）の入力に使う |
| `judgments/` | ダミーの判定ファイル（`schemaVersion: 2`。SPEC.md §3.3）。`roadmap-sample.json` の項目に対するもの |
| `state-sample.json` | `judgments/` を `recalc` した結果の期待値。CLI のテスト（`cmd/skillmatrix`）が突き合わせる |

`judgments/` は、根拠の種類ごとの印・梯子の範囲への切り詰め（V5）・不合格の報告（V6）・確信度の低い保留（V7）・
`manual` の判定・`occurredAt` の指定・「適用前のレベルより低い印では最終根拠日を動かさない」を一通り含む。
棄却（V1〜V3・V8・形の崩れ）は含めない（`verify` が通る状態を保つため。棄却はテストの中で作る）。
`state-sample.json` を作り直すときは `go -C backend test ./cmd/skillmatrix -run TestRecalc_testdata -update` を走らせ、`git diff` で中身を読む。

`learning-logs.json` は本人の実際の学習記録ではない。判定ロジックの確認に必要な
「根拠が強いログ」「読んだだけのログ」「自己申告だけのログ」を意図的に混ぜてある。

フォーマットの正本は `SPEC.md` §2、検査の実装は `backend/internal/roadmap/`。
