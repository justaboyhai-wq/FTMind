package types

import "time"

// AgentBinding connects an external agent runtime to an FMind agent and an
// organization scope. ConnectorSecret is intentionally not persisted here;
// long-lived secrets belong in the secret store and are exchanged for a
// short-lived binding token by the binding service.
type AgentBinding struct {
	ID               string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID         uint64     `json:"tenant_id" gorm:"not null;index"`
	DepartmentID     string     `json:"department_id,omitempty" gorm:"type:varchar(36);index"`
	WorkspaceID      string     `json:"workspace_id,omitempty" gorm:"type:varchar(36);index"`
	ProjectID        string     `json:"project_id,omitempty" gorm:"type:varchar(36);index"`
	AgentID          string     `json:"agent_id" gorm:"type:varchar(36);not null;index"`
	ExternalAgent    string     `json:"external_agent" gorm:"type:varchar(128);not null"`
	ConnectorType    string     `json:"connector_type" gorm:"type:varchar(64);not null"`
	Status           string     `json:"status" gorm:"type:varchar(32);not null;default:'active';index"`
	CapabilityScopes []string   `json:"capability_scopes" gorm:"type:json"`
	AssetScopes      []string   `json:"asset_scopes" gorm:"type:json"`
	CreatedBy        string     `json:"created_by,omitempty" gorm:"type:varchar(36)"`
	LastUsedAt       *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

func (AgentBinding) TableName() string { return "agent_bindings" }

const (
	AgentBindingStatusActive  = "active"
	AgentBindingStatusRevoked = "revoked"
)
