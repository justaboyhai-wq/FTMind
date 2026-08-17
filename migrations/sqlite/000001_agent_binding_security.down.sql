DROP INDEX IF EXISTS idx_agent_binding_keys_prefix;
DROP INDEX IF EXISTS idx_agent_binding_keys_binding;
DROP INDEX IF EXISTS uq_agent_bindings_active_connector;
DROP INDEX IF EXISTS idx_agent_bindings_expiry;
DROP INDEX IF EXISTS idx_agent_bindings_org_scope;
DROP TABLE IF EXISTS agent_binding_keys;
DROP TABLE IF EXISTS agent_bindings;
