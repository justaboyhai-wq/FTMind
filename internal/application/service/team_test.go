package service

import (
	"context"
	"errors"
	"testing"

	"github.com/justaboyhai-wq/fmind/internal/types"
)

type teamRepoStub struct {
	team                 *types.Team
	department           *types.Department
	userInTenant         bool
	agentInTenant        bool
	activeMembers        bool
	activeAgents         bool
	activeKnowledgeBases bool
	upsertedMember       *types.TeamMember
	upsertedAgent        *types.TeamAgent
}

func teamAdminContext() context.Context {
	ctx := context.WithValue(context.Background(), types.TenantRoleContextKey, types.TenantRoleAdmin)
	return context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
}

func (s *teamRepoStub) CreateDepartment(context.Context, *types.Department) error { return nil }
func (s *teamRepoStub) ListDepartments(context.Context, uint64) ([]*types.Department, error) {
	return nil, nil
}
func (s *teamRepoStub) GetDepartment(_ context.Context, _ uint64, _ string) (*types.Department, error) {
	if s.department == nil {
		return nil, errors.New("department not found")
	}
	return s.department, nil
}
func (s *teamRepoStub) CreateTeam(context.Context, *types.Team) error { return nil }
func (s *teamRepoStub) ListTeams(context.Context, uint64, string) ([]*types.Team, error) {
	return nil, nil
}
func (s *teamRepoStub) GetTeam(_ context.Context, _ uint64, _ string) (*types.Team, error) {
	if s.team == nil {
		return nil, errors.New("team not found")
	}
	return s.team, nil
}
func (s *teamRepoStub) UpdateTeam(context.Context, *types.Team) error    { return nil }
func (s *teamRepoStub) DeleteTeam(context.Context, uint64, string) error { return nil }
func (s *teamRepoStub) HasActiveMembers(context.Context, uint64, string) (bool, error) {
	return s.activeMembers, nil
}
func (s *teamRepoStub) HasActiveAgents(context.Context, uint64, string) (bool, error) {
	return s.activeAgents, nil
}
func (s *teamRepoStub) HasActiveKnowledgeBases(context.Context, uint64, string) (bool, error) {
	return s.activeKnowledgeBases, nil
}
func (s *teamRepoStub) UserBelongsToTenant(context.Context, uint64, string) (bool, error) {
	return s.userInTenant, nil
}
func (s *teamRepoStub) AgentBelongsToTenant(context.Context, uint64, string) (bool, error) {
	return s.agentInTenant, nil
}
func (s *teamRepoStub) UpsertMember(_ context.Context, member *types.TeamMember) error {
	s.upsertedMember = member
	return nil
}
func (s *teamRepoStub) ListMembers(context.Context, uint64, string) ([]*types.TeamMember, error) {
	return nil, nil
}
func (s *teamRepoStub) RemoveMember(context.Context, uint64, string, string) error { return nil }
func (s *teamRepoStub) UpsertAgent(_ context.Context, agent *types.TeamAgent) error {
	s.upsertedAgent = agent
	return nil
}
func (s *teamRepoStub) ListAgents(context.Context, uint64, string) ([]*types.TeamAgent, error) {
	return nil, nil
}
func (s *teamRepoStub) RemoveAgent(context.Context, uint64, string, string) error { return nil }

func TestTeamServiceMemberRoleCannotElevateTenantRole(t *testing.T) {
	repo := &teamRepoStub{
		team:         &types.Team{ID: "team-1", TenantID: 7, Status: "active"},
		userInTenant: true,
	}
	svc := NewTeamService(repo)
	member := &types.TeamMember{TenantID: 7, TeamID: "team-1", UserID: "user-1", Role: types.TeamRoleAdmin}

	if err := svc.AddMember(teamAdminContext(), 7, member); err != nil {
		t.Fatalf("AddMember() error = %v", err)
	}
	if member.Role != types.TeamRoleViewer || member.Status != "active" {
		t.Fatalf("team membership must remain non-elevating viewer, got role=%q status=%q", member.Role, member.Status)
	}
	if repo.upsertedMember != member {
		t.Fatal("expected validated member to be persisted")
	}
}

func TestTeamServiceRejectsOrganizationMutationForViewer(t *testing.T) {
	repo := &teamRepoStub{}
	viewer := context.WithValue(context.Background(), types.TenantRoleContextKey, types.TenantRoleViewer)
	if _, err := NewTeamService(repo).CreateDepartment(viewer, 7, "Engineering", "eng"); !errors.Is(err, ErrTeamForbidden) {
		t.Fatalf("viewer CreateDepartment() error = %v, want ErrTeamForbidden", err)
	}
}

func TestTeamServiceRejectsCrossTenantMemberAndAgent(t *testing.T) {
	repo := &teamRepoStub{team: &types.Team{ID: "team-1", TenantID: 7, Status: "active"}}
	svc := NewTeamService(repo)

	if err := svc.AddMember(teamAdminContext(), 7, &types.TeamMember{TenantID: 7, TeamID: "team-1", UserID: "user-2"}); !errors.Is(err, ErrTeamInvalid) {
		t.Fatalf("cross-tenant member error = %v, want ErrTeamInvalid", err)
	}
	if err := svc.AddAgent(teamAdminContext(), 7, &types.TeamAgent{TenantID: 7, TeamID: "team-1", AgentID: "agent-2"}); !errors.Is(err, ErrTeamInvalid) {
		t.Fatalf("cross-tenant agent error = %v, want ErrTeamInvalid", err)
	}
}

func TestTeamServiceDeleteRejectsTeamWithActiveResources(t *testing.T) {
	tests := []struct {
		name string
		set  func(*teamRepoStub)
	}{
		{name: "members", set: func(s *teamRepoStub) { s.activeMembers = true }},
		{name: "agents", set: func(s *teamRepoStub) { s.activeAgents = true }},
		{name: "knowledge bases", set: func(s *teamRepoStub) { s.activeKnowledgeBases = true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &teamRepoStub{team: &types.Team{ID: "team-1", TenantID: 7, Status: "active"}}
			tt.set(repo)
			if err := NewTeamService(repo).DeleteTeam(teamAdminContext(), 7, "team-1"); !errors.Is(err, ErrTeamInUse) {
				t.Fatalf("DeleteTeam() error = %v, want ErrTeamInUse", err)
			}
		})
	}
}
