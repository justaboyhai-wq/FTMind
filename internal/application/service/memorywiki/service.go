package memorywiki

import (
	"context"
	"errors"
	"strings"

	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

var (
	ErrTrustedL3EventRequired      = errors.New("memory Wiki submissions require the trusted L3 event intake")
	ErrMemoryReviewNotApproved     = errors.New("memory Wiki publication requires approved L3 status")
	ErrInvalidMemoryWikiTarget     = errors.New("target knowledge base must match the tenant and team memory Wiki")
	ErrMemoryClaimEvidenceRequired = errors.New("every factual memory claim requires valid evidence")
	ErrMemoryClaimSourceMismatch   = errors.New("memory claim must be safe, unique, and present in the audited source snapshot")
	ErrStaleMemoryWikiVersion      = errors.New("a newer memory version is already published")
	ErrMemoryWikiReviewerRequired  = errors.New("memory Wiki review requires tenant admin")
	ErrMemorySourceRevoked         = errors.New("source memory was revoked")
)

type memoryWikiPageGateway interface {
	GetPageBySlug(context.Context, string, string) (*types.WikiPage, error)
	CreatePage(context.Context, *types.WikiPage) (*types.WikiPage, error)
	UpdatePage(context.Context, *types.WikiPage) (*types.WikiPage, error)
	UpdatePageMeta(context.Context, *types.WikiPage) error
}

type knowledgeBaseReader interface {
	GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error)
	ListKnowledgeBasesByTenantID(context.Context, uint64) ([]*types.KnowledgeBase, error)
	CreateKnowledgeBase(context.Context, *types.KnowledgeBase) (*types.KnowledgeBase, error)
}

type Service struct {
	repo interfaces.MemoryWikiPublicationRepository
	wiki memoryWikiPageGateway
	kb   knowledgeBaseReader
}

func NewService(repo interfaces.MemoryWikiPublicationRepository, wiki interfaces.WikiPageService, kb interfaces.KnowledgeBaseService) *Service {
	return newService(repo, wiki, kb)
}

func newService(repo interfaces.MemoryWikiPublicationRepository, wiki memoryWikiPageGateway, kb knowledgeBaseReader) *Service {
	return &Service{repo: repo, wiki: wiki, kb: kb}
}

// Submit is intentionally closed: public callers cannot construct governance
// rows or choose an approved status. Authenticated internal event intake must
// call ReceiveTrustedL3Event instead.
func (s *Service) Submit(context.Context, *types.MemoryWikiPublication) error {
	return ErrTrustedL3EventRequired
}

func (s *Service) List(ctx context.Context, tenantID uint64, status string) ([]*types.MemoryWikiPublication, error) {
	if tenantID == 0 {
		return nil, errors.New("tenant is required")
	}
	if err := requireMemoryWikiReviewer(ctx, tenantID); err != nil {
		return nil, err
	}
	return s.repo.ListMemoryWikiPublications(ctx, tenantID, status)
}

