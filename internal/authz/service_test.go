package authz

import (
	"context"
	"testing"

	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/stretchr/testify/require"
)

func accessForRole(role types.TenantRole, caps ...string) AccessContext {
	return AccessContext{UserID: "u1", TenantID: 7, TenantRole: role, TeamIDs: []string{"team-a"}, Capabilities: caps}
}

func TestMemoryRoleDefaultsDoNotEscalateAcrossCapabilities(t *testing.T) {
	s := NewService()
	tests := []struct {
		name   string
		role   types.TenantRole
		action Action
		want   bool
	}{
		{"owner publishes", types.TenantRoleOwner, ActionPublish, true},
		{"admin revokes", types.TenantRoleAdmin, ActionRevoke, true},
		{"contributor captures", types.TenantRoleContributor, ActionCapture, true},
		{"contributor cannot review", types.TenantRoleContributor, ActionReview, false},
		{"viewer cannot capture", types.TenantRoleViewer, ActionCapture, false},
		{"viewer can read wiki", types.TenantRoleViewer, ActionRead, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := WithContext(context.Background(), accessForRole(tt.role,
				CapabilityMemoryContext, CapabilityMemoryCapture, CapabilityMemoryRecall,
				CapabilityMemoryReview, CapabilityMemoryPublish, CapabilityMemoryRevoke,
				CapabilityWikiGet,
			))
			resource := Resource{Type: "memory", ID: "m1", TenantID: 7, TeamID: "team-a", Status: "published"}
			if tt.want {
				require.NoError(t, s.Authorize(ctx, tt.action, resource))
			} else {
				require.ErrorIs(t, s.Authorize(ctx, tt.action, resource), ErrForbidden)
			}
		})
	}
}

func TestBindingCapabilityAndAssetScopeAreIntersections(t *testing.T) {
	s := NewService()
	ctx := WithContext(context.Background(), AccessContext{
		UserID: "u1", TenantID: 7, TenantRole: types.TenantRoleAdmin,
		TeamIDs: []string{"team-a"}, Capabilities: []string{CapabilityMemoryCapture, "knowledge.search"},
		AssetScopes: []string{"team:team-a", "knowledge_base:kb-a"},
		BindingID:   "binding-a",
	})
	require.NoError(t, s.Authorize(ctx, ActionCapture, Resource{Type: "memory", TenantID: 7, TeamID: "team-a"}))
	require.ErrorIs(t, s.Authorize(ctx, ActionReview, Resource{Type: "memory", TenantID: 7, TeamID: "team-a"}), ErrForbidden)
	require.ErrorIs(t, s.Authorize(ctx, ActionSearch, Resource{Type: "knowledge_base", ID: "kb-b", TenantID: 7, TeamID: "team-a"}), ErrForbidden)
	require.NoError(t, s.Authorize(ctx, ActionSearch, Resource{Type: "knowledge_base", ID: "kb-a", TenantID: 7, TeamID: "team-a"}))
	require.ErrorIs(t, s.Authorize(ctx, ActionRead, Resource{Type: "memory", TenantID: 8, TeamID: "team-a"}), ErrForbidden)
}

func TestTenantAdminCanManageResourcesAcrossTeams(t *testing.T) {
	s := NewService()
	ctx := WithContext(context.Background(), AccessContext{
		TenantID: 7, TenantRole: types.TenantRoleAdmin,
		Capabilities: []string{CapabilityMemoryReview, CapabilityMemoryPublish, CapabilityMemoryRevoke, CapabilityWikiGet},
	})
	require.NoError(t, s.Authorize(ctx, ActionReview, Resource{Type: "memory_wiki", TenantID: 7, TeamID: "team-other"}))
}

func TestMenuPermissionsUseRoleMatrixAndActualCapabilities(t *testing.T) {
	s := NewService()
	ctx := WithContext(context.Background(), accessForRole(types.TenantRoleContributor, CapabilityMemoryCapture, CapabilityMemoryRecall))
	menu := s.GetMenuPermissions(ctx)
	require.Contains(t, menu.RoleMatrix["contributor"], CapabilityMemoryCapture)
	require.Contains(t, menu.RoleMatrix["contributor"], CapabilityMemoryConfirm)
	require.False(t, menu.Menus[1].Visible)
	require.NotContains(t, menu.Menus[1].Actions, "memory.publish")
}
