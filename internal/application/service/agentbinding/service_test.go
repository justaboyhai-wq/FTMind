package agentbinding

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

type bindingRepoFake struct {
	items        map[string]*types.AgentBinding
	createErr    error
	rotateErr    error
	revokeErr    error
	keys         *keyRepoFake
	resolveCalls int
}

func (f *bindingRepoFake) ResolveActiveKeyAndBinding(_ context.Context, hash string, now time.Time) (*types.AgentBindingKey, *types.AgentBinding, error) {
	f.resolveCalls++
	key, err := f.keys.activeByHash(hash, now)
	if err != nil {
		return nil, nil, err
	}
	binding, err := f.GetAgentBinding(context.Background(), key.TenantID, key.BindingID)
	if err != nil || binding.Status != types.AgentBindingStatusActive || bindingExpired(binding, now) {
		return nil, nil, ErrInvalidBinding
	}
	return key, binding, nil
}

func (f *bindingRepoFake) TouchAgentBinding(_ context.Context, tenantID uint64, bindingID string, usedAt time.Time) error {
	b, err := f.GetAgentBinding(context.Background(), tenantID, bindingID)
	if err != nil {
		return err
	}
	b.LastUsedAt = &usedAt
	return nil
}

func (f *bindingRepoFake) CreateAgentBinding(_ context.Context, b *types.AgentBinding) error {
	if f.items == nil {
		f.items = map[string]*types.AgentBinding{}
	}
	f.items[b.ID] = b
	return nil
}
func (f *bindingRepoFake) CreateAgentBindingWithKey(_ context.Context, b *types.AgentBinding, k *types.AgentBindingKey) error {
	if f.createErr != nil {
		return f.createErr
	}
	_ = f.CreateAgentBinding(context.Background(), b)
	f.keys.keys = append(f.keys.keys, k)
	return nil
}
func (f *bindingRepoFake) GetAgentBinding(_ context.Context, tenantID uint64, id string) (*types.AgentBinding, error) {
	b := f.items[id]
	if b == nil || b.TenantID != tenantID {
		return nil, errors.New("not found")
	}
	return b, nil
}
func (f *bindingRepoFake) ListAgentBindings(_ context.Context, tenantID uint64) ([]*types.AgentBinding, error) {
	var out []*types.AgentBinding
	for _, b := range f.items {
		if b.TenantID == tenantID {
			out = append(out, b)
		}
	}
	return out, nil
}
func (f *bindingRepoFake) FindActiveAgentBinding(_ context.Context, tenantID uint64, externalAgent, connectorType string) (*types.AgentBinding, error) {
	for _, b := range f.items {
		if b.TenantID == tenantID && b.ExternalAgent == externalAgent && b.ConnectorType == connectorType && b.Status == types.AgentBindingStatusActive {
			return b, nil
		}
	}
	return nil, errors.New("not found")
}
func (f *bindingRepoFake) UpdateAgentBinding(_ context.Context, b *types.AgentBinding) error {
	f.items[b.ID] = b
	return nil
}
func (f *bindingRepoFake) RevokeAgentBinding(_ context.Context, tenantID uint64, id string) error {
	b, err := f.GetAgentBinding(context.Background(), tenantID, id)
	if err != nil {
		return err
	}
	b.Status = types.AgentBindingStatusRevoked
	return nil
}
func (f *bindingRepoFake) RotateAgentBindingKey(_ context.Context, tenantID uint64, bindingID string, k *types.AgentBindingKey) error {
	if f.rotateErr != nil {
		return f.rotateErr
	}
	for _, old := range f.keys.keys {
		if old.TenantID == tenantID && old.BindingID == bindingID && old.RevokedAt == nil {
			now := time.Now()
			old.RevokedAt = &now
		}
	}
	f.keys.keys = append(f.keys.keys, k)
	f.items[bindingID].PolicyVersion++
	return nil
}
func (f *bindingRepoFake) RevokeAgentBindingWithKeys(_ context.Context, tenantID uint64, bindingID string) error {
	if f.revokeErr != nil {
		return f.revokeErr
	}
	b, err := f.GetAgentBinding(context.Background(), tenantID, bindingID)
	if err != nil {
		return err
	}
	b.Status = types.AgentBindingStatusRevoked
	return f.keys.RevokeAgentBindingKeys(context.Background(), tenantID, bindingID)
}

