package service

import (
	"context"
	"github.com/google/uuid"
	apperrors "github.com/justaboyhai-wq/fmind/internal/errors"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"strings"
)

var ErrTeamInvalid = apperrors.NewValidationError("invalid organization resource")
var ErrTeamInUse = apperrors.NewConflictError("team has active members or resources")
var ErrTeamForbidden = apperrors.NewForbiddenError("organization management requires Owner or Admin")
var ErrTeamConflict = apperrors.NewConflictError("organization resource already exists")

func normalizeTeamRepoError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "record not found") || strings.Contains(message, "organization resource not found") {
		return ErrTeamInvalid
	}
	if strings.Contains(message, "duplicate") || strings.Contains(message, "unique constraint") || strings.Contains(message, "unique violation") {
		return ErrTeamConflict
	}
	return err
}

type teamService struct{ repo interfaces.TeamRepository }

func NewTeamService(repo interfaces.TeamRepository) interfaces.TeamService {
	return &teamService{repo: repo}
}

func requireTeamAdmin(ctx context.Context) error {
	if types.IsSystemAdminFromContext(ctx) {
		return nil
	}
	role := types.TenantRoleFromContext(ctx)
	if role == types.TenantRoleOwner || role == types.TenantRoleAdmin {
		return nil
	}
	return ErrTeamForbidden
}

func requireTeamTenant(ctx context.Context, tenantID uint64) error {
	if types.IsSystemAdminFromContext(ctx) {
		return nil
	}
	current, ok := types.TenantIDFromContext(ctx)
	if !ok || current == 0 || current != tenantID {
		return ErrTeamInvalid
	}
	return nil
}

