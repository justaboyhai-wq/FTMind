ALTER TABLE agent_bindings ADD COLUMN setup_expires_at DATETIME;
ALTER TABLE agent_bindings ADD COLUMN activated_at DATETIME;
ALTER TABLE agent_bindings ADD COLUMN last_handshake_at DATETIME;
ALTER TABLE agent_bindings ADD COLUMN setup_attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agent_binding_keys ADD COLUMN purpose TEXT NOT NULL DEFAULT 'memory_binding_runtime';
ALTER TABLE agent_binding_keys ADD COLUMN consumed_at DATETIME;
UPDATE agent_bindings SET status = 'active' WHERE status IS NULL OR status = '';
UPDATE agent_binding_keys SET purpose = 'memory_binding_runtime' WHERE purpose IS NULL OR purpose = '';
DROP INDEX IF EXISTS uq_agent_bindings_active_connector;
CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_bindings_pending_or_active_connector
    ON agent_bindings(tenant_id, external_agent, connector_type)
    WHERE status IN ('pending_setup', 'active') AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_agent_binding_setup_expiry ON agent_bindings(tenant_id, status, setup_expires_at);
CREATE INDEX IF NOT EXISTS idx_agent_binding_keys_purpose ON agent_binding_keys(binding_id, purpose, consumed_at);
