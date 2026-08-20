-- 000003 の巻き戻し。制約は列と一緒に消えるが、意図を明示するため先に落とす
ALTER TABLE roadmaps DROP CONSTRAINT IF EXISTS roadmaps_levels_has_five_entries;
ALTER TABLE roadmaps DROP COLUMN IF EXISTS levels;
