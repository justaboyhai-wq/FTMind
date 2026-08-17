package types

import "time"

type BindingContext struct {
	TokenID          string    `json:"token_id"`
	BindingID        string    `json:"binding_id"`
	TenantID         uint64    `json:"tenant_id"`
	DepartmentID     string    `json:"department_id,omitempty"`
	WorkspaceID      string    `json:"workspace_id,omitempty"`
	ProjectID        string    `json:"project_id,omitempty"`
	AgentID          string    `json:"agent_id"`
	TaskID           string    `json:"task_id,omitempty"`
	CapabilityScopes []string  `json:"capability_scopes"`
	AssetScopes      []string  `json:"asset_scopes"`
	CaptureEnabled   bool      `json:"capture_enabled"`
	RecallEnabled    bool      `json:"recall_enabled"`
	L3ReviewRequired bool      `json:"l3_review_required"`
	PolicyVersion    uint64    `json:"policy_version"`
	ExpiresAt        time.Time `json:"expires_at"`
}
type BindingIntrospectionResult struct {
	BindingToken string         `json:"binding_token"`
	Context      BindingContext `json:"context"`
}
