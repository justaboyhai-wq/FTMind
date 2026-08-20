package agentbinding

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

const (
	bindingTokenIssuer     = "fmind"
	bindingTokenAudience   = "fmind-memory"
	bindingTokenTTL        = 5 * time.Minute
	bindingTokenClockSkew  = 5 * time.Second
	minimumSecretBytes     = 32
	maxExternalAgentLength = 128
	maxAssetScopeLength    = 256
	maxAssetScopes         = 64
)

var (
	ErrUnsupportedConnector = errors.New("unsupported agent connector")
	ErrInvalidBinding       = errors.New("invalid agent binding")
	ErrSetupUnavailable     = errors.New("agent binding setup unavailable")
	ErrBindingAdminRequired = errors.New("agent binding management requires tenant admin")
	supportedConnectors     = map[string]struct{}{
		"openclaw_plugin": {}, "hermes_provider": {}, "openai_proxy": {},
		"anthropic_proxy": {}, "generic_sdk": {}, "mcp": {},
	}
	// Closed allowlist for the frozen Cognition tools:
	// memory_get_context -> memory.context; memory_search -> memory.recall;
	// memory_capture_turn -> memory.capture; memory_confirm_candidate ->
	// memory.confirm; knowledge_search/wiki_get_page/document_read/
	// context_assemble -> their dotted equivalents. memory.l3.publish is the
	// separate reviewed publication capability and never bypasses review.
	supportedCapabilities = map[string]struct{}{
		"memory.context": {}, "memory.capture": {}, "memory.recall": {},
		"memory.confirm": {}, "memory.publish": {}, "memory.l3.publish": {}, "knowledge.search": {},
		"wiki.get": {}, "document.read": {}, "context.assemble": {},
	}
	supportedAssetKinds = map[string]struct{}{
		"tenant": {}, "team": {}, "department": {}, "workspace": {},
		"project": {}, "task": {}, "knowledge_base": {}, "wiki_page": {}, "document": {},
	}
	assetScopeIDPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	externalAgentPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
)

type Service struct {
	bindings  interfaces.AgentBindingRepository
	keys      interfaces.AgentBindingKeyRepository
	validator interfaces.AgentBindingScopeValidator
}

func NewService(bindings interfaces.AgentBindingRepository, keys interfaces.AgentBindingKeyRepository, validator interfaces.AgentBindingScopeValidator) interfaces.AgentBindingService {
	return &Service{bindings: bindings, keys: keys, validator: validator}
}

func (s *Service) Create(ctx context.Context, req interfaces.AgentBindingCreateRequest) (*interfaces.AgentBindingCreateResult, error) {
	if err := requireBindingAdmin(ctx); err != nil {
		return nil, err
	}
	tenantID, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateCreateRequest(&req); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBinding, err)
	}
	// TenantID is deliberately sourced only from the authenticated context. It
	// is the organization security boundary; no request body can override it.
	setupFlow := setupFlowEnabled()
	if setupFlow && req.UserAPIKeyID == 0 {
		return nil, fmt.Errorf("%w: user_api_key_id is required when setup flow is enabled", ErrInvalidBinding)
	}
	binding := &types.AgentBinding{
		ID: uuid.NewString(), TenantID: tenantID, DepartmentID: req.DepartmentID,
		TeamID: req.TeamID, WorkspaceID: req.WorkspaceID, ProjectID: req.ProjectID,
		UserID: req.UserID, AgentID: req.AgentID, TaskID: req.TaskID,
		UserAPIKeyID:  req.UserAPIKeyID,
		ExternalAgent: req.ExternalAgent, ConnectorType: req.ConnectorType,
		Status: types.AgentBindingStatusActive, CaptureEnabled: req.CaptureEnabled,
		RecallEnabled: req.RecallEnabled, L3WikiEnabled: req.L3WikiEnabled,
		L3ReviewRequired: req.L3ReviewRequired, CapabilityScopes: types.StringArray(req.CapabilityScopes),
		AssetScopes: types.StringArray(req.AssetScopes), PolicyVersion: 1,
		CreatedBy: req.CreatedBy, ExpiresAt: req.ExpiresAt,
	}
	if s.validator == nil {
		return nil, ErrUnverifiableBindingScope
	}
	if _, err := s.validator.ValidateCreate(ctx, binding); err != nil {
		return nil, err
	}
	secret, err := newSecret()
	if err != nil {
		return nil, err
	}
	hash, err := hashConnectorSecret(secret)
	if err != nil {
		return nil, err
	}
	keyPurpose := "memory_binding_runtime"
	keyExpiresAt := req.ExpiresAt
	if setupFlow {
		binding.Status = types.AgentBindingStatusPendingSetup
		setupExpires := time.Now().UTC().Add(30 * time.Minute)
		binding.SetupExpiresAt = &setupExpires
		keyPurpose = "memory_binding_setup"
		keyExpiresAt = &setupExpires
	}
	key := &types.AgentBindingKey{
		ID: uuid.NewString(), BindingID: binding.ID, TenantID: tenantID,
		KeyPrefix: connectorSecretPrefix(secret), KeyHash: hash,
		CreatedBy: req.CreatedBy, ExpiresAt: keyExpiresAt, Purpose: keyPurpose,
	}
	var manifest *interfaces.AgentBindingSetupManifest
	var prompt string
	if setupFlow {
		// Validate deployment endpoints before persisting a binding. A setup
		// binding without a usable public endpoint cannot ever be activated.
		manifest, prompt, err = buildSetupArtifacts(binding, secret)
		if err != nil {
			return nil, err
		}
	}
	if err := s.bindings.CreateAgentBindingWithKey(ctx, binding, key); err != nil {
		return nil, err
	}
	result := &interfaces.AgentBindingCreateResult{Binding: binding, ConnectorSecret: secret}
	if setupFlow {
		result.CredentialPurpose = "memory_binding_setup"
		result.SetupExpiresAt = binding.SetupExpiresAt
		result.SetupManifest = manifest
		result.SetupPrompt = prompt
	} else {
		result.CredentialPurpose = "memory_binding_runtime"
	}
	return result, nil
}

