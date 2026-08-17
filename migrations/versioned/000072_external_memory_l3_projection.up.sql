CREATE TABLE IF NOT EXISTS memory_integration_events (
    id VARCHAR(36) PRIMARY KEY,
    event_id VARCHAR(128) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    event_class VARCHAR(32) NOT NULL CHECK (event_class IN ('projection', 'revocation')),
    schema_version VARCHAR(16) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    department_id VARCHAR(128),
    workspace_id VARCHAR(128),
    project_id VARCHAR(128),
    team_id VARCHAR(128) NOT NULL,
    binding_id VARCHAR(128) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    agent_id VARCHAR(128) NOT NULL,
    task_id VARCHAR(128),
    memory_id VARCHAR(128) NOT NULL,
    memory_version BIGINT NOT NULL CHECK (memory_version > 0),
    content_checksum VARCHAR(71) NOT NULL,
    status VARCHAR(32) NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 1,
    last_error TEXT NOT NULL DEFAULT '',
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ux_memory_integration_event_id UNIQUE (event_id)
);

CREATE INDEX IF NOT EXISTS idx_memory_integration_scope
    ON memory_integration_events (tenant_id, team_id, binding_id, memory_id, memory_version);
CREATE UNIQUE INDEX IF NOT EXISTS ux_memory_integration_projection
    ON memory_integration_events (tenant_id, team_id, binding_id, memory_id, memory_version, event_class);

CREATE TABLE IF NOT EXISTS memory_l3_snapshots (
    id VARCHAR(36) PRIMARY KEY,
    event_id VARCHAR(128) NOT NULL REFERENCES memory_integration_events(event_id),
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    team_id VARCHAR(128) NOT NULL,
    binding_id VARCHAR(128) NOT NULL,
    department_id VARCHAR(128),
    workspace_id VARCHAR(128),
    project_id VARCHAR(128),
    user_id VARCHAR(128) NOT NULL,
    agent_id VARCHAR(128) NOT NULL,
    task_id VARCHAR(128),
    memory_id VARCHAR(128) NOT NULL,
    memory_version BIGINT NOT NULL CHECK (memory_version > 0),
    memory_level VARCHAR(8) NOT NULL DEFAULT 'L3',
    maturity VARCHAR(32) NOT NULL DEFAULT 'matured',
    title VARCHAR(512) NOT NULL,
    summary TEXT NOT NULL,
    content_markdown TEXT NOT NULL,
    confidence DOUBLE PRECISION NOT NULL,
    sensitivity VARCHAR(32) NOT NULL,
    evidence_refs JSONB NOT NULL DEFAULT '[]'::jsonb,
    claims JSONB NOT NULL DEFAULT '[]'::jsonb,
    content_checksum VARCHAR(71) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ux_memory_snapshot_event UNIQUE (event_id),
    CONSTRAINT ux_memory_snapshot_projection UNIQUE
        (tenant_id, team_id, binding_id, memory_id, memory_version)
);

CREATE TABLE IF NOT EXISTS memory_review_tasks (
    id VARCHAR(36) PRIMARY KEY,
    snapshot_id VARCHAR(36) NOT NULL UNIQUE REFERENCES memory_l3_snapshots(id) ON DELETE CASCADE,
    event_id VARCHAR(128) NOT NULL,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    team_id VARCHAR(128) NOT NULL,
    binding_id VARCHAR(128) NOT NULL,
    department_id VARCHAR(128),
    workspace_id VARCHAR(128),
    project_id VARCHAR(128),
    user_id VARCHAR(128) NOT NULL,
    agent_id VARCHAR(128) NOT NULL,
    task_id VARCHAR(128),
    memory_id VARCHAR(128) NOT NULL,
    memory_version BIGINT NOT NULL CHECK (memory_version > 0),
    title_snapshot VARCHAR(512) NOT NULL,
    content_snapshot TEXT NOT NULL,
    evidence_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,
    claims_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,
    content_checksum VARCHAR(71) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending_review',
    reviewer_id VARCHAR(128) NOT NULL DEFAULT '',
    review_comment TEXT NOT NULL DEFAULT '',
    reviewed_at TIMESTAMPTZ,
    target_knowledge_base_id VARCHAR(36) NOT NULL DEFAULT '',
    lock_version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_memory_review_status CHECK
        (status IN ('pending_review', 'changes_requested', 'approved', 'publishing', 'published', 'rejected', 'revoked')),
    CONSTRAINT ux_memory_review_projection UNIQUE
        (tenant_id, team_id, binding_id, memory_id, memory_version)
);

