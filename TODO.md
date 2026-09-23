# TODO

SessionStart フックが `- [ ]` の行を先頭12件まで自動で提示する。**この記法を崩さないこと。**
着手順に並べる。上ほど先にやる。

## フェーズ1：設計（完了）

- [x] `PLAN.md` を書く
- [x] 理解度のスケールと色の濃度への割り当てを決める（既存5段階を踏襲＋前段階状態）
- [x] 時間経過による理解度の減衰を入れるか決める（入れない。根拠の鮮度を別軸で持つ）
- [x] スケジュール調整の定義を決める（自動再配置はせず「次にやること Top N」に絞る）
- [x] データの置き場所を決める（PostgreSQL + Docker Compose。デプロイは後回し）
- [x] ロードマップ・マスタ JSON のスキーマを決める
- [x] AI 判定の入出力を決める（プロンプト構成・構造化出力・検証ルール V1〜V8）
- [x] 公開範囲とスコープの段階分けを決める（v1〜v4）
- [x] 認証方式を決める（GitHub OAuth）
- [x] ロードマップ共有の設計を決める（star は参照のみ、使うときは fork でスナップショット複製）
- [x] `SPEC.md` の初版を書く

## フェーズ2：v1 実装

### 2-1. 土台

- [x] Docker Compose を用意する（postgres + migrate。backend / worker / frontend は実装後に追加）
- [x] マイグレーション基盤を入れて `SPEC.md` §3 のテーブルを作る（golang-migrate）
- [x] `internal/config` で環境変数を読む（`.env.example` を `SPEC.md` §8.3 に合わせて更新）
- [x] GitHub OAuth ログインとセッション（HttpOnly Cookie）を実装する
- [x] GitHub OAuth App を実際に作り、ブラウザでログインを一度通す（2026-08-20 完了。認可 → トークン交換 → セッション発行 → `/api/me` まで実機で確認）
- [x] `internal/store` の DB テストの土台を作る（`TEST_DATABASE_URL` が無ければスキップ、あればテストごとに専用スキーマへマイグレーションを流す。CI にも postgres サービスを追加。`store_test.go`）
- [x] `internal/store` の `sessions` の DB テストを足す（期限切れ・cascade 削除。土台は `store_test.go` にある。`sessions_test.go` に7本）
- [x] `docker-compose.yml` に backend サービスを足す（Dockerfile とセット）
  完了条件：`docker compose up -d --build backend` の1コマンドで DB 起動 → マイグレーション → API サーバ起動まで進み、`/health` が 200、セッション付きの `/api/me` が 200 を返す。`.env` はイメージに入らない（KNOWLEDGE.md 2026-09-18）
- [x] `CLAUDE.md` に「`apps-workflow:handoff` を誰が実行するか」を正しく書く（当初は「ユーザー起動限定なのでユーザーに頼む」を追記する予定だったが、2026-09-18 の apps-workflow v1.4.2 で限定が外れたため、逆の内容＝Claude が自分で呼ぶ、に改めた。KNOWLEDGE.md 2026-09-14 / 2026-09-18）
  完了条件：`CLAUDE.md` の「セッション開始時にすること」節に、区切りでは Claude が handoff を自分で呼ぶこと、ユーザー起動限定は `pr-check` だけであることが書かれ、`../CLAUDE.md` と矛盾しない

### 2-2. 理解度モデル（純粋関数・先に作る）

- [x] `internal/domain` に型を定義する（Level / PreState / EvidenceType / ItemState / Judgment / Violation）
- [x] `ApplyJudgment` と検証ルール V1〜V8 を実装し、単体テストを書く
- [x] `RollupDomain` / `Staleness` / `NextActions` を実装し、単体テストを書く
- [x] `BuildPath`（依存関係のトポロジカルソート、done/current/upcoming の分類）を実装し、単体テストを書く

### 2-3. ロードマップ

