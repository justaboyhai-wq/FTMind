package interfaces

import (
	"context"
	"time"

	"github.com/justaboyhai-wq/fmind/internal/types"
)

type AgentBindingCreateRequest struct {
	WorkspaceID      string
	ProjectID        string
	DepartmentID     string
	TeamID           string
	UserID           string
	UserAPIKeyID     uint64
	AgentID          string
	TaskID           string
	ExternalAgent    string
	ConnectorType    string
	CapabilityScopes []string
	AssetScopes      []string
	CaptureEnabled   bool
	RecallEnabled    bool
	L3WikiEnabled    bool
	L3ReviewRequired bool
	ExpiresAt        *time.Time
	CreatedBy        string
}
type AgentBindingCreateResult struct {
	Binding           *types.AgentBinding        `json:"binding"`
	ConnectorSecret   string                     `json:"connector_secret"`
	CredentialPurpose string                     `json:"credential_purpose"`
	SetupExpiresAt    *time.Time                 `json:"setup_expires_at,omitempty"`
	SetupManifest     *AgentBindingSetupManifest `json:"setup_manifest,omitempty"`
	SetupPrompt       string                     `json:"setup_prompt,omitempty"`
}

type AgentBindingSetupManifest struct {
	BindingID           string   `json:"binding_id"`
	ExternalAgent       string   `json:"external_agent"`
	ConnectorType       string   `json:"connector_type"`
	FMindEndpoint       string   `json:"fmind_endpoint"`
	MemoryCoreEndpoint  string   `json:"memory_core_endpoint,omitempty"`
	MemoryProxyEndpoint string   `json:"memory_proxy_endpoint,omitempty"`
	Capabilities        []string `json:"capabilities"`
	AssetScopes         []string `json:"asset_scopes"`
}

type AgentBindingSetupRequest struct {
	BindingID       string `json:"binding_id"`
	ExternalAgent   string `json:"external_agent"`
	ConnectorType   string `json:"connector_type"`
	ClientVersion   string `json:"client_version,omitempty"`
	ConnectorSecret string `json:"-"`
	UserAPIKey      string `json:"-"`
	UserAPIKeyID    uint64 `json:"-"`
}

type AgentBindingSetupResult struct {
	BindingID           string     `json:"binding_id"`
	Status              string     `json:"status"`
	ConnectorSecret     string     `json:"connector_secret"`
	MemoryAccessKey     string     `json:"memory_access_key,omitempty"`
	FMindEndpoint       string     `json:"fmind_endpoint"`
	MemoryCoreEndpoint  string     `json:"memory_core_endpoint,omitempty"`
	MemoryProxyEndpoint string     `json:"memory_proxy_endpoint,omitempty"`
	PolicyVersion       uint64     `json:"policy_version"`
	ExpiresAt           *time.Time `json:"expires_at,omitempty"`
}

type AgentBindingSetupStatus struct {
	BindingID       string     `json:"binding_id"`
	Status          string     `json:"status"`
	SetupExpiresAt  *time.Time `json:"setup_expires_at,omitempty"`
	ActivatedAt     *time.Time `json:"activated_at,omitempty"`
	LastHandshakeAt *time.Time `json:"last_handshake_at,omitempty"`
	SetupAttempts   int        `json:"setup_attempts"`
}

// AgentBindingSetupService is optional to preserve compatibility with older
// service fakes and integrations while the setup flow is rolled out.
type AgentBindingSetupService interface {
	Setup(context.Context, AgentBindingSetupRequest) (*AgentBindingSetupResult, error)
	SetupStatus(context.Context, string) (*AgentBindingSetupStatus, error)
}

type AgentBindingSetupRotationService interface {
	RotateSetupKey(context.Context, string, string) (*AgentBindingCreateResult, error)
}

type AgentBindingService interface {
	Create(context.Context, AgentBindingCreateRequest) (*AgentBindingCreateResult, error)
	List(context.Context) ([]*types.AgentBinding, error)
	Get(context.Context, string) (*types.AgentBinding, error)
	Revoke(context.Context, string) error
	RotateKey(context.Context, string, string) (string, error)
	Introspect(context.Context, string) (*types.BindingIntrospectionResult, error)
	VerifyBindingToken(context.Context, string) (*types.BindingContext, error)
}
