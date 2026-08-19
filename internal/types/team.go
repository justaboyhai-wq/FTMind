package types

import (
	"gorm.io/gorm"
	"time"
)

// Department and Team are the first-class tenant organization resources.
type Department struct {
	ID        string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID  uint64         `json:"tenant_id" gorm:"not null;index"`
	Name      string         `json:"name" gorm:"type:varchar(128);not null"`
	Code      string         `json:"code" gorm:"type:varchar(64);not null"`
	Status    string         `json:"status" gorm:"type:varchar(20);not null;default:'active'"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (Department) TableName() string { return "departments" }

type Team struct {
	ID           string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID     uint64         `json:"tenant_id" gorm:"not null;index"`
	DepartmentID string         `json:"department_id" gorm:"type:varchar(36);not null;index"`
	Name         string         `json:"name" gorm:"type:varchar(128);not null"`
	Code         string         `json:"code" gorm:"type:varchar(64);not null"`
	Status       string         `json:"status" gorm:"type:varchar(20);not null;default:'active'"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

func (Team) TableName() string { return "teams" }

type TeamRole string

const (
	TeamRoleAdmin       TeamRole = "team_admin"
	TeamRoleReviewer    TeamRole = "reviewer"
	TeamRoleContributor TeamRole = "contributor"
	TeamRoleViewer      TeamRole = "viewer"
)

type TeamMember struct {
	ID        uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	TeamID    string         `json:"team_id" gorm:"type:varchar(36);not null;index"`
	TenantID  uint64         `json:"tenant_id" gorm:"not null;index"`
	UserID    string         `json:"user_id" gorm:"type:varchar(36);not null;index"`
	Role      TeamRole       `json:"role" gorm:"type:varchar(24);not null;default:'viewer'"`
	Status    string         `json:"status" gorm:"type:varchar(20);not null;default:'active'"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (TeamMember) TableName() string { return "team_members" }

type TeamAgent struct {
	ID        uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	TeamID    string         `json:"team_id" gorm:"type:varchar(36);not null;index"`
	TenantID  uint64         `json:"tenant_id" gorm:"not null;index"`
	AgentID   string         `json:"agent_id" gorm:"type:varchar(36);not null;index"`
	Status    string         `json:"status" gorm:"type:varchar(20);not null;default:'active'"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (TeamAgent) TableName() string { return "team_agents" }
