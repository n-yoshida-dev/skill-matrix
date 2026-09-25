# skill-map.md → skill-matrix 移行計画（最終案・未実行）

作成 2026-09-25。**確定版（2026-09-25 に ChatGPT レビューの修正 2 点＝preState の扱い・react-state-props の移行を反映）。**
`study/learner-profile/skill-map.md` の習熟度を skill-matrix へ移し、以後の正本を skill-matrix にするための設計と手順。
判定モデルの変更（2 本の梯子）は ChatGPT とのレビューで確定済み（論点 6）。以後の細部は既存方針と整合する範囲で Claude が判断し、
止めて確認するのは重大な矛盾・データ損失・公開情報上の問題がある場合だけ（2026-09-25 ユーザー指示）。
この文書は「何をどう変えるか」を一か所に集めたもので、確定したら `SPEC.md` と `logs/decisions.md` へ分けて書き写し、
この文書は `docs/` に人間向けの経緯として残す。

---

## 1. 判定モデル（確定）

### 1.1 言葉

| 言葉 | 意味 |
|---|---|
| **印（evidencedLevels）** | 「レベル n の基準を満たす根拠が付いた」ことを表す、1〜5 それぞれの印の集合。例 `[1, 3]` |
| **verifiedLevel** | 1 から数えて途切れずに印が付いている最上段。`[3]` なら 0、`[1,3]` なら 1、`[1,2,3]` なら 3 |
| **梯子** | 印の付き方の 2 系統。理解の梯子 1→2、実装の梯子 3→4→5 |
| **preState** | レベルとは別の印。`learning` / `explained_only` / `self_reported` / `none`（変更なし） |

### 1.2 根拠の種類が付ける印

| evidenceType | 付ける印 | preState |
|---|---|---|
| `drill` | {1} | |
| `self_explanation` | {1, 2} | |
| `implementation` | {3} | |
| `unaided_implementation` | {3, 4} | |
| `cross_context` | {3, 4, 5} | |
| `learning_activity`（新設） | なし | `learning` |
| `explained_to` | なし | `explained_only` |
| `self_report` | なし | `self_reported` |

**3 は 1・2 を含意しない。** 実装した事実から基礎理解を推測しない（W-008 の実例）。

### 1.3 `proposedLevel` の意味（変更）

判定ファイルの `proposedLevel` は残すが、意味を「この根拠が示す最上段」に変える。書ける値は次の 2 通りだけ。

- **その根拠の梯子の範囲内**（`drill` なら 1、`self_explanation` なら 1 か 2、`unaided_implementation` なら 3 か 4 …）。
  印は梯子の下端からこの値まで付く。既定は上限（1.2 の表のとおり）
- **0 ＝ 不合格の報告**（ドリルに落ちた、説明できなかった）。印は付けず `needsReview` を立てる

上限を超えた値は V5 で上限に切り詰める。梯子の下端未満で 0 でない値（`implementation` で 2 など）は V5 で下端に切り上げる。

### 1.4 適用の規則（`ApplyJudgment`）

1. 印を付ける。`evidencedLevels` に 1.2 の集合（`proposedLevel` で上限を切った範囲）を加える
2. `verifiedLevel` を導出し直す（1 からの連続）
3. **`preState` は `evidencedLevels` が空のときだけ意味を持つ。** 印が 1 つでもある項目の `preState` は常に `none`（導出）。
   印がある項目に後から `explained_to` / `learning_activity` が来ても、イベントは履歴に残すが現在の `preState` は変えない
   （「実装根拠あり＋説明を受けた」を独立して持ちたくなったら、その時点で別のフラグを検討する。v1 ではやらない）
4. `lastEvidenceAt` の更新は、**この判定が付けた最上段の印 ≥ 適用前の `verifiedLevel`** のときだけ（3 の項目にドリルが付いても「3 を保持している」証明にならない。現行の規則と同じ）。更新したときは `needsReview` を消す
5. `proposedLevel` が 0 なら印を付けず `needsReview = true`（V6）

### 1.5 `source` と `confidence`

