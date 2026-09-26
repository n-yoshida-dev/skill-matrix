# 確定仕様

**実装が参照する正本。** ユーザーの確認を経たものだけを書く。検討中のことは `PLAN.md` に置く。

人間向けの解説は [`docs/spec-guide.md`](docs/spec-guide.md) にある。
**この文書と同じ事実を二重に書かないこと。** 解説側は「なぜそうしたか」と具体例を担当する。

**2026-09-23 に v1 を「自分専用のツール」へ絞り込んだ**（`logs/decisions.md` 2026-09-23）。
§0〜§9 は自分専用版の仕様。以前の Web サービス版の仕様（DB スキーマ・非同期ジョブ・API・環境変数）は
消さずに **§10「v2 へ棚上げした仕様」** にまとめてある。実装済みのコードもそこに残っている。

---

## 0. スコープ

段階を分けて作る。**v1 は自分ひとりが使える形を通す。**

| 段階 | 中身 |
|---|---|
| **v1（自分専用）** | ロードマップ・判定・理解度をリポジトリ内の JSON で持つ / 判定は Claude Code・ChatGPT に指示書を読ませて行う / 検証と再計算は Go の CLI、CI でも検証 / 画面は静的サイト（GitHub Pages）：理解度マトリクス・**学習パス**・項目詳細・次にやること Top N |
| v2（他人にも使わせる） | §10 の棚上げ分を戻す：GitHub ログイン / PostgreSQL / 学習ログ投稿 API → 非同期判定 / 判定の反映モード / レート制限。加えて公開ロードマップの一覧・検索・star・fork / publish / GUI エディタ |
| v3 | SNS シェア（マトリクス画像・公開プロフィールページ） |
| v4 | フレンド機能（進捗の相互閲覧、opt-in） |

**v1 でやらないこと**：ログイン、サーバ、DB、画面からの編集、GUI でのロードマップ作成、公開ロードマップの共有、クイズ形式の理解度測定、
学習時間の自動計測、GitHub リポジトリの自動連携、モバイルアプリ、上流ロードマップの変更取り込み（fork 後の `git pull` 相当）。

v2 へ進むか、検証ロジックを CLI として配布する道へ進むかは、自分専用版を実際に使ってから決める（`logs/decisions.md` 2026-09-23「見直す条件」）。

**習熟度の正本はこのリポジトリ。** `~/workspace/study/learner-profile/skill-map.md`（同じ5段階モデルの手動台帳）は
2026-09-25 の決定で skill-matrix へ移行して凍結する（`logs/decisions.md` 2026-09-25「習熟度の正本を…移す」。手順は `docs/skill-map-migration.md` §8）。
移行が済むまでの間だけ skill-map.md が最新で、移行判定（`source: "migration"`。§3.3）で写し取る。

---

## 1. 理解度モデル

`~/workspace/study/learner-profile/` の5段階モデルを踏襲する。判定基準が言語化済みで運用実績があるため。

### 1.1 レベル（0〜5）

| level | 名称 | 到達条件 |
|---|---|---|
| 0 | 未着手 | 学習を開始していない |
| 1 | 基礎理解を確認 | 確認質問・ドリル・自分の言葉での説明で基礎理解を確認できた |
| 2 | 自分で説明可能 | 他人（または AI）に自分の言葉で説明できる |
| 3 | ガイド付き実装 | AI や資料の助けを借りれば実装できる |
| 4 | 自力実装・レビュー | 独力で実装でき、他者のコードをレビューできる |
| 5 | 別文脈へ応用 | 学んだ文脈と異なる場面で使える |

**レベルは「印」から導出する（2026-09-25 決定。`logs/decisions.md`「レベルは『印』から導出する」）。**
項目は `evidencedLevels`（1〜5 それぞれに「その段の基準を満たす根拠が付いた」印の集合。例 `[1, 3]`）を持ち、
`verifiedLevel` は「1 から途切れずに印が付いている最上段」として導出する。`[3]` なら 0、`[1, 3]` なら 1、`[1, 2, 3]` なら 3。

印の付き方は 2 本の梯子に分かれる。**理解の梯子（1 → 2）と実装の梯子（3 → 4 → 5）は独立で、3 は 1・2 を含意しない。**
実装に触れた事実から基礎理解を推測しない（実装の直後でも基礎の説明に失点する実例があった）。

| evidenceType | 付ける印 | `preState` |
|---|---|---|
| `drill` | {1} | |
| `self_explanation` | {1, 2} | |
| `implementation` | {3} | |
| `unaided_implementation` | {3, 4} | |
| `cross_context` | {3, 4, 5} | |
| `learning_activity` | なし | `learning` |
| `explained_to` | なし | `explained_only` |
| `self_report` | なし | `self_reported` |

`[3]` だけの項目（実装の根拠はあるが基礎理解が未確認）を「初心者なので基礎から」とは扱わない。
画面（§7.4）と `verifyBy`（§2）は「既存の実装について L1 / L2 を短いドリル・自己説明で確認する」を次の行動として出す。

### 1.2 段階前の状態（`preState`）

レベルとは別軸のフラグ。**「レベル 0.5」ではなく「根拠の種類がまだ弱い」ことを表す。**

| preState | 意味 |
|---|---|
| `none` | 特記なし |
| `learning` | 未確認（学習中）。学習は始めたが習熟度を判定できる根拠がない。根拠の種類 `learning_activity` が立てる |
| `explained_only` | 説明済み・理解未確認。説明を受けて質問は出なかったが確認していない。`explained_to` が立てる |
| `self_reported` | 実務経験あり・横断評価未実施。自己申告のみ。`self_report` が立てる |

**`preState` は印（`evidencedLevels`）が無いときだけ意味を持つ。** 印が 1 つでもある項目の `preState` は常に `none`（導出値）。
印がある項目に後から `explained_to` / `learning_activity` が来ても、イベントは履歴に残すが現在の `preState` は変えない
（「実装根拠あり＋説明を受けた」を独立に持ちたくなったら別のフラグを検討する。v1 ではやらない）。

### 1.3 時間経過による減衰

**入れない。** スコアを自動で下げない。代わりに **`lastEvidenceAt`（最終根拠日）** を持ち、経過日数から鮮度を導出する。

| staleness | 条件（既定値。`data/settings.json` で変更可。§8.3） |
|---|---|
| `fresh` | 30日以内 |
| `aging` | 31〜90日 |
| `stale` | 91日以上 |

理由：自動で下がった値は「なぜ下がったか」を説明できない。既存モデルの「昇格には根拠が必要」という思想とも合わない。減衰式は後から純粋関数として追加できる。

鮮度は「今日」に依存するので `data/state.json` には書かず、画面を開いたときに計算する（§5）。

---

## 2. ロードマップ・マスタ JSON

ロードマップの正本フォーマット。**コードに直書きしない。** v1 では `data/roadmap.json` がこの形で置かれる。

```jsonc
{
  "schemaVersion": 1,
  "name": "SaaS エンジニア学習ロードマップ",
  "description": "Go / React を中心とした転職向けロードマップ",
  "origin": "external",   // "manual" | "external" | "builtin"
  "source": "https://github.com/n-yoshida-dev/study/blob/main/go-react/curriculum.md",
  "checkedAt": "2026-08-09",
  "levels": [
    { "level": 1, "name": "基礎理解を確認", "criteria": "確認質問・ドリル・自分の言葉での説明で基礎理解を確認できた" },
    { "level": 2, "name": "自分で説明可能", "criteria": "他人（または AI）に自分の言葉で説明できる" },
    { "level": 3, "name": "ガイド付き実装", "criteria": "AI や資料の助けを借りれば実装できる" },
    { "level": 4, "name": "自力実装・レビュー", "criteria": "独力で実装でき、他者のコードをレビューできる" },
    { "level": 5, "name": "別文脈へ応用", "criteria": "学んだ文脈と異なる場面で使える" }
  ],
  "domains": [
    {
      "key": "go",
      "name": "Go",
      "goal": "Java との差分を理解したうえで、Go で Web API を設計・実装・テストできる",
      "items": [
        {
          "key": "go-syntax-basics",
          "name": "基本構文（変数・型・関数・struct・slice/map）",
          "description": "Java脳との差分（ポインタ・ゼロ値）を含む",
          "outcome": "Go のコードを読んで型と値の流れを追え、Java との差分を説明できる",
          "dependsOn": [],
          "verifyBy": "Tour Basics 完了後にドリル",
          "moduleRefs": ["go-01"]   // 任意。カリキュラムのモジュールとの対応（検証はしない）
        },
        {
          "key": "go-methods-interfaces",
          "name": "メソッド・インターフェース・埋め込み",
          "outcome": "型に振る舞いを持たせ、インターフェースで実装を差し替えられる",
          "dependsOn": ["go-syntax-basics"]
        }
      ]
    }
  ]
}
```

制約：

- `key` は同一ロードマップ内で一意。`^[a-z0-9][a-z0-9-]{0,63}$`。カリキュラムのモジュール番号に依存させない（`go-syntax-basics` のように内容で付ける）
- `item.moduleRefs` は任意の文字列配列（各 64 文字以内。検証はしない）。カリキュラムのモジュール（`go-01` 等）との対応を持つ
- **`verifyBy` / `outcome` / `goal` / `description` に所属先・企業名・人名・転職活動・人事評価を書かない**（`rationale` と同じ縛り。§9）。
  `verifyBy` は技術的な検証条件だけを書く（「面接想定で説明する」ではなく「ADR を見ずに設計理由とトレードオフを自分の言葉で説明できる」）。
  印が `[3]` で止まっている項目の `verifyBy` は、教材のやり直しではなく「既存の実装について短いドリルか自己説明で 1・2 を確認する」と書く（§1.1）
- `dependsOn` は同一ロードマップ内の `item.key` のみ参照可。**循環参照は検証時（§6 の CLI）に弾く**
- `levels` は 1〜5 を必ず全て含む。`criteria` は**AI への指示書（§4.2）がそのまま参照する**ため、判定基準の正本はここ1か所
- 上限：domains 50、1 domain あたり items 100、1 ロードマップあたり items 500
- **1フィールドの文字数の上限**（バイト数ではなく文字数。日本語1文字＝1）：`name`（ロードマップ・分野・項目・レベル）200、
  `description` / `outcome` / `goal` / `criteria` / `verifyBy` 2,000、`source` 2,048。
  `criteria` / `outcome` / `goal` / `verifyBy` は AI に読ませる文面に載るので、上限が無いと1回の判定で AI が読む量が青天井になる。
  超えたらエラー（`too_long`）。定数は `backend/internal/roadmap/schema.go`
