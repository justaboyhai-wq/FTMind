package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/authz"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

type BindingIntrospectionHandler struct {
	service    interfaces.AgentBindingService
	authorizer *authz.Service
	apiKeys    interfaces.TenantAPIKeyService
}

func NewBindingIntrospectionHandler(service interfaces.AgentBindingService) *BindingIntrospectionHandler {
	return &BindingIntrospectionHandler{service: service}
}

func NewBindingIntrospectionHandlerWithAuthorization(service interfaces.AgentBindingService, authorizer *authz.Service) *BindingIntrospectionHandler {
	return &BindingIntrospectionHandler{service: service, authorizer: authorizer}
}

func NewBindingIntrospectionHandlerWithAuthorizationAndAPIKey(service interfaces.AgentBindingService, authorizer *authz.Service, apiKeys interfaces.TenantAPIKeyService) *BindingIntrospectionHandler {
	return &BindingIntrospectionHandler{service: service, authorizer: authorizer, apiKeys: apiKeys}
}
func (h *BindingIntrospectionHandler) Introspect(c *gin.Context) {
	secret := c.GetHeader("X-FMind-Connector-Secret")
	if secret == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid connector credentials"})
		return
	}
	result, err := h.service.Introspect(c, secret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid connector credentials"})
		return
	}
	if h.apiKeys != nil {
		userSecret := strings.TrimSpace(c.GetHeader("X-FMind-User-Key"))
		userKey, keyErr := h.apiKeys.AuthenticateAPIKey(c, userSecret)
		if keyErr != nil || userKey == nil || userKey.TenantID != result.Context.TenantID ||
			(result.Context.UserAPIKeyID != 0 && userKey.ID != result.Context.UserAPIKeyID) ||
			(userKey.UserID != "" && userKey.UserID != result.Context.UserID) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user credentials"})
			return
		}
	}
	c.JSON(http.StatusOK, result)
}

func (h *BindingIntrospectionHandler) Verify(c *gin.Context) {
	token, ok := bindingTokenFromHeaders(c)
	c.Request.Header.Del("Authorization")
	c.Request.Header.Del("X-FMind-Binding-Token")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid binding token"})
		return
	}
	value, err := h.service.VerifyBindingToken(c, token)
	if err != nil || value == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid binding token"})
		return
	}
	if h.authorizer != nil {
		if err := h.authorizer.ValidateBindingContext(c, *value); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid binding token"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"context": value})
}

func bindingTokenFromHeaders(c *gin.Context) (string, bool) {
	authorization := strings.TrimSpace(c.GetHeader("Authorization"))
	dedicated := strings.TrimSpace(c.GetHeader("X-FMind-Binding-Token"))
	bearer := ""
	if authorization != "" {
		parts := strings.Fields(authorization)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			return "", false
		}
		bearer = parts[1]
	}
	if bearer != "" && dedicated != "" && bearer != dedicated {
		return "", false
	}
	if bearer != "" {
		return bearer, true
	}
	if dedicated != "" {
		return dedicated, true
	}
	return "", false
}

func RegisterBindingIntrospectionRoutes(r *gin.Engine, h *BindingIntrospectionHandler) {
	r.POST("/internal/v1/agent-bindings/introspect", h.Introspect)
	r.POST("/internal/v1/agent-bindings/verify", h.Verify)
}
