# 自分用に作るには（AI 向けの説明書）

読み手は利用者の手元の AI（Claude Code・Codex CLI など、ファイルを読み書きしてコマンドを実行できるエージェント）。
利用者はあなたに「`https://github.com/n-yoshida-dev/skill-matrix` の `docs/build-your-own.md` を読んで、自分用の skill-matrix を作って」と頼んでいる。
この説明書のとおりに、利用者の空のフォルダへ必要な部品を写し、まず見本のデータで画面を出し、そのあと利用者のロードマップと学習ログに切り替える。

- 約束は「参考にどうぞ」。このリポジトリは作者が自分用に作ったもので、利用者の環境で動くことの保証や、作者側の更新の取り込みは無い。写したものは利用者のものとして直してよい
- 判定の基準・根拠の種類・検査のルールは、この説明書に書き写していない。正本は写した先の `prompts/judge.md`・`data/roadmap.json`・`backend/internal/domain/types.go` にある
- 人が読む紹介は [README](../README.md)、仕様の正本は [SPEC.md](../SPEC.md)、仕様の解説は [docs/spec-guide.md](spec-guide.md)

## 0. 完成形を先に見る

| 画面 | URL |
|---|---|
| 見本（架空の学習者のデータ。この説明書で最初に作るのはこれと同じ画面） | https://n-yoshida-dev.github.io/skill-matrix/demo/ |
| 作者の実物 | https://n-yoshida-dev.github.io/skill-matrix/ |

画面は公開ビュー `#/`（分野ごとの要約・能力値チャート・項目のタイル・根拠）、学習パス `#/plan/path/<分野>`（依存順の一列とスキルツリー）、項目詳細 `#/plan/item/<項目キー>` の 3 つ。
どれもビルド時に JSON を取り込んだ静的サイトで、サーバも DB もログインも無い。

## 1. 仕組み

```
学習ログ（Markdown。利用者が書く。リポジトリにはコミットしない）
   │  あなた（AI）が prompts/judge.md を読んで判定する
   ▼
判定ファイル data/judgments/*.json（ログ 1 本に 1 ファイル。追記のみ）
   │  Go の CLI の recalc：形の検査 → 検証ルール V1〜V8 → 適用
   ▼
理解度 data/state.json（CLI だけが書く。手で直さない）
   │  画面のビルド時に取り込む
   ▼
画面（React の静的サイト。GitHub Pages などに置ける）
```

| 部品 | 中身 | 詳しくは |
|---|---|---|
| データ `data/` | `roadmap.json`（分野・項目・レベルの基準）、`judgments/`（判定の履歴）、`state.json`（再計算の結果）、`settings.json`（閾値・禁止語・画面の設定）、`certifications.json`（資格。任意） | [data/README.md](../data/README.md)、SPEC.md §2・§3 |
| 判定の指示書 `prompts/` | `judge.md`（学習ログ → 判定ファイル）、`check-judgment.md`（別の AI が理由の文と根拠の行を突き合わせる） | SPEC.md §4 |
| CLI `backend/cmd/skillmatrix` | `recalc`（検査して `state.json` を書く）、`verify`（CI 用。書かずに一致を見る）、`pending`（未判定のログを出す）、`refs`（根拠の行を出す） | SPEC.md §6 |
| 画面 `frontend/` | Vite + React + TypeScript。`@data` という別名で `../data` を取り込む（`frontend/vite.config.ts`） | SPEC.md §7 |

理解度の考え方のうち、利用者に説明が要るのは次の 2 つ。

- 理解度は 0〜5 の段。判定は段に「印」を付け、1 から途切れずに印がある最上段が Verified Level になる
- 実装の根拠（3）は基礎の理解（1・2）を含まない。作っただけの項目は「基礎が未確認」と出る（見本の `react-routing` など）

## 2. 写すもの

元のリポジトリは利用者のフォルダの外（一時的な場所）に clone し、そこから下の表のものだけを写す。**フォルダの並びは変えない**（`prompts/judge.md` と CLI が `data/`・`backend/` の相対パスを前提にしている）。
「写したあとに直すところ」は、時期の書いてあるもの（4 の手順 3・5 節）を除き、写すときに一緒に直す（4 の手順 2）。

