package service

import (
	"context"
	"errors"
	"testing"

	"github.com/justaboyhai-wq/fmind/internal/application/repository"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

type memoryWikiGuardKBStub struct{ kb *types.KnowledgeBase }

type wikiAtomicUpdateRepoStub struct {
	interfaces.WikiPageRepository
	existing        *types.WikiPage
	updateCalls     int
	updateMetaCalls int
}

type wikiIssueRepoStub struct {
	interfaces.WikiPageRepository
	issue        *types.WikiPageIssue
	updateCalls  int
	updateTenant uint64
	updateKB     string
}

func (r *wikiIssueRepoStub) GetIssueByID(context.Context, string) (*types.WikiPageIssue, error) {
	copy := *r.issue
	return &copy, nil
}

func (r *wikiIssueRepoStub) UpdateIssueStatus(_ context.Context, tenantID uint64, kbID, _ string, _ string) error {
	r.updateCalls++
	r.updateTenant = tenantID
	r.updateKB = kbID
	return nil
}

func (r *wikiAtomicUpdateRepoStub) GetBySlug(context.Context, string, string) (*types.WikiPage, error) {
	copy := *r.existing
	return &copy, nil
}

func (r *wikiAtomicUpdateRepoStub) Update(_ context.Context, page *types.WikiPage) error {
	r.updateCalls++
	page.Version++
	return nil
}

func (r *wikiAtomicUpdateRepoStub) UpdateMeta(context.Context, *types.WikiPage) error {
	r.updateMetaCalls++
	return nil
}

func (s memoryWikiGuardKBStub) GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error) {
	return s.kb, nil
}

func (s memoryWikiGuardKBStub) GetKnowledgeBaseByIDOnly(context.Context, string) (*types.KnowledgeBase, error) {
	return s.kb, nil
}

func TestMemoryWikiMutationGuardRejectsGenericWritesAndInvalidInvariant(t *testing.T) {
	memoryKB := &types.KnowledgeBase{
		ID: "memory-kb", TenantID: 7, Type: types.KnowledgeBaseTypeWiki,
		IsMemoryWiki: true, MemoryTeamID: "team-a",
		WikiConfig:       &types.WikiConfig{IsMemoryWiki: true, MemoryTeamID: "team-a"},
		IndexingStrategy: types.IndexingStrategy{WikiEnabled: true},
	}
	service := &wikiPageService{kbService: memoryWikiGuardKBStub{kb: memoryKB}}
	if err := service.requireMemoryWikiMutationAuthority(context.Background(), memoryKB.ID); !errors.Is(err, ErrDedicatedMemoryWikiManagedInternally) {
		t.Fatalf("generic write error=%v", err)
	}
	if err := service.requireMemoryWikiMutationAuthority(types.WithMemoryWikiMutation(context.Background()), memoryKB.ID); err != nil {
		t.Fatalf("trusted publisher error=%v", err)
	}

	invalid := *memoryKB
	invalid.IndexingStrategy.VectorEnabled = true
	service.kbService = memoryWikiGuardKBStub{kb: &invalid}
	if err := service.requireMemoryWikiMutationAuthority(types.WithMemoryWikiMutation(context.Background()), invalid.ID); !errors.Is(err, ErrDedicatedMemoryWikiManagedInternally) {
		t.Fatalf("invalid zero-RAG invariant error=%v", err)
	}
}

