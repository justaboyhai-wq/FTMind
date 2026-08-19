package agentbinding

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"gorm.io/gorm"
)

var ErrUnverifiableBindingScope = errors.New("agent binding scope cannot be verified")

var managedScopeIDPattern = assetScopeIDPattern

type databaseScopeValidator struct {
	db *gorm.DB
}

func NewScopeValidator(db *gorm.DB) interfaces.AgentBindingScopeValidator {
	return &databaseScopeValidator{db: db}
}

func (v *databaseScopeValidator) ValidateCreate(ctx context.Context, binding *types.AgentBinding) (types.StringArray, error) {
	if !types.TenantRoleFromContext(ctx).HasPermission(types.TenantRoleAdmin) && !types.IsSystemAdminFromContext(ctx) {
		return nil, fmt.Errorf("%w: binding creation requires tenant admin", ErrUnverifiableBindingScope)
	}
	return v.ResolveRoles(ctx, binding)
}

func (v *databaseScopeValidator) ResolveRoles(ctx context.Context, binding *types.AgentBinding) (types.StringArray, error) {
	if binding.UserAPIKeyID != 0 {
		var apiKey types.TenantAPIKey
		if err := v.db.WithContext(ctx).Where("id = ? AND tenant_id = ? AND user_id = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)", binding.UserAPIKeyID, binding.TenantID, binding.UserID).First(&apiKey).Error; err != nil {
			return nil, fmt.Errorf("%w: user api key", ErrUnverifiableBindingScope)
		}
	}
	// Team and department are authoritative organization resources. A binding
	// cannot mint a namespace for an arbitrary string: both rows must belong to
	// the authenticated tenant and the team must belong to the selected
	// department when one is supplied.
	var team types.Team
	if err := v.db.WithContext(ctx).Where("id = ? AND tenant_id = ? AND status = ?", binding.TeamID, binding.TenantID, "active").First(&team).Error; err != nil {
		return nil, fmt.Errorf("%w: team ownership", ErrUnverifiableBindingScope)
	}
	if binding.DepartmentID != "" && team.DepartmentID != binding.DepartmentID {
		return nil, fmt.Errorf("%w: department/team mismatch", ErrUnverifiableBindingScope)
	}
	// Built-in FMind agents are system-owned and are not tenant/team
	// resources. A binding must target a tenant custom agent that has been
	// explicitly attached to the selected team.
	if types.IsBuiltinAgentID(binding.AgentID) {
		return nil, fmt.Errorf("%w: built-in agents cannot be bound", ErrUnverifiableBindingScope)
	}
	var teamMember types.TeamMember
	if err := v.db.WithContext(ctx).Where("team_id = ? AND tenant_id = ? AND user_id = ? AND status = ?", binding.TeamID, binding.TenantID, binding.UserID, "active").First(&teamMember).Error; err != nil {
		return nil, fmt.Errorf("%w: team membership", ErrUnverifiableBindingScope)
	}
	if !types.TenantRoleFromContext(ctx).HasPermission(types.TenantRoleAdmin) && teamMember.Role != types.TeamRoleAdmin {
		return nil, fmt.Errorf("%w: team management role", ErrUnverifiableBindingScope)
	}
	var teamAgent types.TeamAgent
	if err := v.db.WithContext(ctx).Where("team_id = ? AND tenant_id = ? AND agent_id = ? AND status = ?", binding.TeamID, binding.TenantID, binding.AgentID, "active").First(&teamAgent).Error; err != nil {
		return nil, fmt.Errorf("%w: agent team ownership", ErrUnverifiableBindingScope)
	}
	// Workspace/Project/Task remain immutable namespace IDs until their
	// first-class resources are introduced. We validate a closed identifier
	// format and require every namespace to agree with its asset scope. Runtime
	// callers can never override them: only the signed Binding Context is
	// authoritative.
	managed := map[string]string{
		"team": binding.TeamID, "department": binding.DepartmentID,
		"workspace": binding.WorkspaceID, "project": binding.ProjectID, "task": binding.TaskID,
	}
	managedMaxLength := map[string]int{"team": 36, "department": 36, "workspace": 36, "project": 36, "task": 64}
	for kind, id := range managed {
		if id == "" {
			if kind == "team" {
				return nil, fmt.Errorf("%w: team is required", ErrUnverifiableBindingScope)
			}
			continue
		}
		if len(id) > managedMaxLength[kind] || !managedScopeIDPattern.MatchString(id) {
			return nil, fmt.Errorf("%w: invalid %s namespace", ErrUnverifiableBindingScope, kind)
		}
		if !containsString(binding.AssetScopes, kind+":"+id) {
			return nil, fmt.Errorf("%w: %s namespace is missing from asset_scopes", ErrUnverifiableBindingScope, kind)
		}
	}
	for _, asset := range binding.AssetScopes {
		kind, id, err := parseAssetScope(asset)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUnverifiableBindingScope, err)
		}
		if kind == "tenant" && id != strconv.FormatUint(binding.TenantID, 10) {
			return nil, fmt.Errorf("%w: tenant asset scope conflicts with binding namespace", ErrUnverifiableBindingScope)
		}
		if expected, ok := managed[kind]; ok && id != expected {
			return nil, fmt.Errorf("%w: %s asset scope conflicts with binding namespace", ErrUnverifiableBindingScope, kind)
		}
	}
	if err := v.validatePersistedAssetOwnership(ctx, binding); err != nil {
		return nil, err
	}
	var tenantMember types.TenantMember
	if err := v.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ? AND status = ?", binding.UserID, binding.TenantID, types.TenantMemberStatusActive).
		First(&tenantMember).Error; err != nil {
		return nil, fmt.Errorf("%w: tenant membership", ErrUnverifiableBindingScope)
	}
	if !tenantMember.Role.IsValid() {
		return nil, fmt.Errorf("%w: tenant role", ErrUnverifiableBindingScope)
	}
	if !types.IsBuiltinAgentID(binding.AgentID) {
		var count int64
		if err := v.db.WithContext(ctx).Model(&types.CustomAgent{}).
			Where("id = ? AND tenant_id = ?", binding.AgentID, binding.TenantID).Count(&count).Error; err != nil || count != 1 {
			return nil, fmt.Errorf("%w: agent ownership", ErrUnverifiableBindingScope)
		}
	}
	return types.StringArray{
		"tenant:" + string(tenantMember.Role),
	}, nil
}

