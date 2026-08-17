package cognition

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/justaboyhai-wq/fmind/internal/types"
)

func TestHTTPMemoryGatewayForwardsOnlyVerifiedAuthority(t *testing.T) {
	var received struct {
		authorization string
		bindingToken  string
		team          string
		user          string
		agent         string
		task          string
		body          map[string]any
		path          string
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.authorization = r.Header.Get("Authorization")
		received.bindingToken = r.Header.Get("X-FMind-Binding-Token")
		received.team = r.Header.Get("X-TDAI-Team-ID")
		received.user = r.Header.Get("X-TDAI-User-ID")
		received.agent = r.Header.Get("X-TDAI-Agent-ID")
		received.task = r.Header.Get("X-TDAI-Task-ID")
		received.path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&received.body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":0,"message":"ok","request_id":"r1","data":{"items":[]}}`)
	}))
	defer upstream.Close()

	client, err := NewHTTPMemoryGateway(MemoryGatewayConfig{BaseURL: upstream.URL, APIKey: "gateway-key", ServiceID: "fmind", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	binding := types.BindingContext{BindingID: "binding-1", TenantID: 7, TeamID: "team-1", UserID: "user-1", AgentID: "agent-1", TaskID: "task-1", PolicyVersion: 3}
	result, err := client.Invoke(context.Background(), ToolMemorySearch, binding, "binding-token", map[string]any{"query": "roadmap"}, "trace-1")
	if err != nil || result == nil {
		t.Fatalf("invoke result=%#v err=%v", result, err)
	}
	if received.authorization != "Bearer gateway-key" || received.bindingToken != "binding-token" || received.team != "team-1" || received.user != "user-1" || received.agent != "agent-1" || received.task != "task-1" {
		t.Fatalf("authority headers missing: %#v", received)
	}
	if received.path != "/v3/atomic/search" || received.body["query"] != "roadmap" {
		t.Fatalf("unexpected request: %#v", received)
	}
	encoded, _ := json.Marshal(received.body)
	if strings.Contains(string(encoded), "binding-token") || strings.Contains(string(encoded), "team-1") {
		t.Fatalf("token or authoritative identity leaked into tool body: %s", encoded)
	}
}

func TestHTTPMemoryGatewayMapsCaptureAndCognitionRoutes(t *testing.T) {
	want := map[string]string{
		ToolMemoryCaptureTurn:      "/v3/conversation/add",
		ToolMemorySearch:           "/v3/atomic/search",
		ToolMemoryGetContext:       "/v3/cognition/context",
		ToolMemoryConfirmCandidate: "/v3/cognition/candidate/confirm",
	}
	for tool, path := range want {
		t.Run(tool, func(t *testing.T) {
			var gotPath string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				_, _ = io.WriteString(w, `{"code":0,"data":{}}`)
			}))
			defer upstream.Close()
			client, err := NewHTTPMemoryGateway(MemoryGatewayConfig{BaseURL: upstream.URL, APIKey: "key", ServiceID: "service", Timeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Invoke(context.Background(), tool, types.BindingContext{BindingID: "b", TeamID: "t", UserID: "u", AgentID: "a", PolicyVersion: 1}, "token", map[string]any{}, "")
			if err != nil || gotPath != path {
				t.Fatalf("path=%q err=%v want=%q", gotPath, err, path)
			}
		})
	}
}

func TestHTTPMemoryGatewayRejectsUnsafeOrIncompleteConfiguration(t *testing.T) {
	tests := []MemoryGatewayConfig{
		{},
		{BaseURL: "http://memory-core:3000", APIKey: "key", ServiceID: "service"},
		{BaseURL: "https://memory.example", ServiceID: "service"},
		{BaseURL: "https://memory.example", APIKey: "key"},
		{BaseURL: "http://memory-core:3000", APIKey: "key", ServiceID: "service", AllowInsecureHTTP: true, InsecureHosts: []string{"another-host"}},
	}
	for _, config := range tests {
		if _, err := NewHTTPMemoryGateway(config); err == nil {
			t.Fatalf("accepted unsafe config: %#v", config)
		}
	}
}

func TestHTTPMemoryGatewayFailsClosedOnTimeoutAndRedactsUpstreamErrors(t *testing.T) {
	binding := types.BindingContext{BindingID: "b", TeamID: "t", UserID: "u", AgentID: "a", PolicyVersion: 1}
	t.Run("timeout", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			_, _ = io.WriteString(w, `{"code":0,"data":{}}`)
		}))
		defer upstream.Close()
		client, err := NewHTTPMemoryGateway(MemoryGatewayConfig{BaseURL: upstream.URL, APIKey: "key", ServiceID: "service", Timeout: 20 * time.Millisecond})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = client.Invoke(context.Background(), ToolMemorySearch, binding, "token", map[string]any{"query": "x"}, ""); err == nil {
			t.Fatal("timeout must fail closed")
		}
	})
	t.Run("redacted upstream error", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "database password=secret", http.StatusInternalServerError)
		}))
		defer upstream.Close()
		client, err := NewHTTPMemoryGateway(MemoryGatewayConfig{BaseURL: upstream.URL, APIKey: "key", ServiceID: "service", Timeout: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Invoke(context.Background(), ToolMemorySearch, binding, "token", map[string]any{"query": "x"}, "")
		if err == nil || strings.Contains(err.Error(), "password") || strings.Contains(err.Error(), "secret") {
			t.Fatalf("unsafe upstream error: %v", err)
		}
	})
}
