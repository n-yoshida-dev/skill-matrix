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

**2026-09-25 追記：v1.5「テンプレート配布」（フェーズ 2.5）に向けて、v1 のうちに守る 6 点が `PLAN.md`「v1.5 テンプレート配布」にある。
2-1（CLI・`sources.local.json`）、2-4（指示書）、2-6（Pages 公開）に着手する前に読むこと。**

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
- [x] `PLAN.md` と `SPEC.md` を新しい方針へ書き換える（2026-09-25 完了。棚上げ分は SPEC.md §10 にまとめた。コードのコメントが指す §3.2 / §4.4 / §4.5 / §4.7 は意味を保って同じ番号に置いた）
  完了条件：SPEC.md §3（DB スキーマ）・§4.1（非同期ジョブ）・§4.2（プロンプト構成）・§4.5（検証の置き場所）・§6（API）・§8.3（環境変数）が新しい方針に沿って書き換わっているか「v2 へ棚上げ」と明示されていて、v1 のデータ配置と判定の流れが読める。
  `PLAN.md`「技術的な方針」に、既定スタック（`../CLAUDE.md`）から外れる点＝サーバと DB を持たないこと、サーバ通信が無いので TanStack Query を入れないことが書いてある。`docs/spec-guide.md` も食い違っていない。
  **`PLAN.md`「やらないこと」の2行も直す**（PR #32 のレビューで判明）：「判定の入力は学習ログのテキストのみ」と「GitHub リポジトリ連携」は、`evidenceRef` が `cert:` / `work:` / `repo:` を根拠として認める決定とずれている。判定の入力はログ本文のままだが、**根拠の出どころはログに限らない**ことを書き分ける
- [x] `CLAUDE.md`（このリポジトリ）の「守ること」を見直す（2026-09-25 完了。冒頭の説明文も自分専用版に合わせた）
  完了条件：「API キーをリポジトリに入れない」「Claude API は必ずバックエンドから呼ぶ」など、前提が変わった項目が新しい方針と矛盾していない。棚上げした内容は消さずに「v2 で戻す」と分かる形で残っている

### 2-1. 土台（自分専用版。skill-map.md からの移行を含む）

置き場所と粒度は 2026-09-23 に決まった（`logs/decisions.md`）。**2026-09-25 に判定モデルを「2 本の梯子」へ変え、
`skill-map.md` からの移行を v1 の土台に組み込んだ（`logs/decisions.md` 2026-09-25 の 4 件）。設計・手順・対応表・期待表は
`docs/skill-map-migration.md`（以下「移行計画」）。段階 A（決定の記録・TODO の分割）は 2026-09-25 に完了。** 着手順は上から。

- [x] `SPEC.md` を 2 本の梯子モデルへ書き換える（移行計画の段階 B。§7 の表のとおり）（2026-09-25 完了。同じ PR で §7.3 公開ビュー・§7.4 を追加）
  完了条件：§0（正本の移行）・§1（印と `verifiedLevel`、`learning_activity`、`preState` の導出）・§2（`moduleRefs`、公開前提の縛り）・§3.3（`schemaVersion: 2`、`source: migration`、`evidenceRefs`、`confidence` の条件、`occurredAt`、`repo:` `log:`）・§3.4（`verifiedLevel` / `evidencedLevels` / `marked`）・§3.5（`logs` と `repos`）・§4.2・§4.4・§4.5（V4 欠番）・§5（`Action.PendingLevels`）・§7 が書き換わり、`docs/spec-guide.md` と食い違わない。
  `PLAN.md`「未確定」の skill-map の行と `KNOWLEDGE.md` 2026-08-10「移行しない」に「2026-09-25 の決定で置き換え」の注記がある。
  SPEC に写した後は SPEC が正本なので、段階 C 以降の完了条件にある「移行計画 §x」の参照を SPEC の節番号へ付け替えてある。
  `CLAUDE.md`「判定基準を指示書に書き写さない」の「根拠の種類と上限」を「根拠の種類と印」に直してある