func (v *databaseScopeValidator) validatePersistedAssetOwnership(ctx context.Context, binding *types.AgentBinding) error {
	knowledgeBaseScopes := make(map[string]struct{})
	knowledgeBaseIDs := make([]string, 0)
	wikiPageIDs := make([]string, 0)
	documentIDs := make([]string, 0)
	for _, asset := range binding.AssetScopes {
		kind, id, err := parseAssetScope(asset)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrUnverifiableBindingScope, err)
		}
		switch kind {
		case "knowledge_base":
			knowledgeBaseScopes[id] = struct{}{}
			knowledgeBaseIDs = appendUniqueID(knowledgeBaseIDs, id)
		case "wiki_page":
			wikiPageIDs = appendUniqueID(wikiPageIDs, id)
		case "document":
			documentIDs = appendUniqueID(documentIDs, id)
		}
	}

	pagesByID := make(map[string]types.WikiPage, len(wikiPageIDs))
	if len(wikiPageIDs) > 0 {
		var pages []types.WikiPage
		if err := v.db.WithContext(ctx).
			Where("tenant_id = ? AND id IN ?", binding.TenantID, wikiPageIDs).
			Find(&pages).Error; err != nil {
			return fmt.Errorf("%w: wiki_page asset lookup", ErrUnverifiableBindingScope)
		}
		for _, page := range pages {
			pagesByID[page.ID] = page
		}
		for _, id := range wikiPageIDs {
			page, ok := pagesByID[id]
			if !ok {
				return fmt.Errorf("%w: wiki_page asset %q is not owned by tenant", ErrUnverifiableBindingScope, id)
			}
			if !matchesKnowledgeBaseHierarchy(page.KnowledgeBaseID, knowledgeBaseScopes) {
				return fmt.Errorf("%w: wiki_page asset %q conflicts with knowledge_base hierarchy", ErrUnverifiableBindingScope, id)
			}
			knowledgeBaseIDs = appendUniqueID(knowledgeBaseIDs, page.KnowledgeBaseID)
		}
	}

	documentsByID := make(map[string]types.Knowledge, len(documentIDs))
	if len(documentIDs) > 0 {
		var documents []types.Knowledge
		if err := v.db.WithContext(ctx).
			Where("tenant_id = ? AND id IN ?", binding.TenantID, documentIDs).
			Find(&documents).Error; err != nil {
			return fmt.Errorf("%w: document asset lookup", ErrUnverifiableBindingScope)
		}
		for _, document := range documents {
			documentsByID[document.ID] = document
		}
		for _, id := range documentIDs {
			document, ok := documentsByID[id]
			if !ok {
				return fmt.Errorf("%w: document asset %q is not owned by tenant", ErrUnverifiableBindingScope, id)
			}
			if !matchesKnowledgeBaseHierarchy(document.KnowledgeBaseID, knowledgeBaseScopes) {
				return fmt.Errorf("%w: document asset %q conflicts with knowledge_base hierarchy", ErrUnverifiableBindingScope, id)
			}
			knowledgeBaseIDs = appendUniqueID(knowledgeBaseIDs, document.KnowledgeBaseID)
		}
	}

	if len(knowledgeBaseIDs) > 0 {
		var knowledgeBases []types.KnowledgeBase
		if err := v.db.WithContext(ctx).
			Where("tenant_id = ? AND id IN ?", binding.TenantID, knowledgeBaseIDs).
			Find(&knowledgeBases).Error; err != nil {
			return fmt.Errorf("%w: knowledge_base asset lookup", ErrUnverifiableBindingScope)
		}
		found := make(map[string]struct{}, len(knowledgeBases))
		for _, knowledgeBase := range knowledgeBases {
			found[knowledgeBase.ID] = struct{}{}
		}
		for _, id := range knowledgeBaseIDs {
			if _, ok := found[id]; !ok {
				return fmt.Errorf("%w: knowledge_base asset %q is not owned by tenant", ErrUnverifiableBindingScope, id)
			}
		}
	}
	// KnowledgeBase/WikiPage/Knowledge have no team/workspace/project columns.
	// Those namespaces remain binding-owned and immutable; tenant and the
	// persisted KB ancestry above are the deepest currently verifiable hierarchy.
	return nil
}

func appendUniqueID(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func matchesKnowledgeBaseHierarchy(parentID string, allowed map[string]struct{}) bool {
	if parentID == "" {
		return false
	}
	if len(allowed) == 0 {
		return true
	}
	_, ok := allowed[parentID]
	return ok
}

func containsString(values types.StringArray, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