type keyRepoFake struct {
	keys     []*types.AgentBindingKey
	touchErr error
}

func (f *keyRepoFake) CreateAgentBindingKey(_ context.Context, k *types.AgentBindingKey) error {
	f.keys = append(f.keys, k)
	return nil
}
func (f *keyRepoFake) GetActiveAgentBindingKeyByHash(_ context.Context, hash string) (*types.AgentBindingKey, error) {
	return f.activeByHash(hash, time.Now())
}

func (f *keyRepoFake) activeByHash(hash string, now time.Time) (*types.AgentBindingKey, error) {
	for i := len(f.keys) - 1; i >= 0; i-- {
		k := f.keys[i]
		if k.KeyHash == hash && k.RevokedAt == nil && (k.ExpiresAt == nil || k.ExpiresAt.After(now)) {
			return k, nil
		}
	}
	return nil, errors.New("not found")
}

func TestAgentBindingIntrospectionUsesAtomicKeyBindingResolution(t *testing.T) {
	svc, bindings, _, ctx := newBindingService(t)
	created, err := svc.Create(ctx, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Introspect(context.Background(), created.ConnectorSecret); err != nil {
		t.Fatal(err)
	}
	if bindings.resolveCalls != 1 {
		t.Fatalf("introspection used split key/binding reads instead of atomic resolution: calls=%d", bindings.resolveCalls)
	}
}
func (f *keyRepoFake) RevokeAgentBindingKeys(_ context.Context, tenantID uint64, bindingID string) error {
	for _, k := range f.keys {
		if k.TenantID == tenantID && k.BindingID == bindingID {
			now := time.Now()
			k.RevokedAt = &now
		}
	}
	return nil
}
func (f *keyRepoFake) TouchAgentBindingKey(_ context.Context, _ string, _ time.Time) error {
	return f.touchErr
}

type scopeValidatorFake struct {
	err   error
	roles types.StringArray
}

func (f *scopeValidatorFake) ValidateCreate(_ context.Context, _ *types.AgentBinding) (types.StringArray, error) {
	return f.resolve()
}

func (f *scopeValidatorFake) ResolveRoles(_ context.Context, _ *types.AgentBinding) (types.StringArray, error) {
	return f.resolve()
}

func (f *scopeValidatorFake) resolve() (types.StringArray, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append(types.StringArray(nil), f.roles...), nil
}

func newBindingService(t *testing.T) (*Service, *bindingRepoFake, *keyRepoFake, context.Context) {
	t.Helper()
	t.Setenv("FMIND_BINDING_KEY_PEPPER", "test-pepper-at-least-32-bytes-long")
	t.Setenv("FMIND_BINDING_TOKEN_SECRET", "test-signing-secret-at-least-32-bytes")
	keys := &keyRepoFake{}
	bindings := &bindingRepoFake{items: map[string]*types.AgentBinding{}, keys: keys}
	svc := NewService(bindings, keys, &scopeValidatorFake{roles: types.StringArray{"tenant:admin", "organization:editor"}}).(*Service)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(42))
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleAdmin)
	return svc, bindings, keys, ctx
}

func validCreateRequest() interfaces.AgentBindingCreateRequest {
	return interfaces.AgentBindingCreateRequest{
		DepartmentID: "department-1", TeamID: "team-1", WorkspaceID: "workspace-1", ProjectID: "project-1",
		UserID: "user-1", AgentID: "agent-1", TaskID: "task-1", ExternalAgent: "openclaw", ConnectorType: "openclaw_plugin",
		CapabilityScopes: []string{"memory.context", "memory.capture", "memory.recall", "memory.confirm", "memory.l3.publish", "knowledge.search", "wiki.get", "document.read", "context.assemble"},
		AssetScopes:      []string{" team:team-1 ", "department:department-1", "workspace:workspace-1", "workspace:workspace-1", "project:project-1", "task:task-1"},
		CaptureEnabled:   true, RecallEnabled: true, L3WikiEnabled: true, L3ReviewRequired: true,
	}
}

