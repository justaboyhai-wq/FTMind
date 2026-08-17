DROP INDEX IF EXISTS uq_agent_bindings_active_connector;
DROP INDEX IF EXISTS idx_agent_binding_keys_prefix;
DROP INDEX IF EXISTS idx_agent_bindings_expiry;
DROP INDEX IF EXISTS idx_agent_bindings_org_scope;

ALTER TABLE agent_bindings DROP CONSTRAINT IF EXISTS chk_agent_bindings_active_identity;
ALTER TABLE agent_binding_keys DROP CONSTRAINT IF EXISTS chk_agent_binding_keys_active_prefix;

ALTER TABLE agent_binding_keys DROP COLUMN IF EXISTS key_prefix;

ALTER TABLE agent_bindings DROP COLUMN IF EXISTS policy_version;
ALTER TABLE agent_bindings DROP COLUMN IF EXISTS l3_review_required;
ALTER TABLE agent_bindings DROP COLUMN IF EXISTS l3_wiki_enabled;
ALTER TABLE agent_bindings DROP COLUMN IF EXISTS recall_enabled;
ALTER TABLE agent_bindings DROP COLUMN IF EXISTS capture_enabled;
ALTER TABLE agent_bindings DROP COLUMN IF EXISTS task_id;
ALTER TABLE agent_bindings DROP COLUMN IF EXISTS user_id;
ALTER TABLE agent_bindings DROP COLUMN IF EXISTS team_id;
