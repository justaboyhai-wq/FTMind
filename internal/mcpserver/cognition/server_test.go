package cognition

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/justaboyhai-wq/fmind/internal/types"
)

func TestMCPHTTPHandlerRejectsMissingTransportCredential(t *testing.T) {
	server := NewServer(&verifierStub{}, &executorStub{})
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp/cognition", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.HTTPHandler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing credential status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestMCPHTTPHandlerRejectsOversizedRequest(t *testing.T) {
	server := NewServer(&verifierStub{}, &executorStub{})
	req := httptest.NewRequest(http.MethodPost, "/mcp/cognition", strings.NewReader(strings.Repeat("x", maxCognitionRequestBytes+1)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-FMind-Binding-Token", "signed-token")
	w := httptest.NewRecorder()
	server.HTTPHandler().ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("oversized request unexpectedly succeeded: %s", w.Body.String())
	}
}

type verifierStub struct {
	value *types.BindingContext
	err   error
	token string
}

func (v *verifierStub) VerifyBindingToken(_ context.Context, token string) (*types.BindingContext, error) {
	v.token = token
	return v.value, v.err
}

type executorStub struct {
	invocation Invocation
	result     any
	err        error
}

func (e *executorStub) ExecuteCognitionTool(_ context.Context, invocation Invocation) (any, error) {
	e.invocation = invocation
	return e.result, e.err
}

func scopedBinding(capabilities, assets []string) *types.BindingContext {
	return &types.BindingContext{
		TokenID: "jti-1", BindingID: "binding-1", TenantID: 7,
		TeamID: "team-1", UserID: "user-1", AgentID: "agent-1",
		TaskID: "task-1", ConnectorType: "openai_proxy",
		CapabilityScopes: types.StringArray(capabilities), AssetScopes: types.StringArray(assets),
		CaptureEnabled: true, RecallEnabled: true, PolicyVersion: 3,
		ExpiresAt: time.Now().Add(time.Minute),
	}
}

func TestDispatchUsesVerifiedBindingAndPropagatesIdentity(t *testing.T) {
	verifier := &verifierStub{value: scopedBinding([]string{"memory.recall"}, []string{"team:team-1"})}
	executor := &executorStub{result: map[string]any{"items": []any{}}}
	server := NewServer(verifier, executor)

	response, err := server.Dispatch(context.Background(), "signed-token", Request{
		Tool: ToolMemorySearch, TraceID: "trace-1", Arguments: map[string]any{"query": "roadmap"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if verifier.token != "signed-token" {
		t.Fatalf("verifier got %q", verifier.token)
	}
	if response.TraceID != "trace-1" || !reflect.DeepEqual(response.Data, executor.result) {
		t.Fatalf("unexpected response: %#v", response)
	}
	got := executor.invocation.Binding
	if got.TenantID != 7 || got.TeamID != "team-1" || got.UserID != "user-1" || got.AgentID != "agent-1" || got.TaskID != "task-1" {
		t.Fatalf("verified identity not propagated: %#v", got)
	}
	if executor.invocation.bindingToken != "signed-token" {
		t.Fatal("verified token was not confined to the explicit data-plane invocation")
	}
}

func TestDispatchRejectsMissingInvalidAndExpiredTokens(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		verifier *verifierStub
	}{
		{name: "missing", verifier: &verifierStub{value: scopedBinding([]string{"memory.recall"}, nil)}},
		{name: "invalid", token: "bad", verifier: &verifierStub{err: errors.New("revoked")}},
		{name: "expired", token: "old", verifier: &verifierStub{value: func() *types.BindingContext {
			value := scopedBinding([]string{"memory.recall"}, nil)
			value.ExpiresAt = time.Now().Add(-time.Second)
			return value
		}()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(tt.verifier, &executorStub{})
			if _, err := server.Dispatch(context.Background(), tt.token, Request{Tool: ToolMemorySearch}); err == nil {
				t.Fatal("expected authentication failure")
			}
		})
	}
}

func TestDispatchEnforcesCapabilitiesAndPolicyFlags(t *testing.T) {
	tests := []struct {
		name       string
		tool       string
		capability string
		mutate     func(*types.BindingContext)
	}{
		{name: "context", tool: ToolMemoryGetContext, capability: "memory.context"},
		{name: "search", tool: ToolMemorySearch, capability: "memory.recall", mutate: func(v *types.BindingContext) { v.RecallEnabled = false }},
		{name: "capture", tool: ToolMemoryCaptureTurn, capability: "memory.capture", mutate: func(v *types.BindingContext) { v.CaptureEnabled = false }},
		{name: "confirm", tool: ToolMemoryConfirmCandidate, capability: "memory.confirm"},
		{name: "knowledge", tool: ToolKnowledgeSearch, capability: "knowledge.search"},
		{name: "wiki", tool: ToolWikiGetPage, capability: "wiki.get"},
		{name: "document", tool: ToolDocumentRead, capability: "document.read"},
		{name: "assemble", tool: ToolContextAssemble, capability: "context.assemble"},
	}
	for _, tt := range tests {
		t.Run(tt.name+" missing capability", func(t *testing.T) {
			binding := scopedBinding(nil, []string{"knowledge_base:kb-1", "wiki_page:page-1", "document:doc-1"})
			server := NewServer(&verifierStub{value: binding}, &executorStub{})
			if _, err := server.Dispatch(context.Background(), "token", Request{Tool: tt.tool, Arguments: authorizedArguments(tt.tool)}); err == nil {
				t.Fatalf("expected %s denial", tt.capability)
			}
		})
		if tt.mutate != nil {
			t.Run(tt.name+" disabled by policy", func(t *testing.T) {
				binding := scopedBinding([]string{tt.capability}, []string{"knowledge_base:kb-1", "wiki_page:page-1", "document:doc-1"})
				tt.mutate(binding)
				server := NewServer(&verifierStub{value: binding}, &executorStub{})
				if _, err := server.Dispatch(context.Background(), "token", Request{Tool: tt.tool, Arguments: authorizedArguments(tt.tool)}); err == nil {
					t.Fatal("expected policy denial")
				}
			})
		}
	}
}

func TestDispatchEnforcesExactAssetScopes(t *testing.T) {
	tests := []struct {
		tool       string
		capability string
		allowed    string
		argument   map[string]any
	}{
		{ToolKnowledgeSearch, "knowledge.search", "knowledge_base:kb-1", map[string]any{"knowledge_base_ids": []any{"kb-2"}, "query": "x"}},
		{ToolWikiGetPage, "wiki.get", "wiki_page:page-1", map[string]any{"wiki_page_id": "page-2"}},
		{ToolDocumentRead, "document.read", "document:doc-1", map[string]any{"document_id": "doc-2"}},
		{ToolContextAssemble, "context.assemble", "knowledge_base:kb-1", map[string]any{"asset_scopes": []any{"knowledge_base:kb-2"}}},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			binding := scopedBinding([]string{tt.capability}, []string{tt.allowed})
			server := NewServer(&verifierStub{value: binding}, &executorStub{})
			if _, err := server.Dispatch(context.Background(), "token", Request{Tool: tt.tool, Arguments: tt.argument}); err == nil {
				t.Fatal("expected exact asset scope denial")
			}
		})
	}
}

func TestDispatchRejectsMalformedOrEmptyAssetRequests(t *testing.T) {
	tests := []Request{
		{Tool: ToolKnowledgeSearch, Arguments: map[string]any{"knowledge_base_ids": []any{}}},
		{Tool: ToolKnowledgeSearch, Arguments: map[string]any{"knowledge_base_ids": []any{"kb:escape"}}},
		{Tool: ToolWikiGetPage, Arguments: map[string]any{"wiki_page_id": ""}},
		{Tool: ToolDocumentRead, Arguments: map[string]any{"document_id": 17}},
		{Tool: ToolContextAssemble, Arguments: map[string]any{"asset_scopes": []any{}}},
	}
	for _, request := range tests {
		policy := toolPolicies[request.Tool]
		binding := scopedBinding([]string{policy.capability}, []string{"knowledge_base:kb-1", "wiki_page:page-1", "document:doc-1"})
		server := NewServer(&verifierStub{value: binding}, &executorStub{})
		if _, err := server.Dispatch(context.Background(), "token", request); err == nil {
			t.Fatalf("expected malformed asset denial for %#v", request)
		}
	}
}

func TestDispatchRejectsIdentityOverrides(t *testing.T) {
	binding := scopedBinding([]string{"memory.recall"}, []string{"team:team-1"})
	server := NewServer(&verifierStub{value: binding}, &executorStub{})
	for _, field := range []string{"tenant_id", "team_id", "user_id", "agent_id", "task_id", "binding_id", "policy_version"} {
		t.Run(field, func(t *testing.T) {
			if _, err := server.Dispatch(context.Background(), "token", Request{
				Tool: ToolMemorySearch, Arguments: map[string]any{field: "attacker"},
			}); err == nil {
				t.Fatal("expected caller identity override denial")
			}
		})
	}
}

func TestContextAssembleCannotBypassUnderlyingCapabilities(t *testing.T) {
	tests := []struct {
		name         string
		assets       []string
		arguments    map[string]any
		capabilities []string
	}{
		{name: "memory", arguments: map[string]any{"asset_scopes": []any{"team:team-1"}, "include_memory": true}, assets: []string{"team:team-1"}, capabilities: []string{"context.assemble"}},
		{name: "rag", arguments: map[string]any{"asset_scopes": []any{"knowledge_base:kb-1"}}, assets: []string{"knowledge_base:kb-1"}, capabilities: []string{"context.assemble"}},
		{name: "wiki", arguments: map[string]any{"asset_scopes": []any{"wiki_page:page-1"}}, assets: []string{"wiki_page:page-1"}, capabilities: []string{"context.assemble"}},
		{name: "document", arguments: map[string]any{"asset_scopes": []any{"document:doc-1"}}, assets: []string{"document:doc-1"}, capabilities: []string{"context.assemble"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binding := scopedBinding(tt.capabilities, tt.assets)
			server := NewServer(&verifierStub{value: binding}, &executorStub{})
			if _, err := server.Dispatch(context.Background(), "token", Request{Tool: ToolContextAssemble, Arguments: tt.arguments}); err == nil {
				t.Fatal("context_assemble bypassed an underlying capability")
			}
		})
	}
}

func TestBindingTokenFromRequestUsesBearerOrDedicatedHeader(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		dedicated     string
		want          string
		wantErr       bool
	}{
		{name: "bearer", authorization: "Bearer signed-a", want: "signed-a"},
		{name: "dedicated", dedicated: "signed-b", want: "signed-b"},
		{name: "matching", authorization: "Bearer signed-c", dedicated: "signed-c", want: "signed-c"},
		{name: "conflict", authorization: "Bearer signed-a", dedicated: "signed-b", wantErr: true},
		{name: "wrong scheme", authorization: "Basic signed-a", wantErr: true},
		{name: "missing", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, "/mcp/cognition", nil)
			req.Header.Set("Authorization", tt.authorization)
			req.Header.Set("X-FMind-Binding-Token", tt.dedicated)
			got, err := bindingTokenFromRequest(req)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("got token=%q err=%v", got, err)
			}
		})
	}
}

