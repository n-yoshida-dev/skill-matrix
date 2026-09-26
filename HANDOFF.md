# 引き継ぎ

**次のセッションが最初に読むファイル。** 区切りで Claude が `apps-workflow:handoff` を呼んで更新する。
書くのは「今どこにいるか」と「次の一手」だけ。履歴と決定事項は積まない（行き先は末尾の表）。

## 0. 進め方のルール（最優先）

**設計・仕様・技術判断を提示するときは、ジュニアレベルでも分かる解説を先に付けること。**
専門用語は初出時に「何のためにそうするのか」を添え、抽象的な説明の前に具体例を出す。
承認を求めるのはその後。**中身が分からないまま承認させない。**

本人の自己申告：「作りたいアプリの最終イメージは持っているが、技術的な設計判断は曖昧」。
実務は Java / Spring Boot なので DB 設計・REST API・OpenAPI・JUnit は通じる。Go と React は学習中。

**テストコードを「仕様の確認材料」として示すときは、コードの読み方を教えない。**
「何を保証しているか」を日本語の箇条書きで列挙する。

**長い返答は読まれない（2026-09-25 に 2 回）。** 設計の議論でも結論と表を先に、詳細はファイルへ。
ChatGPT に渡す文章を求められたら、コードブロック 1 つでそのまま貼れる形にする。

**同じ作業ディレクトリを別セッションと同時に使わない。** 2026-09-25 に別セッションが同じチェックアウトで main を進め、
ローカルブランチが消えるなど衝突の余地があった。着手前に `git log --oneline -3` と `git branch` で状態を見る。

## 進捗

```
進捗（TODO.md より自動集計・2026-09-26）

████████████████████ 100%  残り  0 / 11  フェーズ1：設計（完了）
█████████░░░░░░░░░░░  45%  残り 22 / 40  フェーズ2：v1 実装（自分専用版。ゴールは「画面で自分のマトリクスが見える」こと）
███████████░░░░░░░░░  56%  残り 22 / 51  合計
あなたの回答待ち：4 件（回答済み 4 件）
保留（合計に含めない）：19 件（済み 15 件）

前回の区切り（2026-09-25）から：完了 +4 件、新たに見つかったタスク +1 件
```

## 1. 現在地

`main` はクリーン（PR #41 まで）。作業中のブランチは無い。

- **SPEC は 2 本の梯子モデル（段階 B 済み）、`internal/domain` と `llm/output.go` もそれに追従済み（段階 C-1 済み）。**
  `ItemState` は `VerifiedLevel`（導出）＋ `Evidenced`（印）、`Judgment` は `Source` / `EvidenceRefs` / `HasConfidence` を持つ。V4 は廃止
- **画面は公開ビュー（`/`。採用担当者向け・タイル表示）まで動く。** 作業ビュー（`/plan`）は土台だけ。
  `data/` はダミー（`roadmap.json` は testdata と同じ内容、`state.json` は手書き）。実物は段階 D・F で入る
- 棚上げ中の `store` / `worker` / `stub` / `prompt` は改名に機械的に追従しただけ（挙動は変えていない）
- 以後の細部は既存方針と整合する範囲で Claude が判断してよい。止めて確認するのは重大な矛盾・データ損失・公開情報上の問題だけ（2026-09-25 ユーザー指示）

## 2. 次セッションで最初にやること

**TODO 2-1 の段階 C-2＝検証と再計算の CLI（`backend/cmd/skillmatrix`。`recalc` / `verify`）。**
仕様は `SPEC.md` §6（モードと終了コード）・§3.3〜§3.4（判定ファイルと `state.json` の形）・§4.5（形の検査 → V1〜V8）。
完了条件は TODO 2-1 にある（`preState` の `""` → `"none"` 正規化、処理順、`backend/testdata/` の判定を `schemaVersion: 2` に更新、を含む）。
`state.json` の JSON エンコードは決定的に（項目はロードマップ順。整形の細部は C-2 で決めて SPEC §3.4 に書く）。`verify` はバイト単位で一致を見る。

その後は TODO 2-1 の順（CI で `verify` → `roadmap.json` の実物 → `sources` → study 側の凍結 → 移行判定 → 切り替え）。

## 3. 動作確認コマンド

```bash
# 純粋関数のテスト（DB 不要）
go -C backend test ./internal/... -cover

# store を触ったら DB テストも回す（ローカルでは Postgres 無しだとスキップされ、CI だけが落ちる。KNOWLEDGE.md 2026-09-26）
docker compose up -d postgres
TEST_DATABASE_URL='postgres://skillmatrix:skillmatrix@localhost:5432/skillmatrix?sslmode=disable' go -C backend test ./internal/store/
docker compose stop postgres

# 画面（frontend）。#/ が公開ビュー、#/plan が作業ビュー
cd frontend && npm ci && npm run dev
# CI と同じ検査
cd frontend && npm run format:check && npm run lint && npm run typecheck && npx vitest run && npm run build
```

## ここに書かないもの（行き先）

| 書きたくなったこと | 行き先 |
|---|---|
| ユーザーと合意した判断 | `logs/decisions.md`（設計・方針を提案する前に読む） |
| 設計判断の理由・ハマりどころ | `KNOWLEDGE.md`（**方針を変えたくなったら必ず先に読む**） |
| やること・確認待ち | `TODO.md` |
| やったこと | `git log` と PR |
