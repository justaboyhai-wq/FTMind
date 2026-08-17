package repository

import (
	"context"
	"errors"
	"time"

	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"gorm.io/gorm"
)

var ErrAgentBindingKeyNotFound = errors.New("agent binding key not found")

type agentBindingKeyRepository struct{ db *gorm.DB }

func NewAgentBindingKeyRepository(db *gorm.DB) interfaces.AgentBindingKeyRepository {
	return &agentBindingKeyRepository{db: db}
}
func (r *agentBindingKeyRepository) CreateAgentBindingKey(ctx context.Context, k *types.AgentBindingKey) error {
	return r.db.WithContext(ctx).Create(k).Error
}
func (r *agentBindingKeyRepository) GetActiveAgentBindingKeyByHash(ctx context.Context, hash string) (*types.AgentBindingKey, error) {
	var k types.AgentBindingKey
	err := r.db.WithContext(ctx).Where("key_hash = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)", hash, time.Now()).First(&k).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAgentBindingKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}
func (r *agentBindingKeyRepository) RevokeAgentBindingKeys(ctx context.Context, tenantID uint64, bindingID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&types.AgentBindingKey{}).Where("tenant_id = ? AND binding_id = ? AND revoked_at IS NULL", tenantID, bindingID).Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error
}