- [x] `internal/domain` と `internal/llm/output.go` を 2 本の梯子モデルへ変更する（段階 C-1。SPEC.md §1.1・§4.4・§4.5・§5。Go の型名の対応は移行計画 §3）（2026-09-26 完了。V6 の識別子は `V6_failed_check`）
  完了条件：`ItemState` に `VerifiedLevel`（導出）と `Evidenced`、`Judgment` に `Source` / `EvidenceRefs` / `HasConfidence`、`EvidenceLearningActivity`、`evidenceLadder`（Base / Top）がある。V4（`MaxLevelStep` / `ViolationLevelJump`）が消え、V5 は梯子の範囲、V6 は `proposedLevel == 0`、V7 は `source: ai` のときだけ。印がある項目の `preState` は常に `none`。導出（`[3]`→0、`[1,3]`→1、`[1,2,3]`→3、`[1,2,3,4]`→4）・不合格報告・鮮度の更新条件・source 別の V7 がテストにある。`ParseOutput` が `evidenceRefs`（配列・1 件以上・書式）と `confidence` の有無（`ai` なら必須、他は禁止）を検査する。棚上げ中の `store` は改名に機械的に追従するだけ（挙動は変えない）。`EvidenceRefs` は判定材料にしない
- [x] 検証と再計算の CLI を作る（段階 C-2。`backend/cmd/skillmatrix`。JSON を読み、形の検査 → V1〜V8 → `ApplyJudgment` → 理解度を書き出す）（2026-09-26 完了。下準備として形の検査を `internal/judgment` へ切り出した＝PR #43。書き出しの形は SPEC §3.4 末尾）
  完了条件：`recalc` と `verify` の2モードがある。`recalc` は `data/state.json`（SPEC.md §3.4 の形）を書き出す。`verify` は書き出さず、違反があれば終了コード 1 と違反の一覧（どの項目のどのルールか）を出し、**さらに `state.json` が再計算結果と一致しなければ終了コード 1**。処理順はファイル名の昇順・配列の順。`preState` のゼロ値（`""`）は書き出し時に `"none"` にする（`state.json` は `none` を要求する。PR #41 のレビューで判明）。DB・HTTP・LLM クライアントを import しない。単体テストがある。`backend/testdata/` のダミー判定を `schemaVersion: 2` に更新してある
- [x] その CLI を CI で走らせる（`verify` モード）（2026-09-26 完了。PR #46。backend とは別の独立ジョブ「data/ の検証（verify）」。「PR 2 本」は同じ PR に push した 2 コミット（どちらも pull_request の run）で代えた。赤の run 2 本の URL は PR 本文。前提として `internal/roadmap` に `moduleRefs` を追加）
  完了条件：`.github/workflows/ci.yml` に組み込まれ、JSON をわざと壊した PR と、`state.json` だけ古い PR の両方で CI が赤くなることを一度確かめてある。
  **先に `data/state.json` を `recalc` で作り直す**（今は手書きのダミーで、`verify` は一致しない。`data/judgments/` が空なので全項目 0 になり、
  公開ビューのダミー表示は消える。段階 D で実物のロードマップに替わるまでの間だけ。`data/README.md` の「今はダミー」節も直す）
- [x] `data/roadmap.json` を自分の実物として作る（段階 D。移行計画 §8.3 の対応表から起こす。8 分野・約 40 項目）（2026-09-27 完了。PR #50。46 項目。`verifyBy` は §8.4 の移行後の状態から見た「次の印」で書いた）
  完了条件：`verify` が通る。項目 key はハイフン区切りでカリキュラムに依存しない（`go-syntax-basics`）。`moduleRefs` でモジュールと対応付けてある。`verifyBy` / `outcome` / `goal` / `description` を「所属|企業|面接|転職|人事」で grep して 0 件。`data/state.json` は全項目 0 の初期値で置く。**`backend/testdata/` のダミーとは別物**（ロードマップの定義は個人データではない）
- [x] 根拠の出どころの設定ファイル `sources.local.json` を用意する（`.example` だけコミット）（2026-09-27 完了。PR #51。実ファイルは手元だけ。`learning-logs/` を `.gitignore` と CI の秘密情報の検査に追加）
  完了条件：`sources.local.json.example` が `logs`（既定の置き場 `learning-logs/`）と `repos`（`n-yoshida-dev/study` → ローカルパスと `logsGlob`）の 2 キーの形（SPEC.md §3.5）。実ファイルはコミットされていない。
  **`.gitignore` は追記不要**（`*.local.json` が既に `.gitignore:18` にあり、`.example` は guard-secrets の末尾一致に掛からずコミットできる。PR #32 のレビューで確認）
