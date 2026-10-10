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
進捗（TODO.md より自動集計・2026-10-11）

████████████████████ 100%  残り  0 / 11  フェーズ1：設計（完了）
████████████████████ 100%  残り  0 / 42  フェーズ2：v1 実装（自分専用版。ゴールは「画面で自分のマトリクスが見える」こと）
████████████████████ 100%  残り  0 /  5  フェーズ2b：スキルツリーを RPG のスキルツリーのように読める形にする（2026-10-03 決定）
████████████████████ 100%  残り  0 /  8  フェーズ2c：判定の回し方を作り直す（2026-10-03 決定）
░░░░░░░░░░░░░░░░░░░░   0%  残り  2 /  2  フェーズ2d：リリース後の改善（少しずつ。2026-10-10 決定）
███████████████████░  97%  残り  2 / 68  合計
あなたの回答待ち：9 件（回答済み 9 件）
保留（合計に含めない）：19 件（済み 15 件）

前回の区切り（2026-10-10）から：完了 +1 件、新たに見つかったタスク +0 件
```

## 1. 現在地

- **リリース済み（2026-10-11）**：https://n-yoshida-dev.github.io/skill-matrix/ （GitHub Pages。`main` への push ごとに `pages.yml` が公開する）。フェーズ1〜2c は全部完了
  （PR `docs/released-on-pages` のマージ後の状態で書いている）。置き場所は本人が Vercel と迷ったうえで GitHub Pages、リポジトリは Public のまま（`logs/decisions.md` 2026-10-11）
- **方針：少しずつ改善する**（本人「リリースできる状態になったら一旦リリースして、少しずつ改善していきたい。」`logs/decisions.md` 2026-10-10）。残りはフェーズ2d「リリース後の改善」の 2 件（react-01・SPEC の追い付き）と確認待ち
- 判定の流れ：`/judge-log` は `pending` で探して古い順にまとめて判定し、別の AI（突き合わせ役）が理由の文とログの行を突き合わせる。誤った判定は `retractions` で外せる。未判定は 0 本。
  Qiita の記事は判定の材料にしない（本人「足さないでOK。」）。OrgFlow の基礎の確認（`/study-drill`）は本人のタスクとして後回し（付箋 `ops-dhj.24`）
- 本人のタスク：ポートフォリオサイトの作品のリンクに公開 URL を足す（portfolio のセッションで頼む。付箋 `ops-dhj.25`）
- 本人の答え待ちの付箋（1 件ずつ）：`.9` verifyBy → `.11` personal-ai-context → `.12` 提案の残り 2 件（(1) orgflow の `/study-log`・(3) Vitest。(2) は回答済み） → `.13` `../CLAUDE.md` の 3 点 →
  react-01 の追い付かせ（TODO フェーズ2d）をいつ一緒に進めるか。→ `.14`（作業ビューの残りの細部）。`.19`（任意。手元の禁止語ファイル）
- リポジトリは Public。`main` はブランチ保護あり（PR 経由のみ・CI の 5 ジョブが必須）。細部は既存方針と整合する範囲で Claude が判断してよい（2026-09-25）

## 2. 次セッションで最初にやること

1. 起動時の「未判定の学習ログ n 本」が 1 本以上なら `/judge-log` で判定する（study のログなら確認なし）。判定をマージしたら、公開 URL に反映されたかを見る
2. **本人への確認は、作業の区切りに 1 メッセージ 1 件ずつ。** 上の付箋の順に、「何のためか」を先に噛み砕き、おすすめを添えて聞く（2026-09-29 本人：「…ヒアリングとか作業指示とかを出すところから始めて」）。
   用語は前のセッションで説明していても、具体例から説明し直す。次のセッションのプロンプトに「A／B」の選ぶ欄を入れない（KNOWLEDGE.md 2026-10-10）。
   9/28 の判定はもう読んでもらったので、Pages（`.4.1`）の前に読むかどうかは聞かなくてよい

## 3. 動作確認コマンド

```bash
# 開発ダッシュボード（進捗・人間待ち・CI・今のタスクを 1 画面で）。ブラウザが開き、画面の「更新」ボタンで最新にできる
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

# 画面（frontend）。http://localhost:5173/#/ が公開ビュー、#/plan が作業ビュー。JSON を変えると自動で作り直される
(cd frontend && npm ci && npm run dev)
# CI と同じ検査（E2E は Playwright。初回は npx playwright install chromium）
(cd frontend && npm run format:check && npm run lint && npm run typecheck && npx vitest run && npm run build && npm run e2e)
```

## ここに書かないもの（行き先）

| 書きたくなったこと | 行き先 |
|---|---|
| ユーザーと合意した判断 | `logs/decisions.md`（設計・方針を提案する前に読む） |
| 設計判断の理由・ハマりどころ | `KNOWLEDGE.md`（**方針を変えたくなったら必ず先に読む**） |
| やること・確認待ち | `TODO.md` |
| やったこと | `git log` と PR |