- **定義にないフィールドはエラーにして弾く。** `dependsOn` を `dependOn` と打ち間違えたときに、黙って依存関係が消えて学習パスの並び順だけが静かに狂う、という壊れ方を避けるため
- **検査は1件目で打ち切らず、問題を全部集めて返す。** エラー（CLI が終了コード 1 で止まる）と警告（通す）を分け、`domains[0].items[2].key` の形で場所を添える。実装は `backend/internal/roadmap/`

`origin` による出典の扱い：

| `origin` | 意味 | `source` / `checkedAt` |
|---|---|---|
| `manual` | 利用者が自分で書いた | **任意**（省略可） |
| `external` | 外部の記事・カリキュラム等を元にした | **必須** |
| `builtin` | アプリ同梱のサンプル・既定ロードマップ | **必須**（元ネタがある場合。オリジナルなら `source` は省略し `checkedAt` のみ） |

共通ルール（`../CLAUDE.md`）の「外部由来の定義は出典と確認日を併記」は**外部由来のものだけに掛かる**。
利用者が自分の頭で組んだロードマップにまで出典を要求すると、作成のハードルを無意味に上げる。
`origin` を省略した場合は `manual` とみなす。

### 2.1 `outcome` / `goal`（到達状態）

`item.outcome` は「この項目を身につけると何ができるようになるか」、
`domain.goal` は「この分野を修めると何ができるようになるか」を1〜2文で書く。
**学習パス画面（§7.2）と「次にやること」の表示に使う。**

**`outcome` は必須にしない。** `origin` によって扱いを変える。

| `origin` | `outcome` / `goal` の扱い |
|---|---|
| `manual` | **任意。** 無くてよい。§4.8 の AI 下書きで埋められる |
| `external` | **推奨。** 無い項目があれば検証時に警告する（エラーにはしない） |
| `builtin` | **必須。** アプリ同梱なので用意する側が責任を持つ |

理由：**初学者は自分のロードマップの到達状態を自分では書けない。**
「これを学ぶと何ができるようになるか」が分かっているなら、そもそも先が見えていないという課題が存在しない。
だから `outcome` は「外部のロードマップに書いてあるもの」か「AI が下書きしたもの」を主な供給源とし、
自作ロードマップに書くことを強制しない。

---

## 3. データ配置（リポジトリ内の JSON）

データの持ち主は CLI（Go）と AI であって画面ではない。`backend/` `frontend/` と並ぶ第三のディレクトリ `data/` に置く
（`logs/decisions.md` 2026-09-23）。

### 3.1 ファイル構成

```
data/
  roadmap.json                       ロードマップ（§2 の形）。マスタ。人（と AI）が編集する
  judgments/
    2026-09-20-tour-basics.json      判定。学習ログ1件 = 1ファイル。作ったら以後触らない（§3.3）
    2026-09-22-http-server.json
  state.json                         理解度。導出値。CLI が毎回まるごと書き直す。手で編集しない（§3.4）
  settings.json                      検証ルール・鮮度・重みの設定（§8.3）。省略可
sources.local.json                   根拠の出どころ（学習ログのパス等）。コミットしない（§3.5）
sources.local.json.example           その雛形。こちらはコミットする
```

画面が読むのは `roadmap.json` と `state.json` と `settings.json` の3つだけ。`judgments/` は CLI しか読まない
（項目ごとの履歴は `state.json` に写してあるので画面はそちらを見る）。

### 3.2 設計の要点

- **`judgments/` が一次データ、`state.json` は導出値。** 検証ルールや集計を変えても `recalc` で再計算できる。
  「なぜこの升目が濃いのか」を根拠つきで説明できる
- **判定は追記のみ。** AI がやるのは新しいファイルを1個作るだけで、既存の判定を壊しようがない。
  `git diff` も「新規1ファイル」で読める。訂正したいときも既存ファイルは触らず、新しい判定ファイルを足す（§4.7）
- **処理順は決定的。** ファイル名の昇順（＝日付順）、ファイル内は配列の順で適用する。順序が変わると `lastEvidenceAt` の更新（§3.4）や `preState` の
  クランプ結果が変わるため、順序はファイル名だけで決まるようにする
- **`state.json` は同じ入力から常に同じバイト列になる。** 生成日時のような「今」に依存する値を入れない。
  CI の `verify`（§6）はこの性質を使い、コミットされた `state.json` が再計算結果とバイト単位で一致するかを見る
- **学習ログ本文はリポジトリに置かない。** 正本は `~/workspace/study`（Private）。判定は `evidenceRefs` で出どころを指すだけ。
  これにより判定は実物をコミットでき、リポジトリを Public にできる（§9）

### 3.3 判定ファイル `data/judgments/YYYY-MM-DD-<短い名前>.json`

学習ログ1件（または移行なら分野1つ）につき1ファイル。

```jsonc
{
  "schemaVersion": 2,
  "loggedAt": "2026-10-01",             // 判定を書いた日。ファイル名の日付と一致させる
  "source": "ai",                       // "ai"（AI の判定）| "manual"（人が書いた判定・訂正）| "migration"（skill-map.md からの写し）
  "judge": "claude-code",               // 任意。判定した AI ツール名（"chatgpt" 等）。ai 以外なら省略
  "judgments": [
    {
      "itemKey": "go-syntax-basics",
      "evidenceType": "self_explanation",
      "proposedLevel": 2,               // この根拠が示す最上段（§1.1 の梯子の範囲内）。0 は不合格の報告
      "occurredAt": "2026-09-28",       // 任意。根拠が生じた日。省略時は loggedAt（移行と、まとめ書きのために持つ）
      "evidenceRefs": [                 // 1 件以上必須
        "repo:n-yoshida-dev/study@a1b2c3d/go-react/logs/2026-09-28.md#L10-L30"
      ],
      "rationale": "naked return の可読性の問題に自分の言葉で触れている",
      "confidence": 0.8                 // source が ai のときだけ必須。manual / migration では書かない
    }
  ],
  "unmatched": ["ロードマップのどの項目にも対応づけられなかった記述の要約"]   // 無ければ []
}
```

制約：

- ファイル名は `^\d{4}-\d{2}-\d{2}-[a-z0-9][a-z0-9-]{0,63}\.json$`。`loggedAt` はファイル名の日付と一致する。移行は `YYYY-MM-DD-migration-<domain>.json`
- `judgments` の各要素は §4.3 の出力そのもの。1件の形の検査と V1〜V8 は §4.5
- `proposedLevel` は「この根拠が示す最上段」。根拠の種類の梯子の範囲内（`self_explanation` なら 1 か 2、`unaided_implementation` なら 3 か 4）か、
  **0 ＝ 不合格の報告**（ドリルに落ちた、説明できなかった）。指示書には常に書かせる。
  印を付けない種類（`learning_activity` / `explained_to` / `self_report`）では `proposedLevel` に意味が無く、検証は値を無視する（0 と書く。0 でも不合格の報告にはならない）。**省略はできない**（既定値を設けず、形の検査で棄却する。KNOWLEDGE.md 2026-09-26「段階 C-1」）
- **`evidenceRefs` は 1 件以上必須。** 各要素は `<種別>:<識別子>`。種別は `^[a-z][a-z0-9-]*$`。**種別の一覧は決めない**（検証は書式だけ）が、Git 由来はすべて次の 1 つの文法に統一する：

  | 種別 | 形 | 例 |
  |---|---|---|
  | `repo` | `repo:<owner>/<repo>@<commit>[/<path>[#L<n>[-L<m>]]]`（GitHub の `blob/<commit>/<path>` と同じ並び） | `repo:n-yoshida-dev/study@a1b2c3d/go-react/logs/2026-09-28.md#L10` |
  | `log` | `log:<path>`（コミットされない `learning-logs/` 用。v1.5 のテンプレート利用者の既定） | `log:learning-logs/2026-10-01.md` |
  | `cert` / `work` | 書式だけ予約。**v1 では使わない**（`work:` は経歴が分かる。`cert:` は対応する根拠の種類が無い） | |

  commit を含めるのは、指し先を不変にするため（後からファイルが書き換わっても根拠が消えない）。`verify` は書式だけを見て、到達可能かは見ない（study は Private）。
  種別が増えたら（`article:` 等）そのまま使ってよい（`logs/decisions.md` 2026-09-25「`evidenceRef` は配列 `evidenceRefs` にし…」）
- **`rationale` と `evidenceRefs` には技術的な事実だけを書く。** 所属先・企業名・人名・転職活動・人事評価に関する記述を含めない
  （どちらもコミットされ、公開される前提のため）。機械で検査できないので、コミット前の `git diff` で目視する（§9）
- `source` ごとの扱い（V4 は廃止。§4.5）：

  | `source` | 誰が | `confidence` | V7（保留） |
  |---|---|---|---|
  | `ai` | Claude Code / ChatGPT | **必須**（0〜1） | 適用 |
  | `manual` | 人（訂正・自己入力） | **書かない**（あれば形の検査で棄却） | 適用しない |
  | `migration` | skill-map.md からの写し（1 回きり。`docs/skill-map-migration.md` §8） | **書かない** | 適用しない |

  `manual` / `migration` も V1〜V3・V5・V6・V8 は通す。人の申告でも根拠の種類で付けられる印は変わらない
- 定義にないフィールドはエラー（§2 と同じ理由）。**外枠**（`schemaVersion` / `loggedAt` / `source` / `judge` / `judgments` / `unmatched` 以外のキー）にあればファイル全体をエラーにして CLI が止まり、
  **`judgments[i]` の中**にあればその1件だけを棄却して `rejected` に記録する（§4.5 の形の検査。他の判定は生かす）

### 3.4 理解度 `data/state.json`

`judgments/` を順に適用した結果。**CLI の `recalc` だけが書く。**