- [x] 【study 側】`skill-map.md` を凍結し、参照先を skill-matrix へ切り替える（段階 E。`~/workspace/study` で作業する。段階 D の後）（2026-09-27 完了。**凍結コミットは study の `284c0f1`**（push 済み）。study の 18 ファイルで更新先・読み先を切り替え、過去の記録は残した。KNOWLEDGE.md 2026-09-27「段階 E」）
  完了条件：`learner-profile/skill-map.md` 冒頭に更新停止の注記（文面は移行計画 §8.1）。`learner-profile/README.md` の正本分担表、`CLAUDE.md` の「skill-map.md を確認する」「learner-profile へ反映する」、`.claude/rules/teaching.md`、`.claude/rules/learning-ops.md`（「skill-map レベル 2 以上」→ `verifiedLevel`）、`.claude/skills/kickoff/SKILL.md` の参照先が skill-matrix になっている。study 内で `skill-map.md` を「更新先」として指す記述が 0 件。**凍結コミットのハッシュを控える**（移行判定の `evidenceRefs` が指す）
- [x] `skill-map.md` から移行判定を作り `data/judgments/` に置く（段階 F。段階 E の直後、同じ日か翌日に）（2026-09-27 完了。7 本・41 件で `state.json` が §8.4 と 46 項目一致。
  凍結版との突き合わせで state/props を「判定なし」→ `drill` {1} に改訂。途中で見つけた `ApplyJudgment` の不具合（ドリル不合格が「説明済み」を消す）をテスト付きで直した。KNOWLEDGE.md 2026-09-27「段階 F」）
  完了条件：`prompts/migrate-skill-map.md`（移行計画 §8.2 を指示にしたもの。通常の `judge.md` には混ぜない）がある。`data/judgments/YYYY-MM-DD-migration-<domain>.json` が 7 本（java-spring 13 件・db 3・go 4・react 5（凍結版との突き合わせで 4 から改訂。§8.3）・devops 9・baas-auth 5・ai-collab 2）。`recalc` 後の `state.json` が移行計画 §8.4 の期待表と一致し、その表が PR 本文にある。`data/` を「所属|企業|面接|転職|人事」で grep して 0 件。`evidenceRefs` の 1 件目が凍結コミットの `skill-map.md` の行を指し、統合前の study のハッシュを書いていない。CI 緑
- [ ] 【study / personal-ai-context 側】正本の切り替えを終える（段階 G。段階 F の後）
  完了条件：`personal-ai-context/learning/technical-skills.md` と `learning-history.md` のリンク先が skill-matrix。study に `go-react/logs/` があり、`CLAUDE.md` に「学習ログは `logs/` に追記し、習熟度は skill-matrix の判定経由でのみ更新する。`state.json` は読むだけで編集しない。読めなければ習熟度不明として支援レベルを下げない」が書いてある（習熟度の文は段階 E で study の `CLAUDE.md`「横断学習プロファイルと習熟度の運用」に書き済み。G で足すのは `logs/` の文）。**凍結後の学習ログ 1 件から `/judge-log` で通常判定を 1 本作り、`state.json` が動いた**（2-4 の `judge.md` と `judge-log` が先に要る）
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
v1 で要るのは「AI に渡す指示書」と、AI が返した JSON を検査する側（`internal/judgment` と 2-1 の CLI）だけ。

- [x] `internal/llm` にインタフェースと **stub プロバイダ**を実装する（`LLM_PROVIDER=stub`。開発中の課金ゼロ＋テストの決定性）
  完了条件：`llm.New` が `LLM_PROVIDER=stub` で LLM を呼ばない実装を返し、同じ入力に同じ判定を返す。その判定が `domain.ApplyJudgments` を違反なしで通る。返す JSON を差し替えた stub で V1・V2・V4 が記録される。形の崩れた判定は捨てずに `Rejected` に残る（KNOWLEDGE.md 2026-09-19）
  **2026-09-23 に「棚上げ（コードとテストは残す）」と決定。** v1 の手順書と README には登場させない（`logs/decisions.md`）
