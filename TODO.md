# TODO

SessionStart フックが `- [ ]` の行を先頭12件まで自動で提示する。**この記法を崩さないこと。**
着手順に並べる。上ほど先にやる。

**2026-09-23 に v1 の方針が「公開する Web サービス」→「自分専用のツール」へ変わった（`logs/decisions.md` 2026-09-23）。**
棚上げした分は消さずフェーズ3（保留）へ理由つきで移してある。フェーズ2 に残っているのは自分専用版で画面まで到達するのに要るものだけ。
節番号（2-1〜2-6）は `KNOWLEDGE.md` と `logs/decisions.md` から参照があるので付け替えていない。中身だけ入れ替えた。

## フェーズ1：設計（完了）

- [x] `PLAN.md` を書く
- [x] 理解度のスケールと色の濃度への割り当てを決める（既存5段階を踏襲＋前段階状態）
- [x] 時間経過による理解度の減衰を入れるか決める（入れない。根拠の鮮度を別軸で持つ）
- [x] スケジュール調整の定義を決める（自動再配置はせず「次にやること Top N」に絞る）
- [x] データの置き場所を決める（PostgreSQL + Docker Compose。デプロイは後回し）**2026-09-23 にリポジトリ内の `data/` 配下の JSON へ決め直した（`logs/decisions.md`）。PostgreSQL はフェーズ3 へ棚上げ**
- [x] ロードマップ・マスタ JSON のスキーマを決める
- [x] AI 判定の入出力を決める（プロンプト構成・構造化出力・検証ルール V1〜V8）
- [x] 公開範囲とスコープの段階分けを決める（v1〜v4）
- [x] 認証方式を決める（GitHub OAuth）**v2 へ棚上げ**
- [x] ロードマップ共有の設計を決める（star は参照のみ、使うときは fork でスナップショット複製）**v2 へ棚上げ**
- [x] `SPEC.md` の初版を書く

## フェーズ2：v1 実装（自分専用版。ゴールは「画面で自分のマトリクスが見える」こと）

### 2-0. 方針変更の反映（2026-09-23 決定。`logs/decisions.md` を先に読む）

**v1 を「公開する Web サービス」から「自分専用のツール」へ絞り込む。** データはリポジトリ内の JSON、判定は
Claude Code / ChatGPT、検証は CI、画面は静的サイト。DB・ログイン・ワーカーは消さずに棚上げしてフェーズ3へ送った。

- [x] TODO.md を仕分ける（2026-09-23 完了。画面のタスクを前に出し、棚上げ分をフェーズ3へ移した）
  完了条件：フェーズ2 に残っているのが自分専用版に必要なタスクだけになり、棚上げした分はフェーズ3に「v2：他人にも使わせる」として理由つきで並んでいる。進捗表の残り件数が、画面まで到達するのに必要な件数を表している
- [x] 自分専用版のデータの置き場所と、AI への指示の形を決める（2026-09-23 完了。`logs/decisions.md` に6件記録）
  完了条件：`logs/decisions.md` に次の5点が記録されている。(1) 判定結果の JSON をリポジトリのどこに、どんな粒度（1ファイル/項目 か 1ファイル/ロードマップ か）で置くか (2) Claude Code / ChatGPT に渡す指示をどのファイルに置くか（`internal/llm/prompt.go` の文面を流用するか） (3) 検証（V1〜V8）をいつ走らせるか（CI か、書き込み前の CLI か） (4) 学習ログの本文と、判定の `rationale`（ログの引用を含みうる）をリポジトリに置くか。置くなら公開時に読まれる前提になり、CLAUDE.md「実データをコミットしない」と衝突する (5) `internal/llm` の stub の扱い（判定の主体が AI ツール側に移ると役割が変わる）
  決まった内容（要約。正本は `logs/decisions.md` 2026-09-23 の6件）：
  (1) `data/roadmap.json` ＋ `data/judgments/YYYY-MM-DD-<短い名前>.json`（学習ログ1件 = 1ファイル・追記のみ）＋ `data/state.json`（導出値。CLI が書き直す）
  (2) 指示書の本体は `prompts/judge.md`。`.claude/skills/judge-log/SKILL.md` は薄い入口。判定基準は書き写さず `data/roadmap.json` と `domain/types.go` を参照させる
  (3) 手元の `recalc` と CI の `verify` の両方。`verify` は `state.json` が再計算結果と一致するかまで見る
  (4) **学習ログ本文は置かない。** 正本は `~/workspace/study`（Private）。判定に `evidenceRef` を持たせて出どころを指す。`rationale` と `evidenceRef` は置くが、どちらも技術的な事実だけに縛る。判定 JSON は実物をコミットし、リポジトリは公開できる
  (5) stub は棚上げ（コードとテストは残す）。`internal/llm/output.go` は v1 の中核として残る
