package agentbinding

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

var ErrUnsupportedConnector = errors.New("unsupported agent connector")
var supportedConnectors = map[string]bool{"openclaw_plugin": true, "hermes_provider": true, "openai_proxy": true, "anthropic_proxy": true, "generic_sdk": true}

type Service struct {
	bindings interfaces.AgentBindingRepository
	keys     interfaces.AgentBindingKeyRepository
}

func NewService(bindings interfaces.AgentBindingRepository, keys interfaces.AgentBindingKeyRepository) interfaces.AgentBindingService {
	return &Service{bindings: bindings, keys: keys}
}

func (s *Service) Create(ctx context.Context, req interfaces.AgentBindingCreateRequest) (*interfaces.AgentBindingCreateResult, error) {
	tenantID, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if !supportedConnectors[req.ConnectorType] {
		return nil, ErrUnsupportedConnector
	}
	b := &types.AgentBinding{ID: uuid.NewString(), TenantID: tenantID, DepartmentID: req.DepartmentID, WorkspaceID: req.WorkspaceID, ProjectID: req.ProjectID, AgentID: req.AgentID, ExternalAgent: req.ExternalAgent, ConnectorType: req.ConnectorType, Status: types.AgentBindingStatusActive, CapabilityScopes: req.CapabilityScopes, AssetScopes: req.AssetScopes, CreatedBy: req.CreatedBy}
	if err := s.bindings.CreateAgentBinding(ctx, b); err != nil {
		return nil, err
	}
	secret, err := newSecret()
	if err != nil {
		return nil, err
	}
	if err := s.persistKey(ctx, b, secret); err != nil {
		return nil, err
	}
	return &interfaces.AgentBindingCreateResult{Binding: b, ConnectorSecret: secret}, nil
}

func (s *Service) List(ctx context.Context) ([]*types.AgentBinding, error) {
	tenantID, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return s.bindings.ListAgentBindings(ctx, tenantID)
}
func (s *Service) Get(ctx context.Context, id string) (*types.AgentBinding, error) {
	tenantID, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return s.bindings.GetAgentBinding(ctx, tenantID, id)
}
func (s *Service) Revoke(ctx context.Context, id string) error {
	tenantID, err := tenantFromContext(ctx)
	if err != nil {
		return err
	}
	if err := s.bindings.RevokeAgentBinding(ctx, tenantID, id); err != nil {
		return err
	}
	return s.keys.RevokeAgentBindingKeys(ctx, tenantID, id)
}
func (s *Service) RotateKey(ctx context.Context, id, createdBy string) (string, error) {
	b, err := s.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if err := s.keys.RevokeAgentBindingKeys(ctx, b.TenantID, id); err != nil {
		return "", err
	}
	secret, err := newSecret()
	if err != nil {
		return "", err
	}
	if err := s.persistKey(ctx, b, secret); err != nil {
		return "", err
	}
	return secret, nil
}

func (s *Service) persistKey(ctx context.Context, b *types.AgentBinding, secret string) error {
	h := sha256.Sum256([]byte(secret))
	return s.keys.CreateAgentBindingKey(ctx, &types.AgentBindingKey{ID: uuid.NewString(), BindingID: b.ID, TenantID: b.TenantID, KeyHash: hex.EncodeToString(h[:]), CreatedBy: b.CreatedBy})
}
func newSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate binding secret: %w", err)
	}
	return "fmind_" + hex.EncodeToString(b), nil
}
func tenantFromContext(ctx context.Context) (uint64, error) {
	id, ok := types.TenantIDFromContext(ctx)
	if !ok || id == 0 {
		return 0, errors.New("tenant context is required")
	}
	return id, nil
}
