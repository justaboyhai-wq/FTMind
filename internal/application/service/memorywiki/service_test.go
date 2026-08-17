package memorywiki

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/justaboyhai-wq/fmind/internal/application/repository"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type memoryWikiPageFake struct {
	mu                sync.Mutex
	pages             map[string]*types.WikiPage
	createCalls       int
	updateCalls       int
	successfulUpdates int
	metaUpdateCalls   int
	failCreate        int
	failUpdate        int
	forceMisses       int
	beforeUpdate      func(*types.WikiPage)
	updateEntered     chan struct{}
	updateRelease     chan struct{}
}

func newMemoryWikiPageFake() *memoryWikiPageFake {
	return &memoryWikiPageFake{pages: make(map[string]*types.WikiPage)}
}

func (f *memoryWikiPageFake) key(kbID, slug string) string { return kbID + "\x00" + slug }

func (f *memoryWikiPageFake) GetPageBySlug(_ context.Context, kbID, slug string) (*types.WikiPage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forceMisses > 0 {
		f.forceMisses--
		return nil, repository.ErrWikiPageNotFound
	}
	page, ok := f.pages[f.key(kbID, slug)]
	if !ok {
		return nil, repository.ErrWikiPageNotFound
	}
	copy := *page
	return &copy, nil
}

func (f *memoryWikiPageFake) CreatePage(_ context.Context, page *types.WikiPage) (*types.WikiPage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCalls++
	if f.failCreate > 0 {
		f.failCreate--
		return nil, errors.New("wiki create unavailable")
	}
	key := f.key(page.KnowledgeBaseID, page.Slug)
	if _, exists := f.pages[key]; exists {
		return nil, errors.New("duplicate wiki slug")
	}
	copy := *page
	if copy.ID == "" {
		copy.ID = uuid.NewString()
	}
	if copy.Version == 0 {
		copy.Version = 1
	}
	f.pages[key] = &copy
	result := copy
	return &result, nil
}