- [x] AI への指示書 `prompts/judge.md` を用意する（`internal/llm/prompt.go` の文面を流用する）（2026-09-27 完了。前提を知らないサブエージェントに読ませて 3 回試運転し（受け入れレビュー 4 回）、`data/` のコピーで `recalc` の棄却 0 件。チャット経路は「ファイルを読めない AI 役」に貼り付け用の束を渡して模擬し、返った JSON が棄却 0 件（本物の ChatGPT では未試行）。見つかった穴と判断は KNOWLEDGE.md 2026-09-27「`prompts/judge.md`」）
  完了条件：Claude Code / ChatGPT にそのファイルを読ませるだけで判定の JSON が出てくる。役割・根拠の種類・印の付き方・禁止事項・出力の形が載っている。**レベルの基準と根拠の印は書き写さず、`data/roadmap.json` の `levels` と `backend/internal/domain/types.go` を読ませる**。禁止事項に「**`rationale` と `evidenceRefs` に**所属先・企業名・人名・転職活動・人事評価に関する記述を含めない」がある。`evidenceRefs` の書式（`repo:<owner>/<repo>@<commit>/<path>#L<n>`・`log:<path>`）と、`sources.local.json` から根拠の出どころを読む手順が載っている。**2026-09-25 追加**：`unaided_implementation` は AI から具体的なコード提示や逐次ガイドを受けていない場合だけ／不合格は `proposedLevel: 0`／`confidence` は必ず書く（`source: ai`）。到達状態（`outcome`）の下書きもこの指示書で作れる
- [ ] `.claude/skills/judge-log/SKILL.md` を作る（`prompts/judge.md` を読ませる薄い入口）
  完了条件：`/judge-log` で呼べ、`prompts/judge.md` と `sources.local.json` を読み、未判定の学習ログを探して `data/judgments/` に新しいファイルを1つ書くところまで進む。**既存の判定ファイルは書き換えない**。判定基準や禁止事項をこのファイルに書き写していない（`prompts/judge.md` へのリンクだけ）

### 2-5. 画面（当初の目的の本体。ここが v1 のゴール）

**2026-09-25 に画面を「公開ビュー（`/`）」と「作業ビュー（`/plan`）」の 2 つの入口に分けた（`logs/decisions.md`、SPEC.md §7）。最初に作る画面は公開ビュー。**

- [x] 画面の土台を作る（テンプレートの初期画面を外し、ルーティングと JSON の読み込みを置く）（2026-09-25 完了。`server.fs.allow` は実際に要った。KNOWLEDGE.md 2026-09-25）
  完了条件：`frontend/src` からテンプレートのデモ画面が消え、`data/roadmap.json`（SPEC.md §2。`schemaVersion: 1`）・`data/state.json`（§3.4。`schemaVersion: 2`）・`data/settings.json`（§8.3）を型付きで読み込んで各画面へ渡せる。ハッシュルーティングで `/` と `/plan` が切り替わる。サーバ通信が無いので TanStack Query は入れない（既定スタックから外れる点は PLAN.md に書いてある）。
  **2-1 の段階 D（実物のロードマップ）より先に着手するため、`data/` にはダミー（`backend/testdata/` と同じ内容を `schemaVersion: 2` にしたもの）を仮置きし、README に「段階 D で実物に差し替える」と書く。`state.json` は CLI が無い間は手書きで、段階 C-2 の `recalc` で生成し直す**
  **`data/` は `frontend/` の外にあるので、dev サーバで読むのに `vite.config.ts` の `server.fs.allow` が要る可能性がある**（PR #32 のレビューで指摘。着手時に実機で確かめる）
- [x] 公開ビュー（SPEC.md §7.3。採用担当者向け・タイル表示・押すと根拠）（2026-09-25 完了。ダミーデータで表示。実物は 2-1 段階 D・F で入る）
  完了条件：`/` を開くと、見出しと 2 文、分野ごとの行（分野名・`goal`・根拠のある項目数）、項目名が常時見えるタイル、押すとその行の直下に根拠（日付・根拠の種類・`rationale`）が出る。§7.3「出さないもの」が 1 つも出ていない。`[3]` の項目が「実装の根拠はあるが基礎の確認が未了」と出る。フッターはリポジトリへのリンクだけ。`docs/demo/public-view.html`（2026-09-25 のデモ）と同じ見た目・画面構成（文言は SPEC.md §7.3・§4.5 に従う）。Vitest で「出さないもの」が描画されないことを 1 本確かめる
