package interfaces

import (
	"context"
	"time"

	"github.com/justaboyhai-wq/fmind/internal/types"
)

// AgentBindingRepository persists external-agent bindings with tenant scope.
type AgentBindingRepository interface {
	CreateAgentBinding(context.Context, *types.AgentBinding) error
	CreateAgentBindingWithKey(context.Context, *types.AgentBinding, *types.AgentBindingKey) error
	ResolveActiveKeyAndBinding(context.Context, string, time.Time) (*types.AgentBindingKey, *types.AgentBinding, error)
	GetAgentBinding(context.Context, uint64, string) (*types.AgentBinding, error)
	ListAgentBindings(context.Context, uint64) ([]*types.AgentBinding, error)
	FindActiveAgentBinding(context.Context, uint64, string, string) (*types.AgentBinding, error)
	UpdateAgentBinding(context.Context, *types.AgentBinding) error
	TouchAgentBinding(context.Context, uint64, string, time.Time) error
	RevokeAgentBinding(context.Context, uint64, string) error
	RotateAgentBindingKey(context.Context, uint64, string, *types.AgentBindingKey) error
	RevokeAgentBindingWithKeys(context.Context, uint64, string) error
}

type AgentBindingSetupRepository interface {
	CompleteSetup(context.Context, string, string, string, string, uint64, *types.AgentBindingKey, *types.AgentBindingKey, time.Time) (*types.AgentBinding, error)
}

// AgentBindingScopeValidator verifies tenant-owned managed namespaces and
// resolves roles from authoritative server-side membership state.
type AgentBindingScopeValidator interface {
	ValidateCreate(context.Context, *types.AgentBinding) (types.StringArray, error)
	ResolveRoles(context.Context, *types.AgentBinding) (types.StringArray, error)
}
