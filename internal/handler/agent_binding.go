package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/application/repository"
	"github.com/justaboyhai-wq/fmind/internal/application/service/agentbinding"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

type AgentBindingHandler struct {
	service interfaces.AgentBindingService
	apiKeys interfaces.TenantAPIKeyService
}

func NewAgentBindingHandler(service interfaces.AgentBindingService) *AgentBindingHandler {
	return &AgentBindingHandler{service: service}
}

func NewAgentBindingHandlerWithAPIKey(service interfaces.AgentBindingService, apiKeys interfaces.TenantAPIKeyService) *AgentBindingHandler {
	return &AgentBindingHandler{service: service, apiKeys: apiKeys}
}

type agentBindingRequest struct {
	WorkspaceID  string `json:"workspace_id"`
	ProjectID    string `json:"project_id"`
	DepartmentID string `json:"department_id"`
	TeamID       string `json:"team_id" binding:"required"`
	AgentID      string `json:"agent_id" binding:"required"`
	UserID       string `json:"user_id" binding:"required"`
	// UserAPIKeyID is required for the setup-flow path. It remains optional
	// for legacy active bindings so existing integrations can be upgraded
	// without breaking their create payloads.
	UserAPIKeyID     uint64     `json:"user_api_key_id"`
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
		TeamID: req.TeamID, UserID: req.UserID, UserAPIKeyID: req.UserAPIKeyID, AgentID: req.AgentID, TaskID: req.TaskID,
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
	if setup, ok := h.service.(interfaces.AgentBindingSetupRotationService); ok {
		if binding, err := h.service.Get(c, c.Param("id")); err == nil && binding.Status == types.AgentBindingStatusPendingSetup {
			result, rotateErr := setup.RotateSetupKey(c, c.Param("id"), userIDString(userID))
			if rotateErr != nil {
				writeAgentBindingError(c, rotateErr)
				return
			}
			c.JSON(http.StatusOK, result)
			return
		}
	}
	secret, err := h.service.RotateKey(c, c.Param("id"), userIDString(userID))
	if err != nil {
		writeAgentBindingError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"connector_secret": secret, "credential_purpose": "memory_binding_runtime"})
}

func (h *AgentBindingHandler) Setup(c *gin.Context) {
	setup, ok := h.service.(interfaces.AgentBindingSetupService)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent binding setup unavailable"})
		return
	}
	var req interfaces.AgentBindingSetupRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid setup request"})
		return
	}
	req.ConnectorSecret = c.GetHeader("X-FMind-Connector-Secret")
	req.UserAPIKey = strings.TrimSpace(c.GetHeader("X-FMind-User-Key"))
	if h.apiKeys != nil {
		if req.UserAPIKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent binding setup"})
			return
		}
		key, err := h.apiKeys.AuthenticateAPIKey(c, req.UserAPIKey)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent binding setup"})
			return
		}
		req.UserAPIKeyID = key.ID
	}
	result, err := setup.Setup(c, req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent binding setup"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AgentBindingHandler) SetupStatus(c *gin.Context) {
	setup, ok := h.service.(interfaces.AgentBindingSetupService)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent binding setup unavailable"})
		return
	}
	result, err := setup.SetupStatus(c, c.Param("id"))
	if err != nil {
		writeAgentBindingError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
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
	g.GET("/:id/setup-status", h.SetupStatus)
}