func (f *memoryWikiPageFake) UpdatePage(ctx context.Context, page *types.WikiPage) (*types.WikiPage, error) {
	if f.updateEntered != nil {
		f.updateEntered <- struct{}{}
		select {
		case <-f.updateRelease:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateCalls++
	if f.beforeUpdate != nil {
		f.beforeUpdate(page)
	}
	if f.failUpdate > 0 {
		f.failUpdate--
		return nil, errors.New("wiki update unavailable")
	}
	key := f.key(page.KnowledgeBaseID, page.Slug)
	existing, exists := f.pages[key]
	if !exists {
		return nil, repository.ErrWikiPageNotFound
	}
	if page.Version != existing.Version {
		return nil, repository.ErrWikiPageConflict
	}
	copy := *page
	copy.ID = existing.ID
	copy.Version = existing.Version + 1
	f.pages[key] = &copy
	f.successfulUpdates++
	result := copy
	return &result, nil
}

func (f *memoryWikiPageFake) UpdatePageMeta(_ context.Context, page *types.WikiPage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := f.key(page.KnowledgeBaseID, page.Slug)
	existing, exists := f.pages[key]
	if !exists || existing.ID != page.ID {
		return repository.ErrWikiPageNotFound
	}
	copy := *page
	copy.Version = existing.Version
	f.pages[key] = &copy
	f.metaUpdateCalls++
	return nil
}

type memoryKnowledgeBaseFake struct {
	mu   sync.Mutex
	byID map[string]*types.KnowledgeBase
}

type blockingWikiGateway struct {
	*memoryWikiPageFake
	publicationRepo interfaces.MemoryWikiPublicationRepository
	entered         chan struct{}
	release         chan struct{}
	once            sync.Once
}

type blockingCreateWikiGateway struct {
	*memoryWikiPageFake
	publicationRepo interfaces.MemoryWikiPublicationRepository
	entered         chan struct{}
	release         chan struct{}
	once            sync.Once
}

func publicationWriteAllowed(ctx context.Context, repo interfaces.MemoryWikiPublicationRepository) error {
	guard, guarded := types.MemoryWikiPublicationGuardFromContext(ctx)
	if !guarded || repo == nil {
		return nil
	}
	publication, err := repo.GetMemoryWikiPublication(ctx, guard.TenantID, guard.PublicationID)
	if err != nil || publication.Status != types.MemoryReviewStatusPublishing {
		return repository.ErrWikiPageConflict
	}
	return nil
}

func (g *blockingWikiGateway) UpdatePage(ctx context.Context, page *types.WikiPage) (*types.WikiPage, error) {
	g.once.Do(func() { close(g.entered) })
	select {
	case <-g.release:
		if err := publicationWriteAllowed(ctx, g.publicationRepo); err != nil {
			return nil, err
		}
		return g.memoryWikiPageFake.UpdatePage(ctx, page)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (g *blockingCreateWikiGateway) CreatePage(ctx context.Context, page *types.WikiPage) (*types.WikiPage, error) {
	g.once.Do(func() { close(g.entered) })
	select {
	case <-g.release:
		if err := publicationWriteAllowed(ctx, g.publicationRepo); err != nil {
			return nil, err
		}
		return g.memoryWikiPageFake.CreatePage(ctx, page)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (f *memoryKnowledgeBaseFake) GetKnowledgeBaseByID(_ context.Context, id string) (*types.KnowledgeBase, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	kb, ok := f.byID[id]
	if !ok {
		return nil, errors.New("knowledge base not found")
	}
	copy := *kb
	return &copy, nil
}

func (f *memoryKnowledgeBaseFake) ListKnowledgeBasesByTenantID(_ context.Context, tenantID uint64) ([]*types.KnowledgeBase, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]*types.KnowledgeBase, 0, len(f.byID))
	for _, kb := range f.byID {
		if kb.TenantID == tenantID {
			copy := *kb
			result = append(result, &copy)
		}
	}
	return result, nil
}

func (f *memoryKnowledgeBaseFake) CreateKnowledgeBase(_ context.Context, kb *types.KnowledgeBase) (*types.KnowledgeBase, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, existing := range f.byID {
		if existing.TenantID == kb.TenantID && existing.IsMemoryWiki && existing.MemoryTeamID == kb.MemoryTeamID {
			return nil, errors.New("duplicate team memory Wiki")
		}
	}
	copy := *kb
	if copy.ID == "" {
		copy.ID = uuid.NewString()
	}
	copy.EnsureDefaults()
	f.byID[copy.ID] = &copy
	result := copy
	return &result, nil
}

type failCompleteMemoryRepository struct {
	interfaces.MemoryWikiPublicationRepository
	failures int
}

type failRevisionMemoryRepository struct {
	interfaces.MemoryWikiPublicationRepository
	failures int
}

func (r *failRevisionMemoryRepository) CreateMemoryWikiRevision(ctx context.Context, revision *types.MemoryWikiRevision) (*types.MemoryWikiRevision, bool, error) {
	if r.failures > 0 {
		r.failures--
		return nil, false, errors.New("revision store unavailable")
	}
	return r.MemoryWikiPublicationRepository.CreateMemoryWikiRevision(ctx, revision)
}

type concurrentReadBarrierRepository struct {
	interfaces.MemoryWikiPublicationRepository
	mu      sync.Mutex
	readers int
	release chan struct{}
}

type casLoserMemoryRepository struct {
	interfaces.MemoryWikiPublicationRepository
	loseStart    bool
	loseComplete bool
}

func (r *casLoserMemoryRepository) StartMemoryWikiPublishing(ctx context.Context, key types.MemoryProjectionKey, publicationID, knowledgeBaseID string) (*types.MemoryWikiPublication, error) {
	publication, err := r.MemoryWikiPublicationRepository.StartMemoryWikiPublishing(ctx, key, publicationID, knowledgeBaseID)
	if err == nil && r.loseStart {
		r.loseStart = false
		return nil, repository.ErrExternalMemoryStateConflict
	}
	return publication, err
}

func (r *casLoserMemoryRepository) CompleteMemoryWikiPublishing(ctx context.Context, key types.MemoryProjectionKey, publicationID string, result types.MemoryWikiPublishResult) (*types.MemoryWikiPublication, error) {
	publication, err := r.MemoryWikiPublicationRepository.CompleteMemoryWikiPublishing(ctx, key, publicationID, result)
	if err == nil && r.loseComplete {
		r.loseComplete = false
		return nil, repository.ErrExternalMemoryStateConflict
	}
	return publication, err
}

func (r *concurrentReadBarrierRepository) GetMemoryWikiPublication(ctx context.Context, tenantID uint64, id string) (*types.MemoryWikiPublication, error) {
	publication, err := r.MemoryWikiPublicationRepository.GetMemoryWikiPublication(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.readers++
	if r.readers == 2 {
		close(r.release)
	}
	r.mu.Unlock()
	select {
	case <-r.release:
		return publication, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *failCompleteMemoryRepository) CompleteMemoryWikiPublishing(ctx context.Context, key types.MemoryProjectionKey, publicationID string, result types.MemoryWikiPublishResult) (*types.MemoryWikiPublication, error) {
	if r.failures > 0 {
		r.failures--
		return nil, errors.New("publication checkpoint unavailable")
	}
	return r.MemoryWikiPublicationRepository.CompleteMemoryWikiPublishing(ctx, key, publicationID, result)
}

func newMemoryWikiServiceTest(t *testing.T) (*Service, interfaces.MemoryWikiPublicationRepository, *memoryWikiPageFake, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&types.MemoryIntegrationEvent{},
		&types.MemoryL3Snapshot{},
		&types.MemoryReviewTask{},
		&types.MemoryReviewHistory{},
		&types.MemoryWikiPublication{},
		&types.MemoryWikiRevision{},
		&types.WikiClaimEvidence{},
	))
	repo := repository.NewMemoryWikiPublicationRepository(db)
	wiki := newMemoryWikiPageFake()
	kb := memoryKnowledgeBaseFake{byID: map[string]*types.KnowledgeBase{
		"kb-team-a": {
			ID: "kb-team-a", TenantID: 7, Type: types.KnowledgeBaseTypeWiki,
			IsMemoryWiki: true, MemoryTeamID: "team-a", IndexingStrategy: types.IndexingStrategy{WikiEnabled: true},
			WikiConfig: &types.WikiConfig{IsMemoryWiki: true, MemoryTeamID: "team-a"},
		},
		"kb-team-b": {
			ID: "kb-team-b", TenantID: 7, Type: types.KnowledgeBaseTypeWiki,
			IsMemoryWiki: true, MemoryTeamID: "team-b", IndexingStrategy: types.IndexingStrategy{WikiEnabled: true},
			WikiConfig: &types.WikiConfig{IsMemoryWiki: true, MemoryTeamID: "team-b"},
		},
		"kb-ordinary-wiki": {
			ID: "kb-ordinary-wiki", TenantID: 7, Type: types.KnowledgeBaseTypeWiki,
			WikiConfig: &types.WikiConfig{MemoryTeamID: "team-a"},
		},
		"kb-cross-tenant": {
			ID: "kb-cross-tenant", TenantID: 8, Type: types.KnowledgeBaseTypeWiki,
			IsMemoryWiki: true, MemoryTeamID: "team-a", IndexingStrategy: types.IndexingStrategy{WikiEnabled: true},
			WikiConfig: &types.WikiConfig{IsMemoryWiki: true, MemoryTeamID: "team-a"},
		},
	}}
	return newService(repo, wiki, &kb), repo, wiki, db
}

func memoryWikiAdminContext() context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	return context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleAdmin)
}

func checksumForMemoryContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("sha256:%x", sum[:])
}

func validTrustedL3Event(eventID, memoryID string, version uint64) types.TrustedL3Event {
	claimText := "The recovery sequence is stable."
	content := fmt.Sprintf("## Audited source\n\nSource-only audit narrative for version %d.\n\n%s", version, claimText)
	locator := types.EvidenceReference{Type: "memory_l1", ID: "l1-1", Locator: "session:s-1/turn:4"}
	return types.TrustedL3Event{
		EventID: eventID, EventType: types.MemoryL3EventMatured, SchemaVersion: "1.0",
		OccurredAt: time.Now().UTC(), TenantID: 7, DepartmentID: "department-1",
		WorkspaceID: "workspace-1", ProjectID: "project-1", TeamID: "team-a",
		BindingID: "binding-a", UserID: "user-1", AgentID: "agent-1", TaskID: "task-1",
		MemoryID: memoryID, MemoryVersion: version, MemoryLevel: "L3", Maturity: "matured",
		Title: "Safe recovery sequence", Summary: "A mature operational conclusion",
		ContentMarkdown: content, Confidence: 0.97, Sensitivity: "internal",
		EvidenceRefs:    types.EvidenceReferences{locator},
		Claims:          types.ClaimEvidenceSet{{ClaimID: "claim-1", Text: claimText, Factual: true, Evidence: types.EvidenceReferences{locator}}},
		ContentChecksum: checksumForMemoryContent(content),
	}
}

func sameProjectionNextVersion(event types.TrustedL3Event, eventID string) types.TrustedL3Event {
	event.EventID = eventID
	event.MemoryVersion++
	event.OccurredAt = event.OccurredAt.Add(time.Second)
	return event
}

func approveProjection(t *testing.T, service *Service, projection *interfaces.ExternalMemoryProjection) {
	t.Helper()
	_, err := service.Approve(
		memoryWikiAdminContext(), projection.Snapshot.ProjectionKey(), projection.ReviewTask.ID,
		"reviewer-1", "claim evidence verified",
	)
	require.NoError(t, err)
}

func TestExternalMemoryTrustedIntakeValidatesAndDeduplicates(t *testing.T) {
	service, _, _, db := newMemoryWikiServiceTest(t)
	ctx := context.Background()
	event := validTrustedL3Event("evt-intake", "memory-a", 1)

	first, duplicate, err := service.ReceiveTrustedL3Event(ctx, event)
	require.NoError(t, err)
	require.False(t, duplicate)
	require.Equal(t, types.MemoryReviewStatusPendingReview, first.ReviewTask.Status)
	require.Equal(t, event.ContentChecksum, first.Snapshot.ContentChecksum)

	second, duplicate, err := service.ReceiveTrustedL3Event(ctx, event)
	require.NoError(t, err)
	require.True(t, duplicate)
	require.Equal(t, first.ReviewTask.ID, second.ReviewTask.ID)

	var events, snapshots, reviews, publications int64
	require.NoError(t, db.Model(&types.MemoryIntegrationEvent{}).Count(&events).Error)
	require.NoError(t, db.Model(&types.MemoryL3Snapshot{}).Count(&snapshots).Error)
	require.NoError(t, db.Model(&types.MemoryReviewTask{}).Count(&reviews).Error)
	require.NoError(t, db.Model(&types.MemoryWikiPublication{}).Count(&publications).Error)
	require.Equal(t, int64(1), events)
	require.Equal(t, int64(1), snapshots)
	require.Equal(t, int64(1), reviews)
	require.Equal(t, int64(1), publications)
}

func TestExternalMemoryTrustedIntakeRejectsInvalidEvents(t *testing.T) {
	service, _, _, _ := newMemoryWikiServiceTest(t)
	ctx := context.Background()

	tests := map[string]func(*types.TrustedL3Event){
		"not l3":                func(e *types.TrustedL3Event) { e.MemoryLevel = "L2" },
		"not matured":           func(e *types.TrustedL3Event) { e.Maturity = "draft" },
		"unsupported event":     func(e *types.TrustedL3Event) { e.EventType = "memory.l2.confirmed" },
		"missing binding":       func(e *types.TrustedL3Event) { e.BindingID = "" },
		"missing user":          func(e *types.TrustedL3Event) { e.UserID = "" },
		"invalid optional task": func(e *types.TrustedL3Event) { e.TaskID = "   " },
		"bad checksum":          func(e *types.TrustedL3Event) { e.ContentChecksum = "sha256:deadbeef" },
		"non-finite confidence": func(e *types.TrustedL3Event) { e.Confidence = math.NaN() },
		"missing evidence":      func(e *types.TrustedL3Event) { e.EvidenceRefs = nil },
		"invalid locator":       func(e *types.TrustedL3Event) { e.EvidenceRefs[0].Locator = "" },
		"unsafe claim id":       func(e *types.TrustedL3Event) { e.Claims[0].ClaimID = "../../fact" },
		"claim absent from source": func(e *types.TrustedL3Event) {
			e.Claims[0].Text = "This claim was never present in the audited source."
		},
		"oversized markdown": func(e *types.TrustedL3Event) {
			e.ContentMarkdown = strings.Repeat("x", MaxTrustedL3MarkdownBytes+1)
			e.ContentChecksum = checksumForMemoryContent(e.ContentMarkdown)
		},
		"oversized evidence": func(e *types.TrustedL3Event) {
			e.EvidenceRefs[0].Locator = strings.Repeat("x", MaxTrustedL3EvidenceBytes)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			suffix := strings.ReplaceAll(name, " ", "-")
			event := validTrustedL3Event("evt-invalid-"+suffix, "memory-invalid-"+suffix, 1)
			mutate(&event)
			_, _, err := service.ReceiveTrustedL3Event(ctx, event)
			require.Error(t, err)
		})
	}
}

