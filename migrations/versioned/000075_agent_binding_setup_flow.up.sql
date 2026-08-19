ALTER TABLE agent_bindings ADD COLUMN IF NOT EXISTS setup_expires_at TIMESTAMP;
ALTER TABLE agent_bindings ADD COLUMN IF NOT EXISTS activated_at TIMESTAMP;
ALTER TABLE agent_bindings ADD COLUMN IF NOT EXISTS last_handshake_at TIMESTAMP;
ALTER TABLE agent_bindings ADD COLUMN IF NOT EXISTS setup_attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agent_binding_keys ADD COLUMN IF NOT EXISTS purpose VARCHAR(40) NOT NULL DEFAULT 'memory_binding_runtime';
ALTER TABLE agent_binding_keys ADD COLUMN IF NOT EXISTS consumed_at TIMESTAMP;

UPDATE agent_bindings SET status = 'active' WHERE status IS NULL OR status = '';
UPDATE agent_binding_keys SET purpose = 'memory_binding_runtime' WHERE purpose IS NULL OR purpose = '';

ALTER TABLE agent_bindings DROP CONSTRAINT IF EXISTS chk_agent_bindings_status;
ALTER TABLE agent_bindings ADD CONSTRAINT chk_agent_bindings_status CHECK (status IN ('pending_setup', 'active', 'revoked'));
DROP INDEX IF EXISTS uq_agent_bindings_active_connector;
CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_bindings_pending_or_active_connector
    ON agent_bindings(tenant_id, external_agent, connector_type)
    WHERE status IN ('pending_setup', 'active') AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_agent_binding_setup_expiry
    ON agent_bindings(tenant_id, status, setup_expires_at);
CREATE INDEX IF NOT EXISTS idx_agent_binding_keys_purpose
    ON agent_binding_keys(binding_id, purpose, consumed_at);
