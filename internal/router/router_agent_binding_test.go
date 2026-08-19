package router

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/config"
	"github.com/justaboyhai-wq/fmind/internal/handler"
	"github.com/justaboyhai-wq/fmind/internal/mcpserver/cognition"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

type routerBindingServiceStub struct{}

func (*routerBindingServiceStub) Create(context.Context, interfaces.AgentBindingCreateRequest) (*interfaces.AgentBindingCreateResult, error) {
	return nil, nil
}

type routerCognitionExecutorStub struct{}

func (*routerCognitionExecutorStub) ExecuteCognitionTool(context.Context, cognition.Invocation) (any, error) {
	return map[string]any{"ok": true}, nil
}
func (*routerBindingServiceStub) List(context.Context) ([]*types.AgentBinding, error) {
	return []*types.AgentBinding{}, nil
}
func (*routerBindingServiceStub) Get(context.Context, string) (*types.AgentBinding, error) {
	return nil, nil
}
func (*routerBindingServiceStub) Revoke(context.Context, string) error { return nil }
func (*routerBindingServiceStub) RotateKey(context.Context, string, string) (string, error) {
	return "", nil
}
func (*routerBindingServiceStub) Introspect(context.Context, string) (*types.BindingIntrospectionResult, error) {
	return nil, nil
}
func (*routerBindingServiceStub) VerifyBindingToken(context.Context, string) (*types.BindingContext, error) {
	return &types.BindingContext{BindingID: "binding-1"}, nil
}

func TestAgentBindingManagementRoutesRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	g := &rbacGuards{cfg: &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantRoleContextKey, types.TenantRoleViewer)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	RegisterAgentBindingRoutes(r.Group("/api/v1"), handler.NewAgentBindingHandler(&routerBindingServiceStub{}), g)
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/agent-bindings"},
		{http.MethodGet, "/api/v1/agent-bindings/binding-1"},
		{http.MethodPost, "/api/v1/agent-bindings"},
		{http.MethodPost, "/api/v1/agent-bindings/binding-1/revoke"},
		{http.MethodPost, "/api/v1/agent-bindings/binding-1/keys/rotate"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s %s: viewer status=%d", tc.method, tc.path, w.Code)
		}
	}
}

func TestMemoryWikiReviewRoutesRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	g := &rbacGuards{cfg: &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantRoleContextKey, types.TenantRoleViewer)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	RegisterMemoryWikiReviewRoutes(r.Group("/api/v1"), &handler.MemoryWikiHandler{}, g)
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/api/v1/external-memory/l3/reviews", http.StatusInternalServerError},
		{http.MethodGet, "/api/v1/external-memory/l3/reviews/review-1", http.StatusInternalServerError},
		{http.MethodPost, "/api/v1/external-memory/l3/reviews/review-1/approve", http.StatusForbidden},
		{http.MethodPost, "/api/v1/external-memory/l3/reviews/review-1/reject", http.StatusForbidden},
		{http.MethodPost, "/api/v1/external-memory/l3/reviews/review-1/request-changes", http.StatusForbidden},
		{http.MethodPost, "/api/v1/external-memory/l3/reviews/review-1/publish", http.StatusForbidden},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatalf("%s %s: viewer status=%d want=%d", tc.method, tc.path, w.Code, tc.want)
		}
	}
}

func TestBindingIntrospectionRouteIsRegisteredBeforeUserAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterBindingIntrospectionRoutes(r, handler.NewBindingIntrospectionHandler(&routerBindingServiceStub{}))
	// Gin snapshots middleware at route registration. A global user-auth
	// middleware installed afterward must not be attached to introspection.
	r.Use(func(c *gin.Context) { c.AbortWithStatus(http.StatusTeapot) })
	for _, tc := range []struct {
		path   string
		header string
		value  string
	}{
		{path: "/internal/v1/agent-bindings/introspect", header: "X-FMind-Connector-Secret", value: "connector-only"},
		{path: "/internal/v1/agent-bindings/verify", header: "X-FMind-Binding-Token", value: "signed-token"},
	} {
		req := httptest.NewRequest(http.MethodPost, tc.path, nil)
		req.Header.Set(tc.header, tc.value)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s inherited later user auth: status=%d body=%s", tc.path, w.Code, w.Body.String())
		}
	}
}

func TestCognitionMCPRouteIsRegisteredBeforeUserAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	verifier := &routerBindingServiceStub{}
	server := cognition.NewServer(verifier, &routerCognitionExecutorStub{})
	r := gin.New()
	RegisterCognitionMCPRoutes(r, server)
	// Like connector verification, Cognition MCP authenticates with the
	// short-lived Binding Token and must not inherit the browser/API-key auth.
	r.Use(func(c *gin.Context) { c.AbortWithStatus(http.StatusTeapot) })
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp/cognition", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-FMind-Binding-Token", "signed-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusTeapot || w.Code == http.StatusNotFound {
		t.Fatalf("cognition MCP inherited later user auth or was not registered: status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestInternalMemoryEventRouteIsRegisteredBeforeUserAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterInternalMemoryEventRoutes(r, &handler.InternalMemoryEventHandler{})
	r.Use(func(c *gin.Context) { c.AbortWithStatus(http.StatusTeapot) })
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/memory/events", bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusTeapot || w.Code == http.StatusNotFound {
		t.Fatalf("internal memory route inherited user auth or is missing: status=%d body=%s", w.Code, w.Body.String())
	}
}
