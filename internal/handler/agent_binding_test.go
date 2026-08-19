package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

type agentBindingServiceStub struct {
	createReq       interfaces.AgentBindingCreateRequest
	rotateCreatedBy string
	secret          string
	introErr        error
	createErr       error
	listErr         error
	getErr          error
	revokeErr       error
	rotateErr       error
	verifyToken     string
	verifyValue     *types.BindingContext
	verifyErr       error
}

type setupBindingServiceStub struct {
	agentBindingServiceStub
	setupRequest interfaces.AgentBindingSetupRequest
}

func (s *setupBindingServiceStub) Setup(_ context.Context, req interfaces.AgentBindingSetupRequest) (*interfaces.AgentBindingSetupResult, error) {
	s.setupRequest = req
	return &interfaces.AgentBindingSetupResult{BindingID: req.BindingID, Status: types.AgentBindingStatusActive, ConnectorSecret: "fmind_runtime"}, nil
}
func (s *setupBindingServiceStub) SetupStatus(context.Context, string) (*interfaces.AgentBindingSetupStatus, error) {
	return &interfaces.AgentBindingSetupStatus{Status: types.AgentBindingStatusPendingSetup}, nil
}

func TestAgentBindingSetupConsumesDedicatedHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &setupBindingServiceStub{}
	r := gin.New()
	r.POST("/internal/v1/agent-bindings/setup", NewAgentBindingHandler(svc).Setup)
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/agent-bindings/setup", bytes.NewBufferString(`{"binding_id":"binding-1","external_agent":"openclaw","connector_type":"openclaw_plugin"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-FMind-Connector-Secret", "fmind_setup")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || svc.setupRequest.ConnectorSecret != "fmind_setup" {
		t.Fatalf("setup header was not passed to service: status=%d body=%s request=%+v", w.Code, w.Body.String(), svc.setupRequest)
	}
}

func (s *agentBindingServiceStub) Create(_ context.Context, req interfaces.AgentBindingCreateRequest) (*interfaces.AgentBindingCreateResult, error) {
	s.createReq = req
	if s.createErr != nil {
		return nil, s.createErr
	}
	return &interfaces.AgentBindingCreateResult{Binding: &types.AgentBinding{ID: "binding-1"}, ConnectorSecret: s.secret}, nil
}
func (s *agentBindingServiceStub) List(context.Context) ([]*types.AgentBinding, error) {
	return nil, s.listErr
}
func (s *agentBindingServiceStub) Get(context.Context, string) (*types.AgentBinding, error) {
	return nil, s.getErr
}
func (s *agentBindingServiceStub) Revoke(context.Context, string) error { return s.revokeErr }
func (s *agentBindingServiceStub) RotateKey(_ context.Context, _ string, createdBy string) (string, error) {
	s.rotateCreatedBy = createdBy
	if s.rotateErr != nil {
		return "", s.rotateErr
	}
	return s.secret, nil
}
func (s *agentBindingServiceStub) Introspect(_ context.Context, secret string) (*types.BindingIntrospectionResult, error) {
	if s.introErr != nil || secret != s.secret {
		return nil, context.Canceled
	}
	return &types.BindingIntrospectionResult{BindingToken: "signed.jwt.value", Context: types.BindingContext{BindingID: "binding-1"}}, nil
}

func TestAgentBindingRotatePropagatesAuthenticatedActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &agentBindingServiceStub{secret: "fmind_once"}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(types.UserIDContextKey.String(), "admin-1"); c.Next() })
	RegisterAgentBindingRoutes(r.Group("/api/v1"), NewAgentBindingHandler(svc))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent-bindings/binding-1/keys/rotate", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if svc.rotateCreatedBy != "admin-1" {
		t.Fatalf("authenticated rotation actor was not propagated: %q", svc.rotateCreatedBy)
	}
}
func (s *agentBindingServiceStub) VerifyBindingToken(_ context.Context, token string) (*types.BindingContext, error) {
	s.verifyToken = token
	return s.verifyValue, s.verifyErr
}