| `source` | 誰が | `confidence` | V4（廃止） | V7 |
|---|---|---|---|---|
| `ai` | Claude Code / ChatGPT | **必須**（0〜1） | ― | 適用 |
| `manual` | 人 | **書かない** | ― | 適用しない |
| `migration` | skill-map からの写し | **書かない** | ― | 適用しない |

---

## 2. 検証ルール（V1〜V8 改訂）

| ID | ルール | 違反時 | 変更 |
|---|---|---|---|
| V1 | `itemKey` が実在する | 棄却 | なし |
| V2 | `proposedLevel` が 0〜5 の整数 | 棄却 | なし |
| V3 | `evidenceType` が許可リストにある | 棄却 | `learning_activity` を追加 |
| V4 | （一気飛び禁止） | ― | **廃止。** 番号は欠番にして SPEC に理由を残す。1 件の根拠が同じ梯子の下位を同時に証明するのは自然（`unaided_implementation` → {3,4}）。AI の過大評価は指示書側（`unaided` の条件を厳格化・`confidence` 必須・`rationale` と `evidenceRefs` 必須）と `git diff` の目視で抑える |
| V5 | `proposedLevel` が根拠の梯子の範囲内 | 上限超えは上限へ、下端未満（0 以外）は下端へ切り詰めて記録 | 意味を「梯子の範囲」に読み替え |
| V6 | `proposedLevel = 0`（不合格の報告） | 印を付けず `needsReview = true`。記録 | 意味を「現在レベル未満」から「不合格」に読み替え |
| V7 | `confidence` が閾値未満 | 保留 | **`source: ai` のときだけ適用** |
| V8 | 1 ファイルの判定件数が上限（20）以内 | 超過分を棄却 | なし。移行も免除しない |

形の検査（`ParseOutput`）に足すもの：`evidenceRefs` が 1 件以上の配列で各要素が `<種別>:<識別子>`／`confidence` は `source: ai` なら必須、それ以外なら**存在してはいけない**／
`occurredAt`（任意。§4 参照）が `YYYY-MM-DD`。

---

## 3. domain（Go）の変更

| ファイル | 変更 |
|---|---|
| `types.go` | `ItemState.Level` → `VerifiedLevel`（導出値）。`ItemState.Evidenced [LevelCount]bool` を追加。`Judgment.Source`（`ai`/`manual`/`migration`）、`Judgment.EvidenceRefs []string`、`Judgment.HasConfidence bool` を追加。`EvidenceLearningActivity` を追加し `evidencePreStates` に `learning` を登録。`evidenceCaps` を **`evidenceLadder map[EvidenceType]struct{Base, Top Level}`** に置き換える（drill 1-1 / self_explanation 1-2 / implementation 3-3 / unaided 3-4 / cross_context 3-5）。`Rules.MaxLevelStep` と `ViolationLevelJump` を削除 |
| `apply.go` | `decide` を 1.4 の規則に書き換える。V4 の分岐を削除。V5 は梯子の範囲チェック、V6 は `proposedLevel == 0`。V7 は `j.Source == SourceAI && j.HasConfidence` のときだけ |
| `apply_test.go` | 印の付き方（8 種）・導出（`[3]`→0、`[1,3]`→1、`[1,2,3]`→3、`[1,2,3,4]`→4）・不合格報告・印がある項目では preState が none のまま・鮮度の更新条件・source 別の V7 |
| `summary.go` / `plan.go` | `Level` を `VerifiedLevel` に読み替えるだけ。`Action` に `PendingLevels []Level`（印はあるが verifiedLevel に届いていない段）を追加し、`NextActions` が埋める。優先度の式は変えない |
| `llm/output.go` | `ProposedJudgment` に `EvidenceRefs []string`、`Confidence *float64`。`DomainJudgments(source, occurredAt)` |

`EvidenceRefs` は判定材料にしない（形の検査だけが見る）。これは変えない。

---

## 4. 判定ファイルと `state.json` のスキーマ

### 4.1 判定ファイル（SPEC §3.3 の差分）

