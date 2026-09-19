# 確定仕様

**実装が参照する正本。** ユーザーの確認を経たものだけを書く。検討中のことは `PLAN.md` に置く。

人間向けの解説は [`docs/spec-guide.md`](docs/spec-guide.md) にある。
**この文書と同じ事実を二重に書かないこと。** 解説側は「なぜそうしたか」と具体例を担当する。

---

## 0. スコープ

段階を分けて作る。**v1 は一人でも価値が出る範囲に絞る。**

| 段階 | 中身 |
|---|---|
| **v1** | GitHub ログイン / ロードマップの JSON インポート・fork / 学習ログ投稿 → AI 判定（非同期）/ 理解度マトリクス表示 / **学習パス表示** / 次にやること Top N / 判定の手動上書き / **到達状態の AI 下書き** |
| v2 | 公開ロードマップの一覧・検索・star・fork / 自分のロードマップの publish / ロードマップ GUI エディタ |
| v3 | SNS シェア（マトリクス画像・公開プロフィールページ） |
| v4 | フレンド機能（進捗の相互閲覧、opt-in） |

**v1 でやらないこと**：GUI でのロードマップ作成、公開・共有、クイズ形式の理解度測定、学習時間の自動計測、モバイルアプリ、上流ロードマップの変更取り込み（fork 後の `git pull` 相当）。

**`~/workspace/study/learner-profile/skill-map.md` は今まで通り手動運用の正本。** skill-matrix は独立したアプリとして作り、移行は行わない。将来アプリ側へ移す判断は、出来を見てから別途行う。

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

### 1.2 段階前の状態（`preState`）

レベルとは別軸のフラグ。**「レベル 0.5」ではなく「根拠の種類がまだ弱い」ことを表す。**

| preState | 意味 |
|---|---|
| `none` | 特記なし |
| `learning` | 未確認（学習中）。学習は始めたが習熟度を判定できる根拠がない |
| `explained_only` | 説明済み・理解未確認。説明を受けて質問は出なかったが確認していない |
| `self_reported` | 実務経験あり・横断評価未実施。自己申告のみ |

`preState` は level 0 のときだけでなく、どのレベルでも併存しうる。

### 1.3 時間経過による減衰

**入れない。** スコアを自動で下げない。代わりに **`lastEvidenceAt`（最終根拠日）** を持ち、経過日数から鮮度を導出する。

| staleness | 条件（既定値。設定ファイルで変更可） |
|---|---|
| `fresh` | 30日以内 |
| `aging` | 31〜90日 |
| `stale` | 91日以上 |

理由：自動で下がった値は「なぜ下がったか」を説明できない。既存モデルの「昇格には根拠が必要」という思想とも合わない。減衰式は後から純粋関数として追加できる。

---

## 2. ロードマップ・マスタ JSON