| 写すもの | 写した先 | 写したあとに直すところ |
|---|---|---|
| `examples/demo/data/` の中身 | `data/` | 最初はそのまま（見本の画面を出すため）。自分用に切り替えるのは 4 の手順 5 |
| `prompts/judge.md`・`prompts/check-judgment.md` | `prompts/` | 作者専用の部分を直す（5） |
| `backend/go.mod`・`backend/go.sum` | `backend/` | `module` の行を利用者のモジュール名に変え、`go mod tidy` で使わない依存を落とす（4 の手順 3） |
| `backend/cmd/skillmatrix/` | `backend/cmd/skillmatrix/` | import のモジュール名を合わせる |
| `backend/internal/domain/`・`internal/judgment/`・`internal/roadmap/` | 同じ場所 | 同上。CLI が使うのはこの 3 つだけ（`go list -deps ./cmd/skillmatrix` で確かめられる） |
| `backend/testdata/` | 同じ場所 | なし（CLI と `roadmap` のテストが読むダミーのデータ） |
| `frontend/`（`node_modules/`・`dist/` を除く） | `frontend/` | 見本の画面を作る部分を外す：`package.json` の `build:demo`、`vite.config.ts` の `mode === 'demo'` の分岐（写した先に `examples/` は無い）。`src/features/plan/` に棚上げの部品（`Dashboard.tsx` など）が残っているが、学習パスと項目詳細が同じフォルダの `matrix.ts`・`next.ts` を使っているので分けない。E2E（`e2e/`・`playwright.config.ts`）は要らなければ外してよい |
| `sources.local.json.example` | 同じ場所 | `repos` のキーを消す（作者の学習リポジトリを指している） |
| `.gitignore` | 同じ場所 | `learning-logs/` と `*.local.json` の行は残す（学習ログの本文と手元の設定をコミットしないため）。`dashboard/data.js`・`backend/server` などの作者用の行は、消しても残しても害は無い |
| `.github/workflows/ci.yml`・`pages.yml`（任意） | 同じ場所 | 両方から見本の手順（`build:demo`・`examples/demo/data` の `verify`）を外す。`ci.yml` の `backend` のジョブからは、棚上げのコードの DB テスト用の Postgres（`services:` と `TEST_DATABASE_URL`）を外す。作者の文書を指すコメントは消してよい |

## 3. 写さないもの

| 写さないもの | 理由 |
|---|---|
| `data/`（作者の実データ） | 作者の判定と理解度。写すと利用者の理解度に混ざる |
| `sources.local.json`・`learning-logs/` | もともとリポジトリに無い（手元だけ）。利用者が自分で作る |
| `CLAUDE.md`・`HANDOFF.md`・`TODO.md`・`KNOWLEDGE.md`・`PLAN.md`・`SPEC.md`・`DESIGN.md`・`logs/`・`dashboard/`・`docs/` | 作者の開発用の文書と道具。必要なら元のリポジトリを読みに行けばよい |
| `.claude/`・`.mcp.json` | 作者の Claude Code の設定（フック・作者のプラグイン・判定の指示書を読ませるスキル）。指示書を読ませる決まりは利用者の環境に合わせて作る（4 の手順 7） |
| `backend/cmd/server`・`cmd/worker`・`internal/config`・`httpapi`・`llm`・`store`・`worker`・`migrations/`・`Dockerfile`・`.env.example`・`docker-compose.yml` | 棚上げのコード（サーバ・DB・API での判定。SPEC.md §10）。CLI は使わない |
| `prompts/migrate-skill-map.md`・`docs/skill-map-migration.md` | 作者の古い記録からの 1 回きりの移行用 |
| `README.md`・`LICENSE` | README は利用者向けに新しく書く。ライセンスは元の README の「ライセンス」節に従う |

## 4. 手順（空のフォルダから）

1. 元のリポジトリを一時的な場所に clone する。利用者のフォルダの中には clone しない（写さないものまで入るため）
2. 2 の表のとおり写す。利用者のフォルダで `git init` しておく（判定ファイルと `state.json` は履歴として残すもので、`pending` も git を使う）
3. CLI を動かす。`backend/go.mod` の `module` を利用者のもの（例：`github.com/<利用者>/<リポジトリ名>`）に変え、写した Go のファイルの import の
   `github.com/n-yoshida-dev/skill-matrix/` を同じ名前に置き換える。続けて、フォルダの直下で

   ```bash
   go -C backend mod tidy
   go -C backend test ./...
   go -C backend run ./cmd/skillmatrix verify --data ../data
   ```

   `tidy` で外部の依存が 0 になり、`go.sum` は空になる（失敗ではない）。`verify` が通れば、見本のデータの `state.json` と再計算の結果が一致している
   （見本に最初から入っている保留 1 件・記録 1 件も出る）。Go のバージョンは `backend/go.mod` の `go` の行のとおり。
   置き換えのあとも `internal/judgment` のテストの文字列に `repo:n-yoshida-dev/study@…` が残るが、根拠の書式の例なので直さなくてよい
4. 画面を出す。Node.js は 24 を使う

   ```bash
   (cd frontend && npm ci && npm run build)
   ```

   ビルドが通ったら、利用者が見るための開発サーバ（`npm run dev`。終わらないコマンド）を**バックグラウンドで**起動し、http://localhost:5173/#/ を利用者に伝える。
   0 の見本と同じ 5 分野（React・TypeScript・Go の API・データベース・CI とデプロイ）が出れば、ここまでは完了。
   ブラウザを持たないあなたが確かめるなら、HTML には画面の中身が入っていないので、ビルドされた `frontend/dist/assets/*.js` に 5 分野の名前があるかを見る。
   見本の `data/settings.json` は「今日」を `site.fixedToday` で固定し、見本だと書いた 1 行（`site.notice`）を出している。
   **ここで利用者に画面を見せ、続けてよいか聞く。** 利用者が見終わったら開発サーバを止める（止めると URL は開けなくなる。また見るときは `npm run dev` で起動し直す）