func (s *Service) List(ctx context.Context) ([]*types.AgentBinding, error) {
	if err := requireBindingAdmin(ctx); err != nil {
		return nil, err
	}
	tenantID, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return s.bindings.ListAgentBindings(ctx, tenantID)
}

func (s *Service) Get(ctx context.Context, id string) (*types.AgentBinding, error) {
	if err := requireBindingAdmin(ctx); err != nil {
		return nil, err
	}
	tenantID, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return s.bindings.GetAgentBinding(ctx, tenantID, id)
}

func (s *Service) Revoke(ctx context.Context, id string) error {
	if err := requireBindingAdmin(ctx); err != nil {
		return err
	}
	tenantID, err := tenantFromContext(ctx)
	if err != nil {
		return err
	}
	return s.bindings.RevokeAgentBindingWithKeys(ctx, tenantID, id)
}

func (s *Service) RotateKey(ctx context.Context, id, createdBy string) (string, error) {
	if err := requireBindingAdmin(ctx); err != nil {
		return "", err
	}
	tenantID, err := tenantFromContext(ctx)
	if err != nil {
		return "", err
	}
	binding, err := s.bindings.GetAgentBinding(ctx, tenantID, id)
	if err != nil {
		return "", err
	}
	if binding.Status != types.AgentBindingStatusActive && binding.Status != types.AgentBindingStatusPendingSetup || bindingExpired(binding, time.Now().UTC()) {
		return "", ErrInvalidBinding
	}
	secret, err := newSecret()
	if err != nil {
		return "", err
	}
	hash, err := hashConnectorSecret(secret)
	if err != nil {
		return "", err
	}
	keyPurpose := "memory_binding_runtime"
	keyExpiresAt := binding.ExpiresAt
	if binding.Status == types.AgentBindingStatusPendingSetup {
		setupExpires := time.Now().UTC().Add(30 * time.Minute)
		binding.SetupExpiresAt = &setupExpires
		keyPurpose = "memory_binding_setup"
		keyExpiresAt = &setupExpires
	}
	key := &types.AgentBindingKey{
		ID: uuid.NewString(), BindingID: id, TenantID: tenantID,
		KeyPrefix: connectorSecretPrefix(secret), KeyHash: hash,
		CreatedBy: createdBy, ExpiresAt: keyExpiresAt, Purpose: keyPurpose,
	}
	if err := s.bindings.RotateAgentBindingKey(ctx, tenantID, id, key); err != nil {
		return "", err
	}
	return secret, nil
}

func (s *Service) RotateSetupKey(ctx context.Context, id, createdBy string) (*interfaces.AgentBindingCreateResult, error) {
	binding, err := s.Get(ctx, id)
	if err != nil || binding.Status != types.AgentBindingStatusPendingSetup {
		return nil, ErrInvalidBinding
	}
	if _, _, _, err := setupEndpoints(binding.ConnectorType); err != nil {
		return nil, ErrSetupUnavailable
	}
	secret, err := s.RotateKey(ctx, id, createdBy)
	if err != nil {
		return nil, err
	}
	// RotateKey updates setup_expires_at inside the repository transaction.
	// Re-read the binding before building the response so the prompt and
	// setup_expires_at reflect the committed value rather than the pre-rotation
	// snapshot loaded above.
	binding, err = s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	manifest, prompt, err := buildSetupArtifacts(binding, secret)
	if err != nil {
		return nil, err
	}
	return &interfaces.AgentBindingCreateResult{Binding: binding, ConnectorSecret: secret, CredentialPurpose: "memory_binding_setup", SetupExpiresAt: binding.SetupExpiresAt, SetupManifest: manifest, SetupPrompt: prompt}, nil
}

