# 引き継ぎ

`/apps-workflow:handoff` で更新する。**次のセッションが最初に読むファイル。**

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

**インポートの永続化（`store.ImportRoadmap`）と DB テストの土台まで完了。
次は 2-3 の残り＝インポート API（`POST /api/roadmaps/import`）。**

`internal/roadmap` までは **PR #1 で `main` にマージ済み**。`ImportRoadmap` も **PR #8 でマージ済み**
（feat のコミットが chore ブランチに乗った経緯は KNOWLEDGE.md 2026-08-22）。CI（gofmt / vet / test / build、
フロントの lint / typecheck / test / build、秘密情報スキャン）は `main` と全 PR で走る。

- `SPEC.md` — 実装が参照する正本。9節すべて記入済み
- `docs/spec-guide.md` — **人間向けの解説。本人はこちらを読む。** SPEC.md と同じ事実を二重に書かない
- `backend/internal/domain/` — 理解度モデルの純粋関数。テスト62件・カバレッジ96.6%
- `backend/internal/roadmap/` — **マスタ JSON の検査と `domain.Roadmap` への変換。
  テスト66件・カバレッジ95.6%**
- `backend/internal/config/` — 環境変数の読み込みと検証。カバレッジ94.3%
- `backend/internal/store/` — PostgreSQL アクセス（users / sessions / roadmaps）。
  **`ImportRoadmap` は DB テスト5件。`sessions` の DB テストは未着手**（土台は `store_test.go` にある）
- `backend/internal/httpapi/` — chi ルータ、CORS、GitHub OAuth、セッション。カバレッジ39.1%
  （DB を使う経路が未検査。低いのはそのため）。**ログインは実機で通し確認済み（2026-08-20）**
- `backend/migrations/` — 12テーブル ＋ `roadmaps.levels`。`up` → `down -all` → `up` を実機確認済み
- `backend/testdata/` — ダミーのロードマップ2種とダミー学習ログ6件。**実データは置かない**

## 2. 直近でやったこと

- **`store.ImportRoadmap`** — 検査済みの `roadmap.Document` から `roadmaps` / `domains` / `items` を
  1トランザクションで作る。`levels` は jsonb に丸ごと、`depends_on_keys` は key のまま `text[]`、
  `outcome` があれば `outcome_source='authored'`。戻り値は `roadmaps.id`
- **DB テストの土台**（`store_test.go`）— `TEST_DATABASE_URL` が無ければスキップ。あればテストごとに
  専用スキーマを作り `migrations/*.up.sql` を流す。CI の backend ジョブに postgres サービスを追加。
  要点は KNOWLEDGE.md 2026-08-22
- **`internal/roadmap`** — マスタ JSON の検査。詳細は KNOWLEDGE.md 2026-08-14 の3件。要点だけ：
  - **問題を全部集めて返す**（1件目で打ち切らない）。エラー＝インポート中止、警告＝通す
  - 未知のフィールドはエラー（`dependsOn` の打ち間違いを黙って通さない）
  - 循環参照を DFS で検出し、経路をメッセージに出す
  - 問題の場所を `domains[0].items[2]` の形の経路で返し、フロントが該当箇所を指せる
- **`migrations/000003_roadmap_levels`** — `roadmaps.levels`（jsonb）。SPEC に保存先が
  抜けていたのを埋めた。**このマイグレーションは `roadmaps` が0行である前提**
- `backend/testdata/` — ダミーのロードマップ（正常・壊れたもの）とダミー学習ログ
- `.env.example` をコミット（**前セッションから未コミットだった。原因は §4-5**）
- `.mcp.json`（context7）をリポジトリに追加

## 3. 確定している決定事項

設計判断の理由はすべて `KNOWLEDGE.md` にある。**方針を変えたくなったら必ず先に読むこと。**

- **理解度は既存 skill-map.md の5段階を踏襲。** 0-100 の点数にしない
- **時間経過でレベルを下げない。** 代わりに「根拠の鮮度」を別軸で持つ
- **判定結果はイベントとして追記し、現在の状態は導出値**（`assessment_events` → `item_states`）
- **LLM の出力は提案。検証ルール V1〜V8 を通してから適用し、弾いた内容は必ず記録する**
- **ロードマップ共有は star（参照）と fork（スナップショット複製）を分ける**
- **`outcome`（到達状態）は必須にしない。** `origin` 別に扱いを変え、AI 下書きを用意する
- **判定は v1 から非同期ジョブ**（`llm_jobs` + ワーカー + ポーリング）
- **レベル定義（`levels`）はロードマップごとに `roadmaps.levels`（jsonb）で持つ。**
  アプリ共通の固定値にしない。`criteria` が LLM プロンプトの判定基準の正本