- [ ] `PLAN.md` と `SPEC.md` を新しい方針へ書き換える
  完了条件：SPEC.md §3（DB スキーマ）・§4.1（非同期ジョブ）・§4.2（プロンプト構成）・§4.5（検証の置き場所）・§6（API）・§8.3（環境変数）が新しい方針に沿って書き換わっているか「v2 へ棚上げ」と明示されていて、v1 のデータ配置と判定の流れが読める。
  `PLAN.md`「技術的な方針」に、既定スタック（`../CLAUDE.md`）から外れる点＝サーバと DB を持たないこと、サーバ通信が無いので TanStack Query を入れないことが書いてある。`docs/spec-guide.md` も食い違っていない。
  **`PLAN.md`「やらないこと」の2行も直す**（PR #32 のレビューで判明）：「判定の入力は学習ログのテキストのみ」と「GitHub リポジトリ連携」は、`evidenceRef` が `cert:` / `work:` / `repo:` を根拠として認める決定とずれている。判定の入力はログ本文のままだが、**根拠の出どころはログに限らない**ことを書き分ける
- [ ] `CLAUDE.md`（このリポジトリ）の「守ること」を見直す
  完了条件：「API キーをリポジトリに入れない」「Claude API は必ずバックエンドから呼ぶ」など、前提が変わった項目が新しい方針と矛盾していない。棚上げした内容は消さずに「v2 で戻す」と分かる形で残っている

### 2-1. 土台（自分専用版）

置き場所と粒度は 2026-09-23 に決まった（`logs/decisions.md`）。着手順は上から。

- [ ] `domain.Judgment` に `EvidenceRef` を足す（根拠の出どころを指す文字列。例 `study:orgflow-learning/logs/2026-08-05.md`）
  完了条件：`domain.Judgment` と `llm.ProposedJudgment` に `EvidenceRef` があり、`llm.Output.DomainJudgments` が引き渡す。空文字を許すか必須にするかを決めて単体テストがある。`ApplyJudgment` の判定ロジックは `EvidenceRef` を見ない（出どころは記録であって判定材料ではない）
  書式は `<種別>:<識別子>`（`study:<パス>` / `cert:<資格名>/<年月>-合格` / `repo:<リポジトリ>@<コミット>` / `work:<期間>/<担当>`）
- [ ] 判定結果と理解度の JSON をリポジトリに置く
  完了条件：`data/roadmap.json`・`data/judgments/YYYY-MM-DD-<短い名前>.json`（ダミー2〜3件）・`data/state.json` が置かれ、`backend/testdata/` と同じく実データを含まない。ファイルの形が SPEC.md に書いてある。**学習ログ本文は置かない**（`evidenceRef` で指すだけ）
- [ ] 検証と再計算の CLI を作る（`backend/cmd/`。JSON を読み、形の検査 → V1〜V8 → `ApplyJudgment` → 理解度を書き出す）
  完了条件：`recalc` と `verify` の2モードがある。`recalc` は `data/state.json` を書き出す。`verify` は書き出さず、違反があれば終了コード 1 と違反の一覧（どの項目のどのルールか）を出し、**さらに `state.json` が再計算結果と一致しなければ終了コード 1**。DB・HTTP・LLM クライアントを import しない。単体テストがある
- [ ] その CLI を CI で走らせる（`verify` モード）
  完了条件：`.github/workflows/ci.yml` に組み込まれ、JSON をわざと壊した PR と、`state.json` だけ古い PR の両方で CI が赤くなることを一度確かめてある
