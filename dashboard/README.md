# 開発ダッシュボード

Claude Code に開発を任せているあいだ、人間がコードやログを追わずに「今どこ・何が終わった・何で止まっている・次は何・CI は緑か・自分の判断待ちは何か」を
1 画面で見るための観測画面。**ダッシュボード独自の状態は持たない。** 開くたびにリポジトリの正本を読み直して描くだけで、
ここに書き込むものは無い（試験導入 2026-09-26）。

## 起動

```bash
node dashboard/update.mjs --serve      # http://127.0.0.1:8787/ を開く。/data.json は開くたびに再生成される
node dashboard/update.mjs              # 配信せず dashboard/data.json を作り直すだけ
node dashboard/update.mjs --serve --host 0.0.0.0 --port 8787   # 同じ LAN のスマートフォンから見るとき（認証は無いので LAN 内だけ）
```

依存は Node の標準ライブラリだけ（npm install 不要、ビルド不要）。`git` は必須。`gh`（GitHub CLI、ログイン済み）・`bd`（Beads）・`go` は
無ければその項目が「取得できず」になるだけで、ほかは表示される。1 回の生成は数秒（`go run ... verify` と `gh` の呼び出し分）。

## 表示するもの（上ほど重要）と取得元

| 表示 | 取得元（正本） |
|---|---|
| 注意（CI 失敗・verify 失敗・HANDOFF の遅れ・未コミット・未 push） | 下の各正本から機械的に導く |
| 人間の判断・作業待ち | Beads `bd list --json`（このプロジェクトの epic 配下でラベル `human`）と `TODO.md`「確認待ち」節の未完 |
| 進捗（フェーズ別の達成率・残り件数・現在のフェーズと節） | `TODO.md`（apps-workflow の `progress.sh` と同じ数え方） |
| 現在地と次の一手 | `HANDOFF.md` の「現在地」「次セッションで最初にやること」節と、その最終コミットからの遅れ |
| 次のタスク | `TODO.md` の未完タスク先頭 10 件 |
| CI と PR | `gh pr list` / `gh run list` / `gh run view --json jobs`（frontend / backend / 秘密情報の各ジョブ） |
| Git | `git status` / `git log` / upstream との差分 / 直近 14 日でよく変わったファイル |
| データ（このプロジェクト固有） | `data/roadmap.json` `data/state.json` `data/judgments/` の件数と分布、`skillmatrix verify` の結果 |
| 最近の合意 | `logs/decisions.md` の見出し |

## 更新と自動化

- 手動：上の 1 コマンド。`--serve` 中はブラウザで開き直す（または 60 秒ごとの自動再読込）だけで最新になる
- **フック等での自動更新は入れていない。** 配信モードが開くたびに正本を読み直すので、別の更新経路（Claude Code フック・git フック・CI）を足すと
  同じ結果を二重に作ることになり、セッションごとに数秒の待ちが増えるだけだから
- `dashboard/data.json` は派生物なので `.gitignore` 済み。コミットしない

## 構成

```
dashboard/
  index.html   画面。data.json を読んで描く（HTML + CSS + 素の JS、ライブラリなし）
  update.mjs   データ生成と配信（Node 標準ライブラリだけ）
  data.json    生成物（git 管理外）
  README.md    これ
```

## 直すとき

`TODO.md` の見出しの使い方（`##` = フェーズ、「確認待ち」「保留」）、`HANDOFF.md` の節名、CI のジョブ名、Beads の題名規約（`[skill-matrix] ...`）を
変えたら `update.mjs` の該当する読み取り関数を直す。表示項目を足すときは `collect()` に読み取りを 1 つ足し、`index.html` の `render()` にカードを 1 つ足す。
