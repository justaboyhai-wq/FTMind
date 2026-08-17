package repository

import (
	"context"
	"errors"

	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"gorm.io/gorm"
)

var ErrMemoryWikiPublicationNotFound = errors.New("memory wiki publication not found")

type memoryWikiPublicationRepository struct{ *externalMemoryRepository }

func NewMemoryWikiPublicationRepository(db *gorm.DB) interfaces.MemoryWikiPublicationRepository {
	return &memoryWikiPublicationRepository{externalMemoryRepository: &externalMemoryRepository{db: db}}
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
