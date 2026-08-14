-- ロードマップごとのレベル定義（SPEC.md §2 の levels）
--
-- なぜロードマップ側に持つか：
--   levels の criteria（判定基準）は、そのまま LLM のプロンプトに埋め込まれる判定基準の正本。
--   「レベル3＝ガイド付き実装」の意味は分野によって書き分けたくなる（Go の実装と英語学習では
--   同じ言葉が同じ意味にならない）ため、アプリ共通の固定値ではなくロードマップに紐づける。
--
-- なぜ専用テーブルにせず jsonb か：
--   常に1〜5の5件セットでしか読み書きしないため、行に割ると JOIN が増えるだけで得がない。
--   個別の level を単独で更新する要件も無い。
--
-- 前提：このマイグレーション時点で roadmaps に行は無い（インポート API が未実装のため）。
-- 既存行があると 5 件必須の制約に引っかかる。

ALTER TABLE roadmaps ADD COLUMN levels jsonb NOT NULL DEFAULT '[]'::jsonb;

-- 既定値は「既存行を埋めるため」だけのもの。以降は必ずアプリから明示的に入れさせる
ALTER TABLE roadmaps ALTER COLUMN levels DROP DEFAULT;

-- 中身の妥当性（level が 1〜5 揃っているか・criteria が空でないか）は
-- internal/roadmap の検査で見る。DB では「5件の配列であること」だけを保証する
ALTER TABLE roadmaps ADD CONSTRAINT roadmaps_levels_has_five_entries CHECK (
    jsonb_typeof(levels) = 'array' AND jsonb_array_length(levels) = 5
);

COMMENT ON COLUMN roadmaps.levels IS
    'レベル1〜5の定義。[{"level":1,"name":"...","criteria":"..."}, ...]。criteria は LLM プロンプトの正本（SPEC.md §2）';
