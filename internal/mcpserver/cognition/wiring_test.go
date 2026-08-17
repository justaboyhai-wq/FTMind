package cognition

import (
	"context"
	"strings"
	"testing"

	"github.com/justaboyhai-wq/fmind/internal/types"
)

func TestNewMemoryGatewayFromEnvironmentDefaultsToDisabled(t *testing.T) {
	t.Setenv("FMIND_MEMORY_CORE_URL", "")
	t.Setenv("FMIND_MEMORY_CORE_API_KEY", "")
	t.Setenv("FMIND_MEMORY_CORE_SERVICE_ID", "")

	gateway, err := NewMemoryGatewayFromEnvironment()
	if err != nil || gateway == nil {
		t.Fatalf("gateway=%#v err=%v", gateway, err)
	}
	_, err = gateway.Invoke(context.Background(), ToolMemorySearch, validWiringBinding(), "token", map[string]any{"query": "x"}, "")
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("disabled gateway error=%v", err)
	}
}

func TestNewMemoryGatewayFromEnvironmentFailsClosedOnPartialConfiguration(t *testing.T) {
	t.Setenv("FMIND_MEMORY_CORE_URL", "https://memory.example")
	t.Setenv("FMIND_MEMORY_CORE_API_KEY", "")
	t.Setenv("FMIND_MEMORY_CORE_SERVICE_ID", "fmind")
	if _, err := NewMemoryGatewayFromEnvironment(); err == nil {
		t.Fatal("partial MemoryCore configuration must fail startup")
	}
}

func TestNewMemoryGatewayFromEnvironmentAllowsExplicitDevelopmentHTTP(t *testing.T) {
	t.Setenv("FMIND_MEMORY_CORE_URL", "http://memory-core:3000")
	t.Setenv("FMIND_MEMORY_CORE_API_KEY", "gateway-key")
	t.Setenv("FMIND_MEMORY_CORE_SERVICE_ID", "fmind")
	t.Setenv("FMIND_MEMORY_CORE_ALLOW_INSECURE_HTTP", "true")
	t.Setenv("FMIND_MEMORY_CORE_INSECURE_HOSTS", "memory-core")
	if gateway, err := NewMemoryGatewayFromEnvironment(); err != nil || gateway == nil {
		t.Fatalf("gateway=%#v err=%v", gateway, err)
	}
}

func validWiringBinding() types.BindingContext {
	return types.BindingContext{BindingID: "b", TeamID: "t", UserID: "u", AgentID: "a", PolicyVersion: 1}
}
