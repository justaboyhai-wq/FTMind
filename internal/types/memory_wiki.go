package types

import "time"

const (
	MemoryWikiPendingReview = "pending_review"
	MemoryWikiApproved      = "approved"
	MemoryWikiRejected      = "rejected"
	MemoryWikiRevoked       = "revoked"
)

// MemoryWikiPublication is the governance record for an L3 memory claim.
// Only Approved records may be materialized as a memory Wiki page; L2 is
// intentionally absent from this publication model.
type MemoryWikiPublication struct {
	ID              string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID        uint64     `json:"tenant_id" gorm:"not null;index"`
	WorkspaceID     string     `json:"workspace_id,omitempty" gorm:"type:varchar(36);index"`
	ProjectID       string     `json:"project_id,omitempty" gorm:"type:varchar(36);index"`
	MemoryID        string     `json:"memory_id" gorm:"type:varchar(128);not null;index"`
	Title           string     `json:"title" gorm:"type:varchar(255);not null"`
	Markdown        string     `json:"markdown" gorm:"type:text;not null"`
	Evidence        []string   `json:"evidence" gorm:"type:json"`
	Status          string     `json:"status" gorm:"type:varchar(32);not null;default:'pending_review';index"`
	ReviewedBy      string     `json:"reviewed_by,omitempty" gorm:"type:varchar(36)"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	PublishedPageID string     `json:"published_page_id,omitempty" gorm:"type:varchar(36)"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (MemoryWikiPublication) TableName() string { return "memory_wiki_publications" }
