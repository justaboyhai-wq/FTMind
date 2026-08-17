package repository

import (
	"context"
	"errors"

	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"gorm.io/gorm"
)

var ErrAgentBindingNotFound = errors.New("agent binding not found")

type agentBindingRepository struct{ db *gorm.DB }

func NewAgentBindingRepository(db *gorm.DB) interfaces.AgentBindingRepository {
	return &agentBindingRepository{db: db}
}

func (r *agentBindingRepository) CreateAgentBinding(ctx context.Context, b *types.AgentBinding) error {
	return r.db.WithContext(ctx).Create(b).Error
}

func (r *agentBindingRepository) GetAgentBinding(ctx context.Context, tenantID uint64, id string) (*types.AgentBinding, error) {
	var b types.AgentBinding
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&b).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAgentBindingNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *agentBindingRepository) ListAgentBindings(ctx context.Context, tenantID uint64) ([]*types.AgentBinding, error) {
	var bindings []*types.AgentBinding
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&bindings).Error
	return bindings, err
}

func (r *agentBindingRepository) FindActiveAgentBinding(ctx context.Context, tenantID uint64, externalAgent, connectorType string) (*types.AgentBinding, error) {
	var b types.AgentBinding
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND external_agent = ? AND connector_type = ? AND status = ?", tenantID, externalAgent, connectorType, types.AgentBindingStatusActive).First(&b).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAgentBindingNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *agentBindingRepository) UpdateAgentBinding(ctx context.Context, b *types.AgentBinding) error {
	return r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", b.TenantID, b.ID).Updates(b).Error
}

func (r *agentBindingRepository) RevokeAgentBinding(ctx context.Context, tenantID uint64, id string) error {
	result := r.db.WithContext(ctx).Model(&types.AgentBinding{}).Where("tenant_id = ? AND id = ?", tenantID, id).Update("status", types.AgentBindingStatusRevoked)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAgentBindingNotFound
	}
	return nil
}
