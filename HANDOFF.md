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
░░░░░░░░░░░░░░░░░░░░   0%  残り  4 /  4  フェーズ2d：リリース後の改善（少しずつ。2026-10-10 決定）
██████████████████░░  94%  残り  4 / 70  合計
あなたの回答待ち：10 件（回答済み 9 件）
保留（合計に含めない）：19 件（済み 15 件）

前回の区切り（2026-10-11）から：完了 +0 件、新たに見つかったタスク +2 件
```

## 1. 現在地

- **リリース済み**：https://n-yoshida-dev.github.io/skill-matrix/ （GitHub Pages。`main` への push ごとに `pages.yml` が公開する。push での公開も確かめた）。フェーズ1〜2c は全部完了。
  方針は「少しずつ改善する」（`logs/decisions.md` 2026-10-10）。残りはフェーズ2d の 4 件と確認待ち
- **見た目の決まりは `DESIGN.md`**（PR `fix/look-pass` のマージ後の状態で書いている）。今ある決まりを写し、2026-10-11 に `/apps-workflow:look` の「B 毎回の点検」を全画面に通して、
  字の濃さ・折り返し・フォーカス・Tab の順と、部品単位の「AI がよく作る形」を直した。**画面のファイルを変える前に `DESIGN.md` を読み、画面を変えた PR は B を通す。**
  見た目の作り替え（RPG らしくする）は本人が保留中のまま。Claude から方向の見直し（A）を始めない。点検の経緯と、直さなかった残りは KNOWLEDGE.md 2026-10-11
- 判定：未判定は 0 本（10-07・10-10・10-11 を PR #103 で判定）。OrgFlow の基礎の確認（`/study-drill`）は本人のタスクとして後回し（付箋 `ops-dhj.24`）
- **本人の答え待ち（いちばん新しい頼みごと）**：見た目の点検の比較ページ（TODO の確認待ちの先頭。https://claude.ai/artifact/92pk2ze8ZSSsUjqg4PgGtJ ）に、番号か「OK」
- 本人の答え待ちの付箋（1 件ずつ）：`.9` verifyBy → `.11` personal-ai-context → `.12` 提案の残り 2 件（(1) orgflow の `/study-log`・(3) Vitest） → `.13` `../CLAUDE.md` の 3 点 →
  react-01 の追い付かせ（TODO フェーズ2d）をいつ一緒に進めるか → `.14`（作業ビューの残りの細部）。`.19`（任意。手元の禁止語ファイル）
- リポジトリは Public。`main` はブランチ保護あり（PR 経由のみ・CI の 5 ジョブが必須）。細部は既存方針と整合する範囲で Claude が判断してよい（2026-09-25）

## 2. 次セッションで最初にやること

1. **比較ページに本人の答えがあれば、それに従う。** 直すと答えた番号は、`DESIGN.md` を読んでから直し、`/apps-workflow:look` の B を通して PR にする。
   直さないと答えたものは、`DESIGN.md` の該当の行に「本人の判断：直さない」と書く。答えが無ければ、作業の区切りに 1 回だけ聞く
2. 起動時の「未判定の学習ログ n 本」が 1 本以上なら `/judge-log` で判定する（study のログなら確認なし）
3. それ以外はフェーズ2d から。小さいのは「`judge.md` 手順 6 を実際の進め方に合わせる」（文章だけ）。
   **本人への確認は、作業の区切りに 1 メッセージ 1 件ずつ**（上の順。「何のためか」を具体例から噛み砕き、おすすめを添える。次のセッションのプロンプトに「A／B」の選ぶ欄を入れない。KNOWLEDGE.md 2026-10-10）

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