func TestMemoryWikiReadGuardRequiresAdminOrVerifiedExactBindingScope(t *testing.T) {
	memoryKB := &types.KnowledgeBase{
		ID: "memory-kb", TenantID: 7, Type: types.KnowledgeBaseTypeWiki,
		IsMemoryWiki: true, MemoryTeamID: "team-a",
		WikiConfig:       &types.WikiConfig{IsMemoryWiki: true, MemoryTeamID: "team-a"},
		IndexingStrategy: types.IndexingStrategy{WikiEnabled: true},
	}
	service := &wikiPageService{kbService: memoryWikiGuardKBStub{kb: memoryKB}}
	if err := service.requireMemoryWikiReadAuthority(context.Background(), memoryKB.ID, "page-a"); !errors.Is(err, ErrDedicatedMemoryWikiManagedInternally) {
		t.Fatalf("generic read error=%v", err)
	}

	admin := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	admin = context.WithValue(admin, types.TenantRoleContextKey, types.TenantRoleAdmin)
	if err := service.requireMemoryWikiReadAuthority(admin, memoryKB.ID, "page-a"); err != nil {
		t.Fatalf("tenant admin read error=%v", err)
	}

	binding := types.BindingContext{
		BindingID: "binding-a", TenantID: 7, TeamID: "team-a",
		AssetScopes: types.StringArray{"wiki_page:page-a"},
	}
	bound := types.WithVerifiedBindingContext(context.Background(), binding)
	if err := service.requireMemoryWikiReadAuthority(bound, memoryKB.ID, "page-a"); err != nil {
		t.Fatalf("verified exact binding read error=%v", err)
	}
	if err := service.requireMemoryWikiReadAuthority(bound, memoryKB.ID, "page-b"); !errors.Is(err, ErrDedicatedMemoryWikiManagedInternally) {
		t.Fatalf("out-of-scope binding read error=%v", err)
	}
}

func TestValidateWikiPageExpectedVersionRejectsStalePublisher(t *testing.T) {
	existing := &types.WikiPage{ID: "page-1", Version: 3}
	if err := validateWikiPageExpectedVersion(existing, &types.WikiPage{ID: "page-1", Version: 2}); !errors.Is(err, repository.ErrWikiPageConflict) {
		t.Fatalf("stale expected version error=%v", err)
	}
	if err := validateWikiPageExpectedVersion(existing, &types.WikiPage{ID: "page-1", Version: 3}); err != nil {
		t.Fatalf("current expected version error=%v", err)
	}
	if err := validateWikiPageExpectedVersion(existing, &types.WikiPage{ID: "page-1"}); err != nil {
		t.Fatalf("legacy unspecified expected version error=%v", err)
	}
}

func TestWikiPageContentUpdateUsesSingleAtomicRepositoryWrite(t *testing.T) {
	existing := &types.WikiPage{
		ID: "page-1", KnowledgeBaseID: "kb-1", Slug: "page", Title: "v1",
		Content: "old", Summary: "old", PageType: types.WikiPageTypeConcept,
		Status: types.WikiPageStatusPublished, Version: 1,
	}
	repo := &wikiAtomicUpdateRepoStub{existing: existing}
	normalKB := &types.KnowledgeBase{ID: "kb-1", TenantID: 7, Type: types.KnowledgeBaseTypeWiki}
	service := &wikiPageService{repo: repo, kbService: memoryWikiGuardKBStub{kb: normalKB}}
	requested := *existing
	requested.Title = "v2"
	requested.Content = "new"
	_, err := service.UpdatePage(context.Background(), &requested)
	if err != nil {
		t.Fatalf("UpdatePage error=%v", err)
	}
	if repo.updateCalls != 1 || repo.updateMetaCalls != 0 {
		t.Fatalf("repository writes update=%d updateMeta=%d, want one atomic update", repo.updateCalls, repo.updateMetaCalls)
	}
}

func TestWikiIssueStatusUsesAuthoritativeTenantAndKnowledgeBase(t *testing.T) {
	memoryKB := &types.KnowledgeBase{
		ID: "memory-kb", TenantID: 8, Type: types.KnowledgeBaseTypeWiki,
		IsMemoryWiki: true, MemoryTeamID: "team-b",
		WikiConfig:       &types.WikiConfig{IsMemoryWiki: true, MemoryTeamID: "team-b"},
		IndexingStrategy: types.IndexingStrategy{WikiEnabled: true},
	}
	repo := &wikiIssueRepoStub{issue: &types.WikiPageIssue{
		ID: "issue-secret", TenantID: 8, KnowledgeBaseID: memoryKB.ID, Status: "pending",
	}}
	service := &wikiPageService{repo: repo, kbService: memoryWikiGuardKBStub{kb: memoryKB}}

	err := service.UpdateIssueStatus(context.Background(), "attacker-normal-kb", repo.issue.ID, "resolved")
	if !errors.Is(err, repository.ErrWikiPageIssueNotFound) || repo.updateCalls != 0 {
		t.Fatalf("cross-KB update error=%v calls=%d", err, repo.updateCalls)
	}
	err = service.UpdateIssueStatus(context.Background(), memoryKB.ID, repo.issue.ID, "resolved")
	if !errors.Is(err, ErrDedicatedMemoryWikiManagedInternally) || repo.updateCalls != 0 {
		t.Fatalf("generic memory Wiki update error=%v calls=%d", err, repo.updateCalls)
	}
	err = service.UpdateIssueStatus(types.WithMemoryWikiMutation(context.Background()), memoryKB.ID, repo.issue.ID, "resolved")
	if err != nil || repo.updateCalls != 1 || repo.updateTenant != 8 || repo.updateKB != memoryKB.ID {
		t.Fatalf("trusted update error=%v calls=%d tenant=%d kb=%s", err, repo.updateCalls, repo.updateTenant, repo.updateKB)
	}
}

