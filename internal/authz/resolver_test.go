package authz

import (
	"context"
	"testing"

	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/stretchr/testify/require"
)

type fakeAccessSource struct {
	membership *types.TenantMember
	teams      []*types.TeamMember
	agents     []*types.TeamAgent
	kbs        []*types.KnowledgeBase
}

func (f *fakeAccessSource) TenantMembership(context.Context, string, uint64) (*types.TenantMember, error) {
	return f.membership, nil
}
func (f *fakeAccessSource) TeamMemberships(context.Context, string, uint64) ([]*types.TeamMember, error) {
	return f.teams, nil
}
func (f *fakeAccessSource) Teams(context.Context, uint64, []string) ([]*types.Team, error) {
	return []*types.Team{{ID: "team-a", TenantID: 7, DepartmentID: "dept-a", Status: "active"}}, nil
}
func (f *fakeAccessSource) TeamAgents(context.Context, uint64, []string) ([]*types.TeamAgent, error) {
	return f.agents, nil
}
func (f *fakeAccessSource) KnowledgeBases(context.Context, uint64) ([]*types.KnowledgeBase, error) {
	return f.kbs, nil
}

func TestAccessContextResolverAssemblesOrganizationResourcesAndBindingIntersection(t *testing.T) {
	source := &fakeAccessSource{
		membership: &types.TenantMember{UserID: "u1", TenantID: 7, Role: types.TenantRoleContributor, Status: types.TenantMemberStatusActive},
		teams:      []*types.TeamMember{{UserID: "u1", TenantID: 7, TeamID: "team-a", Role: types.TeamRoleContributor, Status: "active"}},
		agents:     []*types.TeamAgent{{TenantID: 7, TeamID: "team-a", AgentID: "agent-a", Status: "active"}},
		kbs:        []*types.KnowledgeBase{{ID: "kb-a", TenantID: 7, TeamID: "team-a"}, {ID: "kb-b", TenantID: 7, TeamID: "team-b"}, {ID: "wiki-a", TenantID: 7, TeamID: "team-a", IsMemoryWiki: true, MemoryTeamID: "team-a"}},
	}
	r := NewResolver(source)
	base := context.WithValue(context.Background(), types.UserIDContextKey, "u1")
	base = context.WithValue(base, types.TenantIDContextKey, uint64(7))
	base = context.WithValue(base, types.TenantRoleContextKey, types.TenantRoleContributor)
	binding := &types.AgentBinding{ID: "b1", TenantID: 7, UserID: "u1", TeamID: "team-a", AgentID: "agent-a", PolicyVersion: 4, CapabilityScopes: types.StringArray{CapabilityMemoryCapture, CapabilityMemoryReview}, AssetScopes: types.StringArray{"team:team-a", "knowledge_base:kb-a", "knowledge_base:wiki-a"}}
	access, err := r.Resolve(base, binding)
	require.NoError(t, err)
	require.Equal(t, []string{"team-a"}, access.TeamIDs)
	require.Equal(t, []string{"dept-a"}, access.DepartmentIDs)
	require.Equal(t, map[string]string{"team-a": string(types.TeamRoleContributor)}, access.TeamRoles)
	require.Equal(t, []string{"agent-a"}, access.AgentIDs)
	require.Equal(t, []string{"kb-a", "wiki-a"}, access.KnowledgeBaseIDs)
	require.Equal(t, []string{"wiki-a"}, access.MemoryWikiIDs)
	require.Equal(t, []string{CapabilityMemoryCapture}, access.Capabilities)
	require.Equal(t, "b1", access.BindingID)
	require.Equal(t, uint64(4), access.PolicyVersion)
}
