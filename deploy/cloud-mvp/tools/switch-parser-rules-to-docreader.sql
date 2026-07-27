-- Switch legacy MinerU parser rules to the Docker-internal DocReader engines.
--
-- PDF/DOC rules use DocReader's builtin engine. Legacy MinerU PPT/XLS rules
-- use DocReader's markitdown engine. Existing rules are otherwise preserved.
--
-- This script intentionally does not COMMIT. Run it after the OSS migration,
-- inspect the returned rows, and commit only when every target KB is correct.

BEGIN;

WITH updated AS (
  UPDATE knowledge_bases
  SET chunking_config = jsonb_set(
        chunking_config,
        '{parser_engine_rules}',
        (
          SELECT COALESCE(
            jsonb_agg(
              CASE
                WHEN rule->>'engine' IN ('mineru', 'mineru_cloud') THEN
                  jsonb_set(
                    rule,
                    '{engine}',
                    to_jsonb(
                      CASE
                        WHEN rule->'file_types' ?| ARRAY['ppt', 'pptx', 'xls', 'xlsx']
                          THEN 'markitdown'
                        ELSE 'builtin'
                      END
                    )
                  )
                ELSE rule
              END
            ),
            '[]'::jsonb
          )
          FROM jsonb_array_elements(
            COALESCE(chunking_config->'parser_engine_rules', '[]'::jsonb)
          ) AS rule
        )
      ),
      updated_at = NOW()
  WHERE chunking_config::text LIKE '%mineru_cloud%'
     OR chunking_config::text LIKE '%"engine": "mineru"%'
  RETURNING id, name, chunking_config
)
SELECT id, name, chunking_config->'parser_engine_rules' AS parser_engine_rules
FROM updated
ORDER BY name;

-- COMMIT; -- intentionally disabled; verify first.
