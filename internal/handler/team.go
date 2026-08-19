package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"net/http"
)

type TeamHandler struct{ service interfaces.TeamService }

func NewTeamHandler(s interfaces.TeamService) *TeamHandler { return &TeamHandler{service: s} }
func (h *TeamHandler) ListDepartments(c *gin.Context) {
	t := types.MustTenantIDFromContext(c.Request.Context())
	v, e := h.service.ListDepartments(c, t)
	if e != nil {
		c.Error(e)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": v})
}
func (h *TeamHandler) CreateDepartment(c *gin.Context) {
	t := types.MustTenantIDFromContext(c.Request.Context())
	var x struct {
		Name string `json:"name"`
		Code string `json:"code"`
	}
	if c.ShouldBindJSON(&x) != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	v, e := h.service.CreateDepartment(c, t, x.Name, x.Code)
	if e != nil {
		c.Error(e)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": v})
}
func (h *TeamHandler) ListTeams(c *gin.Context) {
	t := types.MustTenantIDFromContext(c.Request.Context())
	v, e := h.service.ListTeams(c, t, c.Query("department_id"))
	if e != nil {
		c.Error(e)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": v})
}
func (h *TeamHandler) CreateTeam(c *gin.Context) {
	t := types.MustTenantIDFromContext(c.Request.Context())
	var x struct {
		DepartmentID string `json:"department_id"`
		Name         string `json:"name"`
		Code         string `json:"code"`
	}
	if c.ShouldBindJSON(&x) != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	v, e := h.service.CreateTeam(c, t, x.DepartmentID, x.Name, x.Code, "")
	if e != nil {
		c.Error(e)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": v})
}
func (h *TeamHandler) UpdateTeam(c *gin.Context) {
	t := types.MustTenantIDFromContext(c.Request.Context())
	var x types.Team
	if c.ShouldBindJSON(&x) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	x.ID = c.Param("id")
	x.TenantID = t
	if e := h.service.UpdateTeam(c, t, &x); e != nil {
		c.Error(e)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
func (h *TeamHandler) DeleteTeam(c *gin.Context) {
	t := types.MustTenantIDFromContext(c.Request.Context())
	if e := h.service.DeleteTeam(c, t, c.Param("id")); e != nil {
		c.Error(e)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *TeamHandler) ListMembers(c *gin.Context) {
	t := types.MustTenantIDFromContext(c.Request.Context())
	v, e := h.service.ListMembers(c, t, c.Param("id"))
	if e != nil {
		c.Error(e)
		return
	}
	c.JSON(200, gin.H{"success": true, "data": v})
}
func (h *TeamHandler) AddMember(c *gin.Context) {
	t := types.MustTenantIDFromContext(c.Request.Context())
	var x types.TeamMember
	if c.ShouldBindJSON(&x) != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	x.TeamID = c.Param("id")
	x.TenantID = t
	e := h.service.AddMember(c, t, &x)
	if e != nil {
		c.Error(e)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true})
}
func (h *TeamHandler) RemoveMember(c *gin.Context) {
	t := types.MustTenantIDFromContext(c.Request.Context())
	e := h.service.RemoveMember(c, t, c.Param("id"), c.Param("user_id"))
	if e != nil {
		c.Error(e)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *TeamHandler) ListAgents(c *gin.Context) {
	t := types.MustTenantIDFromContext(c.Request.Context())
	v, e := h.service.ListAgents(c, t, c.Param("id"))
	if e != nil {
		c.Error(e)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": v})
}
func (h *TeamHandler) AddAgent(c *gin.Context) {
	t := types.MustTenantIDFromContext(c.Request.Context())
	var x types.TeamAgent
	if c.ShouldBindJSON(&x) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	x.TeamID = c.Param("id")
	x.TenantID = t
	if e := h.service.AddAgent(c, t, &x); e != nil {
		c.Error(e)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true})
}
func (h *TeamHandler) RemoveAgent(c *gin.Context) {
	t := types.MustTenantIDFromContext(c.Request.Context())
	if e := h.service.RemoveAgent(c, t, c.Param("id"), c.Param("agent_id")); e != nil {
		c.Error(e)
		return
	}
	c.Status(http.StatusNoContent)
}