func TestStripWikiInlineChunkCitations(t *testing.T) {
	input := "[**橡皮障夹**](#)**钳**\n\n夹钳是用于夹持橡皮障夹的专用器械[c003]。手柄便于操作 [c003]。多个来源[c003, c1000]。"
	want := "[**橡皮障夹**](#)**钳**\n\n夹钳是用于夹持橡皮障夹的专用器械。手柄便于操作。多个来源。"

	if got := stripWikiInlineChunkCitations(input); got != want {
		t.Fatalf("stripWikiInlineChunkCitations() = %q, want %q", got, want)
	}
}

func TestStripWikiInlineChunkCitationsPreservesOrdinaryMarkdown(t *testing.T) {
	input := "保留 [citation]、[C003]、[c12] 和 [[concept/c003|页面链接]]。"
	if got := stripWikiInlineChunkCitations(input); got != input {
		t.Fatalf("stripWikiInlineChunkCitations() changed ordinary Markdown: %q", got)
	}
}

func TestParseOutLinks(t *testing.T) {
	svc := &wikiPageService{}

	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "single link",
			content: "See [[entity/acme-corp]] for details.",
			want:    []string{"entity/acme-corp"},
		},
		{
			name:    "multiple links",
			content: "See [[entity/acme-corp]] and [[concept/rag]] for details.",
			want:    []string{"entity/acme-corp", "concept/rag"},
		},
		{
			name:    "duplicate links deduplicated",
			content: "See [[entity/acme-corp]] and also [[entity/acme-corp]] again.",
			want:    []string{"entity/acme-corp"},
		},
		{
			name:    "pipe syntax: slug|display name",
			content: "See [[entity/acme-corp|Acme Corp]] for details.",
			want:    []string{"entity/acme-corp"},
		},
		{
			name:    "mixed: pipe and bare links",
			content: "See [[entity/acme-corp|Acme Corp]] and [[concept/rag]] here.",
			want:    []string{"entity/acme-corp", "concept/rag"},
		},
		{
			name:    "no links",
			content: "Just plain text without any links.",
			want:    nil,
		},
		{
			name:    "empty content",
			content: "",
			want:    nil,
		},
		{
			name:    "link with spaces normalized",
			content: "See [[Entity/Acme Corp]] for details.",
			want:    []string{"entity/acme-corp"},
		},
		{
			name:    "nested brackets ignored",
			content: "Not a link: [not [a] link]",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.parseOutLinks(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("parseOutLinks() = %v (len %d), want %v (len %d)", got, len(got), tt.want, len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseOutLinks()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestNormalizeSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Entity/Acme Corp", "entity/acme-corp"},
		{"  hello  ", "hello"},
		{"UPPER-CASE", "upper-case"},
		{"already-ok", "already-ok"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeSlug(tt.input)
			if got != tt.want {
				t.Errorf("normalizeSlug(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeWikiHierarchyCleansModelCategoryNoise(t *testing.T) {
	page := &types.WikiPage{
		Slug:         "concept/ai-knowledge",
		Title:        "AI时代的知识困境",
		PageType:     types.WikiPageTypeConcept,
		CategoryPath: types.StringArray{"概念/爱护花草", " 实体/标牌 ", "摘要", "生态/平台", "多余层级"},
	}

	normalizeWikiHierarchy(page)

	want := types.StringArray{"爱护花草", "标牌", "生态"}
	if len(page.CategoryPath) != len(want) {
		t.Fatalf("CategoryPath = %v, want %v", page.CategoryPath, want)
	}
	for i := range want {
		if page.CategoryPath[i] != want[i] {
			t.Fatalf("CategoryPath[%d] = %q, want %q; full path=%v", i, page.CategoryPath[i], want[i], page.CategoryPath)
		}
	}
	if page.Depth != 3 {
		t.Fatalf("Depth = %d, want 3", page.Depth)
	}
	if page.WikiPath != "concept/爱护花草/标牌/生态/AI时代的知识困境" {
		t.Fatalf("WikiPath = %q", page.WikiPath)
	}
}

func TestNormalizeWikiIndexEntryHierarchyCleansModelCategoryNoise(t *testing.T) {
	entry := &types.WikiIndexEntry{
		Slug:         "entity/sign",
		Title:        "爱护花草标牌",
		CategoryPath: types.StringArray{"实体/标牌", "概念", "生态/行为倡导"},
	}

	normalizeWikiIndexEntryHierarchy(entry, types.WikiPageTypeEntity)

	want := types.StringArray{"标牌", "生态", "行为倡导"}
	if len(entry.CategoryPath) != len(want) {
		t.Fatalf("CategoryPath = %v, want %v", entry.CategoryPath, want)
	}
	for i := range want {
		if entry.CategoryPath[i] != want[i] {
			t.Fatalf("CategoryPath[%d] = %q, want %q; full path=%v", i, entry.CategoryPath[i], want[i], entry.CategoryPath)
		}
	}
	if entry.WikiPath != "entity/标牌/生态/行为倡导/爱护花草标牌" {
		t.Fatalf("WikiPath = %q", entry.WikiPath)
	}
}

func TestContainsString(t *testing.T) {
	slice := []string{"a", "b", "c"}
	if !containsString(slice, "b") {
		t.Error("should contain 'b'")
	}
	if containsString(slice, "d") {
		t.Error("should not contain 'd'")
	}
	if containsString(nil, "a") {
		t.Error("nil slice should not contain anything")
	}
}

func TestRemoveString(t *testing.T) {
	slice := types.StringArray{"a", "b", "c", "b"}
	result := removeString(slice, "b")
	if len(result) != 2 {
		t.Errorf("Expected 2 items after removing 'b', got %d: %v", len(result), result)
	}
	if result[0] != "a" || result[1] != "c" {
		t.Errorf("Unexpected result: %v", result)
	}

	// Remove non-existing
	result2 := removeString(slice, "z")
	if len(result2) != 4 {
		t.Errorf("Expected 4 items (nothing removed), got %d", len(result2))
	}
}

// makeGraphFixture builds a small synthetic wiki for GetGraph tests.
//
// Edges (directed):
//
//	hub -> a, hub -> b, hub -> c, hub -> d
//	a   -> hub
//	b   -> hub
//	c   -> d
//	x   -> y   (isolated 2-node cluster, disconnected from hub)
//
// Page types are chosen so type-filter tests can exclude specific nodes.
func makeGraphFixture() []*types.WikiPage {
	return []*types.WikiPage{
		{
			Slug:     "hub",
			Title:    "Hub",
			PageType: types.WikiPageTypeSummary,
			OutLinks: types.StringArray{"a", "b", "c", "d"},
			InLinks:  types.StringArray{"a", "b"},
		},
		{
			Slug:     "a",
			Title:    "A",
			PageType: types.WikiPageTypeEntity,
			OutLinks: types.StringArray{"hub"},
			InLinks:  types.StringArray{"hub"},
		},
		{
			Slug:     "b",
			Title:    "B",
			PageType: types.WikiPageTypeEntity,
			OutLinks: types.StringArray{"hub"},
			InLinks:  types.StringArray{"hub"},
		},
		{
			Slug:     "c",
			Title:    "C",
			PageType: types.WikiPageTypeConcept,
			OutLinks: types.StringArray{"d"},
			InLinks:  types.StringArray{"hub"},
		},
		{
			Slug:     "d",
			Title:    "D",
			PageType: types.WikiPageTypeConcept,
			OutLinks: types.StringArray{},
			InLinks:  types.StringArray{"hub", "c"},
		},
		{
			Slug:     "x",
			Title:    "X",
			PageType: types.WikiPageTypeEntity,
			OutLinks: types.StringArray{"y"},
			InLinks:  types.StringArray{},
		},
		{
			Slug:     "y",
			Title:    "Y",
			PageType: types.WikiPageTypeEntity,
			OutLinks: types.StringArray{},
			InLinks:  types.StringArray{"x"},
		},
	}
}

func nodeSlugs(data *types.WikiGraphData) map[string]bool {
	out := make(map[string]bool, len(data.Nodes))
	for _, n := range data.Nodes {
		out[n.Slug] = true
	}
	return out
}

// TestComputeGraphSubset_OverviewTruncatesByLinkCount verifies that overview
// mode returns the most-connected nodes first and reports truncation
// honestly in Meta. At 4万 pages this is the path that must NOT return the
// full graph — the cap is what keeps the response size and frontend
// rendering tractable.
func TestComputeGraphSubset_OverviewTruncatesByLinkCount(t *testing.T) {
	pages := makeGraphFixture()
	got, err := computeGraphSubset(pages, &types.WikiGraphRequest{
		Mode:  types.WikiGraphModeOverview,
		Limit: 3,
	})
	if err != nil {
		t.Fatalf("computeGraphSubset: %v", err)
	}

	if len(got.Nodes) != 3 {
		t.Errorf("want 3 nodes, got %d (%v)", len(got.Nodes), nodeSlugs(got))
	}
	slugs := nodeSlugs(got)
	// hub has link_count 6 (4 out + 2 in), must survive the cap.
	if !slugs["hub"] {
		t.Errorf("expected hub to survive the top-3 cap, got %v", slugs)
	}
	if !got.Meta.Truncated {
		t.Errorf("expected Meta.Truncated=true when returned < total")
	}
	if got.Meta.Total != len(pages) {
		t.Errorf("Meta.Total = %d, want %d", got.Meta.Total, len(pages))
	}
	if got.Meta.Returned != len(got.Nodes) {
		t.Errorf("Meta.Returned mismatch: %d vs %d", got.Meta.Returned, len(got.Nodes))
	}

	// Every returned edge must connect two surviving nodes.
	for _, e := range got.Edges {
		if !slugs[e.Source] || !slugs[e.Target] {
			t.Errorf("edge %s->%s references a non-returned node", e.Source, e.Target)
		}
	}
}

// TestComputeGraphSubset_OverviewUncapped ensures the Limit<=0 escape hatch
// still works for internal callers (wiki lint) that need every page.
func TestComputeGraphSubset_OverviewUncapped(t *testing.T) {
	pages := makeGraphFixture()
	got, err := computeGraphSubset(pages, &types.WikiGraphRequest{
		Mode:  types.WikiGraphModeOverview,
		Limit: 0,
	})
	if err != nil {
		t.Fatalf("computeGraphSubset: %v", err)
	}
	if len(got.Nodes) != len(pages) {
		t.Errorf("want %d nodes (uncapped), got %d", len(pages), len(got.Nodes))
	}
	if got.Meta.Truncated {
		t.Errorf("expected Meta.Truncated=false when nothing was dropped")
	}
}

// TestComputeGraphSubset_OverviewTypeFilter ensures the type filter applies
// to the candidate set (not just post-hoc), so "total" reflects the
// filter-aware denominator the frontend shows to the user.
func TestComputeGraphSubset_OverviewTypeFilter(t *testing.T) {
	pages := makeGraphFixture()
	got, err := computeGraphSubset(pages, &types.WikiGraphRequest{
		Mode:  types.WikiGraphModeOverview,
		Types: []string{types.WikiPageTypeEntity},
		Limit: 100,
	})
	if err != nil {
		t.Fatalf("computeGraphSubset: %v", err)
	}
	slugs := nodeSlugs(got)
	// Only entity-typed pages: a, b, x, y.
	for _, expected := range []string{"a", "b", "x", "y"} {
		if !slugs[expected] {
			t.Errorf("expected entity %q to be returned, got %v", expected, slugs)
		}
	}
	for _, forbidden := range []string{"hub", "c", "d"} {
		if slugs[forbidden] {
			t.Errorf("%q should have been filtered out by type, got %v", forbidden, slugs)
		}
	}
	if got.Meta.Total != 4 {
		t.Errorf("Meta.Total should equal entity-typed page count (4), got %d", got.Meta.Total)
	}
}

// TestComputeGraphSubset_EgoDepth1 checks that depth=1 returns center +
// immediate neighbors only, with no transitive hops.
func TestComputeGraphSubset_EgoDepth1(t *testing.T) {
	pages := makeGraphFixture()
	got, err := computeGraphSubset(pages, &types.WikiGraphRequest{
		Mode:   types.WikiGraphModeEgo,
		Center: "hub",
		Depth:  1,
		Limit:  100,
	})
	if err != nil {
		t.Fatalf("computeGraphSubset: %v", err)
	}
	slugs := nodeSlugs(got)
	// hub's direct neighbors: a, b, c, d (out) ∪ a, b (in) = {a, b, c, d} + hub.
	want := []string{"hub", "a", "b", "c", "d"}
	for _, s := range want {
		if !slugs[s] {
			t.Errorf("expected %q in ego depth=1 of hub, got %v", s, slugs)
		}
	}
	// x and y are in a disconnected cluster and must not leak in.
	if slugs["x"] || slugs["y"] {
		t.Errorf("disconnected nodes leaked into ego graph: %v", slugs)
	}
	if got.Meta.Mode != types.WikiGraphModeEgo || got.Meta.Center != "hub" || got.Meta.Depth != 1 {
		t.Errorf("Meta not populated correctly for ego: %+v", got.Meta)
	}
}

// TestComputeGraphSubset_EgoDepth2 checks that an extra hop reaches pages
// only accessible through a one-hop neighbor (c -> d becomes reachable from
// "a" at depth 2 because a -> hub -> c -> d ... wait, that's 3 hops. Use
// a clearer case: from "a", depth=2 should reach hub's neighbors.)
func TestComputeGraphSubset_EgoDepth2ExpandsFrontier(t *testing.T) {
	pages := makeGraphFixture()
	depth1, err := computeGraphSubset(pages, &types.WikiGraphRequest{
		Mode:   types.WikiGraphModeEgo,
		Center: "a",
		Depth:  1,
		Limit:  100,
	})
	if err != nil {
		t.Fatalf("depth=1: %v", err)
	}
	depth2, err := computeGraphSubset(pages, &types.WikiGraphRequest{
		Mode:   types.WikiGraphModeEgo,
		Center: "a",
		Depth:  2,
		Limit:  100,
	})
	if err != nil {
		t.Fatalf("depth=2: %v", err)
	}
	if len(depth2.Nodes) <= len(depth1.Nodes) {
		t.Errorf("depth=2 should expand the frontier beyond depth=1 (%d <= %d)",
			len(depth2.Nodes), len(depth1.Nodes))
	}
	// At depth 2 from "a": a -> hub -> {b, c, d}. So b, c, d must appear.
	slugs := nodeSlugs(depth2)
	for _, s := range []string{"a", "hub", "b", "c", "d"} {
		if !slugs[s] {
			t.Errorf("expected %q at depth=2 from a, got %v", s, slugs)
		}
	}
}

// TestComputeGraphSubset_EgoRejectsMissingCenter ensures we fail fast
// rather than returning an empty graph when the caller points at a
// non-existent slug — an empty result here would look identical to "your
// wiki has no links to that page" and hide the real bug.
func TestComputeGraphSubset_EgoRejectsMissingCenter(t *testing.T) {
	pages := makeGraphFixture()
	_, err := computeGraphSubset(pages, &types.WikiGraphRequest{
		Mode:   types.WikiGraphModeEgo,
		Center: "does-not-exist",
		Depth:  1,
		Limit:  100,
	})
	if err == nil {
		t.Fatalf("expected error for missing center slug")
	}
}
