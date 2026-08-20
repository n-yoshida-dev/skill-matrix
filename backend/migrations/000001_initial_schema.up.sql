-- skill-matrix の初期スキーマ
--
-- 仕様は SPEC.md §3、人間向けの解説は docs/spec-guide.md §3.3。
-- 設計の要点は3つ。
--   1. テンプレートと自分用のロードマップを roadmaps 1テーブルで扱う（kind で区別）
--   2. 理解度の一次データは assessment_events（追記のみ）。item_states はそこから導ける値
--   3. LLM を使う仕事は llm_jobs に集約する（判定と到達状態の下書きの2種類）

-- updated_at をアプリ側の書き忘れに関係なく更新するための共通トリガ。
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------------------------
-- 利用者
-- ---------------------------------------------------------------------------
CREATE TABLE users (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    github_id  bigint NOT NULL UNIQUE,
    login      text NOT NULL,
    avatar_url text,
    -- 判定結果を自動で反映するか、確認してから反映するか（SPEC.md §4.7）
    apply_mode text NOT NULL DEFAULT 'auto' CHECK (apply_mode IN ('auto', 'confirm')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- ロードマップ（テンプレートと自分用を同じテーブルで扱う）
-- ---------------------------------------------------------------------------
CREATE TABLE roadmaps (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- template: 公開されて star される側 / personal: fork したコピー。進捗が紐づく
    kind                text NOT NULL CHECK (kind IN ('template', 'personal')),
    owner_user_id       uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name                text NOT NULL CHECK (length(name) > 0),
    description         text,
    -- manual: 利用者が自作 / external: 外部由来 / builtin: アプリ同梱（SPEC.md §2）
    origin              text NOT NULL DEFAULT 'manual'
                        CHECK (origin IN ('manual', 'external', 'builtin')),
    source              text,
    checked_at          date,
    visibility          text NOT NULL DEFAULT 'private'
                        CHECK (visibility IN ('private', 'public')),
    version             int NOT NULL DEFAULT 1 CHECK (version >= 1),
    -- fork 元。上流が更新されても進捗は無傷（行ごと複製しているため）
    forked_from_id      uuid REFERENCES roadmaps (id) ON DELETE SET NULL,
    forked_from_version int,
    target_date         date,
    stars_count         int NOT NULL DEFAULT 0 CHECK (stars_count >= 0),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),

    -- 外部由来のものだけ出典と確認日を必須にする。自作にまで求めない（SPEC.md §2）
    CONSTRAINT roadmaps_external_needs_source
        CHECK (origin <> 'external' OR (source IS NOT NULL AND checked_at IS NOT NULL)),
    -- 公開できるのはテンプレートだけ
    CONSTRAINT roadmaps_only_template_can_be_public
        CHECK (visibility = 'private' OR kind = 'template'),
    -- 目標日を持てるのは自分用のロードマップだけ
    CONSTRAINT roadmaps_target_date_is_personal_only
        CHECK (target_date IS NULL OR kind = 'personal')
);

CREATE TRIGGER roadmaps_set_updated_at BEFORE UPDATE ON roadmaps
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX roadmaps_owner_idx ON roadmaps (owner_user_id, kind);
-- v2 の公開ロードマップ一覧（人気順）用
CREATE INDEX roadmaps_public_idx ON roadmaps (stars_count DESC)
    WHERE kind = 'template' AND visibility = 'public';

-- ---------------------------------------------------------------------------
-- 分野と詳細項目
-- ---------------------------------------------------------------------------
CREATE TABLE domains (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    roadmap_id  uuid NOT NULL REFERENCES roadmaps (id) ON DELETE CASCADE,
    key         text NOT NULL,
    name        text NOT NULL,
    -- この分野を修めると何ができるようになるか（SPEC.md §2.1）
    goal        text,
    order_index int NOT NULL DEFAULT 0,

    UNIQUE (roadmap_id, key)
);

CREATE TABLE items (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    roadmap_id      uuid NOT NULL REFERENCES roadmaps (id) ON DELETE CASCADE,
    domain_id       uuid NOT NULL REFERENCES domains (id) ON DELETE CASCADE,
    key             text NOT NULL,
    name            text NOT NULL,
    description     text,
    -- 身につくと何ができるようになるか。必須にしない（SPEC.md §2.1）
    outcome         text,
    -- authored: 人が書いた / ai_draft: AI の下書き。画面で下書きと明示する
    outcome_source  text CHECK (outcome_source IN ('authored', 'ai_draft')),
    -- 次の確認方法
    verify_by       text,
    -- 先に着手すべき項目の key。学習パスの並び順に使う
    depends_on_keys text[] NOT NULL DEFAULT '{}',
    order_index     int NOT NULL DEFAULT 0,

    UNIQUE (roadmap_id, key),
    -- 到達状態があるなら、それが人の言葉か AI の下書きかを必ず記録する
    CONSTRAINT items_outcome_needs_source
        CHECK (outcome IS NULL OR outcome_source IS NOT NULL)
);

CREATE INDEX items_domain_idx ON items (domain_id, order_index);

-- v2 の star（ブックマーク）。fork と違い進捗とは無関係（SPEC.md §3.2）
CREATE TABLE stars (
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    roadmap_id uuid NOT NULL REFERENCES roadmaps (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (user_id, roadmap_id)
);

-- ---------------------------------------------------------------------------
-- 学習ログ
-- ---------------------------------------------------------------------------
CREATE TABLE learning_logs (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    roadmap_id uuid NOT NULL REFERENCES roadmaps (id) ON DELETE CASCADE,
    body       text NOT NULL CHECK (length(body) > 0),
    -- 学習した日。投稿日（created_at）とは分ける
    logged_at  date NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX learning_logs_user_idx ON learning_logs (user_id, logged_at DESC);

-- ---------------------------------------------------------------------------
-- LLM を使う非同期の仕事（判定と到達状態の下書き）
-- ---------------------------------------------------------------------------
CREATE TABLE llm_jobs (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    type        text NOT NULL CHECK (type IN ('judgment', 'outcome_draft')),
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    log_id      uuid REFERENCES learning_logs (id) ON DELETE CASCADE,
    roadmap_id  uuid REFERENCES roadmaps (id) ON DELETE CASCADE,
    status      text NOT NULL DEFAULT 'queued'
                CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    attempts    int NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    model       text NOT NULL,
    error       text,
    started_at  timestamptz,
    finished_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),

    -- 仕事の種類ごとに、必要な紐づけ先があることを保証する
    CONSTRAINT llm_jobs_needs_target CHECK (
        (type = 'judgment'      AND log_id IS NOT NULL) OR
        (type = 'outcome_draft' AND roadmap_id IS NOT NULL)
    )
);

-- ワーカーがキューの先頭を取り出すとき用（FOR UPDATE SKIP LOCKED と併用）
CREATE INDEX llm_jobs_queue_idx ON llm_jobs (created_at) WHERE status = 'queued';
CREATE INDEX llm_jobs_log_idx ON llm_jobs (log_id);

-- LLM の生レスポンス。検証で弾いたものも含めて残す（握りつぶさない：SPEC.md §4.5）
CREATE TABLE llm_responses (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id        uuid NOT NULL REFERENCES llm_jobs (id) ON DELETE CASCADE,
    raw           jsonb NOT NULL,
    -- 検証ルール V1〜V8 で弾いた・切り詰めた内容
    violations    jsonb NOT NULL DEFAULT '[]'::jsonb,
    input_tokens  int,
    output_tokens int,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX llm_responses_job_idx ON llm_responses (job_id);

-- ---------------------------------------------------------------------------
-- 理解度（イベントが一次データ、状態は導出値）
-- ---------------------------------------------------------------------------
CREATE TABLE assessment_events (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    item_id        uuid NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    -- 手動で上書きしたときは null
    log_id         uuid REFERENCES learning_logs (id) ON DELETE SET NULL,
    source         text NOT NULL CHECK (source IN ('ai', 'manual')),
    -- LLM が提案した値（切り詰める前）。manual のときは null
    proposed_level int CHECK (proposed_level BETWEEN 0 AND 5),
    -- 検証を通して実際に適用した値
    applied_level  int NOT NULL CHECK (applied_level BETWEEN 0 AND 5),
    pre_state      text NOT NULL DEFAULT 'none'
                   CHECK (pre_state IN ('none', 'learning', 'explained_only', 'self_reported')),
    evidence_type  text NOT NULL,
    rationale      text NOT NULL,
    confidence     real CHECK (confidence BETWEEN 0 AND 1),
    occurred_at    timestamptz NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE assessment_events IS
    '理解度の一次データ。追記のみで更新・削除しない。item_states はここから再計算できる';

CREATE INDEX assessment_events_item_idx ON assessment_events (item_id, occurred_at);
CREATE INDEX assessment_events_user_idx ON assessment_events (user_id, created_at DESC);
CREATE INDEX assessment_events_log_idx ON assessment_events (log_id);

-- 現在の理解度。assessment_events から導ける値なので、消えても再計算できる
CREATE TABLE item_states (
    item_id          uuid PRIMARY KEY REFERENCES items (id) ON DELETE CASCADE,
    user_id          uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    level            int NOT NULL DEFAULT 0 CHECK (level BETWEEN 0 AND 5),
    pre_state        text NOT NULL DEFAULT 'none'
                     CHECK (pre_state IN ('none', 'learning', 'explained_only', 'self_reported')),
    -- 降格の提案があった（検証ルール V6）。レベルは下げず、この印だけ立てる
    needs_review     boolean NOT NULL DEFAULT false,
    -- 最後に根拠が付いた日時。時間経過でレベルを減衰させる代わりに鮮度をここから導く
    last_evidence_at timestamptz,
    last_event_id    uuid REFERENCES assessment_events (id) ON DELETE SET NULL,
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER item_states_set_updated_at BEFORE UPDATE ON item_states
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX item_states_user_idx ON item_states (user_id);

-- ---------------------------------------------------------------------------
-- レート制限（月ごとの判定回数）
-- ---------------------------------------------------------------------------
CREATE TABLE judgment_quotas (
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- 'YYYY-MM'
    period  text NOT NULL CHECK (period ~ '^\d{4}-\d{2}$'),
    used    int NOT NULL DEFAULT 0 CHECK (used >= 0),

    PRIMARY KEY (user_id, period)
);