func TestExternalMemoryTrustedIntakeAllowsOptionalBindingContextScope(t *testing.T) {
	service, _, _, _ := newMemoryWikiServiceTest(t)
	event := validTrustedL3Event("evt-optional-scope", "memory-optional-scope", 1)
	event.DepartmentID = ""
	event.WorkspaceID = ""
	event.ProjectID = ""
	event.TaskID = ""

	projection, duplicate, err := service.ReceiveTrustedL3Event(context.Background(), event)
	require.NoError(t, err)
	require.False(t, duplicate)
	require.Equal(t, "user-1", projection.Event.UserID)
	require.Empty(t, projection.Snapshot.DepartmentID)
	require.Empty(t, projection.ReviewTask.WorkspaceID)
	require.Empty(t, projection.Publication.ProjectID)
	require.Empty(t, projection.Publication.TaskID)
}

func TestExternalMemoryTrustedIntakeRejectsNonFiniteConfidenceBeforePersistence(t *testing.T) {
	event := validTrustedL3Event("evt-nan", "memory-nan", 1)
	event.Confidence = math.NaN()
	require.ErrorContains(t, validateTrustedL3Event(event), "confidence")
}

func TestExternalMemoryOrdinarySubmitCannotForgeTrustedMemory(t *testing.T) {
	service, _, _, _ := newMemoryWikiServiceTest(t)
	err := service.Submit(context.Background(), &types.MemoryWikiPublication{
		TenantID: 7, TeamID: "team-a", BindingID: "binding-a", MemoryID: "forged",
		MemoryVersion: 1, Markdown: "forged L3", Evidence: types.StringArray{"forged"},
	})
	require.ErrorIs(t, err, ErrTrustedL3EventRequired)
}

func TestExternalMemoryChangesRequestedReviewFlow(t *testing.T) {
	service, _, _, _ := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	projection, _, err := service.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-review-changes", "memory-review-changes", 1))
	require.NoError(t, err)

	changed, err := service.RequestChanges(
		ctx, projection.Snapshot.ProjectionKey(), projection.ReviewTask.ID,
		"reviewer-1", "clarify the recovery precondition",
	)
	require.NoError(t, err)
	require.Equal(t, types.MemoryReviewStatusChangesRequested, changed.Status)

	pending, err := service.ResubmitChanges(
		ctx, projection.Snapshot.ProjectionKey(), projection.ReviewTask.ID,
		"memorycore:user-1", "revision received",
	)
	require.NoError(t, err)
	require.Equal(t, types.MemoryReviewStatusPendingReview, pending.Status)
}

func TestExternalMemoryReviewAndPublicationRequireAdminEvenWhenRouterRBACIsBypassed(t *testing.T) {
	service, _, _, _ := newMemoryWikiServiceTest(t)
	projection, _, err := service.ReceiveTrustedL3Event(context.Background(), validTrustedL3Event("evt-authz", "memory-authz", 1))
	require.NoError(t, err)
	viewer := context.WithValue(context.Background(), types.TenantRoleContextKey, types.TenantRoleViewer)

	_, err = service.List(viewer, 7, "")
	require.ErrorIs(t, err, ErrMemoryWikiReviewerRequired)
	_, err = service.GetReview(viewer, 7, projection.Publication.ID)
	require.ErrorIs(t, err, ErrMemoryWikiReviewerRequired)
	_, err = service.ApprovePublication(viewer, 7, projection.Publication.ID, "viewer-1", "")
	require.ErrorIs(t, err, ErrMemoryWikiReviewerRequired)
	_, err = service.PublishApproved(viewer, 7, projection.Publication.ID, "kb-team-a")
	require.ErrorIs(t, err, ErrMemoryWikiReviewerRequired)
}

func TestExternalMemoryPublishRejectsUnapprovedAndInvalidTargets(t *testing.T) {
	service, _, wiki, _ := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	projection, _, err := service.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-target", "memory-target", 1))
	require.NoError(t, err)

	_, err = service.PublishApproved(ctx, 7, projection.Publication.ID, "kb-team-a")
	require.ErrorIs(t, err, ErrMemoryReviewNotApproved)
	require.Zero(t, wiki.createCalls)

	approveProjection(t, service, projection)
	for _, kbID := range []string{"kb-team-b", "kb-ordinary-wiki", "kb-cross-tenant"} {
		_, err = service.PublishApproved(ctx, 7, projection.Publication.ID, kbID)
		require.ErrorIs(t, err, ErrInvalidMemoryWikiTarget, kbID)
	}
	require.Zero(t, wiki.createCalls)
}

func TestExternalMemoryPublishAutoProvisionsExactlyOneZeroRAGTeamWiki(t *testing.T) {
	service, _, wiki, _ := newMemoryWikiServiceTest(t)
	kb := service.kb.(*memoryKnowledgeBaseFake)
	kb.mu.Lock()
	delete(kb.byID, "kb-team-a")
	kb.mu.Unlock()
	ctx := memoryWikiAdminContext()
	projection, _, err := service.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-auto-kb", "memory-auto-kb", 1))
	require.NoError(t, err)
	approveProjection(t, service, projection)
	page, err := service.PublishApproved(ctx, 7, projection.Publication.ID, "")
	require.NoError(t, err)
	require.NotEmpty(t, page.KnowledgeBaseID)
	require.Empty(t, page.ChunkRefs)

	kbs, err := kb.ListKnowledgeBasesByTenantID(ctx, 7)
	require.NoError(t, err)
	teamMemoryWikis := 0
	for _, candidate := range kbs {
		if candidate.MemoryTeamID == "team-a" {
			teamMemoryWikis++
			require.True(t, candidate.IsDedicatedMemoryWiki())
			require.False(t, candidate.IndexingStrategy.VectorEnabled)
			require.False(t, candidate.IndexingStrategy.KeywordEnabled)
			require.False(t, candidate.IndexingStrategy.GraphEnabled)
		}
	}
	require.Equal(t, 1, teamMemoryWikis)
	current, err := wiki.GetPageBySlug(ctx, page.KnowledgeBaseID, page.Slug)
	require.NoError(t, err)
	require.Equal(t, page.ID, current.ID)
}

func TestExternalMemoryPublishRequiresEvidenceForEveryFactualClaim(t *testing.T) {
	service, repo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	event := validTrustedL3Event("evt-evidence", "memory-evidence", 1)
	event.Claims[0].Evidence = nil
	projection, _, err := service.ReceiveTrustedL3Event(ctx, event)
	require.NoError(t, err, "missing claim coverage is reviewable but must not be publishable")
	approveProjection(t, service, projection)

	_, err = service.PublishApproved(ctx, 7, projection.Publication.ID, "kb-team-a")
	require.ErrorIs(t, err, ErrMemoryClaimEvidenceRequired)
	require.Zero(t, wiki.createCalls)
	stored, err := repo.GetMemoryWikiPublication(ctx, 7, projection.Publication.ID)
	require.NoError(t, err)
	require.Equal(t, types.MemoryReviewStatusApproved, stored.Status)
	require.Nil(t, stored.PublishedAt)
}

