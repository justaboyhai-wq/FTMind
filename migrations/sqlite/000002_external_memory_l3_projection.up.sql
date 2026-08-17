CREATE TABLE IF NOT EXISTS memory_integration_events (
    id TEXT PRIMARY KEY,
    event_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    event_class TEXT NOT NULL CHECK (event_class IN ('projection', 'revocation')),
    schema_version TEXT NOT NULL,
    occurred_at DATETIME NOT NULL,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    department_id TEXT,
    workspace_id TEXT,
    project_id TEXT,
    team_id TEXT NOT NULL,
    binding_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    task_id TEXT,
    memory_id TEXT NOT NULL,
    memory_version INTEGER NOT NULL CHECK (memory_version > 0),
    content_checksum TEXT NOT NULL,
    status TEXT NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 1,
    last_error TEXT NOT NULL DEFAULT '',
    processed_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_memory_integration_event_id
    ON memory_integration_events(event_id);
CREATE INDEX IF NOT EXISTS idx_memory_integration_scope
    ON memory_integration_events(tenant_id, team_id, binding_id, memory_id, memory_version);
CREATE UNIQUE INDEX IF NOT EXISTS ux_memory_integration_projection
    ON memory_integration_events(tenant_id, team_id, binding_id, memory_id, memory_version, event_class);

CREATE TABLE IF NOT EXISTS memory_l3_snapshots (
    id TEXT PRIMARY KEY,
    event_id TEXT NOT NULL REFERENCES memory_integration_events(event_id),
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    team_id TEXT NOT NULL,
    binding_id TEXT NOT NULL,
    department_id TEXT,
    workspace_id TEXT,
    project_id TEXT,
    user_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    task_id TEXT,
    memory_id TEXT NOT NULL,
    memory_version INTEGER NOT NULL CHECK (memory_version > 0),
    memory_level TEXT NOT NULL DEFAULT 'L3',
    maturity TEXT NOT NULL DEFAULT 'matured',
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    content_markdown TEXT NOT NULL,
    confidence REAL NOT NULL,
    sensitivity TEXT NOT NULL,
    evidence_refs TEXT NOT NULL DEFAULT '[]',
    claims TEXT NOT NULL DEFAULT '[]',
    content_checksum TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_memory_snapshot_event
    ON memory_l3_snapshots(event_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_memory_snapshot_projection
    ON memory_l3_snapshots(tenant_id, team_id, binding_id, memory_id, memory_version);

CREATE TABLE IF NOT EXISTS memory_review_tasks (
    id TEXT PRIMARY KEY,
    snapshot_id TEXT NOT NULL UNIQUE REFERENCES memory_l3_snapshots(id) ON DELETE CASCADE,
    event_id TEXT NOT NULL,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    team_id TEXT NOT NULL,
    binding_id TEXT NOT NULL,
    department_id TEXT,
    workspace_id TEXT,
    project_id TEXT,
    user_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    task_id TEXT,
    memory_id TEXT NOT NULL,
    memory_version INTEGER NOT NULL CHECK (memory_version > 0),
    title_snapshot TEXT NOT NULL,
    content_snapshot TEXT NOT NULL,
    evidence_snapshot TEXT NOT NULL DEFAULT '[]',
    claims_snapshot TEXT NOT NULL DEFAULT '[]',
    content_checksum TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending_review' CHECK
        (status IN ('pending_review', 'changes_requested', 'approved', 'publishing', 'published', 'rejected', 'revoked')),
    reviewer_id TEXT NOT NULL DEFAULT '',
    review_comment TEXT NOT NULL DEFAULT '',
    reviewed_at DATETIME,
    target_knowledge_base_id TEXT NOT NULL DEFAULT '',
    lock_version INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_memory_review_projection
    ON memory_review_tasks(tenant_id, team_id, binding_id, memory_id, memory_version);

CREATE TABLE IF NOT EXISTS memory_wiki_publications (
    id TEXT PRIMARY KEY,
    snapshot_id TEXT NOT NULL UNIQUE REFERENCES memory_l3_snapshots(id) ON DELETE CASCADE,
    review_task_id TEXT NOT NULL UNIQUE REFERENCES memory_review_tasks(id) ON DELETE CASCADE,
    event_id TEXT NOT NULL,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    team_id TEXT NOT NULL,
    binding_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    department_id TEXT,
    workspace_id TEXT,
    project_id TEXT,
    agent_id TEXT NOT NULL,
    task_id TEXT,
    memory_id TEXT NOT NULL,
    memory_version INTEGER NOT NULL CHECK (memory_version > 0),
    title TEXT NOT NULL,
    markdown TEXT NOT NULL,
    evidence TEXT NOT NULL DEFAULT '[]',
    content_checksum TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending_review' CHECK
        (status IN ('pending_review', 'changes_requested', 'approved', 'publishing', 'published', 'rejected', 'revoked')),
    reviewed_by TEXT NOT NULL DEFAULT '',
    review_comment TEXT NOT NULL DEFAULT '',
    reviewed_at DATETIME,
    knowledge_base_id TEXT NOT NULL DEFAULT '',
    published_page_id TEXT NOT NULL DEFAULT '',
    wiki_revision_id TEXT NOT NULL DEFAULT '',
    wiki_page_version INTEGER NOT NULL DEFAULT 0,
    published_at DATETIME,
    failed_stage TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    publish_attempt_count INTEGER NOT NULL DEFAULT 0,
    lock_version INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_memory_publication_projection
    ON memory_wiki_publications(tenant_id, team_id, binding_id, memory_id, memory_version);
CREATE INDEX IF NOT EXISTS idx_memory_publication_review_task
    ON memory_wiki_publications(review_task_id);

CREATE TABLE IF NOT EXISTS memory_review_histories (
    id TEXT PRIMARY KEY,
    review_task_id TEXT NOT NULL REFERENCES memory_review_tasks(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    team_id TEXT NOT NULL,
    binding_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    memory_id TEXT NOT NULL,
    memory_version INTEGER NOT NULL,
    content_checksum TEXT NOT NULL,
    from_status TEXT NOT NULL,
    to_status TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_memory_review_history_task_time
    ON memory_review_histories(review_task_id, created_at);

CREATE TABLE IF NOT EXISTS memory_wiki_revisions (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    team_id TEXT NOT NULL,
    binding_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    knowledge_base_id TEXT NOT NULL,
    wiki_page_id TEXT NOT NULL,
    wiki_page_version INTEGER NOT NULL CHECK (wiki_page_version > 0),
    page_slug TEXT NOT NULL,
    memory_id TEXT NOT NULL,
    memory_version INTEGER NOT NULL CHECK (memory_version > 0),
    source_publication_id TEXT NOT NULL REFERENCES memory_wiki_publications(id) ON DELETE RESTRICT,
    source_review_task_id TEXT NOT NULL REFERENCES memory_review_tasks(id) ON DELETE RESTRICT,
    content_checksum TEXT NOT NULL,
    projection_checksum TEXT NOT NULL,
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    content TEXT NOT NULL,
    page_type TEXT NOT NULL,
    page_status TEXT NOT NULL,
    source_refs TEXT NOT NULL DEFAULT '[]',
    chunk_refs TEXT NOT NULL DEFAULT '[]',
    page_metadata TEXT NOT NULL DEFAULT '{}',
    page_snapshot TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_memory_wiki_revision_page_checksum
    ON memory_wiki_revisions(wiki_page_id, projection_checksum, wiki_page_version);
CREATE INDEX IF NOT EXISTS idx_memory_wiki_revision_source_publication
    ON memory_wiki_revisions(source_publication_id);
CREATE INDEX IF NOT EXISTS idx_memory_wiki_revision_memory
    ON memory_wiki_revisions(tenant_id, team_id, binding_id, memory_id, memory_version);

CREATE TRIGGER IF NOT EXISTS trg_memory_wiki_revision_immutable_update
BEFORE UPDATE ON memory_wiki_revisions
FOR EACH ROW
BEGIN
    SELECT RAISE(ABORT, 'memory Wiki revisions are immutable');
END;

CREATE TRIGGER IF NOT EXISTS trg_memory_wiki_revision_immutable_delete
BEFORE DELETE ON memory_wiki_revisions
FOR EACH ROW
BEGIN
    SELECT RAISE(ABORT, 'memory Wiki revisions are immutable');
END;

CREATE TABLE IF NOT EXISTS wiki_claim_evidences (
    id TEXT PRIMARY KEY,
    publication_id TEXT NOT NULL REFERENCES memory_wiki_publications(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    team_id TEXT NOT NULL,
    binding_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    memory_id TEXT NOT NULL,
    memory_version INTEGER NOT NULL,
    wiki_page_id TEXT NOT NULL,
    wiki_revision_id TEXT NOT NULL REFERENCES memory_wiki_revisions(id) ON DELETE RESTRICT,
    claim_id TEXT NOT NULL,
    claim_text TEXT NOT NULL,
    wiki_locator TEXT NOT NULL,
    factual INTEGER NOT NULL,
    evidence_locators TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(publication_id, wiki_revision_id, claim_id)
);
CREATE INDEX IF NOT EXISTS idx_wiki_claim_evidence_page_revision
    ON wiki_claim_evidences(wiki_page_id, wiki_revision_id);
CREATE INDEX IF NOT EXISTS idx_wiki_claim_evidence_binding_memory
    ON wiki_claim_evidences(tenant_id, team_id, binding_id, memory_id, memory_version);
