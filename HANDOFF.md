# 引き継ぎ

`/handoff` で更新する。**次のセッションが最初に読むファイル。**

## 0. 進め方のルール（最優先）

**設計・仕様・技術判断を提示するときは、ジュニアレベルでも分かる解説を先に付けること。**
専門用語は初出時に「何のためにそうするのか」を添え、抽象的な説明の前に具体例を出す。
承認を求めるのはその後。**中身が分からないまま承認させない。**

本人の自己申告：「作りたいアプリの最終イメージは持っているが、技術的な設計判断は曖昧」。
実務は Java / Spring Boot なので DB 設計・REST API・OpenAPI・JUnit は通じる。
Go と React は学習中。（`~/.claude/CLAUDE.md` にも記載済み）

**テストコードを「仕様の確認材料」として示すときは、コードの読み方を教えない。**
「何を保証しているか」を日本語の箇条書きで列挙する。Go のテストは go-react の go-06 で
本人が初歩から学ぶ予定。

## 1. 現在地

**フェーズ2「2-1 土台」完了。認証まで通り、次は 2-3 ロードマップ（JSON 検証とインポート）。**

- `SPEC.md` — 実装が参照する正本。9節すべて記入済み。`sessions` テーブルと `APP_ENV` /
  `FRONTEND_ORIGIN` を追記済み
- `docs/spec-guide.md` — **人間向けの解説。本人はこちらを読む。** SPEC.md と同じ事実を二重に書かない
- `backend/internal/domain/` — 理解度モデルの純粋関数。テスト62件・カバレッジ96.6%
- `backend/internal/config/` — 環境変数の読み込みと検証。カバレッジ94.3%
- `backend/internal/store/` — PostgreSQL アクセス（users / sessions）。**DB テスト未着手**
- `backend/internal/httpapi/` — chi ルータ、CORS、GitHub OAuth、セッション。カバレッジ39.1%
  （DB を使う経路が未検査。低いのはそのため）
- `backend/migrations/` — 12テーブル。`up` → `down -all` → `up` の往復を実機で確認済み

## 2. 直近でやったこと

- `internal/config` — 環境変数の読み込み。不備は1件目で打ち切らず**全部集めて報告**する。
  `String()` が秘密情報を伏せるのでログにそのまま出せる
- `.env.example` を SPEC.md §8.3 に合わせて全面更新（**読めなかった原因は §4 参照**）
- `migrations/000002_sessions` — セッションテーブル
- `internal/store` — `Open` / `UpsertUserByGitHub` / セッションの CRUD
- `internal/httpapi` — `/health`・`/api/auth/github`・`/api/auth/github/callback`・
  `/api/auth/logout`・`/api/me`、CORS、`requireAuth` ミドルウェア
- `cmd/server/main.go` — DB 接続と graceful shutdown を配線
- 実機で起動確認済み（`/health` 200 / 未認証 `/api/me` 401 / `/api/auth/github` が
  GitHub へ 302 / CORS プリフライト 204）

## 3. 確定している決定事項

設計判断の理由はすべて `KNOWLEDGE.md` にある。**方針を変えたくなったら必ず先に読むこと。**

- **理解度は既存 skill-map.md の5段階を踏襲。** 0-100 の点数にしない
- **時間経過でレベルを下げない。** 代わりに「根拠の鮮度」を別軸で持つ
- **判定結果はイベントとして追記し、現在の状態は導出値**（`assessment_events` → `item_states`）
- **LLM の出力は提案。検証ルール V1〜V8 を通してから適用し、弾いた内容は必ず記録する**
- **ロードマップ共有は star（参照）と fork（スナップショット複製）を分ける**
- **`outcome`（到達状態）は必須にしない。** `origin` 別に扱いを変え、AI 下書きを用意する
- **判定は v1 から非同期ジョブ**（`llm_jobs` + ワーカー + ポーリング）
- **セッションは DB に持つ。** Cookie には32バイトの乱数、DB にはその SHA-256 だけ
- **GitHub OAuth は標準ライブラリで実装。scope は空。アクセストークンは保存しない**
- `~/workspace/study/learner-profile/skill-map.md` は**今まで通り手動運用。移行しない**

## 4. 未確定・確認待ち

1. **学習パスの「今ここ」と「次にやること1位」を画面でどう見分けさせるか**（SPEC.md §7.2）。
   この2つは別の問いに答えるので一致しない。**2-5（フロントエンド）で本人に相談すること**
2. `DefaultWeights()` の重みは仮置き。v1 が動いてから調整する（本人合意済み）
3. デプロイ先（Cloud Run / Render）— v1 がローカルで動いてから決める
4. 1,000人規模になったときの LLM コスト対策（BYOK / 課金 / モデル切り替え）
5. **`.env.example` の読み書きは解決済み。** 原因は `guard-secrets.sh` ではなく
   `~/.claude/settings.json` の `Read(**/.env.*)`。個別指定に置き換えた。
   **副作用として `.env.secret` のような未登録の名前は deny されない**（KNOWLEDGE.md 参照）

## 5. 次セッションのタスク

`TODO.md` の未完タスクを上から。**次は 2-3（ロードマップ）から。**

1. **`internal/roadmap` にマスタ JSON のスキーマ検証を実装する**
   （key 重複・循環参照・上限・`origin` 別の必須項目）
2. インポート API と、自分のロードマップの CRUD
3. `backend/testdata/` にダミーのロードマップとダミー学習ログ

**本人にやってもらう必要があること（AI 側ではできない）**：
GitHub OAuth App を作り（<https://github.com/settings/developers>）、
`backend/.env` に `GITHUB_OAUTH_CLIENT_ID` / `GITHUB_OAUTH_CLIENT_SECRET` /
`SESSION_SECRET` を入れて、ブラウザでログインを一度通すこと。
Authorization callback URL は `http://localhost:8080/api/auth/github/callback`。
**現時点で通しで検証できていないのはこの1点だけ。**

## 6. 動作確認コマンド

```bash
# 純粋関数のテスト（DB 不要）
go -C backend test ./internal/... -cover

# DB を起動してマイグレーション適用
docker compose up -d postgres
docker compose run --rm migrate

# サーバ起動（backend/.env を用意してから）
set -a; source backend/.env; set +a
go -C backend run ./cmd/server

# やり直す
docker compose down -v
```
