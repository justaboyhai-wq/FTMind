ALTER TABLE knowledge_bases
    ADD COLUMN IF NOT EXISTS is_memory_wiki BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS memory_team_id VARCHAR(128);

ALTER TABLE knowledge_bases
    DROP CONSTRAINT IF EXISTS chk_memory_wiki_identity;
ALTER TABLE knowledge_bases
    ADD CONSTRAINT chk_memory_wiki_identity CHECK (
        NOT is_memory_wiki OR (
            type = 'wiki' AND memory_team_id IS NOT NULL AND btrim(memory_team_id) <> ''
        )
    );

CREATE UNIQUE INDEX IF NOT EXISTS ux_knowledge_bases_memory_team
    ON knowledge_bases (tenant_id, memory_team_id)
    WHERE is_memory_wiki = TRUE AND deleted_at IS NULL;

CREATE OR REPLACE FUNCTION reject_memory_wiki_knowledge_ingest()
RETURNS TRIGGER AS $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM knowledge_bases kb
        WHERE kb.id = NEW.knowledge_base_id
          AND kb.is_memory_wiki = TRUE
          AND kb.deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'dedicated memory Wiki rejects document/RAG ingestion';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_reject_memory_wiki_knowledge_ingest ON knowledges;
CREATE TRIGGER trg_reject_memory_wiki_knowledge_ingest
    BEFORE INSERT OR UPDATE OF knowledge_base_id ON knowledges
    FOR EACH ROW EXECUTE FUNCTION reject_memory_wiki_knowledge_ingest();

DROP TRIGGER IF EXISTS trg_reject_memory_wiki_chunk_ingest ON chunks;
CREATE TRIGGER trg_reject_memory_wiki_chunk_ingest
    BEFORE INSERT OR UPDATE OF knowledge_base_id ON chunks
    FOR EACH ROW EXECUTE FUNCTION reject_memory_wiki_knowledge_ingest();

CREATE OR REPLACE FUNCTION reject_populated_memory_wiki_marker()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.is_memory_wiki = TRUE AND (
        EXISTS (SELECT 1 FROM knowledges k WHERE k.knowledge_base_id = NEW.id AND k.deleted_at IS NULL) OR
        EXISTS (SELECT 1 FROM chunks c WHERE c.knowledge_base_id = NEW.id AND c.deleted_at IS NULL)
    ) THEN
        RAISE EXCEPTION 'a populated knowledge base cannot become a dedicated memory Wiki';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_reject_populated_memory_wiki_marker ON knowledge_bases;
CREATE TRIGGER trg_reject_populated_memory_wiki_marker
    BEFORE INSERT OR UPDATE OF is_memory_wiki, memory_team_id ON knowledge_bases
    FOR EACH ROW EXECUTE FUNCTION reject_populated_memory_wiki_marker();

-- Dedicated memory Wikis are never organization-shareable. Retire any
-- historical share before installing the trigger; the application also
-- rejects these rows, but this closes raw-SQL and older-binary bypasses.
UPDATE kb_shares AS share
SET deleted_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE share.deleted_at IS NULL
  AND EXISTS (
      SELECT 1 FROM knowledge_bases kb
      WHERE kb.id = share.knowledge_base_id
        AND kb.deleted_at IS NULL
        AND (
            kb.is_memory_wiki = TRUE OR btrim(COALESCE(kb.memory_team_id, '')) <> '' OR
            COALESCE(lower(kb.wiki_config->>'is_memory_wiki'), 'false') = 'true' OR
            btrim(COALESCE(kb.wiki_config->>'memory_team_id', '')) <> ''
        )
  );

CREATE OR REPLACE FUNCTION reject_memory_wiki_share()
RETURNS TRIGGER AS $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM knowledge_bases kb
        WHERE kb.id = NEW.knowledge_base_id
          AND kb.deleted_at IS NULL
          AND (
              kb.is_memory_wiki = TRUE OR btrim(COALESCE(kb.memory_team_id, '')) <> '' OR
              COALESCE(lower(kb.wiki_config->>'is_memory_wiki'), 'false') = 'true' OR
              btrim(COALESCE(kb.wiki_config->>'memory_team_id', '')) <> ''
          )
    ) THEN
        RAISE EXCEPTION 'dedicated memory Wiki cannot be organization-shared';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_reject_memory_wiki_share ON kb_shares;
CREATE TRIGGER trg_reject_memory_wiki_share
    BEFORE INSERT OR UPDATE OF knowledge_base_id ON kb_shares
    FOR EACH ROW EXECUTE FUNCTION reject_memory_wiki_share();
