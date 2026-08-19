package repository

import (
	"context"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"gorm.io/gorm"
)

type teamRepository struct{ db *gorm.DB }

func NewTeamRepository(db *gorm.DB) interfaces.TeamRepository { return &teamRepository{db: db} }
func (r *teamRepository) CreateDepartment(c context.Context, v *types.Department) error {
	return r.db.WithContext(c).Create(v).Error
}
func (r *teamRepository) ListDepartments(c context.Context, tenant uint64) ([]*types.Department, error) {
	var v []*types.Department
	e := r.db.WithContext(c).Where("tenant_id = ? AND status = ?", tenant, "active").Order("name").Find(&v).Error
	return v, e
}
func (r *teamRepository) GetDepartment(c context.Context, tenant uint64, id string) (*types.Department, error) {
	var v types.Department
	e := r.db.WithContext(c).Where("tenant_id = ? AND id = ? AND status = ?", tenant, id, "active").First(&v).Error
	return &v, e
}
func (r *teamRepository) CreateTeam(c context.Context, v *types.Team) error {
	return r.db.WithContext(c).Create(v).Error
}
func (r *teamRepository) ListTeams(c context.Context, tenant uint64, dept string) ([]*types.Team, error) {
	var v []*types.Team
	q := r.db.WithContext(c).Where("tenant_id = ? AND status = ?", tenant, "active")
	if dept != "" {
		q = q.Where("department_id = ?", dept)
	}
	e := q.Order("name").Find(&v).Error
	return v, e
}
func (r *teamRepository) GetTeam(c context.Context, tenant uint64, id string) (*types.Team, error) {
	var v types.Team
	e := r.db.WithContext(c).Where("tenant_id = ? AND id = ? AND status = ?", tenant, id, "active").First(&v).Error
	return &v, e
}
func (r *teamRepository) UpdateTeam(c context.Context, v *types.Team) error {
	return r.db.WithContext(c).Model(&types.Team{}).Where("tenant_id = ? AND id = ?", v.TenantID, v.ID).Updates(map[string]any{"name": v.Name, "code": v.Code, "status": v.Status, "department_id": v.DepartmentID}).Error
}
func (r *teamRepository) DeleteTeam(c context.Context, tenant uint64, id string) error {
	return r.db.WithContext(c).Model(&types.Team{}).Where("tenant_id = ? AND id = ?", tenant, id).Update("status", "deleted").Error
}
func (r *teamRepository) HasActiveMembers(c context.Context, tenant uint64, team string) (bool, error) {
	var count int64
	err := r.db.WithContext(c).Model(&types.TeamMember{}).
		Where("tenant_id = ? AND team_id = ? AND status = ? AND deleted_at IS NULL", tenant, team, "active").Count(&count).Error
	return count > 0, err
}
func (r *teamRepository) HasActiveAgents(c context.Context, tenant uint64, team string) (bool, error) {
	var count int64
	err := r.db.WithContext(c).Model(&types.TeamAgent{}).
		Where("tenant_id = ? AND team_id = ? AND status = ? AND deleted_at IS NULL", tenant, team, "active").Count(&count).Error
	return count > 0, err
}
func (r *teamRepository) HasActiveKnowledgeBases(c context.Context, tenant uint64, team string) (bool, error) {
	var count int64
	err := r.db.WithContext(c).Table("knowledge_bases").
		Where("tenant_id = ? AND team_id = ? AND deleted_at IS NULL", tenant, team).Count(&count).Error
	return count > 0, err
}
func (r *teamRepository) UserBelongsToTenant(c context.Context, tenant uint64, user string) (bool, error) {
	var count int64
	err := r.db.WithContext(c).Model(&types.TenantMember{}).
		Where("tenant_id = ? AND user_id = ? AND status = ? AND deleted_at IS NULL", tenant, user, types.TenantMemberStatusActive).Count(&count).Error
	return count > 0, err
}
func (r *teamRepository) AgentBelongsToTenant(c context.Context, tenant uint64, agent string) (bool, error) {
	var count int64
	err := r.db.WithContext(c).Table("custom_agents").
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenant, agent).Count(&count).Error
	return count > 0, err
}
func (r *teamRepository) UpsertMember(c context.Context, v *types.TeamMember) error {
	return r.db.WithContext(c).Where("team_id = ? AND user_id = ?", v.TeamID, v.UserID).Assign(map[string]any{"tenant_id": v.TenantID, "role": v.Role, "status": "active"}).FirstOrCreate(v).Error
}
func (r *teamRepository) ListMembers(c context.Context, tenant uint64, team string) ([]*types.TeamMember, error) {
	var v []*types.TeamMember
	e := r.db.WithContext(c).Where("tenant_id = ? AND team_id = ? AND status = ?", tenant, team, "active").Find(&v).Error
	return v, e
}
func (r *teamRepository) RemoveMember(c context.Context, tenant uint64, team, user string) error {
	return r.db.WithContext(c).Model(&types.TeamMember{}).Where("tenant_id = ? AND team_id = ? AND user_id = ?", tenant, team, user).Update("status", "removed").Error
}
func (r *teamRepository) UpsertAgent(c context.Context, v *types.TeamAgent) error {
	return r.db.WithContext(c).Where("team_id = ? AND agent_id = ?", v.TeamID, v.AgentID).Assign(map[string]any{"tenant_id": v.TenantID, "status": "active"}).FirstOrCreate(v).Error
}
func (r *teamRepository) ListAgents(c context.Context, tenant uint64, team string) ([]*types.TeamAgent, error) {
	var v []*types.TeamAgent
	e := r.db.WithContext(c).Where("tenant_id = ? AND team_id = ? AND status = ?", tenant, team, "active").Find(&v).Error
	return v, e
}
func (r *teamRepository) RemoveAgent(c context.Context, tenant uint64, team, agent string) error {
	return r.db.WithContext(c).Model(&types.TeamAgent{}).Where("tenant_id = ? AND team_id = ? AND agent_id = ?", tenant, team, agent).Update("status", "removed").Error
}
