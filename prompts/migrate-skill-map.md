# 移行判定の指示書（skill-map.md → `data/judgments/`）

凍結した `study/learner-profile/skill-map.md` の習熟度を、判定ファイルに写すための指示。**1 回きり**（段階 F。`docs/skill-map-migration.md` §8）。
通常の学習ログの判定（`prompts/judge.md`）には使わない。混ぜない。

## あなたの役割

skill-map の各行を、根拠の種類ごとの判定に**写す**。再判定はしない。習熟度のラベルは付け直さない。

## 読むもの（この順）

1. 凍結版の skill-map：`git -C <study のパス> show <凍結コミット>:learner-profile/skill-map.md`。
   study のパスは `sources.local.json` の `repos["n-yoshida-dev/study"].path`。凍結コミットは `TODO.md` 段階 E の行（`284c0f1`）。
   **作業コピーではなく凍結コミットを読む**（行番号を固定するため）
2. 対応表 `docs/skill-map-migration.md` §8.3：skill-map の行 → 項目 key・根拠の種類・日付。**判定はこの表どおりに作る**。表に無い判定を足さない
3. 根拠の種類と付けられる印：`backend/internal/domain/types.go` の `evidenceLadder`。レベルの意味：`data/roadmap.json` の `levels`
4. 判定ファイルの形：`SPEC.md` §3.3・§4.3

凍結版と §8.3 が食い違う行（§8.3 を書いた後に skill-map が更新された行）を見つけたら、判定を書く前に止まって報告する。
直すのは §8.3・§8.4 の側で、判定側で辻褄を合わせない（`logs/decisions.md` 2026-09-25「移行判定は…写し」の見直す条件）。

## 書き方

- **ファイル**：ロードマップの分野ごとに 1 本。`data/judgments/<書いた日>-migration-<分野>.json`。1 本 20 件以内（V8。移行も免除しない）
- **外枠**：`"schemaVersion": 2`、`"loggedAt"` はファイル名の日付、`"source": "migration"`、`"unmatched": []`。`judge` は書かない
- **1 件**：`itemKey`・`evidenceType`・`proposedLevel`・`occurredAt`・`evidenceRefs`・`rationale` の 6 つだけ。**`confidence` は書かない**（形の検査で棄却される）
- **並び**：分野の中は `roadmap.json` の項目順。同じ項目に判定が複数あれば `occurredAt` の古い順（適用順で `needsReview` と鮮度が決まる）
- **`proposedLevel`**：
  - 印を付ける種類は、§8.3 に書いた印の最上段（`drill` なら 1、`self_explanation` の {1,2} なら 2、`unaided_implementation` の {3,4} なら 4）
  - 不合格の報告（§8.3 に「proposedLevel 0」とある `drill`）は 0
  - 印を付けない種類（`learning_activity` / `explained_to` / `self_report`）は 0 と書く（意味は無いが省略できない）
- **`occurredAt`**：§8.3 の日付。その根拠が生じた日で、行に複数の日付があればその根拠のうち最新
- **`evidenceRefs`**：
  - 1 件目は必ず `repo:n-yoshida-dev/study@<凍結コミット>/learner-profile/skill-map.md#L<行>`
  - 2 件目以降は、§8.3 に書いたコミット（orgflow・ai-study-coach）と、凍結コミット時点の study のファイルだけ。どちらも 7 桁
  - 統合前の study のハッシュ（`d683aa7` 等。skill-map の注記どおり現リポジトリに無い）は書かない
- **`rationale`**：その判定の根拠になった技術的な事実を 1〜2 文。
  - 書かない：所属先・企業名・人名・転職活動・面接・人事評価。本人の気持ちの引用（「全然わからない」等）。職歴
  - 書いてよい：何を実装したか・どの問いに答えられたか・どこが未達か、弱点の番号（`W-008`）
  - 不合格の報告は「ドリル Q3・Q4 未達（W-008）」の形

## 作らない判定

- 「未着手」の行、「移行しない」の行（職歴・経歴由来）
- 根拠の種類が確定しない行（例：「基礎理解を確認」だがドリル未実施で、根拠が予想と確認 1 問だけ）。凍結後の通常判定で追い付かせる

## 書いた後

1. `go -C backend run ./cmd/skillmatrix recalc --data ../data` で `state.json` を作り直す。棄却（`rejected`）が 0 件であること
2. `state.json` を §8.4 の期待表と突き合わせる。1 項目でも違えば、判定ではなく §8.3・§8.4 のどちらが正しいかを先に確かめる
3. `grep -rnE '所属|企業|面接|転職|人事' data/` が 0 件。`git diff` で `rationale` を全件目で読む
