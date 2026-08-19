ALTER TABLE agent_bindings ADD COLUMN user_api_key_id INTEGER;
ALTER TABLE tenant_api_keys ADD COLUMN user_id TEXT;
CREATE INDEX IF NOT EXISTS idx_agent_bindings_user_api_key_id ON agent_bindings(user_api_key_id);
CREATE INDEX IF NOT EXISTS idx_tenant_api_keys_user_id ON tenant_api_keys(user_id);
