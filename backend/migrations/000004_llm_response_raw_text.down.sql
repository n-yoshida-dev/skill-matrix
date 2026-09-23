-- 000004 の巻き戻し。
-- raw_text にしか中身が無い行は raw が NULL なので、NOT NULL を戻す前に落とす。
ALTER TABLE llm_responses DROP CONSTRAINT IF EXISTS llm_responses_has_payload;
DELETE FROM llm_responses WHERE raw IS NULL;
ALTER TABLE llm_responses DROP COLUMN IF EXISTS raw_text;
ALTER TABLE llm_responses ALTER COLUMN raw SET NOT NULL;