func (s *Service) Setup(ctx context.Context, req interfaces.AgentBindingSetupRequest) (*interfaces.AgentBindingSetupResult, error) {
	if !setupFlowEnabled() || strings.TrimSpace(req.ConnectorSecret) == "" || strings.TrimSpace(req.BindingID) == "" {
		return nil, ErrSetupUnavailable
	}
	if _, _, _, err := setupEndpoints(strings.TrimSpace(req.ConnectorType)); err != nil {
		return nil, ErrSetupUnavailable
	}
	repo, ok := s.bindings.(interfaces.AgentBindingSetupRepository)
	if !ok {
		return nil, ErrSetupUnavailable
	}
	hash, err := hashConnectorSecret(req.ConnectorSecret)
	if err != nil {
		return nil, ErrInvalidBinding
	}
	runtimeSecret, err := newSecret()
	if err != nil {
		return nil, err
	}
	runtimeHash, err := hashConnectorSecret(runtimeSecret)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	runtimeKey := &types.AgentBindingKey{ID: uuid.NewString(), BindingID: req.BindingID, TenantID: 0, KeyPrefix: connectorSecretPrefix(runtimeSecret), KeyHash: runtimeHash, Purpose: "memory_binding_runtime"}
	var dataSecret string
	var dataKey *types.AgentBindingKey
	if connectorNeedsMemoryDataKey(req.ConnectorType) {
		dataSecret, err = newSecret()
		if err != nil {
			return nil, err
		}
		dataHash, hashErr := hashConnectorSecret(dataSecret)
		if hashErr != nil {
			return nil, hashErr
		}
		dataKey = &types.AgentBindingKey{ID: uuid.NewString(), BindingID: req.BindingID, TenantID: 0, KeyPrefix: connectorSecretPrefix(dataSecret), KeyHash: dataHash, Purpose: "memory_binding_data"}
	}
	binding, err := repo.CompleteSetup(ctx, hash, req.BindingID, strings.TrimSpace(req.ExternalAgent), strings.TrimSpace(req.ConnectorType), req.UserAPIKeyID, runtimeKey, dataKey, now)
	if err != nil {
		return nil, ErrInvalidBinding
	}
	return &interfaces.AgentBindingSetupResult{BindingID: binding.ID, Status: binding.Status, ConnectorSecret: runtimeSecret, MemoryAccessKey: dataSecret, FTMindEndpoint: publicEndpoint("FMIND_PUBLIC_BASE_URL"), MemoryCoreEndpoint: publicEndpoint("MEMORY_CORE_PUBLIC_URL"), MemoryProxyEndpoint: publicEndpoint("MEMORY_PROXY_PUBLIC_URL"), PolicyVersion: binding.PolicyVersion, ExpiresAt: binding.ExpiresAt}, nil
}

func connectorNeedsMemoryDataKey(connector string) bool {
	// MemoryCore is kept private. External agents use the FTMind/MemoryProxy
	// gateway, so a second data-plane secret has no verifier and must not be
	// issued. Keep the function for wire compatibility with older callers.
	return false
}

func (s *Service) SetupStatus(ctx context.Context, id string) (*interfaces.AgentBindingSetupStatus, error) {
	if err := requireBindingAdmin(ctx); err != nil {
		return nil, err
	}
	tenantID, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	binding, err := s.bindings.GetAgentBinding(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	return &interfaces.AgentBindingSetupStatus{BindingID: binding.ID, Status: binding.Status, SetupExpiresAt: binding.SetupExpiresAt, ActivatedAt: binding.ActivatedAt, LastHandshakeAt: binding.LastHandshakeAt, SetupAttempts: binding.SetupAttempts}, nil
}

func setupFlowEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("FMIND_AGENT_BINDING_SETUP_FLOW")), "true")
}

func publicEndpoint(name string) string {
	value, _ := validatedEndpoint(name, false)
	return value
}

// validatedEndpoint is deliberately strict because these values are copied
// into Agent configuration and prompt text. Never accept credentials, query
// strings, fragments, or an arbitrary HTTP endpoint in a production prompt.
func validatedEndpoint(name string, required bool) (string, error) {
	raw := strings.TrimRight(strings.TrimSpace(os.Getenv(name)), "/")
	if raw == "" {
		if required {
			return "", fmt.Errorf("%w: %s is required", ErrSetupUnavailable, name)
		}
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("%w: invalid %s", ErrSetupUnavailable, name)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("%w: unsupported scheme for %s", ErrSetupUnavailable, name)
	}
	if u.Scheme == "http" {
		host := strings.ToLower(u.Hostname())
		loopback := host == "localhost" || host == "127.0.0.1" || host == "::1"
		if strings.EqualFold(strings.TrimSpace(os.Getenv("NODE_ENV")), "production") || !loopback {
			return "", fmt.Errorf("%w: insecure endpoint %s is not allowed", ErrSetupUnavailable, name)
		}
	}
	return raw, nil
}

func setupEndpoints(connector string) (fmind, core, proxy string, err error) {
	if fmind, err = validatedEndpoint("FMIND_PUBLIC_BASE_URL", true); err != nil {
		return "", "", "", err
	}
	// MemoryCore is an internal service. External agents must use the FTMind /
	// MemoryProxy gateway and must never be given a public Core endpoint or a
	// direct data-plane credential. Keep the optional value for backwards
	// compatible manifests, but do not make setup depend on it.
	needsCore := false
	needsProxy := connector == "openai_proxy" || connector == "anthropic_proxy"
	if core, err = validatedEndpoint("MEMORY_CORE_PUBLIC_URL", needsCore); err != nil {
		return "", "", "", err
	}
	if proxy, err = validatedEndpoint("MEMORY_PROXY_PUBLIC_URL", needsProxy); err != nil {
		return "", "", "", err
	}
	return fmind, core, proxy, nil
}