- [ ] 根拠の出どころの設定ファイル `sources.local.json` を用意する（`.example` だけコミット）
  完了条件：`sources.local.json.example` に study のパスを書く形が示されている。実ファイルはコミットされていない。
  **`.gitignore` は追記不要**（`*.local.json` が既に `.gitignore:18` にあり、`.example` は guard-secrets の末尾一致に掛からずコミットできる。PR #32 のレビューで確認）
- [x] `CLAUDE.md` に「`apps-workflow:handoff` を誰が実行するか」を正しく書く（当初は「ユーザー起動限定なのでユーザーに頼む」を追記する予定だったが、2026-09-18 の apps-workflow v1.4.2 で限定が外れたため、逆の内容＝Claude が自分で呼ぶ、に改めた。KNOWLEDGE.md 2026-09-14 / 2026-09-18）
  完了条件：`CLAUDE.md` の「セッション開始時にすること」節に、区切りでは Claude が handoff を自分で呼ぶこと、ユーザー起動限定は `pr-check` だけであることが書かれ、`../CLAUDE.md` と矛盾しない

### 2-2. 理解度モデル（純粋関数。v1 の中核。完了）

- [x] `internal/domain` に型を定義する（Level / PreState / EvidenceType / ItemState / Judgment / Violation）
- [x] `ApplyJudgment` と検証ルール V1〜V8 を実装し、単体テストを書く
- [x] `RollupDomain` / `Staleness` / `NextActions` を実装し、単体テストを書く
- [x] `BuildPath`（依存関係のトポロジカルソート、done/current/upcoming の分類）を実装し、単体テストを書く

### 2-3. ロードマップ（マスタ JSON の検証。v1 の中核）

DB への永続化・インポート API・CRUD はサーバごと棚上げした（フェーズ3 の 3-1）。残るのは JSON を読んで検証する部分。

- [x] `internal/roadmap` にマスタ JSON のスキーマ検証を実装する（key 重複・循環参照・上限・`origin` 別の必須項目）
- [x] `backend/testdata/` にダミーのロードマップとダミー学習ログを用意する

### 2-4. AI 判定（自分専用版：判定は Claude Code / ChatGPT にやらせる）

サーバから Claude API を呼ぶ部分（クライアント・キュー・ワーカー・クォータ・ジョブ API）は棚上げした（フェーズ3 の 3-1 / 3-2）。
v1 で要るのは「AI に渡す指示書」と、AI が返した JSON を検査する側（`internal/llm/output.go` と 2-1 の CLI）だけ。

- [x] `internal/llm` にインタフェースと **stub プロバイダ**を実装する（`LLM_PROVIDER=stub`。開発中の課金ゼロ＋テストの決定性）
  完了条件：`llm.New` が `LLM_PROVIDER=stub` で LLM を呼ばない実装を返し、同じ入力に同じ判定を返す。その判定が `domain.ApplyJudgments` を違反なしで通る。返す JSON を差し替えた stub で V1・V2・V4 が記録される。形の崩れた判定は捨てずに `Rejected` に残る（KNOWLEDGE.md 2026-09-19）
  **2026-09-23 に「棚上げ（コードとテストは残す）」と決定。** v1 の手順書と README には登場させない（`logs/decisions.md`）
- [ ] AI への指示書 `prompts/judge.md` を用意する（`internal/llm/prompt.go` の文面を流用する）
  完了条件：Claude Code / ChatGPT にそのファイルを読ませるだけで判定の JSON が出てくる。役割・根拠の種類・昇格ルール・禁止事項・出力の形が載っている。**レベルの基準と根拠の上限は書き写さず、`data/roadmap.json` の `levels` と `backend/internal/domain/types.go` を読ませる**。禁止事項に「**`rationale` と `evidenceRef` に**所属先・企業名・人名・転職活動・人事評価に関する記述を含めない」がある。`evidenceRef` の書式（`<種別>:<識別子>`）と、`sources.local.json` から根拠の出どころを読む手順が載っている。到達状態（`outcome`）の下書きもこの指示書で作れる
