# 引き継ぎ

**次のセッションが最初に読むファイル。** 区切りで Claude が `apps-workflow:handoff` を呼んで更新する。
書くのは「今どこにいるか」と「次の一手」だけ。履歴と決定事項は積まない（行き先は末尾の表）。

## 0. 進め方のルール（最優先）

**設計・仕様・技術判断を提示するときは、ジュニアレベルでも分かる解説を先に付けること。**
専門用語は初出時に「何のためにそうするのか」を添え、抽象的な説明の前に具体例を出す。
承認を求めるのはその後。**中身が分からないまま承認させない。**

本人の自己申告：「作りたいアプリの最終イメージは持っているが、技術的な設計判断は曖昧」。
実務は Java / Spring Boot なので DB 設計・REST API・OpenAPI・JUnit は通じる。Go と React は学習中。

**テストコードを「仕様の確認材料」として示すときは、コードの読み方を教えない。**
「何を保証しているか」を日本語の箇条書きで列挙する。

**長い返答は読まれない（2026-09-25 に 2 回）。** 設計の議論でも結論と表を先に、詳細はファイルへ。
ChatGPT に渡す文章を求められたら、コードブロック 1 つでそのまま貼れる形にする。

**同じ作業ディレクトリを別セッションと同時に使わない。** 2026-09-25 に別セッションが同じチェックアウトで main を進め、
ローカルブランチが消えるなど衝突の余地があった（2026-09-26 にも再発）。着手前に `git log --oneline -3` と `git branch` で状態を見る。
ブランチの先頭が想定と違えば `git reflog --date=iso` で誰がいつ動かしたかを確かめる。

## 進捗

```
進捗（TODO.md より自動集計・2026-09-29）

████████████████████ 100%  残り  0 / 11  フェーズ1：設計（完了）
██████████████████░░  90%  残り  4 / 44  フェーズ2：v1 実装（自分専用版。ゴールは「画面で自分のマトリクスが見える」こと）
██████████████████░░  92%  残り  4 / 55  合計
あなたの回答待ち：8 件（回答済み 4 件）
保留（合計に含めない）：19 件（済み 15 件）

前回の区切り（2026-09-28）から：完了 +12 件、新たに見つかったタスク +2 件
```

## 1. 現在地

`main` はクリーン。作業中のブランチは無い。

- **フェーズ 2 のうち Claude が進められるものは終わった。** 画面は公開ビュー（`#/`）と作業ビュー（`#/plan`：ダッシュボード・学習パスの一列／ツリー・項目詳細）が実データで動く。
  E2E（Playwright）が CI の frontend ジョブで走る。GitHub Pages の公開ワークフローはあるが、Pages が無効なので公開は飛ばしている（緑）
- 段階 G（正本の切り替え）は済み。study の `go-react/logs/` から最初の通常判定 2 本（`react-hooks` 0→1）。`react-data-fetching` の 3 件は保留
- **残り 4 件はどれも本人の判断・操作が先**：重みの調整（Top 5 の並びを本人が見る）／Pages を有効にする（本人の操作）／SPEC を実装に追い付かせる（本人の了承が先）／
  react-01 の記録から `react-render-flow`・`react-jsx-ts-basics` を追い付かせる（判定がほぼ保留になり、本人が採否を決める作業。本人と一緒に進める）
- **本人は、前回の返答で頼んだ 5 件（確認待ちの上 4 件＋Pages の有効化）に、まだ一つも手をつけていない。** ダッシュボードも起動していない
- リポジトリは Public。`main` はブランチ保護あり（PR 経由のみ・CI の 5 ジョブが必須）。細部は既存方針と整合する範囲で Claude が判断してよい（2026-09-25）

## 2. 次セッションで最初にやること

**本人へのヒアリングと作業のお願いから始める。** 2026-09-29 本人：「次のセッションでは、私の確認・操作・判断をするところから始める。なので、ヒアリングとか作業指示とかを出すところから始めて」

1. 最初の返答で、画面とダッシュボードの開き方を本人に渡す（下の「動作確認コマンド」の 2 つ。コピペで動く形・URL 付き）
2. TODO.md「確認待ち」の上 4 件（付箋 `ops-dhj.8`・`.9`・`.11`・`.12`）と、Pages の有効化（`ops-dhj.4.1`。
   https://github.com/n-yoshida-dev/skill-matrix/settings/pages ）、Top 5 の並び（`ops-dhj.10`）を、**1 メッセージ 1 件ずつ**聞く。
   順番は、画面を見る 2 件（`.8` 見せ方 → `.10` Top 5）→ 文面の `.9` → 操作の `.4.1` → `.11` → `.12`。
   各件は「何のためか」を先に噛み砕き、選択肢とおすすめを添える。Pages の前には「9/28 の判定 8 件は根拠の全文をまだ本人が読んでいない。有効にする前に読むか」も聞く
3. 答えをもらったものから Claude が片付ける（SPEC への写し・`verifyBy`・重み・personal-ai-context の push など）。答えが無いものは付箋に残す

## 3. 動作確認コマンド

```bash
# 開発ダッシュボード（進捗・人間待ち・CI・今のタスクを 1 画面で）。ブラウザが開き、画面の「更新」ボタンで最新にできる
node dashboard/update.mjs --serve --open

# 純粋関数と CLI のテスト（DB 不要）
go -C backend test ./internal/... ./cmd/skillmatrix -cover

# 判定から state.json を作り直す／コミット済みの state.json が最新か確かめる（CI 用）
go -C backend run ./cmd/skillmatrix recalc --data ../data
go -C backend run ./cmd/skillmatrix verify --data ../data

# store を触ったら DB テストも回す（ローカルでは Postgres 無しだとスキップされ、CI だけが落ちる。KNOWLEDGE.md 2026-09-26）
docker compose up -d postgres
TEST_DATABASE_URL='postgres://skillmatrix:skillmatrix@localhost:5432/skillmatrix?sslmode=disable' go -C backend test ./internal/store/
docker compose stop postgres

# 画面（frontend）。http://localhost:5173/#/ が公開ビュー、#/plan が作業ビュー。JSON を変えると自動で作り直される
(cd frontend && npm ci && npm run dev)
# CI と同じ検査（E2E は Playwright。初回は npx playwright install chromium）
(cd frontend && npm run format:check && npm run lint && npm run typecheck && npx vitest run && npm run build && npm run e2e)
```

## ここに書かないもの（行き先）

| 書きたくなったこと | 行き先 |
|---|---|
| ユーザーと合意した判断 | `logs/decisions.md`（設計・方針を提案する前に読む） |
| 設計判断の理由・ハマりどころ | `KNOWLEDGE.md`（**方針を変えたくなったら必ず先に読む**） |
| やること・確認待ち | `TODO.md` |
| やったこと | `git log` と PR |