```jsonc
{
  "schemaVersion": 2,
  "loggedAt": "2026-10-01",
  "source": "ai",                       // "ai" | "manual" | "migration"
  "judge": "claude-code",               // 任意
  "judgments": [
    {
      "itemKey": "go-syntax-basics",
      "evidenceType": "self_explanation",
      "proposedLevel": 2,               // 省略時は根拠の上限。0 は不合格の報告
      "occurredAt": "2026-09-28",       // 任意。根拠が生じた日。省略時は loggedAt（移行と、まとめ書きのために持つ）
      "evidenceRefs": [                 // 1 件以上
        "repo:n-yoshida-dev/study@a1b2c3d/go-react/logs/2026-09-28.md#L10-L30"
      ],
      "rationale": "...",
      "confidence": 0.8                 // source=ai のときだけ
    }
  ],
  "unmatched": []
}
```

`evidenceRefs` の書式：

| 種別 | 形 | 例 |
|---|---|---|
| `repo` | `repo:<owner>/<repo>@<commit>[/<path>[#L<n>[-L<m>]]]` | `repo:n-yoshida-dev/orgflow@6647ba8` |
| `log` | `log:<path>`（コミットされていない `learning-logs/` 用。Template 利用者の既定） | `log:learning-logs/2026-10-01.md` |
| `cert` / `work` | 書式だけ予約。v1 では使わない | |

`study:` は廃止し `repo:n-yoshida-dev/study@…` に統一する。`sources.local.json` は次の形にする（`learning-logs/` の既定は 2026-09-25 の決定のまま）。

```jsonc
{
  "logs": { "path": "learning-logs" },                    // 既定。Template 利用者
  "repos": {
    "n-yoshida-dev/study": { "path": "/home/<user>/workspace/study", "logsGlob": "**/logs/*.md" }
  }
}
```

### 4.2 `state.json`（SPEC §3.4 の差分）

```jsonc
{
  "schemaVersion": 2,
  "items": [
    {
      "itemKey": "java-junit",
      "verifiedLevel": 0,
      "evidencedLevels": [3],
      "preState": "none",
      "needsReview": false,
      "lastEvidenceAt": "2026-08-05",
      "events": [
        {
          "file": "2026-10-01-migration-java-spring.json",
          "index": 1,
          "occurredAt": "2026-08-05",
          "source": "migration",
          "evidenceType": "implementation",
          "proposedLevel": 3,
          "marked": [3],                // この判定が付けた印
          "evidenceRefs": ["repo:n-yoshida-dev/study@<凍結>/learner-profile/skill-map.md#L42", "repo:n-yoshida-dev/orgflow@6647ba8"],
          "rationale": "...",
          "confidence": null,
          "violations": []
        }
      ]
    }
  ],
  "deferred": [],
  "rejected": []
}
```

`appliedLevel` は無くなる（`marked` と `verifiedLevel` で足りる）。

---

## 5. roadmap スキーマ（SPEC §2 の差分）

- `item.moduleRefs`（任意・文字列配列・各 64 文字以内・検証はしない）を追加。カリキュラムのモジュール（`go-01`）との対応を持つ。
  項目の `key` はカリキュラムに依存させない（`go-syntax-basics`）。**`key` の正規表現は変えない**（ハイフン区切り）
- `verifyBy` / `outcome` / `goal` / `description` に所属先・企業名・人名・転職活動・人事評価を書かない（`rationale` と同じ縛り）。
  `verifyBy` は技術的な検証条件だけ。例：「ADR-001 を見ずに、設計理由とトレードオフを自分の言葉で説明できる」
- `verifyBy` は「その項目の次の印を得る方法」を書く。印が `[3]` で止まっている項目では、教材のやり直しではなく
  **既存の実装について短いドリルか自己説明で 1・2 を確認する**旨を書く（画面の「次にやること」もこれを出す。§6）

---

## 6. UI での扱い