func TestMCPTransportPublishesExactlyFrozenTools(t *testing.T) {
	server := NewServer(&verifierStub{}, &executorStub{})
	want := []string{
		ToolContextAssemble, ToolDocumentRead, ToolKnowledgeSearch,
		ToolMemoryCaptureTurn, ToolMemoryConfirmCandidate, ToolMemoryGetContext,
		ToolMemorySearch, ToolWikiGetPage,
	}
	if got := server.ToolNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("tool names = %v, want %v", got, want)
	}
	if server.HTTPHandler() == nil {
		t.Fatal("expected streamable MCP HTTP handler")
	}
}

func TestMCPContextNeverStoresRawAuthorizationHeader(t *testing.T) {
	server := NewServer(&verifierStub{}, &executorStub{})
	req, _ := http.NewRequest(http.MethodPost, "/mcp/cognition", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer secret-token")
	ctx := server.httpContext(context.Background(), req)
	if got, _ := ctx.Value(bindingTokenContextKey{}).(string); got != "secret-token" {
		t.Fatalf("binding token context = %q", got)
	}
	if strings.Contains(strings.TrimSpace(req.Header.Get("Authorization")), "secret-token") {
		t.Fatal("raw authorization header must be consumed before diagnostics")
	}
}

func authorizedArguments(tool string) map[string]any {
	switch tool {
	case ToolKnowledgeSearch:
		return map[string]any{"knowledge_base_ids": []any{"kb-1"}, "query": "x"}
	case ToolWikiGetPage:
		return map[string]any{"wiki_page_id": "page-1"}
	case ToolDocumentRead:
		return map[string]any{"document_id": "doc-1"}
	case ToolContextAssemble:
		return map[string]any{"asset_scopes": []any{"knowledge_base:kb-1"}}
	default:
		return map[string]any{}
	}
}
