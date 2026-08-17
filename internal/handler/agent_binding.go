package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/application/repository"
	"github.com/justaboyhai-wq/fmind/internal/application/service/agentbinding"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

type AgentBindingHandler struct {
	service interfaces.AgentBindingService
}

func NewAgentBindingHandler(service interfaces.AgentBindingService) *AgentBindingHandler {
	return &AgentBindingHandler{service: service}
}

type agentBindingRequest struct {
	WorkspaceID      string     `json:"workspace_id"`
	ProjectID        string     `json:"project_id"`
	DepartmentID     string     `json:"department_id"`
	TeamID           string     `json:"team_id" binding:"required"`
	AgentID          string     `json:"agent_id" binding:"required"`
	UserID           string     `json:"user_id" binding:"required"`
	TaskID           string     `json:"task_id"`
	ExternalAgent    string     `json:"external_agent" binding:"required"`
	ConnectorType    string     `json:"connector_type" binding:"required"`
	CapabilityScopes []string   `json:"capability_scopes"`
	AssetScopes      []string   `json:"asset_scopes"`
	CaptureEnabled   bool       `json:"capture_enabled"`
	RecallEnabled    bool       `json:"recall_enabled"`
	L3WikiEnabled    bool       `json:"l3_wiki_enabled"`
	L3ReviewRequired bool       `json:"l3_review_required"`
	ExpiresAt        *time.Time `json:"expires_at"`
}

func (h *AgentBindingHandler) Create(c *gin.Context) {
	var req agentBindingRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	userID, _ := c.Get(types.UserIDContextKey.String())
	result, err := h.service.Create(c, interfaces.AgentBindingCreateRequest{
		WorkspaceID: req.WorkspaceID, ProjectID: req.ProjectID, DepartmentID: req.DepartmentID,
		TeamID: req.TeamID, UserID: req.UserID, AgentID: req.AgentID, TaskID: req.TaskID,
		ExternalAgent: req.ExternalAgent, ConnectorType: req.ConnectorType,
		CapabilityScopes: req.CapabilityScopes, AssetScopes: req.AssetScopes,
		CaptureEnabled: req.CaptureEnabled, RecallEnabled: req.RecallEnabled,
		L3WikiEnabled: req.L3WikiEnabled, L3ReviewRequired: req.L3ReviewRequired,
		ExpiresAt: req.ExpiresAt, CreatedBy: userIDString(userID),
	})
	if err != nil {
		writeAgentBindingError(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}
func (h *AgentBindingHandler) List(c *gin.Context) {
	result, err := h.service.List(c)
	if err != nil {
		writeAgentBindingError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *AgentBindingHandler) Get(c *gin.Context) {
	result, err := h.service.Get(c, c.Param("id"))
	if err != nil {
		writeAgentBindingError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
func (h *AgentBindingHandler) Revoke(c *gin.Context) {
	if err := h.service.Revoke(c, c.Param("id")); err != nil {
		writeAgentBindingError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *AgentBindingHandler) Rotate(c *gin.Context) {
	userID, _ := c.Get(types.UserIDContextKey.String())
	secret, err := h.service.RotateKey(c, c.Param("id"), userIDString(userID))
	if err != nil {
		writeAgentBindingError(c, err)
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

func writeAgentBindingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, agentbinding.ErrBindingAdminRequired):
		c.JSON(http.StatusForbidden, gin.H{"error": "agent binding management requires tenant admin"})
	case errors.Is(err, repository.ErrAgentBindingNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "agent binding not found"})
	case errors.Is(err, repository.ErrAgentBindingPolicyVersionOverflow):
		c.JSON(http.StatusConflict, gin.H{"error": "agent binding policy version cannot be incremented"})
	case errors.Is(err, agentbinding.ErrUnsupportedConnector),
		errors.Is(err, agentbinding.ErrInvalidBinding),
		errors.Is(err, agentbinding.ErrUnverifiableBindingScope):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent binding"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

func RegisterAgentBindingRoutes(r *gin.RouterGroup, h *AgentBindingHandler) {
	g := r.Group("/agent-bindings")
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:id", h.Get)
	g.POST("/:id/revoke", h.Revoke)
	g.POST("/:id/keys/rotate", h.Rotate)
}
