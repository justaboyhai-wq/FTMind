package cognition

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

var (
	ErrUnauthenticated = errors.New("cognition authentication failed")
	ErrForbidden       = errors.New("cognition scope denied")
	ErrInvalidRequest  = errors.New("invalid cognition request")
)

const maxCognitionRequestBytes = 4 << 20

type BindingTokenVerifier interface {
	VerifyBindingToken(context.Context, string) (*types.BindingContext, error)
}

type ToolExecutor interface {
	ExecuteCognitionTool(context.Context, Invocation) (any, error)
}

type Invocation struct {
	Tool         string
	Arguments    map[string]any
	Binding      types.BindingContext
	TraceID      string
	bindingToken string
}

type toolPolicy struct {
	capability string
	assetKind  string
	assetField string
	flag       func(types.BindingContext) bool
}

var toolPolicies = map[string]toolPolicy{
	ToolMemoryGetContext:       {capability: "memory.context"},
	ToolMemorySearch:           {capability: "memory.recall", flag: func(v types.BindingContext) bool { return v.RecallEnabled }},
	ToolMemoryCaptureTurn:      {capability: "memory.capture", flag: func(v types.BindingContext) bool { return v.CaptureEnabled }},
	ToolMemoryConfirmCandidate: {capability: "memory.confirm"},
	ToolKnowledgeSearch:        {capability: "knowledge.search", assetKind: "knowledge_base", assetField: "knowledge_base_ids"},
	ToolWikiGetPage:            {capability: "wiki.get", assetKind: "wiki_page", assetField: "wiki_page_id"},
	ToolDocumentRead:           {capability: "document.read", assetKind: "document", assetField: "document_id"},
	ToolContextAssemble:        {capability: "context.assemble", assetField: "asset_scopes"},
}

type Server struct {
	verifier BindingTokenVerifier
	executor ToolExecutor
	now      func() time.Time
}

type bindingTokenContextKey struct{}
type bindingTokenErrorContextKey struct{}

func NewServer(verifier BindingTokenVerifier, executor ToolExecutor) *Server {
	return &Server{verifier: verifier, executor: executor, now: time.Now}
}

