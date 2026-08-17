package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"net/http"
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "connector secret required"})
		return
	}
	result, err := h.service.Introspect(c, secret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired connector secret"})
		return
	}
	c.JSON(http.StatusOK, result)
}
func RegisterBindingIntrospectionRoutes(r *gin.Engine, h *BindingIntrospectionHandler) {
	r.POST("/internal/v1/agent-bindings/introspect", h.Introspect)
}
