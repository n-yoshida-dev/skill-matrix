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

## 進捗

```
進捗（TODO.md より自動集計・2026-09-19）

████████████████████ 100%  残り  0 / 11  フェーズ1：設計（完了）
█████████░░░░░░░░░░░  46%  残り 24 / 45  フェーズ2：v1 実装
░░░░░░░░░░░░░░░░░░░░   -   （タスク未定義）  フェーズ3：v2 以降
███████████░░░░░░░░░  57%  残り 24 / 56  合計（ほかに未定義のフェーズ 1）
あなたの回答待ち：0 件（回答済み 1 件）

前回の区切り（2026-09-19）から：完了 +0 件、新たに見つかったタスク +0 件
```

## 1. 現在地

**フェーズ2 の 2-1（土台）・2-2（理解度モデル）・2-3（ロードマップ）が完了。
作業中のブランチは無い（どこまで取り込んだかは `git log` を見る）。次は 2-4（AI 判定）。**

バックエンドは、GitHub ログイン → ロードマップのインポートと CRUD までが実機で動く（フロントエンドは未着手）。
`docker compose up -d --build backend` で DB 起動 → マイグレーション → サーバ起動まで進む。
CI（backend の gofmt / vet / test / build、frontend の lint / typecheck / test / build、秘密情報スキャン）は `main` と全 PR で走る。

- `SPEC.md` — 実装が参照する正本。`docs/spec-guide.md` — 人間向けの解説（本人はこちらを読む）
- `backend/internal/` — `domain`（理解度モデルの純粋関数）/ `roadmap`（マスタ JSON の検査）/ `config` /
  `store`（PostgreSQL。DB テストあり）/ `httpapi`（chi、OAuth、セッション、ロードマップ API）
- `backend/testdata/` — ダミーのロードマップとダミー学習ログ。**実データは置かない**

## 2. 次セッションで最初にやること

**2-4 の先頭＝`internal/llm` のインタフェースと stub プロバイダ**（`LLM_PROVIDER=stub`。開発中の課金ゼロとテストの決定性のため）。
着手前に SPEC.md §4 と `logs/decisions.md` を読み、設計を初心者向けに説明してから実装する（§0）。

## 3. 動作確認コマンド

```bash
# 純粋関数のテスト（DB 不要）
go -C backend test ./internal/... -cover

# DB を使うテスト（store）。postgres を起動してから。TEST_DATABASE_URL が未設定ならスキップされる
docker compose up -d postgres
TEST_DATABASE_URL='postgres://skillmatrix:skillmatrix@localhost:5432/skillmatrix?sslmode=disable' \
  go -C backend test ./internal/store/ -v

# サーバ起動（backend/.env を用意してから。.env はローカルにしか無く、コミットしていない）
docker compose up -d --build backend      # DB 起動 → マイグレーション → サーバ起動まで1コマンド
set -a; source backend/.env; set +a; go -C backend run ./cmd/server   # Docker を使わない場合

# Claude が実機確認するとき（.env は Claude から読めない）：ダミー値で起動する。
# 手順は KNOWLEDGE.md 2026-09-14（go run）と 2026-09-18（compose）

# やり直す
docker compose down -v
```

## ここに書かないもの（行き先）

| 書きたくなったこと | 行き先 |
|---|---|
| ユーザーと合意した判断 | `logs/decisions.md`（設計・方針を提案する前に読む） |
| 設計判断の理由・ハマりどころ | `KNOWLEDGE.md`（**方針を変えたくなったら必ず先に読む**） |
| やること・確認待ち | `TODO.md` |
| やったこと | `git log` と PR |