func (s *Service) GetReview(ctx context.Context, tenantID uint64, id string) (*interfaces.ExternalMemoryProjection, error) {
	if tenantID == 0 || strings.TrimSpace(id) == "" {
		return nil, errors.New("tenant and review id are required")
	}
	if err := requireMemoryWikiReviewer(ctx, tenantID); err != nil {
		return nil, err
	}
	publication, err := s.repo.GetMemoryWikiPublication(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	return s.repo.GetMemoryProjection(ctx, publicationProjectionKey(publication))
}

func (s *Service) Review(ctx context.Context, tenantID uint64, id, reviewer string, approve bool) error {
	if tenantID == 0 || reviewer == "" {
		return errors.New("tenant and reviewer are required")
	}
	if err := requireMemoryWikiReviewer(ctx, tenantID); err != nil {
		return err
	}
	publication, err := s.repo.GetMemoryWikiPublication(ctx, tenantID, id)
	if err != nil {
		return err
	}
	status := types.MemoryReviewStatusRejected
	if approve {
		status = types.MemoryReviewStatusApproved
	}
	_, err = s.repo.TransitionMemoryReview(
		ctx, publicationProjectionKey(publication), publication.ReviewTaskID,
		types.MemoryReviewStatusPendingReview, status, reviewer, "",
	)
	return err
}

func (s *Service) Approve(ctx context.Context, key types.MemoryProjectionKey, reviewTaskID, reviewer, comment string) (*types.MemoryReviewTask, error) {
	if key.TenantID == 0 || reviewer == "" {
		return nil, errors.New("tenant and reviewer are required")
	}
	if err := requireMemoryWikiReviewer(ctx, key.TenantID); err != nil {
		return nil, err
	}
	return s.repo.TransitionMemoryReview(
		ctx, key, reviewTaskID, types.MemoryReviewStatusPendingReview,
		types.MemoryReviewStatusApproved, reviewer, comment,
	)
}

func (s *Service) RequestChanges(ctx context.Context, key types.MemoryProjectionKey, reviewTaskID, reviewer, comment string) (*types.MemoryReviewTask, error) {
	if key.TenantID == 0 || strings.TrimSpace(reviewer) == "" || strings.TrimSpace(comment) == "" {
		return nil, errors.New("tenant, reviewer, and review comment are required")
	}
	if err := requireMemoryWikiReviewer(ctx, key.TenantID); err != nil {
		return nil, err
	}
	return s.repo.TransitionMemoryReview(
		ctx, key, reviewTaskID, types.MemoryReviewStatusPendingReview,
		types.MemoryReviewStatusChangesRequested, reviewer, comment,
	)
}

func (s *Service) ResubmitChanges(ctx context.Context, key types.MemoryProjectionKey, reviewTaskID, actor, comment string) (*types.MemoryReviewTask, error) {
	if key.TenantID == 0 || strings.TrimSpace(actor) == "" || strings.TrimSpace(comment) == "" {
		return nil, errors.New("tenant, actor, and revision comment are required")
	}
	if err := requireMemoryWikiReviewer(ctx, key.TenantID); err != nil {
		return nil, err
	}
	return s.repo.TransitionMemoryReview(
		ctx, key, reviewTaskID, types.MemoryReviewStatusChangesRequested,
		types.MemoryReviewStatusPendingReview, actor, comment,
	)
}

func (s *Service) ApprovePublication(ctx context.Context, tenantID uint64, id, reviewer, comment string) (*types.MemoryReviewTask, error) {
	return s.transitionPublicationReview(ctx, tenantID, id, reviewer, comment, types.MemoryReviewStatusApproved)
}

func (s *Service) RejectPublication(ctx context.Context, tenantID uint64, id, reviewer, comment string) (*types.MemoryReviewTask, error) {
	return s.transitionPublicationReview(ctx, tenantID, id, reviewer, comment, types.MemoryReviewStatusRejected)
}

func (s *Service) RequestPublicationChanges(ctx context.Context, tenantID uint64, id, reviewer, comment string) (*types.MemoryReviewTask, error) {
	if strings.TrimSpace(comment) == "" {
		return nil, errors.New("review comment is required")
	}
	return s.transitionPublicationReview(ctx, tenantID, id, reviewer, comment, types.MemoryReviewStatusChangesRequested)
}

func (s *Service) transitionPublicationReview(ctx context.Context, tenantID uint64, id, reviewer, comment, status string) (*types.MemoryReviewTask, error) {
	if tenantID == 0 || strings.TrimSpace(id) == "" || strings.TrimSpace(reviewer) == "" {
		return nil, errors.New("tenant, review id, and reviewer are required")
	}
	if err := requireMemoryWikiReviewer(ctx, tenantID); err != nil {
		return nil, err
	}
	publication, err := s.repo.GetMemoryWikiPublication(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	return s.repo.TransitionMemoryReview(
		ctx, publicationProjectionKey(publication), publication.ReviewTaskID,
		types.MemoryReviewStatusPendingReview, status, reviewer, strings.TrimSpace(comment),
	)
}

func requireMemoryWikiReviewer(ctx context.Context, tenantID uint64) error {
	if types.IsSystemAdminFromContext(ctx) {
		return nil
	}
	contextTenantID, ok := types.TenantIDFromContext(ctx)
	if ok && contextTenantID != 0 && contextTenantID == tenantID &&
		types.TenantRoleFromContext(ctx).HasPermission(types.TenantRoleAdmin) {
		return nil
	}
	return ErrMemoryWikiReviewerRequired
}
