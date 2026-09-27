---
name: judge-log
description: 学習ログを読んで理解度を判定し、data/judgments/ に判定ファイルを 1 つ書く（指示書 prompts/judge.md の入口）。「学習ログを判定して」「理解度に反映して」「judge-log」で起動。study に学習ログをコミットしたあと、Claude 側から提案してよい。
argument-hint: 判定するファイルのパス（例 go-react/logs/2026-09-28.md）。省略時は未判定のログを古い順に 1 件探す
---

`prompts/judge.md` を最初から最後まで読み、その「手順」どおりに進める。判定の基準・禁止事項・出力の形・手順はすべてそこにあり、このファイルには書かない。

指定されたファイル: $ARGUMENTS

- 空なら、`prompts/judge.md` 手順 2 の自動の探索から始める。パスがあれば、手順 2 の「ユーザーがファイルを指定したら」として扱う
- 判定ファイルの `judge` は `claude-code`
- 手順 7 の報告で止まる。ユーザーが差分を読んでコミットを頼んだら、`apps-workflow:pr-flow` の手順で PR にする