- [x] `internal/roadmap` にマスタ JSON のスキーマ検証を実装する（key 重複・循環参照・上限・`origin` 別の必須項目）
- [x] `backend/testdata/` にダミーのロードマップとダミー学習ログを用意する
- [x] `internal/store` にインポートの永続化を実装する（`ImportRoadmap`。`roadmaps` / `domains` / `items` を1トランザクションで作る。`levels` は jsonb でそのまま入れる。DB テスト5件）
- [x] インポート API（`POST /api/roadmaps/import`）を実装する（`outcome` 欠落は警告として 201 の応答に載せ、インポート自体は通す。エラーは 400 で全問題を返す。SPEC.md §6）
- [x] インポート時のリクエストボディのサイズ制限（2 MiB → 413）と、1フィールドの文字数上限（`too_long`）を入れる（SPEC.md §2）
- [x] 自分のロードマップの CRUD を実装する（一覧・取得・名前と目標日の更新・削除）。完了条件：`GET /api/roadmaps` `GET/PATCH/DELETE /api/roadmaps/:id` が他人のロードマップには 404 を返し、`depends_on_keys` が `[]string` で読める（KNOWLEDGE.md 2026-08-22）。SPEC.md §6
- [x] `docs/spec-guide.md` にインポートと CRUD の説明を足す（壊れた JSON を貼るとどうなるか＝何がエラーで何が警告か、`roadmaps.levels` の保存先、他人のロードマップが 404 になる理由）
  完了条件：SPEC.md §2 §6 の事実を二重に書かず、`docs/spec-guide.md` §3 から SPEC へリンクした状態で、Naoki が読んで「JSON を貼ったら何が起きるか」を説明できる

### 2-4. AI 判定

- [x] `internal/llm` にインタフェースと **stub プロバイダ**を実装する（`LLM_PROVIDER=stub`。開発中の課金ゼロ＋テストの決定性）
  完了条件：`llm.New` が `LLM_PROVIDER=stub` で LLM を呼ばない実装を返し、同じ入力に同じ判定を返す。その判定が `domain.ApplyJudgments` を違反なしで通る。返す JSON を差し替えた stub で V1・V2・V4 が記録される。形の崩れた判定は捨てずに `Rejected` に残る（KNOWLEDGE.md 2026-09-19）
- [x] `internal/llm` に Claude API クライアントとプロンプト組み立てを実装する（共通部を先頭に固める。判定基準は `roadmaps.levels` の `criteria` を使い、コードに書かない）
  完了条件：`llm.New` が `LLM_PROVIDER=anthropic` で Claude API を呼ぶ実装を返す。system に共通部（役割・`criteria`・根拠の種類・禁止事項）とキャッシュの印が載り、ロードマップ・現在の状態・ログ本文は後ろの user ブロックに分かれる。返事は `output_config.format` の JSON Schema で縛り、stub と同じ `ParseOutput` を通す。API を呼ばずに `httptest` で送信内容とエラーの仕分けを検証している（KNOWLEDGE.md 2026-09-23）
- [x] 判定ジョブのキューとワーカーを実装する（`FOR UPDATE SKIP LOCKED`・リトライ・失敗記録。形の検査で弾いた `llm.Output.Rejected` も V1〜V8 の違反と一緒に `llm_responses.violations` へ記録する）
  完了条件：`cmd/worker` が順番待ちから判定を1件ずつ処理し、応答（読めなかった生の出力・拒否・課金済みの失敗を含む）を `llm_responses` に残し、
  検証を通った判定を `assessment_events` → `item_states` へ1トランザクションで反映する。再試行するのは `llm.ErrTemporary` が付いた失敗だけで、
  上限は `JUDGMENT_MAX_ATTEMPTS`（既定3）。同じ仕事を二度取り出さない。`LLM_PROVIDER=stub` で実機の通し確認ができる（KNOWLEDGE.md 2026-09-23）
- [ ] レート制限（月次クォータ）を実装する
- [ ] ログ投稿 API（202 + jobId）とジョブ状態 API を実装する
- [ ] 反映モード（`auto` / `confirm`）と保留判定の確定 API を実装する
- [ ] 到達状態（`outcome`）の AI 下書き生成ジョブと API を実装する

### 2-5. フロントエンド

