package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/application/service/memorywiki"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"net/http"
)

type MemoryWikiHandler struct{ service *memorywiki.Service }

func NewMemoryWikiHandler(service *memorywiki.Service) *MemoryWikiHandler {
	return &MemoryWikiHandler{service: service}
}
func (h *MemoryWikiHandler) Submit(c *gin.Context) {
	var p types.MemoryWikiPublication
	if c.ShouldBindJSON(&p) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if p.TenantID != 0 && p.TenantID != tenantID {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant_id cannot cross the authenticated tenant"})
		return
	}
	p.TenantID = tenantID
	if err := h.service.Submit(c, &p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, p)
}
func (h *MemoryWikiHandler) List(c *gin.Context) {
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	pages, err := h.service.List(c, tenantID, c.Query("status"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pages)
}
func (h *MemoryWikiHandler) Review(c *gin.Context) {
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	reviewer, _ := types.UserIDFromContext(c.Request.Context())
	var req struct {
		Approve bool `json:"approve"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.service.Review(c, tenantID, c.Param("id"), reviewer, req.Approve); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *MemoryWikiHandler) Publish(c *gin.Context) {
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	var req struct {
		KnowledgeBaseID string `json:"knowledge_base_id"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	page, err := h.service.PublishApproved(c, tenantID, c.Param("id"), req.KnowledgeBaseID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, page)
}
func RegisterMemoryWikiRoutes(r *gin.RouterGroup, h *MemoryWikiHandler) {
	g := r.Group("/memory-wiki")
	g.POST("", h.Submit)
	g.GET("", h.List)
	g.POST("/:id/review", h.Review)
	g.POST("/:id/publish", h.Publish)
}
