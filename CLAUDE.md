# {{APP_NAME}}

{{一行で「何をするアプリか」。SETUP.md の手順で埋めること}}

## セッション開始時にすること

1. **`HANDOFF.md` を読む** — 現在地・決定事項・次のタスクがすべてここにある
2. {{個人データを扱うアプリなら「実データが必要な作業なら `PRIVATE.md` を読む（.gitignore 対象）」。扱わないなら、この行ごと削除する}}

未完タスクは SessionStart フックが `TODO.md` から自動で提示する。
セッションの区切りには `/handoff` を実行して引き継ぎを書く。

## ドキュメントの役割

| ファイル | 役割 | 読み手 |
|---|---|---|
| `HANDOFF.md` | セッション引き継ぎ・現在地 | AI |
| `PLAN.md` | 何を作るか・なぜ作るか | AI |
| `SPEC.md` | 確定仕様。実装が参照する正本 | AI |
| `TODO.md` | タスクと進捗 | AI |
| `KNOWLEDGE.md` | 設計判断・ハマりどころ | AI |
| `docs/` | 要件定義・アーキテクチャの解説 | 人間 |

**同じ事実を AI 用と人間用の両方に書かない。** AI 用から人間用へリンクする。

## 守ること

{{このアプリ固有の制約をここに書く。以下は雛形の例。該当しないものは消す}}

- **秘密情報・個人の実数値をリポジトリに入れない。** サンプル・プリセットはすべてダミー値。
  実データは `PRIVATE.md` と gitignore 対象の `*.local.json` で扱う
- **`{{ドメインロジックのディレクトリ}}` は純粋関数のみ。** React・DOM・localStorage を import しない
- **外部由来の数値（税率・料金表・制度の閾値）をコードに直書きしない。**
  JSON に分離し、`source` と `checkedAt` を記録する
- ロジックを変更したら、対応する単体テストを同時に更新する

## Claude Code の設定

| 種別 | 中身 |
|---|---|
| `/handoff` | HANDOFF / TODO / KNOWLEDGE を更新して次のセッションへ渡す |
| `guard-secrets.sh` | 秘密情報・ローカル専用ファイルのコミットを阻止（PreToolUse） |
| `check-edited.sh` | frontend の typecheck / lint、backend の go vet（PostToolUse） |
| `session-briefing.sh` | TODO.md の未完タスクを起動時に提示（SessionStart） |

固有のルールが増えたら `.claude/rules/` に切り出し、CLAUDE.md からはリンクだけにする。

## 共通ルール

自作プロダクト共通の開発ルール（技術スタック・Git運用・コーディング規則）は `../CLAUDE.md` にある。
上位ディレクトリの CLAUDE.md は自動でロードされる。
