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
進捗（TODO.md より自動集計・2026-09-27）

████████████████████ 100%  残り  0 / 11  フェーズ1：設計（完了）
█████████████░░░░░░░  65%  残り 14 / 41  フェーズ2：v1 実装（自分専用版。ゴールは「画面で自分のマトリクスが見える」こと）
██████████████░░░░░░  73%  残り 14 / 52  合計
あなたの回答待ち：4 件（回答済み 4 件）
保留（合計に含めない）：19 件（済み 15 件）

前回の区切り（2026-09-27）から：完了 +2 件、新たに見つかったタスク +0 件
```

## 1. 現在地

`main` はクリーン。作業中のブランチは無い。

- **段階 F まで済み。** `data/judgments/` に移行判定 7 本（41 件）。`state.json` は移行計画 §8.4 の期待表と 46 項目一致（レベル 3 が 2・1 が 8・0 が 36）。
  凍結コミットは study の `284c0f1`。study 側はもう skill-map を更新せず、習熟度は「skill-matrix の判定で拾う」運用に切り替え済み
- **学習ログから判定を作る入口ができた**（指示書 `prompts/judge.md`・スキル `/judge-log`。PR #58・#59）。本物の `/judge-log` は呼べて、今は「未判定 0 件」で止まる。
  study の `*/logs/*.md` 30 件はすべて凍結前で、自動の探索では「移行で写し済み」として外れるため。凍結後の学習（react-02 など）は progress.md に溜まっていて、まだログの形になっていない
- **リポジトリは Public**（2026-09-27）。`main` はブランチ保護あり（PR 経由のみ・CI の 5 ジョブが必須・管理者は迂回可）
- CI の独立ジョブ「data/ の検証（verify）」が全 PR と main で `verify` を走らせる。`sources.local.json` は手元にだけある（study のパス入り）
- **画面は公開ビュー（`/`）まで動き、実データで描ける**（「46 項目のうち根拠のある項目が 23」）。作業ビュー（`/plan`）は土台だけ
- 以後の細部は既存方針と整合する範囲で Claude が判断してよい。止めて確認するのは重大な矛盾・データ損失・公開情報上の問題だけ（2026-09-25 ユーザー指示）

## 2. 次セッションで最初にやること

**2026-09-28 以降なら TODO 2-1 段階 G、それより前なら TODO 2-5 のマトリクス画面（作業ビュー。SPEC §7.1）。**

段階 G「正本の切り替えを終える」の完了条件は TODO.md の該当行。study に `go-react/logs/` を作り、凍結後の学習（react-02 など）をログ 1 件にしてコミットしてから、`/judge-log` で最初の通常判定を作る。
**判定ファイルは 2026-09-28 以降に書く**（同じ日付だと移行ファイルより先に適用される。KNOWLEDGE.md 2026-09-27「`prompts/judge.md`」）。
凍結前にしか根拠が無い分（React の起動フローなど）は、`/judge-log <ファイル>` で指定すれば台帳に無い出来事だけを判定できる。

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

# 画面（frontend）。#/ が公開ビュー、#/plan が作業ビュー
cd frontend && npm ci && npm run dev
# CI と同じ検査
cd frontend && npm run format:check && npm run lint && npm run typecheck && npx vitest run && npm run build
```

## ここに書かないもの（行き先）

| 書きたくなったこと | 行き先 |
|---|---|
| ユーザーと合意した判断 | `logs/decisions.md`（設計・方針を提案する前に読む） |
| 設計判断の理由・ハマりどころ | `KNOWLEDGE.md`（**方針を変えたくなったら必ず先に読む**） |
| やること・確認待ち | `TODO.md` |
| やったこと | `git log` と PR |
