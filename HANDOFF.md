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
ローカルブランチが消えるなど衝突の余地があった。着手前に `git log --oneline -3` と `git branch` で状態を見る。

## 進捗

```
進捗（TODO.md より自動集計・2026-09-26）

████████████████████ 100%  残り  0 / 11  フェーズ1：設計（完了）
█████████░░░░░░░░░░░  47%  残り 21 / 40  フェーズ2：v1 実装（自分専用版。ゴールは「画面で自分のマトリクスが見える」こと）
███████████░░░░░░░░░  58%  残り 21 / 51  合計
あなたの回答待ち：4 件（回答済み 4 件）
保留（合計に含めない）：19 件（済み 15 件）

前回の区切り（2026-09-26）から：完了 +1 件、新たに見つかったタスク +0 件
```

## 1. 現在地

`main` はクリーン（PR #44 まで）。作業中のブランチは無い。

- **段階 C-2 まで済み。** `backend/cmd/skillmatrix` に `recalc` / `verify` がある（SPEC §6）。`state.json` の書き出しの形は SPEC §3.4 末尾。
  形の検査は `internal/judgment` に移した（`internal/llm/output.go` は棚上げ中のコード用の入口だけ）
- **CI はまだ `verify` を走らせていない。** `data/state.json` は手書きのダミーのままで、`verify --data ../data` は一致しない
- **画面は公開ビュー（`/`）まで動く。** 作業ビュー（`/plan`）は土台だけ。`data/roadmap.json` は testdata と同じダミーで、実物は段階 D・F で入る
- 以後の細部は既存方針と整合する範囲で Claude が判断してよい。止めて確認するのは重大な矛盾・データ損失・公開情報上の問題だけ（2026-09-25 ユーザー指示）

## 2. 次セッションで最初にやること

**TODO 2-1「その CLI を CI で走らせる（`verify` モード）」。** 最初に `go -C backend run ./cmd/skillmatrix recalc --data ../data` で
`data/state.json` を作り直してコミットする（全項目 0 になり公開ビューのダミー表示が消える。段階 D まで。`data/README.md` の「今はダミー」節も直す）。
そのあと `.github/workflows/ci.yml` の backend ジョブに `go run ./cmd/skillmatrix verify --data ../data` を足し、
完了条件どおり「JSON を壊した PR」「`state.json` だけ古い PR」で CI が赤くなるのを一度ずつ確かめる。

その後は TODO 2-1 の順（`roadmap.json` の実物 → `sources` → study 側の凍結 → 移行判定 → 切り替え）。

## 3. 動作確認コマンド

```bash
# 開発ダッシュボード（進捗・人間待ち・CI・現在地を 1 画面で）。生成後に dashboard/index.html をブラウザで開く。--serve なら http://127.0.0.1:8787/
node dashboard/update.mjs

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
