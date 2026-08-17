package interfaces

import (
	"context"
	"github.com/justaboyhai-wq/fmind/internal/types"
)

type AgentBindingCreateRequest struct {
	WorkspaceID      string
	ProjectID        string
	DepartmentID     string
	AgentID          string
	ExternalAgent    string
	ConnectorType    string
	CapabilityScopes []string
	AssetScopes      []string
	CreatedBy        string
}
type AgentBindingCreateResult struct {
	Binding         *types.AgentBinding
	ConnectorSecret string
}

type AgentBindingService interface {
	Create(context.Context, AgentBindingCreateRequest) (*AgentBindingCreateResult, error)
	List(context.Context) ([]*types.AgentBinding, error)
	Get(context.Context, string) (*types.AgentBinding, error)
	Revoke(context.Context, string) error
	RotateKey(context.Context, string, string) (string, error)
}
