# 開発ダッシュボード

Claude Code に開発を任せているあいだ、人間がコードやログを追わずに「今どこ・何が終わった・何で止まっている・次は何・CI は緑か・自分の判断待ちは何か」を
1 画面で見るための観測画面。**ダッシュボード独自の状態は持たない。** 開くたびにリポジトリの正本を読み直して描くだけで、
ここに書き込むものは無い（試験導入 2026-09-26）。

## 起動

```bash
node dashboard/update.mjs              # データ（dashboard/data.js）を作り直す → dashboard/index.html をブラウザでダブルクリックして開く
node dashboard/update.mjs --serve      # 配信もする。http://127.0.0.1:8787/ を開くたびに再生成（10 秒以内の連続アクセスは前回の結果を使い回す）
node dashboard/update.mjs --serve --host 0.0.0.0 --port 8787   # 同じ LAN のスマートフォンから見るとき（認証は無いので LAN 内だけ）
```

**HTML だけでは動かない理由**：表示する中身（git の状態・CI・Beads）はブラウザからは読めないので、`update.mjs` が集めて `data.js` に書く工程が要る。
ブラウザで開くだけなら `--serve` は不要で、`node dashboard/update.mjs` を 1 回実行して `index.html` を開けばよい。最新にしたいときはもう一度実行して再読込する。

依存は Node の標準ライブラリだけ（npm install 不要、ビルド不要）。`git` は必須。`gh`（GitHub CLI、ログイン済み）・`bd`（Beads）・`go` は
無ければその項目が「取得できず」になるだけで、ほかは表示される。1 回の生成は数秒（`go run ... verify` と `gh` の呼び出し分）。

## 画面の並び（上ほど重要）と取得元

文章は一覧用に短く縮めて 1 行で出す（括弧書き・記法・2 文目以降を落とす）。全文はマウスを載せると出る。押すと全文に切り替わる（スマートフォン向け）。

| 表示 | 取得元（正本） |
|---|---|
| 異常の帯（CI 失敗・verify 失敗・HANDOFF の遅れ・取得失敗）。異常が無ければ出ない | 下の各正本から機械的に導く |
| タイル：進捗・あなた待ち・CI（main）・データ検証・作業ツリー | 下の各正本 |
| あなた待ち（判断・確認・作業の札つき） | Beads `bd list --json`（このプロジェクトの epic 配下でラベル `human`）、`TODO.md`「確認待ち」節の未完、`data/state.json` の `deferred`（保留中の判定。1 件以上のときだけ） |
| 今のタスクとこの後 4 件 | `TODO.md` の未完タスク（上から順） |
| CI（開いている PR とその CI の結果、main の最近の実行） | `gh pr list` / `gh run list` / `gh run view --json jobs` |
| 最近のコミット | `git log` |
| データ（このプロジェクト固有） | `data/roadmap.json` `data/state.json` `data/judgments/` の件数と分布、`skillmatrix verify` の結果 |
| 折りたたみ：引き継ぎメモ全文・最近の合意・verify の出力・未コミットのファイル・Claude 側の付箋・よく変わったファイル | `HANDOFF.md`・`logs/decisions.md`・`git status`・Beads・`git log` |

進捗の数え方は apps-workflow の `progress.sh` と同じ（`##` 見出し = フェーズ、「確認待ち」「保留」の節は合計から外す）。

## 更新と自動化

- 手動：`node dashboard/update.mjs` を実行してブラウザを再読込。`--serve` 中は開き直す（または 60 秒ごとの自動再読込）だけで最新になる
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

`TODO.md` の見出しの使い方（`##` = フェーズ、「確認待ち」「保留」）、`HANDOFF.md` の節名、CI のジョブ名、Beads の題名規約（`[skill-matrix] ...`）を
変えたら `update.mjs` の該当する読み取り関数を直す。表示項目を足すときは `collect()` に読み取りを 1 つ足し、`index.html` の `render()` にカードを 1 つ足す。