- [ ] マトリクス画面（作業ビュー。可変長グリッド＋サマリー帯、色＝`verifiedLevel`／枠線＝要再確認／角の印＝上位の根拠あり。SPEC.md §7.1）
  完了条件：升目の塗りは `verifiedLevel`。`evidencedLevels` の最上段が `verifiedLevel` より上の升目に角の印が付き、ツールチップに「Verified n / 根拠の印 … / 未確認 …」が出る。サマリー帯に「実装根拠あり・理解未確認」の列がある
- [ ] 学習パス画面・一列表示（依存順の縦一列、済／今ここ／この先、到達状態の表示）
- [ ] 学習パス画面・スキルツリー表示（SPEC §7.2）
  - [ ] `features/path/layout.ts`：段・列の配置を純粋関数で計算（Vitest）
  - [ ] SVG で線を引く。4状態（未解放／解放済み・未着手／習得済み／深掘り候補）の描き分け
  - [ ] 他分野の前提をゴーストノードで置き、クリックでその分野へ移動
  - [ ] 一列／ツリーの切り替えと、選択の記憶
- [ ] 項目詳細画面（レベル遷移の履歴と根拠、到達状態の表示）
  完了条件：見出しに Verified Level／付いている印／未確認の段の 3 行がある。ある項目の印がいつ・どの根拠で付いたかが、`events[].marked`・`rationale`・`evidenceRefs` で読める。**この画面から編集はしない**（2026-09-23 決定。静的サイトは書き込み先を持たず、更新は AI ツールが `data/judgments/` にファイルを足す形で行う）
- [ ] 次にやること Top N の表示（到達状態と次の確認方法をセットで）
  完了条件：`PendingLevels` がある項目は文言が「既存の実装について L1/L2 を短いドリル・自己説明で確認する」になり、`verifyBy` はその下に添える（`[3]` の項目を「基礎からやり直し」と見せない。SPEC.md §7.4）

### 2-6. 仕上げ

- [ ] E2E（Playwright）で、JSON を差し替えるとマトリクスと学習パスが変わることを1本通す
  完了条件：ビルドした静的サイトに対して Playwright が1本通り、CI で走る
- [x] GitHub Actions で lint / typecheck / test / build を通す（テンプレート由来の `.github/workflows/ci.yml` が要件を満たしている。PR #1 で実際に通ることを確認）
- [ ] README を書く（セットアップ手順・スクリーンショット）
  完了条件：クローン直後の人が README だけで「学習ログを AI に判定させる → JSON を更新する → 画面で見る」を一周できる。画面のスクリーンショットがある
- [ ] `DefaultWeights()` の重みを実データの手触りで調整する（今は仮置き。v1 が動いてから、と本人合意済み）
  完了条件：「次にやること Top N」の並びを本人が見て違和感が無い。変えた重みと理由が KNOWLEDGE.md にあり、単体テストが更新されている
- [x] `~/workspace/study/learner-profile/skill-map.md` との関係を決める（2026-09-23 に判明 → **2026-09-25 に決定。skill-matrix を正本にし、移行する。** `logs/decisions.md` 2026-09-25 の 4 件。手順は 2-1 の段階 B〜G に分割済み）
  完了条件：どちらを正本にするかが `logs/decisions.md` にあり、畳む場合は study 側の変更内容（どのファイルをどう書き換えるか）がタスクとして起きている
- [ ] 画面を GitHub Pages に公開する
  完了条件：`main` への push で静的サイトがビルドされて Pages に上がり、URL を開くとマトリクスが見える。**Public にする直前に、`data/judgments/` 全件の `rationale` と `evidenceRefs` を目で読み、固有名詞（所属先・企業名・人名）が混ざっていないことを確かめる**（2026-09-23 決定の残リスク。`logs/decisions.md`）
  **【ユーザー作業】リポジトリを Public にする操作はユーザーが行う**（公開設定の変更は不可逆。URL は実施時に添える）

### 2-7. 開発環境（v1 の機能ではない。Claude Code に任せた開発を人間が観測するための道具）

