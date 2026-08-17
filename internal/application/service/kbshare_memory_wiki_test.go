package service

import (
	"context"
	"testing"

	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type memoryWikiShareKBRepo struct {
	interfaces.KnowledgeBaseRepository
	kb *types.KnowledgeBase
}

func (r *memoryWikiShareKBRepo) GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error) {
	return r.kb, nil
}

type memoryWikiShareRepo struct {
	interfaces.KBShareRepository
	shares []*types.KnowledgeBaseShare
}

func (r *memoryWikiShareRepo) ListByKnowledgeBase(context.Context, string) ([]*types.KnowledgeBaseShare, error) {
	return r.shares, nil
}

func (r *memoryWikiShareRepo) ListSharedKBsForTenant(context.Context, uint64) ([]*types.KnowledgeBaseShare, error) {
	return r.shares, nil
}

type memoryWikiShareOrgRepo struct {
	interfaces.OrganizationRepository
}

func (r *memoryWikiShareOrgRepo) GetTenantMember(context.Context, string, uint64) (*types.OrganizationTenantMember, error) {
	return &types.OrganizationTenantMember{Role: types.OrgRoleAdmin}, nil
}

func dedicatedMemoryWikiForShareTest() *types.KnowledgeBase {
	return &types.KnowledgeBase{
		ID: "memory-kb", TenantID: 7, Type: types.KnowledgeBaseTypeWiki,
		IsMemoryWiki: true, MemoryTeamID: "team-a",
		WikiConfig:       &types.WikiConfig{IsMemoryWiki: true, MemoryTeamID: "team-a"},
		IndexingStrategy: types.IndexingStrategy{WikiEnabled: true},
	}
}

func TestKBShareRejectsDedicatedMemoryWiki(t *testing.T) {
	s := &kbShareService{kbRepo: &memoryWikiShareKBRepo{kb: dedicatedMemoryWikiForShareTest()}}
	_, err := s.ShareKnowledgeBase(context.Background(), "memory-kb", "org-1", "user-1", 7, types.OrgRoleViewer)
	require.ErrorIs(t, err, ErrDedicatedMemoryWikiManagedInternally)
}

func TestKBShareHistoricalMemoryWikiCannotResolvePermissionOrList(t *testing.T) {
	kb := dedicatedMemoryWikiForShareTest()
	share := &types.KnowledgeBaseShare{
		ID: "share-1", KnowledgeBaseID: kb.ID, OrganizationID: "org-1",
		SourceTenantID: kb.TenantID, Permission: types.OrgRoleAdmin, KnowledgeBase: kb,
	}
	s := &kbShareService{
		kbRepo:    &memoryWikiShareKBRepo{kb: kb},
		shareRepo: &memoryWikiShareRepo{shares: []*types.KnowledgeBaseShare{share}},
		orgRepo:   &memoryWikiShareOrgRepo{},
	}

	role, shared, err := s.CheckTenantKBPermission(context.Background(), kb.ID, 99, types.TenantRoleAdmin)
	require.NoError(t, err)
	require.False(t, shared)
	require.Empty(t, role)

	listed, err := s.ListSharedKnowledgeBases(context.Background(), 99, types.TenantRoleAdmin)
	require.NoError(t, err)
	require.Empty(t, listed)
}