CREATE TABLE IF NOT EXISTS memory_review_histories (
    id VARCHAR(36) PRIMARY KEY,
    review_task_id VARCHAR(36) NOT NULL REFERENCES memory_review_tasks(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    team_id VARCHAR(128) NOT NULL,
    binding_id VARCHAR(128) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    memory_id VARCHAR(128) NOT NULL,
    memory_version BIGINT NOT NULL,
    content_checksum VARCHAR(71) NOT NULL,
    from_status VARCHAR(32) NOT NULL,
    to_status VARCHAR(32) NOT NULL,
    actor_id VARCHAR(128) NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_memory_review_history_task_time
    ON memory_review_histories (review_task_id, created_at);

ALTER TABLE memory_wiki_publications
    ADD COLUMN IF NOT EXISTS snapshot_id VARCHAR(36) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS review_task_id VARCHAR(36) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS event_id VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS team_id VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS binding_id VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS user_id VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS department_id VARCHAR(128),
    ADD COLUMN IF NOT EXISTS agent_id VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS task_id VARCHAR(128),
    ADD COLUMN IF NOT EXISTS memory_version BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS content_checksum VARCHAR(71) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS review_comment TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS knowledge_base_id VARCHAR(36) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS wiki_revision_id VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS wiki_page_version INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS failed_stage VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS publish_attempt_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS lock_version BIGINT NOT NULL DEFAULT 1;

-- Rows written by the temporary 000070 API predate binding/team/version
-- provenance and may contain duplicate tenant+memory pairs. Preserve them as
-- non-publishable legacy records with a per-row synthetic binding so adding
-- the real projection uniqueness constraint cannot block an upgrade.
UPDATE memory_wiki_publications
SET snapshot_id = CASE WHEN snapshot_id = '' THEN id ELSE snapshot_id END,
    review_task_id = CASE WHEN review_task_id = '' THEN id ELSE review_task_id END,
    event_id = CASE WHEN event_id = '' THEN 'legacy:' || id ELSE event_id END,
    team_id = CASE WHEN team_id = '' THEN 'legacy' ELSE team_id END,
    binding_id = CASE WHEN binding_id = '' THEN 'legacy:' || id ELSE binding_id END,
    user_id = CASE WHEN user_id = '' THEN 'legacy' ELSE user_id END,
    agent_id = CASE WHEN agent_id = '' THEN 'legacy' ELSE agent_id END,
    content_checksum = CASE WHEN content_checksum = '' THEN 'legacy:unverified:' || id ELSE content_checksum END,
    reviewed_by = COALESCE(reviewed_by, ''),
    published_page_id = COALESCE(published_page_id, '')
WHERE event_id = '' OR binding_id = '' OR team_id = '' OR user_id = '';

ALTER TABLE memory_wiki_publications
    ALTER COLUMN title TYPE VARCHAR(512),
    ALTER COLUMN workspace_id TYPE VARCHAR(128),
    ALTER COLUMN project_id TYPE VARCHAR(128),
    ALTER COLUMN reviewed_by TYPE VARCHAR(128),
    ALTER COLUMN reviewed_by SET DEFAULT '',
    ALTER COLUMN reviewed_by SET NOT NULL,
    ALTER COLUMN published_page_id SET DEFAULT '',
    ALTER COLUMN published_page_id SET NOT NULL;

ALTER TABLE memory_wiki_publications
    DROP CONSTRAINT IF EXISTS ck_memory_wiki_publication_status;
ALTER TABLE memory_wiki_publications
    ADD CONSTRAINT ck_memory_wiki_publication_status CHECK
        (status IN ('pending_review', 'changes_requested', 'approved', 'publishing', 'published', 'rejected', 'revoked'));

CREATE UNIQUE INDEX IF NOT EXISTS ux_memory_publication_projection
    ON memory_wiki_publications (tenant_id, team_id, binding_id, memory_id, memory_version);
CREATE UNIQUE INDEX IF NOT EXISTS ux_memory_publication_snapshot
    ON memory_wiki_publications (snapshot_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_memory_publication_review_task
    ON memory_wiki_publications (review_task_id);
CREATE INDEX IF NOT EXISTS idx_memory_publication_review_task
    ON memory_wiki_publications (review_task_id);

CREATE TABLE IF NOT EXISTS memory_wiki_revisions (
    id VARCHAR(128) PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    team_id VARCHAR(128) NOT NULL,
    binding_id VARCHAR(128) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    wiki_page_id VARCHAR(36) NOT NULL,
    wiki_page_version INTEGER NOT NULL CHECK (wiki_page_version > 0),
    page_slug VARCHAR(512) NOT NULL,
    memory_id VARCHAR(128) NOT NULL,
    memory_version BIGINT NOT NULL CHECK (memory_version > 0),
    source_publication_id VARCHAR(36) NOT NULL REFERENCES memory_wiki_publications(id) ON DELETE RESTRICT,
    source_review_task_id VARCHAR(36) NOT NULL REFERENCES memory_review_tasks(id) ON DELETE RESTRICT,
    content_checksum VARCHAR(71) NOT NULL,
    projection_checksum VARCHAR(71) NOT NULL,
    title VARCHAR(512) NOT NULL,
    summary TEXT NOT NULL,
    content TEXT NOT NULL,
    page_type VARCHAR(32) NOT NULL,
    page_status VARCHAR(32) NOT NULL,
    source_refs JSONB NOT NULL DEFAULT '[]'::jsonb,
    chunk_refs JSONB NOT NULL DEFAULT '[]'::jsonb,
    page_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    page_snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_memory_wiki_revision_page_checksum
    ON memory_wiki_revisions (wiki_page_id, projection_checksum, wiki_page_version);
CREATE INDEX IF NOT EXISTS idx_memory_wiki_revision_source_publication
    ON memory_wiki_revisions (source_publication_id);
CREATE INDEX IF NOT EXISTS idx_memory_wiki_revision_memory
    ON memory_wiki_revisions (tenant_id, team_id, binding_id, memory_id, memory_version);

CREATE OR REPLACE FUNCTION reject_memory_wiki_revision_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'memory Wiki revisions are immutable';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_memory_wiki_revision_immutable ON memory_wiki_revisions;
CREATE TRIGGER trg_memory_wiki_revision_immutable
    BEFORE UPDATE OR DELETE ON memory_wiki_revisions
    FOR EACH ROW EXECUTE FUNCTION reject_memory_wiki_revision_mutation();

CREATE TABLE IF NOT EXISTS wiki_claim_evidences (
    id VARCHAR(36) PRIMARY KEY,
    publication_id VARCHAR(36) NOT NULL REFERENCES memory_wiki_publications(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    team_id VARCHAR(128) NOT NULL,
    binding_id VARCHAR(128) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    memory_id VARCHAR(128) NOT NULL,
    memory_version BIGINT NOT NULL,
    wiki_page_id VARCHAR(36) NOT NULL,
    wiki_revision_id VARCHAR(128) NOT NULL REFERENCES memory_wiki_revisions(id) ON DELETE RESTRICT,
    claim_id VARCHAR(128) NOT NULL,
    claim_text TEXT NOT NULL,
    wiki_locator VARCHAR(256) NOT NULL,
    factual BOOLEAN NOT NULL,
    evidence_locators JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ux_wiki_claim_evidence_revision_claim UNIQUE
        (publication_id, wiki_revision_id, claim_id)
);
CREATE INDEX IF NOT EXISTS idx_wiki_claim_evidence_page_revision
    ON wiki_claim_evidences (wiki_page_id, wiki_revision_id);
CREATE INDEX IF NOT EXISTS idx_wiki_claim_evidence_binding_memory
    ON wiki_claim_evidences (tenant_id, team_id, binding_id, memory_id, memory_version);
