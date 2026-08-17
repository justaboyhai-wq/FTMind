package types

import "time"

// AgentBindingKey stores only a digest of a connector secret. The plaintext
// is returned exactly once by the management service and is never serialized.
type AgentBindingKey struct {
	ID         string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	BindingID  string     `json:"binding_id" gorm:"type:varchar(36);not null;index"`
	TenantID   uint64     `json:"tenant_id" gorm:"not null;index"`
	KeyPrefix  string     `json:"key_prefix" gorm:"type:varchar(24);not null;index"`
	KeyHash    string     `json:"-" gorm:"type:varchar(64);not null;uniqueIndex"`
	CreatedBy  string     `json:"created_by,omitempty" gorm:"type:varchar(36)"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (AgentBindingKey) TableName() string { return "agent_binding_keys" }
