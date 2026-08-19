package authz

import (
	"context"
	"errors"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"gorm.io/gorm"
)

var ErrForbidden = errors.New("forbidden")

type MenuPermission struct {
	Key     string   `json:"key"`
	Visible bool     `json:"visible"`
	Actions []string `json:"actions"`
}
type MenuPermissionSet struct {
	Menus      []MenuPermission    `json:"menus"`
	RoleMatrix map[string][]string `json:"role_matrix"`
}

type Service struct{ resolver *Resolver }

func NewService() *Service { return &Service{} }
func NewServiceWithDB(db *gorm.DB) *Service {
	return &Service{resolver: NewResolver(NewGormAccessSource(db))}
}

// ValidateBindingContext rehydrates the organization and resource graph for
// a verified data-plane binding. The signed binding claims remain the
// transport contract, but they are not the authority for current membership,
// team-agent relationships, or resource existence. A revoked membership or
// deleted team therefore fails the next token verification immediately.
func (s *Service) ValidateBindingContext(ctx context.Context, value types.BindingContext) error {
	if s == nil || s.resolver == nil {
		return nil
	}
	binding := &types.AgentBinding{
		ID: value.BindingID, TenantID: value.TenantID, DepartmentID: value.DepartmentID,
		TeamID: value.TeamID, WorkspaceID: value.WorkspaceID, ProjectID: value.ProjectID,
		UserID: value.UserID, AgentID: value.AgentID, TaskID: value.TaskID,
		ExternalAgent: value.ExternalAgent, ConnectorType: value.ConnectorType,
		CapabilityScopes: append(types.StringArray(nil), value.CapabilityScopes...),
		AssetScopes:      append(types.StringArray(nil), value.AssetScopes...),
		UserAPIKeyID:     value.UserAPIKeyID,
		PolicyVersion:    value.PolicyVersion,
	}
	_, err := s.resolver.Resolve(ctx, binding)
	return err
}

// ResolveContext rebuilds authorization state from the authenticated request.
// The fallback is intentionally limited to role defaults for lightweight
// installations and tests that do not wire a database resolver.
func (s *Service) ResolveContext(ctx context.Context) (context.Context, error) {
	if s != nil && s.resolver != nil {
		access, err := s.resolver.Resolve(ctx, nil)
		if err != nil {
			return ctx, err
		}
		return WithContext(ctx, access), nil
	}
	tenantID, _ := types.TenantIDFromContext(ctx)
	userID, _ := types.UserIDFromContext(ctx)
	role := types.TenantRoleFromContext(ctx)
	return WithContext(ctx, AccessContext{UserID: userID, TenantID: tenantID, TenantRole: role, Capabilities: DefaultCapabilities(role)}), nil
}
func (s *Service) Authorize(ctx context.Context, action Action, resource Resource) error {
	c, err := FromContext(ctx)
	if err != nil || c.TenantID == 0 {
		return ErrForbidden
	}
	if resource.TenantID != 0 && resource.TenantID != c.TenantID {
		return ErrForbidden
	}
	if resource.TeamID != "" && !c.HasTeam(resource.TeamID) {
		return ErrForbidden
	}
	if resource.Status == "revoked" || resource.Status == "archived" {
		if action == ActionRead || action == ActionSearch || action == ActionRecall {
			return ErrForbidden
		}
	}
	if !roleAllowsAction(c.TenantRole, action, resource.Type) {
		return ErrForbidden
	}
	required := requiredCapability(action, resource.Type)
	if required != "" && c.BindingID != "" && !hasRequiredCapability(c, action, required) {
		return ErrForbidden
	}
	if required != "" && c.BindingID == "" && len(c.Capabilities) > 0 && !hasRequiredCapability(c, action, required) {
		return ErrForbidden
	}
	if !resourceInScope(c, resource) {
		return ErrForbidden
	}
	return nil
}