インポート・エクスポートの正本フォーマット。**コードに直書きしない。**

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
          "key": "go-01",
          "name": "基本構文（変数・型・関数・struct・slice/map）",
          "description": "Java脳との差分（ポインタ・ゼロ値）を含む",
          "outcome": "Go のコードを読んで型と値の流れを追え、Java との差分を説明できる",
          "dependsOn": [],
          "verifyBy": "Tour Basics 完了後にドリル"
        },
        {
          "key": "go-02",
          "name": "メソッド・インターフェース・埋め込み",
          "outcome": "型に振る舞いを持たせ、インターフェースで実装を差し替えられる",
          "dependsOn": ["go-01"]
        }
      ]
    }
  ]
}
```

制約：

- `key` は同一ロードマップ内で一意。`^[a-z0-9][a-z0-9-]{0,63}$`
- `dependsOn` は同一ロードマップ内の `item.key` のみ参照可。**循環参照はインポート時に弾く**
- `levels` は 1〜5 を必ず全て含む。`criteria` は**そのまま LLM のプロンプトに埋め込まれる**ため、判定基準の正本はここ1か所
- 上限：domains 50、1 domain あたり items 100、1 ロードマップあたり items 500
- **1フィールドの文字数の上限**（バイト数ではなく文字数。日本語1文字＝1）：`name`（ロードマップ・分野・項目・レベル）200、
  `description` / `outcome` / `goal` / `criteria` / `verifyBy` 2,000、`source` 2,048。
  `criteria` / `outcome` / `goal` / `verifyBy` は LLM のプロンプトに載るので、上限が無いと判定1回の課金に直結する。
  超えたらエラー（`too_long`）。定数は `backend/internal/roadmap/schema.go`
- **定義にないフィールドはエラーにして弾く。** `dependsOn` を `dependOn` と打ち間違えたときに、黙って依存関係が消えて学習パスの並び順だけが静かに狂う、という壊れ方を避けるため
- **検査は1件目で打ち切らず、問題を全部集めて返す。** エラー（インポート中止）と警告（インポートは通す）を分け、`domains[0].items[2].key` の形で場所を添える。実装は `backend/internal/roadmap/`

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
| `manual` | **任意。** 無くてよい。§4.8 の AI 下書きを案内する |
| `external` | **推奨。** 無い項目があればインポート時に警告し、AI 下書きを提案する（インポート自体は通す） |
| `builtin` | **必須。** アプリ同梱なので用意する側が責任を持つ |

理由：**初学者は自分のロードマップの到達状態を自分では書けない。**
「これを学ぶと何ができるようになるか」が分かっているなら、そもそも先が見えていないという課題が存在しない。
だから `outcome` は「外部のロードマップに書いてあるもの」か「AI が下書きしたもの」を主な供給源とし、
自作ロードマップに書くことを強制しない。

---

## 3. データモデル（PostgreSQL）

### 3.1 テーブル

```
users
  id              uuid pk
  github_id       bigint unique not null
  login           text not null
  avatar_url      text
  apply_mode      text not null default 'auto'   -- 'auto' | 'confirm'（§4.7）
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
  raw           jsonb not null
  violations    jsonb not null default '[]'   -- 検証で弾いた内容
  input_tokens  int
  output_tokens int
  created_at

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

### 3.2 設計の要点

- **`assessment_events` が一次データ、`item_states` は導出値。** 算出ロジックを変えても再計算できる。「なぜこの升目が濃いのか」を根拠つきで説明できる
- **fork は行の複製。** テンプレートの `roadmaps` / `domains` / `items` をコピーして `kind='personal'` の新しい木を作り、`forked_from_id` と `forked_from_version` を記録する。上流が更新されても進捗は無傷
- **publish は逆向きの複製。** personal から `kind='template'`, `visibility='public'` の木を作る（v2）
- `stars_count` は集計キャッシュ。正は `stars` テーブル
- **`levels` はロードマップごとに持ち、jsonb に丸ごと入れる。** レベルの意味は分野によって書き分けたくなる（Go の実装と英語学習では「ガイド付き実装」が同じ意味にならない）ためアプリ共通の固定値にしない。常に1〜5の5件セットでしか読み書きしないので、行に割っても JOIN が増えるだけで得がない
- **セッションはトークンそのものを保存しない。** Cookie に入れるのは32バイトの乱数で、DB に置くのはその SHA-256 だけ。DB が漏れても、その値をそのまま Cookie に入れてログインすることはできない。ログアウトは行の削除なので即座に効く。ユーザー削除で cascade する（§9 の退会時削除）

---

## 4. AI 判定

### 4.1 実行フロー

```
POST /api/roadmaps/:id/logs
  → learning_logs に保存
  → llm_jobs に queued で登録
  → 202 Accepted { logId, jobId } を即返す

ワーカー（別プロセス）
  → queued を1件取得（SELECT ... FOR UPDATE SKIP LOCKED）
  → プロンプト組み立て → Claude API 呼び出し
  → llm_responses に生レスポンスを保存
  → 検証 → assessment_events を INSERT → item_states を更新
  → status を succeeded/failed に

フロントは GET /api/jobs/:id をポーリング
```

**同期処理にしない理由**：判定に数秒〜十数秒かかる。投稿リクエスト内で待たせるとマルチユーザーで詰まる。後から非同期化するのは大きな作り直しになる。

