package handler

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/application/service"
	apperrors "github.com/justaboyhai-wq/fmind/internal/errors"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"net/http"
	"strconv"
)

type AnswerFeedbackHandler struct {
	feedback interfaces.AnswerFeedbackService
	sessions interfaces.SessionService
	messages interfaces.MessageService
}

func NewAnswerFeedbackHandler(f interfaces.AnswerFeedbackService, s interfaces.SessionService, m interfaces.MessageService) *AnswerFeedbackHandler {
	return &AnswerFeedbackHandler{feedback: f, sessions: s, messages: m}
}
func feedbackActor(c *gin.Context) (string, uint64, bool) {
	v, ok := c.Get(types.UserIDContextKey.String())
	if !ok {
		return "", 0, false
	}
	u, _ := v.(string)
	t := c.GetUint64(types.TenantIDContextKey.String())
	return u, t, u != "" && t > 0
}
func feedbackError(c *gin.Context, e error) {
	if errors.Is(e, service.ErrFeedbackInvalid) {
		c.Error(apperrors.NewBadRequestError(e.Error()))
		return
	}
	if errors.Is(e, service.ErrFeedbackForbidden) {
		c.Error(apperrors.NewForbiddenError(e.Error()))
		return
	}
	if errors.Is(e, service.ErrFeedbackNotFound) {
		c.Error(apperrors.NewNotFoundError(e.Error()))
		return
	}
	c.Error(apperrors.NewInternalServerError(e.Error()))
}
func feedbackSessionID(c *gin.Context) string {
	if v := c.Param("session_id"); v != "" {
		return v
	}
	return c.Param("id")
}
func (h *AnswerFeedbackHandler) Submit(c *gin.Context) {
	u, t, ok := feedbackActor(c)
	if !ok {
		c.Error(apperrors.NewUnauthorizedError("user not found"))
		return
	}
	sid, mid := feedbackSessionID(c), c.Param("message_id")
	sess, e := h.sessions.GetSession(c.Request.Context(), sid)
	if e != nil {
		feedbackError(c, e)
		return
	}
	msg, e := h.messages.GetMessage(c.Request.Context(), sid, mid)
	if e != nil {
		feedbackError(c, e)
		return
	}
	var req types.AnswerFeedbackRequest
	if c.ShouldBindJSON(&req) != nil {
		c.Error(apperrors.NewBadRequestError("invalid feedback body"))
		return
	}
	f, e := h.feedback.Submit(c.Request.Context(), t, u, "user", sess, msg, req)
	if e != nil {
		feedbackError(c, e)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": f})
}
func (h *AnswerFeedbackHandler) GetForMessage(c *gin.Context) {
	u, t, ok := feedbackActor(c)
	if !ok {
		c.Error(apperrors.NewUnauthorizedError("user not found"))
		return
	}
	if _, e := h.messages.GetMessage(c.Request.Context(), feedbackSessionID(c), c.Param("message_id")); e != nil {
		feedbackError(c, e)
		return
	}
	f, e := h.feedback.GetMineForMessage(c.Request.Context(), t, u, c.Param("message_id"))
	if e != nil {
		feedbackError(c, e)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": f})
}
func (h *AnswerFeedbackHandler) Mine(c *gin.Context) {
	u, t, ok := feedbackActor(c)
	if !ok {
		c.Error(apperrors.NewUnauthorizedError("user not found"))
		return
	}
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	z, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	a, n, e := h.feedback.ListMine(c.Request.Context(), t, u, p, z)
	if e != nil {
		feedbackError(c, e)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": a, "total": n, "page": p, "page_size": z})
}
func (h *AnswerFeedbackHandler) Comment(c *gin.Context) {
	u, t, ok := feedbackActor(c)
	if !ok {
		c.Error(apperrors.NewUnauthorizedError("user not found"))
		return
	}
	var req types.AnswerFeedbackCommentRequest
	if c.ShouldBindJSON(&req) != nil {
		c.Error(apperrors.NewBadRequestError("invalid comment body"))
		return
	}
	if e := h.feedback.Comment(c.Request.Context(), t, u, "user", c.Param("id"), req.Comment); e != nil {
		feedbackError(c, e)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
func (h *AnswerFeedbackHandler) Reopen(c *gin.Context) {
	u, t, ok := feedbackActor(c)
	if !ok {
		c.Error(apperrors.NewUnauthorizedError("user not found"))
		return
	}
	var req types.AnswerFeedbackCommentRequest
	_ = c.ShouldBindJSON(&req)
	if e := h.feedback.Reopen(c.Request.Context(), t, u, c.Param("id"), req.Comment); e != nil {
		feedbackError(c, e)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
func (h *AnswerFeedbackHandler) AdminList(c *gin.Context) {
	_, t, ok := feedbackActor(c)
	if !ok {
		c.Error(apperrors.NewUnauthorizedError("user not found"))
		return
	}
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	z, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	a, n, e := h.feedback.List(c.Request.Context(), t, types.AnswerFeedbackListQuery{Status: c.Query("status"), Category: c.Query("category"), Page: p, PageSize: z})
	if e != nil {
		feedbackError(c, e)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": a, "total": n, "page": p, "page_size": z})
}
func (h *AnswerFeedbackHandler) AdminStats(c *gin.Context) {
	_, t, ok := feedbackActor(c)
	if !ok {
		c.Error(apperrors.NewUnauthorizedError("user not found"))
		return
	}
	stats := map[string]int64{}
	for _, status := range []string{"pending", "reviewing", "needs_info", "fixing", "resolved", "dismissed"} {
		_, n, err := h.feedback.List(c.Request.Context(), t, types.AnswerFeedbackListQuery{Status: status, Page: 1, PageSize: 1})
		if err != nil {
			feedbackError(c, err)
			return
		}
		stats[status] = n
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}
func (h *AnswerFeedbackHandler) AdminDetail(c *gin.Context) {
	_, t, ok := feedbackActor(c)
	if !ok {
		c.Error(apperrors.NewUnauthorizedError("user not found"))
		return
	}
	f, e := h.feedback.Get(c.Request.Context(), t, c.Param("id"))
	if e != nil {
		feedbackError(c, e)
		return
	}
	ev, e := h.feedback.Events(c.Request.Context(), t, f.ID)
	if e != nil {
		feedbackError(c, e)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": f, "events": ev})
}
func (h *AnswerFeedbackHandler) AdminUpdate(c *gin.Context) {
	u, t, ok := feedbackActor(c)
	if !ok {
		c.Error(apperrors.NewUnauthorizedError("user not found"))
		return
	}
	var req types.AnswerFeedbackAdminUpdate
	if c.ShouldBindJSON(&req) != nil {
		c.Error(apperrors.NewBadRequestError("invalid update body"))
		return
	}
	f, e := h.feedback.Update(c.Request.Context(), t, u, c.Param("id"), req)
	if e != nil {
		feedbackError(c, e)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": f})
}
func (h *AnswerFeedbackHandler) AdminComment(c *gin.Context) {
	u, t, ok := feedbackActor(c)
	if !ok {
		c.Error(apperrors.NewUnauthorizedError("user not found"))
		return
	}
	var req types.AnswerFeedbackCommentRequest
	if c.ShouldBindJSON(&req) != nil {
		c.Error(apperrors.NewBadRequestError("invalid comment body"))
		return
	}
	if e := h.feedback.Comment(c.Request.Context(), t, u, "admin", c.Param("id"), req.Comment); e != nil {
		feedbackError(c, e)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
