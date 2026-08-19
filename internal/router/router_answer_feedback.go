package router

import (
	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/handler"
)

func RegisterAnswerFeedbackRoutes(r *gin.RouterGroup, h *handler.AnswerFeedbackHandler, g *rbacGuards) {
	user := g.apiKeyGroup(r.Group("/sessions"), apiKeyFullAccess())
	// The POST /sessions/:session_id/messages tree already uses :session_id
	// for message suggestions; keep the wildcard name identical so Gin can
	// share the radix branch without a startup panic.
	user.POST("/:session_id/messages/:message_id/feedback", g.Viewer(), h.Submit)
	user.GET("/:id/messages/:message_id/feedback", g.Viewer(), h.GetForMessage)
	feedback := g.apiKeyGroup(r.Group("/answer-feedbacks"), apiKeyFullAccess())
	feedback.GET("/mine", g.Viewer(), h.Mine)
	feedback.POST("/:id/comments", g.Viewer(), h.Comment)
	feedback.POST("/:id/reopen", g.Viewer(), h.Reopen)
	admin := g.apiKeyGroup(r.Group("/admin/answer-feedbacks"), apiKeyFullAccess())
	admin.GET("", g.Admin(), h.AdminList)
	admin.GET("/stats", g.Admin(), h.AdminStats)
	admin.GET("/:id", g.Admin(), h.AdminDetail)
	admin.PUT("/:id", g.Admin(), h.AdminUpdate)
	admin.POST("/:id/comments", g.Admin(), h.AdminComment)
}
