#!/usr/bin/env bash
# SessionStart — 未判定の学習ログの本数を context に流し込む（logs/decisions.md 2026-10-03「判定を始めるきっかけ」）
#
# 本数は CLI の `skillmatrix pending --count`（SPEC.md §6）が数える。数え方の規則はここに書かない。
# stdout に書いた内容が Claude の context に入る（ユーザーの画面には出ないので、最初の返答で伝えるよう指示する）。
# sources.local.json（学習ログの置き場。コミットしない）が無い環境では何も出さない。
# 数えられなかったときもセッションの開始は止めない（終了コード 0）。理由の調べ方だけを出す。
set -uo pipefail

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$(pwd)}"
[ -f "$PROJECT_DIR/sources.local.json" ] || exit 0
command -v go > /dev/null 2>&1 || exit 0

# 起動を待たせすぎないよう 60 秒で打ち切る（初回は go のビルドで数秒かかる）
if ! COUNT=$(cd "$PROJECT_DIR" && timeout 60 go -C backend run ./cmd/skillmatrix pending --count 2> /dev/null); then
  echo "## 未判定の学習ログ"
  echo
  echo "本数を数えられなかった。理由は \`go -C backend run ./cmd/skillmatrix pending\` で出る。最初の返答で本人に 1 行で伝えること。"
  exit 0
fi

echo "## 未判定の学習ログ：${COUNT} 本"
echo
if [ "$COUNT" -gt 0 ] 2> /dev/null; then
  echo "**最初の返答で、進捗表のすぐ後に「未判定の学習ログが ${COUNT} 本あります。/judge-log で判定できます」と 1 行で伝えること。**"
  echo "どのログかは \`go -C backend run ./cmd/skillmatrix pending\` で出る。"
else
  echo "最初の返答で、進捗表のすぐ後に「未判定の学習ログは 0 本です」と 1 行で伝えること。"
fi
exit 0
