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
	Binding         *types.AgentBinding `json:"binding"`
	ConnectorSecret string              `json:"connector_secret"`
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
