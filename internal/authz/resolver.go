package authz

import (
	"context"
	"errors"
	"sort"

	"github.com/justaboyhai-wq/fmind/internal/types"
	"gorm.io/gorm"
)

// AccessSource is the authoritative read model used to assemble a request's
// access context. Implementations must apply tenant and active-status filters
// in the database; callers never provide these relationships in request data.
type AccessSource interface {
	TenantMembership(context.Context, string, uint64) (*types.TenantMember, error)
	TeamMemberships(context.Context, string, uint64) ([]*types.TeamMember, error)
	Teams(context.Context, uint64, []string) ([]*types.Team, error)
	TeamAgents(context.Context, uint64, []string) ([]*types.TeamAgent, error)
	KnowledgeBases(context.Context, uint64) ([]*types.KnowledgeBase, error)
}

type Resolver struct{ source AccessSource }

func NewResolver(source AccessSource) *Resolver { return &Resolver{source: source} }

func (r *Resolver) Resolve(ctx context.Context, binding *types.AgentBinding) (AccessContext, error) {
	if r == nil || r.source == nil {
		return AccessContext{}, ErrMissingContext
	}
	userID, _ := types.UserIDFromContext(ctx)
	tenantID, _ := types.TenantIDFromContext(ctx)
	if binding != nil {
		if userID == "" {
			userID = binding.UserID
		}
		if tenantID == 0 {
			tenantID = binding.TenantID
		}
		if (userID != "" && binding.UserID != userID) || (tenantID != 0 && binding.TenantID != tenantID) {
			return AccessContext{}, ErrForbidden
		}
	}
	if userID == "" || tenantID == 0 {
		return AccessContext{}, ErrMissingContext
	}
	role := types.TenantRoleFromContext(ctx)
	membership, membershipErr := r.source.TenantMembership(ctx, userID, tenantID)
	if membershipErr != nil {
		return AccessContext{}, membershipErr
	}
	if membership == nil || membership.Status != types.TenantMemberStatusActive {
		if !types.IsSystemAdminFromContext(ctx) {
			return AccessContext{}, ErrForbidden
		}
	} else {
		role = membership.Role
	}
	if !role.IsValid() && !types.IsSystemAdminFromContext(ctx) {
		return AccessContext{}, ErrForbidden
	}

	teams, err := r.source.TeamMemberships(ctx, userID, tenantID)
	if err != nil {
		return AccessContext{}, err
	}
	teamIDs := make([]string, 0, len(teams))
	teamRoles := make(map[string]string)
	for _, member := range teams {
		if member == nil || member.Status != "active" || member.TenantID != tenantID {
			continue
		}
		teamIDs = appendUnique(teamIDs, member.TeamID)
		teamRoles[member.TeamID] = string(member.Role)
	}
	if binding != nil && binding.TeamID != "" && !contains(teamIDs, binding.TeamID) && !types.IsSystemAdminFromContext(ctx) {
		return AccessContext{}, ErrForbidden
	}
	teamRows, err := r.source.Teams(ctx, tenantID, teamIDs)
	if err != nil {
		return AccessContext{}, err
	}
	departmentIDs := make([]string, 0, len(teamRows))
	for _, team := range teamRows {
		if team != nil && team.TenantID == tenantID && contains(teamIDs, team.ID) {
			departmentIDs = appendUnique(departmentIDs, team.DepartmentID)
		}
	}

	agents, err := r.source.TeamAgents(ctx, tenantID, teamIDs)
	if err != nil {
		return AccessContext{}, err
	}
	agentIDs := make([]string, 0, len(agents))
	for _, agent := range agents {
		if agent == nil || agent.Status != "active" || !contains(teamIDs, agent.TeamID) {
			continue
		}
		agentIDs = appendUnique(agentIDs, agent.AgentID)
	}
	if binding != nil && binding.AgentID != "" && !contains(agentIDs, binding.AgentID) && !types.IsBuiltinAgentID(binding.AgentID) && !types.IsSystemAdminFromContext(ctx) {
		return AccessContext{}, ErrForbidden
	}

	kbs, err := r.source.KnowledgeBases(ctx, tenantID)
	if err != nil {
		return AccessContext{}, err
	}
	access := AccessContext{UserID: userID, TenantID: tenantID, TenantRole: role, DepartmentIDs: departmentIDs, TeamIDs: teamIDs, TeamRoles: teamRoles, AgentIDs: agentIDs}
	for _, kb := range kbs {
		if kb == nil || kb.TenantID != tenantID || !resourceVisible(role, teamIDs, kb.TeamID) {
			continue
		}
		if binding != nil && !bindingAssetAllows(binding.AssetScopes, kb) {
			continue
		}
		access.KnowledgeBaseIDs = appendUnique(access.KnowledgeBaseIDs, kb.ID)
		if kb.HasMemoryWikiMarker() {
			access.MemoryWikiIDs = appendUnique(access.MemoryWikiIDs, kb.ID)
		}
	}
	if binding == nil {
		access.Capabilities = DefaultCapabilities(role)
	} else {
		access.BindingID = binding.ID
		access.PolicyVersion = binding.PolicyVersion
		access.UserAPIKeyID = binding.UserAPIKeyID
		access.AssetScopes = append([]string(nil), binding.AssetScopes...)
		access.Capabilities = intersectCapabilities(DefaultCapabilities(role), binding.CapabilityScopes)
	}
	return access, nil
}

