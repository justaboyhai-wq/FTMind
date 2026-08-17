package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/config"
	"github.com/justaboyhai-wq/fmind/internal/handler"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

type routerBindingServiceStub struct{}

func (*routerBindingServiceStub) Create(context.Context, interfaces.AgentBindingCreateRequest) (*interfaces.AgentBindingCreateResult, error) {
	return nil, nil
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
	return nil, nil
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

func TestBindingIntrospectionRouteIsRegisteredBeforeUserAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterBindingIntrospectionRoutes(r, handler.NewBindingIntrospectionHandler(&routerBindingServiceStub{}))
	// Gin snapshots middleware at route registration. A global user-auth
	// middleware installed afterward must not be attached to introspection.
	r.Use(func(c *gin.Context) { c.AbortWithStatus(http.StatusTeapot) })
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/agent-bindings/introspect", nil)
	req.Header.Set("X-FMind-Connector-Secret", "connector-only")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("introspection inherited later user auth: status=%d body=%s", w.Code, w.Body.String())
	}
}
