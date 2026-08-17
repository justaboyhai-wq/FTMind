package repository

import (
	"context"
	"errors"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"gorm.io/gorm"
	"time"
)

var ErrMemoryWikiPublicationNotFound = errors.New("memory wiki publication not found")

type memoryWikiPublicationRepository struct{ db *gorm.DB }

func NewMemoryWikiPublicationRepository(db *gorm.DB) interfaces.MemoryWikiPublicationRepository {
	return &memoryWikiPublicationRepository{db: db}
}
func (r *memoryWikiPublicationRepository) CreateMemoryWikiPublication(ctx context.Context, p *types.MemoryWikiPublication) error {
	return r.db.WithContext(ctx).Create(p).Error
}
func (r *memoryWikiPublicationRepository) GetMemoryWikiPublication(ctx context.Context, tenantID uint64, id string) (*types.MemoryWikiPublication, error) {
	var p types.MemoryWikiPublication
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMemoryWikiPublicationNotFound
	}
	return &p, err
}
func (r *memoryWikiPublicationRepository) ListMemoryWikiPublications(ctx context.Context, tenantID uint64, status string) ([]*types.MemoryWikiPublication, error) {
	var out []*types.MemoryWikiPublication
	q := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Order("created_at DESC").Find(&out).Error
	return out, err
}
func (r *memoryWikiPublicationRepository) ReviewMemoryWikiPublication(ctx context.Context, tenantID uint64, id, reviewer, status string) error {
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&types.MemoryWikiPublication{}).Where("tenant_id = ? AND id = ? AND status = ?", tenantID, id, types.MemoryWikiPendingReview).Updates(map[string]any{"status": status, "reviewed_by": reviewer, "reviewed_at": now, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrMemoryWikiPublicationNotFound
	}
	return nil
}

func (r *memoryWikiPublicationRepository) MarkMemoryWikiPublished(ctx context.Context, tenantID uint64, id, pageID string) error {
	res := r.db.WithContext(ctx).Model(&types.MemoryWikiPublication{}).Where("tenant_id = ? AND id = ? AND status = ?", tenantID, id, types.MemoryWikiApproved).Updates(map[string]any{"published_page_id": pageID, "updated_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrMemoryWikiPublicationNotFound
	}
	return nil
}
