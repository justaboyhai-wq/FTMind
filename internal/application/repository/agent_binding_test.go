package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAgentBindingTestRepo(t *testing.T) (*agentBindingRepository, *agentBindingKeyRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.AgentBinding{}, &types.AgentBindingKey{}))
	return NewAgentBindingRepository(db).(*agentBindingRepository), NewAgentBindingKeyRepository(db).(*agentBindingKeyRepository), db
}

func TestAgentBindingRepositoryIsolatesTenantAndScopes(t *testing.T) {
	repo, _, _ := newAgentBindingTestRepo(t)
	ctx := context.Background()
	b := &types.AgentBinding{ID: uuid.NewString(), TenantID: 7, TeamID: "team-1", UserID: "user-1", WorkspaceID: "ws-1", ProjectID: "p-1", AgentID: "agent-1", ExternalAgent: "openclaw-prod", ConnectorType: "openclaw", Status: types.AgentBindingStatusActive, CapabilityScopes: types.StringArray{"memory.capture"}, AssetScopes: types.StringArray{"workspace:ws-1"}}
	require.NoError(t, repo.CreateAgentBinding(ctx, b))
	found, err := repo.FindActiveAgentBinding(ctx, 7, "openclaw-prod", "openclaw")
	require.NoError(t, err)
	require.Equal(t, "ws-1", found.WorkspaceID)
	require.Equal(t, types.StringArray{"memory.capture"}, found.CapabilityScopes)
	_, err = repo.GetAgentBinding(ctx, 8, b.ID)
	require.ErrorIs(t, err, ErrAgentBindingNotFound)
	require.NoError(t, repo.RevokeAgentBinding(ctx, 7, b.ID))
	_, err = repo.FindActiveAgentBinding(ctx, 7, "openclaw-prod", "openclaw")
	require.ErrorIs(t, err, ErrAgentBindingNotFound)
}

func TestAgentBindingRepositoryCreateWithKeyRollsBackOnKeyFailure(t *testing.T) {
	repo, keyRepo, _ := newAgentBindingTestRepo(t)
	ctx := context.Background()
	existing := &types.AgentBinding{ID: uuid.NewString(), TenantID: 7, TeamID: "team", UserID: "user", AgentID: "agent-a", ExternalAgent: "external-a", ConnectorType: "generic_sdk", Status: types.AgentBindingStatusActive}
	require.NoError(t, repo.CreateAgentBindingWithKey(ctx, existing, &types.AgentBindingKey{ID: uuid.NewString(), BindingID: existing.ID, TenantID: 7, KeyHash: "duplicate"}))

	candidate := &types.AgentBinding{ID: uuid.NewString(), TenantID: 7, TeamID: "team", UserID: "user", AgentID: "agent-b", ExternalAgent: "external-b", ConnectorType: "generic_sdk", Status: types.AgentBindingStatusActive}
	err := repo.CreateAgentBindingWithKey(ctx, candidate, &types.AgentBindingKey{ID: uuid.NewString(), BindingID: candidate.ID, TenantID: 7, KeyHash: "duplicate"})
	require.Error(t, err)
	_, err = repo.GetAgentBinding(ctx, 7, candidate.ID)
	require.ErrorIs(t, err, ErrAgentBindingNotFound)
	_, err = keyRepo.GetActiveAgentBindingKeyByHash(ctx, "duplicate")
	require.NoError(t, err)
}

func TestAgentBindingRepositoryRotateCreatesReplacementBeforeRevokingOldAndRollsBack(t *testing.T) {
	repo, keyRepo, _ := newAgentBindingTestRepo(t)
	ctx := context.Background()
	b := &types.AgentBinding{ID: uuid.NewString(), TenantID: 8, TeamID: "team", UserID: "user", AgentID: "agent", ExternalAgent: "external", ConnectorType: "generic_sdk", Status: types.AgentBindingStatusActive}
	oldKey := &types.AgentBindingKey{ID: uuid.NewString(), BindingID: b.ID, TenantID: 8, KeyHash: "old-hash"}
	require.NoError(t, repo.CreateAgentBindingWithKey(ctx, b, oldKey))

	err := repo.RotateAgentBindingKey(ctx, 8, b.ID, &types.AgentBindingKey{ID: uuid.NewString(), BindingID: b.ID, TenantID: 8, KeyHash: "old-hash"})
	require.Error(t, err)
	stillActive, err := keyRepo.GetActiveAgentBindingKeyByHash(ctx, "old-hash")
	require.NoError(t, err)
	require.Equal(t, oldKey.ID, stillActive.ID)

	newKey := &types.AgentBindingKey{ID: uuid.NewString(), BindingID: b.ID, TenantID: 8, KeyHash: "new-hash"}
	require.NoError(t, repo.RotateAgentBindingKey(ctx, 8, b.ID, newKey))
	_, err = keyRepo.GetActiveAgentBindingKeyByHash(ctx, "old-hash")
	require.ErrorIs(t, err, ErrAgentBindingKeyNotFound)
	active, err := keyRepo.GetActiveAgentBindingKeyByHash(ctx, "new-hash")
	require.NoError(t, err)
	require.Equal(t, newKey.ID, active.ID)
}