- [ ] `.claude/skills/judge-log/SKILL.md` を作る（`prompts/judge.md` を読ませる薄い入口）
  完了条件：`/judge-log` で呼べ、`prompts/judge.md` と `sources.local.json` を読み、未判定の学習ログを探して `data/judgments/` に新しいファイルを1つ書くところまで進む。**既存の判定ファイルは書き換えない**。判定基準や禁止事項をこのファイルに書き写していない（`prompts/judge.md` へのリンクだけ）

### 2-5. 画面（当初の目的の本体。ここが v1 のゴール）

- [ ] 画面の土台を作る（テンプレートの初期画面を外し、ルーティングと JSON の読み込みを置く）
  完了条件：`frontend/src` からテンプレートのデモ画面が消え、2-1 で置いた JSON を型付きで読み込んで各画面へ渡せる。サーバ通信が無いので TanStack Query は入れない（既定スタックから外れる点は PLAN.md に書く）
  **`data/` は `frontend/` の外にあるので、dev サーバで読むのに `vite.config.ts` の `server.fs.allow` が要る可能性がある**（PR #32 のレビューで指摘。着手時に実機で確かめる）
- [ ] マトリクス画面（可変長グリッド＋サマリー帯、色＝レベル／枠線＝要再確認）
- [ ] 学習パス画面・一列表示（依存順の縦一列、済／今ここ／この先、到達状態の表示）
- [ ] 学習パス画面・スキルツリー表示（SPEC §7.2）
  - [ ] `features/path/layout.ts`：段・列の配置を純粋関数で計算（Vitest）
  - [ ] SVG で線を引く。4状態（未解放／解放済み・未着手／習得済み／深掘り候補）の描き分け
  - [ ] 他分野の前提をゴーストノードで置き、クリックでその分野へ移動
  - [ ] 一列／ツリーの切り替えと、選択の記憶
- [ ] 項目詳細画面（レベル遷移の履歴と根拠、到達状態の表示）
  完了条件：ある項目のレベルがいつ・何を根拠に上がったかが、`rationale` と `evidenceRef` で読める。**この画面から編集はしない**（2026-09-23 決定。静的サイトは書き込み先を持たず、更新は AI ツールが `data/judgments/` にファイルを足す形で行う）
- [ ] 次にやること Top N の表示（到達状態と次の確認方法をセットで）

### 2-6. 仕上げ

- [ ] E2E（Playwright）で、JSON を差し替えるとマトリクスと学習パスが変わることを1本通す
  完了条件：ビルドした静的サイトに対して Playwright が1本通り、CI で走る
- [x] GitHub Actions で lint / typecheck / test / build を通す（テンプレート由来の `.github/workflows/ci.yml` が要件を満たしている。PR #1 で実際に通ることを確認）
- [ ] README を書く（セットアップ手順・スクリーンショット）
  完了条件：クローン直後の人が README だけで「学習ログを AI に判定させる → JSON を更新する → 画面で見る」を一周できる。画面のスクリーンショットがある
- [ ] `DefaultWeights()` の重みを実データの手触りで調整する（今は仮置き。v1 が動いてから、と本人合意済み）
  完了条件：「次にやること Top N」の並びを本人が見て違和感が無い。変えた重みと理由が KNOWLEDGE.md にあり、単体テストが更新されている
- [ ] `~/workspace/study/learner-profile/skill-map.md` との関係を決める（2026-09-23 に判明）
  skill-map.md は同じ5段階モデルで手で維持されている理解度台帳で、**skill-matrix はこれを自動化するアプリ**。
  2つ並立すると必ずずれる。方針の案は「画面が実用になるまで skill-map.md が正本のまま並行運転 → 実用になったら
  skill-map.md を『skill-matrix を見よ』に畳む」。**畳むかどうかはユーザー判断**（study 側の運用が変わるため）
  完了条件：どちらを正本にするかが `logs/decisions.md` にあり、畳む場合は study 側の変更内容（どのファイルをどう書き換えるか）がタスクとして起きている
- [ ] 画面を GitHub Pages に公開する
  完了条件：`main` への push で静的サイトがビルドされて Pages に上がり、URL を開くとマトリクスが見える。**Public にする直前に、`data/judgments/` 全件の `rationale` と `evidenceRef` を目で読み、固有名詞（所属先・企業名・人名）が混ざっていないことを確かめる**（2026-09-23 決定の残リスク。`logs/decisions.md`）
  **【ユーザー作業】リポジトリを Public にする操作はユーザーが行う**（公開設定の変更は不可逆。URL は実施時に添える）