func DefaultCapabilities(role types.TenantRole) []string {
	all := roleCapabilityMatrix()[string(role)]
	return append([]string(nil), all...)
}

func intersectCapabilities(role []string, binding types.StringArray) []string {
	if len(binding) == 0 {
		return nil
	}
	out := make([]string, 0)
	for _, capability := range role {
		if containsString(binding, capability) {
			out = append(out, capability)
		}
	}
	return out
}

func bindingAssetAllows(scopes types.StringArray, kb *types.KnowledgeBase) bool {
	if len(scopes) == 0 {
		return false
	}
	explicitKB := false
	for _, scope := range scopes {
		if len(scope) > len("knowledge_base:") && scope[:len("knowledge_base:")] == "knowledge_base:" {
			explicitKB = true
			if scope == "knowledge_base:"+kb.ID {
				return true
			}
		}
	}
	if explicitKB {
		return false
	}
	return containsString(scopes, "team:"+kb.TeamID)
}

func resourceVisible(role types.TenantRole, teams []string, teamID string) bool {
	if role == types.TenantRoleOwner || role == types.TenantRoleAdmin || types.IsBuiltinAgentID(teamID) {
		return true
	}
	return contains(teams, teamID)
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsString(values types.StringArray, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// gormAccessSource is the production adapter. Every query is tenant-scoped
// and active-only so the resolver cannot accidentally rehydrate stale or
// cross-tenant organization rows.
type gormAccessSource struct{ db *gorm.DB }

func NewGormAccessSource(db *gorm.DB) AccessSource { return &gormAccessSource{db: db} }

func (s *gormAccessSource) TenantMembership(ctx context.Context, userID string, tenantID uint64) (*types.TenantMember, error) {
	var row types.TenantMember
	if err := s.db.WithContext(ctx).Where("user_id = ? AND tenant_id = ? AND status = ?", userID, tenantID, types.TenantMemberStatusActive).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}
func (s *gormAccessSource) TeamMemberships(ctx context.Context, userID string, tenantID uint64) ([]*types.TeamMember, error) {
	var rows []*types.TeamMember
	err := s.db.WithContext(ctx).Where("user_id = ? AND tenant_id = ? AND status = ?", userID, tenantID, "active").Find(&rows).Error
	return rows, err
}
func (s *gormAccessSource) Teams(ctx context.Context, tenantID uint64, teamIDs []string) ([]*types.Team, error) {
	if len(teamIDs) == 0 {
		return []*types.Team{}, nil
	}
	var rows []*types.Team
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND id IN ? AND status = ?", tenantID, teamIDs, "active").Find(&rows).Error
	return rows, err
}
func (s *gormAccessSource) TeamAgents(ctx context.Context, tenantID uint64, teamIDs []string) ([]*types.TeamAgent, error) {
	if len(teamIDs) == 0 {
		return []*types.TeamAgent{}, nil
	}
	var rows []*types.TeamAgent
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND team_id IN ? AND status = ?", tenantID, teamIDs, "active").Find(&rows).Error
	return rows, err
}
func (s *gormAccessSource) KnowledgeBases(ctx context.Context, tenantID uint64) ([]*types.KnowledgeBase, error) {
	var rows []*types.KnowledgeBase
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND is_temporary = ?", tenantID, false).Find(&rows).Error
	return rows, err
}

func sortStrings(values []string) []string { sort.Strings(values); return values }
