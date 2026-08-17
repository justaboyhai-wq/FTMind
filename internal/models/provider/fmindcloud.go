package provider

import "github.com/justaboyhai-wq/fmind/internal/types"

const (
	ProviderFMindCloud ProviderName = "fmindcloud"

	// FMindCloudBaseURL FMindCloud 服务硬编码 Base URL（统一入口，路径由各实现拼接）
	FMindCloudBaseURL = "https://fmind.weixin.qq.com"
)

type FMindCloudProvider struct{}

func init() {
	Register(&FMindCloudProvider{})
}

func (p *FMindCloudProvider) Info() ProviderInfo {
	return ProviderInfo{
		Name:        ProviderFMindCloud,
		DisplayName: "FMindCloud",
		Description: "FMind云服务，模型：chat, embedding, rerank, vlm",
		DefaultURLs: map[types.ModelType]string{
			types.ModelTypeKnowledgeQA: FMindCloudBaseURL,
			types.ModelTypeEmbedding:   FMindCloudBaseURL,
			types.ModelTypeRerank:      FMindCloudBaseURL,
			types.ModelTypeVLLM:        FMindCloudBaseURL,
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

func (p *FMindCloudProvider) ValidateConfig(config *Config) error {
	// AppID/AppSecret 通过专用初始化接口写入，此处仅做结构校验。
	// 其中 AppSecret 字段当前实际承载上游 API Key。
	return nil
}