| 画面 | `verifiedLevel` | `evidencedLevels` |
|---|---|---|
| マトリクス（§7.1） | 升目の塗り（0〜5 の 6 階調） | 最上段の印 > verifiedLevel のとき、升目の角に小さな印（例：右上の点）。ツールチップに「Verified 0 / 根拠の印 3 / 未確認 1, 2」 |
| サマリー帯 | 分野 × verifiedLevel の項目数 | 「実装根拠あり・理解未確認」の件数を 1 列足す |
| 学習パス（§7.2） | `done` は verifiedLevel ≥ 1（変更なし） | ノードにバッジ。`current` の判定は変えない |
| 項目詳細 | 見出しに 3 行：Verified Level / 付いている印 / 未確認の段 | `events[].marked` で「いつ・どの根拠が・どの印を付けたか」を並べる |
| 次にやること | `gap` は verifiedLevel から計算（変更なし） | `PendingLevels` があれば行の文言を「既存の実装について L1/L2 を短いドリル・自己説明で確認する」に差し替える。`verifyBy` はその下に添える |

**`[3]` の項目を「初心者なので基礎から」と扱わない。** 画面の文言と `verifyBy` の書き方の両方で守る。
study 側（`learning-ops.md` の「レベル 2 以上で支援を減らす」）が読むのは `verifiedLevel`。`evidencedLevels` に 3 があれば
「実装は経験済み」として説明の抽象度は初心者向けのままでも、課題は既存コードの説明から入れる。これは study 側の運用に書く。

---

## 7. SPEC への反映（節ごと）

| 節 | 変更 |
|---|---|
| §0 | 「skill-map.md は手動運用の正本。移行は行わない」を削除し、「2026-XX-XX に skill-map.md から移行。以後の正本は本リポジトリ」に |
| §1.1 | 表はそのまま。「レベルは印から導出する。理解の梯子（1→2）と実装の梯子（3→4→5）。3 は 1・2 を含意しない」を追記。1.2 の表を載せる |
| §1.2 | `learning` を立てる根拠の種類 `learning_activity` を追記。`preState` は印が無いときだけ意味を持ち、印があれば常に `none` |
| §2 | `moduleRefs`。公開前提の縛りを roadmap の文字列にも |
| §3.3 | `schemaVersion: 2`、`source` に `migration`、`evidenceRefs`、`confidence` の条件、`occurredAt`、`study:` 廃止と `repo:` `log:` |
| §3.4 | `verifiedLevel` / `evidencedLevels` / `marked` |
| §3.5 | `logs` と `repos` の形 |
| §4.2 | 指示書に「`unaided_implementation` は AI から具体的なコード提示や逐次ガイドを受けていない場合だけ」「不合格は `proposedLevel: 0`」「`evidenceRefs` は commit 付き」を追加 |
| §4.4 | 「昇格の可否」列を「付ける印」列に |
| §4.5 | §2 の表に置き換え。V4 は欠番 |
| §5 | 関数の引数・戻り値は同じ。`Action.PendingLevels` |
| §7 | §6 の表 |

---

## 8. 移行計画

### 8.1 段階（1 段階 = PR 1 本。C だけは domain／CLI／CI の 3 本に分ける）