```jsonc
{
  "schemaVersion": 2,
  "items": [                            // ロードマップの全項目を定義順に。未着手も入る
    {
      "itemKey": "java-junit",
      "verifiedLevel": 0,               // 導出値。1 から途切れずに印が付いている最上段（§1.1）
      "evidencedLevels": [3],           // 印。昇順
      "preState": "none",               // 印があれば常に none（§1.2）
      "needsReview": false,             // 不合格の報告があった（V6）
      "lastEvidenceAt": "2026-08-05",   // 根拠がまだ無ければ null
      "events": [                       // この項目に適用した判定の履歴。適用順
        {
          "file": "2026-10-01-migration-java-spring.json",
          "index": 1,                   // そのファイルの judgments 配列内の位置
          "occurredAt": "2026-08-05",
          "source": "migration",
          "evidenceType": "implementation",
          "proposedLevel": 3,
          "marked": [3],                // この判定が付けた印（V5 で切り詰めた後）
          "evidenceRefs": ["repo:n-yoshida-dev/study@<凍結コミット>/learner-profile/skill-map.md#L42", "repo:n-yoshida-dev/orgflow@6647ba8"],
          "rationale": "...",
          "confidence": null,           // source が ai 以外は null
          "violations": []              // 記録された違反（V5 / V6）。§4.5
        }
      ]
    }
  ],
  "deferred": [                         // 保留（V7）。適用していない。§4.7
    { "file": "2026-09-22-http-server.json", "index": 1, "itemKey": "go-03", "code": "V7_low_confidence", "detail": "confidence 0.4 < 0.5" }
  ],
  "rejected": [                         // 棄却（形の検査・V1・V2・V3・V8）。適用していない
    { "file": "2026-09-22-http-server.json", "index": 2, "itemKey": "go-99", "code": "V1_unknown_item", "detail": "ロードマップに無い" }
  ]
}
```

`appliedLevel` は持たない（`marked` と `verifiedLevel` で足りる）。`lastEvidenceAt` を更新するのは、
**その判定が付けた最上段の印 ≥ 適用前の `verifiedLevel`** のときだけ（`verifiedLevel` 3 の項目にドリル（印 1）が付いても「3 を保持している」証明にならない。`[3]` で `verifiedLevel` 0 の項目なら印 1 ≥ 0 なので更新される）。更新したときは `needsReview` を消す。

`deferred` と `rejected` を持つのは「握りつぶさない」ため。以前の `llm_responses.violations`（§10.1）と同じ役割。
`rejected` が空でない `state.json` は `verify` が失敗にする（§6）ので、通常はコミット前に判定ファイルを直して空にする。

**書き出しの形**（`verify` がバイト単位で比べるので、ここに書いたとおりに毎回同じバイト列にする。2026-09-26 に段階 C-2 で確定）：

- 整形は Go 標準（`encoding/json` の 2 スペース字下げ）。配列も 1 要素 1 行。末尾は改行 1 つ。キーの順は上の例のとおり
- `<` `>` `&` を `<` 等に置き換えない（`rationale` の `List<String>` のような技術用語を `git diff` で読めるように）
- 空の配列は `null` ではなく `[]`。`lastEvidenceAt` は根拠が無ければ `null`、`confidence` は `source` が `ai` 以外なら `null`
- 日付は `YYYY-MM-DD`。`preState` のゼロ値（内部の `""`）は `"none"` にする
- `index` は判定ファイルの `judgments` 配列での位置（0 始まり）。形の検査で弾いた要素があっても元の位置を書く
- `deferred` / `rejected` はファイル名の昇順、ファイル内は `index` の昇順
- 形の検査で弾いた判定の `code` は `shape_rejected`（V1〜V8 とは別の層なので分ける。棚上げ中の `store.ShapeRejectedCode` と同じ値）。
  `itemKey` は読めればその値、読めなければ `""`

### 3.5 根拠の出どころ `sources.local.json`

AI が学習ログを自分で読みに行くための設定。ローカルのディレクトリ構成を公開リポジトリに書かないため、
実ファイルはコミットせず `.example` だけ置く（`.gitignore` の `*.local.json` で除外済み）。

```jsonc
{
  "logs": { "path": "learning-logs" },        // 既定の置き場（リポジトリ内・.gitignore 済み。v1.5 のテンプレート利用者向け）
  "repos": {                                  // リポジトリ名 → ローカルパスと、学習ログとして読むファイルのパターン
    "n-yoshida-dev/study": { "path": "/home/<user>/workspace/study", "logsGlob": "**/logs/*.md" }
  }
}
```

`repos` のキーは `evidenceRefs` の `repo:<owner>/<repo>` と同じ名前にする。無ければ AI はユーザーに場所を聞く。
判定済みかどうかは `data/judgments/` の `evidenceRefs` の一覧と **path 単位**で突き合わせる（追記型のログなら commit の違いは無視してよい）。

---

## 4. AI 判定

### 4.1 実行フロー

```
学習ログを書く（~/workspace/study。skill-matrix の外）
  ↓
Claude Code で /judge-log を呼ぶ（ChatGPT なら prompts/judge.md を貼る）
  → AI が sources.local.json を読み、学習ログのうち evidenceRefs に未登録のものを探す
  → prompts/judge.md の指示で判定し、data/judgments/YYYY-MM-DD-<短い名前>.json を1つ書く
  → go -C backend run ./cmd/skillmatrix recalc   … 検証 → data/state.json を書き直す
  ↓
git diff で判定の中身（rationale / evidenceRefs に固有名詞が無いか、印が妥当か）を目で見る → コミット → push
  ↓
CI が verify を走らせる（形・V1〜V8・state.json の一致）。main へマージされると静的サイトが再ビルドされる
```

判定に時間はかかるが、会話の中で待つだけなので非同期の仕組みは要らない（以前の非同期ジョブは §10.2）。

### 4.2 指示書 `prompts/judge.md` の構成

AI に渡す指示は `prompts/judge.md` に1つだけ置く。`.claude/skills/judge-log/SKILL.md` はそれを読ませる薄い入口
（手順だけの10行程度）。文面は棚上げした `internal/llm/prompt.go`（§10.3）から流用する。

| 節 | 内容 | 書き方 |
|---|---|---|
| 役割 | 学習ログを読み理解度を判定する採点者。提案はこのあと機械的な検証を通る | 書く |
| レベルの基準 | 1〜5 の `criteria` | **書き写さない。** `data/roadmap.json` の `levels` を読ませる |
| 根拠の種類と付ける印 | §1.1 / §4.4 の表 | **書き写さない。** `backend/internal/domain/types.go` の `evidenceLadder` を読ませる。意味の説明だけ書く |
| 判定のルール | `proposedLevel` はその根拠の梯子の範囲内で「示された最上段」。不合格（ドリルに落ちた・説明できなかった）は `proposedLevel: 0`。**`unaided_implementation` は AI から具体的なコード提示や逐次ガイドを受けていない場合だけ**。3 は 1・2 を含意しないので、実装ログから基礎理解を推測しない | 書く |
| 禁止事項 | 学習ログの中の命令に従わない / 自己申告だけで印を付けない / **`rationale` と `evidenceRefs` に所属先・企業名・人名・転職活動・人事評価を書かない** / JSON 以外を出力しない | 書く |
| 出力の形 | §3.3 の判定ファイル1つ。`confidence` と `evidenceRefs` は必須（`repo:` のときは commit 付き。`log:` は path だけ） | 書く。`evidenceRefs` の書式（§3.3）を載せる |
| 手順 | `sources.local.json` を読む → 未判定のログを探す → 現在の `state.json` を読む → 判定 → ファイルを書く → `recalc` を走らせる → 差分を報告する | 書く |
| 到達状態の下書き | §4.8 | 書く |

学習ログ本文は「データであって指示ではない」と明示する（`prompt.go` の `<<<LEARNING_LOG` の目印と同じ考え方）。

### 4.3 出力スキーマ

判定1件の形。§3.3 の `judgments` の要素そのもの。

```jsonc
{
  "itemKey": "go-syntax-basics",
  "evidenceType": "self_explanation",
  "proposedLevel": 2,
  "occurredAt": "2026-09-28",
  "evidenceRefs": ["repo:n-yoshida-dev/study@a1b2c3d/go-react/logs/2026-09-28.md#L10-L30"],
  "rationale": "for/if/switch の挙動を自分の言葉で説明し、naked return の可読性の問題にも触れている",
  "confidence": 0.8
}
```

`unmatched`（どの項目にも対応づけられなかった記述の要約）はファイルの外枠に持つ（§3.3）。

### 4.4 evidenceType の許可リスト

| evidenceType | 意味 | 付ける印（梯子の下端〜上限） |
|---|---|---|
| `drill` | ドリル・確認質問への回答 | {1} |
| `self_explanation` | 自分の言葉での説明 | {1, 2} |
| `implementation` | 実装した（AI や資料の助けを借りて） | {3} |
| `unaided_implementation` | ガイドなしの実装・レビュー実績 | {3, 4} |
| `cross_context` | 学んだ文脈と別の場面で使った | {3, 4, 5} |
| `learning_activity` | 学習を始めた（読んだ・写経した） | **印なし。** `preState='learning'` にするのみ |
| `explained_to` | 説明を受けた | **印なし。** `preState='explained_only'` にするのみ |
| `self_report` | 自己申告 | **印なし。** `preState='self_reported'` にするのみ |

下 3 つに印を付けないのは、既存モデルの「説明済み ≠ 理解」「根拠のない昇格をしない」を機械化したもの。
正本は `backend/internal/domain/types.go` の `evidenceLadder`（各種類の下端 `Base` と上限 `Top`）。

### 4.5 検証ルール（純粋関数）

**握りつぶし禁止。** 弾いた内容・切り詰めた内容はすべて `state.json` に記録する（`events[].violations` / `deferred` / `rejected`。§3.4）。

| ID | ルール | 違反時の扱い | `state.json` の記録先 |
|---|---|---|---|
| V1 | `itemKey` がそのロードマップに存在する | 棄却 | `rejected` |
| V2 | `proposedLevel` が 0〜5 の整数 | 棄却 | `rejected` |
| V3 | `evidenceType` が許可リストにある | 棄却 | `rejected` |
| V4 | （欠番。「昇格幅は +1 まで」は 2026-09-25 に廃止） | ― | ― |
| V5 | `proposedLevel` がその根拠の梯子の範囲内（4.4 の表） | 上限超えは上限へ、下端未満（0 以外）は下端へ切り詰めて適用 | `events[].violations` |
| V6 | `proposedLevel` が 0（不合格の報告） | 印を付けず `needsReview=true` を立てる | `events[].violations` |
| V7 | `confidence` が閾値（既定 0.5）未満。**`source: ai` のときだけ** | 適用せず保留 | `deferred` |
| V8 | 1ファイルあたりの判定件数が上限（既定 20）以内。移行も免除しない | 超過分を棄却 | `rejected` |

V4 を廃止した理由：1 件の根拠が同じ梯子の下位を同時に証明するのは自然（`unaided_implementation` → {3, 4}）で、
自力実装の明確な証拠があってももう 1 回実証を要求するのは根拠モデルとして不自然。AI の過大評価の歯止めは
指示書（§4.2 の `unaided_implementation` の条件・`confidence` 必須・`rationale` と `evidenceRefs` 必須）と `git diff` の目視へ移した。

