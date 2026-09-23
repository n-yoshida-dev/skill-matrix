-- 読み取れなかった LLM の出力を残す場所（SPEC.md §4.5「握りつぶし禁止」）
--
-- なぜ列を足すか：
--   raw は jsonb なので、JSON として壊れた出力（「承知しました。判定結果は…」等）は入らない。
--   捨てると「AI が何と言ったか」が残らず、プロンプトのどこが悪かったのかを調べようがない。
--
-- なぜ raw を text 型に変えないか：
--   正常な出力は jsonb のまま検索・抽出したい（violations 列と同じ扱い）。
--   「読めた出力」と「読めなかった出力」を別の列に分けておくと、
--   WHERE raw_text IS NOT NULL で壊れた応答だけを一覧できる。
--
-- 前提：このマイグレーション時点で llm_responses に行は無い（ワーカーが未実装のため）。

-- 読めなかったときは raw に入れるものが無いので、NOT NULL を外す
ALTER TABLE llm_responses ALTER COLUMN raw DROP NOT NULL;

ALTER TABLE llm_responses ADD COLUMN raw_text text;

-- どちらも空の行は「応答が無かった」のか「保存し忘れ」なのか区別できないので、
-- 少なくとも片方は埋まっていることを DB 側で保証する
ALTER TABLE llm_responses ADD CONSTRAINT llm_responses_has_payload CHECK (
    raw IS NOT NULL OR raw_text IS NOT NULL
);

COMMENT ON COLUMN llm_responses.raw IS
    'LLM が返した判定 JSON。読み取れなかったときは NULL で、生の文字列は raw_text に入る';
COMMENT ON COLUMN llm_responses.raw_text IS
    'JSON として読み取れなかった生の出力。正常時は NULL（SPEC.md §4.5）';