| 段階 | 場所 | 内容 | 完了条件 |
|---|---|---|---|
| A. 決定の記録 | skill-matrix | `logs/decisions.md` に 4 件（正本の移行／2 本の梯子と V4 廃止／evidenceRefs の文法／移行判定の扱い）。TODO 2-6 のタスクを B〜G に分割。`CLAUDE.md`「守ること」に roadmap の文字列の縛りを追加 | decisions に 4 件あり、TODO が段階どおりに並んでいる |
| B. SPEC | skill-matrix | §7 の表のとおり書き換え | SPEC と `docs/spec-guide.md` が食い違わない |
| C. domain と CLI | skill-matrix | §3 の変更＋テスト。`recalc` / `verify` CLI（TODO 2-1 の 3〜4 件目をこの形で実装） | `go test ./internal/...` 緑。`[3]`→0 などの導出がテストにある。CI で `verify` が走る |
| D. roadmap | skill-matrix | `data/roadmap.json` を §8.3 の対応表から起こす（実物。`backend/testdata/` のダミーとは別） | `verify` が通る。`verifyBy` に転職・面接の語が無い |
| E. 凍結 | study | skill-map.md 冒頭に注記（下の文面）。`learner-profile/README.md` の正本分担表、`CLAUDE.md` 3 行目と「反映する」の行、`rules/teaching.md`、`rules/learning-ops.md`、`skills/kickoff/SKILL.md` の参照先を skill-matrix へ。**コミットしてハッシュを控える** | study 内で skill-map.md を「更新先」として指す記述が 0 件 |
| F. 移行判定 | skill-matrix | 凍結コミットの skill-map を読み、§8.3 の表どおりに `data/judgments/YYYY-MM-DD-migration-<domain>.json` を書く（指示書は `prompts/migrate-skill-map.md`。通常の `judge.md` に混ぜない）。`recalc` → §8.4 の期待表と突き合わせ → grep → PR | `state.json` が期待表と一致。禁止語 grep 0 件。CI 緑 |
| G. 切り替え | personal-ai-context / study | `technical-skills.md` と `learning-history.md` のリンク先を skill-matrix へ。study の `go-react/logs/` を作り、CLAUDE.md に「学習ログは logs/ に追記し、習熟度は skill-matrix の judgment 経由でのみ更新する」を書く | 凍結後の学習ログ 1 件から `/judge-log` で通常判定を 1 本作り、`state.json` が動く |

E と F は続けて 1〜2 セッションで終える（空白期間を作らない）。

skill-map.md 冒頭の注記（案）：

> **YYYY-MM-DD 以降更新停止。** 現在の習熟度の正本は skill-matrix（`~/workspace/apps/skill-matrix/data/state.json`、公開後は GitHub Pages）。
> 更新は skill-matrix の `data/judgments/` に判定ファイルを足す形でのみ行う。このファイルは移行時点の記録として残す（削除しない）。
> 移行の対応表は skill-matrix の `docs/skill-map-migration.md`。

### 8.2 移行判定の書き方

- **写しであって再判定ではない。** skill-map の習熟度ラベルは付け直さない。根拠欄の記述を「根拠の種類」ごとに分けて判定にする（1 行から複数の判定が出る）
- `source: "migration"`、`confidence` 無し、`occurredAt` はその根拠の日付（行に複数の日付があれば最新）
- `evidenceRefs` の 1 件目は必ず `repo:n-yoshida-dev/study@<凍結コミット>/learner-profile/skill-map.md#L<行>`。2 件目以降は現在も解決できるコミットだけ（orgflow・ai-study-coach）。統合前の study のハッシュ（`d683aa7` 等）は書かない（skill-map 自体に残る）
- `rationale` は技術的な事実だけ。「本人が『全然わからない』と申告」のような記述は書かず「ドリル Q3・Q4 未達（W-008）」の形にする
- ドリルの失点が根拠欄に明記されている行は `proposedLevel: 0` の `drill` を足し、`needsReview` を立てる（2 件：Testcontainers、CI の実行モデル）
- 「未着手」の行は判定を作らない
- skill-map の最終更新（2026-08-31）より後に progress.md で進んだ項目（React の起動フロー・state/props など）は**移行では直さない**。凍結後に通常の判定で追い付かせる

### 8.3 対応表（skill-map の行 → 項目と判定）

分野は 8 つ：`go` / `react` / `java-spring` / `db` / `devops` / `baas-auth` / `ai-collab` / `billing`。
「印」列がその行から出す判定。行番号は凍結後に確定する。