適用の規則（`ApplyJudgment`）：(1) 印を付ける（4.4 の集合を `proposedLevel` で上限を切った範囲）→ (2) `verifiedLevel` を導出し直す →
(3) 印があれば `preState` は `none`、無ければ根拠の種類が立てる値 → (4) `lastEvidenceAt` は §3.4 の条件で更新 → (5) `proposedLevel` が 0 なら印を付けず `needsReview`。

**V1〜V8 の前に、形の検査を通す**（`internal/judgment` の `Parse`。棚上げ中の stub・Claude API クライアントは `internal/llm` の `ParseOutput` 経由で同じ関数を通る。
V1〜V8 は意味の検査で `internal/domain` が担当し、同じ検査を2か所に書かない）。
形の崩れた判定はその1件だけを棄却し（`rejected`）、同じファイルの他の判定は生かす。
ファイル全体が JSON として読めない・外枠の型が合わない（`judgments` が配列でない等）・`judgments` が無い場合は、そのファイルをエラーとして CLI が止まる。

| 形の検査 | 違反時の扱い |
|---|---|
| 型が合っている（`proposedLevel` が整数、など） | 棄却して記録 |
| 必須の欄（`itemKey` / `proposedLevel` / `evidenceType` / `evidenceRefs` / `rationale`）が揃っている | 棄却して記録 |
| `rationale` が空でない | 棄却して記録 |
| `evidenceRefs` が 1 件以上の配列で、各要素が §3.3 の書式（`<種別>:<識別子>`） | 棄却して記録 |
| `confidence` は `source: ai` なら必須で 0〜1 の範囲内、それ以外の `source` なら**存在してはいけない** | 棄却して記録 |
| `occurredAt` があれば `YYYY-MM-DD` | 棄却して記録 |
| 定義にないフィールドが無い（判定1件の中。外枠はファイル単位のエラー。§3.3） | 棄却して記録 |

**配点の分配はしない。** 1つのログが複数項目にまたがる場合も、項目ごとに独立して判定させる。合計制約は設けない（LLM が苦手で、意味もない）。

**`EvidenceRefs` は記録であって判定材料ではない。** `ApplyJudgment` は `EvidenceRefs` を見ない（形の検査だけが見る）。

### 4.6 判定の主体とコスト

v1 の判定は **Claude Code / ChatGPT** が行う。どちらもサブスクリプション内で、**API 課金は発生しない。API キーもどこにも置かない。**

サーバから Claude API を呼ぶ実装（`internal/llm` のクライアント）と、開発中に LLM を呼ばない stub（`LLM_PROVIDER=stub`）は
棚上げした（§10.4）。画面開発に要る「それらしい判定ファイル」は AI に1回出させるか手で書く。

### 4.7 判定の確認と反映

**確認は `git diff`。** 判定ファイルは AI が書いた時点では作業ツリーにあるだけで、コミットするまで何も確定しない。
`recalc` の出力（どの項目が何レベルになったか、違反は何か）と差分を見て、納得したらコミットする。
納得できなければ判定ファイルを直すか消す（コミット前なので「追記のみ」の制約はまだ掛からない）。

コミット後に訂正したいときは、**既存の判定ファイルを触らず、`source: "manual"` の判定ファイルを新しく足す**（§3.3）。
以前の「手動上書き API」と「反映モード（auto / confirm）」はこれで置き換える（§10.5）。

V7 で保留になった判定は `state.json` の `deferred` に残り、項目詳細画面に「レビュー待ち」として出る。
採用したいときは同じ内容を `source: "manual"`（`confidence` 無し）で書いた判定ファイルを足す。採用しないなら放置してよい（適用されない）。

### 4.8 到達状態（`outcome`）の AI 下書き

`outcome` が空の項目について、`prompts/judge.md` の手順で AI に下書きを頼み、**`data/roadmap.json` を直接編集させる。**
入力は分野名・`goal`、各項目の `key` / `name` / `description` / `dependsOn`、レベル定義。1項目 200 文字以内。

- 下書きかどうかの印（以前の `outcome_source`。§10.1）は v1 では持たない。**何が AI の下書きかは `git diff` と履歴で分かる**
- **必ず人が手直しできる形にする。** AI が書いた到達状態が的外れなまま「先が見える」画面に出ると、
  間違った地図を渡すことになり、当初の課題（先が見えない不安）をかえって悪化させる。コミット前に読む

---

## 5. スコア集計（純粋関数）

計算は2か所に分かれる。**分かれ目は「今日」に依存するかどうか。**

| 計算 | 「今日」に依存 | 置き場所 | 出力先 |
|---|---|---|---|
| 判定の検証と適用（V1〜V8、`ApplyJudgment` / `ApplyJudgments`） | しない | Go `backend/internal/domain/` | `data/state.json` |
| 分野のロールアップ（レベル別件数・進捗率）、学習パス（依存順・done/current/upcoming）、ツリーの配置 | しない | TypeScript `frontend/src/features/*/` | 画面 |
| 鮮度（`Staleness`）、要再確認（`NeedsAttention`）、次にやること Top N、目標日の逼迫度 | **する** | TypeScript `frontend/src/features/*/` | 画面 |

Go 側（`backend/internal/domain/`）は **DB・HTTP・LLM クライアントを import しない。** 単体テスト必須。
`RollupDomain` / `Staleness` / `NextActions` / `BuildPath` の Go 実装は仕様の参照実装として残し、CLI の `recalc` が
端末に出す要約（「次にやること Top 5」）に使う。TypeScript 側は同じ仕様を写し、**同じ入力例で同じ答えになることを Vitest で確かめる**
（テストの入力例は `backend/testdata/` の JSON を共有する）。DOM・ファイル読み込みを import しない。

```go
// 判定1件を検証して現在の状態に適用する（V2〜V7）。違反は握りつぶさず返す。
func ApplyJudgment(cur ItemState, j Judgment, rules Rules) Applied

// 1ファイル分の判定をまとめて適用する（V1・V8 を追加で見る）。
func ApplyJudgments(rm Roadmap, states map[ItemKey]ItemState, js []Judgment, rules Rules) BatchResult

// 最終根拠日からの経過で鮮度を決める（減衰ではない）。
func Staleness(now, lastEvidenceAt time.Time, cfg StalenessConfig) StalenessLevel

// 要再確認か。根拠が古い場合と、降格提案があった場合（V6）の両方を拾う。
func NeedsAttention(st ItemState, now time.Time, cfg StalenessConfig) bool

// 分野のロールアップ。進捗率 = 達成レベルの合計 / (項目数 × MaxLevel)
func RollupDomain(d Domain, states map[ItemKey]ItemState, now time.Time, cfg StalenessConfig) DomainSummary
func RollupAll(rm Roadmap, states map[ItemKey]ItemState, now time.Time, cfg StalenessConfig) []DomainSummary

// 目標日に対する逼迫度。項目の並び順には使わない（§5.1 の注記）。
func BuildSchedule(rm Roadmap, states map[ItemKey]ItemState, now time.Time) Schedule

// 次にやること Top N
func NextActions(rm Roadmap, states map[ItemKey]ItemState, now time.Time, cfg StalenessConfig, w Weights, n int) []Action

// 学習パス（§7.2）。分野の項目を依存関係の順に並べ、done / current / upcoming に分類する。
// 依存関係の解決はトポロジカルソート。同順位は定義順で安定させる。
func BuildPath(d Domain, states map[ItemKey]ItemState) Path
```

### 5.1 優先度スコア

```
priority = w.Readiness * readiness   // 依存項目がすべて L1 以上なら 1.0、未達なら 0
         + w.Gap       * gap         // (MaxLevel - currentLevel) / MaxLevel
         + w.Staleness * staleBonus  // stale=1.0 / aging=0.5 / それ以外=0
         + w.Unlocks   * unlocks     // これを終えると着手可能になる項目数（正規化）
```

重みは `data/settings.json` に切り出す（ハードコード禁止。§8.3）。並び順は
**「着手不能なものを常に後ろ」→「priority の降順」→「定義順」** で安定させる。

> **`urgency`（目標日までの逼迫度）を式から外した。**
> 逼迫度はロードマップ全体で1つの値になるため、全項目に同じ数を足すことになり**順位が一切変わらない**。
> 優先度に混ぜても意味がないので、`BuildSchedule` が返す独立した値として画面に出す
> （「目標日まで残り30日、未着手22項目、1日あたり0.7項目のペースが必要」のように表示する）。
> 代わりに、**`unlocks`（この項目を終えると何項目が着手可能になるか）** を入れた。
> こちらは項目ごとに値が違うので、実際に順位を動かす。

`Action` には `item.verifyBy`（次の確認方法）と `item.outcome`（身につくと何ができるか）をそのまま載せる。
加えて `PendingLevels`（印はあるが `verifiedLevel` に届いていない段。例：`[3]` の項目なら `[3]`、未確認は 1・2）を持ち、画面はこれで文言を変える（§7.4）。
`gap` は `verifiedLevel` から計算する（変更なし）。
**「次に何をやるか」だけでなく「やると何ができるようになるか」を毎回同時に見せる**ため。

---

## 6. CLI（`backend/cmd/skillmatrix`）

判定ファイルを検証し、理解度を再計算する。**DB・HTTP・LLM クライアントを import しない**（読むのは `internal/domain` / `internal/roadmap` / `internal/judgment` だけ。
`internal/llm` は棚上げ中の Claude API クライアントと同居しているので import しない）。

```bash
go -C backend run ./cmd/skillmatrix recalc --data ../data   # 検証して data/state.json を書き直す（手元で使う）
go -C backend run ./cmd/skillmatrix verify --data ../data   # 検証だけ。state.json は書かない（CI で使う）
```

| モード | やること | 終了コード 1 になる条件 |
|---|---|---|
| `recalc` | `roadmap.json` の検査（§2）→ `judgments/` を順に読み、形の検査と V1〜V8 → `state.json` を書き出す → 要約（分野ごとの進捗、変わった項目、違反、次にやること Top 5）を端末に出す | `roadmap.json` にエラー / 判定ファイルが読めない（外枠が壊れている・ファイル名と `loggedAt` の不一致・**外枠の**未知のフィールド。判定1件の中の問題は `rejected` に記録して続行。§3.3） |
| `verify` | `recalc` と同じ検証を走らせ、**書き出す代わりに、コミットされた `state.json` と再計算結果をバイト単位で比べる** | `recalc` の条件に加えて：`rejected` が空でない / `state.json` が一致しない / `state.json` が無い |

- 違反は「どのファイルの何件目の、どの項目の、どのルールか」を一覧で出す。V5〜V7 は記録されるが `verify` を失敗にはしない
  （梯子の範囲に切り詰める、不合格の報告、確信度の低い提案を保留にする、はモデルの想定どおりの動きで、修正を要求するものではない）
