package agentbinding

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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
	ErrBindingAdminRequired = errors.New("agent binding management requires tenant admin")
	supportedConnectors     = map[string]struct{}{
		"openclaw_plugin": {}, "hermes_provider": {}, "openai_proxy": {},
		"anthropic_proxy": {}, "generic_sdk": {},
	}
	// Closed allowlist for the frozen Cognition tools:
	// memory_get_context -> memory.context; memory_search -> memory.recall;
	// memory_capture_turn -> memory.capture; memory_confirm_candidate ->
	// memory.confirm; knowledge_search/wiki_get_page/document_read/
	// context_assemble -> their dotted equivalents. memory.l3.publish is the
	// separate reviewed publication capability and never bypasses review.
	supportedCapabilities = map[string]struct{}{
		"memory.context": {}, "memory.capture": {}, "memory.recall": {},
		"memory.confirm": {}, "memory.l3.publish": {}, "knowledge.search": {},
		"wiki.get": {}, "document.read": {}, "context.assemble": {},
	}
	supportedAssetKinds = map[string]struct{}{
		"tenant": {}, "team": {}, "department": {}, "workspace": {},
		"project": {}, "task": {}, "knowledge_base": {}, "wiki_page": {}, "document": {},
	}
	assetScopeIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
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
	binding := &types.AgentBinding{
		ID: uuid.NewString(), TenantID: tenantID, DepartmentID: req.DepartmentID,
		TeamID: req.TeamID, WorkspaceID: req.WorkspaceID, ProjectID: req.ProjectID,
		UserID: req.UserID, AgentID: req.AgentID, TaskID: req.TaskID,
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
	key := &types.AgentBindingKey{
		ID: uuid.NewString(), BindingID: binding.ID, TenantID: tenantID,
		KeyPrefix: connectorSecretPrefix(secret), KeyHash: hash,
		CreatedBy: req.CreatedBy, ExpiresAt: req.ExpiresAt,
	}
	if err := s.bindings.CreateAgentBindingWithKey(ctx, binding, key); err != nil {
		return nil, err
	}
	return &interfaces.AgentBindingCreateResult{Binding: binding, ConnectorSecret: secret}, nil
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
	if binding.Status != types.AgentBindingStatusActive || bindingExpired(binding, time.Now().UTC()) {
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
	key := &types.AgentBindingKey{
		ID: uuid.NewString(), BindingID: id, TenantID: tenantID,
		KeyPrefix: connectorSecretPrefix(secret), KeyHash: hash,
		CreatedBy: createdBy, ExpiresAt: binding.ExpiresAt,
	}
	if err := s.bindings.RotateAgentBindingKey(ctx, tenantID, id, key); err != nil {
		return "", err
	}
	return secret, nil
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
	AgentID          string            `json:"agent_id"`
	TaskID           string            `json:"task_id,omitempty"`
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
		UserID: value.UserID, AgentID: value.AgentID, TaskID: value.TaskID, ConnectorType: value.ConnectorType,
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
		claims.TeamID == "" || claims.UserID == "" || claims.AgentID == "" || claims.ConnectorType == "" {
		return nil, ErrInvalidBinding
	}
	verified := &types.BindingContext{
		TokenID: claims.ID, BindingID: claims.BindingID, TenantID: claims.TenantID,
		DepartmentID: claims.DepartmentID, TeamID: claims.TeamID, WorkspaceID: claims.WorkspaceID,
		ProjectID: claims.ProjectID, UserID: claims.UserID, AgentID: claims.AgentID,
		TaskID: claims.TaskID, ConnectorType: claims.ConnectorType,
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
		ProjectID: binding.ProjectID, UserID: binding.UserID, AgentID: binding.AgentID,
		TaskID: binding.TaskID, ConnectorType: binding.ConnectorType,
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
		left.ProjectID == right.ProjectID && left.UserID == right.UserID && left.AgentID == right.AgentID &&
		left.TaskID == right.TaskID && left.ConnectorType == right.ConnectorType && stringArraysEqual(left.Roles, right.Roles) &&
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
	return binding.TenantID != 0 && binding.TeamID != "" && binding.UserID != "" && binding.AgentID != "" && binding.ConnectorType != ""
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
