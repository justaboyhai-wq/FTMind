package handler

import (
	"net/http"

	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// FMindCloudHandler 处理 FMindCloud 凭证管理
type FMindCloudHandler struct {
	svc interfaces.FMindCloudService
}

// NewFMindCloudHandler 构造函数
func NewFMindCloudHandler(svc interfaces.FMindCloudService) *FMindCloudHandler {
	return &FMindCloudHandler{svc: svc}
}

type fmindCloudCredentialsRequest struct {
	AppID     string `json:"app_id"     binding:"required"`
	AppSecret string `json:"app_secret" binding:"required"`
}

// SaveCredentials POST /api/v1/fmindcloud/credentials
// 仅保存 APPID/APPSECRET 凭证到空间配置，不自动创建模型
//
// SaveCredentials godoc
// @Summary      保存 FMindCloud 凭证
// @Description  保存 APPID/APPSECRET 到当前空间配置（不自动创建模型）
// @Tags         FMindCloud
// @Accept       json
// @Produce      json
// @Param        request  body      map[string]interface{}  true  "{app_id, app_secret}"
// @Success      200      {object}  map[string]interface{}  "success: true"
// @Failure      400      {object}  map[string]interface{}  "请求参数错误"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /fmindcloud/credentials [post]
func (h *FMindCloudHandler) SaveCredentials(c *gin.Context) {
	var req fmindCloudCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.SaveCredentials(c.Request.Context(), req.AppID, req.AppSecret); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "凭证保存成功"})
}

// Status GET /api/v1/models/fmindcloud/status
// 检查当前空间的 FMindCloud 凭证是否完好，如需重新初始化则返回 needs_reinit=true
//
// Status godoc
// @Summary      检查 FMindCloud 凭证状态
// @Description  检查当前空间的 FMindCloud 凭证是否完好；needs_reinit=true 表示需要重新保存
// @Tags         FMindCloud
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "凭证状态"
// @Failure      500  {object}  map[string]interface{}  "服务器错误"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /models/fmindcloud/status [get]
func (h *FMindCloudHandler) Status(c *gin.Context) {
	result, err := h.svc.CheckStatus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
