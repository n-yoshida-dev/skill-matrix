# 開発ダッシュボード

Claude Code に開発を任せているあいだ、人間がコードやログを追わずに「今どこ・何が終わった・何で止まっている・次は何・CI は緑か・自分の判断待ちは何か」を
1 画面で見るための観測画面。**ダッシュボード独自の状態は持たない。** 開くたびにリポジトリの正本を読み直して描くだけで、
ここに書き込むものは無い（試験導入 2026-09-26）。

**普段の入口は横断タスクボード（ops）の「開発状況」タブ。** この画面はカードの「詳しく見る」の行き先（2026-10-28）。下の起動は、手元で単独に見たいときだけ使う。

## 起動

```bash
node dashboard/update.mjs --serve --open   # ブラウザが開く。開き直すと最新になる（ターミナルは起動したまま置く）
node dashboard/update.mjs                  # データ（dashboard/data.js）を作り直すだけ → dashboard/index.html をダブルクリックで開く
node dashboard/update.mjs --serve --host 0.0.0.0 --port 8787   # 同じ LAN のスマートフォンから見るとき（認証は無いので LAN 内だけ）
```

**ターミナルが要る理由**：表示する中身（git の状態・CI）はブラウザからは読めない。`--serve` で起動した Node のプロセスがブラウザの代わりに
git / gh を実行し、ページを開き直したとき（10 秒以上たっていれば）とハブからの `POST /update` で `data.js` を作り直す。
起動は 1 回でよく、閉じるまで動き続ける（VS Code のターミナルで起動しておくのが楽）。

依存は Node の標準ライブラリだけ（npm install 不要、ビルド不要）。`git` は必須。`gh`（GitHub CLI、ログイン済み）・`go` は
無ければその項目が「取得できず」になるだけで、ほかは表示される。1 回の生成は数秒（`go run ... verify` と `gh` の呼び出し分）。

## 画面の並び（上ほど重要）と取得元

説明の文・欄の名前・種類の札は出さず、並びと大きさで読ませる。文章は一覧用に短く縮めて 1 行で出す（括弧書き・記法・2 文目以降を落とす）。
全文・行番号はマウスを載せると出る。押すと全文に切り替わる（スマートフォン向け）。

| 表示 | 取得元（正本） |
|---|---|
| 異常の帯（CI 失敗・verify 失敗・未判定の学習ログを数えられない／警告あり・HANDOFF の遅れ・取得失敗）。異常が無ければ出ない | 下の各正本から機械的に導く |
| あなた待ち（判断・確認・作業のタグつき。上位 6 件、残りは「ほか N 件」） | `TODO.md`「確認待ち（本人）」節の未完（W 番号を頭に付けて出す。2026-10-29 に Beads をやめ、人間待ちの正本はここだけ）、`data/state.json` の `deferred`（保留中の判定。1 件以上のときだけ）、`skillmatrix pending --count`（未判定の学習ログ。1 本以上のときだけ） |
| 進捗の帯（済み／全体）・今のフェーズ・今のタスク（大きく）とこの後 4 件 | `TODO.md` の未完タスク（上から順） |
| 開いている PR とその CI、最近のコミット 6 件（main の CI の結果は行の先頭の記号 ✓ ✕ ●） | `gh pr list` / `gh run list --workflow ci.yml`（GitHub Pages の公開の実行は含めない） / `gh run view --json jobs` / `git log` |
| データ（このプロジェクト固有） | `data/roadmap.json` `data/state.json` `data/judgments/` の件数と分布、`skillmatrix verify` の結果、`skillmatrix pending --count` の未判定ログの本数（`sources.local.json` が無ければ「-」） |
| 折りたたみ：引き継ぎメモ全文・最近の合意・未コミットのファイル（あるときだけ）。verify の出力は固有の指標のカードの中 | `HANDOFF.md`・`logs/decisions.md`・`git status` |

進捗の数え方は todo-progress（apps-workflow が依存する公開版）の `progress.sh` と同じ（`##` 見出し = フェーズ、「確認待ち」「保留」の節は合計から外す）。

## 更新と自動化

- 横断タスクボード：ops の会話で「ダッシュボード更新して」と頼む（ops の `snippets/build_board_dashboards.py` が作り直して載せ直す）
- 手元：`--serve` 中は開き直す。ファイルを直接開く使い方なら `node dashboard/update.mjs` を実行して再読込
- **フック等での自動更新は入れていない。** 配信モードが開くたびに正本を読み直すので、別の更新経路（Claude Code フック・git フック・CI）を足すと
  同じ結果を二重に作ることになり、セッションごとに数秒の待ちが増えるだけだから
- `dashboard/data.js` は派生物なので `.gitignore` 済み。コミットしない

## 構成

```
dashboard/
  index.html   画面。data.js を読んで描く（HTML + CSS + 素の JS、ライブラリなし。ダブルクリックで開ける）
  update.mjs   データ生成と配信（Node 標準ライブラリだけ）
  data.js      生成物（`window.DASHBOARD_DATA = {...}`。git 管理外）
  README.md    これ
```

## 直すとき

`TODO.md` の見出しの使い方（`##` = フェーズ、「確認待ち」「保留」）と確認待ちの行の書式（`W<番号>【判断】…`）、`HANDOFF.md` の節名、CI のジョブ名を
変えたら `update.mjs` の該当する読み取り関数を直す。表示項目を足すときは `collect()` に読み取りを 1 つ足し、`index.html` の `render()` にカードを 1 つ足す。