## フェーズ3：v2「他人にも使わせる」（保留・合計に含めない）

2026-09-23 に v1 を自分専用へ絞り込んだときに棚上げした分。**コードもタスクも消さない。**
戻す条件は `logs/decisions.md` 2026-09-23 の「見直す条件」＝自分専用の画面まで到達し、実際に使って「他人にも使わせたい」と思えたとき。
そう思わなければ CLI 配布へ進み、ここは畳む。

### 3-1. 棚上げした実装（済み。コードは残っている）

棚上げの理由（共通）：自分専用版はサーバ・DB・ログイン・非同期ジョブを持たない。他人に使わせる段になったら丸ごと要るので消さない。

- [x] Docker Compose を用意する（postgres + migrate。backend / worker / frontend は実装後に追加）
- [x] マイグレーション基盤を入れて `SPEC.md` §3 のテーブルを作る（golang-migrate）
- [x] `internal/config` で環境変数を読む（`.env.example` を `SPEC.md` §8.3 に合わせて更新）
  **この1件だけは 2026-09-23 の決定の棚上げ列挙に無い。** 中身はサーバ・DB・LLM の設定なのでここへ置いた。
  2026-09-23 の決定で、v1 の CLI が読む設定は `sources.local.json`（根拠の出どころのパス）だけになり、
  `internal/config` の環境変数とは別系統になった。**`internal/config` はこのまま棚上げでよい**（CLI からは import しない）
- [x] GitHub OAuth ログインとセッション（HttpOnly Cookie）を実装する
- [x] GitHub OAuth App を実際に作り、ブラウザでログインを一度通す（2026-08-20 完了。認可 → トークン交換 → セッション発行 → `/api/me` まで実機で確認）
- [x] `internal/store` の DB テストの土台を作る（`TEST_DATABASE_URL` が無ければスキップ、あればテストごとに専用スキーマへマイグレーションを流す。CI にも postgres サービスを追加。`store_test.go`）
- [x] `internal/store` の `sessions` の DB テストを足す（期限切れ・cascade 削除。土台は `store_test.go` にある。`sessions_test.go` に7本）
- [x] `docker-compose.yml` に backend サービスを足す（Dockerfile とセット）
  完了条件：`docker compose up -d --build backend` の1コマンドで DB 起動 → マイグレーション → API サーバ起動まで進み、`/health` が 200、セッション付きの `/api/me` が 200 を返す。`.env` はイメージに入らない（KNOWLEDGE.md 2026-09-18）
- [x] `internal/store` にインポートの永続化を実装する（`ImportRoadmap`。`roadmaps` / `domains` / `items` を1トランザクションで作る。`levels` は jsonb でそのまま入れる。DB テスト5件）
- [x] インポート API（`POST /api/roadmaps/import`）を実装する（`outcome` 欠落は警告として 201 の応答に載せ、インポート自体は通す。エラーは 400 で全問題を返す。SPEC.md §6）
- [x] インポート時のリクエストボディのサイズ制限（2 MiB → 413）と、1フィールドの文字数上限（`too_long`）を入れる（SPEC.md §2）
- [x] 自分のロードマップの CRUD を実装する（一覧・取得・名前と目標日の更新・削除）。完了条件：`GET /api/roadmaps` `GET/PATCH/DELETE /api/roadmaps/:id` が他人のロードマップには 404 を返し、`depends_on_keys` が `[]string` で読める（KNOWLEDGE.md 2026-08-22）。SPEC.md §6
- [x] `docs/spec-guide.md` にインポートと CRUD の説明を足す（壊れた JSON を貼るとどうなるか＝何がエラーで何が警告か、`roadmaps.levels` の保存先、他人のロードマップが 404 になる理由）
  完了条件：SPEC.md §2 §6 の事実を二重に書かず、`docs/spec-guide.md` §3 から SPEC へリンクした状態で、Naoki が読んで「JSON を貼ったら何が起きるか」を説明できる
