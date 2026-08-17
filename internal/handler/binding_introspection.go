package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

type BindingIntrospectionHandler struct {
	service interfaces.AgentBindingService
}

func NewBindingIntrospectionHandler(service interfaces.AgentBindingService) *BindingIntrospectionHandler {
	return &BindingIntrospectionHandler{service: service}
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