- **インポート JSON の未知フィールドはエラーにして弾く。** 前方互換より打ち間違いの検知を取る
- **セッションは DB に持つ。** Cookie には32バイトの乱数、DB にはその SHA-256 だけ
- **GitHub OAuth は標準ライブラリで実装。scope は空。アクセストークンは保存しない**
- `~/workspace/study/learner-profile/skill-map.md` は**今まで通り手動運用。移行しない**

## 4. 未確定・確認待ち

1. **学習パスの「今ここ」と「次にやること1位」の見分け方は方針決定済み**（SPEC.md §7.2）。
   2026-08-22 に学習パスを「一列表示 / スキルツリー表示」の切り替えにした（`logs/decisions.md` 3件）。
   ツリー表示では「解放済み・未着手」の枠強調と「深掘り候補」バッジで見分ける。細部の見た目は 2-5 で本人と詰める
2. `DefaultWeights()` の重みは仮置き。v1 が動いてから調整する（本人合意済み）
3. デプロイ先（Cloud Run / Render）— v1 がローカルで動いてから決める
4. 1,000人規模になったときの LLM コスト対策（BYOK / 課金 / モデル切り替え）
5. **`.env.example` の読み書き・コミットは両方とも解決済み。** 別の場所にある別の原因が2つあった。
   読み取りは `~/.claude/settings.json` の `Read(**/.env.*)`、コミットは
   `~/.claude/hooks/staged-secrets-guard.sh` のバグ2件（KNOWLEDGE.md 2026-08-12 と 2026-08-17）。
   **副作用として `.env.secret` のような未登録の名前は deny されない。**
   なおフック自体の編集は自動承認の分類器に拒否されるので、直すときは本人の手で当ててもらう

## 5. 次セッションのタスク

`TODO.md` の未完タスクを上から。**次は 2-3 の残り（インポート API）から。**

1. **インポート API（`POST /api/roadmaps/import`）。**
   `roadmap.ParseAndValidate` → `store.ImportRoadmap(ctx, user.ID, store.RoadmapKindPersonal, doc)`。
   **エラーは 400、警告は 201 の応答に載せる**（`outcome` 欠落でインポートを止めない）。
   `Result` の JSON 形はテストで固定してある
2. **リクエストボディのサイズ制限と1フィールドの長さ上限。**
   `criteria` / `outcome` は LLM のプロンプトに載るのでコストに直結する。件数の上限は入れたが
   長さは未着手（KNOWLEDGE.md 2026-08-14）
3. 自分のロードマップの CRUD（一覧・取得・名前と目標日の更新・削除）。
   `depends_on_keys` の読み出しは `pgtype.NewMap().SQLScanner` が要る（KNOWLEDGE.md 2026-08-22）
4. `internal/store` の `sessions` の DB テスト（期限切れ・cascade 削除）

**本人にやってもらう必要があること：現時点で無し。**
GitHub OAuth App は作成済みで、2026-08-20 に認可 → トークン交換 → セッション発行 →
`/api/me` まで実機で通した。**未検証のまま残っている経路はもう無い。**

ただし `backend/.env` は**ローカルにしか無い**（コミットしていない）。別のマシンで動かすときは
作り直しが要る。値は GitHub の OAuth App 設定と `openssl rand -base64 48` から取り直す。
本番にデプロイするときは Redirect URI が変わるので**本番用の OAuth App を別に作る**こと
（Client Secret をローカルと共用しない）。

## 6. 動作確認コマンド

```bash
# 純粋関数のテスト（DB 不要）
go -C backend test ./internal/... -cover

# マスタ JSON の検査だけを詳しく見る
go -C backend test ./internal/roadmap/ -v

# DB を使うテスト（store）。postgres を起動してから。未設定ならスキップされる
docker compose up -d postgres
TEST_DATABASE_URL='postgres://skillmatrix:skillmatrix@localhost:5432/skillmatrix?sslmode=disable' \
  go -C backend test ./internal/store/ -v

# DB を起動してマイグレーション適用
docker compose up -d postgres
docker compose run --rm migrate

# サーバ起動（backend/.env を用意してから）
set -a; source backend/.env; set +a
go -C backend run ./cmd/server

# やり直す
docker compose down -v
```
