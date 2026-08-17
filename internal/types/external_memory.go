package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

const (
	MemoryL3EventMatured = "memory.l3.matured"
	MemoryL3EventUpdated = "memory.l3.updated"
	MemoryL3EventRevoked = "memory.l3.revoked"

	MemoryIntegrationEventReceived         = "received"
	MemoryIntegrationEventProcessed        = "processed"
	MemoryIntegrationEventValidationFailed = "validation_failed"

	MemoryIntegrationEventClassProjection = "projection"
	MemoryIntegrationEventClassRevocation = "revocation"

	MemoryReviewStatusPendingReview    = MemoryWikiPendingReview
	MemoryReviewStatusChangesRequested = MemoryWikiChangesRequested
	MemoryReviewStatusApproved         = MemoryWikiApproved
	MemoryReviewStatusPublishing       = MemoryWikiPublishing
	MemoryReviewStatusPublished        = MemoryWikiPublished
	MemoryReviewStatusRejected         = MemoryWikiRejected
	MemoryReviewStatusRevoked          = MemoryWikiRevoked
)

// MemoryProjectionKey is the isolation and idempotency boundary for one
// immutable L3 version. Memory IDs are not globally unique across bindings.
type MemoryProjectionKey struct {
	TenantID      uint64 `json:"tenant_id"`
	TeamID        string `json:"team_id"`
	BindingID     string `json:"binding_id"`
	MemoryID      string `json:"memory_id"`
	MemoryVersion uint64 `json:"memory_version"`
}

// TrustedL3Event is passed only by the authenticated internal event intake.
// It deliberately contains no review or publication status fields, so callers
// cannot submit pre-approved content.
type TrustedL3Event struct {
	EventID         string             `json:"event_id"`
	EventType       string             `json:"event_type"`
	SchemaVersion   string             `json:"schema_version"`
	OccurredAt      time.Time          `json:"occurred_at"`
	TenantID        uint64             `json:"tenant_id"`
	DepartmentID    string             `json:"department_id"`
	WorkspaceID     string             `json:"workspace_id"`
	ProjectID       string             `json:"project_id"`
	TeamID          string             `json:"team_id"`
	BindingID       string             `json:"binding_id"`
	UserID          string             `json:"user_id"`
	AgentID         string             `json:"agent_id"`
	TaskID          string             `json:"task_id"`
	MemoryID        string             `json:"memory_id"`
	MemoryVersion   uint64             `json:"memory_version"`
	MemoryLevel     string             `json:"memory_level"`
	Maturity        string             `json:"maturity"`
	Title           string             `json:"title"`
	Summary         string             `json:"summary"`
	ContentMarkdown string             `json:"content_markdown"`
	Confidence      float64            `json:"confidence"`
	Sensitivity     string             `json:"sensitivity"`
	EvidenceRefs    EvidenceReferences `json:"evidence_refs"`
	Claims          ClaimEvidenceSet   `json:"claims"`
	ContentChecksum string             `json:"content_checksum"`
}

func (e TrustedL3Event) ProjectionKey() MemoryProjectionKey {
	return MemoryProjectionKey{TenantID: e.TenantID, TeamID: e.TeamID, BindingID: e.BindingID, MemoryID: e.MemoryID, MemoryVersion: e.MemoryVersion}
}

// EvidenceReference is a structured locator into MemoryCore evidence. It is
// deliberately metadata-only; the raw conversation is not copied into Wiki.
type EvidenceReference struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Locator  string `json:"locator,omitempty"`
	Checksum string `json:"checksum,omitempty"`
}

type EvidenceReferences []EvidenceReference

func (e EvidenceReferences) Value() (driver.Value, error) { return json.Marshal(e) }

func (e *EvidenceReferences) Scan(value any) error {
	if value == nil {
		*e = nil
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("scan evidence references from %T", value)
	}
	return json.Unmarshal(data, e)
}

// ClaimEvidence binds one rendered claim to its evidence locators. Factual
// claims require at least one valid locator before formal publication.
type ClaimEvidence struct {
	ClaimID  string             `json:"claim_id"`
	Text     string             `json:"text"`
	Factual  bool               `json:"factual"`
	Evidence EvidenceReferences `json:"evidence"`
}

type ClaimEvidenceSet []ClaimEvidence

func (c ClaimEvidenceSet) Value() (driver.Value, error) { return json.Marshal(c) }

func (c *ClaimEvidenceSet) Scan(value any) error {
	if value == nil {
		*c = nil
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("scan claim evidence from %T", value)
	}
	return json.Unmarshal(data, c)
}

