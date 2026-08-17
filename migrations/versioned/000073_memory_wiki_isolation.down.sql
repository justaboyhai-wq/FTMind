DROP TRIGGER IF EXISTS trg_reject_memory_wiki_share ON kb_shares;
DROP FUNCTION IF EXISTS reject_memory_wiki_share();
DROP TRIGGER IF EXISTS trg_reject_populated_memory_wiki_marker ON knowledge_bases;
DROP FUNCTION IF EXISTS reject_populated_memory_wiki_marker();
DROP TRIGGER IF EXISTS trg_reject_memory_wiki_chunk_ingest ON chunks;
DROP TRIGGER IF EXISTS trg_reject_memory_wiki_knowledge_ingest ON knowledges;
DROP FUNCTION IF EXISTS reject_memory_wiki_knowledge_ingest();
DROP INDEX IF EXISTS ux_knowledge_bases_memory_team;
ALTER TABLE knowledge_bases DROP CONSTRAINT IF EXISTS chk_memory_wiki_identity;
ALTER TABLE knowledge_bases
    DROP COLUMN IF EXISTS memory_team_id,
    DROP COLUMN IF EXISTS is_memory_wiki;
