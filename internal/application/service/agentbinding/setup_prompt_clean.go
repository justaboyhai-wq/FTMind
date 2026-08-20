package agentbinding

import (
	"fmt"
	"strings"

	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

// buildSetupArtifactsClean is the canonical user-facing setup artifact
// generator. It never embeds an existing FTMind API key or a direct
// MemoryCore credential.
func buildSetupArtifactsClean(binding *types.AgentBinding, secret string) (*interfaces.AgentBindingSetupManifest, string, error) {
	fmind, _, proxy, err := setupEndpoints(binding.ConnectorType)
	if err != nil {
		return nil, "", err
	}
	manifest := &interfaces.AgentBindingSetupManifest{
		BindingID: binding.ID, ExternalAgent: binding.ExternalAgent,
		ConnectorType: binding.ConnectorType, FTMindEndpoint: fmind,
		MemoryProxyEndpoint: proxy,
		Capabilities: append([]string(nil), binding.CapabilityScopes...),
		AssetScopes: append([]string(nil), binding.AssetScopes...),
	}
	template := setupTemplateClean(binding.ConnectorType, fmind, proxy)
	prompt := fmt.Sprintf(`FTMind 外部 Agent 一键接入

目标 Agent：%s
接入类型：%s
Binding ID：%s
FTMind 地址：%s
MemoryProxy 地址：%s

请在外部 Agent 的安全环境变量中配置（不要提交到代码库）：
FMIND_USER_API_KEY="${FMIND_USER_API_KEY}"
FMIND_AGENT_SETUP_KEY="%s"

这是一次性接入密钥，默认 30 分钟有效且只能兑换一次。不要把任何密钥写入日志、URL、模型上下文或代码仓库。

已启用能力：
- %s

资产范围：
- %s

%s

配置完成后执行 setup：
POST %s/internal/v1/agent-bindings/setup
X-FMind-Connector-Secret: $FMIND_AGENT_SETUP_KEY
X-FMind-User-Key: $FMIND_USER_API_KEY
请求体：{"binding_id":"%s","external_agent":"%s","connector_type":"%s","client_version":"<your-version>"}

setup 成功后立即删除 FMIND_AGENT_SETUP_KEY，只保存响应中的运行期 Agent Key。后续请求必须同时携带用户 API Key 和 Agent Key，由 FTMind 按当前角色、能力和资产范围重新鉴权。`,
		binding.ExternalAgent, binding.ConnectorType, binding.ID, fmind, proxy, secret,
		strings.Join(binding.CapabilityScopes, "\n- "), strings.Join(binding.AssetScopes, "\n- "),
		template, fmind, binding.ID, binding.ExternalAgent, binding.ConnectorType)
	return manifest, prompt, nil
}

func setupTemplateClean(connector, fmind, proxy string) string {
	switch connector {
	case "openclaw_plugin":
		return fmt.Sprintf("OpenClaw：将 FMIND_ENDPOINT=%s、FMIND_USER_API_KEY=${FMIND_USER_API_KEY} 和 FMIND_AGENT_SETUP_KEY=${FMIND_AGENT_SETUP_KEY} 写入安全环境变量，完成 setup 后仅保留运行期 Agent Key。", fmind)
	case "openai_proxy", "anthropic_proxy":
		return fmt.Sprintf("代理：将 BASE_URL=%s 和 FMIND_USER_API_KEY=${FMIND_USER_API_KEY} 配置到代理环境，setup 成功后使用运行期 Agent Key 作为代理认证。", proxy)
	case "mcp":
		return fmt.Sprintf("MCP：将 Cognition 服务配置为 %s/mcp/cognition，并通过安全环境变量注入用户 API Key 与运行期 Agent Key。", fmind)
	default:
		return fmt.Sprintf("通用 SDK：配置 FMIND_ENDPOINT=%s 和 FMIND_USER_API_KEY=${FMIND_USER_API_KEY}，先完成 setup，再调用 recall/capture。", fmind)
	}
}