func TestExternalMemoryPublishIsIdempotentAndPersistsClaimEvidence(t *testing.T) {
	service, repo, wiki, db := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	projection, _, err := service.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-publish", "memory-publish", 1))
	require.NoError(t, err)
	approveProjection(t, service, projection)

	first, err := service.PublishApproved(ctx, 7, projection.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	second, err := service.PublishApproved(ctx, 7, projection.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, first.Slug, second.Slug)
	require.Equal(t, 1, wiki.createCalls)
	require.Zero(t, wiki.updateCalls)
	require.Contains(t, first.Content, "schema: fmind.cognition/v1")
	require.Contains(t, first.Content, "source_memory_version: 1")
	require.Contains(t, first.Content, `user_id: "user-1"`)
	require.Contains(t, first.Content, `id="claim-1"`)
	require.Contains(t, first.Content, "The recovery sequence is stable.")
	require.Contains(t, first.Content, "Source-only audit narrative", "the reviewed mature Markdown must remain the primary Wiki document")
	metadata, err := first.PageMetadata.Map()
	require.NoError(t, err)
	require.Equal(t, "user-1", metadata["source_user_id"])
	require.Empty(t, first.ChunkRefs, "memory Wiki publication must not create RAG chunk provenance")

	stored, err := repo.GetMemoryWikiPublication(ctx, 7, projection.Publication.ID)
	require.NoError(t, err)
	require.Equal(t, types.MemoryReviewStatusPublished, stored.Status)
	require.Equal(t, first.ID, stored.PublishedPageID)
	require.Equal(t, 1, stored.WikiPageVersion)
	require.NotNil(t, stored.PublishedAt)

	var claims []types.WikiClaimEvidence
	require.NoError(t, db.Where("publication_id = ?", projection.Publication.ID).Find(&claims).Error)
	require.Len(t, claims, 1)
	require.Equal(t, "claim-1", claims[0].ClaimID)
	require.Equal(t, "binding-a", claims[0].BindingID)
	require.Equal(t, "user-1", claims[0].UserID)
	require.Equal(t, "#claim-1", claims[0].WikiLocator)
	require.NotEmpty(t, claims[0].EvidenceLocators)
}

func TestExternalMemoryRepeatedPublishToleratesTransientPageReadMiss(t *testing.T) {
	service, _, wiki, _ := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	projection, _, err := service.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-transient-read", "memory-transient-read", 1))
	require.NoError(t, err)
	approveProjection(t, service, projection)

	first, err := service.PublishApproved(ctx, 7, projection.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	wiki.forceMisses = 1

	second, err := service.PublishApproved(ctx, 7, projection.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, 1, wiki.createCalls)
}

func TestExternalMemoryNewVersionUpdatesSamePage(t *testing.T) {
	service, repo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	firstProjection, _, err := service.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-version-1", "memory-versioned", 1))
	require.NoError(t, err)
	approveProjection(t, service, firstProjection)
	firstPage, err := service.PublishApproved(ctx, 7, firstProjection.Publication.ID, "kb-team-a")
	require.NoError(t, err)

	secondProjection, duplicate, err := service.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-version-2", "memory-versioned", 2))
	require.NoError(t, err)
	require.False(t, duplicate)
	require.NotEqual(t, firstProjection.ReviewTask.ID, secondProjection.ReviewTask.ID)
	approveProjection(t, service, secondProjection)
	secondPage, err := service.PublishApproved(ctx, 7, secondProjection.Publication.ID, "kb-team-a")
	require.NoError(t, err)

	require.Equal(t, firstPage.ID, secondPage.ID)
	require.Equal(t, firstPage.Slug, secondPage.Slug)
	require.Equal(t, 1, wiki.createCalls)
	require.Equal(t, 1, wiki.updateCalls)
	require.Equal(t, 2, secondPage.Version)
	require.Contains(t, secondPage.Content, "source_memory_version: 2")

	stored, err := repo.GetMemoryWikiPublication(ctx, 7, secondProjection.Publication.ID)
	require.NoError(t, err)
	require.Equal(t, 2, stored.WikiPageVersion)
}

func TestExternalMemoryNewVersionWithSameChecksumAdvancesLifecycleRevision(t *testing.T) {
	service, repo, wiki, db := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	firstEvent := validTrustedL3Event("evt-same-checksum-1", "memory-same-checksum", 1)
	firstProjection, _, err := service.ReceiveTrustedL3Event(ctx, firstEvent)
	require.NoError(t, err)
	approveProjection(t, service, firstProjection)
	firstPage, err := service.PublishApproved(ctx, 7, firstProjection.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	firstPublication, err := repo.GetMemoryWikiPublication(ctx, 7, firstProjection.Publication.ID)
	require.NoError(t, err)

	secondEvent := validTrustedL3Event("evt-same-checksum-2", "memory-same-checksum", 2)
	secondEvent.ContentMarkdown = firstEvent.ContentMarkdown
	secondEvent.ContentChecksum = firstEvent.ContentChecksum
	secondEvent.Claims = firstEvent.Claims
	secondEvent.EvidenceRefs = firstEvent.EvidenceRefs
	secondProjection, duplicate, err := service.ReceiveTrustedL3Event(ctx, secondEvent)
	require.NoError(t, err)
	require.False(t, duplicate)
	approveProjection(t, service, secondProjection)
	secondPage, err := service.PublishApproved(ctx, 7, secondProjection.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	secondPublication, err := repo.GetMemoryWikiPublication(ctx, 7, secondProjection.Publication.ID)
	require.NoError(t, err)

	require.Equal(t, firstPage.ID, secondPage.ID)
	require.Equal(t, 1, wiki.createCalls)
	require.Equal(t, 1, wiki.updateCalls)
	require.NotEmpty(t, firstPublication.WikiRevisionID)
	require.NotEqual(t, firstPublication.WikiRevisionID, secondPublication.WikiRevisionID)
	require.Equal(t, firstPublication.WikiPageVersion+1, secondPublication.WikiPageVersion)
	var revisionCount int64
	require.NoError(t, db.Model(&types.MemoryWikiRevision{}).Count(&revisionCount).Error)
	require.Equal(t, int64(2), revisionCount)
	linkedPublications, err := repo.ListMemoryWikiPublicationsByRevision(ctx, 7, firstPublication.WikiRevisionID)
	require.NoError(t, err)
	require.Len(t, linkedPublications, 1)
}

func TestExternalMemorySameMarkdownWithChangedStructuredClaimsCreatesNewRevision(t *testing.T) {
	service, repo, wiki, db := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	firstEvent := validTrustedL3Event("evt-same-checksum-claims-1", "memory-same-checksum-claims", 1)
	firstProjection, _, err := service.ReceiveTrustedL3Event(ctx, firstEvent)
	require.NoError(t, err)
	approveProjection(t, service, firstProjection)
	_, err = service.PublishApproved(ctx, 7, firstProjection.Publication.ID, "kb-team-a")
	require.NoError(t, err)

	secondEvent := validTrustedL3Event("evt-same-checksum-claims-2", "memory-same-checksum-claims", 2)
	secondEvent.ContentMarkdown = firstEvent.ContentMarkdown
	secondEvent.ContentChecksum = firstEvent.ContentChecksum
	secondEvent.Claims = types.ClaimEvidenceSet{{
		ClaimID: "claim-2", Text: "Source-only audit narrative", Factual: true,
		Evidence: firstEvent.EvidenceRefs,
	}}
	secondProjection, duplicate, err := service.ReceiveTrustedL3Event(ctx, secondEvent)
	require.NoError(t, err)
	require.False(t, duplicate)
	approveProjection(t, service, secondProjection)
	secondPage, err := service.PublishApproved(ctx, 7, secondProjection.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	require.Equal(t, 1, wiki.createCalls)
	require.Equal(t, 1, wiki.updateCalls)
	require.Contains(t, secondPage.Content, "claim-2")
	stored, getErr := repo.GetMemoryWikiPublication(ctx, 7, secondProjection.Publication.ID)
	require.NoError(t, getErr)
	require.Equal(t, types.MemoryReviewStatusPublished, stored.Status)
	require.NotNil(t, stored.PublishedAt)
	var revisions int64
	require.NoError(t, db.Model(&types.MemoryWikiRevision{}).Count(&revisions).Error)
	require.Equal(t, int64(2), revisions)
}

func TestExternalMemorySameMarkdownWithChangedRenderedMetadataUpdatesWiki(t *testing.T) {
	service, repo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	firstEvent := validTrustedL3Event("evt-render-meta-1", "memory-render-meta", 1)
	first, _, err := service.ReceiveTrustedL3Event(ctx, firstEvent)
	require.NoError(t, err)
	approveProjection(t, service, first)
	firstPage, err := service.PublishApproved(ctx, 7, first.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	firstPublication, err := repo.GetMemoryWikiPublication(ctx, 7, first.Publication.ID)
	require.NoError(t, err)

	secondEvent := validTrustedL3Event("evt-render-meta-2", "memory-render-meta", 2)
	secondEvent.ContentMarkdown = firstEvent.ContentMarkdown
	secondEvent.ContentChecksum = firstEvent.ContentChecksum
	secondEvent.Claims = firstEvent.Claims
	secondEvent.EvidenceRefs = firstEvent.EvidenceRefs
	secondEvent.Title = "Restricted recovery sequence"
	secondEvent.Confidence = 0.81
	secondEvent.Sensitivity = "restricted"
	second, _, err := service.ReceiveTrustedL3Event(ctx, secondEvent)
	require.NoError(t, err)
	approveProjection(t, service, second)
	secondPage, err := service.PublishApproved(ctx, 7, second.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	require.Equal(t, firstPage.ID, secondPage.ID)
	require.Equal(t, "Restricted recovery sequence", secondPage.Title)
	require.Contains(t, secondPage.Content, "confidence: 0.81")
	require.Contains(t, secondPage.Content, `sensitivity: "restricted"`)
	require.Equal(t, 1, wiki.updateCalls)
	secondPublication, err := repo.GetMemoryWikiPublication(ctx, 7, second.Publication.ID)
	require.NoError(t, err)
	require.NotEqual(t, firstPublication.WikiRevisionID, secondPublication.WikiRevisionID)
}

func TestExternalMemoryPersistsImmutableRevisionForWinningWikiUpdate(t *testing.T) {
	service, repo, wiki, db := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	firstProjection, _, err := service.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-revision-order-1", "memory-revision-order", 1))
	require.NoError(t, err)
	approveProjection(t, service, firstProjection)
	_, err = service.PublishApproved(ctx, 7, firstProjection.Publication.ID, "kb-team-a")
	require.NoError(t, err)

	secondEvent := validTrustedL3Event("evt-revision-order-2", "memory-revision-order", 2)
	secondProjection, _, err := service.ReceiveTrustedL3Event(ctx, secondEvent)
	require.NoError(t, err)
	approveProjection(t, service, secondProjection)
	secondPage, err := service.PublishApproved(ctx, 7, secondProjection.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	require.Equal(t, 1, wiki.successfulUpdates)

	secondPublication, err := repo.GetMemoryWikiPublication(ctx, 7, secondProjection.Publication.ID)
	require.NoError(t, err)
	revision, err := repo.GetMemoryWikiRevision(ctx, 7, secondPublication.WikiRevisionID)
	require.NoError(t, err)
	require.Equal(t, secondPage.ID, revision.WikiPageID)
	require.Equal(t, secondEvent.ContentChecksum, revision.ContentChecksum)
	require.Equal(t, secondPage.Content, revision.Content)
	var revisionCount int64
	require.NoError(t, db.Model(&types.MemoryWikiRevision{}).
		Where("wiki_page_id = ? AND content_checksum = ?", secondPage.ID, secondEvent.ContentChecksum).
		Count(&revisionCount).Error)
	require.Equal(t, int64(1), revisionCount)
}

func TestExternalMemoryRevisionIDIsDeterministic(t *testing.T) {
	checksum := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	first := StableMemoryWikiRevisionID("page-1", checksum, 1)
	second := StableMemoryWikiRevisionID("page-1", checksum, 1)
	require.Equal(t, first, second)
	require.Regexp(t, `^mwr_[a-f0-9]{64}$`, first)
	require.NotEqual(t, first, StableMemoryWikiRevisionID("page-1", strings.Replace(checksum, "a", "b", 1), 1))
	require.NotEqual(t, first, StableMemoryWikiRevisionID("page-1", checksum, 2))
}

func TestExternalMemoryPublishFailureReturnsToApprovedAndRetries(t *testing.T) {
	service, repo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	projection, _, err := service.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-retry", "memory-retry", 1))
	require.NoError(t, err)
	approveProjection(t, service, projection)
	wiki.failCreate = 1

	_, err = service.PublishApproved(ctx, 7, projection.Publication.ID, "kb-team-a")
	require.ErrorContains(t, err, "wiki create unavailable")
	stored, getErr := repo.GetMemoryWikiPublication(ctx, 7, projection.Publication.ID)
	require.NoError(t, getErr)
	require.Equal(t, types.MemoryReviewStatusApproved, stored.Status)
	require.Nil(t, stored.PublishedAt)
	require.NotEmpty(t, stored.LastError)

	page, err := service.PublishApproved(ctx, 7, projection.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	require.NotEmpty(t, page.ID)
	require.Equal(t, 2, wiki.createCalls)
	stored, err = repo.GetMemoryWikiPublication(ctx, 7, projection.Publication.ID)
	require.NoError(t, err)
	require.Equal(t, types.MemoryReviewStatusPublished, stored.Status)
}

func TestExternalMemoryCheckpointFailureReusesCreatedPageOnRetry(t *testing.T) {
	baseService, baseRepo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	projection, _, err := baseService.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-checkpoint", "memory-checkpoint", 1))
	require.NoError(t, err)
	approveProjection(t, baseService, projection)

	failingRepo := &failCompleteMemoryRepository{MemoryWikiPublicationRepository: baseRepo, failures: 1}
	service := newService(failingRepo, wiki, baseService.kb)
	_, err = service.PublishApproved(ctx, 7, projection.Publication.ID, "kb-team-a")
	require.ErrorContains(t, err, "publication checkpoint unavailable")
	stored, getErr := baseRepo.GetMemoryWikiPublication(ctx, 7, projection.Publication.ID)
	require.NoError(t, getErr)
	require.Equal(t, types.MemoryReviewStatusPublishing, stored.Status)
	require.Nil(t, stored.PublishedAt)
	require.Equal(t, 1, wiki.createCalls)

	page, err := service.PublishApproved(ctx, 7, projection.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	require.NotEmpty(t, page.ID)
	require.Equal(t, 1, wiki.createCalls, "retry must find the stable slug instead of creating another page")
	require.Zero(t, wiki.updateCalls)
}

func TestExternalMemoryConcurrentPublishersConvergeOnOnePage(t *testing.T) {
	baseService, baseRepo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx, cancel := context.WithTimeout(memoryWikiAdminContext(), 5*time.Second)
	defer cancel()
	projection, _, err := baseService.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-concurrent", "memory-concurrent", 1))
	require.NoError(t, err)
	approveProjection(t, baseService, projection)

	barrierRepo := &concurrentReadBarrierRepository{
		MemoryWikiPublicationRepository: baseRepo,
		release:                         make(chan struct{}),
	}
	service := newService(barrierRepo, wiki, baseService.kb)
	wiki.forceMisses = 2
	type publishResult struct {
		page *types.WikiPage
		err  error
	}
	results := make(chan publishResult, 2)
	for range 2 {
		go func() {
			page, publishErr := service.PublishApproved(ctx, 7, projection.Publication.ID, "kb-team-a")
			results <- publishResult{page: page, err: publishErr}
		}()
	}
	first := <-results
	second := <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.NotNil(t, first.page)
	require.NotNil(t, second.page)
	require.Equal(t, first.page.ID, second.page.ID)
	wiki.mu.Lock()
	require.Len(t, wiki.pages, 1)
	wiki.mu.Unlock()
	stored, err := baseRepo.GetMemoryWikiPublication(ctx, 7, projection.Publication.ID)
	require.NoError(t, err)
	require.Equal(t, types.MemoryReviewStatusPublished, stored.Status)
}

func TestExternalMemoryConcurrentPublishersUpdateExistingPageOnce(t *testing.T) {
	baseService, baseRepo, wiki, db := newMemoryWikiServiceTest(t)
	ctx, cancel := context.WithTimeout(memoryWikiAdminContext(), 5*time.Second)
	defer cancel()
	firstProjection, _, err := baseService.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-concurrent-update-1", "memory-concurrent-update", 1))
	require.NoError(t, err)
	approveProjection(t, baseService, firstProjection)
	firstPage, err := baseService.PublishApproved(ctx, 7, firstProjection.Publication.ID, "kb-team-a")
	require.NoError(t, err)

	secondProjection, _, err := baseService.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-concurrent-update-2", "memory-concurrent-update", 2))
	require.NoError(t, err)
	approveProjection(t, baseService, secondProjection)
	barrierRepo := &concurrentReadBarrierRepository{
		MemoryWikiPublicationRepository: baseRepo,
		release:                         make(chan struct{}),
	}
	service := newService(barrierRepo, wiki, baseService.kb)
	wiki.updateEntered = make(chan struct{}, 2)
	wiki.updateRelease = make(chan struct{})
	type publishResult struct {
		page *types.WikiPage
		err  error
	}
	results := make(chan publishResult, 2)
	for range 2 {
		go func() {
			page, publishErr := service.PublishApproved(ctx, 7, secondProjection.Publication.ID, "kb-team-a")
			results <- publishResult{page: page, err: publishErr}
		}()
	}
	<-wiki.updateEntered
	<-wiki.updateEntered
	close(wiki.updateRelease)
	first := <-results
	second := <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.Equal(t, firstPage.ID, first.page.ID)
	require.Equal(t, first.page.ID, second.page.ID)
	require.Equal(t, 2, first.page.Version)
	require.Equal(t, 2, second.page.Version)
	require.Equal(t, 2, wiki.updateCalls, "both workers may attempt the optimistic update")
	require.Equal(t, 1, wiki.successfulUpdates, "only one current-page write may win")
	var revisionCount int64
	require.NoError(t, db.Model(&types.MemoryWikiRevision{}).Count(&revisionCount).Error)
	require.Equal(t, int64(2), revisionCount, "one immutable revision per distinct content checksum")
	stored, err := baseRepo.GetMemoryWikiPublication(ctx, 7, secondProjection.Publication.ID)
	require.NoError(t, err)
	require.Equal(t, types.MemoryReviewStatusPublished, stored.Status)
	require.Equal(t, first.page.ID, stored.PublishedPageID)
}

func TestExternalMemoryConcurrentDifferentVersionsEventuallyKeepNewestWithoutOrphanRevision(t *testing.T) {
	baseService, baseRepo, wiki, db := newMemoryWikiServiceTest(t)
	ctx, cancel := context.WithTimeout(memoryWikiAdminContext(), 5*time.Second)
	defer cancel()
	first, _, err := baseService.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-version-race-1", "memory-version-race", 1))
	require.NoError(t, err)
	approveProjection(t, baseService, first)
	_, err = baseService.PublishApproved(ctx, 7, first.Publication.ID, "kb-team-a")
	require.NoError(t, err)

	second, _, err := baseService.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-version-race-2", "memory-version-race", 2))
	require.NoError(t, err)
	third, _, err := baseService.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-version-race-3", "memory-version-race", 3))
	require.NoError(t, err)
	approveProjection(t, baseService, second)
	approveProjection(t, baseService, third)

	blocked := &blockingWikiGateway{memoryWikiPageFake: wiki, publicationRepo: baseRepo, entered: make(chan struct{}), release: make(chan struct{})}
	thirdService := newService(baseRepo, blocked, baseService.kb)
	thirdResult := make(chan error, 1)
	go func() {
		_, publishErr := thirdService.PublishApproved(ctx, 7, third.Publication.ID, "kb-team-a")
		thirdResult <- publishErr
	}()
	<-blocked.entered
	// v3 has read v1 and reached the Wiki CAS. Let v2 commit first, then v3's
	// stale expected version must lose without leaving an immutable v3 row.
	_, err = baseService.PublishApproved(ctx, 7, second.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	close(blocked.release)
	require.Error(t, <-thirdResult)

	// Whichever version won the first CAS, retrying v3 must converge to the
	// newest reviewed content. A speculative pre-CAS revision used to make
	// this retry permanently conflict when v2 happened to win first.
	newest, err := baseService.PublishApproved(ctx, 7, third.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	version, _, memoryID, _, provenanceErr := memoryWikiPageProvenance(newest)
	require.NoError(t, provenanceErr)
	require.Equal(t, uint64(3), version)
	require.Equal(t, "memory-version-race", memoryID)

	var revisions int64
	require.NoError(t, db.Model(&types.MemoryWikiRevision{}).Count(&revisions).Error)
	require.Equal(t, int64(3), revisions, "failed CAS attempts must not leave speculative immutable revisions")
}

func TestExternalMemorySameProjectionCASLoserCannotPublishWinnersRevision(t *testing.T) {
	baseService, repo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx, cancel := context.WithTimeout(memoryWikiAdminContext(), 5*time.Second)
	defer cancel()
	firstEvent := validTrustedL3Event("evt-same-version-race-1", "memory-same-version-race", 1)
	first, _, err := baseService.ReceiveTrustedL3Event(ctx, firstEvent)
	require.NoError(t, err)
	approveProjection(t, baseService, first)
	_, err = baseService.PublishApproved(ctx, 7, first.Publication.ID, "kb-team-a")
	require.NoError(t, err)

	secondEvent := sameProjectionNextVersion(firstEvent, "evt-same-version-race-2")
	thirdEvent := sameProjectionNextVersion(secondEvent, "evt-same-version-race-3")
	second, _, err := baseService.ReceiveTrustedL3Event(ctx, secondEvent)
	require.NoError(t, err)
	third, _, err := baseService.ReceiveTrustedL3Event(ctx, thirdEvent)
	require.NoError(t, err)
	approveProjection(t, baseService, second)
	approveProjection(t, baseService, third)

	blocked := &blockingWikiGateway{memoryWikiPageFake: wiki, publicationRepo: repo, entered: make(chan struct{}), release: make(chan struct{})}
	secondService := newService(repo, blocked, baseService.kb)
	secondResult := make(chan error, 1)
	go func() {
		_, publishErr := secondService.PublishApproved(ctx, 7, second.Publication.ID, "kb-team-a")
		secondResult <- publishErr
	}()
	<-blocked.entered
	thirdPage, err := baseService.PublishApproved(ctx, 7, third.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	close(blocked.release)
	require.Error(t, <-secondResult, "the v2 CAS loser must not claim v3's revision")

	secondStored, err := repo.GetMemoryWikiPublication(ctx, 7, second.Publication.ID)
	require.NoError(t, err)
	require.NotEqual(t, types.MemoryReviewStatusPublished, secondStored.Status)
	thirdStored, err := repo.GetMemoryWikiPublication(ctx, 7, third.Publication.ID)
	require.NoError(t, err)
	require.Equal(t, types.MemoryReviewStatusPublished, thirdStored.Status)
	require.Equal(t, thirdPage.ID, thirdStored.PublishedPageID)
	revision, err := repo.GetMemoryWikiRevision(ctx, 7, thirdStored.WikiRevisionID)
	require.NoError(t, err)
	require.Equal(t, uint64(3), revision.MemoryVersion)
	require.Equal(t, third.Publication.ID, revision.SourcePublicationID)
	require.Equal(t, third.ReviewTask.ID, revision.SourceReviewTaskID)
}

func TestExternalMemoryRevocationArchivesPublishedWikiAndIsIdempotent(t *testing.T) {
	service, repo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	event := validTrustedL3Event("evt-revoke-source", "memory-revoke", 1)
	projection, _, err := service.ReceiveTrustedL3Event(ctx, event)
	require.NoError(t, err)
	approveProjection(t, service, projection)
	page, err := service.PublishApproved(ctx, 7, projection.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	require.Equal(t, types.WikiPageStatusPublished, page.Status)

	revoked := event
	revoked.EventID = "evt-revoke"
	revoked.EventType = types.MemoryL3EventRevoked
	revoked.Maturity = "revoked"
	revokedProjection, duplicate, err := service.ReceiveTrustedL3Event(ctx, revoked)
	require.NoError(t, err)
	require.False(t, duplicate)
	require.Equal(t, types.MemoryReviewStatusRevoked, revokedProjection.ReviewTask.Status)
	require.Equal(t, types.MemoryReviewStatusRevoked, revokedProjection.Publication.Status)

	archived, err := wiki.GetPageBySlug(ctx, "kb-team-a", page.Slug)
	require.NoError(t, err)
	require.Equal(t, types.WikiPageStatusArchived, archived.Status)
	require.Equal(t, 1, wiki.successfulUpdates, "revocation must advance the Wiki CAS fence")
	archivedVersion := archived.Version
	var archivedMetadata map[string]any
	require.NoError(t, json.Unmarshal(archived.PageMetadata, &archivedMetadata))
	require.Equal(t, "evt-revoke", archivedMetadata["memory_revoked_event_id"])

	// A publisher that observes the durable revoke after its Wiki write may
	// compensate the page again. That convergence must not overwrite the real
	// source event provenance or advance the Wiki page a second time.
	require.NoError(t, service.archiveRevokedMemoryWikiPage(ctx, revokedProjection, "publisher-compensation"))
	compensated, err := wiki.GetPageBySlug(ctx, "kb-team-a", page.Slug)
	require.NoError(t, err)
	require.Equal(t, archivedVersion, compensated.Version)
	require.Equal(t, 1, wiki.successfulUpdates)
	var compensatedMetadata map[string]any
	require.NoError(t, json.Unmarshal(compensated.PageMetadata, &compensatedMetadata))
	require.Equal(t, "evt-revoke", compensatedMetadata["memory_revoked_event_id"])

	retried, duplicate, err := service.ReceiveTrustedL3Event(ctx, revoked)
	require.NoError(t, err)
	require.True(t, duplicate)
	require.Equal(t, types.MemoryReviewStatusRevoked, retried.Publication.Status)
	require.Equal(t, 1, wiki.successfulUpdates)

	_, err = service.PublishApproved(ctx, 7, projection.Publication.ID, "kb-team-a")
	require.ErrorIs(t, err, ErrMemoryReviewNotApproved)
	stored, err := repo.GetMemoryWikiPublication(ctx, 7, projection.Publication.ID)
	require.NoError(t, err)
	require.Equal(t, types.MemoryReviewStatusRevoked, stored.Status)
}

func TestExternalMemoryRevocationWinsAgainstInFlightWikiUpdate(t *testing.T) {
	baseService, repo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx, cancel := context.WithTimeout(memoryWikiAdminContext(), 5*time.Second)
	defer cancel()
	firstEvent := validTrustedL3Event("evt-revoke-race-1", "memory-revoke-race", 1)
	first, _, err := baseService.ReceiveTrustedL3Event(ctx, firstEvent)
	require.NoError(t, err)
	approveProjection(t, baseService, first)
	page, err := baseService.PublishApproved(ctx, 7, first.Publication.ID, "kb-team-a")
	require.NoError(t, err)

	secondEvent := validTrustedL3Event("evt-revoke-race-2", "memory-revoke-race", 2)
	second, _, err := baseService.ReceiveTrustedL3Event(ctx, secondEvent)
	require.NoError(t, err)
	approveProjection(t, baseService, second)
	blocked := &blockingWikiGateway{memoryWikiPageFake: wiki, publicationRepo: repo, entered: make(chan struct{}), release: make(chan struct{})}
	service := newService(repo, blocked, baseService.kb)
	publishResult := make(chan error, 1)
	go func() {
		_, publishErr := service.PublishApproved(ctx, 7, second.Publication.ID, "kb-team-a")
		publishResult <- publishErr
	}()
	<-blocked.entered

	revoked := secondEvent
	revoked.EventID = "evt-revoke-race"
	revoked.EventType = types.MemoryL3EventRevoked
	revoked.Maturity = "revoked"
	_, _, err = baseService.ReceiveTrustedL3Event(ctx, revoked)
	require.NoError(t, err)
	close(blocked.release)
	require.ErrorIs(t, <-publishResult, ErrMemorySourceRevoked)

	stored, err := repo.GetMemoryWikiPublication(ctx, 7, second.Publication.ID)
	require.NoError(t, err)
	require.Equal(t, types.MemoryReviewStatusRevoked, stored.Status)
	current, err := wiki.GetPageBySlug(ctx, "kb-team-a", page.Slug)
	require.NoError(t, err)
	require.Equal(t, types.WikiPageStatusPublished, current.Status)
	version, _, _, _, provenanceErr := memoryWikiPageProvenance(current)
	require.NoError(t, provenanceErr)
	require.Equal(t, uint64(1), version, "the stale v2 publisher must lose before writing")
	require.Equal(t, 0, wiki.successfulUpdates, "v2 revocation must neither publish v2 nor archive the visible v1 head")
}

func TestExternalMemoryRevocationFencesInFlightFirstCreate(t *testing.T) {
	baseService, repo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx, cancel := context.WithTimeout(memoryWikiAdminContext(), 5*time.Second)
	defer cancel()
	event := validTrustedL3Event("evt-revoke-create-source", "memory-revoke-create", 1)
	projection, _, err := baseService.ReceiveTrustedL3Event(ctx, event)
	require.NoError(t, err)
	approveProjection(t, baseService, projection)

	blocked := &blockingCreateWikiGateway{memoryWikiPageFake: wiki, publicationRepo: repo, entered: make(chan struct{}), release: make(chan struct{})}
	service := newService(repo, blocked, baseService.kb)
	publishResult := make(chan error, 1)
	go func() {
		_, publishErr := service.PublishApproved(ctx, 7, projection.Publication.ID, "kb-team-a")
		publishResult <- publishErr
	}()
	<-blocked.entered

	revoked := event
	revoked.EventID = "evt-revoke-create"
	revoked.EventType = types.MemoryL3EventRevoked
	revoked.Maturity = "revoked"
	_, _, err = baseService.ReceiveTrustedL3Event(ctx, revoked)
	require.NoError(t, err)
	close(blocked.release)
	require.ErrorIs(t, <-publishResult, ErrMemorySourceRevoked)

	slug := StableMemoryWikiSlug(7, event.TeamID, event.BindingID, event.MemoryID)
	page, err := wiki.GetPageBySlug(ctx, "kb-team-a", slug)
	require.NoError(t, err)
	require.Equal(t, types.WikiPageStatusArchived, page.Status)
	require.Contains(t, page.Content, "no longer available")
	stored, err := repo.GetMemoryWikiPublication(ctx, 7, projection.Publication.ID)
	require.NoError(t, err)
	require.Equal(t, types.MemoryReviewStatusRevoked, stored.Status)
}

func TestExternalMemoryDelayedOldVersionRevocationKeepsNewerWikiVisible(t *testing.T) {
	service, repo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	firstEvent := validTrustedL3Event("evt-delayed-revoke-1", "memory-delayed-revoke", 1)
	first, _, err := service.ReceiveTrustedL3Event(ctx, firstEvent)
	require.NoError(t, err)
	approveProjection(t, service, first)
	_, err = service.PublishApproved(ctx, 7, first.Publication.ID, "kb-team-a")
	require.NoError(t, err)

	secondEvent := validTrustedL3Event("evt-delayed-revoke-2", "memory-delayed-revoke", 2)
	second, _, err := service.ReceiveTrustedL3Event(ctx, secondEvent)
	require.NoError(t, err)
	approveProjection(t, service, second)
	newest, err := service.PublishApproved(ctx, 7, second.Publication.ID, "kb-team-a")
	require.NoError(t, err)

	revoked := firstEvent
	revoked.EventID = "evt-delayed-revoke"
	revoked.EventType = types.MemoryL3EventRevoked
	revoked.Maturity = "revoked"
	_, _, err = service.ReceiveTrustedL3Event(ctx, revoked)
	require.NoError(t, err)
	current, err := wiki.GetPageBySlug(ctx, "kb-team-a", newest.Slug)
	require.NoError(t, err)
	require.Equal(t, types.WikiPageStatusPublished, current.Status)
	version, _, _, _, provenanceErr := memoryWikiPageProvenance(current)
	require.NoError(t, provenanceErr)
	require.Equal(t, uint64(2), version)
	oldPublication, err := repo.GetMemoryWikiPublication(ctx, 7, first.Publication.ID)
	require.NoError(t, err)
	require.Equal(t, types.MemoryReviewStatusRevoked, oldPublication.Status)
}

func TestExternalMemorySameProjectionHeadAdvanceSurvivesDelayedOldRevoke(t *testing.T) {
	service, repo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	firstEvent := validTrustedL3Event("evt-same-head-1", "memory-same-head", 1)
	first, _, err := service.ReceiveTrustedL3Event(ctx, firstEvent)
	require.NoError(t, err)
	approveProjection(t, service, first)
	firstPage, err := service.PublishApproved(ctx, 7, first.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	firstPublication, err := repo.GetMemoryWikiPublication(ctx, 7, first.Publication.ID)
	require.NoError(t, err)

	secondEvent := sameProjectionNextVersion(firstEvent, "evt-same-head-2")
	second, _, err := service.ReceiveTrustedL3Event(ctx, secondEvent)
	require.NoError(t, err)
	approveProjection(t, service, second)
	secondPage, err := service.PublishApproved(ctx, 7, second.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	secondPublication, err := repo.GetMemoryWikiPublication(ctx, 7, second.Publication.ID)
	require.NoError(t, err)
	require.Equal(t, firstPage.Version+1, secondPage.Version, "a new L3 version must advance the Wiki lifecycle head")
	require.NotEqual(t, firstPublication.WikiRevisionID, secondPublication.WikiRevisionID)

	revoked := firstEvent
	revoked.EventID = "evt-same-head-revoke-1"
	revoked.EventType = types.MemoryL3EventRevoked
	revoked.Maturity = "revoked"
	_, _, err = service.ReceiveTrustedL3Event(ctx, revoked)
	require.NoError(t, err)
	current, err := wiki.GetPageBySlug(ctx, "kb-team-a", firstPage.Slug)
	require.NoError(t, err)
	require.Equal(t, types.WikiPageStatusPublished, current.Status)
}

func TestExternalMemoryApprovedNewerProjectionDoesNotSuppressVisibleOldRevocation(t *testing.T) {
	service, _, wiki, _ := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	firstEvent := validTrustedL3Event("evt-approved-newer-1", "memory-approved-newer", 1)
	first, _, err := service.ReceiveTrustedL3Event(ctx, firstEvent)
	require.NoError(t, err)
	approveProjection(t, service, first)
	firstPage, err := service.PublishApproved(ctx, 7, first.Publication.ID, "kb-team-a")
	require.NoError(t, err)

	secondEvent := sameProjectionNextVersion(firstEvent, "evt-approved-newer-2")
	second, _, err := service.ReceiveTrustedL3Event(ctx, secondEvent)
	require.NoError(t, err)
	approveProjection(t, service, second)
	// Approval alone has not materialized a new Wiki head, so it cannot keep
	// the actually visible v1 alive after its source is revoked.
	revoked := firstEvent
	revoked.EventID = "evt-approved-newer-revoke-1"
	revoked.EventType = types.MemoryL3EventRevoked
	revoked.Maturity = "revoked"
	_, _, err = service.ReceiveTrustedL3Event(ctx, revoked)
	require.NoError(t, err)
	current, err := wiki.GetPageBySlug(ctx, "kb-team-a", firstPage.Slug)
	require.NoError(t, err)
	require.Equal(t, types.WikiPageStatusArchived, current.Status)
}

func TestExternalMemoryNewerRevocationCannotArchiveOlderVisibleHead(t *testing.T) {
	service, repo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	firstEvent := validTrustedL3Event("evt-publishing-revoke-1", "memory-publishing-revoke", 1)
	first, _, err := service.ReceiveTrustedL3Event(ctx, firstEvent)
	require.NoError(t, err)
	approveProjection(t, service, first)
	firstPage, err := service.PublishApproved(ctx, 7, first.Publication.ID, "kb-team-a")
	require.NoError(t, err)

	secondEvent := sameProjectionNextVersion(firstEvent, "evt-publishing-revoke-2")
	second, _, err := service.ReceiveTrustedL3Event(ctx, secondEvent)
	require.NoError(t, err)
	approveProjection(t, service, second)
	_, err = repo.StartMemoryWikiPublishing(ctx, publicationProjectionKey(second.Publication), second.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	revoked := secondEvent
	revoked.EventID = "evt-publishing-revoke-v2"
	revoked.EventType = types.MemoryL3EventRevoked
	revoked.Maturity = "revoked"
	_, _, err = service.ReceiveTrustedL3Event(ctx, revoked)
	require.NoError(t, err)
	current, err := wiki.GetPageBySlug(ctx, "kb-team-a", firstPage.Slug)
	require.NoError(t, err)
	require.Equal(t, types.WikiPageStatusPublished, current.Status)
	version, _, _, _, provenanceErr := memoryWikiPageProvenance(current)
	require.NoError(t, provenanceErr)
	require.Equal(t, uint64(1), version)
}

func TestExternalMemorySameProjectionHeadCASConvergesWhenOldRevokeReadWins(t *testing.T) {
	baseService, repo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx, cancel := context.WithTimeout(memoryWikiAdminContext(), 5*time.Second)
	defer cancel()
	firstEvent := validTrustedL3Event("evt-same-cas-1", "memory-same-cas", 1)
	first, _, err := baseService.ReceiveTrustedL3Event(ctx, firstEvent)
	require.NoError(t, err)
	approveProjection(t, baseService, first)
	firstPage, err := baseService.PublishApproved(ctx, 7, first.Publication.ID, "kb-team-a")
	require.NoError(t, err)

	blocked := &blockingWikiGateway{memoryWikiPageFake: wiki, publicationRepo: repo, entered: make(chan struct{}), release: make(chan struct{})}
	revokeService := newService(repo, blocked, baseService.kb)
	revoked := firstEvent
	revoked.EventID = "evt-same-cas-revoke-1"
	revoked.EventType = types.MemoryL3EventRevoked
	revoked.Maturity = "revoked"
	revokeResult := make(chan error, 1)
	go func() {
		_, _, revokeErr := revokeService.ReceiveTrustedL3Event(ctx, revoked)
		revokeResult <- revokeErr
	}()
	<-blocked.entered

	secondEvent := sameProjectionNextVersion(firstEvent, "evt-same-cas-2")
	second, _, err := baseService.ReceiveTrustedL3Event(ctx, secondEvent)
	require.NoError(t, err)
	approveProjection(t, baseService, second)
	secondPage, err := baseService.PublishApproved(ctx, 7, second.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	require.Equal(t, firstPage.Version+1, secondPage.Version)
	close(blocked.release)
	require.ErrorIs(t, <-revokeResult, repository.ErrWikiPageConflict)

	// The durable outbox retries the already-recorded revoke. The second
	// attempt observes the newer page head and converges without hiding it.
	_, duplicate, err := baseService.ReceiveTrustedL3Event(ctx, revoked)
	require.NoError(t, err)
	require.True(t, duplicate)
	current, err := wiki.GetPageBySlug(ctx, "kb-team-a", firstPage.Slug)
	require.NoError(t, err)
	require.Equal(t, types.WikiPageStatusPublished, current.Status)
	version, _, _, _, provenanceErr := memoryWikiPageProvenance(current)
	require.NoError(t, provenanceErr)
	require.Equal(t, uint64(2), version)
}

func TestExternalMemorySameProjectionCanReactivateArchivedOlderVersion(t *testing.T) {
	service, repo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	firstEvent := validTrustedL3Event("evt-reactivate-1", "memory-reactivate", 1)
	first, _, err := service.ReceiveTrustedL3Event(ctx, firstEvent)
	require.NoError(t, err)
	approveProjection(t, service, first)
	firstPage, err := service.PublishApproved(ctx, 7, first.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	firstPublication, err := repo.GetMemoryWikiPublication(ctx, 7, first.Publication.ID)
	require.NoError(t, err)

	revoked := firstEvent
	revoked.EventID = "evt-reactivate-revoke"
	revoked.EventType = types.MemoryL3EventRevoked
	revoked.Maturity = "revoked"
	_, _, err = service.ReceiveTrustedL3Event(ctx, revoked)
	require.NoError(t, err)
	archived, err := wiki.GetPageBySlug(ctx, "kb-team-a", firstPage.Slug)
	require.NoError(t, err)
	require.Equal(t, types.WikiPageStatusArchived, archived.Status)

	secondEvent := sameProjectionNextVersion(firstEvent, "evt-reactivate-2")
	second, _, err := service.ReceiveTrustedL3Event(ctx, secondEvent)
	require.NoError(t, err)
	approveProjection(t, service, second)
	reactivated, err := service.PublishApproved(ctx, 7, second.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	require.Equal(t, types.WikiPageStatusPublished, reactivated.Status)
	require.Greater(t, reactivated.Version, archived.Version)
	secondPublication, err := repo.GetMemoryWikiPublication(ctx, 7, second.Publication.ID)
	require.NoError(t, err)
	require.NotEqual(t, firstPublication.WikiRevisionID, secondPublication.WikiRevisionID)
}

func TestExternalMemoryRetryRepairsMissingLifecycleRevisionAfterWikiCAS(t *testing.T) {
	baseService, repo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	firstEvent := validTrustedL3Event("evt-revision-repair-1", "memory-revision-repair", 1)
	first, _, err := baseService.ReceiveTrustedL3Event(ctx, firstEvent)
	require.NoError(t, err)
	approveProjection(t, baseService, first)
	firstPage, err := baseService.PublishApproved(ctx, 7, first.Publication.ID, "kb-team-a")
	require.NoError(t, err)

	secondEvent := sameProjectionNextVersion(firstEvent, "evt-revision-repair-2")
	second, _, err := baseService.ReceiveTrustedL3Event(ctx, secondEvent)
	require.NoError(t, err)
	approveProjection(t, baseService, second)
	failingRepo := &failRevisionMemoryRepository{MemoryWikiPublicationRepository: repo, failures: 1}
	failingService := newService(failingRepo, wiki, baseService.kb)
	_, err = failingService.PublishApproved(ctx, 7, second.Publication.ID, "kb-team-a")
	require.ErrorContains(t, err, "revision store unavailable")

	advanced, err := wiki.GetPageBySlug(ctx, "kb-team-a", firstPage.Slug)
	require.NoError(t, err)
	require.Equal(t, firstPage.Version+1, advanced.Version, "the Wiki CAS succeeds before the simulated revision outage")

	repaired, err := baseService.PublishApproved(ctx, 7, second.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	require.Equal(t, advanced.ID, repaired.ID)
	require.Equal(t, advanced.Version, repaired.Version)
	stored, err := repo.GetMemoryWikiPublication(ctx, 7, second.Publication.ID)
	require.NoError(t, err)
	require.Equal(t, types.MemoryReviewStatusPublished, stored.Status)
	require.Equal(t, StableMemoryWikiRevisionID(repaired.ID, memoryWikiProjectionChecksum(second.Snapshot), repaired.Version), stored.WikiRevisionID)
}

func TestExternalMemoryStartCASLoserContinuesFromPublishing(t *testing.T) {
	baseService, baseRepo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	projection, _, err := baseService.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-start-cas", "memory-start-cas", 1))
	require.NoError(t, err)
	approveProjection(t, baseService, projection)

	repo := &casLoserMemoryRepository{MemoryWikiPublicationRepository: baseRepo, loseStart: true}
	service := newService(repo, wiki, baseService.kb)
	page, err := service.PublishApproved(ctx, 7, projection.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	require.NotEmpty(t, page.ID)
}

func TestExternalMemoryCompleteCASLoserReturnsPublishedPage(t *testing.T) {
	baseService, baseRepo, wiki, _ := newMemoryWikiServiceTest(t)
	ctx := memoryWikiAdminContext()
	projection, _, err := baseService.ReceiveTrustedL3Event(ctx, validTrustedL3Event("evt-complete-cas", "memory-complete-cas", 1))
	require.NoError(t, err)
	approveProjection(t, baseService, projection)

	repo := &casLoserMemoryRepository{MemoryWikiPublicationRepository: baseRepo, loseComplete: true}
	service := newService(repo, wiki, baseService.kb)
	page, err := service.PublishApproved(ctx, 7, projection.Publication.ID, "kb-team-a")
	require.NoError(t, err)
	require.NotEmpty(t, page.ID)
	stored, err := baseRepo.GetMemoryWikiPublication(ctx, 7, projection.Publication.ID)
	require.NoError(t, err)
	require.Equal(t, types.MemoryReviewStatusPublished, stored.Status)
	require.Equal(t, page.ID, stored.PublishedPageID)
}

func TestExternalMemorySlugIsStableAndPathSafe(t *testing.T) {
	first := StableMemoryWikiSlug(7, "team-a", "binding-a", "../../Etc/Passwd %2F")
	second := StableMemoryWikiSlug(7, "team-a", "binding-a", "../../Etc/Passwd %2F")
	require.Equal(t, first, second)
	require.True(t, regexp.MustCompile(`^memory/[a-f0-9]{64}$`).MatchString(first), first)
}

func TestMemoryWikiReviewerAuthorityIsBoundToRequestedTenant(t *testing.T) {
	service, _, _, _ := newMemoryWikiServiceTest(t)
	_, err := service.List(memoryWikiAdminContext(), 8, "")
	require.ErrorIs(t, err, ErrMemoryWikiReviewerRequired)
}
