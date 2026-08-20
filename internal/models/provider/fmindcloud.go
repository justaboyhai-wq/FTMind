package provider

import "github.com/justaboyhai-wq/fmind/internal/types"

const (
	ProviderFTMindCloud ProviderName = "fmindcloud"

	// FTMindCloudBaseURL FTMindCloud 服务硬编码 Base URL（统一入口，路径由各实现拼接）
	FTMindCloudBaseURL = "https://fmind.weixin.qq.com"
)

type FTMindCloudProvider struct{}

func init() {
	Register(&FTMindCloudProvider{})
}

func (p *FTMindCloudProvider) Info() ProviderInfo {
	return ProviderInfo{
		Name:        ProviderFTMindCloud,
		DisplayName: "FTMindCloud",
		Description: "FTMind云服务，模型：chat, embedding, rerank, vlm",
		DefaultURLs: map[types.ModelType]string{
			types.ModelTypeKnowledgeQA: FTMindCloudBaseURL,
			types.ModelTypeEmbedding:   FTMindCloudBaseURL,
			types.ModelTypeRerank:      FTMindCloudBaseURL,
			types.ModelTypeVLLM:        FTMindCloudBaseURL,
		},
		ModelTypes: []types.ModelType{
			types.ModelTypeKnowledgeQA,
			types.ModelTypeEmbedding,
			types.ModelTypeRerank,
			types.ModelTypeVLLM,
		},
		RequiresAuth: true,
	}
}

func (p *FTMindCloudProvider) ValidateConfig(config *Config) error {
	// AppID/AppSecret 通过专用初始化接口写入，此处仅做结构校验。
	// 其中 AppSecret 字段当前实际承载上游 API Key。
	return nil
}
