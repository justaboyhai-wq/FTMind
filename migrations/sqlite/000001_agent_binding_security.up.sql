CREATE TABLE IF NOT EXISTS agent_bindings (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    department_id TEXT,
    team_id TEXT NOT NULL,
    workspace_id TEXT,
    project_id TEXT,
    user_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    task_id TEXT,
    external_agent TEXT NOT NULL,
    connector_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    capture_enabled INTEGER NOT NULL DEFAULT 0,
    recall_enabled INTEGER NOT NULL DEFAULT 0,
    l3_wiki_enabled INTEGER NOT NULL DEFAULT 0,
    l3_review_required INTEGER NOT NULL DEFAULT 1,
    capability_scopes TEXT NOT NULL DEFAULT '[]',
    asset_scopes TEXT NOT NULL DEFAULT '[]',
    policy_version INTEGER NOT NULL DEFAULT 1,
    created_by TEXT,
    last_used_at DATETIME,
    expires_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE TABLE IF NOT EXISTS agent_binding_keys (
    id TEXT PRIMARY KEY,
    binding_id TEXT NOT NULL REFERENCES agent_bindings(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    key_prefix TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    created_by TEXT,
    expires_at DATETIME,
    revoked_at DATETIME,
    last_used_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agent_bindings_org_scope
    ON agent_bindings(tenant_id, department_id, team_id, workspace_id, project_id, user_id, agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_bindings_expiry
    ON agent_bindings(tenant_id, status, expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_bindings_active_connector
    ON agent_bindings(tenant_id, external_agent, connector_type)
    WHERE status = 'active' AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_agent_binding_keys_binding
    ON agent_binding_keys(tenant_id, binding_id);
CREATE INDEX IF NOT EXISTS idx_agent_binding_keys_prefix
    ON agent_binding_keys(key_prefix);
