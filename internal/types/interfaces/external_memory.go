package interfaces

import (
	"context"

	"github.com/justaboyhai-wq/fmind/internal/types"
)

type ExternalMemoryProjection struct {
	Event       *types.MemoryIntegrationEvent
	Snapshot    *types.MemoryL3Snapshot
	ReviewTask  *types.MemoryReviewTask
	Publication *types.MemoryWikiPublication
}

// ExternalMemoryRepository owns the durable, tenant/team/binding-scoped L3
// projection and its review state machine.
type ExternalMemoryRepository interface {
	CreateMaturedMemoryProjection(
		context.Context,
		*types.MemoryIntegrationEvent,
		*types.MemoryL3Snapshot,
		*types.MemoryReviewTask,
		*types.MemoryWikiPublication,
	) (*ExternalMemoryProjection, bool, error)
	RevokeMemoryProjection(
		context.Context,
		*types.MemoryIntegrationEvent,
	) (*ExternalMemoryProjection, bool, error)
	GetMemoryProjection(context.Context, types.MemoryProjectionKey) (*ExternalMemoryProjection, error)
	TransitionMemoryReview(
		context.Context,
		types.MemoryProjectionKey,
		string,
		string,
		string,
		string,
		string,
	) (*types.MemoryReviewTask, error)
	StartMemoryWikiPublishing(context.Context, types.MemoryProjectionKey, string, string) (*types.MemoryWikiPublication, error)
	FailMemoryWikiPublishing(context.Context, types.MemoryProjectionKey, string, string, string) error
	CompleteMemoryWikiPublishing(context.Context, types.MemoryProjectionKey, string, types.MemoryWikiPublishResult) (*types.MemoryWikiPublication, error)
	CreateMemoryWikiRevision(context.Context, *types.MemoryWikiRevision) (*types.MemoryWikiRevision, bool, error)
	GetMemoryWikiRevision(context.Context, uint64, string) (*types.MemoryWikiRevision, error)
	GetMemoryWikiRevisionByPageChecksum(context.Context, uint64, string, string) (*types.MemoryWikiRevision, error)
	ListMemoryWikiPublicationsByRevision(context.Context, uint64, string) ([]*types.MemoryWikiPublication, error)
}