- `rejected` で失敗にするのは、ロードマップに無い項目や範囲外のレベルが**コミットされた判定ファイルに残っている**のは
  直すべき不備だから。コミット前に `recalc` で気づいて直す。CI はその取りこぼしを止める
- `state.json` の一致まで見るのは「AI が判定ファイルを書いたのに `recalc` を走らせ忘れた」「`state.json` を手でいじった」を止めるため
- 終了コード：0 = 問題なし、1 = 上の条件、2 = 使い方の誤り・ファイル IO の失敗
- 設定（閾値・上限・鮮度・重み）は `data/settings.json` から読む。無ければ `domain.DefaultRules()` 等の既定値（§8.3）。
  書かれていないキーは既定値のまま。**未知のキーは終了コード 1**（`confidenceThreshhold` のような打ち間違いで閾値が黙って既定値に戻るのを防ぐ）
- `--data` の既定は `../data`（`go -C backend run` で動かす前提）
- `judgments/` が無いときは判定 0 件として扱う（git は空のディレクトリを記録しないので、判定をコミットする前の CI には無い）。
  `.` で始まるファイル（`.gitkeep` 等）は読まない。中にディレクトリがあれば終了コード 1
- 外枠の崩れた判定ファイルは 1 本目で止めず全部挙げてから終了コード 1 にする。このとき `recalc` は `state.json` を書かない
- `recalc` は `rejected` が空でなくても `state.json` を書き出す（何が弾かれたかを `state.json` と端末で見られるように）。そのまま
  コミットすると `verify` が失敗することを端末に出す
- 「次にやること」と鮮度は「今日」に依存するので端末の要約にだけ使い、`state.json` には入れない（§3.2）。
  計算は Go 側の参照実装（`domain.NextActions` / `RollupAll`）で、画面の TypeScript 実装とずれうる。画面が正

CI では `verify` を `main` と全 PR で走らせる。**`state.json` は生成物だがコミットする**（画面が読むため。生成物をコミットして CI で一致を確認するのは、生成物をリポジトリに置く構成での定石）。

---

## 7. 画面

静的サイト。**すべて読み取り専用。** 更新は `data/` へのファイル追加で行い、画面からは何も書き込まない。

**入口は 2 つ**（2026-09-25 決定。`logs/decisions.md`「画面は『公開ビュー』と『作業ビュー』の 2 つの入口に分ける」）。
見る人の問いが違うので、同じ `state.json` を別の絞り込みで見せる。

| 入口 | 見る人 | 答える問い | 画面 |
|---|---|---|---|
| **公開ビュー**（トップ `/`） | 採用担当者など初めて見る人 | 何をどこまでできるか。それは信用できるか | §7.3 の 1 画面 |
| **作業ビュー**（`/plan` 配下） | 自分 | 次に何をやるか。どこが古いか。どの判定が保留か | 下の 3 画面 |

作業ビューの画面：

| 画面 | 内容 |
|---|---|
| ダッシュボード（マトリクス） | サマリー帯（分野 × `verifiedLevel` の到達項目数、実装根拠あり・理解未確認の件数）＋ 可変長グリッドのマトリクス ＋ 次にやること Top 5 ＋ 目標日の逼迫度（設定されていれば） |
| **学習パス** | **分野を選ぶと、項目が依存順に縦一列で並ぶ。済 / 今ここ / この先と到達状態を見せる（§7.2）** |
| 項目詳細 | 見出しに 3 行（Verified Level／付いている印／未確認の段）、印の履歴（`state.json` の `events[].marked`）、各判定の根拠（`rationale` / `evidenceRefs`）、レビュー待ち（`deferred`）、次の確認方法、到達状態（§7.4）。**編集はしない** |

以前のログイン・ログ投稿・ログ一覧・ロードマップ管理の画面は v2（§10.7）。

### 7.1 マトリクスの表示形式

**行 = 分野、列 = その分野の詳細項目の可変長グリッド。** 分野ごとに列数が違う。

```
Go           ■ ■ □ □ □ □        (6項目)
React / TS   ■ ■ □ □ □          (5項目)
Java/Spring  ■ ■ ■ ■ ■ ■ ■ ■    (8項目)
```

分野ごとに詳細項目が異なるため、共通の列軸を持つ真の行列にすると疎行列になる。上部に「分野 × `verifiedLevel` の到達項目数」のサマリー帯を置いて一覧性を補う。
サマリー帯には「実装根拠あり・理解未確認」（印が `verifiedLevel` より上にある項目。`DomainSummary.PendingCount`）の列を 1 つ足す。

**二重符号化**：升目の**色の濃さ = `verifiedLevel`（0〜5 の6階調）**、**枠線・斜線 = 要再確認**（`staleness` が `stale`、または `needsReview=true`）。
**角の印 = 上位の根拠あり**（`evidencedLevels` の最上段が `verifiedLevel` より上。例：`[3]` で `verifiedLevel` 0）。
ツールチップに「Verified 0 / 根拠の印 3 / 未確認 1, 2」と `preState` を出す。

配色は `dataviz` の指針に従い、light / dark 両テーマで判別できる連続スケールを使う。

### 7.2 学習パス画面（一列表示 / スキルツリー表示）

マトリクスが**今の状態**を見せる画面なのに対し、こちらは**道筋**を見せる画面。
解こうとしている課題は「初学者は先が見えない。暗いトンネルを壁伝いに進んでいる感覚」（PLAN.md）。

分野を選ぶと、その分野の項目を **2つの表示で切り替えて**見られる。データは共通で、描き分けだけが違う。

| 表示 | 答えている問い | 既定 |
|---|---|---|
| 一列表示 | 次に何をやるか（順番が一意に決まる） | 初回はこちら |
| スキルツリー表示 | どの前提を取ると何が解放されるか（依存の形が見える） | 切り替えを記憶する |

判断の経緯は `logs/decisions.md`（2026-08-22 の3件）。

#### 一列表示

項目が**依存関係の順に縦一列**で並ぶ。

```
Go        目標: Java との差分を理解したうえで、Go で Web API を設計・実装・テストできる
          進捗: 6項目中2項目（レベル1以上）

  ✓ go-01  基本構文                     レベル2
           Go のコードを読んで型と値の流れを追え、Java との差分を説明できる

  ▶ go-02  メソッド・インターフェース     レベル0  ← 今ここ
           身につくと: 型に振る舞いを持たせ、インターフェースで実装を差し替えられる
           次の確認方法: Tour Methods の後にドリル

  ・ go-03  エラーハンドリング            レベル0
           身につくと: error を戻り値で扱い、wrapping で文脈を足せる

  ・ go-04  並行処理                      レベル0
  ・ go-05  net/http で最小 API サーバー   レベル0
  ・ go-06  テスト                        レベル0
```

並び順はトポロジカルソート（同順位は定義順で安定）。分類は3つだけ。
`done`（レベル1以上）/ `current`（依存順で最初に現れる未達かつ着手可能な項目）/ `upcoming`（その他）。

#### スキルツリー表示

同じ分野の項目を、**前提が上・派生が下**になる階層図として描く。

```
Go        目標: …   進捗: 6項目中2項目

  段0      [go-01 基本構文 L2]
              │
  段1      [go-02 メソッド/IF ◎]      ← 解放済み・未着手（今ここ）
              │         │
  段2      [go-03 エラー 🔒] [go-04 並行処理 🔒]
              │
  段3      [go-05 net/http 🔒] ─── (react-03 fetch と連携)   ← 他分野のゴーストノード
              │
  段4      [go-06 テスト 🔒]
```

- **分野ごとに描く。** 他分野の項目に依存している場合、その項目を**灰色のゴーストノード**として置き、
  クリックでその分野のツリーへ移動する。ゴーストノードからは線を引くが、ゴーストノード自身の依存は描かない
- **レイアウトは自前の階層配置。グラフ描画ライブラリは使わない。**
  段 = 依存の深さ（前提なし = 0段、前提の最大段 + 1）。同じ段は定義順で横に並べる。線は SVG で引く。
  配置計算は**フロントエンドの純粋関数**（`features/path/layout.ts`、Vitest で検証）。
  入力は `roadmap.json` と `state.json` から組んだ学習パス、出力は各ノードの `(段, 列)` と辺のリスト。DOM を import しない
- ノードの状態は4つ。色はマトリクス（§7.1）と同じ連続スケールを使う

  | 状態 | 条件 | 見た目 |
  |---|---|---|
  | 未解放 | `ready=false`（前提に L0 がある） | 灰色・鍵マーク。線も薄く |
  | 解放済み・未着手 | `ready=true` かつレベル0 | 枠を強調。`current` の項目は「今ここ」ラベル |
  | 習得済み | `verifiedLevel` 1以上 | 塗りの濃さ = `verifiedLevel`（6階調）。`stale` / `needsReview` は枠線・斜線、上位の根拠ありは角の印（§7.1 と同じ符号化） |
  | 深掘り候補 | 習得済みかつ「次にやること」Top N に入っている | 習得済みの上にバッジ |

- 1分野の項目が30を超えて線が交差して読めなくなったら、その時点で React Flow + dagre への切り替えを検討する
  （`logs/decisions.md` の「見直す条件」）

#### 両表示に共通の制約

- **`current` と「次にやること」の1位は一致しないことがある。両者は別の問いに答えている。**

  | | 答えている問い |
  |---|---|
  | 学習パスの `current` | 道のりのどこまで来たか（レベル1に達した項目の先端） |
  | 次にやること の1位 | 今いちばん時間を使うべき項目（**済んだ項目の深掘りも含む**） |

  例：`go-01` がレベル1に達していれば、パス上では「済」で先端は `go-02` に進む。
  一方「次にやること」は `go-01` をレベル2へ引き上げることを1位に挙げうる。どちらも正しい。
  **画面ではこの違いが分かるように出すこと。** 一列表示では済項目に「深掘り候補」の印、
  ツリー表示では上記「深掘り候補」バッジで示す
- `outcome` が空の項目は「未記入」と分かるように出す（一列表示では行を詰めて、ツリー表示ではノード内で）。
  埋めるのは §4.8 の手順で、画面からは頼めない
- 常に上部に `domain.goal` と進捗を出す。「あと何個でどうなるか」が視界から消えないようにする

### 7.3 公開ビュー（採用担当者向け。トップ `/`）

1 画面で完結する。答えるのは「何をどこまでできるか」「それは信用できるか」の 2 つだけ。

