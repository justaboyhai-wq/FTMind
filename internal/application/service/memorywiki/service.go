package memorywiki

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"strings"
)

type Service struct {
	repo interfaces.MemoryWikiPublicationRepository
	wiki interfaces.WikiPageService
	kb   interfaces.KnowledgeBaseService
}

func NewService(repo interfaces.MemoryWikiPublicationRepository, wiki interfaces.WikiPageService, kb interfaces.KnowledgeBaseService) *Service {
	return &Service{repo: repo, wiki: wiki, kb: kb}
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

func (s *Service) PublishApproved(ctx context.Context, tenantID uint64, id, knowledgeBaseID string) (*types.WikiPage, error) {
	if tenantID == 0 || knowledgeBaseID == "" {
		return nil, errors.New("tenant and memory wiki knowledge base are required")
	}
	p, err := s.repo.GetMemoryWikiPublication(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if p.Status != types.MemoryWikiApproved {
		return nil, errors.New("memory wiki publication requires approved L3 status")
	}
	kb, err := s.kb.GetKnowledgeBaseByID(ctx, knowledgeBaseID)
	if err != nil || kb == nil || kb.TenantID != tenantID || kb.Type != types.KnowledgeBaseTypeWiki || kb.WikiConfig == nil || !kb.WikiConfig.IsMemoryWiki {
		return nil, errors.New("target knowledge base must be a tenant-scoped memory wiki")
	}
	slug := "memory/" + strings.Trim(strings.ToLower(p.MemoryID), "/")
	page, err := s.wiki.CreatePage(ctx, &types.WikiPage{TenantID: tenantID, KnowledgeBaseID: knowledgeBaseID, Slug: slug, Title: p.Title, PageType: types.WikiPageTypeSummary, Status: types.WikiPageStatusPublished, Content: p.Markdown, Summary: p.Title, SourceRefs: types.StringArray{"memory:" + p.MemoryID}})
	if err != nil {
		return nil, err
	}
	if err := s.repo.MarkMemoryWikiPublished(ctx, tenantID, id, page.ID); err != nil {
		return nil, err
	}
	return page, nil
}