5. 利用者用に切り替える。
   - `data/roadmap.json` を利用者のロードマップにする（形は SPEC.md §2。見本の roadmap を下敷きにして分野と項目を差し替えるのが早い）。
     外部のロードマップを元にするなら `source`（出典）と `checkedAt`（確認日）を書く。項目の `verifyBy` には「次の段を何で確かめるか」を技術的な条件で書く
   - `data/judgments/` を空にし、`data/certifications.json` を利用者の資格にする（無ければ空の一覧）
   - `data/settings.json` の `site` から `fixedToday` と `notice` を消し、`repoUrl` を利用者のリポジトリにする（無いと作者のリポジトリへのリンクが出る）。
     `forbiddenWords`（公開される文に入れない語）を利用者に合わせる
   - `go -C backend run ./cmd/skillmatrix recalc --data ../data` で `state.json` を作り直し、画面が空の状態になることを確かめる
6. 最初の判定を 1 本通す。`cp sources.local.json.example sources.local.json`、`mkdir -p learning-logs` で置き場を作り、利用者に学習ログを 1 本書いてもらう
   （形は自由。やったこと・自分の言葉で説明できたこと・答えた確認問題・作ったものが書いてあれば根拠になる）。
   `learning-logs/` のログはコミットしない（`.gitignore` 済み。判定の根拠は `log:learning-logs/<ファイル>#L…` の形で指す）。
   別の Git リポジトリに書いているログを読むなら `sources.local.json` の `repos` に足す（形は SPEC.md §3.5）。こちらは判定がコミットを指すので、コミット済みのログだけが対象になる。
   `go -C backend run ./cmd/skillmatrix pending` で未判定のログが出ることを確かめ、そのあと `prompts/judge.md` を読んで判定し、`recalc` を通し、画面の項目詳細で印が増えたことを確かめる
7. 判定のときに指示書を読ませる決まりを作る。利用者が毎回この説明書を渡さなくて済むように、使っている AI の決まりのファイルに 1 行書く。
   例：Codex なら `AGENTS.md`、Claude Code なら `CLAUDE.md` に「学習ログの判定は `prompts/judge.md` を読んで、その手順どおりに行う」。
   作者は Claude Code のスキル（元のリポジトリの `.claude/skills/judge-log/SKILL.md`）にしている。スキルに書いてあるのは指示書を読ませることだけで、基準は書いていない
8. 公開するなら、判定ファイルの `rationale` と `evidenceRefs` は誰でも読めるようになる。Public にする前に全件を利用者と読み、所属先・人名などの固有名詞が無いことを確かめる。
   GitHub Pages は `pages.yml` を写し、リポジトリの Settings → Pages → Source を「GitHub Actions」にする（利用者の操作）

## 5. `prompts/judge.md` を利用者用に直す

指示書の前半（「あなたの役割」〜「出力の形」）は汎用で、基準は `data/roadmap.json` と `types.go` から読ませている。直すのは作者の環境に結び付いた次の部分だけ。

| 場所 | 作者専用の中身 | 直し方 |
|---|---|---|
| 冒頭 | `skill-map.md` からの移行への言及 | 消す |
| 「あなたの役割」・手順 7 | 作者の学習リポジトリ（`study`）のログなら本人の確認を省く決まり | 消す。確認を省くかは利用者に決めてもらう |
| 「出力の形」の例 | `repo:n-yoshida-dev/study@…` | 利用者の置き場の例に変える（`log:learning-logs/<日付>.md#L…` など） |
| 手順 1・2、「チャットに貼って使う場合」 | 「凍結コミット」「移行」「台帳」（作者が古い記録から移したときの決まり） | 移行が無ければ、その段落ごと消す。`pending` の出力に「凍結」は出てこない |
| 手順 6 | 突き合わせ役（別の AI）に `check-judgment.md` を読ませる | 残してよい（作り話の理由を減らす）。別の AI を起動できない環境なら、`refs` の出力を利用者と一緒に読む形にする |
| 手順 7・8 | 確認の表の書式、作者のプラグインの PR の手順（`apps-workflow:pr-flow`）、`TODO.md` の書式 | 「`recalc` が通ったら利用者に判定の文を見せ、OK ならコミットする」程度に縮める |

直したら、`grep -n "study\|凍結\|移行\|apps-workflow\|TODO.md" prompts/judge.md` で残りが無いかを見る。

## 6. 利用者が最初に渡す頼み方の例

利用者に聞かれたら、この形を案内する（利用者が自分の AI に貼る文）。

```
https://github.com/n-yoshida-dev/skill-matrix の docs/build-your-own.md を読んで、
このフォルダに自分用の skill-matrix を作ってください。
まず見本のデータで画面が出るところまで進めて、画面の URL を教えてください。
そのあと、私のロードマップ（〇〇の学習。分野は △△・□□）に切り替えるのを手伝ってください。
```