- [ ] `vite.config.ts` に `server: { port: 5173, strictPort: true }` を入れる（ポートが黙ってずれると `FRONTEND_ORIGIN` と食い違う。KNOWLEDGE.md 2026-08-20）
- [ ] API クライアントと認証まわりの土台（TanStack Query）
  完了条件：`/api/me` で未ログインを判定してログイン導線を出す。エラー応答は `error.code` で分岐し、`issues` は「あれば表示」にする（ボディの読み込み失敗など `issues` の無い 400 がある。SPEC.md §6）
- [ ] マトリクス画面（可変長グリッド＋サマリー帯、色＝レベル／枠線＝要再確認）
- [ ] 学習パス画面・一列表示（依存順の縦一列、済／今ここ／この先、到達状態の表示）
- [ ] 学習パス画面・スキルツリー表示（SPEC §7.2）
  - [ ] `features/path/layout.ts`：段・列の配置を純粋関数で計算（Vitest）
  - [ ] SVG で線を引く。4状態（未解放／解放済み・未着手／習得済み／深掘り候補）の描き分け
  - [ ] 他分野の前提をゴーストノードで置き、クリックでその分野へ移動
  - [ ] 一列／ツリーの切り替えと、選択の記憶
- [ ] ログ投稿画面（判定中インジケータ → 結果表示 → その場で上書き）
- [ ] 項目詳細画面（レベル遷移の履歴と根拠、到達状態の編集）
- [ ] 次にやること Top N の表示（到達状態と次の確認方法をセットで）

### 2-6. 仕上げ

- [ ] E2E（Playwright）でログ投稿からマトリクス更新までを1本通す
- [x] GitHub Actions で lint / typecheck / test / build を通す（テンプレート由来の `.github/workflows/ci.yml` が要件を満たしている。PR #1 で実際に通ることを確認）
- [ ] README を書く（セットアップ手順・スクリーンショット）
  完了条件：クローン直後の人が README だけで `docker compose up -d --build backend` まで進める。必要な Docker Compose のバージョン（v2.24 以降。`env_file` の `required: false` のため）が書かれている
- [ ] CI に `docker build backend` を足す（2026-09-19 に採用。今の CI は Dockerfile をビルドしないので、壊れても気づけない。`logs/decisions.md`）
  完了条件：backend に変更のある PR で Dockerfile のビルドが CI で走り、わざと壊した Dockerfile では赤くなることを一度確かめてある
- [ ] `DefaultWeights()` の重みを実データの手触りで調整する（今は仮置き。v1 が動いてから、と本人合意済み）
  完了条件：「次にやること Top N」の並びを本人が見て違和感が無い。変えた重みと理由が KNOWLEDGE.md にあり、単体テストが更新されている
- [ ] デプロイ先を決める（Cloud Run / Render。v1 がローカルで動いてから。費用が絡むのでユーザー判断）
  完了条件：選んだ先と理由が `logs/decisions.md` にある。本番用の GitHub OAuth App をローカル用とは別に作る（Redirect URI が変わる。Client Secret を共用しない）手順がタスクに起きている

## フェーズ3：v2 以降

（v1 が動いてから起こす。公開ロードマップの一覧・star・fork、publish、GUI エディタ。
利用者が増えたときの LLM コスト対策＝BYOK / 課金 / モデル切り替えもここで扱う）

## 確認待ち

- [ ] 【ユーザー作業】全アプリの apps-workflow を v1.4.4 に更新する（`/plugin` から。反映は各アプリの次のセッションから）
  v1.4.4 は claude-plugins の PR #14 で main にマージ済み（2026-09-21）。pr-flow の後始末が「Claude が確認 2 点のうえ削除する」に変わる。
  `/plugin` に 1.4.4 が出ないときはマーケットプレイスを取得し直す（ローカルのカタログは更新するまで 1.4.3 のまま）。
  2026-09-21 時点の導入版は skill-matrix が 1.4.3、app-template / home-site-finder / babyfood-check /
  life-plan-simulator / photo-prompt-builder が 1.4.2
  完了条件：`jq -r '.plugins["apps-workflow@n-yoshida-dev"][] | "\(.version)  \(.projectPath)"' ~/.claude/plugins/installed_plugins.json` の全行が 1.4.4 になっている