Cloud Run にデプロイする場合、**リクエスト処理外の CPU が絞られる**ため、ワーカーは別サービスにするか CPU always-on を有効にする。

### 4.2 プロンプト構成

**共通部を先頭に固める。** プロンプトキャッシュはプレフィックス一致なので、全ユーザーで共通の部分を先に置くと命中率が上がる。

| 位置 | 内容 | キャッシュ |
|---|---|---|
| 1. system（共通） | 判定の役割、5段階の `criteria`、`evidenceType` の許可リストと意味、昇格ルール、出力スキーマ、禁止事項 | 対象（全ユーザー共通） |
| 2. user（個別） | ロードマップの分野・項目一覧（key / name / description） | 対象外 |
| 3. user（個別） | 対象項目の現在レベル・preState・直近の根拠 | 対象外 |
| 4. user（個別） | 学習ログ本文 | 対象外 |

注意：Claude Sonnet 5 のキャッシュ最小プレフィックスは 1,024 トークン。共通部がこれ未満だと無言でキャッシュされない。

### 4.3 出力スキーマ（構造化出力）

`output_config.format` で JSON Schema を指定する。

```jsonc
{
  "judgments": [
    {
      "itemKey": "go-01",
      "proposedLevel": 1,
      "evidenceType": "self_explanation",
      "rationale": "for/if/switch の挙動を自分の言葉で説明し、naked return の可読性の問題にも触れている",
      "confidence": 0.8
    }
  ],
  "unmatched": ["ロードマップのどの項目にも対応づけられなかった記述の要約"]
}
```

### 4.4 evidenceType の許可リスト

| evidenceType | 意味 | 昇格の可否 |
|---|---|---|
| `drill` | ドリル・確認質問への回答 | L1 まで昇格可 |
| `self_explanation` | 自分の言葉での説明 | L2 まで昇格可 |
| `implementation` | 実装した | L3 まで昇格可 |
| `unaided_implementation` | ガイドなしの実装・レビュー実績 | L4 まで昇格可 |
| `cross_context` | 学んだ文脈と別の場面で使った | L5 まで昇格可 |
| `explained_to` | 説明を受けた | **昇格させない。** `preState='explained_only'` にするのみ |
| `self_report` | 自己申告 | **昇格させない。** `preState='self_reported'` にするのみ |

`explained_to` と `self_report` を昇格させないのは、既存モデルの「説明済み ≠ 理解」「根拠のない昇格をしない」を機械化したもの。

### 4.5 検証ルール（純粋関数）

**握りつぶし禁止。** 弾いた内容はすべて `llm_responses.violations` に記録する。

| ID | ルール | 違反時の扱い |
|---|---|---|
| V1 | `itemKey` がそのロードマップに存在する | 棄却して記録 |
| V2 | `proposedLevel` が 0〜5 の整数 | 棄却して記録 |
| V3 | `evidenceType` が許可リストにある | 棄却して記録 |
| V4 | 昇格幅は現在レベル +1 まで | +1 にクランプして記録（一気飛び禁止） |
| V5 | `evidenceType` がそのレベルの昇格に足りている（4.4 の表） | 到達可能な上限にクランプして記録 |
| V6 | `proposedLevel` が現在レベル未満（降格提案） | **レベルは下げず** `needs_review=true` を立てて記録 |
| V7 | `confidence` が閾値（既定 0.5）未満 | 適用せず「レビュー待ち」として記録 |
| V8 | 1ログあたりの判定件数が上限（既定 20）以内 | 超過分を棄却して記録 |

**V1〜V8 の前に、形の検査を通す**（`internal/llm` の `ParseOutput`。V1〜V8 は意味の検査で `internal/domain` が担当し、同じ検査を2か所に書かない）。
形の崩れた判定はその1件だけを棄却し、同じログの他の判定は生かす。棄却した判定も V1〜V8 の違反と同じく `llm_responses.violations` に記録する。
出力全体が JSON として読めない・`judgments` が無い場合は、判定ジョブの失敗として扱う。

