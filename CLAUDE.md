# skill-matrix

ロードマップをマスタとして、学習ログを AI（Claude Code / ChatGPT）に読ませて理解度を判定させ、分野 × 詳細項目の理解度マトリクスと学習パスを
静的サイトで見せる自分専用のツール。データはリポジトリ内の JSON（`data/`）、検証は Go の CLI と CI。
**2026-09-23 に「公開する Web サービス」から絞り込んだ**（`logs/decisions.md`）。サーバ・DB・ログイン・非同期判定の実装は消さずに棚上げ中（`SPEC.md` §10）。

## セッション開始時にすること

1. **`HANDOFF.md` を読む** — 現在地と次の一手だけがここにある。ユーザーと合意した判断は `logs/decisions.md`、
   設計判断の理由とハマりどころは `KNOWLEDGE.md`（設計・方針を提案する前、方針を変えたくなったときに読む）

未完タスクは SessionStart フックが `TODO.md` から自動で提示する。
セッションの区切りには **Claude が `apps-workflow:handoff` スキルを自分で呼んで**引き継ぎを書く。ユーザーに実行を頼まない
（apps-workflow v1.4.2 でユーザー起動限定を外した。呼ぶ場面は `../CLAUDE.md`「開発ドキュメント」と同じ）。
**ユーザー起動限定のまま残っているのは `/apps-workflow:pr-check` だけ**で、こちらは Claude からは呼べない。
コミット前の検査は `apps-workflow:pr-flow` の手順どおりコマンドを直接回す（KNOWLEDGE.md 2026-09-14 / 2026-09-18）。

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

- **API キーをどこにも置かない。** v1 は Claude API を呼ばない（判定は Claude Code / ChatGPT のサブスクリプション内）。
  v2 でバックエンドを戻すときも、Claude API は必ずバックエンドから呼び、フロントエンドに `ANTHROPIC_API_KEY` を持たせない。
  `.env` はコミットせず `.env.example` だけ置く（棚上げ中の `backend/.env.example` はそのまま）
- **学習ログの本文をリポジトリに置かない。** 正本は `~/workspace/study`（Private）。判定は `evidenceRefs` で出どころを指すだけ（`SPEC.md` §3.3）。
  **判定（`data/judgments/`）と理解度（`data/state.json`）は実物をコミットする。** これは `../CLAUDE.md`「実データをコミットしない」の
  意図した例外で、条件は3つ：本文を置かない／`rationale` と `evidenceRefs` は技術的な事実だけ（所属先・企業名・人名・転職活動・人事評価を書かない）／
  Public にする直前に全件を目で読む（`SPEC.md` §9）。`backend/testdata/` は引き続きダミーだけ
- **`data/roadmap.json` の文字列（`verifyBy` / `outcome` / `goal` / `description`）にも同じ縛りを掛ける。** 所属先・企業名・人名・転職活動・人事評価を書かず、
  `verifyBy` は技術的な検証条件だけを書く（「面接想定で説明する」ではなく「ADR を見ずに設計理由とトレードオフを自分の言葉で説明できる」）。
  転職・面接の文脈は study 側に残す（`logs/decisions.md` 2026-09-25「習熟度の正本を…移す」）
- **習熟度の正本はこのリポジトリ。** `~/workspace/study/learner-profile/skill-map.md` は移行後に凍結する（更新しない）。
  レベルは「印」（`evidencedLevels`）から導出した `verifiedLevel` で、**実装の根拠（3）は基礎理解（1・2）を含意しない**。
  `[3]` だけの項目を「基礎からやり直し」とは扱わず、既存の実装について L1/L2 を確認する行動を出す（`docs/skill-map-migration.md`、`logs/decisions.md` 2026-09-25）
- **`data/judgments/` は追記のみ、`data/state.json` は手で編集しない。** コミット後の訂正は既存ファイルを触らず `source: "manual"` の判定ファイルを足す
  （コミット前は `recalc` の結果を見て直してよい。`SPEC.md` §4.7）。
  `state.json` は CLI の `recalc` が書き、CI の `verify` が再計算結果との一致を見る（`SPEC.md` §3.2・§6）
- **ロードマップ定義（分野・詳細項目の一覧）をコードに直書きしない。**
  マスタデータ（`data/roadmap.json`）に分離し、`source`（出典 URL 等）と `checkedAt` を記録する
- **判定基準を AI への指示書に書き写さない。** `prompts/judge.md` はレベルの基準を `data/roadmap.json` の `levels` から、
  根拠の種類と印（付けられるレベル）を `backend/internal/domain/types.go` から読ませる（`SPEC.md` §4.2）。基準の正本は1か所
- **理解度スコアの算出ロジックは純粋関数として分離する。** Go 側（`internal/domain`）は DB・HTTP・LLM クライアントを import しない。
  AI の出力（判定結果）は入力として受け取るだけにして、検証・集計・進捗率の計算は単体テスト可能に保つ。
  「今日」に依存する計算（鮮度・次にやること）は TypeScript 側の純粋関数に置き、DOM・ファイル読み込みを import しない（`SPEC.md` §5）
- **AI の出力を信用しきらない。** 形の検査（`internal/llm/output.go`）と意味の検査（V1〜V8）を通し、
  想定外の項目 ID・範囲外レベルは弾いて `state.json` の `rejected` / `deferred` / `events[].violations` に残す。握りつぶし禁止
- ロジックを変更したら、対応する単体テストを同時に更新する
- **棚上げしたコード（`SPEC.md` §10 の一覧）は消さない。** テストを緑に保つための最小限の追随（型に 1 フィールド足す等）はしてよい。
  それ以上の手直しが要るなら、直さずに棚上げの範囲を見直す。Go 側の `Staleness` / `NextActions` 等は参照実装として残す（`SPEC.md` §5）

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