func buildSetupArtifacts(binding *types.AgentBinding, secret string) (*interfaces.AgentBindingSetupManifest, string, error) {
	return buildSetupArtifactsClean(binding, secret)
	/*
		fmind, core, proxy, err := setupEndpoints(binding.ConnectorType)
		if err != nil {
			return nil, "", err
		}
		manifest := &interfaces.AgentBindingSetupManifest{BindingID: binding.ID, ExternalAgent: binding.ExternalAgent, ConnectorType: binding.ConnectorType, FTMindEndpoint: fmind, MemoryCoreEndpoint: core, MemoryProxyEndpoint: proxy, Capabilities: append([]string(nil), binding.CapabilityScopes...), AssetScopes: append([]string(nil), binding.AssetScopes...)}
		template := setupTemplate(binding.ConnectorType, fmind, manifest.MemoryCoreEndpoint, manifest.MemoryProxyEndpoint)
		prompt := fmt.Sprintf("FTMind 外部 Agent 一键接入\n\n目标 Agent: %s\nConnector: %s\nBinding ID: %s\nFTMind 地址: %s\nMemoryCore 地址: %s\nMemoryProxy 地址: %s\n一次性接入密钥（仅可使用一次，30分钟后失效）: %s\n\n已启用能力:\n- %s\n\n资产范围:\n- %s\n\n%s\n\n请将密钥保存到本地安全环境变量 FMIND_CONNECTOR_SECRET，不要输出到日志、URL、模型上下文或提交到代码仓库。配置完成后调用：\nPOST %s/internal/v1/agent-bindings/setup\n请求体: {\"binding_id\":\"%s\",\"external_agent\":\"%s\",\"connector_type\":\"%s\",\"client_version\":\"<your-version>\"}\n请求头: X-FMind-Connector-Secret: $FMIND_CONNECTOR_SECRET\n\nsetup 成功后删除一次性密钥，将响应中的 connector_secret 保存为运行期凭证；如果响应包含 memory_access_key，则仅保存到 FMIND_MEMORY_ACCESS_KEY。后续每次请求都必须重新校验 Binding Token，并遵守角色、能力和资产范围。失败时请检查公开 HTTPS 地址、反向代理路由、密钥是否过期以及 Agent/Connector 标识是否完全匹配。", binding.ExternalAgent, binding.ConnectorType, binding.ID, fmind, manifest.MemoryCoreEndpoint, manifest.MemoryProxyEndpoint, secret, strings.Join(binding.CapabilityScopes, "\n- "), strings.Join(binding.AssetScopes, "\n- "), template, fmind, binding.ID, binding.ExternalAgent, binding.ConnectorType)
		return manifest, prompt, nil
	*/
}

func buildSetupArtifactsUTF8(binding *types.AgentBinding, secret string) (*interfaces.AgentBindingSetupManifest, string, error) {
	fmind, core, proxy, err := setupEndpoints(binding.ConnectorType)
	if err != nil {
		return nil, "", err
	}
	manifest := &interfaces.AgentBindingSetupManifest{
		BindingID: binding.ID, ExternalAgent: binding.ExternalAgent, ConnectorType: binding.ConnectorType,
		FTMindEndpoint: fmind, MemoryCoreEndpoint: core, MemoryProxyEndpoint: proxy,
		Capabilities: append([]string(nil), binding.CapabilityScopes...), AssetScopes: append([]string(nil), binding.AssetScopes...),
	}
	template := setupTemplateUTF8(binding.ConnectorType, fmind, proxy)
	prompt := fmt.Sprintf("FTMind 外部 Agent 一键接入\n\n目标 Agent: %s\nConnector: %s\nBinding ID: %s\nFTMind 地址: %s\nMemoryProxy 地址: %s\n\n用户 API Key（管理员选择的现有 Key，请在外部 Agent 本地环境变量中填写）：\nFMIND_USER_API_KEY=${FMIND_USER_API_KEY}\n\n一次性 Agent 接入密钥（仅可使用一次，30 分钟后失效）：\nFMIND_AGENT_SETUP_KEY=%s\n\n已启用能力：\n- %s\n\n资产范围：\n- %s\n\n%s\n\n请勿输出或记录密钥，不要放入 URL、模型上下文或代码仓库。配置完成后调用 setup，成功后删除 FMIND_AGENT_SETUP_KEY，并只保存响应中的运行期 Agent Key。\nsetup：POST %s/internal/v1/agent-bindings/setup\n请求头：X-FMind-Connector-Secret: $FMIND_AGENT_SETUP_KEY\n请求体：{\"binding_id\":\"%s\",\"external_agent\":\"%s\",\"connector_type\":\"%s\",\"client_version\":\"<your-version>\"}\n后续数据请求必须同时携带 X-FMind-User-Key 与 X-FMind-Agent-Key，由 FTMind 网关校验用户角色、Binding 能力和资产范围。", binding.ExternalAgent, binding.ConnectorType, binding.ID, fmind, proxy, secret, strings.Join(binding.CapabilityScopes, "\n- "), strings.Join(binding.AssetScopes, "\n- "), template, fmind, binding.ID, binding.ExternalAgent, binding.ConnectorType)
	return manifest, prompt, nil
}

