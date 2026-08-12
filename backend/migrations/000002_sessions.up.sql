-- ログインセッション（SPEC.md §3.1、設計の要点は §3.2）
--
-- Cookie に入れるのは32バイトの乱数。DB にはその SHA-256 だけを置く。
-- こうしておくと、DB が漏れても、その値をそのまま Cookie に詰めてログインすることはできない。
-- 主キーをハッシュそのものにしているので、引き当てに追加の索引が要らない。

CREATE TABLE sessions (
    token_hash   bytea PRIMARY KEY CHECK (length(token_hash) = 32),
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- 発行から30日。アクセスのたびに延長する
    expires_at   timestamptz NOT NULL,
    last_used_at timestamptz NOT NULL DEFAULT now(),
    created_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT sessions_expires_after_creation CHECK (expires_at > created_at)
);

COMMENT ON COLUMN sessions.token_hash IS
    'Cookie に入れた32バイト乱数の SHA-256。生のトークンは保存しない';

-- ログアウト（全端末）と、退会時の削除で user_id を引くため
CREATE INDEX sessions_user_idx ON sessions (user_id);
-- 期限切れの行をまとめて掃除するため
CREATE INDEX sessions_expires_idx ON sessions (expires_at);
