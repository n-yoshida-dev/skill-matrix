# skill-matrix

ロードマップをマスタとして、投稿された学習ログを AI が読み取り、分野 × 詳細項目の理解度マトリクスと学習スケジュールを更新するアプリ。

## セッション開始時にすること

1. **`HANDOFF.md` を読む** — 現在地・決定事項・次のタスクがすべてここにある

未完タスクは SessionStart フックが `TODO.md` から自動で提示する。
セッションの区切りには `/apps-workflow:handoff` で引き継ぎを書く。
**`/apps-workflow:handoff` はユーザー起動限定で、Claude からは呼べない**（`/apps-workflow:pr-check` と同じ扱い。
どちらも `disable-model-invocation` が付いている）。区切りでは Claude が呼ぼうとせず、**ユーザーに実行を頼む**（KNOWLEDGE.md 2026-09-14）。

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

- **API キーをリポジトリに入れない。** Claude API は必ずバックエンドから呼ぶ。
  フロントエンドに `ANTHROPIC_API_KEY` を持たせない。`.env` はコミットせず `.env.example` だけ置く
- **自分の学習ログの実データをコミットしない。** サンプル・シードはすべてダミーの学習ログを使う
- **ロードマップ定義（分野・詳細項目の一覧）をコードに直書きしない。**
  マスタデータ（JSON）に分離し、`source`（出典 URL 等）と `checkedAt` を記録する
- **理解度スコアの算出ロジックは純粋関数として分離する。** DB・HTTP・LLM クライアントを import しない。
  LLM の出力（判定結果）は入力として受け取るだけにして、集計・減衰・進捗率の計算は単体テスト可能に保つ
- **LLM の出力を信用しきらない。** スキーマ検証を通し、想定外の分野 ID・範囲外スコアは弾いてログに残す。
  握りつぶし禁止
- ロジックを変更したら、対応する単体テストを同時に更新する

## Claude Code の設定

共通のフック3種と `/apps-workflow:handoff` `/apps-workflow:pr-check` は [apps-workflow プラグイン](https://github.com/n-yoshida-dev/claude-plugins)から来る（`.claude/settings.json` の `enabledPlugins`）。
**`enabledPlugins` だけでは install されない。アプリごとに project スコープで install が要る**（反映は次のセッションから。このアプリは 2026-08-30 に `enabledPlugins` の全プラグインを導入済み）。手順は `../CLAUDE.md`「新しいアプリを作るとき」を参照。

| 種別 | 中身 | 出どころ |
|---|---|---|
| `/apps-workflow:handoff` | HANDOFF / TODO / KNOWLEDGE を更新して次のセッションへ渡す | プラグイン |
| `/apps-workflow:pr-check` | CI と同じ検査をローカルでまとめて実行する（コミット・PR の前） | プラグイン |
| `guard-secrets.sh` | 秘密情報・ローカル専用ファイルのコミットを阻止（PreToolUse） | プラグイン |
| `check-edited.sh` | frontend の typecheck / lint、backend の go vet（PostToolUse） | プラグイン |
| `session-briefing.sh` | TODO.md の未完タスクを起動時に提示（SessionStart） | プラグイン |

このアプリ固有のフック・スキルは `.claude/` に置く。固有のルールが増えたら `.claude/rules/` に切り出し、CLAUDE.md からはリンクだけにする。

## 共通ルール

自作プロダクト共通の開発ルール（技術スタック・Git運用・コーディング規則）は `../CLAUDE.md` にある。
上位ディレクトリの CLAUDE.md は自動でロードされる。
