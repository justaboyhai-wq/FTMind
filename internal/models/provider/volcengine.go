package provider

import (
	"fmt"

	"github.com/justaboyhai-wq/fmind/internal/types"
)

const (
	// VolcengineAgentPlanBaseURL 是 Ark AgentPlan 专属 API Base URL。
	VolcengineAgentPlanBaseURL = "https://ark.cn-beijing.volces.com/api/plan/v3"
)

// VolcengineProvider 实现火山引擎 Ark 的 Provider 接口
type VolcengineProvider struct{}

func init() {
	Register(&VolcengineProvider{})
}

// Info 返回火山引擎 provider 的元数据
func (p *VolcengineProvider) Info() ProviderInfo {
	return ProviderInfo{
		Name:        ProviderVolcengine,
		DisplayName: "火山引擎 AgentPlan",
		Description: "火山引擎 AgentPlan（Ark 兼容接口）",
		DefaultURLs: map[types.ModelType]string{
			types.ModelTypeKnowledgeQA: VolcengineAgentPlanBaseURL,
			types.ModelTypeEmbedding:   VolcengineAgentPlanBaseURL,
			types.ModelTypeVLLM:        VolcengineAgentPlanBaseURL,
		},
		ModelTypes: []types.ModelType{
			types.ModelTypeKnowledgeQA,
			types.ModelTypeEmbedding,
			types.ModelTypeVLLM,
		},
		RequiresAuth: true,
	}
}

// ValidateConfig 验证火山引擎 provider 配置
func (p *VolcengineProvider) ValidateConfig(config *Config) error {
	if config.APIKey == "" {
		return fmt.Errorf("API key is required for Volcengine Ark provider")
	}
	if config.ModelName == "" {
		return fmt.Errorf("model name is required")
	}
	return nil
}
