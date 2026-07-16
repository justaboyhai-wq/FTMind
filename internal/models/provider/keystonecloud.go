package provider

import "github.com/justaboyhai-wq/keystone/internal/types"

const (
	ProviderKeystoneCloud ProviderName = "keystonecloud"

	// KeystoneCloudBaseURL KeystoneCloud 服务硬编码 Base URL（统一入口，路径由各实现拼接）
	KeystoneCloudBaseURL = "https://keystone.weixin.qq.com"
)

type KeystoneCloudProvider struct{}

func init() {
	Register(&KeystoneCloudProvider{})
}

func (p *KeystoneCloudProvider) Info() ProviderInfo {
	return ProviderInfo{
		Name:        ProviderKeystoneCloud,
		DisplayName: "KeystoneCloud",
		Description: "Keystone云服务，模型：chat, embedding, rerank, vlm",
		DefaultURLs: map[types.ModelType]string{
			types.ModelTypeKnowledgeQA: KeystoneCloudBaseURL,
			types.ModelTypeEmbedding:   KeystoneCloudBaseURL,
			types.ModelTypeRerank:      KeystoneCloudBaseURL,
			types.ModelTypeVLLM:        KeystoneCloudBaseURL,
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

func (p *KeystoneCloudProvider) ValidateConfig(config *Config) error {
	// AppID/AppSecret 通过专用初始化接口写入，此处仅做结构校验。
	// 其中 AppSecret 字段当前实际承载上游 API Key。
	return nil
}
