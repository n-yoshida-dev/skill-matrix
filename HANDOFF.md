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
**本人に答えを求めるメッセージは頼みごと 1 つだけ。** 冒頭に「やってほしいこと」を 1 行で書き、普通の文で返す言葉を指定して聞く（「「OK」か「困る」とだけ返してください」）。
読む物があるときは選択肢の画面（`AskUserQuestion`）を使わない（2026-10-04。判定の確認で起きたことから Claude が広げた判断。経緯は KNOWLEDGE.md 同日）。

**引き継ぎは頼まれるのを待たずに書き、続けられるときはそのまま次へ進む**（本人の要望と Claude の解釈は `logs/decisions.md` 2026-10-04。決まりの正本は `~/.claude/CLAUDE.md` と `../CLAUDE.md`）。

**同じ作業ディレクトリを別セッションと同時に使わない。** 2026-09-25 に別セッションが同じチェックアウトで main を進め、
ローカルブランチが消えるなど衝突の余地があった（2026-09-26 にも再発）。着手前に `git log --oneline -3` と `git branch` で状態を見る。
ブランチの先頭が想定と違えば `git reflog --date=iso` で誰がいつ動かしたかを確かめる。

## 進捗

```
進捗（TODO.md より自動集計・2026-10-31） / Progress from TODO.md

                     完了率  残り/全体  フェーズ   (done %, open/total, phase)
████████████████████  100%     0 / 11    フェーズ1：設計（完了）
████████████████████  100%     0 / 42    フェーズ2：v1 実装（自分専用版。ゴールは「画面で自分のマトリクスが見える」こと）
████████████████████  100%     0 / 5     フェーズ2b：スキルツリーを RPG のスキルツリーのように読める形にする（2026-10-03 決定）
████████████████████  100%     0 / 8     フェーズ2c：判定の回し方を作り直す（2026-10-03 決定）
████████████░░░░░░░░   60%     2 / 5     フェーズ2d：リリース後の改善（少しずつ。2026-10-10 決定）
████████████████████  100%     0 / 3     フェーズ2e：トップを「要約 → 項目 → 根拠」の 3 層にする（2026-10-12 決定）
████████████░░░░░░░░   60%     2 / 5     フェーズ2.5：v1.5 見本つきの再現ガイド（2026-10-25。対象は手元の AI＝Claude Code・Codex を使う人）
██████████████████░░   94%     4 / 79    合計 / Total
回答待ち：4 件（回答済み 17 件） / Waiting: 4 open, 17 answered
保留（合計に含めない）：23 件（済み 15 件） / On hold (not in total): 23 open, 15 done

前回の区切り（2026-10-29）から：完了 +0 件、タスクの数 +0 件 / Since last checkpoint (2026-10-29): done +0, tasks +0
```

## 1. 現在地

- **フェーズ2.5（v1.5）は 2026-10-25 に「見本つきの再現ガイド」へ切り替えた**（Template での配布はやめて保留へ。`logs/decisions.md` 同日「外に出す形は…」）。
  見本のデータ（架空の学習者。5 分野・22 項目）は `examples/demo/data/`（PR #126）。見本の画面は https://n-yoshida-dev.github.io/skill-matrix/demo/ （`feat/demo-site` の PR。このマージ後の状態で書いている）。
  見本の「今日」は 2026-10-25 に固定し、ヘッダの下に見本だと書いた 1 行を出す（`settings.json` の `site.fixedToday` / `site.notice`。KNOWLEDGE.md 2026-10-29）。
  残りは AI 向けの説明書 `docs/build-your-own.md`・紹介向けの README と `LICENSE`（TODO フェーズ2.5）。Template 設定の頼みごと（旧 ops-dhj.5）は閉じた