| 要素 | 内容 |
|---|---|
| ヘッダ | 画面の切り替え（理解度台帳／作業ビュー）とリポジトリへのリンクだけ。個人名・プロフィールへのリンクは置かない（個人のリンクは README に限る。2026-09-25 決定） |
| 見出しと 2 文 | 「学習ログを AI が判定し、ルールで検証して履歴に残している。自己申告では上がらない」の 1 文と、「N 分野 M 項目のうち根拠のある項目が X、実装まで届いているものが Y」の 1 文（X は印が 1 つでもある項目、Y は `verifiedLevel` 3 以上。`[3]` だけの項目は X に入り Y には入らない） |
| 分野ごとの行 | 分野名、`goal`、「M 項目中 X に根拠」、その下に項目のタイル |
| **タイル** | 項目キー・`verifiedLevel`（0 は「未着手」と文字で。**`verifiedLevel` 0 で印が上位にある項目**は「実装済み・基礎未確認」。L1 以上の項目はレベルだけを出し、上位の印は根拠側の注記で示す。2026-09-26）・項目名（括弧の補足を外し 2 行まで）を**常時表示**。幅 150px 程度で横幅に合わせて折り返す。塗りは §7.1 と同じ 6 階調。**押さなくても何の項目か分かる**ことを優先する（2026-09-25 決定） |
| 根拠（タイルを押すと、その行の直下に開く） | 項目名、レベル名、`outcome`、印が付いた判定を新しい順に「日付・根拠の種類・`rationale` の一文」。印が付かなかった判定は「これだけでは上がらない扱い」と正直に出す。`[3]` の項目は「実装の根拠はあるが基礎の確認が未了」と出す |
| 色の読み方 | L0〜L5 の 6 段を名前つきで 1 行。名前は初めて見る人向けの言い換えで固定：未着手／基礎を確認した／自分の言葉で説明できる／資料を見れば実装できる／自力で実装しレビューできる／別の場面でも使える（判定基準の正本は `roadmap.levels` の `criteria` のまま。ここは表示の言い換えで、画面コードが持つ） |
| どうやって決まるか | 4 段の手順（ログを書く → AI が判定 → 機械的な検証 → 履歴に残る）。ここは本当に手順なので番号を付ける |
| フッター | 「Built with skill-matrix」でリポジトリへリンクするだけ（2026-09-25 決定。個人の宣伝リンクは README に限る） |

**出さないもの**：`preState`、鮮度（`staleness`）、要再確認の枠線、保留（`deferred`）、棄却（`rejected`）、優先度と次にやること、検証ルールの記号（V5 等）、
`evidenceRefs` の生の文字列、`confidence`。これらは作業ビューにある。載せる項目を増やしたくなったら「採用担当者が 10 秒で読めるか」で判断する。

見た目：1 書体（IBM Plex Sans JP。数字と日付だけ IBM Plex Mono）、色は L1〜L5 に藍の 5 段階で L0 は無色（合わせて §7.1 の 6 階調）、
選択中のタイルにだけ朱、カードも影も使わず罫線と余白で区切る。
参照実装は `docs/demo/public-view.html`（2026-09-25 に本人が了承したデモ。ダミーデータ入りの単体 HTML）。
**見た目と画面構成の参照であって、文言のうち検証の説明は §4.5 に、ヘッダ・フッタは上の表に従う。** デモのダミーデータは旧モデル（1 本の数直線）のままなので、印の例としては使わない。

### 7.4 項目詳細と「次にやること」の文言

- 項目詳細の見出しは 3 行：**Verified Level n**／付いている印（`evidencedLevels`）／未確認の段（1〜最上段の印のうち印が無いもの）。
  履歴は `events[].marked` で「いつ・どの根拠が・どの印を付けたか」を並べる
- 「次にやること」で `PendingLevels` がある項目は、文言を **「既存の実装について L1 / L2 を短いドリル・自己説明で確認する」** にし、
  `verifyBy` はその下に添える。`[3]` の項目を「基礎からやり直し」と見せない（§1.1）
- 一列表示・ツリー表示のノードにも、上位の根拠ありのバッジを出す（`done` の判定は `verifiedLevel` ≥ 1 のまま。`current` の判定は変えない）

---

## 8. 技術構成

### 8.1 ディレクトリ

```
data/                       ロードマップ・判定・理解度の JSON（§3）
prompts/judge.md            AI への指示書（§4.2）
sources.local.json.example  根拠の出どころの雛形（§3.5）
backend/
  cmd/skillmatrix/          recalc / verify の CLI（§6）
  internal/
    domain/                 純粋関数（理解度モデル・検証・集計）※ DB/HTTP/LLM を import しない
    roadmap/                マスタ JSON のスキーマ定義と検証
    judgment/               判定の JSON の形の検査（Parse）。v1 の中核
  testdata/                 ダミーのロードマップ・学習ログ・判定
  ── 以下は v2 へ棚上げ（§10）。消さない ──
  cmd/server/ cmd/worker/ internal/{store,httpapi,worker,config} internal/llm/{anthropic,prompt,stub,output}.go migrations/
  （llm/output.go は judgment への入口と、生の出力・消費トークン数を運ぶエラー型。呼ぶのは棚上げ中のコードだけ）
frontend/
  src/
    data/                   data/*.json の読み込みと型（Vite の別名で data/ を指す）
    features/public/        公開ビュー（§7.3。タイルと根拠）
    features/matrix/        マトリクス・サマリー帯（rollup は純粋関数）
    features/path/          学習パス（一列表示・スキルツリー表示。build.ts / layout.ts は純粋関数）
    features/next/          次にやること Top N（priority.ts は純粋関数）
    features/item/          項目詳細
    lib/staleness.ts        鮮度（純粋関数）
    components/
docs/                       要件定義・アーキテクチャの人間向け解説
.claude/skills/judge-log/   /judge-log の入口（prompts/judge.md を読ませるだけ）
```

### 8.2 スタック

`../CLAUDE.md` のデフォルトから外れる点は `PLAN.md`「技術的な方針」にある。確定したもの：

- **サーバと DB を持たない。** ローカル開発は `npm run dev` と Go の CLI だけ。Docker Compose は v2 用に残す（v1 では使わない）
- フロントは React + TypeScript + Vite。React Router は **ハッシュルーティング**（GitHub Pages では深いパスの直リンクが 404 になるため）。
  **TanStack Query は入れない**（サーバ通信が無い）。React Hook Form / Zod は入力フォームが無いので入れない。Recharts はサマリー帯に要れば使う
- `data/` は `frontend/` の外にあるので、`vite.config.ts` の別名（`@data`）で指し、dev サーバの `server.fs.allow` に加える
- 静的サイトは GitHub Actions でビルドして GitHub Pages に置く（`main` への push で更新）

### 8.3 設定 `data/settings.json`

CLI と画面が共通で読む。秘密情報は入らないのでコミットする。無ければ既定値。

```jsonc
{
  "rules":     { "confidenceThreshold": 0.5, "maxItemsPerLog": 20 },                       // §4.5。domain.DefaultRules()。maxLevelStep は V4 廃止で消えた
  "staleness": { "freshWithinDays": 30, "agingWithinDays": 90 },                          // §1.3。domain.DefaultStalenessConfig()
  "weights":   { "readiness": 1.0, "gap": 0.8, "staleness": 0.3, "unlocks": 0.5 },        // §5.1。domain.DefaultWeights()
  "nextActions": { "limit": 5 },
  "site": { "repoUrl": "https://github.com/n-yoshida-dev/skill-matrix" }   // 画面のヘッダ・フッターが指すリポジトリ（§7.3）。v1.5 の利用者は自分のものに変える
}
```

環境変数は v1 では使わない（サーバ版の一覧は §10.8）。ローカル固有の値は `sources.local.json`（§3.5）だけ。

---

## 9. セキュリティとプライバシー

- **API キーをどこにも置かない。** v1 はサーバから LLM を呼ばない。v2 で戻すときも必ずバックエンドだけが持つ（§10.9）
- **学習ログの本文はリポジトリに置かない。** ログには職場・実務・転職の話が混ざりうる。置くのは判定（どの項目が・どのレベルに・なぜ）だけ
- **`rationale` と `evidenceRefs` は技術的な事実だけ。** 所属先・企業名・人名・転職活動・人事評価を含めない（§3.3）。
  指示書で縛り、コミット前の `git diff` で目視する。**リポジトリを Public にする直前に `data/judgments/` 全件を読み直す**
- **`sources.local.json` はコミットしない。** ローカルのディレクトリ構成を公開しない
- **画面の外部依存は Google Fonts だけ。** 訪問者のブラウザからフォント要求が Google に出るが、学習データも判定も送らない。
  同梱に切り替える理由（オフライン閲覧・訪問者のプライバシー方針）が出たら `frontend/public/fonts/` に置く（2026-09-26 に本人了承）
- **`data/judgments/` と `data/state.json` は実物をコミットする。** これは `../CLAUDE.md`「実データをコミットしない」の
  例外で、上の3点（本文を置かない・技術的事実だけ・公開前の目視）を守ることが条件。`backend/testdata/` は引き続きダミーだけ
- 目視で固有名詞を拾い損ねる事故が起きたら、`verify` に禁止語リストの検査を足す。それでも不安なら Public をやめて Private に戻す（仕組みは変えずに済む）

---

## 10. v2 へ棚上げした仕様（他人にも使わせる版）

2026-09-23 に棚上げした分。**消さない。** 実装済みのコードは `backend/` に残っている（`TODO.md` フェーズ3）。
戻す条件は `logs/decisions.md` 2026-09-23「見直す条件」。ここは棚上げ時点の内容で、v1 の変更（`evidenceRef` の追加など）は反映していない。
戻すときに v1 の §3・§4 と突き合わせて直す。
**棚上げしたコードのコメント（`cmd/server` / `cmd/worker` / `internal/{store,httpapi,worker,config}` / `internal/llm/{anthropic,prompt,stub,output}.go` / `migrations/`）と
`KNOWLEDGE.md` の過去の記録は旧番号のまま。** §3 → §10.1、§4.1 → §10.2、§4.2 → §10.3、§4.6 → §10.4、§4.7 → §10.5、§4.8 → §10.6、§6 → §10.7、§8.3 → §10.8、§9 → §10.9 と読み替える。
v1 の中核である `internal/domain` にも旧番号の参照が残っている（`types.go` の `evidenceOrder` のプロンプトキャッシュの注記 §4.2 → §10.3、
`apply.go` / `types.go` の「保留分は `assessment_events` に書かず」→ v1 では `state.json` の `deferred`）。2-1 で `EvidenceRef` を足すときに直す。

### 10.1 データモデル（PostgreSQL）

#### テーブル