| 形の検査 | 違反時の扱い |
|---|---|
| 型が合っている（`proposedLevel` が整数、など） | 棄却して記録 |
| 必須の欄（`itemKey` / `proposedLevel` / `evidenceType` / `rationale` / `confidence`）が揃っている | 棄却して記録 |
| `rationale` が空でない | 棄却して記録 |
| `confidence` が 0〜1 の範囲内（範囲外は `assessment_events.confidence` の CHECK 制約にも反する） | 棄却して記録 |

**配点の分配はしない。** 1つのログが複数項目にまたがる場合も、項目ごとに独立して判定させる。合計制約は設けない（LLM が苦手で、意味もない）。

### 4.6 モデル選択とコスト

- 既定は **Claude Sonnet 5**（`claude-sonnet-5`）。モデル ID は環境変数 `ANTHROPIC_MODEL` で切り替え可能にし、負荷が上がったら Haiku 4.5 に落とせるようにする
- API キーは**バックエンドのみ**が保持する（`ANTHROPIC_API_KEY`）。フロントには置かない
- **レート制限**：ユーザーあたり月 N 回（既定 100、環境変数で変更可）。超過時は 429 を返す。BYOK は v2 以降

参考見積もり（1判定 = 入力 約7,500 / 出力 約500 トークン、Sonnet 5 通常価格 $3 / $15 per MTok、$1 = 155円）：

| 規模 | 月間判定数 | 概算 |
|---|---|---|
| **自分ひとり（20件/月）** | 20 | **約 90 円**（Haiku 4.5 なら約 30 円） |
| 100人 × 20件/月 | 2,000 | 約 9,000 円（キャッシュ命中で約 5,000 円） |
| 1,000人 × 20件/月 | 20,000 | 約 90,000 円 |

**Anthropic API に常設の無料枠はない。** Claude Pro / Max のサブスクリプションは claude.ai と Claude Code
のためのもので、自作アプリからの API 呼び出しには使えない。ただし自分ひとりの利用なら月100円弱で、
実質的に無視できる額。1,000人規模では成立しないため、v2 で BYOK かレート制限の強化が必要になる。

**開発中のコスト対策として、LLM を呼ばない stub モードを用意する。**
環境変数 `LLM_PROVIDER=stub` で、LLM を呼ばず**同じ入力には同じ判定を返す**実装に差し替える。理由は2つ。

- 開発・テスト中の呼び出し回数は本番より多くなりやすく、そちらのほうが高くつく
- 同じ入力なら判定結果が変わらないので、検証ルール V1〜V8 と集計処理のテストが決定的になる

stub の判定は、渡されたロードマップの先頭から、まだ最大レベルでない項目を 3 件まで選び、それぞれ「現在レベル + 1」を提案する
（確信度は閾値を上回る固定値）。文面を完全に固定しないのは、項目の `key` がロードマップごとに違い、固定の `key` では全件が V1 で弾かれるため。
学習ログの本文は読まないので、開発環境で同じログを繰り返し投稿するとマトリクスは先頭から順に埋まる。
テストでは、返す JSON を丸ごと指定した stub で「行儀の悪い LLM」を演じさせる。stub の出力も本物と同じ読み取り処理（4.5 の形の検査）を通す。

Anthropic の Console で**支出上限（spend limit）を設定しておく**こと。事故で青天井にならないようにする。

### 4.7 判定結果の反映モード

**既定は自動反映。** ユーザー設定（`users.apply_mode`）で「確認してから反映」に切り替えられる。

| `apply_mode` | 動き |
|---|---|
| `auto`（既定） | 検証を通った判定を即座に `assessment_events` へ記録し `item_states` を更新する。結果は画面に表示され、その場で上書きできる |
| `confirm` | 検証までは同じだが `item_states` を更新せず「保留」として提示する。ユーザーが採否を決めてから確定する |

`confirm` の保留分は `assessment_events` に `applied_level` を書かずに持つのではなく、
**判定結果は `llm_responses` に残したまま `assessment_events` を作らない**ことで表現する。
承認された時点で初めてイベントを積む。イベントテーブルは「確定したできごと」だけを持つ、という原則を崩さない。