func validName(v string) bool { return strings.TrimSpace(v) != "" && len([]rune(v)) <= 128 }
func validCode(v string) bool { return strings.TrimSpace(v) != "" && len([]rune(v)) <= 64 }
func (s *teamService) CreateDepartment(c context.Context, t uint64, n, code string) (*types.Department, error) {
	if err := requireTeamAdmin(c); err != nil {
		return nil, err
	}
	if err := requireTeamTenant(c, t); err != nil {
		return nil, err
	}
	if t == 0 || !validName(n) || !validCode(code) {
		return nil, ErrTeamInvalid
	}
	v := &types.Department{ID: uuid.NewString(), TenantID: t, Name: strings.TrimSpace(n), Code: strings.TrimSpace(code), Status: "active"}
	return v, normalizeTeamRepoError(s.repo.CreateDepartment(c, v))
}
func (s *teamService) ListDepartments(c context.Context, t uint64) ([]*types.Department, error) {
	if err := requireTeamTenant(c, t); err != nil {
		return nil, err
	}
	return s.repo.ListDepartments(c, t)
}
func (s *teamService) CreateTeam(c context.Context, t uint64, d, n, code, user string) (*types.Team, error) {
	if err := requireTeamAdmin(c); err != nil {
		return nil, err
	}
	if err := requireTeamTenant(c, t); err != nil {
		return nil, err
	}
	if t == 0 || strings.TrimSpace(d) == "" || !validName(n) || !validCode(code) {
		return nil, ErrTeamInvalid
	}
	if _, e := s.repo.GetDepartment(c, t, d); e != nil {
		return nil, normalizeTeamRepoError(e)
	}
	if strings.TrimSpace(user) != "" {
		belongs, e := s.repo.UserBelongsToTenant(c, t, user)
		if e != nil {
			return nil, e
		}
		if !belongs {
			return nil, ErrTeamInvalid
		}
	}
	v := &types.Team{ID: uuid.NewString(), TenantID: t, DepartmentID: d, Name: strings.TrimSpace(n), Code: strings.TrimSpace(code), Status: "active"}
	if e := normalizeTeamRepoError(s.repo.CreateTeam(c, v)); e != nil {
		return nil, e
	}
	if strings.TrimSpace(user) != "" {
		// Team membership only scopes resources.  It must never grant a
		// capability beyond the member's tenant role, so even the creator is
		// stored with the non-elevating compatibility role.
		_ = s.repo.UpsertMember(c, &types.TeamMember{TeamID: v.ID, TenantID: t, UserID: user, Role: types.TeamRoleViewer, Status: "active"})
	}
	return v, nil
}
func (s *teamService) ListTeams(c context.Context, t uint64, d string) ([]*types.Team, error) {
	if err := requireTeamTenant(c, t); err != nil {
		return nil, err
	}
	return s.repo.ListTeams(c, t, d)
}
func (s *teamService) UpdateTeam(c context.Context, t uint64, v *types.Team) error {
	if err := requireTeamAdmin(c); err != nil {
		return err
	}
	if err := requireTeamTenant(c, t); err != nil {
		return err
	}
	if v == nil || v.ID == "" || v.TenantID != t || !validName(v.Name) || !validCode(v.Code) {
		return ErrTeamInvalid
	}
	if _, e := s.repo.GetDepartment(c, t, v.DepartmentID); e != nil {
		return normalizeTeamRepoError(e)
	}
	return s.repo.UpdateTeam(c, v)
}
func (s *teamService) DeleteTeam(c context.Context, t uint64, id string) error {
	if err := requireTeamAdmin(c); err != nil {
		return err
	}
	if err := requireTeamTenant(c, t); err != nil {
		return err
	}
	if id == "" {
		return ErrTeamInvalid
	}
	if _, e := s.repo.GetTeam(c, t, id); e != nil {
		return normalizeTeamRepoError(e)
	}
	for _, check := range []func(context.Context, uint64, string) (bool, error){
		s.repo.HasActiveMembers,
		s.repo.HasActiveAgents,
		s.repo.HasActiveKnowledgeBases,
	} {
		used, e := check(c, t, id)
		if e != nil {
			return e
		}
		if used {
			return ErrTeamInUse
		}
	}
	return s.repo.DeleteTeam(c, t, id)
}
func (s *teamService) AddMember(c context.Context, t uint64, v *types.TeamMember) error {
	if err := requireTeamAdmin(c); err != nil {
		return err
	}
	if err := requireTeamTenant(c, t); err != nil {
		return err
	}
	if v == nil || v.TenantID != t || v.TeamID == "" || v.UserID == "" {
		return ErrTeamInvalid
	}
	if _, e := s.repo.GetTeam(c, t, v.TeamID); e != nil {
		return normalizeTeamRepoError(e)
	}
	belongs, e := s.repo.UserBelongsToTenant(c, t, v.UserID)
	if e != nil {
		return e
	}
	if !belongs {
		return ErrTeamInvalid
	}
	// Team membership is a resource scope, not a second role system. Keep
	// the legacy column for compatibility but never allow it to elevate a
	// tenant role.
	v.Role = types.TeamRoleViewer
	v.Status = "active"
	return s.repo.UpsertMember(c, v)
}
func (s *teamService) ListMembers(c context.Context, t uint64, id string) ([]*types.TeamMember, error) {
	if err := requireTeamTenant(c, t); err != nil {
		return nil, err
	}
	if id == "" {
		return nil, ErrTeamInvalid
	}
	if _, e := s.repo.GetTeam(c, t, id); e != nil {
		return nil, normalizeTeamRepoError(e)
	}
	return s.repo.ListMembers(c, t, id)
}
func (s *teamService) RemoveMember(c context.Context, t uint64, team, user string) error {
	if err := requireTeamAdmin(c); err != nil {
		return err
	}
	if err := requireTeamTenant(c, t); err != nil {
		return err
	}
	if team == "" || user == "" {
		return ErrTeamInvalid
	}
	if _, e := s.repo.GetTeam(c, t, team); e != nil {
		return normalizeTeamRepoError(e)
	}
	return s.repo.RemoveMember(c, t, team, user)
}
func (s *teamService) AddAgent(c context.Context, t uint64, v *types.TeamAgent) error {
	if err := requireTeamAdmin(c); err != nil {
		return err
	}
	if err := requireTeamTenant(c, t); err != nil {
		return err
	}
	if v == nil || v.TenantID != t || v.TeamID == "" || v.AgentID == "" {
		return ErrTeamInvalid
	}
	if types.IsBuiltinAgentID(v.AgentID) {
		return ErrTeamInvalid
	}
	if _, e := s.repo.GetTeam(c, t, v.TeamID); e != nil {
		return normalizeTeamRepoError(e)
	}
	belongs, e := s.repo.AgentBelongsToTenant(c, t, v.AgentID)
	if e != nil {
		return e
	}
	if !belongs {
		return ErrTeamInvalid
	}
	v.Status = "active"
	return s.repo.UpsertAgent(c, v)
}
func (s *teamService) ListAgents(c context.Context, t uint64, id string) ([]*types.TeamAgent, error) {
	if err := requireTeamTenant(c, t); err != nil {
		return nil, err
	}
	if id == "" {
		return nil, ErrTeamInvalid
	}
	if _, e := s.repo.GetTeam(c, t, id); e != nil {
		return nil, normalizeTeamRepoError(e)
	}
	return s.repo.ListAgents(c, t, id)
}
func (s *teamService) RemoveAgent(c context.Context, t uint64, team, agent string) error {
	if err := requireTeamAdmin(c); err != nil {
		return err
	}
	if err := requireTeamTenant(c, t); err != nil {
		return err
	}
	if team == "" || agent == "" {
		return ErrTeamInvalid
	}
	if _, e := s.repo.GetTeam(c, t, team); e != nil {
		return normalizeTeamRepoError(e)
	}
	return s.repo.RemoveAgent(c, t, team, agent)
}