// ToolNames returns the frozen Cognition MCP surface in stable order. Keeping
// this list closed prevents a newly added internal helper from becoming an
// externally callable tool by accident.
func (s *Server) ToolNames() []string {
	names := make([]string, 0, len(toolPolicies))
	for name := range toolPolicies {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// HTTPHandler exposes the Cognition tools over MCP Streamable HTTP. Binding
// Tokens are consumed from transport headers and are never accepted inside a
// tool argument or caller-supplied BindingContext.
func (s *Server) HTTPHandler() http.Handler {
	mcpServer := mcpserver.NewMCPServer(
		"FTMind Cognition",
		"1.0.0",
		mcpserver.WithToolCapabilities(false),
	)
	for _, name := range s.ToolNames() {
		toolName := name
		mcpServer.AddTool(cognitionTool(toolName), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if authErr, _ := ctx.Value(bindingTokenErrorContextKey{}).(error); authErr != nil {
				return mcp.NewToolResultError(ErrUnauthenticated.Error()), nil
			}
			token, _ := ctx.Value(bindingTokenContextKey{}).(string)
			arguments := request.GetArguments()
			traceID, _ := arguments["trace_id"].(string)
			response, err := s.Dispatch(ctx, token, Request{Tool: toolName, Arguments: arguments, TraceID: traceID})
			if err != nil {
				// Do not expose repository, verifier, or upstream errors to an MCP
				// caller. The typed category is enough to correct the request.
				switch {
				case errors.Is(err, ErrUnauthenticated):
					return mcp.NewToolResultError(ErrUnauthenticated.Error()), nil
				case errors.Is(err, ErrForbidden):
					return mcp.NewToolResultError(ErrForbidden.Error()), nil
				case errors.Is(err, ErrInvalidRequest):
					return mcp.NewToolResultError(ErrInvalidRequest.Error()), nil
				default:
					return mcp.NewToolResultError("cognition tool execution failed"), nil
				}
			}
			return mcp.NewToolResultStructuredOnly(response), nil
		})
	}
	transport := mcpserver.NewStreamableHTTPServer(
		mcpServer,
		mcpserver.WithHTTPContextFunc(s.httpContext),
		mcpserver.WithDisableStreaming(true),
	)
	return requireBindingCredential(transport)
}

func (s *Server) httpContext(ctx context.Context, request *http.Request) context.Context {
	if token, ok := ctx.Value(bindingTokenContextKey{}).(string); ok && token != "" {
		return ctx
	}
	token, err := bindingTokenFromRequest(request)
	// Consume credentials before any downstream diagnostics or tracing can
	// capture the raw request headers.
	request.Header.Del("Authorization")
	request.Header.Del("X-FMind-Binding-Token")
	if err != nil {
		return context.WithValue(ctx, bindingTokenErrorContextKey{}, err)
	}
	return context.WithValue(ctx, bindingTokenContextKey{}, token)
}

func requireBindingCredential(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		token, err := bindingTokenFromRequest(request)
		// The transport credential is never an upstream model credential and
		// must not remain available to downstream middleware or diagnostics.
		request.Header.Del("Authorization")
		request.Header.Del("X-FMind-Binding-Token")
		if err != nil {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"error":"cognition authentication failed"}`))
			return
		}
		if request.Body != nil {
			request.Body = http.MaxBytesReader(writer, request.Body, maxCognitionRequestBytes)
		}
		ctx := context.WithValue(request.Context(), bindingTokenContextKey{}, token)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func bindingTokenFromRequest(request *http.Request) (string, error) {
	if request == nil {
		return "", ErrUnauthenticated
	}
	authorization := strings.TrimSpace(request.Header.Get("Authorization"))
	dedicated := strings.TrimSpace(request.Header.Get("X-FMind-Binding-Token"))
	bearer := ""
	if authorization != "" {
		parts := strings.Fields(authorization)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			return "", ErrUnauthenticated
		}
		bearer = parts[1]
	}
	if bearer != "" && dedicated != "" && bearer != dedicated {
		return "", ErrUnauthenticated
	}
	if bearer != "" {
		return bearer, nil
	}
	if dedicated != "" {
		return dedicated, nil
	}
	return "", ErrUnauthenticated
}

func cognitionTool(name string) mcp.Tool {
	options := []mcp.ToolOption{
		mcp.WithDescription("FTMind scoped cognition operation: " + name),
		mcp.WithString("trace_id", mcp.Description("End-to-end audit trace identifier")),
		mcp.WithSchemaAdditionalProperties(true),
	}
	switch name {
	case ToolMemoryGetContext:
		options = append(options, mcp.WithString("external_session_id", mcp.Required()))
	case ToolMemorySearch:
		options = append(options, mcp.WithString("query", mcp.Required()))
	case ToolMemoryCaptureTurn:
		options = append(options,
			mcp.WithString("external_session_id", mcp.Required()),
			mcp.WithArray("messages", mcp.Required()),
		)
	case ToolMemoryConfirmCandidate:
		options = append(options,
			mcp.WithString("candidate_id", mcp.Required()),
			mcp.WithString("decision", mcp.Required()),
		)
	case ToolKnowledgeSearch:
		options = append(options,
			mcp.WithString("query", mcp.Required()),
			mcp.WithArray("knowledge_base_ids", mcp.Required(), mcp.WithStringItems()),
		)
	case ToolWikiGetPage:
		options = append(options, mcp.WithString("wiki_page_id", mcp.Required()))
	case ToolDocumentRead:
		options = append(options, mcp.WithString("document_id", mcp.Required()))
	case ToolContextAssemble:
		options = append(options,
			mcp.WithString("query", mcp.Required()),
			mcp.WithArray("asset_scopes", mcp.Required(), mcp.WithStringItems()),
		)
	}
	return mcp.NewTool(name, options...)
}

func (s *Server) Dispatch(ctx context.Context, bindingToken string, request Request) (Response, error) {
	if s == nil || s.verifier == nil || s.executor == nil {
		return Response{}, fmt.Errorf("%w: cognition server is not configured", ErrUnauthenticated)
	}
	if err := ValidateRequest(request); err != nil {
		return Response{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	if strings.TrimSpace(bindingToken) == "" {
		return Response{}, ErrUnauthenticated
	}
	binding, err := s.verifier.VerifyBindingToken(ctx, bindingToken)
	if err != nil || !validVerifiedBinding(binding, s.now().UTC()) {
		return Response{}, ErrUnauthenticated
	}
	if err := authorizeTool(*binding, request); err != nil {
		return Response{}, err
	}
	executionContext := contextForBinding(ctx, *binding, request.TraceID)
	result, err := s.executor.ExecuteCognitionTool(executionContext, Invocation{
		Tool: request.Tool, Arguments: cloneArguments(request.Arguments),
		Binding: *binding, TraceID: request.TraceID, bindingToken: bindingToken,
	})
	if err != nil {
		return Response{}, err
	}
	return Response{Data: result, TraceID: request.TraceID}, nil
}

func validVerifiedBinding(binding *types.BindingContext, now time.Time) bool {
	return binding != nil && binding.TokenID != "" && binding.BindingID != "" && binding.TenantID != 0 &&
		binding.TeamID != "" && binding.UserID != "" && binding.AgentID != "" && binding.PolicyVersion != 0 &&
		!binding.ExpiresAt.IsZero() && binding.ExpiresAt.After(now)
}

func authorizeTool(binding types.BindingContext, request Request) error {
	policy := toolPolicies[request.Tool]
	role, ok := bindingTenantRole(binding)
	if !ok || !roleAllowsTool(role, request.Tool) {
		return fmt.Errorf("%w: tenant role cannot perform %s", ErrForbidden, request.Tool)
	}
	if !contains(binding.CapabilityScopes, policy.capability) {
		return fmt.Errorf("%w: missing capability %s", ErrForbidden, policy.capability)
	}
	if policy.flag != nil && !policy.flag(binding) {
		return fmt.Errorf("%w: binding policy disables %s", ErrForbidden, request.Tool)
	}
	if err := rejectIdentityOverrides(request.Arguments); err != nil {
		return err
	}
	if policy.assetField == "" {
		return nil
	}
	requested, err := requestedAssetScopes(policy, request.Arguments)
	if err != nil {
		return err
	}
	for _, asset := range requested {
		if !contains(binding.AssetScopes, asset) {
			return fmt.Errorf("%w: asset %s is outside binding scope", ErrForbidden, asset)
		}
	}
	if request.Tool == ToolContextAssemble {
		if err := authorizeContextAssembly(binding, request.Arguments, requested); err != nil {
			return err
		}
	}
	return nil
}

func bindingTenantRole(binding types.BindingContext) (types.TenantRole, bool) {
	for _, raw := range binding.Roles {
		if !strings.HasPrefix(raw, "tenant:") {
			continue
		}
		value := strings.TrimPrefix(raw, "tenant:")
		// tenant:member was emitted by pre-RBAC bindings. Keep it as the
		// least write-capable compatibility role while new tokens use the
		// canonical four TenantRole values.
		if value == "member" {
			return types.TenantRoleContributor, true
		}
		role := types.TenantRole(value)
		if role.IsValid() {
			return role, true
		}
	}
	return "", false
}

func roleAllowsTool(role types.TenantRole, tool string) bool {
	if role == types.TenantRoleOwner || role == types.TenantRoleAdmin {
		return true
	}
	switch tool {
	case ToolMemoryCaptureTurn, ToolMemoryConfirmCandidate:
		return role == types.TenantRoleContributor
	default:
		return role == types.TenantRoleContributor || role == types.TenantRoleViewer
	}
}

func authorizeContextAssembly(binding types.BindingContext, arguments map[string]any, requested []string) error {
	if raw, ok := arguments["include_memory"]; ok {
		includeMemory, valid := raw.(bool)
		if !valid {
			return fmt.Errorf("%w: include_memory must be boolean", ErrInvalidRequest)
		}
		if includeMemory && (!contains(binding.CapabilityScopes, "memory.context") || !binding.RecallEnabled) {
			return fmt.Errorf("%w: memory context capability is required", ErrForbidden)
		}
	}
	requiredByKind := map[string]string{
		"knowledge_base": "knowledge.search",
		"wiki_page":      "wiki.get",
		"document":       "document.read",
	}
	for _, asset := range requested {
		kind := strings.SplitN(asset, ":", 2)[0]
		if capability := requiredByKind[kind]; capability != "" && !contains(binding.CapabilityScopes, capability) {
			return fmt.Errorf("%w: %s requires %s", ErrForbidden, asset, capability)
		}
	}
	return nil
}

func rejectIdentityOverrides(arguments map[string]any) error {
	for _, key := range []string{"tenant_id", "team_id", "department_id", "workspace_id", "project_id", "user_id", "agent_id", "task_id", "binding_id", "policy_version"} {
		if _, ok := arguments[key]; ok {
			return fmt.Errorf("%w: %s is supplied by the binding context", ErrInvalidRequest, key)
		}
	}
	return nil
}

func requestedAssetScopes(policy toolPolicy, arguments map[string]any) ([]string, error) {
	value, ok := arguments[policy.assetField]
	if !ok {
		return nil, fmt.Errorf("%w: %s is required", ErrInvalidRequest, policy.assetField)
	}
	if policy.assetField == "asset_scopes" {
		values, err := stringSlice(value)
		if err != nil || len(values) == 0 {
			return nil, fmt.Errorf("%w: asset_scopes must be a non-empty string array", ErrInvalidRequest)
		}
		for _, item := range values {
			if strings.Count(item, ":") != 1 {
				return nil, fmt.Errorf("%w: invalid asset scope", ErrInvalidRequest)
			}
		}
		return values, nil
	}
	if policy.assetField == "knowledge_base_ids" {
		ids, err := stringSlice(value)
		if err != nil || len(ids) == 0 {
			return nil, fmt.Errorf("%w: knowledge_base_ids must be a non-empty string array", ErrInvalidRequest)
		}
		for _, id := range ids {
			if strings.TrimSpace(id) == "" || strings.Contains(id, ":") {
				return nil, fmt.Errorf("%w: knowledge_base_ids contains an invalid asset id", ErrInvalidRequest)
			}
		}
		return prefixAssets(policy.assetKind, ids), nil
	}
	id, ok := value.(string)
	id = strings.TrimSpace(id)
	if !ok || id == "" || strings.Contains(id, ":") {
		return nil, fmt.Errorf("%w: %s must be a non-empty asset id", ErrInvalidRequest, policy.assetField)
	}
	return []string{policy.assetKind + ":" + id}, nil
}

func prefixAssets(kind string, ids []string) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" && !strings.Contains(id, ":") {
			result = append(result, kind+":"+id)
		}
	}
	return result
}

func stringSlice(value any) ([]string, error) {
	switch values := value.(type) {
	case []string:
		return append([]string(nil), values...), nil
	case []any:
		result := make([]string, 0, len(values))
		for _, value := range values {
			item, ok := value.(string)
			if !ok || strings.TrimSpace(item) == "" {
				return nil, ErrInvalidRequest
			}
			result = append(result, strings.TrimSpace(item))
		}
		return result, nil
	default:
		return nil, ErrInvalidRequest
	}
}

func contains(values types.StringArray, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func cloneArguments(arguments map[string]any) map[string]any {
	if arguments == nil {
		return map[string]any{}
	}
	result := make(map[string]any, len(arguments))
	for key, value := range arguments {
		result[key] = value
	}
	return result
}

func contextForBinding(ctx context.Context, binding types.BindingContext, traceID string) context.Context {
	ctx = types.WithVerifiedBindingContext(ctx, binding)
	ctx = context.WithValue(ctx, types.TenantIDContextKey, binding.TenantID)
	ctx = context.WithValue(ctx, types.SessionTenantIDContextKey, binding.TenantID)
	ctx = context.WithValue(ctx, types.UserIDContextKey, binding.UserID)
	ctx = types.WithPrincipal(ctx, types.Principal{Type: types.PrincipalAPIExternalUser, ID: binding.UserID})
	if traceID != "" {
		ctx = context.WithValue(ctx, types.RequestIDContextKey, traceID)
	}
	for _, role := range binding.Roles {
		if !strings.HasPrefix(role, "tenant:") {
			continue
		}
		parsed := types.TenantRole(strings.TrimPrefix(role, "tenant:"))
		if parsed.IsValid() {
			ctx = context.WithValue(ctx, types.TenantRoleContextKey, parsed)
			break
		}
	}
	// This value is intentionally derived from the verified token and is useful
	// to downstream audit adapters without exposing the token itself.
	ctx = context.WithValue(ctx, bindingAuditContextKey{}, binding.BindingID+":"+strconv.FormatUint(binding.PolicyVersion, 10))
	return ctx
}

type bindingAuditContextKey struct{}