```
users
  id              uuid pk
  github_id       bigint unique not null
  login           text not null
  avatar_url      text
  apply_mode      text not null default 'auto'   -- 'auto' | 'confirm'（§10.5）
  created_at, updated_at

sessions                          -- ログインセッション。Cookie には乱数、DB にはそのハッシュ
  token_hash      bytea pk                       -- Cookie に入れた32バイト乱数の SHA-256
  user_id         uuid not null fk users on delete cascade
  expires_at      timestamptz not null           -- 発行から30日
  last_used_at    timestamptz not null           -- アクセスのたびに更新し、期限を延長する
  created_at

roadmaps                          -- template と personal を同一テーブルで扱う
  id                    uuid pk
  kind                  text not null    -- 'template' | 'personal'
  owner_user_id         uuid not null fk users
  name                  text not null
  description           text
  origin                text not null default 'manual'  -- 'manual'|'external'|'builtin'（§2）
  source                text             -- 出典 URL（origin='external' なら必須）
  checked_at            date             -- 出典の確認日（origin='external'/'builtin' なら必須）
  levels                jsonb not null   -- レベル1〜5の定義（§2）。5件必須。criteria は LLM プロンプトの正本
  visibility            text not null    -- 'private' | 'public'（template のみ public 可）
  version               int not null default 1
  forked_from_id        uuid fk roadmaps -- fork 元（null なら新規）
  forked_from_version   int
  target_date           date             -- personal のみ。目標日
  stars_count           int not null default 0   -- 集計キャッシュ
  created_at, updated_at

domains
  id            uuid pk
  roadmap_id    uuid not null fk roadmaps on delete cascade
  key           text not null
  name          text not null
  goal          text             -- この分野を修めると何ができるか（§2.1）
  order_index   int not null
  unique (roadmap_id, key)

items
  id                uuid pk
  roadmap_id        uuid not null fk roadmaps on delete cascade
  domain_id         uuid not null fk domains on delete cascade
  key               text not null
  name              text not null
  description       text
  outcome           text             -- 身につくと何ができるか（§2.1）
  outcome_source    text             -- 'authored' | 'ai_draft'（AI 下書きは編集済みなら 'authored' へ）
  verify_by         text             -- 次の確認方法
  depends_on_keys   text[] not null default '{}'
  order_index       int not null
  unique (roadmap_id, key)

stars                              -- v2
  user_id       uuid fk users
  roadmap_id    uuid fk roadmaps
  created_at
  primary key (user_id, roadmap_id)

learning_logs
  id            uuid pk
  user_id       uuid not null fk users
  roadmap_id    uuid not null fk roadmaps    -- kind='personal'
  body          text not null
  logged_at     date not null                -- 学習した日（投稿日と別）
  created_at

llm_jobs                           -- 非同期キュー。LLM を使う仕事はすべてここに積む
  id            uuid pk
  type          text not null      -- 'judgment'（ログの判定）| 'outcome_draft'（到達状態の下書き）
  user_id       uuid not null fk users
  log_id        uuid fk learning_logs   -- type='judgment' のとき必須
  roadmap_id    uuid fk roadmaps        -- type='outcome_draft' のとき必須
  status        text not null      -- 'queued'|'running'|'succeeded'|'failed'
  attempts      int not null default 0
  model         text not null      -- 使用したモデル ID
  error         text
  started_at, finished_at, created_at

llm_responses                      -- LLM の生レスポンス。検証前も保存する
  id            uuid pk
  job_id        uuid not null fk llm_jobs
  raw           jsonb                         -- JSON として読めた出力。読めなければ NULL
  raw_text      text                          -- JSON として読めなかった生の文字列（「承知しました…」等）
  violations    jsonb not null default '[]'   -- 検証で弾いた内容
  input_tokens  int
  output_tokens int
  created_at
  -- raw と raw_text のどちらか片方は必ず埋まる（制約 llm_responses_has_payload）。
  -- 実装は migrations/000004_llm_response_raw_text.up.sql。SPEC への反映は 2026-09-23 の書き換えで吸収した

assessment_events                  -- 理解度の一次データ（イベントソーシング）
  id              uuid pk
  user_id         uuid not null fk users
  item_id         uuid not null fk items
  log_id          uuid fk learning_logs      -- 手動上書き時は null
  source          text not null              -- 'ai' | 'manual'
  proposed_level  int                        -- LLM が提案した値（クランプ前）
  applied_level   int not null               -- 検証後に適用した値
  pre_state       text not null
  evidence_type   text not null
  rationale       text not null
  confidence      real
  occurred_at     timestamptz not null
  created_at

item_states                        -- 導出値。assessment_events から再計算できる
  item_id           uuid pk fk items
  user_id           uuid not null fk users
  level             int not null default 0
  pre_state         text not null default 'none'
  needs_review      bool not null default false   -- 降格提案があったときに立つ
  last_evidence_at  timestamptz
  last_event_id     uuid fk assessment_events
  updated_at

judgment_quotas                    -- レート制限
  user_id       uuid fk users
  period        text               -- 'YYYY-MM'
  used          int not null default 0
  primary key (user_id, period)
```

#### 設計の要点

- **`assessment_events` が一次データ、`item_states` は導出値。** v1 の `judgments/` と `state.json`（§3.2）と同じ関係
- **fork は行の複製。** テンプレートの `roadmaps` / `domains` / `items` をコピーして `kind='personal'` の新しい木を作り、`forked_from_id` と `forked_from_version` を記録する。上流が更新されても進捗は無傷
- **publish は逆向きの複製。** personal から `kind='template'`, `visibility='public'` の木を作る
- `stars_count` は集計キャッシュ。正は `stars` テーブル
- **`levels` はロードマップごとに持ち、jsonb に丸ごと入れる。** レベルの意味は分野によって書き分けたくなるためアプリ共通の固定値にしない。常に1〜5の5件セットでしか読み書きしないので、行に割っても JOIN が増えるだけで得がない
- **セッションはトークンそのものを保存しない。** Cookie に入れるのは32バイトの乱数で、DB に置くのはその SHA-256 だけ。ログアウトは行の削除なので即座に効く。ユーザー削除で cascade する

### 10.2 非同期ジョブ

```
POST /api/roadmaps/:id/logs
  → learning_logs に保存
  → llm_jobs に queued で登録
  → 202 Accepted { logId, jobId } を即返す

ワーカー（別プロセス。cmd/worker）
  → queued を1件取得（SELECT ... FOR UPDATE SKIP LOCKED）
  → プロンプト組み立て → Claude API 呼び出し
  → 検証（形の検査 → V1〜V8）
  → llm_responses に生レスポンスと violations を保存
  → assessment_events を INSERT → item_states を更新（1トランザクション）
  → status を succeeded/failed に

フロントは GET /api/jobs/:id をポーリング
```

実装の順は「検証 → 保存 → 反映」（`violations` を同じ行に入れるには先に検証が要る）。
再試行するのは `llm.ErrTemporary` が付いた失敗だけで、上限は `JUDGMENT_MAX_ATTEMPTS`。
**未解決**：`running` のまま取り残された仕事の回収（`started_at` が一定時間より古い `running` を `queued` に戻す方式が素直。`TODO.md` 3-2）。

同期処理にしない理由：判定に数秒〜十数秒かかる。投稿リクエスト内で待たせるとマルチユーザーで詰まる。
Cloud Run にデプロイする場合、リクエスト処理外の CPU が絞られるため、ワーカーは別サービスにするか CPU always-on を有効にする。

### 10.3 プロンプト構成（`internal/llm/prompt.go`）

**共通部を先頭に固める。** プロンプトキャッシュはプレフィックス一致なので、全ユーザーで共通の部分を先に置くと命中率が上がる。

| 位置 | 内容 | キャッシュ |
|---|---|---|
| 1. system（共通） | 判定の役割、5段階の `criteria`、`evidenceType` の許可リストと意味、昇格ルール、禁止事項 | 対象（全ユーザー共通） |
| 2. user（個別） | ロードマップの分野・項目一覧（key / name / description） | 対象外 |
| 3. user（個別） | 対象項目の現在レベル・preState・直近の根拠 | 対象外 |
| 4. user（個別） | 学習ログ本文 | 対象外 |

出力スキーマは system に書かず、`output_config.format`（構造化出力）で指定する。system には「JSON 以外を出力しない」とだけ書く。
Claude Sonnet 5 のキャッシュ最小プレフィックスは 1,024 トークン。共通部がこれ未満だと無言でキャッシュされない。
**この文面は v1 の `prompts/judge.md`（§4.2）へ流用する。**

### 10.4 モデル選択・コスト・stub・レート制限

- 既定は **Claude Sonnet 5**（`claude-sonnet-5`）。モデル ID は環境変数 `ANTHROPIC_MODEL` で切り替え可能にし、負荷が上がったら Haiku 4.5 に落とせるようにする
- API キーは**バックエンドのみ**が保持する（`ANTHROPIC_API_KEY`）。フロントには置かない
- **レート制限**：ユーザーあたり月 N 回（既定 100、環境変数で変更可）。超過時は 429 を返す。BYOK は v2 以降

参考見積もり（1判定 = 入力 約7,500 / 出力 約500 トークン、Sonnet 5 通常価格 $3 / $15 per MTok、$1 = 155円）：

| 規模 | 月間判定数 | 概算 |
|---|---|---|
| 自分ひとり（20件/月） | 20 | 約 90 円（Haiku 4.5 なら約 30 円） |
| 100人 × 20件/月 | 2,000 | 約 9,000 円（キャッシュ命中で約 5,000 円） |
| 1,000人 × 20件/月 | 20,000 | 約 90,000 円 |

Anthropic API に常設の無料枠はない。Claude Pro / Max のサブスクリプションは claude.ai と Claude Code のためのもので、自作アプリからの API 呼び出しには使えない
（**v1 が Claude Code / ChatGPT で判定する理由の一つ**。§4.6）。1,000人規模では成立しないため、v2 で BYOK かレート制限の強化が必要になる。

**stub モード**：環境変数 `LLM_PROVIDER=stub` で、LLM を呼ばず**同じ入力には同じ判定を返す**実装に差し替える（開発中の課金ゼロ・テストの決定性）。
stub の判定は、渡されたロードマップの先頭から、まだ最大レベルでない項目を 3 件まで選び、それぞれ「現在レベル + 1」を提案する
（確信度は 0.9 固定）。学習ログの本文は読まない。テストでは、返す JSON を丸ごと指定した stub で「行儀の悪い LLM」を演じさせる。
stub の出力も本物と同じ読み取り処理（§4.5 の形の検査）を通す。
Anthropic の Console で**支出上限（spend limit）を設定しておく**こと。

### 10.5 判定結果の反映モード

**既定は自動反映。** ユーザー設定（`users.apply_mode`）で「確認してから反映」に切り替えられる。

