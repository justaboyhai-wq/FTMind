package interfaces

import (
	"context"
	"github.com/justaboyhai-wq/fmind/internal/types"
)

type TeamRepository interface {
	CreateDepartment(context.Context, *types.Department) error
	ListDepartments(context.Context, uint64) ([]*types.Department, error)
	GetDepartment(context.Context, uint64, string) (*types.Department, error)
	CreateTeam(context.Context, *types.Team) error
	ListTeams(context.Context, uint64, string) ([]*types.Team, error)
	GetTeam(context.Context, uint64, string) (*types.Team, error)
	UpdateTeam(context.Context, *types.Team) error
	DeleteTeam(context.Context, uint64, string) error
	HasActiveMembers(context.Context, uint64, string) (bool, error)
	HasActiveAgents(context.Context, uint64, string) (bool, error)
	HasActiveKnowledgeBases(context.Context, uint64, string) (bool, error)
	UserBelongsToTenant(context.Context, uint64, string) (bool, error)
	AgentBelongsToTenant(context.Context, uint64, string) (bool, error)
	UpsertMember(context.Context, *types.TeamMember) error
	ListMembers(context.Context, uint64, string) ([]*types.TeamMember, error)
	RemoveMember(context.Context, uint64, string, string) error
	UpsertAgent(context.Context, *types.TeamAgent) error
	ListAgents(context.Context, uint64, string) ([]*types.TeamAgent, error)
	RemoveAgent(context.Context, uint64, string, string) error
}

type TeamService interface {
	CreateDepartment(context.Context, uint64, string, string) (*types.Department, error)
	ListDepartments(context.Context, uint64) ([]*types.Department, error)
	CreateTeam(context.Context, uint64, string, string, string, string) (*types.Team, error)
	ListTeams(context.Context, uint64, string) ([]*types.Team, error)
	UpdateTeam(context.Context, uint64, *types.Team) error
	DeleteTeam(context.Context, uint64, string) error
	AddMember(context.Context, uint64, *types.TeamMember) error
	ListMembers(context.Context, uint64, string) ([]*types.TeamMember, error)
	RemoveMember(context.Context, uint64, string, string) error
	AddAgent(context.Context, uint64, *types.TeamAgent) error
	ListAgents(context.Context, uint64, string) ([]*types.TeamAgent, error)
	RemoveAgent(context.Context, uint64, string, string) error
}
