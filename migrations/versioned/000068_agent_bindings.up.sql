CREATE TABLE IF NOT EXISTS agent_bindings (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    department_id VARCHAR(36),
    workspace_id VARCHAR(36),
    project_id VARCHAR(36),
    agent_id VARCHAR(36) NOT NULL,
    external_agent VARCHAR(128) NOT NULL,
    connector_type VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    capability_scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
    asset_scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by VARCHAR(36),
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_agent_bindings_tenant ON agent_bindings(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agent_bindings_lookup ON agent_bindings(tenant_id, external_agent, connector_type, status);
