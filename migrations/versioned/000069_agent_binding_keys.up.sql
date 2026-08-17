CREATE TABLE IF NOT EXISTS agent_binding_keys (
    id VARCHAR(36) PRIMARY KEY,
    binding_id VARCHAR(36) NOT NULL REFERENCES agent_bindings(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    created_by VARCHAR(36),
    expires_at TIMESTAMP,
    revoked_at TIMESTAMP,
    last_used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_agent_binding_keys_binding ON agent_binding_keys(tenant_id, binding_id);