- [ ] 【ユーザー確認】SPEC.md §4.2 の表で、system に載せるものから「出力スキーマ」を外す（PR #28 の受け入れレビューで判明。2026-09-23）
  実装では返事の形を system の文面ではなく `output_config.format`（構造化出力）で指定している。system 側には「JSON 以外の文章を出力しない」とだけ書いてある。
  縛りとしては構造化出力のほうが強いので実装を変える必要はないが、SPEC の表は「system に出力スキーマを書く」と読める。
  直す箇所：SPEC.md §4.2 の表の1行目「判定の役割、5段階の `criteria`、`evidenceType` の許可リストと意味、昇格ルール、出力スキーマ、禁止事項」から「出力スキーマ」を外し、
  代わりに「出力スキーマは §4.3 のとおり `output_config.format` で指定する（system には『JSON 以外を出力しない』とだけ書く）」を注記する
  完了条件：Naoki が了承し、SPEC.md §4.2 の表と `backend/internal/llm/prompt.go`・`anthropic.go` が同じことを言っている

- [ ] 【別セッション】他アプリに溜まったマージ済みローカルブランチを片付ける
  2026-09-21 時点の非 main ブランチは life-plan-simulator 25 本・babyfood-check 7 本・photo-prompt-builder 2 本・app-template 1 本。
  枝ごとに `gh pr list --state merged --head <枝> --json number,headRefOid` でマージ済み PR を引き、ローカルの先端
  （`git rev-parse <枝>`）が一致することを確かめてから消す。引けない枝・一致しない枝は残す（pr-flow v1.4.4 の「6. 後始末」）。
  **life-plan-simulator の `feat/gap-first-result` は PR #65 が OPEN の作業中の枝。消さず、チェックアウトも動かさない。**
  **同アプリは 2026-09-21 時点で未コミットの変更が 2 件ある（`frontend/src/features/result/rows.ts` と `timeline.ts`）ので、扱いを先に本人へ確認する**
  完了条件：4 アプリで、マージ済み PR に対応するローカルブランチが残っていない。消さなかった枝は理由が報告されている

- [x] 【ユーザー確認】SPEC.md の stub の説明を実装に合わせて直す（2026-09-20 に Naoki が了承し、同日に反映。同じ言い回しが残っていた `backend/.env.example` と `docs/spec-guide.md` §8 も合わせた。PR #24 で判明。理由は KNOWLEDGE.md 2026-09-19）。直す箇所は 3 つ：
  §4.6・§8.3 の「固定レスポンスを返す」→「LLM を呼ばず、同じ入力には同じ判定を返す（ロードマップの先頭から未達の項目を 3 件まで選び、現在レベル + 1 を提案する。同じログを繰り返し投稿すると開発環境のマトリクスは先頭から順に埋まる）」／
  §4.5 に「V1〜V8 の前に形の検査（型の不一致・必須欄の欠落・空の rationale・0〜1 の外の confidence）があり、弾いた判定も `llm_responses.violations` に記録する」を追記／
  `backend/internal/config/config.go` の `ProviderStub` のコメントを同じ文面に合わせる
  完了条件：Naoki が説明を読んで了承し、SPEC.md §4.5・§4.6・§8.3 と config.go のコメントが実装（`backend/internal/llm/stub.go`・`output.go`）と同じことを言っている

- [x] 【ユーザー作業】マージ済みのローカルブランチを削除する（2026-09-19 に本人が実行し、一覧が `main` だけになったことを確認。2026-09-19 に削除で合意。Claude の `git branch -D` は権限設定で拒否されるため本人が実行する。`logs/decisions.md`）
  完了条件：`git branch` の一覧に、PR が MERGED の作業ブランチが残っていない。消す前に「PR が MERGED」「ローカルの先端が PR の先端と一致」の 2 点を確かめてある（PR #13〜#21 の 9 本は 2026-09-19 に確認済み）。**この項目の「拒否されるため本人が実行する」は 2026-09-19 時点の前提。2026-09-21 に ask へ移し、今は Claude が実行する（`logs/decisions.md` 2026-09-21）**
