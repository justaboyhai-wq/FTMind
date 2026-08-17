package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/application/repository"
	"github.com/justaboyhai-wq/fmind/internal/application/service/memorywiki"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

const maxMemoryReviewCommentBytes = 8 << 10

type memoryWikiReviewService interface {
	List(context.Context, uint64, string) ([]*types.MemoryWikiPublication, error)
	GetReview(context.Context, uint64, string) (*interfaces.ExternalMemoryProjection, error)
	ApprovePublication(context.Context, uint64, string, string, string) (*types.MemoryReviewTask, error)
	RejectPublication(context.Context, uint64, string, string, string) (*types.MemoryReviewTask, error)
	RequestPublicationChanges(context.Context, uint64, string, string, string) (*types.MemoryReviewTask, error)
	PublishApproved(context.Context, uint64, string, string) (*types.WikiPage, error)
}

type MemoryWikiHandler struct{ service memoryWikiReviewService }

func NewMemoryWikiHandler(service *memorywiki.Service) *MemoryWikiHandler {
	return &MemoryWikiHandler{service: service}
}

func newMemoryWikiHandler(service memoryWikiReviewService) *MemoryWikiHandler {
	return &MemoryWikiHandler{service: service}
}

func (h *MemoryWikiHandler) ListReviews(c *gin.Context) {
	publications, err := h.service.List(c.Request.Context(), c.GetUint64(types.TenantIDContextKey.String()), c.Query("status"))
	if err != nil {
		writeMemoryWikiError(c, err)
		return
	}
	c.JSON(http.StatusOK, publications)
}

func (h *MemoryWikiHandler) GetReview(c *gin.Context) {
	projection, err := h.service.GetReview(c.Request.Context(), c.GetUint64(types.TenantIDContextKey.String()), c.Param("id"))
	if err != nil {
		writeMemoryWikiError(c, err)
		return
	}
	c.JSON(http.StatusOK, projection)
}

type memoryReviewDecisionRequest struct {
	Comment string `json:"comment"`
}

func (h *MemoryWikiHandler) Approve(c *gin.Context)        { h.review(c, "approve") }
func (h *MemoryWikiHandler) Reject(c *gin.Context)         { h.review(c, "reject") }
func (h *MemoryWikiHandler) RequestChanges(c *gin.Context) { h.review(c, "request_changes") }

func (h *MemoryWikiHandler) review(c *gin.Context, decision string) {
	var request memoryReviewDecisionRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
	}
	request.Comment = strings.TrimSpace(request.Comment)
	if len(request.Comment) > maxMemoryReviewCommentBytes || (decision == "request_changes" && request.Comment == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid review comment"})
		return
	}
	reviewer, ok := types.UserIDFromContext(c.Request.Context())
	if !ok || strings.TrimSpace(reviewer) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "reviewer identity is required"})
		return
	}
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	var (
		review *types.MemoryReviewTask
		err    error
	)
	switch decision {
	case "approve":
		review, err = h.service.ApprovePublication(c.Request.Context(), tenantID, c.Param("id"), reviewer, request.Comment)
	case "reject":
		review, err = h.service.RejectPublication(c.Request.Context(), tenantID, c.Param("id"), reviewer, request.Comment)
	case "request_changes":
		review, err = h.service.RequestPublicationChanges(c.Request.Context(), tenantID, c.Param("id"), reviewer, request.Comment)
	default:
		err = errors.New("unsupported review decision")
	}
	if err != nil {
		writeMemoryWikiError(c, err)
		return
	}
	c.JSON(http.StatusOK, review)
}

func (h *MemoryWikiHandler) Publish(c *gin.Context) {
	var request struct {
		KnowledgeBaseID string `json:"knowledge_base_id"`
	}
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
	}
	page, err := h.service.PublishApproved(
		c.Request.Context(), c.GetUint64(types.TenantIDContextKey.String()), c.Param("id"), strings.TrimSpace(request.KnowledgeBaseID),
	)
	if err != nil {
		writeMemoryWikiError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func writeMemoryWikiError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, memorywiki.ErrMemoryWikiReviewerRequired):
		c.JSON(http.StatusForbidden, gin.H{"error": "memory Wiki review permission denied"})
	case errors.Is(err, repository.ErrMemoryWikiPublicationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "memory review not found"})
	case errors.Is(err, repository.ErrExternalMemoryStateConflict), errors.Is(err, repository.ErrWikiPageConflict), errors.Is(err, memorywiki.ErrStaleMemoryWikiVersion):
		c.JSON(http.StatusConflict, gin.H{"error": "memory review state conflict"})
	case errors.Is(err, memorywiki.ErrInvalidMemoryWikiTarget), errors.Is(err, memorywiki.ErrMemoryReviewNotApproved),
		errors.Is(err, memorywiki.ErrMemoryClaimEvidenceRequired), errors.Is(err, memorywiki.ErrMemoryClaimSourceMismatch):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "memory review cannot be published"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "memory review operation failed"})
	}
}