func setupTemplateUTF8(connector, fmind, proxy string) string {
	switch connector {
	case "openclaw_plugin":
		return fmt.Sprintf("OpenClaw：FMIND_ENDPOINT=%s；FMIND_USER_API_KEY=${FMIND_USER_API_KEY}；FMIND_AGENT_SETUP_KEY=${FMIND_AGENT_SETUP_KEY}", fmind)
	case "openai_proxy", "anthropic_proxy":
		return fmt.Sprintf("代理：BASE_URL=%s；FMIND_USER_API_KEY=${FMIND_USER_API_KEY}；setup 成功后使用 FMIND_AGENT_RUNTIME_KEY", proxy)
	case "mcp":
		return fmt.Sprintf("MCP：{\"url\":\"%s/mcp/cognition\",\"headers\":{\"X-FMind-User-Key\":\"${FMIND_USER_API_KEY}\",\"X-FMind-Agent-Key\":\"${FMIND_AGENT_RUNTIME_KEY}\"}}", fmind)
	default:
		return fmt.Sprintf("SDK：FMIND_ENDPOINT=%s；FMIND_USER_API_KEY=${FMIND_USER_API_KEY}；FMIND_AGENT_SETUP_KEY=${FMIND_AGENT_SETUP_KEY}", fmind)
	}
}

func setupTemplate(connector, fmind, core, proxy string) string {
	switch connector {
	case "openclaw_plugin":
		return fmt.Sprintf("OpenClaw 配置片段:\n{\"fmind\":{\"endpoint\":\"%s\",\"connectorSecret\":\"env:FMIND_CONNECTOR_SECRET\"},\"memoryCore\":{\"url\":\"%s\",\"apiKey\":\"env:FMIND_MEMORY_ACCESS_KEY\"}}\n并执行 openclaw gateway restart。", fmind, core)
	case "openai_proxy", "anthropic_proxy":
		return fmt.Sprintf("代理环境变量:\nOPENAI/ANTHROPIC_BASE_URL=%s\nOPENAI/ANTHROPIC_API_KEY=$FMIND_CONNECTOR_SECRET\nConnector Secret 只能由 FTMind Proxy 消费，禁止转发给上游模型服务。", proxy)
	case "hermes_provider":
		return fmt.Sprintf("Hermes Provider 配置:\nFMIND_ENDPOINT=%s\nFMIND_CONNECTOR_SECRET=$FMIND_CONNECTOR_SECRET\nFMIND_MEMORY_CORE_ENDPOINT=%s\nFMIND_MEMORY_ACCESS_KEY=$FMIND_MEMORY_ACCESS_KEY", fmind, core)
	case "generic_sdk":
		return fmt.Sprintf("Python/Node SDK 环境变量:\nFMIND_ENDPOINT=%s\nFMIND_CONNECTOR_SECRET=$FMIND_CONNECTOR_SECRET\nFMIND_MEMORY_CORE_ENDPOINT=%s\nFMIND_MEMORY_ACCESS_KEY=$FMIND_MEMORY_ACCESS_KEY\n请通过 SDK 的 recall/capture 方法调用，勿硬编码团队或用户身份。", fmind, core)
	case "mcp":
		return fmt.Sprintf("MCP Cognition 配置:\n{\"mcpServers\":{\"fmind-cognition\":{\"url\":\"%s/mcp/cognition\",\"headers\":{\"X-FMind-Connector-Secret\":\"$FMIND_CONNECTOR_SECRET\"}}}}", fmind)
	default:
		return fmt.Sprintf("通用 SDK 配置:\nFMIND_ENDPOINT=%s\nFMIND_CONNECTOR_SECRET=$FMIND_CONNECTOR_SECRET\n请在本地安全环境变量中保存凭证，并使用 SDK 完成 setup、recall/capture。", fmind)
	}
}

func (s *Service) Introspect(ctx context.Context, secret string) (*types.BindingIntrospectionResult, error) {
	hash, err := hashConnectorSecret(secret)
	if err != nil {
		return nil, ErrInvalidBinding
	}
	now := time.Now().UTC()
	key, binding, err := s.bindings.ResolveActiveKeyAndBinding(ctx, hash, now)
	if err != nil || binding.Status != types.AgentBindingStatusActive || bindingExpired(binding, now) || !bindingIdentityComplete(binding) || s.validator == nil {
		return nil, ErrInvalidBinding
	}
	roles, err := s.validator.ResolveRoles(ctx, binding)
	if err != nil {
		return nil, ErrInvalidBinding
	}
	expiresAt := now.Add(bindingTokenTTL)
	if binding.ExpiresAt != nil && binding.ExpiresAt.Before(expiresAt) {
		expiresAt = binding.ExpiresAt.UTC()
	}
	contextValue := bindingContext(binding, roles, uuid.NewString(), expiresAt)
	token, err := signBindingToken(contextValue, now)
	if err != nil {
		return nil, err
	}
	// Authentication has already succeeded. Telemetry write availability must
	// not turn a valid connector into an unauthorized one.
	_ = s.keys.TouchAgentBindingKey(ctx, key.ID, now)
	_ = s.bindings.TouchAgentBinding(ctx, binding.TenantID, binding.ID, now)
	return &types.BindingIntrospectionResult{BindingToken: token, Context: contextValue}, nil
}

