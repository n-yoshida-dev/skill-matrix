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
進捗（TODO.md より自動集計・2026-09-23）

████████████████████ 100%  残り  0 / 11  フェーズ1：設計（完了）
█████████░░░░░░░░░░░  48%  残り 25 / 49  フェーズ2：v1 実装
░░░░░░░░░░░░░░░░░░░░   -   （タスク未定義）  フェーズ3：v2 以降
███████████░░░░░░░░░  58%  残り 25 / 60  合計（ほかに未定義のフェーズ 1）
あなたの回答待ち：2 件（回答済み 4 件）

前回の区切り（2026-09-20）から：完了 +2 件、新たに見つかったタスク +4 件
```

## 1. 現在地

**2026-09-23 に方針が変わった。v1 を「公開する Web サービス」から「自分専用のツール」へ絞り込む
（`logs/decisions.md` 2026-09-23）。ドキュメントはまだ古い方針のまま。作業中のブランチは無い。**

HTTP から動くのは GitHub ログインとロードマップのインポート・CRUD まで。**ログ投稿の API はまだ無い。**
判定 → 理解度への反映は、psql でダミーの仕事を積み、ワーカー単体で通し確認した（`LLM_PROVIDER=stub`。PR #29）。
**フロントエンドは1行も書いていない。ここが当初の目的（スキルツリーの可視化）の本体。**

**方針変更で棚上げするもの・残るものの一覧は `logs/decisions.md` 2026-09-23 にある**（消さずに残し、v2 で戻す）。
1点だけ補足：`internal/llm/prompt.go` は**コードとしては棚上げ**だが、**文面（判定の指示）は AI への指示書へ流用する**。

- `SPEC.md` — 実装が参照する正本。**中身はまだ Web サービス前提**。`docs/spec-guide.md` — 人間向けの解説
- `backend/testdata/` — ダミーのロードマップとダミー学習ログ。**実データは置かない**
- CI（backend の gofmt / vet / test / build、frontend の lint / typecheck / test / build、秘密情報スキャン）は `main` と全 PR で走る

## 2. 次セッションで最初にやること

**2-0 の 1 件目＝TODO.md を仕分ける**（画面のタスクを前に出し、棚上げ分をフェーズ3へ移す）。

着手前に `logs/decisions.md` の 2026-09-23 を読む。**棚上げしたタスクは消さず、フェーズ3 に理由つきで並べる。**
仕分けが終わるまで 2-1〜2-6 には着手しない（棚上げ対象が混ざっている）。

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
docker compose up -d --build worker       # 判定ワーカー（LLM_PROVIDER=stub なら課金ゼロ）
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