- [x] `internal/llm` に Claude API クライアントとプロンプト組み立てを実装する（共通部を先頭に固める。判定基準は `roadmaps.levels` の `criteria` を使い、コードに書かない）
  完了条件：`llm.New` が `LLM_PROVIDER=anthropic` で Claude API を呼ぶ実装を返す。system に共通部（役割・`criteria`・根拠の種類・禁止事項）とキャッシュの印が載り、ロードマップ・現在の状態・ログ本文は後ろの user ブロックに分かれる。返事は `output_config.format` の JSON Schema で縛り、stub と同じ `ParseOutput` を通す。API を呼ばずに `httptest` で送信内容とエラーの仕分けを検証している（KNOWLEDGE.md 2026-09-23）
  **コードは棚上げ。ただし `prompt.go` の文面（判定の指示）は 2-4 の AI 指示書へ流用する**
- [x] 判定ジョブのキューとワーカーを実装する（`FOR UPDATE SKIP LOCKED`・リトライ・失敗記録。形の検査で弾いた `llm.Output.Rejected` も V1〜V8 の違反と一緒に `llm_responses.violations` へ記録する）
  完了条件：`cmd/worker` が順番待ちから判定を1件ずつ処理し、応答（読めなかった生の出力・拒否・課金済みの失敗を含む）を `llm_responses` に残し、
  検証を通った判定を `assessment_events` → `item_states` へ1トランザクションで反映する。再試行するのは `llm.ErrTemporary` が付いた失敗だけで、
  上限は `JUDGMENT_MAX_ATTEMPTS`（既定3）。同じ仕事を二度取り出さない。`LLM_PROVIDER=stub` で実機の通し確認ができる（KNOWLEDGE.md 2026-09-23）

### 3-2. 棚上げした未着手のタスク

- [ ] レート制限（月次クォータ）を実装する
  棚上げの理由：自分専用版はサーバから Claude API を呼ばないので、守る対象（自分の課金）が無い
- [ ] ログ投稿 API（202 + jobId）とジョブ状態 API を実装する
  棚上げの理由：ログの投稿先が HTTP ではなく AI ツールとの会話になる。
  着手時に決めること：**`running` のまま取り残された仕事をどう回収するか**。ワーカーが処理中に落ちるとその仕事は誰にも拾われず、
  画面には「判定中」が出たままになる。`started_at` が一定時間より古い `running` を `queued` に戻す方式が素直（PR #29 のレビューで判明。2026-09-23）
- [ ] 反映モード（`auto` / `confirm`）と保留判定の確定 API を実装する
  棚上げの理由：自分専用版では判定の確認が「JSON の差分を git で見る」になる。他人に使わせる段で API として要る
- [ ] 到達状態（`outcome`）の AI 下書き生成ジョブと API を実装する
  棚上げの理由：下書きの生成は 2-4 の AI 指示書に含める。ジョブと API が要るのは他人に使わせる段
- [ ] ログ投稿画面（判定中インジケータ → 結果表示 → その場で上書き）
  棚上げの理由：投稿先の API が棚上げになったため。自分専用版のログ投稿は AI ツールとの会話で行う
- [ ] `vite.config.ts` に `server: { port: 5173, strictPort: true }` を入れる（ポートが黙ってずれると `FRONTEND_ORIGIN` と食い違う。KNOWLEDGE.md 2026-08-20）
  棚上げの理由：食い違う相手（バックエンドの CORS 設定）が無くなった。バックエンドを戻すときに一緒に入れる
- [ ] CI に `docker build backend` を足す（2026-09-19 に採用。今の CI は Dockerfile をビルドしないので、壊れても気づけない。`logs/decisions.md`）
  完了条件：backend に変更のある PR で Dockerfile のビルドが CI で走り、わざと壊した Dockerfile では赤くなることを一度確かめてある
  棚上げの理由：守る対象の backend イメージ自体が棚上げ。決定（2026-09-19）は取り消していないので、戻すときにそのまま実施する
- [ ] デプロイ先を決める（Cloud Run / Render。費用が絡むのでユーザー判断）
  完了条件：選んだ先と理由が `logs/decisions.md` にある。本番用の GitHub OAuth App をローカル用とは別に作る（Redirect URI が変わる。Client Secret を共用しない）手順がタスクに起きている
  棚上げの理由：自分専用版はサーバを置かない（画面は GitHub Pages。2-6）。他人に使わせる段で必要になる

