package interfaces

import (
	"context"

	"github.com/justaboyhai-wq/fmind/internal/types"
)

// FTMindCloudService 处理 FTMindCloud 凭证管理
type FTMindCloudService interface {
	// SaveCredentials 仅保存 APPID/APPSECRET 凭证到空间配置，不自动创建模型
	SaveCredentials(ctx context.Context, appID, appSecret string) error
	// CheckStatus 检查当前空间的 FTMindCloud 凭证是否可正常解密
	// needsReinit=true 表示加密状态已损坏（salt 变更等），需要用户重新填写凭证
	CheckStatus(ctx context.Context) (*types.FTMindCloudStatusResult, error)
}
