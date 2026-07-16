package handler

import (
	"net/http"

	"github.com/justaboyhai-wq/keystone/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// KeystoneCloudHandler 处理 KeystoneCloud 凭证管理
type KeystoneCloudHandler struct {
	svc interfaces.KeystoneCloudService
}

// NewKeystoneCloudHandler 构造函数
func NewKeystoneCloudHandler(svc interfaces.KeystoneCloudService) *KeystoneCloudHandler {
	return &KeystoneCloudHandler{svc: svc}
}

type keystoneCloudCredentialsRequest struct {
	AppID     string `json:"app_id"     binding:"required"`
	AppSecret string `json:"app_secret" binding:"required"`
}

// SaveCredentials POST /api/v1/keystonecloud/credentials
// 仅保存 APPID/APPSECRET 凭证到空间配置，不自动创建模型
//
// SaveCredentials godoc
// @Summary      保存 KeystoneCloud 凭证
// @Description  保存 APPID/APPSECRET 到当前空间配置（不自动创建模型）
// @Tags         KeystoneCloud
// @Accept       json
// @Produce      json
// @Param        request  body      map[string]interface{}  true  "{app_id, app_secret}"
// @Success      200      {object}  map[string]interface{}  "success: true"
// @Failure      400      {object}  map[string]interface{}  "请求参数错误"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /keystonecloud/credentials [post]
func (h *KeystoneCloudHandler) SaveCredentials(c *gin.Context) {
	var req keystoneCloudCredentialsRequest
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

// Status GET /api/v1/models/keystonecloud/status
// 检查当前空间的 KeystoneCloud 凭证是否完好，如需重新初始化则返回 needs_reinit=true
//
// Status godoc
// @Summary      检查 KeystoneCloud 凭证状态
// @Description  检查当前空间的 KeystoneCloud 凭证是否完好；needs_reinit=true 表示需要重新保存
// @Tags         KeystoneCloud
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "凭证状态"
// @Failure      500  {object}  map[string]interface{}  "服务器错误"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /models/keystonecloud/status [get]
func (h *KeystoneCloudHandler) Status(c *gin.Context) {
	result, err := h.svc.CheckStatus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