// memory.l3.publish was used by the first setup-flow migrations. Keep it as
// a read-time alias while exposing the stable memory.publish action to the
// role matrix and menu API.
func hasRequiredCapability(c AccessContext, action Action, required string) bool {
	if c.HasCapability(required) {
		return true
	}
	return action == ActionPublish && c.HasCapability("memory.l3.publish")
}
func (s *Service) Can(ctx context.Context, action Action, resource Resource) bool {
	return s.Authorize(ctx, action, resource) == nil
}
func (s *Service) GetEffectiveCapabilities(ctx context.Context) []string {
	c, err := FromContext(ctx)
	if err != nil {
		return nil
	}
	return append([]string(nil), c.Capabilities...)
}
func (s *Service) GetMenuPermissions(ctx context.Context) MenuPermissionSet {
	c, err := FromContext(ctx)
	if err != nil {
		tenant, _ := types.TenantIDFromContext(ctx)
		c = AccessContext{TenantID: tenant, TenantRole: types.TenantRoleFromContext(ctx)}
	}
	admin := c.TenantRole == types.TenantRoleOwner || c.TenantRole == types.TenantRoleAdmin
	// Review/publish are tenant-governance actions. A capability on an
	// external binding is never allowed to elevate a Contributor or Viewer
	// into the governance menu; the service-level role floor remains the
	// final enforcement point as well.
	review := admin && (c.BindingID == "" || c.HasCapability(CapabilityMemoryReview))
	return MenuPermissionSet{
		Menus:      []MenuPermission{{Key: "knowledge_base", Visible: true, Actions: []string{"read", "search"}}, {Key: "external_memory", Visible: admin || review, Actions: menuActions(admin, review)}},
		RoleMatrix: roleCapabilityMatrix(),
	}
}
func menuActions(admin, review bool) []string {
	v := []string{"overview"}
	if admin {
		v = append(v, "binding.read", "binding.create", "binding.rotate", "binding.revoke", "memory.read", "memory.revoke")
	}
	if review {
		v = append(v, "memory.read", "memory.review", "memory.publish")
	}
	return v
}

func requiredCapability(action Action, resourceType string) string {
	switch action {
	case ActionCapture:
		return CapabilityMemoryCapture
	case ActionRecall:
		return CapabilityMemoryRecall
	case ActionConfirm:
		return CapabilityMemoryConfirm
	case ActionContext:
		return CapabilityMemoryContext
	case ActionReview:
		return CapabilityMemoryReview
	case ActionPublish:
		return CapabilityMemoryPublish
	case ActionRevoke:
		return CapabilityMemoryRevoke
	case ActionSearch:
		if resourceType == "knowledge_base" {
			return "knowledge.search"
		}
		return CapabilityMemoryRecall
	case ActionRead:
		if resourceType == "memory_wiki" || resourceType == "wiki_page" {
			return CapabilityWikiGet
		}
		return CapabilityMemoryContext
	default:
		return ""
	}
}

func roleAllowsAction(role types.TenantRole, action Action, resourceType string) bool {
	if role == types.TenantRoleOwner || role == types.TenantRoleAdmin {
		return true
	}
	switch action {
	case ActionCapture, ActionConfirm:
		return role == types.TenantRoleContributor
	case ActionRecall:
		// Viewer recall is opt-in by tenant policy. Until that policy is
		// explicitly enabled, the default role floor is Contributor.
		return role == types.TenantRoleContributor
	case ActionRead, ActionSearch:
		return role == types.TenantRoleContributor || role == types.TenantRoleViewer
	case ActionReview, ActionPublish, ActionRevoke, ActionManage:
		return false
	default:
		return role == types.TenantRoleViewer || role == types.TenantRoleContributor
	}
}

func resourceInScope(c AccessContext, resource Resource) bool {
	if resource.TeamID != "" && !c.HasTeam(resource.TeamID) {
		return false
	}
	if len(c.AssetScopes) == 0 {
		return true
	}
	if resource.Type == "memory" && resource.TeamID != "" && c.HasAssetScope("team:"+resource.TeamID) {
		return true
	}
	if resource.ID != "" {
		kind := resource.Type
		if kind == "memory_wiki" {
			kind = "wiki_page"
		}
		if c.HasAssetScope(kind + ":" + resource.ID) {
			return true
		}
	}
	return false
}

func roleCapabilityMatrix() map[string][]string {
	return map[string][]string{
		"owner":       {CapabilityMemoryContext, CapabilityMemoryCapture, CapabilityMemoryRecall, CapabilityMemoryConfirm, CapabilityMemoryReview, CapabilityMemoryPublish, CapabilityMemoryRevoke, CapabilityWikiGet},
		"admin":       {CapabilityMemoryContext, CapabilityMemoryCapture, CapabilityMemoryRecall, CapabilityMemoryConfirm, CapabilityMemoryReview, CapabilityMemoryPublish, CapabilityMemoryRevoke, CapabilityWikiGet},
		"contributor": {CapabilityMemoryContext, CapabilityMemoryCapture, CapabilityMemoryRecall, CapabilityMemoryConfirm, CapabilityWikiGet},
		"viewer":      {CapabilityMemoryContext, CapabilityWikiGet},
	}
}