func TestAgentBindingCreateBindsOrganizationPolicyAndUsesSnakeCaseSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &agentBindingServiceStub{secret: "fmind_once"}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(types.UserIDContextKey.String(), "admin-1"); c.Next() })
	RegisterAgentBindingRoutes(r.Group("/api/v1"), NewAgentBindingHandler(svc))
	body := `{"department_id":"department-1","team_id":"team-1","workspace_id":"workspace-1","project_id":"project-1","user_id":"user-1","agent_id":"agent-1","task_id":"task-1","external_agent":"external","connector_type":"generic_sdk","capture_enabled":true,"recall_enabled":true,"l3_wiki_enabled":true,"l3_review_required":true,"policy_version":3,"capability_scopes":["memory.capture"],"asset_scopes":["team:team-1"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent-bindings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if svc.createReq.TeamID != "team-1" || svc.createReq.UserID != "user-1" || svc.createReq.TaskID != "task-1" || !svc.createReq.L3WikiEnabled {
		t.Fatalf("incomplete request mapping: %+v", svc.createReq)
	}
	if svc.createReq.CreatedBy != "admin-1" {
		t.Fatalf("authenticated actor was not propagated: %+v", svc.createReq)
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["connector_secret"] != "fmind_once" {
		t.Fatalf("secret response is not snake_case: %s", w.Body.String())
	}
	if _, exists := got["ConnectorSecret"]; exists {
		t.Fatalf("PascalCase secret leaked into response: %s", w.Body.String())
	}
}

func TestBindingIntrospectionUsesUniformUnauthorizedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &agentBindingServiceStub{secret: "fmind_valid"}
	r := gin.New()
	RegisterBindingIntrospectionRoutes(r, NewBindingIntrospectionHandler(svc))
	var response string
	for _, secret := range []string{"", "fmind_wrong"} {
		req := httptest.NewRequest(http.MethodPost, "/internal/v1/agent-bindings/introspect", nil)
		if secret != "" {
			req.Header.Set("X-FMind-Connector-Secret", secret)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("secret=%q status=%d", secret, w.Code)
		}
		if response == "" {
			response = w.Body.String()
		} else if response != w.Body.String() {
			t.Fatalf("unauthorized responses disclose failure reason: %q != %q", response, w.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/agent-bindings/introspect", nil)
	req.Header.Set("X-FMind-Connector-Secret", "fmind_valid")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("valid connector secret status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestBindingVerificationUsesBearerOrDedicatedTokenAndRejectsConflicts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	contextValue := &types.BindingContext{TokenID: "jti-1", BindingID: "binding-1", TenantID: 7, TeamID: "team-1", UserID: "user-1", AgentID: "agent-1"}
	tests := []struct {
		name          string
		authorization string
		dedicated     string
		wantStatus    int
		wantToken     string
	}{
		{name: "bearer", authorization: "Bearer signed-token", wantStatus: http.StatusOK, wantToken: "signed-token"},
		{name: "dedicated", dedicated: "signed-token", wantStatus: http.StatusOK, wantToken: "signed-token"},
		{name: "matching", authorization: "Bearer signed-token", dedicated: "signed-token", wantStatus: http.StatusOK, wantToken: "signed-token"},
		{name: "missing", wantStatus: http.StatusUnauthorized},
		{name: "wrong scheme", authorization: "Basic signed-token", wantStatus: http.StatusUnauthorized},
		{name: "conflicting", authorization: "Bearer signed-a", dedicated: "signed-b", wantStatus: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &agentBindingServiceStub{verifyValue: contextValue}
			r := gin.New()
			RegisterBindingIntrospectionRoutes(r, NewBindingIntrospectionHandler(svc))
			req := httptest.NewRequest(http.MethodPost, "/internal/v1/agent-bindings/verify", nil)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			if tt.dedicated != "" {
				req.Header.Set("X-FMind-Binding-Token", tt.dedicated)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if svc.verifyToken != tt.wantToken {
				t.Fatalf("verified token=%q want=%q", svc.verifyToken, tt.wantToken)
			}
			if tt.wantStatus == http.StatusOK {
				var response struct {
					Context types.BindingContext `json:"context"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response.Context.BindingID != contextValue.BindingID {
					t.Fatalf("unexpected context: %#v", response.Context)
				}
			}
		})
	}
}

func TestBindingVerificationFailureIsUniformUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &agentBindingServiceStub{verifyErr: errors.New("database detail")}
	r := gin.New()
	RegisterBindingIntrospectionRoutes(r, NewBindingIntrospectionHandler(svc))
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/agent-bindings/verify", nil)
	req.Header.Set("X-FMind-Binding-Token", "signed-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized || bytes.Contains(w.Body.Bytes(), []byte("database detail")) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestAgentBindingHandlersDoNotLeakUnknownServiceErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const internalDetail = "postgres password=do-not-leak"
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		setup  func(*agentBindingServiceStub)
	}{
		{name: "create", method: http.MethodPost, path: "/api/v1/agent-bindings", body: `{"team_id":"team","user_id":"user","agent_id":"agent","external_agent":"external","connector_type":"generic_sdk"}`, setup: func(s *agentBindingServiceStub) { s.createErr = errors.New(internalDetail) }},
		{name: "list", method: http.MethodGet, path: "/api/v1/agent-bindings", setup: func(s *agentBindingServiceStub) { s.listErr = errors.New(internalDetail) }},
		{name: "get", method: http.MethodGet, path: "/api/v1/agent-bindings/binding-1", setup: func(s *agentBindingServiceStub) { s.getErr = errors.New(internalDetail) }},
		{name: "revoke", method: http.MethodPost, path: "/api/v1/agent-bindings/binding-1/revoke", setup: func(s *agentBindingServiceStub) { s.revokeErr = errors.New(internalDetail) }},
		{name: "rotate", method: http.MethodPost, path: "/api/v1/agent-bindings/binding-1/keys/rotate", setup: func(s *agentBindingServiceStub) { s.rotateErr = errors.New(internalDetail) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &agentBindingServiceStub{}
			tt.setup(svc)
			r := gin.New()
			RegisterAgentBindingRoutes(r.Group("/api/v1"), NewAgentBindingHandler(svc))
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusInternalServerError {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if bytes.Contains(w.Body.Bytes(), []byte(internalDetail)) {
				t.Fatalf("internal error leaked: %s", w.Body.String())
			}
		})
	}
}
