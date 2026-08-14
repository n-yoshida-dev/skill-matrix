# testdata

テストと開発で使うダミーデータ。**実データは一切置かない**（`../../CLAUDE.md`）。

| ファイル | 中身 |
|---|---|
| `roadmap-sample.json` | 正常なロードマップ。検査でエラーも警告も出ない状態 |
| `roadmap-broken.json` | 壊れたロードマップ。検査が問題を全部集めて返すことの確認用 |
| `learning-logs.json` | ダミーの学習ログ。AI 判定（SPEC.md §4）の入力に使う |

`learning-logs.json` は本人の実際の学習記録ではない。判定ロジックの確認に必要な
「根拠が強いログ」「読んだだけのログ」「自己申告だけのログ」を意図的に混ぜてある。

フォーマットの正本は `SPEC.md` §2、検査の実装は `backend/internal/roadmap/`。