type bindingTokenClaims struct {
	BindingID        string            `json:"binding_id"`
	TenantID         uint64            `json:"tenant_id"`
	DepartmentID     string            `json:"department_id,omitempty"`
	TeamID           string            `json:"team_id"`
	WorkspaceID      string            `json:"workspace_id,omitempty"`
	ProjectID        string            `json:"project_id,omitempty"`
	UserID           string            `json:"user_id"`
	UserAPIKeyID     uint64            `json:"user_api_key_id,omitempty"`
	AgentID          string            `json:"agent_id"`
	TaskID           string            `json:"task_id,omitempty"`
	ExternalAgent    string            `json:"external_agent"`
	ConnectorType    string            `json:"connector_type"`
	Roles            types.StringArray `json:"roles"`
	CapabilityScopes types.StringArray `json:"capability_scopes"`
	AssetScopes      types.StringArray `json:"asset_scopes"`
	CaptureEnabled   bool              `json:"capture_enabled"`
	RecallEnabled    bool              `json:"recall_enabled"`
	L3WikiEnabled    bool              `json:"l3_wiki_enabled"`
	L3ReviewRequired bool              `json:"l3_review_required"`
	PolicyVersion    uint64            `json:"policy_version"`
	jwt.RegisteredClaims
}

func signBindingToken(value types.BindingContext, issuedAt time.Time) (string, error) {
	secret, err := bindingTokenSecret()
	if err != nil {
		return "", err
	}
	claims := bindingTokenClaims{
		BindingID: value.BindingID, TenantID: value.TenantID, DepartmentID: value.DepartmentID,
		TeamID: value.TeamID, WorkspaceID: value.WorkspaceID, ProjectID: value.ProjectID,
		UserID: value.UserID, UserAPIKeyID: value.UserAPIKeyID, AgentID: value.AgentID, TaskID: value.TaskID,
		ExternalAgent: value.ExternalAgent, ConnectorType: value.ConnectorType,
		Roles:            value.Roles,
		CapabilityScopes: value.CapabilityScopes, AssetScopes: value.AssetScopes,
		CaptureEnabled: value.CaptureEnabled, RecallEnabled: value.RecallEnabled,
		L3WikiEnabled: value.L3WikiEnabled, L3ReviewRequired: value.L3ReviewRequired,
		PolicyVersion: value.PolicyVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: bindingTokenIssuer, Subject: value.BindingID,
			Audience: jwt.ClaimStrings{bindingTokenAudience}, ID: value.TokenID,
			IssuedAt: jwt.NewNumericDate(issuedAt), NotBefore: jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(value.ExpiresAt),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func (s *Service) VerifyBindingToken(ctx context.Context, tokenString string) (*types.BindingContext, error) {
	secret, err := bindingTokenSecret()
	if err != nil {
		return nil, err
	}
	claims := &bindingTokenClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected binding token algorithm")
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(bindingTokenIssuer),
		jwt.WithAudience(bindingTokenAudience), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithLeeway(bindingTokenClockSkew))
	if err != nil || !token.Valid || !validBindingTokenTimes(claims, time.Now().UTC()) || claims.ID == "" || claims.Subject != claims.BindingID || claims.BindingID == "" || claims.TenantID == 0 ||
		claims.TeamID == "" || claims.UserID == "" || claims.AgentID == "" || claims.ExternalAgent == "" || claims.ConnectorType == "" {
		return nil, ErrInvalidBinding
	}
	verified := &types.BindingContext{
		TokenID: claims.ID, BindingID: claims.BindingID, TenantID: claims.TenantID,
		DepartmentID: claims.DepartmentID, TeamID: claims.TeamID, WorkspaceID: claims.WorkspaceID,
		ProjectID: claims.ProjectID, UserID: claims.UserID, UserAPIKeyID: claims.UserAPIKeyID, AgentID: claims.AgentID,
		TaskID: claims.TaskID, ExternalAgent: claims.ExternalAgent, ConnectorType: claims.ConnectorType,
		Roles:            claims.Roles,
		CapabilityScopes: claims.CapabilityScopes, AssetScopes: claims.AssetScopes,
		CaptureEnabled: claims.CaptureEnabled, RecallEnabled: claims.RecallEnabled,
		L3WikiEnabled: claims.L3WikiEnabled, L3ReviewRequired: claims.L3ReviewRequired,
		PolicyVersion: claims.PolicyVersion, ExpiresAt: claims.ExpiresAt.Time,
	}
	binding, err := s.bindings.GetAgentBinding(ctx, claims.TenantID, claims.BindingID)
	if err != nil || binding.Status != types.AgentBindingStatusActive || bindingExpired(binding, time.Now().UTC()) || !bindingIdentityComplete(binding) {
		return nil, ErrInvalidBinding
	}
	if s.validator == nil {
		return nil, ErrInvalidBinding
	}
	roles, err := s.validator.ResolveRoles(ctx, binding)
	if err != nil {
		return nil, ErrInvalidBinding
	}
	expected := bindingContext(binding, roles, claims.ID, claims.ExpiresAt.Time)
	if !bindingContextsEqual(*verified, expected) {
		return nil, ErrInvalidBinding
	}
	return verified, nil
}

func bindingContext(binding *types.AgentBinding, roles types.StringArray, tokenID string, expiresAt time.Time) types.BindingContext {
	return types.BindingContext{
		TokenID: tokenID, BindingID: binding.ID, TenantID: binding.TenantID,
		DepartmentID: binding.DepartmentID, TeamID: binding.TeamID, WorkspaceID: binding.WorkspaceID,
		ProjectID: binding.ProjectID, UserID: binding.UserID, UserAPIKeyID: binding.UserAPIKeyID, AgentID: binding.AgentID,
		TaskID: binding.TaskID, ExternalAgent: binding.ExternalAgent, ConnectorType: binding.ConnectorType,
		Roles:            append(types.StringArray(nil), roles...),
		CapabilityScopes: append(types.StringArray(nil), binding.CapabilityScopes...),
		AssetScopes:      append(types.StringArray(nil), binding.AssetScopes...),
		CaptureEnabled:   binding.CaptureEnabled, RecallEnabled: binding.RecallEnabled,
		L3WikiEnabled: binding.L3WikiEnabled, L3ReviewRequired: binding.L3ReviewRequired,
		PolicyVersion: binding.PolicyVersion, ExpiresAt: expiresAt,
	}
}

func bindingContextsEqual(left, right types.BindingContext) bool {
	return left.TokenID == right.TokenID && left.BindingID == right.BindingID && left.TenantID == right.TenantID &&
		left.DepartmentID == right.DepartmentID && left.TeamID == right.TeamID && left.WorkspaceID == right.WorkspaceID &&
		left.ProjectID == right.ProjectID && left.UserID == right.UserID && left.UserAPIKeyID == right.UserAPIKeyID && left.AgentID == right.AgentID &&
		left.TaskID == right.TaskID && left.ExternalAgent == right.ExternalAgent && left.ConnectorType == right.ConnectorType && stringArraysEqual(left.Roles, right.Roles) &&
		stringArraysEqual(left.CapabilityScopes, right.CapabilityScopes) && stringArraysEqual(left.AssetScopes, right.AssetScopes) &&
		left.CaptureEnabled == right.CaptureEnabled && left.RecallEnabled == right.RecallEnabled &&
		left.L3WikiEnabled == right.L3WikiEnabled && left.L3ReviewRequired == right.L3ReviewRequired &&
		left.PolicyVersion == right.PolicyVersion && left.ExpiresAt.Equal(right.ExpiresAt)
}

func stringArraysEqual(left, right types.StringArray) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func validateCreateRequest(req *interfaces.AgentBindingCreateRequest) error {
	req.DepartmentID = strings.TrimSpace(req.DepartmentID)
	req.TeamID = strings.TrimSpace(req.TeamID)
	req.WorkspaceID = strings.TrimSpace(req.WorkspaceID)
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.UserID = strings.TrimSpace(req.UserID)
	req.AgentID = strings.TrimSpace(req.AgentID)
	req.TaskID = strings.TrimSpace(req.TaskID)
	req.ExternalAgent = strings.TrimSpace(req.ExternalAgent)
	req.ConnectorType = strings.TrimSpace(req.ConnectorType)
	for name, field := range map[string]struct {
		value string
		max   int
	}{
		"team_id": {req.TeamID, 36}, "user_id": {req.UserID, 36},
		"agent_id": {req.AgentID, 36}, "external_agent": {req.ExternalAgent, maxExternalAgentLength},
	} {
		if field.value == "" || len(field.value) > field.max {
			return fmt.Errorf("%s is required and must be at most %d bytes", name, field.max)
		}
	}
	if !externalAgentPattern.MatchString(req.ExternalAgent) {
		return errors.New("external_agent must be a lowercase route identifier")
	}
	for name, field := range map[string]struct {
		value string
		max   int
	}{
		"department_id": {req.DepartmentID, 36}, "workspace_id": {req.WorkspaceID, 36},
		"project_id": {req.ProjectID, 36}, "task_id": {req.TaskID, 64},
	} {
		if len(field.value) > field.max {
			return fmt.Errorf("%s must be at most %d bytes", name, field.max)
		}
	}
	if _, ok := supportedConnectors[req.ConnectorType]; !ok {
		return ErrUnsupportedConnector
	}
	capabilities, err := normalizeCapabilities(req.CapabilityScopes)
	if err != nil {
		return err
	}
	req.CapabilityScopes = capabilities
	if containsCapability(capabilities, "memory.l3.publish") && !req.L3WikiEnabled {
		return errors.New("memory.l3.publish capability requires the reviewed L3 Wiki policy")
	}
	for capability, enabled := range map[string]bool{
		"memory.capture":    req.CaptureEnabled,
		"memory.recall":     req.RecallEnabled,
		"memory.l3.publish": req.L3WikiEnabled,
	} {
		if enabled && !containsCapability(capabilities, capability) {
			return fmt.Errorf("%s capability is required by the enabled binding policy", capability)
		}
	}
	assets, err := normalizeAssetScopes(req.AssetScopes)
	if err != nil {
		return err
	}
	req.AssetScopes = assets
	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now()) {
		return errors.New("expires_at must be in the future")
	}
	if req.L3WikiEnabled {
		req.L3ReviewRequired = true
	}
	return nil
}