| skill-map の行 | 項目 key（分野） | moduleRefs | 出す判定（evidenceType → 印、occurredAt） | 備考 |
|---|---|---|---|---|
| Java / Spring Boot（言語そのもの） | 移行しない | | | profile.md |
| REST API 設計・OpenAPI | `java-rest-openapi`（java-spring） | | `learning_activity`（2026-08-10） | ラベル「未確認」の写し。orgflow の docs/openapi は通常判定で拾う |
| ソフトウェアテスト（フェーズ運用） | 移行しない | | | 職歴由来。profile.md |
| JUnit・テスト設計 | `java-junit`（java-spring） | | `implementation` → {3}（2026-08-05, orgflow `6647ba8`） | W-005 open |
| CI | `devops-ci-github-actions`（devops） | | `implementation` → {3}（2026-08-13, orgflow `de995eb`） | 自力執筆だが 4 は保留、の写し |
| Spring Security + JWT | `java-spring-security-jwt`（java-spring） | | `implementation` → {3}（2026-07-16） | |
| テナント選択フロー | `java-tenant-switch`（java-spring） | | `implementation` → {3}（2026-07-16） | |
| マルチテナント境界の DB 設計 | `db-multitenant-schema`（db） | | `implementation` → {3}（2026-07-16） | |
| RBAC スキーマ設計 | `db-rbac-schema`（db） | | `implementation` → {3}（2026-07-16） | |
| 申請ドメイン設計 | `java-request-domain`（java-spring） | | `implementation` → {3}（2026-07-29） | |
| 例外設計・HTTP ステータス | `java-exception-http-status`（java-spring） | | `implementation` → {3}（2026-08-05）、`self_explanation` → {1,2}（2026-08-23） | W-006 closed |
| アプリケーションログ | `java-app-logging`（java-spring） | | `implementation` → {3}（2026-08-29, orgflow `b7d85b7`）、`explained_to`（2026-08-29） | 印があるので preState は none。explained_to は履歴にだけ残る |
| Flyway | `db-flyway-migration`（db） | | `implementation` → {3}（2026-08-09, orgflow `f32322c`） | |
| テスト基盤（Testcontainers） | `java-test-infra-testcontainers`（java-spring） | | `explained_to`（2026-08-09, orgflow `4a023fa`）、`drill` proposedLevel 0（2026-08-13） | W-008 open。needsReview |
| ADR・設計ドキュメント運用 | `java-adr-design-docs`（java-spring） | | `implementation` → {3}（2026-07-16）、`self_explanation` → {1,2}（2026-08-23） | |
| Go 基本構文 | `go-syntax-basics`（go） | go-01 | `drill` → {1}（2026-07-25） | |
| Go struct・slice/map・pointer | `go-struct-slice-map-pointer`（go） | go-01 | `drill` → {1}（2026-08-21） | W-009 closed |
| Go 関数値・クロージャ | `go-func-values-closure`（go） | go-01 | `explained_to`（2026-08-12） | |
| Go メソッド | `go-methods`（go） | go-02 | `explained_to`（2026-08-26） | |
| Go その他 | `go-interfaces` `go-error-handling` `go-concurrency` `go-net-http` `go-testing`（go） | go-02〜06 | なし | 未着手 |
| React 起動フロー | `react-render-flow`（react） | react-01 | `explained_to`（2026-07-14） | progress.md では 9/01 に確認済み。凍結後の通常判定で追い付かせる |
| JSX / TSX / TS 基礎 | `react-jsx-ts-basics`（react） | react-01 | `explained_to`（2026-07-14） | 同上 |
| コンポーネント / 親子 / 分割代入 | `react-components-destructuring`（react） | react-01 | `explained_to`（2026-07-21）、`drill` → {1}（2026-08-29） | 分割代入の確認問題 |
| state / props / データフロー | `react-state-props-dataflow`（react） | react-01 | なし | 「基礎理解を確認（ドリル未実施）」で根拠の種類が確定しないため移行では判定を作らない（写しの原則）。凍結後に progress.md と新しい学習ログから通常判定で追い付かせる |
| hooks・データ取得・ルーティング・TS 統合 | `react-hooks` `react-data-fetching` `react-routing-forms` `react-ts-integration`（react） | react-02〜05 | なし | 未着手 |
| Codex への依頼・検証サイクル | `ai-collab-agent-cycle`（ai-collab） | | `implementation` → {3}（2026-07-13） | |
| AI 出力のコードレビュー | `ai-collab-output-review`（ai-collab） | | `learning_activity`（2026-07-14） | |
| GitHub の PR 運用 | `devops-pr-workflow`（devops） | | `implementation` → {3}（2026-08-06） | |
| CI/CD 構築・運用 | `devops-cicd-pipeline`（devops） | | `implementation` → {3}（2026-08-08） | |
| ブランチ保護 | `devops-branch-protection`（devops） | | `unaided_implementation` → {3,4}（2026-08-20）、`drill` → {1}（2026-08-20） | 「自力設定」の写し。理由の言語化は一部自力なので drill 1 まで |
| デプロイ基盤（preview / production） | `devops-deploy-preview-production`（devops） | | `drill` → {1}（2026-08-09） | |
| CI の実行モデル | `devops-ci-execution-model`（devops） | | `learning_activity`（2026-08-09）、`drill` proposedLevel 0（2026-08-09） | W-007。needsReview |
| 必須ステータスチェックの選定基準 | `devops-required-checks`（devops） | | `drill` → {1}（2026-08-20） | |
| BaaS 構成でのスキーマ設計 | `baas-supabase-schema`（baas-auth） | | `implementation` → {3}（2026-08-16, ai-study-coach `24b4b6e`） | |
| RLS | `baas-rls`（baas-auth） | | `explained_to`（2026-08-31） | |
| 外部 ID 連携（OAuth / OIDC） | `auth-oauth-oidc`（baas-auth） | | `explained_to`（2026-08-21, ai-study-coach `dfee429`） | |
| Supabase のデータアクセス | `baas-supabase-js`（baas-auth） | | `explained_to`（2026-08-27, ai-study-coach `1372a02`） | |
| BaaS とバックエンド自作の使い分け | `baas-vs-custom-backend`（baas-auth） | | `drill` → {1}（2026-08-14） | |
| サブスク課金・Webhook | `billing-stripe-overview` `billing-stripe-flow` `billing-stripe-tenant`（billing） | bill-01〜03 | なし | 未着手 |

