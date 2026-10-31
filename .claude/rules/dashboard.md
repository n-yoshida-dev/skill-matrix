---
paths:
  - "dashboard/**"
  - ".github/workflows/ci.yml"
---

# 開発ダッシュボード（`dashboard/`。試験導入 2026-09-26）

人間が進捗・人間待ち・CI・今のタスクを 1 画面で見る観測画面（文章は 1 行に縮め、引き継ぎメモ等の全文は折りたたみ）。普段は横断タスクボード（ops）の「詳しく見る」から開く。手元では `node dashboard/update.mjs --serve --open` で起動し、開き直すと TODO / HANDOFF / decisions / git / gh / `data/` を読み直す
（`--serve` 無しなら `dashboard/data.js` を作るだけで、`dashboard/index.html` を直接開く）。独自の状態は持たない。
使い方と取得元は `dashboard/README.md`。
**`TODO.md` の見出し規約（「確認待ち（本人）」の W 行の書式を含む）・`HANDOFF.md` の節名・CI のジョブ名を変えたら、`dashboard/update.mjs` の読み取りも合わせて直す。**
人間待ちは `TODO.md` の「確認待ち（本人）」だけに書く（書式は `~/.claude/CLAUDE.md`「TODO.md」の節。2026-10-29 に Beads をやめた）。
**CI のジョブ名を変えたら、`main` のブランチ保護の必須チェック名も直す**（一致しないと存在しないチェックを待ち続けてマージできなくなる。KNOWLEDGE.md 2026-09-27「ブランチ保護」）。
重要な指標が増えたら表示項目を足してよいが、ダッシュボードのために正本側へ欄を足さない。
