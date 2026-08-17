package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"net/http"
)

type AgentBindingHandler struct {
	service interfaces.AgentBindingService
}

func NewAgentBindingHandler(service interfaces.AgentBindingService) *AgentBindingHandler {
	return &AgentBindingHandler{service: service}
}

type agentBindingRequest struct {
	WorkspaceID      string   `json:"workspace_id"`
	ProjectID        string   `json:"project_id"`
	DepartmentID     string   `json:"department_id"`
	AgentID          string   `json:"agent_id" binding:"required"`
	ExternalAgent    string   `json:"external_agent" binding:"required"`
	ConnectorType    string   `json:"connector_type" binding:"required"`
	CapabilityScopes []string `json:"capability_scopes"`
	AssetScopes      []string `json:"asset_scopes"`
}

func (h *AgentBindingHandler) Create(c *gin.Context) {
	var req agentBindingRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	userID, _ := c.Get("user_id")
	result, err := h.service.Create(c, interfaces.AgentBindingCreateRequest{WorkspaceID: req.WorkspaceID, ProjectID: req.ProjectID, DepartmentID: req.DepartmentID, AgentID: req.AgentID, ExternalAgent: req.ExternalAgent, ConnectorType: req.ConnectorType, CapabilityScopes: req.CapabilityScopes, AssetScopes: req.AssetScopes, CreatedBy: userIDString(userID)})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, result)
}
func (h *AgentBindingHandler) List(c *gin.Context) {
	result, err := h.service.List(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *AgentBindingHandler) Get(c *gin.Context) {
	result, err := h.service.Get(c, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *AgentBindingHandler) Revoke(c *gin.Context) {
	if err := h.service.Revoke(c, c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *AgentBindingHandler) Rotate(c *gin.Context) {
	userID, _ := c.Get("user_id")
	secret, err := h.service.RotateKey(c, c.Param("id"), userIDString(userID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"connector_secret": secret})
}
func userIDString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
func RegisterAgentBindingRoutes(r *gin.RouterGroup, h *AgentBindingHandler) {
	g := r.Group("/agent-bindings")
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:id", h.Get)
	g.POST("/:id/revoke", h.Revoke)
	g.POST("/:id/keys/rotate", h.Rotate)
}
