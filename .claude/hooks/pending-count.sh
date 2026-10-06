#!/usr/bin/env bash
# SessionStart — 未判定の学習ログの本数を context に流し込む（logs/decisions.md 2026-10-03「判定を始めるきっかけ」）
#
# 本数は CLI の `skillmatrix pending --count`（SPEC.md §6）が数える。数え方の規則はここに書かない。
# stdout に書いた内容が Claude の context に入る（ユーザーの画面には出ないので、最初の返答で伝えるよう指示する）。
# sources.local.json（学習ログの置き場。コミットしない）が無い環境（CI・他人の手元）では何も出さない。
# それ以外で数えられなかったとき（go が見つからない・pending が失敗した）は、黙らずに理由を出す。
# どの場合もセッションの開始は止めない（終了コード 0）。
set -uo pipefail

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$(pwd)}"
[ -f "$PROJECT_DIR/sources.local.json" ] || exit 0

# cannot_count は数えられなかった理由を出す。最初の返答で本人に伝えさせる
cannot_count() {
  echo "## 未判定の学習ログ"
  echo
  echo "本数を数えられなかった（$1）。最初の返答で、進捗表のすぐ後に本人に 1 行で伝えること。"
  echo "理由は \`go -C backend run ./cmd/skillmatrix pending\` で出る。"
  exit 0
}

command -v go > /dev/null 2>&1 || cannot_count "このフックの環境に go が見つからない"

# pending が出す警告（読めない根拠があると、判定済みのログが未判定に数えられる）を拾うため、stderr は一時ファイルに受ける
ERR_FILE=$(mktemp) || cannot_count "一時ファイルを作れない"
# 起動を待たせすぎないよう 60 秒で打ち切る（初回は go のビルドで数秒かかる）
COUNT=$(cd "$PROJECT_DIR" && timeout 60 go -C backend run ./cmd/skillmatrix pending --count 2> "$ERR_FILE")
STATUS=$?
WARNINGS=$(grep -c '^警告:' "$ERR_FILE")
FIRST_ERR=$(grep -v '^警告:' "$ERR_FILE" | head -1)
rm -f "$ERR_FILE"

if [ "$STATUS" -ne 0 ] || ! [ "$COUNT" -ge 0 ] 2> /dev/null; then
  cannot_count "pending が終了コード ${STATUS} で失敗：${FIRST_ERR:-理由の出力なし}"
fi

echo "## 未判定の学習ログ：${COUNT} 本"
echo
if [ "$COUNT" -gt 0 ]; then
  echo "**最初の返答で、進捗表のすぐ後に「未判定の学習ログが ${COUNT} 本あります。/judge-log で判定できます」と 1 行で伝えること。**"
  echo "どのログかは \`go -C backend run ./cmd/skillmatrix pending\` で出る。"
else
  echo "最初の返答で、進捗表のすぐ後に「未判定の学習ログは 0 本です」と 1 行で伝えること。"
fi
if [ "$WARNINGS" -gt 0 ]; then
  echo "ただし pending が警告を ${WARNINGS} 件出した（判定ファイルに読めない根拠があり、本数が多めに出ている可能性がある）。そのことも 1 行で伝え、\`pending\` で確かめる。"
fi
exit 0