V7（確信度が閾値未満）で保留になったものは、`apply_mode` に関係なく常に `confirm` と同じ扱いになる。

既定を `auto` にした理由：このアプリの価値は「ログを書くだけで済むこと」にある。
毎回の確認を必須にすると、手動でチェックボックスを付けるのと手間が変わらなくなる。

### 4.8 到達状態（`outcome`）の AI 下書き

`outcome` が空の項目に対して、**まとめて下書きを生成する**（`llm_jobs.type='outcome_draft'`）。
判定と同じ非同期キューに乗せる。1ロードマップにつき1ジョブで、項目ごとには分けない。

- 入力：分野名・`goal`、各項目の `key` / `name` / `description` / `dependsOn`、レベル定義
- 出力：`[{ "itemKey": "...", "outcome": "..." }]`（構造化出力）
- 生成したものは `items.outcome_source='ai_draft'` として保存し、**画面上で下書きと明示する**
- 人が編集したら `outcome_source='authored'` に変える

検証ルール：`itemKey` の存在確認、1項目 200 文字以内、空文字は棄却。違反は `llm_responses.violations` に残す。

**必ず人が手直しできる形にする。** AI が書いた到達状態が的外れなまま「先が見える」画面に出ると、
間違った地図を渡すことになり、当初の課題（先が見えない不安）をかえって悪化させる。

コストは小さい（1ロードマップ = 1回、出力2,000トークン程度）。判定回数のクォータとは別に数える。

---

## 5. スコア集計（純粋関数）

`backend/internal/domain/` に置く。**DB・HTTP・LLM クライアントを import しない。** 単体テスト必須。

