package repository

import (
	"context"
	"errors"
	"time"

	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrAgentBindingNotFound              = errors.New("agent binding not found")
	ErrAgentBindingPolicyVersionOverflow = errors.New("agent binding policy version cannot be incremented")
)

const maxAgentBindingPolicyVersion = uint64(1<<63 - 1)

type agentBindingRepository struct{ db *gorm.DB }

func NewAgentBindingRepository(db *gorm.DB) interfaces.AgentBindingRepository {
	return &agentBindingRepository{db: db}
}

func (r *agentBindingRepository) CreateAgentBinding(ctx context.Context, b *types.AgentBinding) error {
	return r.db.WithContext(ctx).Create(b).Error
}

func (r *agentBindingRepository) CreateAgentBindingWithKey(ctx context.Context, b *types.AgentBinding, key *types.AgentBindingKey) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(b).Error; err != nil {
			return err
		}
		return tx.Create(key).Error
	})
}

// ResolveActiveKeyAndBinding returns a coherent authentication snapshot. The
// first key read only discovers lock order; the binding is locked first to
// match rotation, then the same key is locked and rechecked. Thus a rotation
// committed while this call waits can never pair the old key with new policy.
func (r *agentBindingRepository) ResolveActiveKeyAndBinding(ctx context.Context, hash string, now time.Time) (*types.AgentBindingKey, *types.AgentBinding, error) {
	var key types.AgentBindingKey
	var binding types.AgentBinding
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidate types.AgentBindingKey
		if err := tx.Where("key_hash = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)", hash, now).
			First(&candidate).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAgentBindingKeyNotFound
			}
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND id = ? AND status = ? AND (expires_at IS NULL OR expires_at > ?)", candidate.TenantID, candidate.BindingID, types.AgentBindingStatusActive, now).
			First(&binding).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAgentBindingNotFound
			}
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND key_hash = ? AND tenant_id = ? AND binding_id = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)", candidate.ID, hash, candidate.TenantID, candidate.BindingID, now).
			First(&key).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAgentBindingKeyNotFound
			}
			return err
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return &key, &binding, nil
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

func (r *agentBindingRepository) TouchAgentBinding(ctx context.Context, tenantID uint64, bindingID string, usedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&types.AgentBinding{}).Where("tenant_id = ? AND id = ?", tenantID, bindingID).
		Updates(map[string]any{"last_used_at": usedAt, "updated_at": usedAt}).Error
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

func (r *agentBindingRepository) RotateAgentBindingKey(ctx context.Context, tenantID uint64, bindingID string, key *types.AgentBindingKey) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var binding types.AgentBinding
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ? AND status = ?", tenantID, bindingID, types.AgentBindingStatusActive).First(&binding).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAgentBindingNotFound
			}
			return err
		}
		if binding.PolicyVersion >= maxAgentBindingPolicyVersion {
			return ErrAgentBindingPolicyVersionOverflow
		}
		// Insert first: a failed replacement must leave all previous keys active.
		if err := tx.Create(key).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.Model(&types.AgentBindingKey{}).
			Where("tenant_id = ? AND binding_id = ? AND id <> ? AND revoked_at IS NULL", tenantID, bindingID, key.ID).
			Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		// A key rotation is also a token-generation rotation. Incrementing the
		// authoritative policy version invalidates every token minted by the old key.
		result := tx.Model(&types.AgentBinding{}).Where("tenant_id = ? AND id = ?", tenantID, bindingID).
			Updates(map[string]any{"policy_version": gorm.Expr("policy_version + 1"), "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrAgentBindingNotFound
		}
		return nil
	})
}

func (r *agentBindingRepository) RevokeAgentBindingWithKeys(ctx context.Context, tenantID uint64, bindingID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var binding types.AgentBinding
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, bindingID).First(&binding).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAgentBindingNotFound
			}
			return err
		}
		now := time.Now().UTC()
		if err := tx.Model(&types.AgentBinding{}).Where("tenant_id = ? AND id = ?", tenantID, bindingID).
			Updates(map[string]any{"status": types.AgentBindingStatusRevoked, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&types.AgentBindingKey{}).
			Where("tenant_id = ? AND binding_id = ? AND revoked_at IS NULL", tenantID, bindingID).
			Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error
	})
}