### 3-3. v2 で新たに起こすもの（タスク未定義）

公開ロードマップの一覧・star・fork、publish、GUI エディタ。
利用者が増えたときの LLM コスト対策＝BYOK / 課金 / モデル切り替えもここで扱う。
2026-09-23 に捨てなかったもう一つの道（検証ロジックを CLI として配布する）も、自分専用版が動いてから改めて比べる。

## 確認待ち

- [ ] 【ユーザー作業】全アプリの apps-workflow を v1.4.4 に更新する（`/plugin` から。反映は各アプリの次のセッションから）
  v1.4.4 は claude-plugins の PR #14 で main にマージ済み（2026-09-21）。pr-flow の後始末が「Claude が確認 2 点のうえ削除する」に変わる。
  `/plugin` に 1.4.4 が出ないときはマーケットプレイスを取得し直す（ローカルのカタログは更新するまで 1.4.3 のまま）。
  2026-09-21 時点の導入版は skill-matrix が 1.4.3、app-template / home-site-finder / babyfood-check /
  life-plan-simulator / photo-prompt-builder が 1.4.2
  完了条件：`jq -r '.plugins["apps-workflow@n-yoshida-dev"][] | "\(.version)  \(.projectPath)"' ~/.claude/plugins/installed_plugins.json` の全行が 1.4.4 になっている

- [x] 【ユーザー確認】SPEC.md §3 の `llm_responses` に `raw_text` 列を足す（**2026-09-23 の方針変更で解消。個別に直さず、2-0 の SPEC 書き換えに吸収する**。§3 のテーブル定義ごと「v2 へ棚上げ」になるため）
  読み取れなかった LLM の出力（「承知しました。判定結果は…」のような JSON でない文字列）を残す場所。
  `raw` は jsonb なので壊れた文字列が入らず、捨てると原因を調べようがなくなる（SPEC §4.5「握りつぶし禁止」）。
  直す箇所：SPEC.md §3 の `llm_responses` の列一覧に `raw_text text` を足し、`raw jsonb not null` を `raw jsonb`（NULL 可）に直す。
  「どちらか片方は必ず埋まる」制約（`llm_responses_has_payload`）があることも書く。
  併せて §4.5 に「出力全体が読めないときは、生の文字列を `llm_responses.raw_text` に残して判定ジョブを失敗にする」を追記する。
  もう1点、§4.1 のフロー図は「生レスポンス保存 → 検証」の順だが、実装は「検証 → 保存 → 反映」の順。
  `llm_responses.violations` を同じ行に入れるには先に検証が要るため（保存が反映より先である点は図と同じ）。図の順を実装に合わせるかも一緒に判断する。
  さらに §8.3 の環境変数一覧に、この PR で足した `JUDGMENT_MAX_ATTEMPTS`（既定3、1〜10）と
  `JUDGMENT_POLL_INTERVAL_SECONDS`（既定5、1〜300）の2行を足す（`backend/.env.example` には反映済み）
  完了条件：Naoki が了承し、SPEC.md §3・§4.5 と `backend/migrations/000004_llm_response_raw_text.up.sql`・`internal/store/jobs.go` が同じことを言っている

- [x] 【ユーザー確認】SPEC.md §4.2 の表で、system に載せるものから「出力スキーマ」を外す（**2026-09-23 の方針変更で解消。2-0 の SPEC 書き換えに吸収する**。判定の主体が Claude Code / ChatGPT に移り、§4.2 のプロンプト構成そのものを書き直すため。文面は `internal/llm/prompt.go` に残っているので流用できる）
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
  完了条件：`git branch` の一覧に、PR が MERGED の作業ブランチが残っていない。消す前に「PR が MERGED」「ローカルの先端がその PR の先端と一致」の 2 点を確かめてある（PR #13〜#21 の 9 本は 2026-09-19 に確認済み）。**この項目の「拒否されるため本人が実行する」は 2026-09-19 時点の前提。2026-09-21 に ask へ移し、今は Claude が実行する（`logs/decisions.md` 2026-09-21）**
