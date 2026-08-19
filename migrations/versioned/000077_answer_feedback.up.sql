CREATE TABLE IF NOT EXISTS answer_feedbacks (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    session_id VARCHAR(36) NOT NULL,
    assistant_message_id VARCHAR(36) NOT NULL,
    request_id VARCHAR(128) NOT NULL DEFAULT '',
    reporter_id VARCHAR(512) NOT NULL,
    reporter_type VARCHAR(32) NOT NULL DEFAULT 'user',
    channel VARCHAR(50) NOT NULL DEFAULT '',
    agent_id VARCHAR(36) NOT NULL DEFAULT '',
    model_id VARCHAR(64) NOT NULL DEFAULT '',
    category VARCHAR(40) NOT NULL,
    description TEXT NOT NULL,
    expected_correction TEXT NOT NULL DEFAULT '',
    quoted_text TEXT NOT NULL DEFAULT '',
    question_snapshot TEXT NOT NULL DEFAULT '',
    answer_snapshot TEXT NOT NULL DEFAULT '',
    references_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    knowledge_base_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    priority VARCHAR(20) NOT NULL DEFAULT 'normal',
    root_cause VARCHAR(40) NOT NULL DEFAULT '',
    resolution_action VARCHAR(40) NOT NULL DEFAULT '',
    public_reply TEXT NOT NULL DEFAULT '',
    internal_note TEXT NOT NULL DEFAULT '',
    assigned_to VARCHAR(36) NOT NULL DEFAULT '',
    resolved_by VARCHAR(36) NOT NULL DEFAULT '',
    resolved_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_answer_feedback_tenant_status ON answer_feedbacks(tenant_id,status,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_answer_feedback_reporter ON answer_feedbacks(tenant_id,reporter_id,created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_answer_feedback_one_per_reporter_message ON answer_feedbacks(tenant_id,reporter_id,assistant_message_id) WHERE deleted_at IS NULL;
CREATE TABLE IF NOT EXISTS answer_feedback_events (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    feedback_id VARCHAR(36) NOT NULL REFERENCES answer_feedbacks(id) ON DELETE CASCADE,
    actor_id VARCHAR(512) NOT NULL DEFAULT '',
    actor_type VARCHAR(32) NOT NULL DEFAULT 'user',
    event_type VARCHAR(40) NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_answer_feedback_events_feedback ON answer_feedback_events(tenant_id,feedback_id,created_at);
