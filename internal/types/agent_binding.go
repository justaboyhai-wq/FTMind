package types

import (
	"time"

	"gorm.io/gorm"
)

// AgentBinding connects an external agent runtime to an FTMind agent and an
// organization scope. ConnectorSecret is intentionally not persisted here;
// long-lived secrets belong in the secret store and are exchanged for a
// short-lived binding token by the binding service.
type AgentBinding struct {
	ID               string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID         uint64         `json:"tenant_id" gorm:"not null;index"`
	DepartmentID     string         `json:"department_id,omitempty" gorm:"type:varchar(36);index"`
	TeamID           string         `json:"team_id" gorm:"type:varchar(36);not null;index"`
	WorkspaceID      string         `json:"workspace_id,omitempty" gorm:"type:varchar(36);index"`
	ProjectID        string         `json:"project_id,omitempty" gorm:"type:varchar(36);index"`
	UserID           string         `json:"user_id" gorm:"type:varchar(36);not null;index"`
	UserAPIKeyID     uint64         `json:"user_api_key_id,omitempty" gorm:"index"`
	AgentID          string         `json:"agent_id" gorm:"type:varchar(36);not null;index"`
	TaskID           string         `json:"task_id,omitempty" gorm:"type:varchar(64);index"`
	ExternalAgent    string         `json:"external_agent" gorm:"type:varchar(128);not null"`
	ConnectorType    string         `json:"connector_type" gorm:"type:varchar(64);not null"`
	Status           string         `json:"status" gorm:"type:varchar(32);not null;default:'active';index"`
	SetupExpiresAt   *time.Time     `json:"setup_expires_at,omitempty"`
	ActivatedAt      *time.Time     `json:"activated_at,omitempty"`
	LastHandshakeAt  *time.Time     `json:"last_handshake_at,omitempty"`
	SetupAttempts    int            `json:"setup_attempts" gorm:"not null;default:0"`
	CaptureEnabled   bool           `json:"capture_enabled" gorm:"not null;default:false"`
	RecallEnabled    bool           `json:"recall_enabled" gorm:"not null;default:false"`
	L3WikiEnabled    bool           `json:"l3_wiki_enabled" gorm:"not null;default:false"`
	L3ReviewRequired bool           `json:"l3_review_required" gorm:"not null;default:true"`
	CapabilityScopes StringArray    `json:"capability_scopes" gorm:"type:json"`
	AssetScopes      StringArray    `json:"asset_scopes" gorm:"type:json"`
	PolicyVersion    uint64         `json:"policy_version" gorm:"not null;default:1"`
	CreatedBy        string         `json:"created_by,omitempty" gorm:"type:varchar(36)"`
	LastUsedAt       *time.Time     `json:"last_used_at,omitempty"`
	ExpiresAt        *time.Time     `json:"expires_at,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

func (AgentBinding) TableName() string { return "agent_bindings" }

const (
	AgentBindingStatusPendingSetup = "pending_setup"
	AgentBindingStatusActive       = "active"
	AgentBindingStatusRevoked      = "revoked"
)
