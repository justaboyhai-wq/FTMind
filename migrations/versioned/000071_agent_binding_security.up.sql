ALTER TABLE agent_bindings ADD COLUMN IF NOT EXISTS team_id VARCHAR(36);
ALTER TABLE agent_bindings ADD COLUMN IF NOT EXISTS user_id VARCHAR(36);
ALTER TABLE agent_bindings ADD COLUMN IF NOT EXISTS task_id VARCHAR(64);
ALTER TABLE agent_bindings ADD COLUMN IF NOT EXISTS capture_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE agent_bindings ADD COLUMN IF NOT EXISTS recall_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE agent_bindings ADD COLUMN IF NOT EXISTS l3_wiki_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE agent_bindings ADD COLUMN IF NOT EXISTS l3_review_required BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE agent_bindings ADD COLUMN IF NOT EXISTS policy_version BIGINT NOT NULL DEFAULT 1;

ALTER TABLE agent_binding_keys ADD COLUMN IF NOT EXISTS key_prefix VARCHAR(24);

-- The older schema could not express team/user authority or key prefixes, so
-- those rows cannot be safely backfilled. Revoke them explicitly before the
-- active-only uniqueness and identity constraint are installed. Operators can
-- recreate them through the control plane and receive a fresh one-time secret.
UPDATE agent_binding_keys AS keys
SET revoked_at = COALESCE(keys.revoked_at, CURRENT_TIMESTAMP), updated_at = CURRENT_TIMESTAMP
FROM agent_bindings AS bindings
WHERE keys.binding_id = bindings.id
  AND (bindings.team_id IS NULL OR bindings.user_id IS NULL OR keys.key_prefix IS NULL);

UPDATE agent_bindings
SET status = 'revoked', updated_at = CURRENT_TIMESTAMP
WHERE team_id IS NULL OR user_id IS NULL;

ALTER TABLE agent_bindings
    ADD CONSTRAINT chk_agent_bindings_active_identity
    CHECK (status <> 'active' OR (team_id IS NOT NULL AND user_id IS NOT NULL));
ALTER TABLE agent_binding_keys
    ADD CONSTRAINT chk_agent_binding_keys_active_prefix
    CHECK (revoked_at IS NOT NULL OR key_prefix IS NOT NULL);

CREATE INDEX IF NOT EXISTS idx_agent_bindings_org_scope
    ON agent_bindings(tenant_id, department_id, team_id, workspace_id, project_id, user_id, agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_bindings_expiry
    ON agent_bindings(tenant_id, status, expires_at);
CREATE INDEX IF NOT EXISTS idx_agent_binding_keys_prefix
    ON agent_binding_keys(key_prefix);
CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_bindings_active_connector
    ON agent_bindings(tenant_id, external_agent, connector_type)
    WHERE status = 'active' AND deleted_at IS NULL;
