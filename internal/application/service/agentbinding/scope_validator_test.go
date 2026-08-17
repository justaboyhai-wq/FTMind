package agentbinding

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newScopeValidatorSQLMock(t *testing.T) (*databaseScopeValidator, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, PreferSimpleProtocol: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &databaseScopeValidator{db: db}, mock
}

func TestBindingScopeValidatorRejectsMissingOrCrossTenantPersistedAssets(t *testing.T) {
	tests := []struct {
		name      string
		kind      string
		id        string
		tableName string
	}{
		{name: "missing knowledge base", kind: "knowledge_base", id: "kb-missing", tableName: "knowledge_bases"},
		{name: "cross tenant knowledge base", kind: "knowledge_base", id: "kb-other-tenant", tableName: "knowledge_bases"},
		{name: "missing wiki page", kind: "wiki_page", id: "page-missing", tableName: "wiki_pages"},
		{name: "cross tenant wiki page", kind: "wiki_page", id: "page-other-tenant", tableName: "wiki_pages"},
		{name: "missing document", kind: "document", id: "document-missing", tableName: "knowledges"},
		{name: "cross tenant document", kind: "document", id: "document-other-tenant", tableName: "knowledges"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			validator, mock := newScopeValidatorSQLMock(t)
			binding := authoritativeBindingScope()
			binding.AssetScopes = append(binding.AssetScopes, tc.kind+":"+tc.id)
			mock.ExpectQuery(`SELECT .* FROM "`+tc.tableName+`" WHERE .*tenant_id = \$1 AND id IN \(\$2\).*deleted_at`).
				WithArgs(uint64(42), tc.id).
				WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id"}))

			_, err := validator.ResolveRoles(context.Background(), binding)
			if err == nil || !strings.Contains(err.Error(), tc.kind+" asset") {
				t.Fatalf("expected authoritative %s asset rejection, got %v", tc.kind, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBindingScopeValidatorRejectsWikiPageAndDocumentKnowledgeBaseHierarchyConflict(t *testing.T) {
	tests := []struct {
		name      string
		kind      string
		id        string
		tableName string
	}{
		{name: "wiki page", kind: "wiki_page", id: "page-1", tableName: "wiki_pages"},
		{name: "document", kind: "document", id: "document-1", tableName: "knowledges"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			validator, mock := newScopeValidatorSQLMock(t)
			binding := authoritativeBindingScope()
			binding.AssetScopes = append(binding.AssetScopes, "knowledge_base:kb-allowed", tc.kind+":"+tc.id)
			mock.ExpectQuery(`SELECT .* FROM "`+tc.tableName+`" WHERE .*tenant_id = \$1 AND id IN \(\$2\).*deleted_at`).
				WithArgs(uint64(42), tc.id).
				WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "knowledge_base_id"}).
					AddRow(tc.id, 42, "kb-conflicting"))

			_, err := validator.ResolveRoles(context.Background(), binding)
			if err == nil || !strings.Contains(err.Error(), "knowledge_base hierarchy") {
				t.Fatalf("expected %s hierarchy rejection, got %v", tc.kind, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBindingScopeValidatorBatchesPersistedAssetsAndAcceptsValidHierarchy(t *testing.T) {
	validator, mock := newScopeValidatorSQLMock(t)
	binding := authoritativeBindingScope()
	binding.AssetScopes = append(binding.AssetScopes,
		"knowledge_base:kb-1", "knowledge_base:kb-2",
		"wiki_page:page-1", "wiki_page:page-2",
		"document:doc-1", "document:doc-2",
	)
	mock.ExpectQuery(`SELECT .* FROM "wiki_pages" WHERE .*tenant_id = \$1 AND id IN \(\$2,\$3\).*deleted_at`).
		WithArgs(uint64(42), "page-1", "page-2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "knowledge_base_id"}).
			AddRow("page-1", 42, "kb-1").AddRow("page-2", 42, "kb-2"))
	mock.ExpectQuery(`SELECT .* FROM "knowledges" WHERE .*tenant_id = \$1 AND id IN \(\$2,\$3\).*deleted_at`).
		WithArgs(uint64(42), "doc-1", "doc-2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "knowledge_base_id"}).
			AddRow("doc-1", 42, "kb-1").AddRow("doc-2", 42, "kb-2"))
	mock.ExpectQuery(`SELECT .* FROM "knowledge_bases" WHERE .*tenant_id = \$1 AND id IN \(\$2,\$3\).*deleted_at`).
		WithArgs(uint64(42), "kb-1", "kb-2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id"}).
			AddRow("kb-1", 42).AddRow("kb-2", 42))
	mock.ExpectQuery(`SELECT .* FROM "tenant_members" WHERE .*user_id = \$1 AND tenant_id = \$2 AND status = \$3.*LIMIT \$4`).
		WithArgs("user-1", uint64(42), types.TenantMemberStatusActive, 1).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "tenant_id", "role", "status"}).AddRow("user-1", 42, "admin", "active"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "custom_agents" WHERE .*id = \$1 AND tenant_id = \$2`).
		WithArgs("agent-1", uint64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	roles, err := validator.ResolveRoles(context.Background(), binding)
	if err != nil {
		t.Fatal(err)
	}
	if !stringArraysEqual(roles, types.StringArray{"tenant:admin"}) {
		t.Fatalf("unexpected roles: %#v", roles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func authoritativeBindingScope() *types.AgentBinding {
	return &types.AgentBinding{TenantID: 42, TeamID: "team-1", UserID: "user-1", AgentID: "agent-1", AssetScopes: types.StringArray{"tenant:42", "team:team-1"}}
}

func TestBindingScopeValidatorRejectsTenantAndOptionalNamespaceMismatchBeforeQuerying(t *testing.T) {
	validator, mock := newScopeValidatorSQLMock(t)
	for name, mutate := range map[string]func(*types.AgentBinding){
		"wrong tenant": func(binding *types.AgentBinding) {
			binding.AssetScopes[0] = "tenant:43"
		},
		"asset for absent optional workspace": func(binding *types.AgentBinding) {
			binding.AssetScopes = append(binding.AssetScopes, "workspace:workspace-1")
		},
	} {
		t.Run(name, func(t *testing.T) {
			binding := authoritativeBindingScope()
			mutate(binding)
			_, err := validator.ResolveRoles(context.Background(), binding)
			if err == nil || !strings.Contains(err.Error(), "conflicts with binding namespace") {
				t.Fatalf("expected namespace conflict, got %v", err)
			}
		})
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBindingScopeValidatorRejectsInvalidOrConflictingManagedNamespacesWithoutQuerying(t *testing.T) {
	validator, mock := newScopeValidatorSQLMock(t)
	binding := authoritativeBindingScope()
	binding.ProjectID = "invalid/project"
	if _, err := validator.ResolveRoles(context.Background(), binding); err == nil {
		t.Fatal("accepted an invalid project namespace")
	}
	binding.ProjectID = "project-1"
	binding.AssetScopes = append(binding.AssetScopes, "project:another-project")
	if _, err := validator.ResolveRoles(context.Background(), binding); err == nil {
		t.Fatal("accepted a project namespace that conflicts with asset_scopes")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBindingScopeValidatorAlwaysScopesUserLookupByTenant(t *testing.T) {
	validator, mock := newScopeValidatorSQLMock(t)
	mock.ExpectQuery(`SELECT .* FROM "tenant_members" WHERE .*user_id = \$1 AND tenant_id = \$2 AND status = \$3.*LIMIT \$4`).
		WithArgs("user-1", uint64(42), types.TenantMemberStatusActive, 1).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "tenant_id", "role", "status"}))
	if _, err := validator.ResolveRoles(context.Background(), authoritativeBindingScope()); err == nil {
		t.Fatal("accepted a user without membership in the binding tenant")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBindingScopeValidatorComputesRolesFromAuthoritativeRows(t *testing.T) {
	validator, mock := newScopeValidatorSQLMock(t)
	mock.ExpectQuery(`SELECT .* FROM "tenant_members" WHERE .*user_id = \$1 AND tenant_id = \$2 AND status = \$3.*LIMIT \$4`).
		WithArgs("user-1", uint64(42), types.TenantMemberStatusActive, 1).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "tenant_id", "role", "status"}).AddRow("user-1", 42, "admin", "active"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "custom_agents" WHERE .*id = \$1 AND tenant_id = \$2`).
		WithArgs("agent-1", uint64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	roles, err := validator.ResolveRoles(context.Background(), authoritativeBindingScope())
	if err != nil {
		t.Fatal(err)
	}
	want := types.StringArray{"tenant:admin"}
	if !stringArraysEqual(roles, want) {
		t.Fatalf("roles=%#v want=%#v", roles, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBindingScopeValidatorCreateRequiresAdmin(t *testing.T) {
	validator, mock := newScopeValidatorSQLMock(t)
	viewerContext := context.WithValue(context.Background(), types.TenantRoleContextKey, types.TenantRoleViewer)
	if _, err := validator.ValidateCreate(viewerContext, authoritativeBindingScope()); err == nil {
		t.Fatal("viewer created a managed binding namespace")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
