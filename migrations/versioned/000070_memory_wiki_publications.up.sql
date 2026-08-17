CREATE TABLE IF NOT EXISTS memory_wiki_publications (
 id VARCHAR(36) PRIMARY KEY, tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
 workspace_id VARCHAR(36), project_id VARCHAR(36), memory_id VARCHAR(128) NOT NULL,
 title VARCHAR(255) NOT NULL, markdown TEXT NOT NULL, evidence JSONB NOT NULL DEFAULT '[]'::jsonb,
 status VARCHAR(32) NOT NULL DEFAULT 'pending_review', reviewed_by VARCHAR(36), reviewed_at TIMESTAMP,
 published_page_id VARCHAR(36), created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_memory_wiki_publications_tenant_status ON memory_wiki_publications(tenant_id,status);
