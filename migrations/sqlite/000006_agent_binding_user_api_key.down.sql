DROP INDEX IF EXISTS idx_agent_bindings_user_api_key_id;
DROP INDEX IF EXISTS idx_tenant_api_keys_user_id;
ALTER TABLE agent_bindings DROP COLUMN user_api_key_id;
ALTER TABLE tenant_api_keys DROP COLUMN user_id;