// MemoryIntegrationEvent is the durable inbound event/idempotency ledger.
type MemoryIntegrationEvent struct {
	ID              string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	EventID         string     `json:"event_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_memory_integration_event_id"`
	EventType       string     `json:"event_type" gorm:"type:varchar(64);not null"`
	SchemaVersion   string     `json:"schema_version" gorm:"type:varchar(16);not null"`
	OccurredAt      time.Time  `json:"occurred_at" gorm:"not null"`
	TenantID        uint64     `json:"tenant_id" gorm:"not null;index;uniqueIndex:ux_memory_integration_projection,priority:1"`
	DepartmentID    string     `json:"department_id,omitempty" gorm:"type:varchar(128);index"`
	WorkspaceID     string     `json:"workspace_id,omitempty" gorm:"type:varchar(128);index"`
	ProjectID       string     `json:"project_id,omitempty" gorm:"type:varchar(128);index"`
	TeamID          string     `json:"team_id" gorm:"type:varchar(128);not null;index;uniqueIndex:ux_memory_integration_projection,priority:2"`
	BindingID       string     `json:"binding_id" gorm:"type:varchar(128);not null;index;uniqueIndex:ux_memory_integration_projection,priority:3"`
	UserID          string     `json:"user_id" gorm:"type:varchar(128);not null;index"`
	AgentID         string     `json:"agent_id" gorm:"type:varchar(128);not null;index"`
	TaskID          string     `json:"task_id,omitempty" gorm:"type:varchar(128);index"`
	MemoryID        string     `json:"memory_id" gorm:"type:varchar(128);not null;index;uniqueIndex:ux_memory_integration_projection,priority:4"`
	MemoryVersion   uint64     `json:"memory_version" gorm:"not null;uniqueIndex:ux_memory_integration_projection,priority:5"`
	EventClass      string     `json:"event_class" gorm:"type:varchar(32);not null;uniqueIndex:ux_memory_integration_projection,priority:6"`
	ContentChecksum string     `json:"content_checksum" gorm:"type:varchar(71);not null"`
	Status          string     `json:"status" gorm:"type:varchar(32);not null;index"`
	AttemptCount    uint       `json:"attempt_count" gorm:"not null;default:1"`
	LastError       string     `json:"last_error,omitempty" gorm:"type:text"`
	ProcessedAt     *time.Time `json:"processed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (MemoryIntegrationEvent) TableName() string { return "memory_integration_events" }

func (e MemoryIntegrationEvent) ProjectionKey() MemoryProjectionKey {
	return MemoryProjectionKey{TenantID: e.TenantID, TeamID: e.TeamID, BindingID: e.BindingID, MemoryID: e.MemoryID, MemoryVersion: e.MemoryVersion}
}

// MemoryL3Snapshot freezes the reviewed source independently of MemoryCore's
// availability or later mutation.
type MemoryL3Snapshot struct {
	ID              string             `json:"id" gorm:"type:varchar(36);primaryKey"`
	EventID         string             `json:"event_id" gorm:"type:varchar(128);not null;index"`
	TenantID        uint64             `json:"tenant_id" gorm:"not null;uniqueIndex:ux_memory_snapshot_projection,priority:1"`
	TeamID          string             `json:"team_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_memory_snapshot_projection,priority:2"`
	BindingID       string             `json:"binding_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_memory_snapshot_projection,priority:3"`
	DepartmentID    string             `json:"department_id,omitempty" gorm:"type:varchar(128);index"`
	WorkspaceID     string             `json:"workspace_id,omitempty" gorm:"type:varchar(128);index"`
	ProjectID       string             `json:"project_id,omitempty" gorm:"type:varchar(128);index"`
	UserID          string             `json:"user_id" gorm:"type:varchar(128);not null;index"`
	AgentID         string             `json:"agent_id" gorm:"type:varchar(128);not null;index"`
	TaskID          string             `json:"task_id,omitempty" gorm:"type:varchar(128);index"`
	MemoryID        string             `json:"memory_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_memory_snapshot_projection,priority:4"`
	MemoryVersion   uint64             `json:"memory_version" gorm:"not null;uniqueIndex:ux_memory_snapshot_projection,priority:5"`
	MemoryLevel     string             `json:"memory_level" gorm:"type:varchar(8);not null;default:'L3'"`
	Maturity        string             `json:"maturity" gorm:"type:varchar(32);not null;default:'matured'"`
	Title           string             `json:"title" gorm:"type:varchar(512);not null"`
	Summary         string             `json:"summary" gorm:"type:text;not null"`
	ContentMarkdown string             `json:"content_markdown" gorm:"type:text;not null"`
	Confidence      float64            `json:"confidence" gorm:"not null"`
	Sensitivity     string             `json:"sensitivity" gorm:"type:varchar(32);not null"`
	EvidenceRefs    EvidenceReferences `json:"evidence_refs" gorm:"type:json;not null"`
	Claims          ClaimEvidenceSet   `json:"claims" gorm:"type:json;not null"`
	ContentChecksum string             `json:"content_checksum" gorm:"type:varchar(71);not null"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

func (MemoryL3Snapshot) TableName() string { return "memory_l3_snapshots" }

func (s MemoryL3Snapshot) ProjectionKey() MemoryProjectionKey {
	return MemoryProjectionKey{TenantID: s.TenantID, TeamID: s.TeamID, BindingID: s.BindingID, MemoryID: s.MemoryID, MemoryVersion: s.MemoryVersion}
}

// MemoryReviewTask stores the immutable review payload plus its CAS version.
type MemoryReviewTask struct {
	ID                    string             `json:"id" gorm:"type:varchar(36);primaryKey"`
	SnapshotID            string             `json:"snapshot_id" gorm:"type:varchar(36);not null;uniqueIndex"`
	EventID               string             `json:"event_id" gorm:"type:varchar(128);not null;index"`
	TenantID              uint64             `json:"tenant_id" gorm:"not null;uniqueIndex:ux_memory_review_projection,priority:1"`
	TeamID                string             `json:"team_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_memory_review_projection,priority:2"`
	BindingID             string             `json:"binding_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_memory_review_projection,priority:3"`
	DepartmentID          string             `json:"department_id,omitempty" gorm:"type:varchar(128);index"`
	WorkspaceID           string             `json:"workspace_id,omitempty" gorm:"type:varchar(128);index"`
	ProjectID             string             `json:"project_id,omitempty" gorm:"type:varchar(128);index"`
	UserID                string             `json:"user_id" gorm:"type:varchar(128);not null;index"`
	AgentID               string             `json:"agent_id" gorm:"type:varchar(128);not null;index"`
	TaskID                string             `json:"task_id,omitempty" gorm:"type:varchar(128);index"`
	MemoryID              string             `json:"memory_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_memory_review_projection,priority:4"`
	MemoryVersion         uint64             `json:"memory_version" gorm:"not null;uniqueIndex:ux_memory_review_projection,priority:5"`
	TitleSnapshot         string             `json:"title_snapshot" gorm:"type:varchar(512);not null"`
	ContentSnapshot       string             `json:"content_snapshot" gorm:"type:text;not null"`
	EvidenceSnapshot      EvidenceReferences `json:"evidence_snapshot" gorm:"type:json;not null"`
	ClaimsSnapshot        ClaimEvidenceSet   `json:"claims_snapshot" gorm:"type:json;not null"`
	ContentChecksum       string             `json:"content_checksum" gorm:"type:varchar(71);not null"`
	Status                string             `json:"status" gorm:"type:varchar(32);not null;index"`
	ReviewerID            string             `json:"reviewer_id,omitempty" gorm:"type:varchar(128);index"`
	ReviewComment         string             `json:"review_comment,omitempty" gorm:"type:text"`
	ReviewedAt            *time.Time         `json:"reviewed_at,omitempty"`
	TargetKnowledgeBaseID string             `json:"target_knowledge_base_id,omitempty" gorm:"type:varchar(36);index"`
	LockVersion           uint64             `json:"lock_version" gorm:"not null;default:1"`
	CreatedAt             time.Time          `json:"created_at"`
	UpdatedAt             time.Time          `json:"updated_at"`
}

func (MemoryReviewTask) TableName() string { return "memory_review_tasks" }

func (r MemoryReviewTask) ProjectionKey() MemoryProjectionKey {
	return MemoryProjectionKey{TenantID: r.TenantID, TeamID: r.TeamID, BindingID: r.BindingID, MemoryID: r.MemoryID, MemoryVersion: r.MemoryVersion}
}

type MemoryReviewHistory struct {
	ID              string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	ReviewTaskID    string    `json:"review_task_id" gorm:"type:varchar(36);not null;index"`
	TenantID        uint64    `json:"tenant_id" gorm:"not null;index"`
	TeamID          string    `json:"team_id" gorm:"type:varchar(128);not null;index"`
	BindingID       string    `json:"binding_id" gorm:"type:varchar(128);not null;index"`
	UserID          string    `json:"user_id" gorm:"type:varchar(128);not null;index"`
	MemoryID        string    `json:"memory_id" gorm:"type:varchar(128);not null;index"`
	MemoryVersion   uint64    `json:"memory_version" gorm:"not null"`
	ContentChecksum string    `json:"content_checksum" gorm:"type:varchar(71);not null"`
	FromStatus      string    `json:"from_status" gorm:"type:varchar(32);not null"`
	ToStatus        string    `json:"to_status" gorm:"type:varchar(32);not null"`
	ActorID         string    `json:"actor_id" gorm:"type:varchar(128);not null"`
	Comment         string    `json:"comment" gorm:"type:text"`
	CreatedAt       time.Time `json:"created_at"`
}

func (MemoryReviewHistory) TableName() string { return "memory_review_histories" }

// MemoryWikiRevision is an immutable, queryable snapshot of one materialized
// Wiki page body. Multiple reviewed memory versions may point at the same
// revision when their audited content checksum is identical.
type MemoryWikiRevision struct {
	ID                  string      `json:"id" gorm:"type:varchar(128);primaryKey"`
	TenantID            uint64      `json:"tenant_id" gorm:"not null;index"`
	TeamID              string      `json:"team_id" gorm:"type:varchar(128);not null;index"`
	BindingID           string      `json:"binding_id" gorm:"type:varchar(128);not null;index"`
	UserID              string      `json:"user_id" gorm:"type:varchar(128);not null;index"`
	KnowledgeBaseID     string      `json:"knowledge_base_id" gorm:"type:varchar(36);not null;index"`
	WikiPageID          string      `json:"wiki_page_id" gorm:"type:varchar(36);not null;uniqueIndex:ux_memory_wiki_revision_page_checksum,priority:1"`
	WikiPageVersion     int         `json:"wiki_page_version" gorm:"not null;uniqueIndex:ux_memory_wiki_revision_page_checksum,priority:3"`
	PageSlug            string      `json:"page_slug" gorm:"type:varchar(512);not null;index"`
	MemoryID            string      `json:"memory_id" gorm:"type:varchar(128);not null;index"`
	MemoryVersion       uint64      `json:"memory_version" gorm:"not null"`
	SourcePublicationID string      `json:"source_publication_id" gorm:"type:varchar(36);not null;index"`
	SourceReviewTaskID  string      `json:"source_review_task_id" gorm:"type:varchar(36);not null;index"`
	ContentChecksum     string      `json:"content_checksum" gorm:"type:varchar(71);not null"`
	ProjectionChecksum  string      `json:"projection_checksum" gorm:"type:varchar(71);not null;uniqueIndex:ux_memory_wiki_revision_page_checksum,priority:2"`
	Title               string      `json:"title" gorm:"type:varchar(512);not null"`
	Summary             string      `json:"summary" gorm:"type:text;not null"`
	Content             string      `json:"content" gorm:"type:text;not null"`
	PageType            string      `json:"page_type" gorm:"type:varchar(32);not null"`
	PageStatus          string      `json:"page_status" gorm:"type:varchar(32);not null"`
	SourceRefs          StringArray `json:"source_refs" gorm:"type:json;not null"`
	ChunkRefs           StringArray `json:"chunk_refs" gorm:"type:json;not null"`
	PageMetadata        JSON        `json:"page_metadata" gorm:"type:json;not null"`
	PageSnapshot        JSON        `json:"page_snapshot" gorm:"type:json;not null"`
	CreatedAt           time.Time   `json:"created_at"`
}

func (MemoryWikiRevision) TableName() string { return "memory_wiki_revisions" }

// WikiClaimEvidence materializes claim-to-evidence provenance for one Wiki
// page revision. It never stores the raw evidence content.
type WikiClaimEvidence struct {
	ID               string             `json:"id" gorm:"type:varchar(36);primaryKey"`
	PublicationID    string             `json:"publication_id" gorm:"type:varchar(36);not null;index"`
	TenantID         uint64             `json:"tenant_id" gorm:"not null;index"`
	TeamID           string             `json:"team_id" gorm:"type:varchar(128);not null;index"`
	BindingID        string             `json:"binding_id" gorm:"type:varchar(128);not null;index"`
	UserID           string             `json:"user_id" gorm:"type:varchar(128);not null;index"`
	MemoryID         string             `json:"memory_id" gorm:"type:varchar(128);not null;index"`
	MemoryVersion    uint64             `json:"memory_version" gorm:"not null"`
	WikiPageID       string             `json:"wiki_page_id" gorm:"type:varchar(36);not null;index"`
	WikiRevisionID   string             `json:"wiki_revision_id" gorm:"type:varchar(128);not null;index"`
	ClaimID          string             `json:"claim_id" gorm:"type:varchar(128);not null"`
	ClaimText        string             `json:"claim_text" gorm:"type:text;not null"`
	WikiLocator      string             `json:"wiki_locator" gorm:"type:varchar(256);not null"`
	Factual          bool               `json:"factual" gorm:"not null"`
	EvidenceLocators EvidenceReferences `json:"evidence_locators" gorm:"type:json;not null"`
	CreatedAt        time.Time          `json:"created_at"`
}

func (WikiClaimEvidence) TableName() string { return "wiki_claim_evidences" }

// MemoryWikiPublishResult is the atomic publication checkpoint persisted only
// after the Wiki create/update succeeds.
type MemoryWikiPublishResult struct {
	KnowledgeBaseID string
	WikiPageID      string
	WikiRevisionID  string
	WikiPageVersion int
	PublishedAt     time.Time
	ClaimEvidence   []WikiClaimEvidence
}
