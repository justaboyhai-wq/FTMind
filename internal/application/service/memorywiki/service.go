package memorywiki

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

type Service struct {
	repo interfaces.MemoryWikiPublicationRepository
}

func NewService(repo interfaces.MemoryWikiPublicationRepository) *Service {
	return &Service{repo: repo}
}
func (s *Service) Submit(ctx context.Context, p *types.MemoryWikiPublication) error {
	if p == nil || p.TenantID == 0 || p.MemoryID == "" || p.Markdown == "" {
		return errors.New("tenant, memory id, and markdown are required")
	}
	if len(p.Evidence) == 0 {
		return errors.New("L3 evidence is required")
	}
	p.ID = uuid.NewString()
	p.Status = types.MemoryWikiPendingReview
	return s.repo.CreateMemoryWikiPublication(ctx, p)
}
func (s *Service) List(ctx context.Context, tenantID uint64, status string) ([]*types.MemoryWikiPublication, error) {
	if tenantID == 0 {
		return nil, errors.New("tenant is required")
	}
	return s.repo.ListMemoryWikiPublications(ctx, tenantID, status)
}
func (s *Service) Review(ctx context.Context, tenantID uint64, id, reviewer string, approve bool) error {
	if tenantID == 0 || reviewer == "" {
		return errors.New("tenant and reviewer are required")
	}
	status := types.MemoryWikiRejected
	if approve {
		status = types.MemoryWikiApproved
	}
	return s.repo.ReviewMemoryWikiPublication(ctx, tenantID, id, reviewer, status)
}
