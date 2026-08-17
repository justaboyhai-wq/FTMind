package types

import "time"

type BindingContext struct {
	TokenID          string      `json:"token_id"`
	BindingID        string      `json:"binding_id"`
	TenantID         uint64      `json:"tenant_id"`
	DepartmentID     string      `json:"department_id,omitempty"`
	TeamID           string      `json:"team_id"`
	WorkspaceID      string      `json:"workspace_id,omitempty"`
	ProjectID        string      `json:"project_id,omitempty"`
	UserID           string      `json:"user_id"`
	AgentID          string      `json:"agent_id"`
	TaskID           string      `json:"task_id,omitempty"`
	ConnectorType    string      `json:"connector_type"`
	Roles            StringArray `json:"roles"`
	CapabilityScopes StringArray `json:"capability_scopes"`
	AssetScopes      StringArray `json:"asset_scopes"`
	CaptureEnabled   bool        `json:"capture_enabled"`
	RecallEnabled    bool        `json:"recall_enabled"`
	L3WikiEnabled    bool        `json:"l3_wiki_enabled"`
	L3ReviewRequired bool        `json:"l3_review_required"`
	PolicyVersion    uint64      `json:"policy_version"`
	ExpiresAt        time.Time   `json:"expires_at"`
}
type BindingIntrospectionResult struct {
	BindingToken string         `json:"binding_token"`
	Context      BindingContext `json:"context"`
}

const ContextPackageSchemaVersion = "fmind.context/v1"

// ContextPackage keeps the memory and knowledge retrieval planes separate.
// Consumers may render the sections together, but provenance, permissions,
// budgets, and storage identities remain independently auditable.
type ContextPackage struct {
	SchemaVersion       string                      `json:"schema_version"`
	BindingID           string                      `json:"binding_id"`
	PolicyVersion       uint64                      `json:"policy_version"`
	TraceID             string                      `json:"trace_id,omitempty"`
	Memory              ContextSection              `json:"memory"`
	RAG                 ContextSection              `json:"rag"`
	Wiki                ContextSection              `json:"wiki"`
	Raw                 ContextSection              `json:"raw"`
	Skill               ContextSection              `json:"skill"`
	Budgets             map[string]ContextBudget    `json:"budgets"`
	Provenance          []ContextProvenance         `json:"provenance"`
	Conflicts           []ContextConflict           `json:"conflicts"`
	Warnings            []string                    `json:"warnings"`
	PermissionDecisions []ContextPermissionDecision `json:"permission_decisions"`
	UsedAssets          []ContextAssetVersion       `json:"used_assets"`
	Partial             bool                        `json:"partial"`
}

type ContextSection struct {
	Items      []ContextItem `json:"items"`
	UsedTokens int           `json:"used_tokens"`
}

type ContextItem struct {
	ID      string  `json:"id"`
	Type    string  `json:"type"`
	Title   string  `json:"title,omitempty"`
	Content string  `json:"content"`
	Score   float64 `json:"score,omitempty"`
	Version string  `json:"version,omitempty"`
}

type ContextBudget struct {
	MaxTokens  int `json:"max_tokens"`
	UsedTokens int `json:"used_tokens"`
}

type ContextProvenance struct {
	Section  string `json:"section"`
	Asset    string `json:"asset"`
	SourceID string `json:"source_id"`
	Version  string `json:"version,omitempty"`
}

type ContextConflict struct {
	LeftID  string `json:"left_id"`
	RightID string `json:"right_id"`
	Reason  string `json:"reason"`
}

type ContextPermissionDecision struct {
	Tool       string `json:"tool"`
	AssetScope string `json:"asset_scope,omitempty"`
	Allowed    bool   `json:"allowed"`
	Reason     string `json:"reason,omitempty"`
}

type ContextAssetVersion struct {
	AssetScope string `json:"asset_scope"`
	Version    string `json:"version,omitempty"`
}
