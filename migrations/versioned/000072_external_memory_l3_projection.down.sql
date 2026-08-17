DROP TRIGGER IF EXISTS trg_memory_wiki_revision_immutable ON memory_wiki_revisions;
DROP FUNCTION IF EXISTS reject_memory_wiki_revision_mutation();
DROP TABLE IF EXISTS wiki_claim_evidences;
DROP TABLE IF EXISTS memory_wiki_revisions;

DROP INDEX IF EXISTS idx_memory_publication_review_task;
DROP INDEX IF EXISTS ux_memory_publication_review_task;
DROP INDEX IF EXISTS ux_memory_publication_snapshot;
DROP INDEX IF EXISTS ux_memory_publication_projection;
ALTER TABLE memory_wiki_publications
    DROP CONSTRAINT IF EXISTS ck_memory_wiki_publication_status,
    DROP COLUMN IF EXISTS lock_version,
    DROP COLUMN IF EXISTS publish_attempt_count,
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS failed_stage,
    DROP COLUMN IF EXISTS published_at,
    DROP COLUMN IF EXISTS wiki_page_version,
    DROP COLUMN IF EXISTS wiki_revision_id,
    DROP COLUMN IF EXISTS knowledge_base_id,
    DROP COLUMN IF EXISTS review_comment,
    DROP COLUMN IF EXISTS content_checksum,
    DROP COLUMN IF EXISTS memory_version,
    DROP COLUMN IF EXISTS task_id,
    DROP COLUMN IF EXISTS agent_id,
    DROP COLUMN IF EXISTS department_id,
    DROP COLUMN IF EXISTS user_id,
    DROP COLUMN IF EXISTS binding_id,
    DROP COLUMN IF EXISTS team_id,
    DROP COLUMN IF EXISTS event_id,
    DROP COLUMN IF EXISTS review_task_id,
    DROP COLUMN IF EXISTS snapshot_id;

ALTER TABLE memory_wiki_publications
    ALTER COLUMN title TYPE VARCHAR(255),
    ALTER COLUMN workspace_id TYPE VARCHAR(36),
    ALTER COLUMN workspace_id DROP NOT NULL,
    ALTER COLUMN workspace_id DROP DEFAULT,
    ALTER COLUMN project_id TYPE VARCHAR(36),
    ALTER COLUMN project_id DROP NOT NULL,
    ALTER COLUMN project_id DROP DEFAULT,
    ALTER COLUMN reviewed_by TYPE VARCHAR(36),
    ALTER COLUMN reviewed_by DROP NOT NULL,
    ALTER COLUMN reviewed_by DROP DEFAULT,
    ALTER COLUMN published_page_id DROP NOT NULL,
    ALTER COLUMN published_page_id DROP DEFAULT;

DROP TABLE IF EXISTS memory_review_histories;
DROP TABLE IF EXISTS memory_review_tasks;
DROP TABLE IF EXISTS memory_l3_snapshots;
DROP TABLE IF EXISTS memory_integration_events;
