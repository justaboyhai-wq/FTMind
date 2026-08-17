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

func newAgentBindingTestRepo(t *testing.T) *agentBindingRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.AgentBinding{}))
	return NewAgentBindingRepository(db).(*agentBindingRepository)
}

func TestAgentBindingRepositoryIsolatesTenantAndScopes(t *testing.T) {
	repo := newAgentBindingTestRepo(t)
	ctx := context.Background()
	b := &types.AgentBinding{ID: uuid.NewString(), TenantID: 7, WorkspaceID: "ws-1", ProjectID: "p-1", AgentID: "agent-1", ExternalAgent: "openclaw-prod", ConnectorType: "openclaw", Status: types.AgentBindingStatusActive, CapabilityScopes: []string{"memory.capture"}, AssetScopes: []string{"workspace:ws-1"}}
	require.NoError(t, repo.CreateAgentBinding(ctx, b))
	found, err := repo.FindActiveAgentBinding(ctx, 7, "openclaw-prod", "openclaw")
	require.NoError(t, err)
	require.Equal(t, "ws-1", found.WorkspaceID)
	require.Equal(t, []string{"memory.capture"}, found.CapabilityScopes)
	_, err = repo.GetAgentBinding(ctx, 8, b.ID)
	require.ErrorIs(t, err, ErrAgentBindingNotFound)
	require.NoError(t, repo.RevokeAgentBinding(ctx, 7, b.ID))
	_, err = repo.FindActiveAgentBinding(ctx, 7, "openclaw-prod", "openclaw")
	require.ErrorIs(t, err, ErrAgentBindingNotFound)
}
