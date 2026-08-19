DROP TABLE IF EXISTS team_agents;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS departments;
ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS visibility;
ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS team_id;
ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS department_id;