- **リリース済み**：https://n-yoshida-dev.github.io/skill-matrix/ （GitHub Pages。`main` への push ごとに `pages.yml` が公開する。push での公開も確かめた）。フェーズ1〜2c は全部完了。
  方針は「少しずつ改善する」（`logs/decisions.md` 2026-10-10）。残りはフェーズ2d の 2 件（どちらも本人の答えが先に要る）と確認待ち
- **フェーズ2e（トップを 3 層に）は 3 本とも済み**：公開ビューの 3 層（PR #113）、「図で見る」の能力値チャート（PR #114）、作業ビューのダッシュボードを外し入口と見出しを「学習パス」に（`feat/plan-path-only` の PR。
  ダッシュボードは `features/plan/Dashboard.tsx` に棚上げ、SPEC §10.10。`/plan` は最初の分野の学習パスへ移る）。Claude が選んだ細部は KNOWLEDGE.md 2026-10-14 の 2 件と 2026-10-18
- CI で時々落ちた `TestRun_pending` は PR #115 で対策済み（テストの git の自動の片付けを止めた。手元では再現しないので、CI で再発したら原因を調べ直す）
- **見た目の決まりは `DESIGN.md`**（PR `fix/look-pass` のマージ後の状態で書いている）。今ある決まりを写し、2026-10-11 に `/apps-workflow:look` の「B 毎回の点検」を全画面に通して、
  字の濃さ・折り返し・フォーカス・Tab の順と、部品単位の「AI がよく作る形」を直した。**画面のファイルを変える前に `DESIGN.md` を読み、CSS の値を変えたら `DESIGN.md` も合わせて直す。**
  見た目の作り替え（RPG らしくする）は本人が保留中のまま。Claude から方向の見直し（A）を始めない。B を画面の PR ごとに回すかは、保留中のアプリは回さない決まり（pr-flow）のまま
  （10-11 は本人の依頼で 1 回通し、10-12 は TODO の完了条件に B を通すとあったので通した。これからも回すかは本人に聞いていない）。点検の経緯は KNOWLEDGE.md 2026-10-11。
  比較ページには本人が「OK」：直した 8 か所はこのまま、好みの問題 P1〜P7 は今は直さない（`logs/decisions.md` 2026-10-12）。
  10-11 の残り 12 行は PR #108 で片付けた（9 行を直し、3 行は直さない理由。KNOWLEDGE.md 2026-10-12。比較ページ https://claude.ai/artifact/F8L4WxQTb3AaaF3HVEEbNB は返事不要で見せただけ）
- 判定：未判定は 0 本（orgflow-learning の 10-25 を `feat/judge-orgflow-1025` の PR で判定。その前の 10-18・10-22 は `feat/judge-orgflow-1018-1022` の PR。保留 2 件は承認ルートの用語の想起と、申請と段の組の外部キーの説明で、どちらも verifyBy に当たらない）。OrgFlow の基礎の確認（`/study-drill`）は本人のタスクとして後回し（TODO.md の保留 H3）
- **補足のラベルを減らした**（本人の希望。12 か所とも採用。`feat/fewer-labels` の PR。このマージ後の状態で書いている）。
  これからの画面の決まり：前置きの語・数の言い直し・凡例の文を足す前に、並びと形で伝わらないかを先に考える（DESIGN.md Layout の先頭、`logs/decisions.md` 2026-10-24）。
  学習パスの説明の 1 行（10-24 に折れ方を直したもの）も、この決定で外した
- 本人の答え待ちは `TODO.md` の「確認待ち（本人）」（未完は W21・W13・W15・W20 の 4 件。2026-10-29 に Beads の付箋をやめ、ここへ 1 つにまとめた）
- **CLAUDE.md の「開発ダッシュボード」の節を `.claude/rules/dashboard.md` へ移した**（2026-10-31。`docs/dashboard-rules` の PR。このマージ後の状態で書いている）。
  `dashboard/` か `.github/workflows/ci.yml` を触ると自動で読まれる。CLAUDE.md は 11,347 → 10,153 バイト。選んだことと測った値は KNOWLEDGE.md 2026-10-31。
  効き目を 2026-11-14 ごろまで見る（TODO.md 保留 H4）。`dashboard/` を触った会話で読まれなかったら CLAUDE.md に戻す
