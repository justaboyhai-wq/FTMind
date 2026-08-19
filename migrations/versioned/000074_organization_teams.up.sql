CREATE TABLE IF NOT EXISTS departments (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    code VARCHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_departments_tenant_code ON departments(tenant_id, code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_departments_id_tenant ON departments(id, tenant_id);

CREATE TABLE IF NOT EXISTS teams (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    department_id VARCHAR(36) NOT NULL,
    name VARCHAR(128) NOT NULL,
    code VARCHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_teams_tenant_code ON teams(tenant_id, code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_teams_department ON teams(tenant_id, department_id) WHERE deleted_at IS NULL;
ALTER TABLE teams ADD CONSTRAINT fk_teams_department_tenant FOREIGN KEY (department_id, tenant_id) REFERENCES departments(id, tenant_id) ON DELETE RESTRICT;

CREATE TABLE IF NOT EXISTS team_members (
    id BIGSERIAL PRIMARY KEY,
    team_id VARCHAR(36) NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(24) NOT NULL DEFAULT 'viewer',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_team_members_active ON team_members(team_id, user_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS team_agents (
    id BIGSERIAL PRIMARY KEY,
    team_id VARCHAR(36) NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    agent_id VARCHAR(36) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_team_agents_active ON team_agents(team_id, agent_id) WHERE deleted_at IS NULL;
ALTER TABLE team_agents ADD CONSTRAINT fk_team_agents_agent_tenant FOREIGN KEY (agent_id, tenant_id) REFERENCES custom_agents(id, tenant_id) ON DELETE CASCADE;

ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS department_id VARCHAR(36);
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS team_id VARCHAR(36);
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS visibility VARCHAR(20) NOT NULL DEFAULT 'team';
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_team_scope ON knowledge_bases(tenant_id, department_id, team_id) WHERE deleted_at IS NULL;