判定ファイル：`migration-java-spring`（13 件）、`migration-db`（3）、`migration-go`（4）、`migration-react`（4）、`migration-devops`（9）、`migration-baas-auth`（5）、`migration-ai-collab`（2）。いずれも V8（20 件）以内。

### 8.4 期待表（`recalc` 後の `state.json` の検算に使う）

| 項目 | evidencedLevels | verifiedLevel | preState | needsReview |
|---|---|---|---|---|
| java-rest-openapi | [] | 0 | learning | |
| java-junit | [3] | 0 | none | |
| devops-ci-github-actions | [3] | 0 | none | |
| java-spring-security-jwt / java-tenant-switch / java-request-domain | [3] | 0 | none | |
| db-multitenant-schema / db-rbac-schema / db-flyway-migration | [3] | 0 | none | |
| java-exception-http-status | [1,2,3] | 3 | none | |
| java-app-logging | [3] | 0 | none | |
| java-test-infra-testcontainers | [] | 0 | explained_only | true |
| java-adr-design-docs | [1,2,3] | 3 | none | |
| go-syntax-basics / go-struct-slice-map-pointer | [1] | 1 | none | |
| go-func-values-closure / go-methods | [] | 0 | explained_only | |
| go-interfaces 〜 go-testing | [] | 0 | none | |
| react-render-flow / react-jsx-ts-basics | [] | 0 | explained_only | |
| react-components-destructuring | [1] | 1 | none | |
| react-state-props-dataflow | [] | 0 | none | 凍結後の通常判定で上がる見込み |
| react-hooks 〜 react-ts-integration | [] | 0 | none | |
| ai-collab-agent-cycle | [3] | 0 | none | |
| ai-collab-output-review | [] | 0 | learning | |
| devops-pr-workflow / devops-cicd-pipeline | [3] | 0 | none | |
| devops-branch-protection | [1,3,4] | 1 | none | |
| devops-deploy-preview-production / devops-required-checks | [1] | 1 | none | |
| devops-ci-execution-model | [] | 0 | learning | true |
| baas-supabase-schema | [3] | 0 | none | |
| baas-rls / auth-oauth-oidc / baas-supabase-js | [] | 0 | explained_only | |
| baas-vs-custom-backend | [1] | 1 | none | |
| billing-* | [] | 0 | none | |

