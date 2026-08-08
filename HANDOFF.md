# 引き継ぎ

`/handoff` で更新する。**次のセッションが最初に読むファイル。**

## 1. 現在地

`~/workspace/apps/` 直下のセッションで、テンプレート（app-template）から初期化した直後。
`PLAN.md` の初版まで書いた。frontend（Vite + React + TS）と backend（go mod）の骨組みだけある状態で、
アプリのコードはまだ何も書いていない。

## 2. 直近でやったこと

- `gh repo create skill-matrix --template n-yoshida-dev/app-template --private --clone`
- 雛形の穴埋め（CLAUDE.md / README.md / PLAN.md）、フックへの実行権限付与
- `frontend/` を Vite の react-ts テンプレートで初期化、`backend/` を `go mod init` で初期化
- SETUP.md を削除

## 3. 確定している決定事項

- リポジトリ名は `skill-matrix`（private）
- スタックは共通ルールどおり React + TS + Vite / Go / PostgreSQL
- **AI 判定は Claude API をバックエンド（Go）から呼ぶ。** フロントに API キーを置かない
- **ロードマップはマスタ（正）**。学習ログの判定でマスタ自体は変化しない
- 判定結果はイベントとして履歴に残し、現在の理解度は導出値とする

## 4. 未確定・確認待ち

`PLAN.md` の「未確定」節に一覧がある。特に大きいのは次の3つ。

- 理解度のスケールと色の濃度への割り当て
- 時間経過による減衰を入れるか
- ローカル完結で始めるか、最初から PostgreSQL + サーバ構成にするか

## 5. 次セッションのタスク

1. `PLAN.md` の「未確定」をユーザーと詰める
2. 詰まったものから `SPEC.md`（データモデル・画面・API）に落とす