- [x] 開発ダッシュボードを試験導入する（`dashboard/`。2026-09-26 完了。HTML + 素の JS + Node 標準ライブラリだけ、npm 依存 0、ビルド無し）
  完了条件：`node dashboard/update.mjs --serve` で起動し、TODO.md の進捗・Beads と「確認待ち」の人間待ち・HANDOFF の現在地と次の一手・
  CI（frontend / backend / 秘密情報の各ジョブ）と開いている PR・git の未コミットと最近のコミット・`data/` の件数と `verify` の結果・最近の合意が 1 画面に出る。
  ダッシュボード独自の状態を持たず（`data.js` は生成物で git 管理外。2026-09-26 に `data.json` から変更、HTML をダブルクリックで開けるように）、開くたびに正本から作り直す。PC 幅とスマートフォン幅で崩れない。
  使い方と取得元は `dashboard/README.md`、維持のルールは `CLAUDE.md`「開発ダッシュボード」
  **2026-09-26 に読みやすさ優先へ変更**（PR #49。ユーザーの「文章が多すぎる」の指摘）：最上段は数字のタイル、一覧は 1 行に縮めて押すと全文。
  CI のジョブ別の結果は失敗したときだけ出し、HANDOFF の全文・最近の合意・よく変わったファイルは折りたたみへ移した。
  **同日に「更新」ボタンを追加**（PR #54）：普段の起動は `node dashboard/update.mjs --serve --open` で、画面のボタンが `POST /update` を叩いて作り直す。
  **同日に起動方法の案内を追加**（PR #55）：ヘッダーの「起動方法」と、更新に失敗したとき・プロセスが止まったときに `cd` から始まる起動コマンドを出す

## フェーズ2.5：v1.5 テンプレート配布（保留・v1 の画面到達後に着手・合計に含めない）

2026-09-25 に構想（`PLAN.md`「v1.5 テンプレート配布」、`logs/decisions.md` 2026-09-25 の 4 件）。
他人が GitHub の Template repository から自分のリポジトリを作り、自分の AI ツール（Claude Code / ChatGPT 等）から使えるようにする。
サーバ・DB・ログインは要らない。**v1 のうちに守る 6 点は `PLAN.md` の同じ節にある。**
「未確定」と付いたタスクは本人の判断待ち（`PLAN.md`「v1.5 の未確定」）。着手順は上から。

- [ ] CLI に `init` サブコマンドを足す（`recalc` / `verify` と同じ入口）
  完了条件：ロードマップの選択（カタログから／自分で書く）・学習ログの置き場（既定 `learning-logs/`。別の場所なら `sources.local.json` を書く）・
  `data/judgments/` と `state.json` の初期化・初回のスキル入力（任意。`source: "manual"` の判定ファイル）の 4 点を対話で聞き、結果のファイルを書く。
  単体テストがある。**作者の実データを消す操作なので、実行前に確認を出す**
- [ ] ロードマップのカタログ `roadmaps/` を用意し、`data/roadmap.json` は選んだ 1 枚にする
  完了条件：`roadmaps/` に 2 枚以上（react / go など）があり、それぞれ `source` と `checkedAt` と元サイトの利用条件の確認結果が書いてある。
  自分の実ロードマップとダミーが区別できる。`init` がここから選べる
- [ ] `learning-logs/` と学習ログの雛形を置く（`.gitignore` への追加と `.example` の既定は PR #51 で済み）
  完了条件：`learning-logs/` が `.gitignore` に入り、雛形（日付・やったこと・自分の言葉で理解したこと・作ったもの）が `templates/` 等にある。
  `sources.local.json.example` の既定がこのディレクトリを指す。Private で使う人向けに `.gitignore` から外す手順が README にある。
  **CI の「秘密情報が混入していないか」の検査式からも `learning-logs/` を外す手順が README にある**（PR #51 で CI も止めるようにしたので、`.gitignore` だけ外すと CI が赤くなる）
- [ ] Claude Code 以外の入口を置く（`AGENTS.md` 等）
  完了条件：`AGENTS.md` に「判定は `prompts/judge.md` を読んで行う」の案内があり、Codex CLI / Cursor / Gemini CLI のいずれか 1 つで実機確認している。
  判定基準や禁止事項を書き写していない（`prompts/judge.md` へのリンクだけ）
- [ ] 【未確定】CI が `recalc` を回して PR に積む
  完了条件：判定ファイルだけを足した PR に対して Actions が `recalc` を実行し、`state.json` の差分をその PR ブランチにコミットし、同じジョブ内で `verify` が通る。
  `GITHUB_TOKEN` のコミットが次の CI を起動しない点を踏まえた構成になっている。採用の判断は本人（`logs/decisions.md` に記録してから着手）
