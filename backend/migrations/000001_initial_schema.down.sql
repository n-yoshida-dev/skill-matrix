-- 000001_initial_schema.up.sql の巻き戻し。
-- 外部キーの依存を壊さないよう、作成と逆の順で落とす。

DROP TABLE IF EXISTS judgment_quotas;
DROP TABLE IF EXISTS item_states;
DROP TABLE IF EXISTS assessment_events;
DROP TABLE IF EXISTS llm_responses;
DROP TABLE IF EXISTS llm_jobs;
DROP TABLE IF EXISTS learning_logs;
DROP TABLE IF EXISTS stars;
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS domains;
DROP TABLE IF EXISTS roadmaps;
DROP TABLE IF EXISTS users;

DROP FUNCTION IF EXISTS set_updated_at();
