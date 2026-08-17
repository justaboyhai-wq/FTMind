package interfaces

import (
	"context"
	"github.com/justaboyhai-wq/fmind/internal/types"
)

type MemoryWikiPublicationRepository interface {
	ExternalMemoryRepository
	GetMemoryWikiPublication(context.Context, uint64, string) (*types.MemoryWikiPublication, error)
	ListMemoryWikiPublications(context.Context, uint64, string) ([]*types.MemoryWikiPublication, error)
}
