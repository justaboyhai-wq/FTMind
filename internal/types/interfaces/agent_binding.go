package interfaces

import (
	"context"
	"github.com/justaboyhai-wq/fmind/internal/types"
)

// AgentBindingRepository persists external-agent bindings with tenant scope.
type AgentBindingRepository interface {
	CreateAgentBinding(context.Context, *types.AgentBinding) error
	GetAgentBinding(context.Context, uint64, string) (*types.AgentBinding, error)
	ListAgentBindings(context.Context, uint64) ([]*types.AgentBinding, error)
	FindActiveAgentBinding(context.Context, uint64, string, string) (*types.AgentBinding, error)
	UpdateAgentBinding(context.Context, *types.AgentBinding) error
	RevokeAgentBinding(context.Context, uint64, string) error
}
