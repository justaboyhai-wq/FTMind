package interfaces

import (
	"context"
	"time"

	"github.com/justaboyhai-wq/fmind/internal/types"
)

type AgentBindingKeyRepository interface {
	CreateAgentBindingKey(context.Context, *types.AgentBindingKey) error
	GetActiveAgentBindingKeyByHash(context.Context, string) (*types.AgentBindingKey, error)
	RevokeAgentBindingKeys(context.Context, uint64, string) error
	TouchAgentBindingKey(context.Context, string, time.Time) error
}