verifiedLevel の分布：3 が 2 項目、1 が 7 項目、0 が残り。**上位の根拠あり（印の最上段 > verifiedLevel。§6 の角の印と同じ定義）が 14 項目**（うち 1 が無いものが 13、`devops-branch-protection` は 1 だけあって 2 が無い）。
これが厳密モデルの意図どおりの姿で、次にやることはこの 14 項目の L1/L2 確認に向く。

### 8.5 変更するファイル・変更しないファイル

変更する：

- skill-matrix：`logs/decisions.md`、`SPEC.md`、`TODO.md`、`CLAUDE.md`、`docs/spec-guide.md`、`backend/internal/domain/{types,apply,summary,plan}.go` とテスト、`backend/internal/llm/output.go` とテスト、`backend/cmd/skillmatrix/`（新規）、`data/roadmap.json`（新規）、`data/judgments/*-migration-*.json`（新規 7 本）、`data/state.json`（生成）、`prompts/judge.md` と `prompts/migrate-skill-map.md`（新規）、`sources.local.json.example`
- study：`learner-profile/skill-map.md`（冒頭注記のみ）、`learner-profile/README.md`、`CLAUDE.md`、`.claude/rules/teaching.md`、`.claude/rules/learning-ops.md`、`.claude/skills/kickoff/SKILL.md`、`go-react/logs/`（新規ディレクトリ）
- personal-ai-context：`learning/technical-skills.md`、`learning/learning-history.md`

変更しない：`weaknesses.md`、`profile.md`、`teaching/`、`go-react/progress.md`・`curriculum.md`、`orgflow-learning/`、orgflow と ai-study-coach、`backend/testdata/`（ダミーのまま。`schemaVersion: 2` へは C で追従）、`backend/internal/{store,httpapi,worker}`（棚上げのまま。`Level` の改名で `store` のコンパイルが落ちるので、そこだけ機械的に追従する）。

### 8.6 リスク

| リスク | 手当て |
|---|---|
| 移行直後のマトリクスが 0 だらけに見える | 意図どおり（A で確定）。角の印とサマリー帯の列で「実装根拠あり」を見せる |
| `rationale` / `verifyBy` に固有名詞・転職の語が混ざる | F と D の完了条件に grep を入れる。禁止語リストを `verify` に足すかは事故が起きてから |
| 凍結後に study セッションが skill-map を更新する | E で CLAUDE.md と rules を同時に直す。E と F を続けて終える |
| skill-map が progress.md より古い行（React） | 移行では直さず、G の通常判定で追い付かせる。期待表に「凍結後に上がる見込み」として印を付けておく |
| `Level` の改名で棚上げコード（store）が壊れる | 機械的に追従。挙動は変えない |
| 古いハッシュを `evidenceRefs` に書いてしまう | 移行の指示書に「解決できるコミットだけ」と書く。`verify` は到達可能性を見ないので目視 |
| 鮮度が最初から `stale` の項目が多い | 事実どおり。既定値（30 / 90 日）を変えるかは画面を見てから |

### 8.7 全体の完了条件

1. `data/state.json` が §8.4 の期待表と一致し、その表が PR 本文にある
2. CI の `verify` が緑。`[3]`→0 の導出と `source` 別の V7 がテストにある
3. `data/` を禁止語で grep して 0 件
4. study 内で skill-map.md を更新先として指す記述が 0 件。冒頭に更新停止の注記がある
5. 凍結後の学習ログ 1 件から通常判定を 1 本作り、`state.json` が動いた

---

## 9. 実装時に決める細部（議論は不要。担当セッションが決めて KNOWLEDGE に書く）

- `proposedLevel` を省略したときの既定を「上限」にするか「必須」にするか（形の検査の都合。指示書には常に書かせる）
- `ItemState` の `Evidenced` を `[6]bool` にするかビット列にするか
- サマリー帯の追加列の名前
- `migrate-skill-map.md` の文面（§8.2 をそのまま指示にする）