func normalizeCapabilities(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if _, ok := supportedCapabilities[value]; !ok {
			return nil, fmt.Errorf("unsupported capability scope %q", value)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}

func containsCapability(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func normalizeAssetScopes(values []string) ([]string, error) {
	if len(values) > maxAssetScopes {
		return nil, fmt.Errorf("asset_scopes cannot contain more than %d entries", maxAssetScopes)
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" || len(value) > maxAssetScopeLength {
			return nil, fmt.Errorf("asset scope must be non-empty and at most %d bytes", maxAssetScopeLength)
		}
		if _, _, err := parseAssetScope(value); err != nil {
			return nil, err
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}

func parseAssetScope(value string) (string, string, error) {
	if strings.Count(value, ":") != 1 {
		return "", "", fmt.Errorf("asset scope %q must use kind:id format", value)
	}
	parts := strings.SplitN(value, ":", 2)
	kind, id := parts[0], parts[1]
	if _, ok := supportedAssetKinds[kind]; !ok {
		return "", "", fmt.Errorf("unsupported asset scope kind %q", kind)
	}
	if !assetScopeIDPattern.MatchString(id) {
		return "", "", fmt.Errorf("asset scope %q has an invalid id", value)
	}
	if kind == "tenant" {
		parsed, err := strconv.ParseUint(id, 10, 64)
		if err != nil || parsed == 0 {
			return "", "", fmt.Errorf("asset scope %q has an invalid tenant id", value)
		}
	}
	return kind, id, nil
}

func hashConnectorSecret(secret string) (string, error) {
	pepper, err := secretFromEnvironment("FMIND_BINDING_KEY_PEPPER")
	if err != nil {
		return "", err
	}
	if secret == "" {
		return "", ErrInvalidBinding
	}
	mac := hmac.New(sha256.New, []byte(pepper))
	_, _ = mac.Write([]byte(secret))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func bindingTokenSecret() (string, error) {
	return secretFromEnvironment("FMIND_BINDING_TOKEN_SECRET")
}

func secretFromEnvironment(name string) (string, error) {
	secret := os.Getenv(name)
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" || trimmed != secret || len(secret) < minimumSecretBytes {
		return "", fmt.Errorf("%s must contain at least %d non-whitespace bytes", name, minimumSecretBytes)
	}
	return secret, nil
}

func validBindingTokenTimes(claims *bindingTokenClaims, now time.Time) bool {
	if claims.IssuedAt == nil || claims.NotBefore == nil || claims.ExpiresAt == nil {
		return false
	}
	issuedAt := claims.IssuedAt.Time
	notBefore := claims.NotBefore.Time
	expiresAt := claims.ExpiresAt.Time
	if issuedAt.After(now.Add(bindingTokenClockSkew)) || notBefore.After(now.Add(bindingTokenClockSkew)) {
		return false
	}
	if notBefore.Before(issuedAt.Add(-bindingTokenClockSkew)) || notBefore.After(issuedAt.Add(bindingTokenClockSkew)) {
		return false
	}
	return expiresAt.After(issuedAt) && expiresAt.Sub(issuedAt) <= bindingTokenTTL
}

func newSecret() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate binding secret: %w", err)
	}
	return "fmind_" + hex.EncodeToString(value), nil
}

func connectorSecretPrefix(secret string) string {
	const prefixLength = 18
	if len(secret) <= prefixLength {
		return secret
	}
	return secret[:prefixLength]
}

func bindingExpired(binding *types.AgentBinding, now time.Time) bool {
	return binding.ExpiresAt != nil && !binding.ExpiresAt.After(now)
}

func bindingIdentityComplete(binding *types.AgentBinding) bool {
	return binding.TenantID != 0 && binding.TeamID != "" && binding.UserID != "" && binding.AgentID != "" && binding.ExternalAgent != "" && binding.ConnectorType != ""
}

func tenantFromContext(ctx context.Context) (uint64, error) {
	id, ok := types.TenantIDFromContext(ctx)
	if !ok || id == 0 {
		return 0, errors.New("tenant context is required")
	}
	return id, nil
}

func requireBindingAdmin(ctx context.Context) error {
	if types.IsSystemAdminFromContext(ctx) || types.TenantRoleFromContext(ctx).HasPermission(types.TenantRoleAdmin) {
		return nil
	}
	return ErrBindingAdminRequired
}