func TestAgentBindingCreateUsesPepperedHMACAndIntrospectionCarriesSignedPolicy(t *testing.T) {
	svc, _, keys, ctx := newBindingService(t)
	created, err := svc.Create(ctx, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.ConnectorSecret, "fmind_") || len(keys.keys) != 1 {
		t.Fatalf("unexpected secret/key result: %+v", created)
	}
	if strings.Contains(keys.keys[0].KeyHash, created.ConnectorSecret) {
		t.Fatal("plaintext connector secret was persisted")
	}
	mac := hmac.New(sha256.New, []byte("test-pepper-at-least-32-bytes-long"))
	_, _ = mac.Write([]byte(created.ConnectorSecret))
	if keys.keys[0].KeyHash != hex.EncodeToString(mac.Sum(nil)) {
		t.Fatal("key was not hashed with HMAC-SHA256 and the server pepper")
	}

	result, err := svc.Introspect(context.Background(), created.ConnectorSecret)
	if err != nil {
		t.Fatal(err)
	}
	c := result.Context
	if c.TenantID != 42 || c.DepartmentID != "department-1" || c.TeamID != "team-1" || c.WorkspaceID != "workspace-1" || c.ProjectID != "project-1" || c.UserID != "user-1" || c.AgentID != "agent-1" || c.TaskID != "task-1" || c.ExternalAgent != "openclaw" || c.ConnectorType != "openclaw_plugin" {
		t.Fatalf("incomplete identity context: %+v", c)
	}
	if !c.CaptureEnabled || !c.RecallEnabled || !c.L3WikiEnabled || !c.L3ReviewRequired || c.PolicyVersion != 1 {
		t.Fatalf("binding policy was not copied to context: %+v", c)
	}
	if len(c.Roles) != 2 || c.Roles[0] != "tenant:admin" {
		t.Fatalf("server-computed roles missing from binding context: %#v", c.Roles)
	}
	if len(c.AssetScopes) != 5 {
		t.Fatalf("asset scopes were not normalized: %#v", c.AssetScopes)
	}
	verified, err := svc.VerifyBindingToken(context.Background(), result.BindingToken)
	if err != nil {
		t.Fatal(err)
	}
	if verified.BindingID != created.Binding.ID || verified.TokenID != c.TokenID || verified.PolicyVersion != 1 || verified.TeamID != c.TeamID || verified.UserID != c.UserID || verified.AgentID != c.AgentID || verified.ExternalAgent != "openclaw" || len(verified.CapabilityScopes) != 9 || len(verified.AssetScopes) != 5 {
		t.Fatalf("verified claims differ from introspection: %+v", verified)
	}
}