- リポジトリは Public。`main` はブランチ保護あり（PR 経由のみ・CI の 5 ジョブが必須）。細部は既存方針と整合する範囲で Claude が判断してよい（2026-09-25）

## 2. 次セッションで最初にやること

1. 起動時の「未判定の学習ログ n 本」が 1 本以上なら `/judge-log` で判定する（study のログなら確認なし）
2. フェーズ2.5 の次：AI 向けの説明書 `docs/build-your-own.md` を書く（完了条件は TODO）。写すもの・写さないものは、実際のファイルの一覧（`git ls-files`）から起こす。最後に Claude Code か Codex に説明書だけを読ませ、空のフォルダで見本のデータの画面が出るところまで試す
3. **本人への確認は、作業の区切りに 1 メッセージ 1 件ずつ**（TODO.md の確認待ちの順に、W21 → W13 → W15 → W20。「何のためか」を具体例から噛み砕き、おすすめを添える。
   次のセッションのプロンプトに「A／B」の選ぶ欄を入れない。KNOWLEDGE.md 2026-10-10）

## 3. 動作確認コマンド

```bash
# 開発ダッシュボード（進捗・人間待ち・CI・今のタスクを 1 画面で）。ブラウザが開き、開き直すと最新になる
node dashboard/update.mjs --serve --open

# 純粋関数と CLI のテスト（DB 不要）
go -C backend test ./internal/... ./cmd/skillmatrix -cover

# 判定から state.json を作り直す／コミット済みの state.json が最新か確かめる（CI 用）
go -C backend run ./cmd/skillmatrix recalc --data ../data
go -C backend run ./cmd/skillmatrix verify --data ../data
# 判定の根拠が指す学習ログの行を、理由の文と並べて出す（手元だけ。sources.local.json が要る）
go -C backend run ./cmd/skillmatrix refs <判定ファイル名>
# 未判定の学習ログを古い順に出す（--count なら本数だけ。手元だけ）
go -C backend run ./cmd/skillmatrix pending

# store を触ったら DB テストも回す（ローカルでは Postgres 無しだとスキップされ、CI だけが落ちる。KNOWLEDGE.md 2026-09-26）
docker compose up -d postgres
TEST_DATABASE_URL='postgres://skillmatrix:skillmatrix@localhost:5432/skillmatrix?sslmode=disable' go -C backend test ./internal/store/
docker compose stop postgres

# 画面（frontend）。http://localhost:5173/#/ が公開ビュー、#/plan が学習パス（作業ビュー）。JSON を変えると自動で作り直される
(cd frontend && npm ci && npm run dev)
# CI と同じ検査（E2E は Playwright。初回は npx playwright install chromium）
(cd frontend && npm run format:check && npm run lint && npm run typecheck && npx vitest run && npm run build && npm run e2e)

# 見た目の点検（/apps-workflow:look の B。撮影・検出器・DESIGN.md の検査は look.mjs。使い方はスキル）。ビルドした画面を preview で動かして撮り、撮り終えたら止める
(cd frontend && npm run build && npx vite preview --port 4410 --strictPort --host 127.0.0.1)
# ツリー表示は shoot の --setup で localStorage を書き換えてから撮る（KNOWLEDGE.md 2026-10-11）
```

## ここに書かないもの（行き先）

| 書きたくなったこと | 行き先 |
|---|---|
| ユーザーと合意した判断 | `logs/decisions.md`（設計・方針を提案する前に読む） |
| 設計判断の理由・ハマりどころ | `KNOWLEDGE.md`（**方針を変えたくなったら必ず先に読む**） |
| やること・確認待ち | `TODO.md` |
| やったこと | `git log` と PR |
