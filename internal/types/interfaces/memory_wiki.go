package interfaces

import (
	"context"
	"github.com/justaboyhai-wq/fmind/internal/types"
)

type MemoryWikiPublicationRepository interface {
	CreateMemoryWikiPublication(context.Context, *types.MemoryWikiPublication) error
	GetMemoryWikiPublication(context.Context, uint64, string) (*types.MemoryWikiPublication, error)
	ListMemoryWikiPublications(context.Context, uint64, string) ([]*types.MemoryWikiPublication, error)
	ReviewMemoryWikiPublication(context.Context, uint64, string, string, string) error
}
