package authz

import (
	"context"
	"errors"
	"github.com/justaboyhai-wq/fmind/internal/types"
)

type Action string

// Memory capabilities are intentionally namespaced so they cannot be
// confused with tenant-role actions or ordinary knowledge-base scopes.
const (
	CapabilityMemoryContext = "memory.context"
	CapabilityMemoryCapture = "memory.capture"
	CapabilityMemoryRecall  = "memory.recall"
	CapabilityMemoryConfirm = "memory.confirm"
	CapabilityMemoryReview  = "memory.review"
	CapabilityMemoryPublish = "memory.publish"
	CapabilityMemoryRevoke  = "memory.revoke"
	CapabilityWikiGet       = "wiki.get"
)

const (
	ActionRead    Action = "read"
	ActionCreate  Action = "create"
	ActionUpdate  Action = "update"
	ActionDelete  Action = "delete"
	ActionSearch  Action = "search"
	ActionCapture Action = "capture"
	ActionRecall  Action = "recall"
	ActionConfirm Action = "confirm"
	ActionContext Action = "context"
	ActionReview  Action = "review"
	ActionPublish Action = "publish"
	ActionRevoke  Action = "revoke"
	ActionShare   Action = "share"
	ActionManage  Action = "manage"
)

type Resource struct {
	Type         string
	ID           string
	TenantID     uint64
	DepartmentID string
	TeamID       string
	Status       string
}

type AccessContext struct {
	UserID           string
	TenantID         uint64
	TenantRole       types.TenantRole
	DepartmentIDs    []string
	TeamIDs          []string
	TeamRoles        map[string]string
	AgentIDs         []string
	KnowledgeBaseIDs []string
	MemoryWikiIDs    []string
	Capabilities     []string
	AssetScopes      []string
	UserAPIKeyID     uint64
	BindingID        string
	PolicyVersion    uint64
}

type contextKey struct{}

var accessContextKey contextKey
var ErrMissingContext = errors.New("authorization context is missing")

func WithContext(ctx context.Context, access AccessContext) context.Context {
	return context.WithValue(ctx, accessContextKey, access)
}
func FromContext(ctx context.Context) (AccessContext, error) {
	v, ok := ctx.Value(accessContextKey).(AccessContext)
	if !ok {
		return AccessContext{}, ErrMissingContext
	}
	return v, nil
}
func (a AccessContext) HasTeam(id string) bool {
	if a.TenantRole == types.TenantRoleOwner || a.TenantRole == types.TenantRoleAdmin {
		return id != ""
	}
	for _, v := range a.TeamIDs {
		if v == id {
			return true
		}
	}
	return false
}
func (a AccessContext) HasCapability(capability string) bool {
	for _, v := range a.Capabilities {
		if v == capability {
			return true
		}
	}
	return false
}

func (a AccessContext) HasAssetScope(scope string) bool {
	if scope == "" || len(a.AssetScopes) == 0 {
		return len(a.AssetScopes) == 0
	}
	for _, value := range a.AssetScopes {
		if value == scope {
			return true
		}
	}
	return false
}