| `apply_mode` | 動き |
|---|---|
| `auto`（既定） | 検証を通った判定を即座に `assessment_events` へ記録し `item_states` を更新する。結果は画面に表示され、その場で上書きできる |
| `confirm` | 検証までは同じだが `item_states` を更新せず「保留」として提示する。ユーザーが採否を決めてから確定する |

`confirm` の保留分は、**判定結果は `llm_responses` に残したまま `assessment_events` を作らない**ことで表現する。
承認された時点で初めてイベントを積む。V7（確信度が閾値未満）で保留になったものは、`apply_mode` に関係なく常に `confirm` と同じ扱いになる。
既定を `auto` にした理由：このアプリの価値は「ログを書くだけで済むこと」にある。v1 ではこれを `git diff` での確認（§4.7）で置き換えた。

### 10.6 到達状態（`outcome`）の AI 下書きジョブ

`outcome` が空の項目に対して、**まとめて下書きを生成する**（`llm_jobs.type='outcome_draft'`）。1ロードマップにつき1ジョブ。

- 入力：分野名・`goal`、各項目の `key` / `name` / `description` / `dependsOn`、レベル定義
- 出力：`[{ "itemKey": "...", "outcome": "..." }]`（構造化出力）
- 生成したものは `items.outcome_source='ai_draft'` として保存し、**画面上で下書きと明示する**。人が編集したら `'authored'` に変える
- 検証ルール：`itemKey` の存在確認、1項目 200 文字以内、空文字は棄却。違反は `llm_responses.violations` に残す
- コストは小さい（1ロードマップ = 1回、出力2,000トークン程度）。判定回数のクォータとは別に数える

### 10.7 API

すべて `/api` 配下。認証は HttpOnly / Secure / SameSite=Lax の Cookie セッション。

| メソッド | パス | 内容 | 実装 |
|---|---|---|---|
| GET | `/api/auth/github` | GitHub OAuth 開始 | 済 |
| GET | `/api/auth/github/callback` | コールバック。セッション発行 | 済 |
| POST | `/api/auth/logout` | セッション破棄 | 済 |
| GET | `/api/me` | ログイン中のユーザー（`applyMode` を含む） | 済 |
| PATCH | `/api/me/settings` | 判定の反映モード（`auto` / `confirm`）の切り替え | 未 |
| GET | `/api/roadmaps` | 自分の personal ロードマップ一覧 | 済 |
| POST | `/api/roadmaps/import` | マスタ JSON をインポートして personal を作る | 済 |
| GET | `/api/roadmaps/:id` | ロードマップ本体（分野・項目） | 済 |
| PATCH | `/api/roadmaps/:id` | 名前・目標日の更新 | 済 |
| DELETE | `/api/roadmaps/:id` | 削除 | 済 |
| GET | `/api/roadmaps/:id/matrix` | マトリクス描画用の集計済みデータ | 未 |
| GET | `/api/roadmaps/:id/next-actions?limit=5` | 次にやること Top N（`outcome` / `verifyBy` を含む） | 未 |
| GET | `/api/roadmaps/:id/path?domain=go` | 学習パス（§7.2）。他分野の依存先は `domainKey` 付きで含める | 未 |
| POST | `/api/roadmaps/:id/outcome-draft` | `outcome` が空の項目に AI 下書きを生成。**202** `{ jobId }` | 未 |
| POST | `/api/roadmaps/:id/logs` | 学習ログ投稿。**202** `{ logId, jobId }` | 未 |
| GET | `/api/roadmaps/:id/logs` | ログ一覧 | 未 |
| GET | `/api/logs/:id` | ログ1件＋その判定結果 | 未 |
| GET | `/api/jobs/:id` | 判定ジョブの状態（ポーリング用）。保留中の判定があればここに含める | 未 |
| POST | `/api/jobs/:id/apply` | 保留中の判定の採否を確定する（`confirm` モード・V7 保留分） | 未 |
| GET | `/api/items/:id/events` | 項目のレベル遷移履歴と根拠 | 未 |
| PATCH | `/api/items/:id/state` | レベルの手動上書き（`source='manual'` のイベントを積む） | 未 |
| PATCH | `/api/items/:id` | `outcome` / `verifyBy` の編集（編集したら `outcome_source='authored'`） | 未 |

v2 以降：`GET /api/templates`（公開一覧・検索）、`POST /api/templates/:id/star`、`DELETE /api/templates/:id/star`、`POST /api/templates/:id/fork`、`POST /api/roadmaps/:id/publish`。

エラー形式：`{ "error": { "code": "quota_exceeded", "message": "今月の判定回数の上限に達しました" } }`

#### インポート（`POST /api/roadmaps/import`）

リクエストボディは §2 のマスタ JSON **そのもの**（何かで包まない）。上限は 2 MiB。
検査（§2）は問題を全部集めて返し、**エラーが1件でもあれば保存しない。警告だけなら保存する。**

| 状況 | ステータス | 応答 |
|---|---|---|
| 保存した | 201 | `{ "id": "<roadmaps.id>", "issues": [警告のみ] }`。警告が無ければ `"issues": []` |
| 検査エラーあり | 400 | `{ "error": { "code": "invalid_roadmap", "message": "..." }, "issues": [エラーと警告の全部] }` |
| JSON として読めない | 400 | 同上（`issues[0].code` が `invalid_json` / `unknown_field`） |
| ボディが上限超え | 413 | `{ "error": { "code": "payload_too_large", "message": "..." } }` |

`issues[]` の1件は `{ "severity": "error" | "warning", "code": "...", "path": "domains[0].items[2].key", "message": "..." }`。
`code` の一覧は `backend/internal/roadmap/validate.go` の `IssueCode`。フロントは `message` ではなく `code` と `path` で分岐する。
インポートで作られるロードマップは常に `kind='personal'`、`visibility='private'`。
`path` は問題の場所が特定できるときだけ付く。`invalid_json` などファイル全体の問題では `path` キー自体が無い。

#### 自分のロードマップの CRUD（`/api/roadmaps`）

対象は自分の `kind='personal'` だけ。**他人のもの・存在しないものはどちらも 404 `not_found`**
（403 にすると「その id は存在する」と教えてしまう）。`:id` が uuid の形でない場合も 404。

| メソッド | 応答 |
|---|---|
| `GET /api/roadmaps` | 200 `{ "roadmaps": [見出し, ...] }`。最近更新した順。0件なら `[]` |
| `GET /api/roadmaps/:id` | 200 見出し ＋ `levels`（§2 の形）＋ `domains: [{ id, key, name, goal, items: [{ id, key, name, description, outcome, outcomeSource, verifyBy, dependsOn }] }]`。分野・項目は JSON に書いた順、`dependsOn` は key の配列（無ければ `[]`） |
| `PATCH /api/roadmaps/:id` | ボディ `{ "name"?: string, "targetDate"?: "YYYY-MM-DD" \| null }`。**キーが無ければ触らない、`null` なら目標日を消す。** 定義にないキー・空の名前（前後の空白を除いて空）・形の違う日付・更新項目なしは 400。名前は前後の空白を除いて保存する。ボディ上限 16 KiB（超えると 413）。200 で更新後の見出し |
| `DELETE /api/roadmaps/:id` | 204。分野・項目・学習ログ・判定は FK の cascade で消える |

見出し：`{ id, kind, name, description, origin, source, checkedAt, targetDate, itemCount, createdAt, updatedAt }`。
日付（`checkedAt` / `targetDate`）は `YYYY-MM-DD` か `null`、日時（`createdAt` / `updatedAt`）は RFC 3339。
未設定の文字列は `null` ではなく `""`（フロントの分岐を減らす）。

#### 画面（サーバ版で追加になるもの）

ログイン（GitHub でログインするだけ）／ログ投稿（テキストエリア＋学習日。投稿後は判定中インジケータ → 判定結果を表示し、その場で採否・上書き）／
ログ一覧（過去のログと、それがどの項目をどう動かしたか）／ロードマップ管理（一覧、JSON インポート、目標日の設定、`outcome` の AI 下書き生成）／
項目詳細への編集機能（到達状態の編集、手動上書き）。フロントの API クライアントは TanStack Query。

### 10.8 環境変数（サーバ版）

`backend/.env.example` に定義。`.env` はコミットしない。

```
APP_ENV=development            # development | production（development のみ Cookie の Secure を落とす）
PORT=8080
FRONTEND_ORIGIN                # CORS の許可とログイン後の戻り先
DATABASE_URL
SESSION_SECRET                 # 32文字以上。OAuth の state Cookie の署名に使う
GITHUB_OAUTH_CLIENT_ID
GITHUB_OAUTH_CLIENT_SECRET
GITHUB_OAUTH_CALLBACK_URL
LLM_PROVIDER=anthropic          # anthropic | stub（§10.4）
ANTHROPIC_API_KEY
ANTHROPIC_MODEL=claude-sonnet-5
JUDGMENT_MONTHLY_QUOTA=100
JUDGMENT_CONFIDENCE_THRESHOLD=0.5
JUDGMENT_MAX_ITEMS_PER_LOG=20
JUDGMENT_MAX_ATTEMPTS=3         # ワーカーの再試行上限（1〜10）
JUDGMENT_POLL_INTERVAL_SECONDS=5   # ワーカーがキューを見に行く間隔（1〜300）
```

ローカル開発は Docker Compose（Postgres + backend + worker + frontend）。ルータは chi。DB アクセスは標準 `database/sql` + `pgx`。
`vite.config.ts` に `server: { port: 5173, strictPort: true }` を入れる（`FRONTEND_ORIGIN` と食い違わないため）。

### 10.9 セキュリティ（サーバ版）

- **`ANTHROPIC_API_KEY` はバックエンドのみ。** フロントに渡さない
- **セッション Cookie は HttpOnly / SameSite=Lax、本番のみ Secure。** 有効期限は30日で、アクセスのたびに延長する
- **GitHub のアクセストークンは保存しない。** ログイン時に `/user` を1回呼ぶためだけに使い、捨てる
- **OAuth の `state` は署名付きの短命 Cookie で持ち、コールバックで一致を検証する。** 検証に失敗したリクエストは処理しない
- **学習ログの本文は、いかなる共有機能でも外部に出さない。** 共有対象は集計後のマトリクス（レベル）のみ
- 進捗は既定で本人のみ閲覧可。フレンドへの公開は opt-in、粒度も選択可（分野レベルのみ / 詳細項目まで）— v4
- 退会時に学習ログと判定イベントを完全削除できること
- 公開時はプライバシーポリシーに「学習ログをサーバに送信し、Anthropic の API に渡すこと」を明記する