```go
// 判定1件を検証して現在の状態に適用する（V2〜V7）。違反は握りつぶさず返す。
func ApplyJudgment(cur ItemState, j Judgment, rules Rules) Applied

// 1ログ分の判定をまとめて適用する（V1・V8 を追加で見る）。
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

重みは設定ファイルに切り出す（ハードコード禁止）。並び順は
**「着手不能なものを常に後ろ」→「priority の降順」→「定義順」** で安定させる。

> **`urgency`（目標日までの逼迫度）を式から外した。**
> 逼迫度はロードマップ全体で1つの値になるため、全項目に同じ数を足すことになり**順位が一切変わらない**。
> 優先度に混ぜても意味がないので、`BuildSchedule` が返す独立した値として画面に出す
> （「目標日まで残り30日、未着手22項目、1日あたり0.7項目のペースが必要」のように表示する）。
> 代わりに、**`unlocks`（この項目を終えると何項目が着手可能になるか）** を入れた。
> こちらは項目ごとに値が違うので、実際に順位を動かす。

`Action` には `item.verifyBy`（次の確認方法）と `item.outcome`（身につくと何ができるか）をそのまま載せる。
**「次に何をやるか」だけでなく「やると何ができるようになるか」を毎回同時に見せる**ため。

---

## 6. API

すべて `/api` 配下。認証は HttpOnly / Secure / SameSite=Lax の Cookie セッション。

### v1

| メソッド | パス | 内容 |
|---|---|---|
| GET | `/api/auth/github` | GitHub OAuth 開始 |
| GET | `/api/auth/github/callback` | コールバック。セッション発行 |
| POST | `/api/auth/logout` | セッション破棄 |
| GET | `/api/me` | ログイン中のユーザー（`applyMode` を含む） |
| PATCH | `/api/me/settings` | 判定の反映モード（`auto` / `confirm`）の切り替え |
| GET | `/api/roadmaps` | 自分の personal ロードマップ一覧 |
| POST | `/api/roadmaps/import` | マスタ JSON をインポートして personal を作る |
| GET | `/api/roadmaps/:id` | ロードマップ本体（分野・項目） |
| PATCH | `/api/roadmaps/:id` | 名前・目標日の更新 |
| DELETE | `/api/roadmaps/:id` | 削除 |
| GET | `/api/roadmaps/:id/matrix` | マトリクス描画用の集計済みデータ |
| GET | `/api/roadmaps/:id/next-actions?limit=5` | 次にやること Top N（`outcome` / `verifyBy` を含む） |
| GET | `/api/roadmaps/:id/path?domain=go` | 学習パス（§7.2）。依存順に並べた項目と `done`/`current`/`upcoming`、項目ごとの `dependsOn` と `ready`（依存がすべて L1 以上）。他分野の依存先は `domainKey` 付きで含める（ツリー表示のゴーストノード用） |
| POST | `/api/roadmaps/:id/outcome-draft` | `outcome` が空の項目に AI 下書きを生成。**202** `{ jobId }` |
| POST | `/api/roadmaps/:id/logs` | 学習ログ投稿。**202** `{ logId, jobId }` |
| GET | `/api/roadmaps/:id/logs` | ログ一覧 |
| GET | `/api/logs/:id` | ログ1件＋その判定結果 |
| GET | `/api/jobs/:id` | 判定ジョブの状態（ポーリング用）。保留中の判定があればここに含める |
| POST | `/api/jobs/:id/apply` | 保留中の判定の採否を確定する（`confirm` モード・V7 保留分） |
| GET | `/api/items/:id/events` | 項目のレベル遷移履歴と根拠 |
| PATCH | `/api/items/:id/state` | レベルの手動上書き（`source='manual'` のイベントを積む） |
| PATCH | `/api/items/:id` | `outcome` / `verifyBy` の編集（編集したら `outcome_source='authored'`） |

### v2 以降

`GET /api/templates`（公開一覧・検索）、`POST /api/templates/:id/star`、`DELETE /api/templates/:id/star`、`POST /api/templates/:id/fork`、`POST /api/roadmaps/:id/publish`。

### エラー形式

```json
{ "error": { "code": "quota_exceeded", "message": "今月の判定回数の上限に達しました" } }
```

### インポート（`POST /api/roadmaps/import`）

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

### 自分のロードマップの CRUD（`/api/roadmaps`）

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

---

## 7. 画面

| 画面 | 内容 |
|---|---|
| ログイン | GitHub でログインするだけ |
| ダッシュボード | サマリー帯（分野 × レベルの到達項目数）＋ 可変長グリッドのマトリクス ＋ 次にやること Top 5 |
| **学習パス** | **分野を選ぶと、項目が依存順に縦一列で並ぶ。済 / 今ここ / この先と到達状態を見せる（§7.2）** |
| ログ投稿 | テキストエリア＋学習日。投稿後は判定中インジケータ → 判定結果を表示し、その場で採否・上書きができる |
| ログ一覧 | 過去のログと、それがどの項目をどう動かしたか |
| 項目詳細 | レベル遷移の履歴、各遷移の根拠と出典ログ、次の確認方法、到達状態の編集、手動上書き |
| ロードマップ管理 | 一覧、JSON インポート、目標日の設定、`outcome` の AI 下書き生成 |

### 7.1 マトリクスの表示形式

**行 = 分野、列 = その分野の詳細項目の可変長グリッド。** 分野ごとに列数が違う。

```
Go           ■ ■ □ □ □ □        (6項目)
React / TS   ■ ■ □ □ □          (5項目)
Java/Spring  ■ ■ ■ ■ ■ ■ ■ ■    (8項目)
```

分野ごとに詳細項目が異なるため、共通の列軸を持つ真の行列にすると疎行列になる。上部に「分野 × レベルの到達項目数」のサマリー帯を置いて一覧性を補う。

**二重符号化**：升目の**色の濃さ = レベル（0〜5 の6階調）**、**枠線・斜線 = 要再確認**（`staleness` が `stale`、または `needs_review=true`）。`preState` はツールチップで表示する。

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
  入力は `/path` のレスポンス、出力は各ノードの `(段, 列)` と辺のリスト。DOM・API クライアントを import しない
- ノードの状態は4つ。色はマトリクス（§7.1）と同じ連続スケールを使う

  | 状態 | 条件 | 見た目 |
  |---|---|---|
  | 未解放 | `ready=false`（前提に L0 がある） | 灰色・鍵マーク。線も薄く |
  | 解放済み・未着手 | `ready=true` かつレベル0 | 枠を強調。`current` の項目は「今ここ」ラベル |
  | 習得済み | レベル1以上 | 塗りの濃さ = レベル（6階調）。`stale` / `needs_review` は枠線・斜線（§7.1 と同じ符号化） |
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
- `outcome` が空の項目は（一列表示では行を詰めて、ツリー表示ではノード内で）**下書き生成へのリンクを出す**（§4.8）
- 常に上部に `domain.goal` と進捗を出す。「あと何個でどうなるか」が視界から消えないようにする

---

## 8. 技術構成

### 8.1 ディレクトリ

```
backend/
  cmd/server/main.go        HTTP サーバ
  cmd/worker/main.go        判定ジョブのワーカー
  internal/
    domain/                 純粋関数（理解度モデル・集計・優先度）※ DB/HTTP/LLM を import しない
    roadmap/                マスタ JSON のスキーマ定義と検証
    llm/                    Claude API クライアント、プロンプト組み立て、出力検証
    store/                  PostgreSQL アクセス
    httpapi/                ハンドラ、認証・レート制限ミドルウェア
    config/                 環境変数・設定ファイルの読み込み
  migrations/               マイグレーション
  testdata/                 ダミーの学習ログ・ロードマップ