func TestAgentBindingCreateStartsPolicyVersionAtOne(t *testing.T) {
	svc, _, _, ctx := newBindingService(t)
	created, err := svc.Create(ctx, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	if created.Binding.PolicyVersion != 1 {
		t.Fatalf("client selected policy version %d", created.Binding.PolicyVersion)
	}
}

func TestAgentBindingSecretsRequireAtLeast32NonWhitespaceBytes(t *testing.T) {
	invalidSecrets := map[string]string{
		"empty":                  "",
		"whitespace":             strings.Repeat(" ", 32),
		"short":                  strings.Repeat("x", 31),
		"surrounding whitespace": " " + strings.Repeat("x", 32),
	}
	for name, invalid := range invalidSecrets {
		t.Run("pepper/"+name, func(t *testing.T) {
			svc, _, _, ctx := newBindingService(t)
			t.Setenv("FMIND_BINDING_KEY_PEPPER", invalid)
			if result, err := svc.Create(ctx, validCreateRequest()); err == nil || result != nil {
				t.Fatalf("accepted invalid pepper: result=%+v err=%v", result, err)
			}
		})
		t.Run("token_secret/"+name, func(t *testing.T) {
			svc, _, _, ctx := newBindingService(t)
			created, err := svc.Create(ctx, validCreateRequest())
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("FMIND_BINDING_TOKEN_SECRET", invalid)
			if result, err := svc.Introspect(context.Background(), created.ConnectorSecret); err == nil || result != nil {
				t.Fatalf("accepted invalid token secret: result=%+v err=%v", result, err)
			}
		})
	}
}

func TestAgentBindingCreateFailsClosedOnIdentityScopeAndPepperValidation(t *testing.T) {
	svc, _, _, ctx := newBindingService(t)
	for name, mutate := range map[string]func(*interfaces.AgentBindingCreateRequest){
		"team":  func(r *interfaces.AgentBindingCreateRequest) { r.TeamID = "" },
		"user":  func(r *interfaces.AgentBindingCreateRequest) { r.UserID = "" },
		"agent": func(r *interfaces.AgentBindingCreateRequest) { r.AgentID = "" },
		"external agent path injection": func(r *interfaces.AgentBindingCreateRequest) {
			r.ExternalAgent = "openclaw/../premium"
		},
		"external agent uppercase ambiguity": func(r *interfaces.AgentBindingCreateRequest) {
			r.ExternalAgent = "OpenClaw"
		},
		"unknown capability": func(r *interfaces.AgentBindingCreateRequest) { r.CapabilityScopes = []string{"memory.root"} },
		"blank asset":        func(r *interfaces.AgentBindingCreateRequest) { r.AssetScopes = []string{" "} },
		"capture flag without capability": func(r *interfaces.AgentBindingCreateRequest) {
			r.CapabilityScopes = []string{"memory.recall", "memory.l3.publish"}
		},
		"recall flag without capability": func(r *interfaces.AgentBindingCreateRequest) {
			r.CapabilityScopes = []string{"memory.capture", "memory.l3.publish"}
		},
		"L3 flag without capability": func(r *interfaces.AgentBindingCreateRequest) {
			r.CapabilityScopes = []string{"memory.capture", "memory.recall"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			req := validCreateRequest()
			mutate(&req)
			if _, err := svc.Create(ctx, req); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	adminWithoutTenant := context.WithValue(context.Background(), types.TenantRoleContextKey, types.TenantRoleAdmin)
	if _, err := svc.Create(adminWithoutTenant, validCreateRequest()); err == nil {
		t.Fatal("expected missing tenant error")
	}
	t.Setenv("FMIND_BINDING_KEY_PEPPER", "")
	if _, err := svc.Create(ctx, validCreateRequest()); err == nil {
		t.Fatal("expected missing pepper error")
	}
}

func TestAgentBindingAssetScopesUseClosedKindIDGrammar(t *testing.T) {
	svc, _, _, ctx := newBindingService(t)
	tests := map[string][]string{
		"unknown kind":  {"root:all"},
		"missing colon": {"team-1"},
		"missing id":    {"team:"},
		"wildcard":      {"team:*"},
		"extra colon":   {"team:team-1:child"},
	}
	for name, assets := range tests {
		t.Run(name, func(t *testing.T) {
			req := validCreateRequest()
			req.AssetScopes = assets
			if result, err := svc.Create(ctx, req); err == nil || result != nil {
				t.Fatalf("accepted invalid asset scope %#v: result=%+v err=%v", assets, result, err)
			}
		})
	}
}

func TestAgentBindingCreateForcesL3ReviewAndRejectsUnverifiedScope(t *testing.T) {
	t.Setenv("FMIND_BINDING_KEY_PEPPER", "test-pepper-at-least-32-bytes-long")
	t.Setenv("FMIND_BINDING_TOKEN_SECRET", "test-signing-secret-at-least-32-bytes")
	keys := &keyRepoFake{}
	bindings := &bindingRepoFake{items: map[string]*types.AgentBinding{}, keys: keys}
	validator := &scopeValidatorFake{roles: types.StringArray{"tenant:admin"}}
	svc := NewService(bindings, keys, validator).(*Service)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(42))
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleAdmin)
	req := validCreateRequest()
	req.L3ReviewRequired = false
	created, err := svc.Create(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !created.Binding.L3ReviewRequired {
		t.Fatal("L3 Wiki binding allowed publication without review")
	}
	validator.err = errors.New("scope does not belong to tenant")
	req.ExternalAgent = "other-external"
	if _, err := svc.Create(ctx, req); err == nil {
		t.Fatal("unverified organization scope was accepted")
	}
}

func TestAgentBindingCreateRejectsL3PublishCapabilityWhenL3PolicyIsDisabled(t *testing.T) {
	svc, _, _, ctx := newBindingService(t)
	for _, reviewRequired := range []bool{false, true} {
		req := validCreateRequest()
		req.L3WikiEnabled = false
		req.L3ReviewRequired = reviewRequired
		if result, err := svc.Create(ctx, req); err == nil || result != nil {
			t.Fatalf("L3 publish scope bypassed disabled policy (review=%v): result=%+v err=%v", reviewRequired, result, err)
		}
	}
}

func TestAgentBindingCreateRotateAndRevokeUseAtomicRepositoryOperations(t *testing.T) {
	svc, bindings, keys, ctx := newBindingService(t)
	bindings.createErr = errors.New("write key failed")
	if _, err := svc.Create(ctx, validCreateRequest()); err == nil || len(bindings.items) != 0 {
		t.Fatalf("create did not roll back cleanly: err=%v items=%d", err, len(bindings.items))
	}
	bindings.createErr = nil
	created, err := svc.Create(ctx, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	old := keys.keys[0]
	issued, err := svc.Introspect(context.Background(), created.ConnectorSecret)
	if err != nil {
		t.Fatal(err)
	}
	bindings.rotateErr = errors.New("new key insert failed")
	if _, err := svc.RotateKey(ctx, created.Binding.ID, "admin"); err == nil || old.RevokedAt != nil {
		t.Fatalf("failed rotation revoked old key: err=%v old=%+v", err, old)
	}
	bindings.rotateErr = nil
	newSecret, err := svc.RotateKey(ctx, created.Binding.ID, "admin")
	if err != nil || old.RevokedAt == nil {
		t.Fatalf("successful rotation did not revoke old key: %v", err)
	}
	if stale, err := svc.Introspect(context.Background(), created.ConnectorSecret); err == nil || stale != nil {
		t.Fatalf("old connector secret minted a token after rotation committed: result=%+v err=%v", stale, err)
	}
	if _, err := svc.VerifyBindingToken(context.Background(), issued.BindingToken); err == nil {
		t.Fatal("rotation left the previous binding token valid")
	}
	fresh, err := svc.Introspect(context.Background(), newSecret)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Context.PolicyVersion != 2 {
		t.Fatalf("rotated secret did not receive current policy generation: %d", fresh.Context.PolicyVersion)
	}
	bindings.revokeErr = errors.New("key revoke failed")
	if err := svc.Revoke(ctx, created.Binding.ID); err == nil || created.Binding.Status != types.AgentBindingStatusActive {
		t.Fatalf("failed revoke changed binding: err=%v status=%s", err, created.Binding.Status)
	}
	bindings.revokeErr = nil
	if err := svc.Revoke(ctx, created.Binding.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.VerifyBindingToken(context.Background(), fresh.BindingToken); err == nil {
		t.Fatal("revocation left the previous binding token valid")
	}
}

func TestAgentBindingIntrospectionRejectsExpiredOrRevokedBindingButIgnoresTouchFailure(t *testing.T) {
	svc, _, keys, ctx := newBindingService(t)
	created, err := svc.Create(ctx, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	keys.touchErr = errors.New("last-used write unavailable")
	if _, err := svc.Introspect(context.Background(), created.ConnectorSecret); err != nil {
		t.Fatalf("best-effort last-used update broke authentication: %v", err)
	}
	if created.Binding.LastUsedAt == nil {
		t.Fatal("successful introspection did not update binding last_used_at")
	}
	expired := time.Now().Add(-time.Second)
	created.Binding.ExpiresAt = &expired
	if _, err := svc.Introspect(context.Background(), created.ConnectorSecret); err == nil {
		t.Fatal("expected expired binding rejection")
	}
	created.Binding.ExpiresAt = nil
	created.Binding.Status = types.AgentBindingStatusRevoked
	if _, err := svc.Introspect(context.Background(), created.ConnectorSecret); err == nil {
		t.Fatal("expected revoked binding rejection")
	}
}

func TestVerifyBindingTokenRejectsServerRoleChange(t *testing.T) {
	t.Setenv("FMIND_BINDING_KEY_PEPPER", "test-pepper-at-least-32-bytes-long")
	t.Setenv("FMIND_BINDING_TOKEN_SECRET", "test-signing-secret-at-least-32-bytes")
	keys := &keyRepoFake{}
	bindings := &bindingRepoFake{items: map[string]*types.AgentBinding{}, keys: keys}
	validator := &scopeValidatorFake{roles: types.StringArray{"tenant:admin", "organization:editor"}}
	svc := NewService(bindings, keys, validator).(*Service)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(42))
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleAdmin)
	created, err := svc.Create(ctx, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	issued, err := svc.Introspect(context.Background(), created.ConnectorSecret)
	if err != nil {
		t.Fatal(err)
	}
	validator.roles = types.StringArray{"tenant:viewer", "organization:viewer"}
	if _, err := svc.VerifyBindingToken(context.Background(), issued.BindingToken); err == nil {
		t.Fatal("token retained elevated roles after authoritative membership changed")
	}
}

func TestVerifyBindingTokenRejectsExternalAgentClaimMismatch(t *testing.T) {
	svc, _, _, ctx := newBindingService(t)
	created, err := svc.Create(ctx, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	issued, err := svc.Introspect(context.Background(), created.ConnectorSecret)
	if err != nil {
		t.Fatal(err)
	}
	claims := &bindingTokenClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(issued.BindingToken, claims); err != nil {
		t.Fatal(err)
	}
	claims.ExternalAgent = "premium"
	tampered, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte("test-signing-secret-at-least-32-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if verified, err := svc.VerifyBindingToken(context.Background(), tampered); err == nil || verified != nil {
		t.Fatalf("accepted token routed to a different external agent: verified=%+v err=%v", verified, err)
	}
}

func TestVerifyBindingTokenRejectsTamperingAndMalformedTokens(t *testing.T) {
	svc, _, _, ctx := newBindingService(t)
	created, err := svc.Create(ctx, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.Introspect(context.Background(), created.ConnectorSecret)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(result.BindingToken, ".")
	if len(parts) != 3 {
		t.Fatalf("binding token is not a JWT: %q", result.BindingToken)
	}
	for _, token := range []string{result.BindingToken + "x", parts[0] + "." + parts[1] + "."} {
		if _, err := svc.VerifyBindingToken(context.Background(), token); err == nil {
			t.Fatalf("accepted invalid token %q", token)
		}
	}
	base := jwt.MapClaims{"iss": "fmind", "aud": "fmind-memory", "sub": "binding-1", "jti": "token-1", "iat": time.Now().Unix(), "nbf": time.Now().Add(-time.Second).Unix(), "exp": time.Now().Add(time.Minute).Unix(), "binding_id": "binding-1", "tenant_id": 42, "team_id": "team-1", "user_id": "user-1", "agent_id": "agent-1", "connector_type": "generic_sdk"}
	for name, mutate := range map[string]func(jwt.MapClaims){
		"issuer":   func(c jwt.MapClaims) { c["iss"] = "attacker" },
		"audience": func(c jwt.MapClaims) { c["aud"] = "other-service" },
		"expiry":   func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Minute).Unix() },
	} {
		t.Run(name, func(t *testing.T) {
			claims := jwt.MapClaims{}
			for k, v := range base {
				claims[k] = v
			}
			mutate(claims)
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-signing-secret-at-least-32-bytes"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.VerifyBindingToken(context.Background(), token); err == nil {
				t.Fatalf("accepted token with wrong %s", name)
			}
		})
	}
	noneToken, err := jwt.NewWithClaims(jwt.SigningMethodNone, base).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.VerifyBindingToken(context.Background(), noneToken); err == nil {
		t.Fatal("accepted non-HS256 algorithm")
	}
}

func TestVerifyBindingTokenRequiresBoundedTemporalClaims(t *testing.T) {
	svc, _, _, ctx := newBindingService(t)
	created, err := svc.Create(ctx, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.Introspect(context.Background(), created.ConnectorSecret)
	if err != nil {
		t.Fatal(err)
	}
	base := &bindingTokenClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(result.BindingToken, base); err != nil {
		t.Fatal(err)
	}

	tests := map[string]func(*bindingTokenClaims){
		"missing iat": func(c *bindingTokenClaims) { c.IssuedAt = nil },
		"missing nbf": func(c *bindingTokenClaims) { c.NotBefore = nil },
		"missing exp": func(c *bindingTokenClaims) { c.ExpiresAt = nil },
		"lifetime over five minutes": func(c *bindingTokenClaims) {
			c.ExpiresAt = jwt.NewNumericDate(c.IssuedAt.Time.Add(bindingTokenTTL + time.Minute))
		},
		"nbf far before iat": func(c *bindingTokenClaims) {
			c.NotBefore = jwt.NewNumericDate(c.IssuedAt.Time.Add(-time.Minute))
		},
		"iat in future": func(c *bindingTokenClaims) {
			future := time.Now().UTC().Add(time.Minute)
			c.IssuedAt = jwt.NewNumericDate(future)
			c.NotBefore = jwt.NewNumericDate(future)
			c.ExpiresAt = jwt.NewNumericDate(future.Add(time.Minute))
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			claims := *base
			claims.RegisteredClaims = base.RegisteredClaims
			mutate(&claims)
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims).
				SignedString([]byte("test-signing-secret-at-least-32-bytes"))
			if err != nil {
				t.Fatal(err)
			}
			if verified, err := svc.VerifyBindingToken(context.Background(), token); err == nil || verified != nil {
				t.Fatalf("accepted token with invalid temporal claims: verified=%+v err=%v", verified, err)
			}
		})
	}
}

func TestAgentBindingTenantIsolation(t *testing.T) {
	svc, _, _, ctx := newBindingService(t)
	created, err := svc.Create(ctx, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	other := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(99))
	other = context.WithValue(other, types.TenantRoleContextKey, types.TenantRoleAdmin)
	if _, err := svc.Get(other, created.Binding.ID); err == nil {
		t.Fatal("cross-tenant get succeeded")
	}
}

func TestAgentBindingManagementRequiresAdminEvenWhenRouteRBACIsBypassed(t *testing.T) {
	svc, bindings, keys, adminCtx := newBindingService(t)
	created, err := svc.Create(adminCtx, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	viewerCtx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(42))
	viewerCtx = context.WithValue(viewerCtx, types.TenantRoleContextKey, types.TenantRoleViewer)

	if result, err := svc.Create(viewerCtx, validCreateRequest()); err == nil || result != nil {
		t.Fatalf("viewer obtained create result or connector secret: result=%+v err=%v", result, err)
	}
	if result, err := svc.List(viewerCtx); err == nil || result != nil {
		t.Fatalf("viewer listed bindings: result=%+v err=%v", result, err)
	}
	if result, err := svc.Get(viewerCtx, created.Binding.ID); err == nil || result != nil {
		t.Fatalf("viewer fetched binding: result=%+v err=%v", result, err)
	}
	if err := svc.Revoke(viewerCtx, created.Binding.ID); err == nil {
		t.Fatal("viewer revoked binding")
	}
	keyCount := len(keys.keys)
	if secret, err := svc.RotateKey(viewerCtx, created.Binding.ID, "viewer"); err == nil || secret != "" {
		t.Fatalf("viewer obtained rotated connector secret: secret=%q err=%v", secret, err)
	}
	if bindings.items[created.Binding.ID].Status != types.AgentBindingStatusActive || len(keys.keys) != keyCount {
		t.Fatal("denied management request mutated binding state")
	}
}

func TestAgentBindingManagementAllowsSystemAdminWithTenantContext(t *testing.T) {
	svc, _, _, adminCtx := newBindingService(t)
	created, err := svc.Create(adminCtx, validCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	systemCtx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(42))
	systemCtx = context.WithValue(systemCtx, types.TenantRoleContextKey, types.TenantRoleViewer)
	systemCtx = context.WithValue(systemCtx, types.SystemAdminContextKey, true)
	if _, err := svc.Get(systemCtx, created.Binding.ID); err != nil {
		t.Fatalf("system admin could not manage tenant binding: %v", err)
	}
}
