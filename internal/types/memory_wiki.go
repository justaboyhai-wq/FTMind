package types

import "time"

const (
	MemoryWikiPendingReview    = "pending_review"
	MemoryWikiChangesRequested = "changes_requested"
	MemoryWikiApproved         = "approved"
	MemoryWikiPublishing       = "publishing"
	MemoryWikiPublished        = "published"
	MemoryWikiRejected         = "rejected"
	MemoryWikiRevoked          = "revoked"
)

// MemoryWikiPublication is the governance record for an L3 memory claim.
// Only Approved records may be materialized as a memory Wiki page; L2 is
// intentionally absent from this publication model.
type MemoryWikiPublication struct {
	ID                  string      `json:"id" gorm:"type:varchar(36);primaryKey"`
	SnapshotID          string      `json:"snapshot_id" gorm:"type:varchar(36);not null;uniqueIndex"`
	ReviewTaskID        string      `json:"review_task_id" gorm:"type:varchar(36);not null;uniqueIndex"`
	EventID             string      `json:"event_id" gorm:"type:varchar(128);not null;index"`
	TenantID            uint64      `json:"tenant_id" gorm:"not null;uniqueIndex:ux_memory_publication_projection,priority:1"`
	TeamID              string      `json:"team_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_memory_publication_projection,priority:2"`
	BindingID           string      `json:"binding_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_memory_publication_projection,priority:3"`
	UserID              string      `json:"user_id" gorm:"type:varchar(128);not null;index"`
	DepartmentID        string      `json:"department_id,omitempty" gorm:"type:varchar(128);index"`
	WorkspaceID         string      `json:"workspace_id,omitempty" gorm:"type:varchar(128);index"`
	ProjectID           string      `json:"project_id,omitempty" gorm:"type:varchar(128);index"`
	AgentID             string      `json:"agent_id" gorm:"type:varchar(128);not null;index"`
	TaskID              string      `json:"task_id,omitempty" gorm:"type:varchar(128);index"`
	MemoryID            string      `json:"memory_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_memory_publication_projection,priority:4"`
	MemoryVersion       uint64      `json:"memory_version" gorm:"not null;uniqueIndex:ux_memory_publication_projection,priority:5"`
	Title               string      `json:"title" gorm:"type:varchar(512);not null"`
	Markdown            string      `json:"markdown" gorm:"type:text;not null"`
	Evidence            StringArray `json:"evidence" gorm:"type:json;not null"`
	ContentChecksum     string      `json:"content_checksum" gorm:"type:varchar(71);not null"`
	Status              string      `json:"status" gorm:"type:varchar(32);not null;default:'pending_review';index"`
	ReviewedBy          string      `json:"reviewed_by,omitempty" gorm:"type:varchar(128)"`
	ReviewComment       string      `json:"review_comment,omitempty" gorm:"type:text"`
	ReviewedAt          *time.Time  `json:"reviewed_at,omitempty"`
	KnowledgeBaseID     string      `json:"knowledge_base_id,omitempty" gorm:"type:varchar(36);index"`
	PublishedPageID     string      `json:"published_page_id,omitempty" gorm:"type:varchar(36);index"`
	WikiRevisionID      string      `json:"wiki_revision_id,omitempty" gorm:"type:varchar(128);index"`
	WikiPageVersion     int         `json:"wiki_page_version,omitempty"`
	PublishedAt         *time.Time  `json:"published_at,omitempty"`
	FailedStage         string      `json:"failed_stage,omitempty" gorm:"type:varchar(64)"`
	LastError           string      `json:"last_error,omitempty" gorm:"type:text"`
	PublishAttemptCount uint        `json:"publish_attempt_count" gorm:"not null;default:0"`
	LockVersion         uint64      `json:"lock_version" gorm:"not null;default:1"`
	CreatedAt           time.Time   `json:"created_at"`
	UpdatedAt           time.Time   `json:"updated_at"`
}

func (MemoryWikiPublication) TableName() string { return "memory_wiki_publications" }