frontend/
  src/
    api/                    自前 API のクライアント（TanStack Query）
    features/matrix/        マトリクス
    features/path/          学習パス（一列表示・スキルツリー表示。layout.ts は純粋関数）
    features/logs/          ログ投稿・一覧
    features/roadmaps/      ロードマップ管理
    features/auth/
    components/
docs/                       要件定義・アーキテクチャの人間向け解説
```

### 8.2 スタック

`../CLAUDE.md` のデフォルトに従う。追加で確定したもの：

- **ローカル開発は Docker Compose**（Postgres + backend + worker + frontend）。SQLite は使わない（Postgres への移行が無駄になるため）
- ルータは chi。DB アクセスは標準 `database/sql` + `pgx`
- フロントは React Router / React Hook Form + Zod / TanStack Query / Recharts
- デプロイは後回し。決めるのは v1 が動いてから

### 8.3 環境変数

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
LLM_PROVIDER=anthropic          # anthropic | stub（stub は LLM を呼ばず、同じ入力には同じ判定を返す。§4.6）
ANTHROPIC_API_KEY
ANTHROPIC_MODEL=claude-sonnet-5
JUDGMENT_MONTHLY_QUOTA=100
JUDGMENT_CONFIDENCE_THRESHOLD=0.5
JUDGMENT_MAX_ITEMS_PER_LOG=20
```

---

## 9. セキュリティとプライバシー

- **`ANTHROPIC_API_KEY` はバックエンドのみ。** フロントに渡さない
- **セッション Cookie は HttpOnly / SameSite=Lax、本番のみ Secure。** 有効期限は30日で、アクセスのたびに延長する
- **GitHub のアクセストークンは保存しない。** ログイン時に `/user` を1回呼ぶためだけに使い、捨てる
- **OAuth の `state` は署名付きの短命 Cookie で持ち、コールバックで一致を検証する。** 検証に失敗したリクエストは処理しない
- **学習ログの本文は、いかなる共有機能でも外部に出さない。** ログには職場・実務・転職の話が混ざりうる。共有対象は集計後のマトリクス（レベル）のみ
- 進捗は既定で本人のみ閲覧可。フレンドへの公開は opt-in、粒度も選択可（分野レベルのみ / 詳細項目まで）— v4
- 退会時に学習ログと判定イベントを完全削除できること
- **リポジトリに実データをコミットしない。** シード・テストはすべて `backend/testdata/` のダミーログを使う
- 公開時はプライバシーポリシーに「学習ログをサーバに送信し、Anthropic の API に渡すこと」を明記する
