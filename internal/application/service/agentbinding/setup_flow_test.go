package agentbinding

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

func TestCreateBindingStartsPendingSetupWithOneTimeManifest(t *testing.T) {
	t.Setenv("FMIND_AGENT_BINDING_SETUP_FLOW", "true")
	t.Setenv("FMIND_PUBLIC_BASE_URL", "https://fmind.example.com")
	t.Setenv("MEMORY_CORE_PUBLIC_URL", "https://memory.example.com")
	t.Setenv("MEMORY_PROXY_PUBLIC_URL", "https://proxy.example.com")
	svc, _, _, ctx := newBindingService(t)
	result, err := svc.Create(ctx, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	if result.Binding.Status != types.AgentBindingStatusPendingSetup {
		t.Fatalf("expected pending_setup, got %q", result.Binding.Status)
	}
	if result.CredentialPurpose != "memory_binding_setup" || result.SetupExpiresAt == nil || result.SetupPrompt == "" {
		t.Fatalf("setup metadata missing: %+v", result)
	}
	if !strings.Contains(result.SetupPrompt, result.ConnectorSecret) || !strings.Contains(result.SetupPrompt, result.Binding.ID) {
		t.Fatal("setup prompt does not contain one-time binding instructions")
	}
}

func TestSetupDoesNotRequirePublicMemoryCoreEndpoint(t *testing.T) {
	t.Setenv("FMIND_AGENT_BINDING_SETUP_FLOW", "true")
	t.Setenv("FMIND_PUBLIC_BASE_URL", "https://fmind.example.com")
	t.Setenv("MEMORY_PROXY_PUBLIC_URL", "https://proxy.example.com")
	t.Setenv("MEMORY_CORE_PUBLIC_URL", "")
	svc, _, _, ctx := newBindingService(t)
	result, err := svc.Create(ctx, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	if result.SetupPrompt == "" || result.SetupManifest == nil {
		t.Fatal("expected gateway-only setup artifacts")
	}
	if result.SetupManifest.MemoryCoreEndpoint != "" {
		t.Fatalf("MemoryCore must remain private, got %q", result.SetupManifest.MemoryCoreEndpoint)
	}
}

func TestSetupConsumesSecretExactlyOnceAndActivatesBinding(t *testing.T) {
	t.Setenv("FMIND_AGENT_BINDING_SETUP_FLOW", "true")
	t.Setenv("FMIND_PUBLIC_BASE_URL", "https://fmind.example.com")
	t.Setenv("MEMORY_CORE_PUBLIC_URL", "https://memory.example.com")
	t.Setenv("MEMORY_PROXY_PUBLIC_URL", "https://proxy.example.com")
	svc, bindings, keys, ctx := newBindingService(t)
	created, err := svc.Create(ctx, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	setup, ok := any(svc).(interface {
		Setup(context.Context, interfaces.AgentBindingSetupRequest) (*interfaces.AgentBindingSetupResult, error)
	})
	if !ok {
		t.Fatal("service does not expose setup exchange")
	}
	result, err := setup.Setup(ctx, interfaces.AgentBindingSetupRequest{
		BindingID: created.Binding.ID, ExternalAgent: "openclaw", ConnectorType: "openclaw_plugin", ClientVersion: "test"})
	if err == nil || result != nil {
		t.Fatal("setup must not use browser context without connector secret")
	}
	setupCtx := context.Background()
	result, err = setup.Setup(setupCtx, interfaces.AgentBindingSetupRequest{
		BindingID: created.Binding.ID, ExternalAgent: "openclaw", ConnectorType: "openclaw_plugin", ClientVersion: "test", ConnectorSecret: created.ConnectorSecret})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != types.AgentBindingStatusActive || result.ConnectorSecret == "" || result.ConnectorSecret == created.ConnectorSecret {
		t.Fatalf("unexpected activation result: %+v", result)
	}
	if bindings.items[created.Binding.ID].Status != types.AgentBindingStatusActive || len(keys.keys) < 2 {
		t.Fatal("binding/key lifecycle was not activated")
	}
	if _, err := setup.Setup(setupCtx, interfaces.AgentBindingSetupRequest{BindingID: created.Binding.ID, ExternalAgent: "openclaw", ConnectorType: "openclaw_plugin", ConnectorSecret: created.ConnectorSecret}); err == nil {
		t.Fatal("setup secret replay must fail")
	}
}

func TestSetupRejectsExpiredSecret(t *testing.T) {
	t.Setenv("FMIND_AGENT_BINDING_SETUP_FLOW", "true")
	t.Setenv("FMIND_PUBLIC_BASE_URL", "https://fmind.example.com")
	t.Setenv("MEMORY_CORE_PUBLIC_URL", "https://memory.example.com")
	t.Setenv("MEMORY_PROXY_PUBLIC_URL", "https://proxy.example.com")
	svc, _, _, ctx := newBindingService(t)
	created, err := svc.Create(ctx, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	created.Binding.SetupExpiresAt = ptrTime(time.Now().UTC().Add(-time.Minute))
	if err := svc.bindings.UpdateAgentBinding(ctx, created.Binding); err != nil {
		t.Fatal(err)
	}
	setup, ok := any(svc).(interface {
		Setup(context.Context, interfaces.AgentBindingSetupRequest) (*interfaces.AgentBindingSetupResult, error)
	})
	if !ok {
		t.Fatal("service does not expose setup exchange")
	}
	if _, err := setup.Setup(context.Background(), interfaces.AgentBindingSetupRequest{BindingID: created.Binding.ID, ExternalAgent: "openclaw", ConnectorType: "openclaw_plugin", ConnectorSecret: created.ConnectorSecret}); err == nil {
		t.Fatal("expired setup secret must fail")
	}
}

func ptrTime(v time.Time) *time.Time { return &v }