- [ ] 【未確定】画面に「記録ページ」を置く（貼り付け用プロンプトの生成と、GitHub 新規ファイル画面へのリンク）
  完了条件：フォームに学習ログを書くと、指示書＋ロードマップ＋現在の理解度＋本文をまとめたプロンプトがコピーできる。
  返ってきた JSON を貼ると、`data/judgments/` 配下の正しいファイル名で GitHub の新規ファイル作成画面が中身入りで開く。
  画面はデータを書き換えない。採用時に 2026-09-23「画面は読むだけ」の決定へ一行足す
- [ ] 【未確定】`prompts/judge.md` に「確認質問」の節を足し、`evidenceRefs` の例に `chat:` を足す
  完了条件：判定の会話で AI が 2〜3 問の確認質問をして `drill` の根拠にできる。会話の要約を `learning-logs/` に書き残す手順がある。
  `PLAN.md`「やらないこと」の「クイズ形式の理解度測定」との線引き（問題バンク・採点画面は作らない）が PLAN に書き分けてある。採用の判断は本人
- [ ] README を公開向けに拡張する（2-6 の README タスクの続き）
  完了条件：README に「これは何か（スクリーンショット・作者の実物 URL）／仕組みの 1 枚図／Template から自分用に作る／`init` による初回セットアップ／
  日々の使い方（A・B・C の 3 群）／GitHub Pages で公開する（Public にする前のチェックリスト）／本家の更新を取り込む／作者と記事（Qiita・note）／ライセンス」の 9 節がある。
  ChatGPT の GitHub コネクタの読み書き可否を実機で確かめて書いてある。**Template から作った別リポジトリで、README だけを見て一周できることを一度確かめる**
- [ ] `LICENSE`（MIT）を置く
  完了条件：`LICENSE` があり、README の「ライセンス」節が「未定」でなくなっている。同梱カタログの出典の利用条件が確認済み
- [ ] 画面のフッターに「Built with skill-matrix」のリポジトリリンクを置く
  完了条件：フッターにリポジトリへのリンクだけがあり、個人リンク（Qiita / note）は無い（`logs/decisions.md` 2026-09-25）
- [ ] 【ユーザー作業】リポジトリの設定で Template repository にする
  完了条件：「Use this template」ボタンが出る。設定画面の URL は実施時に添える

## フェーズ3：v2「他人にも使わせる」（保留・合計に含めない）

2026-09-23 に v1 を自分専用へ絞り込んだときに棚上げした分。**コードもタスクも消さない。**
以下で参照している `SPEC.md §3`・`§6`・`§8.3` は棚上げ時点の節番号で、2026-09-25 の書き換えで `§10.1`・`§10.7`・`§10.8` へ移った。
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

- [ ] 【ユーザー確認】`../CLAUDE.md` に 3 点を足すか（2026-09-26 に判明）：(1)「同じリポジトリの作業ディレクトリを複数セッションで同時に使わない」
  （2026-09-25 に別セッションが同じチェックアウトで main を進め、ローカルブランチが消えるなど衝突の余地があった）。
  (2)「`internal/store` を触る PR は、コミット前に `docker compose up -d postgres` を立てて `TEST_DATABASE_URL` 付きで DB テストを回す」
  （ローカルでは Postgres 無しだとスキップされ、PR #41 で CI だけが落ちた）。
  (3)「棚上げするコードは、使い続けるコードと同じ Go パッケージに置かない」（Go の import はパッケージ単位なので、使わないつもりの依存まで付いてくる。
  PR #43 で `internal/llm` から形の検査を切り出した。KNOWLEDGE.md 2026-09-26）
  完了条件：`../CLAUDE.md` に 3 点が入っているか、入れない理由が本人から示されている
- [ ] 【別リポジトリ】claude-plugins の `check-edited.sh`（PostToolUse）を `package.json` の `lint` スクリプトを呼ぶ形に直す
  （このリポジトリは oxlint で eslint が無く、frontend の編集のたびに「eslint 失敗（missing packages）」が出る。KNOWLEDGE.md 2026-09-25）
  完了条件：skill-matrix の frontend でファイルを編集しても eslint のエラー表示が出ず、oxlint の結果が出る
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
