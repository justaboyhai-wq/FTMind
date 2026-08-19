ALTER TABLE agent_bindings ADD COLUMN IF NOT EXISTS user_api_key_id BIGINT;
ALTER TABLE tenant_api_keys ADD COLUMN IF NOT EXISTS user_id VARCHAR(36);
CREATE INDEX IF NOT EXISTS idx_agent_bindings_user_api_key_id ON agent_bindings(user_api_key_id);
CREATE INDEX IF NOT EXISTS idx_tenant_api_keys_user_id ON tenant_api_keys(user_id);
