ALTER TABLE knowledge_bases ADD COLUMN is_memory_wiki INTEGER NOT NULL DEFAULT 0;
ALTER TABLE knowledge_bases ADD COLUMN memory_team_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS ux_knowledge_bases_memory_team
    ON knowledge_bases (tenant_id, memory_team_id)
    WHERE is_memory_wiki = 1 AND deleted_at IS NULL;

CREATE TRIGGER IF NOT EXISTS trg_reject_memory_wiki_knowledge_insert
BEFORE INSERT ON knowledges
FOR EACH ROW
WHEN EXISTS (
    SELECT 1 FROM knowledge_bases kb
    WHERE kb.id = NEW.knowledge_base_id AND kb.is_memory_wiki = 1 AND kb.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'dedicated memory Wiki rejects document/RAG ingestion');
END;

CREATE TRIGGER IF NOT EXISTS trg_reject_memory_wiki_knowledge_update
BEFORE UPDATE OF knowledge_base_id ON knowledges
FOR EACH ROW
WHEN EXISTS (
    SELECT 1 FROM knowledge_bases kb
    WHERE kb.id = NEW.knowledge_base_id AND kb.is_memory_wiki = 1 AND kb.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'dedicated memory Wiki rejects document/RAG ingestion');
END;

CREATE TRIGGER IF NOT EXISTS trg_reject_memory_wiki_chunk_insert
BEFORE INSERT ON chunks
FOR EACH ROW
WHEN EXISTS (
    SELECT 1 FROM knowledge_bases kb
    WHERE kb.id = NEW.knowledge_base_id AND kb.is_memory_wiki = 1 AND kb.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'dedicated memory Wiki rejects document/RAG ingestion');
END;

CREATE TRIGGER IF NOT EXISTS trg_reject_memory_wiki_chunk_update
BEFORE UPDATE OF knowledge_base_id ON chunks
FOR EACH ROW
WHEN EXISTS (
    SELECT 1 FROM knowledge_bases kb
    WHERE kb.id = NEW.knowledge_base_id AND kb.is_memory_wiki = 1 AND kb.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'dedicated memory Wiki rejects document/RAG ingestion');
END;

CREATE TRIGGER IF NOT EXISTS trg_reject_populated_memory_wiki_marker
BEFORE UPDATE OF is_memory_wiki, memory_team_id ON knowledge_bases
FOR EACH ROW
WHEN NEW.is_memory_wiki = 1 AND (
    EXISTS (SELECT 1 FROM knowledges k WHERE k.knowledge_base_id = NEW.id AND k.deleted_at IS NULL) OR
    EXISTS (SELECT 1 FROM chunks c WHERE c.knowledge_base_id = NEW.id AND c.deleted_at IS NULL)
)
BEGIN
    SELECT RAISE(ABORT, 'a populated knowledge base cannot become a dedicated memory Wiki');
END;

UPDATE kb_shares
SET deleted_at = CURRENT_TIMESTAMP
WHERE deleted_at IS NULL
  AND EXISTS (
      SELECT 1 FROM knowledge_bases kb
      WHERE kb.id = kb_shares.knowledge_base_id
        AND kb.deleted_at IS NULL
        AND (
            kb.is_memory_wiki = 1 OR trim(COALESCE(kb.memory_team_id, '')) <> '' OR
            COALESCE(json_extract(kb.wiki_config, '$.is_memory_wiki'), 0) = 1 OR
            trim(COALESCE(json_extract(kb.wiki_config, '$.memory_team_id'), '')) <> ''
        )
  );

CREATE TRIGGER IF NOT EXISTS trg_reject_memory_wiki_share_insert
BEFORE INSERT ON kb_shares
FOR EACH ROW
WHEN EXISTS (
    SELECT 1 FROM knowledge_bases kb
    WHERE kb.id = NEW.knowledge_base_id AND kb.deleted_at IS NULL
      AND (
          kb.is_memory_wiki = 1 OR trim(COALESCE(kb.memory_team_id, '')) <> '' OR
          COALESCE(json_extract(kb.wiki_config, '$.is_memory_wiki'), 0) = 1 OR
          trim(COALESCE(json_extract(kb.wiki_config, '$.memory_team_id'), '')) <> ''
      )
)
BEGIN
    SELECT RAISE(ABORT, 'dedicated memory Wiki cannot be organization-shared');
END;

CREATE TRIGGER IF NOT EXISTS trg_reject_memory_wiki_share_update
BEFORE UPDATE OF knowledge_base_id ON kb_shares
FOR EACH ROW
WHEN EXISTS (
    SELECT 1 FROM knowledge_bases kb
    WHERE kb.id = NEW.knowledge_base_id AND kb.deleted_at IS NULL
      AND (
          kb.is_memory_wiki = 1 OR trim(COALESCE(kb.memory_team_id, '')) <> '' OR
          COALESCE(json_extract(kb.wiki_config, '$.is_memory_wiki'), 0) = 1 OR
          trim(COALESCE(json_extract(kb.wiki_config, '$.memory_team_id'), '')) <> ''
      )
)
BEGIN
    SELECT RAISE(ABORT, 'dedicated memory Wiki cannot be organization-shared');
END;
