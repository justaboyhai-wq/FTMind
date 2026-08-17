package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newExternalMemoryTestRepository(t *testing.T) (*externalMemoryRepository, *gorm.DB) {
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
	return NewExternalMemoryRepository(db).(*externalMemoryRepository), db
}

func externalMemoryRevisionFixture(id, pageID, checksum, content string) *types.MemoryWikiRevision {
	page := types.WikiPage{
		ID: pageID, TenantID: 7, KnowledgeBaseID: "kb-team-a", Slug: "memory/stable",
		Title: "Safe recovery sequence", Summary: "The recovery sequence is stable.",
		Content: content, PageType: types.WikiPageTypeSummary, Status: types.WikiPageStatusPublished,
		Version: 2, SourceRefs: types.StringArray{"memory:memory-a@2"},
		PageMetadata: types.JSON(`{"schema":"fmind.cognition/v1"}`),
	}
	pageSnapshot, err := json.Marshal(page)
	if err != nil {
		panic(err)
	}
	return &types.MemoryWikiRevision{
		ID: id, TenantID: 7, TeamID: "team-a", BindingID: "binding-a", UserID: "user-1",
		KnowledgeBaseID: page.KnowledgeBaseID, WikiPageID: pageID, WikiPageVersion: page.Version,
		PageSlug: page.Slug, MemoryID: "memory-a", MemoryVersion: 2,
		SourcePublicationID: "publication-2", SourceReviewTaskID: "review-2",
		ContentChecksum: checksum, ProjectionChecksum: checksum, Title: page.Title, Summary: page.Summary, Content: content,
		PageType: page.PageType, PageStatus: page.Status, SourceRefs: page.SourceRefs,
		PageMetadata: page.PageMetadata, PageSnapshot: types.JSON(pageSnapshot),
	}
}

func TestExternalMemoryRepositoryRevisionIsImmutableAndQueryable(t *testing.T) {
	repo, _ := newExternalMemoryTestRepository(t)
	ctx := context.Background()
	revision := externalMemoryRevisionFixture(
		"mwr_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"page-1", "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"immutable revision body",
	)
	var precisePage types.WikiPage
	require.NoError(t, json.Unmarshal(revision.PageSnapshot, &precisePage))
	precisePage.CreatedAt = time.Date(2026, 8, 17, 12, 0, 0, 123456789, time.UTC)
	precisePage.UpdatedAt = precisePage.CreatedAt
	preciseSnapshot, err := json.Marshal(precisePage)
	require.NoError(t, err)
	revision.PageSnapshot = types.JSON(preciseSnapshot)

	first, duplicate, err := repo.CreateMemoryWikiRevision(ctx, revision)
	require.NoError(t, err)
	require.False(t, duplicate)
	require.Equal(t, revision.ID, first.ID)

	second, duplicate, err := repo.CreateMemoryWikiRevision(ctx, revision)
	require.NoError(t, err)
	require.True(t, duplicate)
	require.Equal(t, first.Content, second.Content)
	formatted := *revision
	formatted.PageMetadata = types.JSON(`{ "schema" : "fmind.cognition/v1" }`)
	formattedSnapshot, err := json.MarshalIndent(json.RawMessage(revision.PageSnapshot), "", "  ")
	require.NoError(t, err)
	formatted.PageSnapshot = types.JSON(formattedSnapshot)
	_, duplicate, err = repo.CreateMemoryWikiRevision(ctx, &formatted)
	require.NoError(t, err, "JSONB formatting changes must not turn an idempotent immutable insert into a conflict")
	require.True(t, duplicate)
	precisionVariant := *revision
	precisePage.CreatedAt = precisePage.CreatedAt.Truncate(time.Microsecond)
	precisePage.UpdatedAt = precisePage.UpdatedAt.Truncate(time.Microsecond)
	precisionSnapshot, marshalErr := json.Marshal(precisePage)
	require.NoError(t, marshalErr)
	precisionVariant.PageSnapshot = types.JSON(precisionSnapshot)
	_, duplicate, err = repo.CreateMemoryWikiRevision(ctx, &precisionVariant)
	require.NoError(t, err)
	require.True(t, duplicate, "database timestamp precision must not split one deterministic revision")

	conflict := *revision
	conflict.Content = "attempted overwrite"
	_, _, err = repo.CreateMemoryWikiRevision(ctx, &conflict)
	require.ErrorIs(t, err, ErrMemoryWikiRevisionConflict)

	byID, err := repo.GetMemoryWikiRevision(ctx, 7, revision.ID)
	require.NoError(t, err)
	require.Equal(t, "immutable revision body", byID.Content)
	byChecksum, err := repo.GetMemoryWikiRevisionByPageChecksum(ctx, 7, revision.WikiPageID, revision.ProjectionChecksum)
	require.NoError(t, err)
	require.Equal(t, revision.ID, byChecksum.ID)
}

func externalMemoryProjectionFixture(eventID string, tenantID uint64, teamID, bindingID, memoryID string, version uint64) (*types.MemoryIntegrationEvent, *types.MemoryL3Snapshot, *types.MemoryReviewTask, *types.MemoryWikiPublication) {
	now := time.Now().UTC().Truncate(time.Second)
	checksum := fmt.Sprintf("sha256:%064x", version)
	scope := types.MemoryProjectionKey{
		TenantID: tenantID, TeamID: teamID, BindingID: bindingID,
		MemoryID: memoryID, MemoryVersion: version,
	}
	event := &types.MemoryIntegrationEvent{
		ID: uuid.NewString(), EventID: eventID, EventType: types.MemoryL3EventMatured,
		SchemaVersion: "1.0", OccurredAt: now, TenantID: scope.TenantID,
		DepartmentID: "department-1", WorkspaceID: "workspace-1", ProjectID: "project-1",
		TeamID: scope.TeamID, BindingID: scope.BindingID, UserID: "user-1", AgentID: "agent-1", TaskID: "task-1",
		MemoryID: scope.MemoryID, MemoryVersion: scope.MemoryVersion,
		ContentChecksum: checksum, Status: types.MemoryIntegrationEventProcessed,
	}
	snapshot := &types.MemoryL3Snapshot{
		ID: uuid.NewString(), EventID: eventID, TenantID: scope.TenantID,
		DepartmentID: event.DepartmentID, WorkspaceID: event.WorkspaceID, ProjectID: event.ProjectID,
		TeamID: scope.TeamID, BindingID: scope.BindingID, UserID: event.UserID, AgentID: event.AgentID, TaskID: event.TaskID,
		MemoryID: scope.MemoryID, MemoryVersion: scope.MemoryVersion,
		MemoryLevel: "L3", Maturity: "matured",
		Title: "Operational conclusion", Summary: "A mature L3 conclusion",
		ContentMarkdown: "Use the approved recovery sequence.", Confidence: 0.97,
		Sensitivity:     "internal",
		EvidenceRefs:    types.EvidenceReferences{{Type: "memory_l1", ID: "l1-1", Locator: "turn:4"}},
		Claims:          types.ClaimEvidenceSet{{ClaimID: "claim-1", Text: "The sequence is stable.", Factual: true, Evidence: types.EvidenceReferences{{Type: "memory_l1", ID: "l1-1", Locator: "turn:4"}}}},
		ContentChecksum: checksum,
	}
	review := &types.MemoryReviewTask{
		ID: uuid.NewString(), SnapshotID: snapshot.ID, EventID: eventID,
		TenantID: scope.TenantID, DepartmentID: event.DepartmentID, WorkspaceID: event.WorkspaceID,
		ProjectID: event.ProjectID, TeamID: scope.TeamID, BindingID: scope.BindingID,
		UserID: event.UserID, AgentID: event.AgentID, TaskID: event.TaskID, MemoryID: scope.MemoryID,
		MemoryVersion: scope.MemoryVersion, ContentChecksum: checksum,
		Status: types.MemoryReviewStatusPendingReview, LockVersion: 1,
	}
	publication := &types.MemoryWikiPublication{
		ID: uuid.NewString(), SnapshotID: snapshot.ID, ReviewTaskID: review.ID, EventID: eventID,
		TenantID: scope.TenantID, DepartmentID: event.DepartmentID, WorkspaceID: event.WorkspaceID,
		ProjectID: event.ProjectID, TeamID: scope.TeamID, BindingID: scope.BindingID,
		UserID: event.UserID, AgentID: event.AgentID, TaskID: event.TaskID, MemoryID: scope.MemoryID,
		MemoryVersion: scope.MemoryVersion, ContentChecksum: checksum,
		Title: snapshot.Title, Markdown: snapshot.ContentMarkdown,
		Evidence: types.StringArray{"memory_l1:l1-1#turn:4"},
		Status:   types.MemoryReviewStatusPendingReview, LockVersion: 1,
	}
	return event, snapshot, review, publication
}

func TestExternalMemoryRepositoryRejectsMismatchedUserScope(t *testing.T) {
	repo, _ := newExternalMemoryTestRepository(t)
	event, snapshot, review, publication := externalMemoryProjectionFixture("evt-user-mismatch", 7, "team-a", "binding-a", "memory-a", 1)
	review.UserID = "user-other"

	_, _, err := repo.CreateMaturedMemoryProjection(context.Background(), event, snapshot, review, publication)
	require.ErrorContains(t, err, "scopes do not match")
}

func TestExternalMemoryRepositoryDuplicateEventIsNoOp(t *testing.T) {
	repo, db := newExternalMemoryTestRepository(t)
	ctx := context.Background()
	event, snapshot, review, publication := externalMemoryProjectionFixture("evt-duplicate", 7, "team-a", "binding-a", "memory-a", 1)

	first, duplicate, err := repo.CreateMaturedMemoryProjection(ctx, event, snapshot, review, publication)
	require.NoError(t, err)
	require.False(t, duplicate)

	event2, snapshot2, review2, publication2 := externalMemoryProjectionFixture("evt-duplicate", 7, "team-a", "binding-a", "memory-a", 1)
	event2.OccurredAt = event.OccurredAt
	second, duplicate, err := repo.CreateMaturedMemoryProjection(ctx, event2, snapshot2, review2, publication2)
	require.NoError(t, err)
	require.True(t, duplicate)
	require.Equal(t, first.Event.ID, second.Event.ID)
	require.Equal(t, first.Snapshot.ID, second.Snapshot.ID)
	require.Equal(t, first.ReviewTask.ID, second.ReviewTask.ID)
	require.Equal(t, first.Publication.ID, second.Publication.ID)

	for model, want := range map[any]int64{
		&types.MemoryIntegrationEvent{}: 1,
		&types.MemoryL3Snapshot{}:       1,
		&types.MemoryReviewTask{}:       1,
		&types.MemoryWikiPublication{}:  1,
	} {
		var got int64
		require.NoError(t, db.Model(model).Count(&got).Error)
		require.Equal(t, want, got)
	}
}

func TestExternalMemoryRepositoryDifferentEventIDSameProjectionIsNoOp(t *testing.T) {
	repo, db := newExternalMemoryTestRepository(t)
	ctx := context.Background()
	event, snapshot, review, publication := externalMemoryProjectionFixture("evt-projection-first", 7, "team-a", "binding-a", "memory-a", 1)
	first, duplicate, err := repo.CreateMaturedMemoryProjection(ctx, event, snapshot, review, publication)
	require.NoError(t, err)
	require.False(t, duplicate)

	replayedEvent, replayedSnapshot, replayedReview, replayedPublication := externalMemoryProjectionFixture("evt-projection-retry", 7, "team-a", "binding-a", "memory-a", 1)
	replayedEvent.OccurredAt = event.OccurredAt
	second, duplicate, err := repo.CreateMaturedMemoryProjection(ctx, replayedEvent, replayedSnapshot, replayedReview, replayedPublication)
	require.NoError(t, err)
	require.True(t, duplicate)
	require.Equal(t, first.Event.EventID, second.Event.EventID)

	var eventCount int64
	require.NoError(t, db.Model(&types.MemoryIntegrationEvent{}).Count(&eventCount).Error)
	require.Equal(t, int64(1), eventCount, "one projection must never retain multiple integration events")
}

func TestExternalMemoryRepositoryConcurrentDifferentEventIDsConvergeOnOneProjection(t *testing.T) {
	repo, db := newExternalMemoryTestRepository(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	firstEvent, firstSnapshot, firstReview, firstPublication := externalMemoryProjectionFixture("evt-projection-concurrent-1", 7, "team-a", "binding-a", "memory-concurrent", 1)
	secondEvent, secondSnapshot, secondReview, secondPublication := externalMemoryProjectionFixture("evt-projection-concurrent-2", 7, "team-a", "binding-a", "memory-concurrent", 1)
	secondEvent.OccurredAt = firstEvent.OccurredAt
	type createResult struct {
		projection *interfaces.ExternalMemoryProjection
		duplicate  bool
		err        error
	}
	start := make(chan struct{})
	results := make(chan createResult, 2)
	create := func(event *types.MemoryIntegrationEvent, snapshot *types.MemoryL3Snapshot, review *types.MemoryReviewTask, publication *types.MemoryWikiPublication) {
		<-start
		projection, duplicate, err := repo.CreateMaturedMemoryProjection(ctx, event, snapshot, review, publication)
		results <- createResult{projection: projection, duplicate: duplicate, err: err}
	}
	go create(firstEvent, firstSnapshot, firstReview, firstPublication)
	go create(secondEvent, secondSnapshot, secondReview, secondPublication)
	close(start)
	first := <-results
	second := <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.NotNil(t, first.projection)
	require.NotNil(t, second.projection)
	require.Equal(t, first.projection.Event.ID, second.projection.Event.ID)
	require.NotEqual(t, first.duplicate, second.duplicate, "exactly one concurrent delivery creates the projection")
	var eventCount int64
	require.NoError(t, db.Model(&types.MemoryIntegrationEvent{}).Count(&eventCount).Error)
	require.Equal(t, int64(1), eventCount)
}

func TestExternalMemoryRepositoryDifferentEventIDConflictingProjectionIsRejected(t *testing.T) {
	repo, db := newExternalMemoryTestRepository(t)
	ctx := context.Background()
	event, snapshot, review, publication := externalMemoryProjectionFixture("evt-projection-original", 7, "team-a", "binding-a", "memory-a", 1)
	_, _, err := repo.CreateMaturedMemoryProjection(ctx, event, snapshot, review, publication)
	require.NoError(t, err)

	conflictEvent, conflictSnapshot, conflictReview, conflictPublication := externalMemoryProjectionFixture("evt-projection-conflict", 7, "team-a", "binding-a", "memory-a", 1)
	conflictEvent.OccurredAt = event.OccurredAt
	conflictEvent.ContentChecksum = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	conflictSnapshot.ContentChecksum = conflictEvent.ContentChecksum
	conflictSnapshot.ContentMarkdown = "A conflicting conclusion."
	conflictReview.ContentChecksum = conflictEvent.ContentChecksum
	conflictPublication.ContentChecksum = conflictEvent.ContentChecksum
	_, _, err = repo.CreateMaturedMemoryProjection(ctx, conflictEvent, conflictSnapshot, conflictReview, conflictPublication)
	require.ErrorIs(t, err, ErrExternalMemoryEventConflict)

	var eventCount int64
	require.NoError(t, db.Model(&types.MemoryIntegrationEvent{}).Count(&eventCount).Error)
	require.Equal(t, int64(1), eventCount)
}

func TestExternalMemoryRepositoryRejectsChangedPayloadForReusedEventID(t *testing.T) {
	repo, _ := newExternalMemoryTestRepository(t)
	ctx := context.Background()
	event, snapshot, review, publication := externalMemoryProjectionFixture("evt-replay-conflict", 7, "team-a", "binding-a", "memory-a", 1)
	_, _, err := repo.CreateMaturedMemoryProjection(ctx, event, snapshot, review, publication)
	require.NoError(t, err)

	replayedEvent, replayedSnapshot, replayedReview, replayedPublication := externalMemoryProjectionFixture("evt-replay-conflict", 7, "team-a", "binding-a", "memory-a", 1)
	replayedSnapshot.EvidenceRefs[0].Locator = "session:s-2/turn:99"
	replayedSnapshot.Claims[0].Evidence[0].Locator = "session:s-2/turn:99"
	_, _, err = repo.CreateMaturedMemoryProjection(ctx, replayedEvent, replayedSnapshot, replayedReview, replayedPublication)
	require.ErrorIs(t, err, ErrExternalMemoryEventConflict)

	replayedEvent, replayedSnapshot, replayedReview, replayedPublication = externalMemoryProjectionFixture("evt-replay-conflict", 7, "team-a", "binding-a", "memory-a", 1)
	replayedEvent.OccurredAt = event.OccurredAt.Add(time.Second)
	_, _, err = repo.CreateMaturedMemoryProjection(ctx, replayedEvent, replayedSnapshot, replayedReview, replayedPublication)
	require.ErrorIs(t, err, ErrExternalMemoryEventConflict, "a reused event ID must describe the same event instant")

	replayedEvent, replayedSnapshot, replayedReview, replayedPublication = externalMemoryProjectionFixture("evt-replay-conflict", 7, "team-a", "binding-a", "memory-a", 1)
	replayedEvent.OccurredAt = event.OccurredAt
	replayedEvent.UserID = "user-other"
	replayedSnapshot.UserID = "user-other"
	replayedReview.UserID = "user-other"
	replayedPublication.UserID = "user-other"
	_, _, err = repo.CreateMaturedMemoryProjection(ctx, replayedEvent, replayedSnapshot, replayedReview, replayedPublication)
	require.ErrorIs(t, err, ErrExternalMemoryEventConflict, "a reused event ID cannot cross user scope")
}

func TestExternalMemoryRepositoryScopesSameMemoryIndependently(t *testing.T) {
	repo, db := newExternalMemoryTestRepository(t)
	ctx := context.Background()
	fixtures := []struct {
		eventID  string
		tenantID uint64
		teamID   string
		binding  string
	}{
		{"evt-scope-1", 7, "team-a", "binding-a"},
		{"evt-scope-2", 7, "team-b", "binding-a"},
		{"evt-scope-3", 7, "team-a", "binding-b"},
		{"evt-scope-4", 8, "team-a", "binding-a"},
	}
	for _, f := range fixtures {
		event, snapshot, review, publication := externalMemoryProjectionFixture(f.eventID, f.tenantID, f.teamID, f.binding, "shared-memory", 1)
		_, duplicate, err := repo.CreateMaturedMemoryProjection(ctx, event, snapshot, review, publication)
		require.NoError(t, err)
		require.False(t, duplicate)
	}

	var snapshots, reviews, publications int64
	require.NoError(t, db.Model(&types.MemoryL3Snapshot{}).Count(&snapshots).Error)
	require.NoError(t, db.Model(&types.MemoryReviewTask{}).Count(&reviews).Error)
	require.NoError(t, db.Model(&types.MemoryWikiPublication{}).Count(&publications).Error)
	require.Equal(t, int64(4), snapshots)
	require.Equal(t, int64(4), reviews)
	require.Equal(t, int64(4), publications)

	key := types.MemoryProjectionKey{TenantID: 7, TeamID: "team-a", BindingID: "binding-a", MemoryID: "shared-memory", MemoryVersion: 1}
	got, err := repo.GetMemoryProjection(ctx, key)
	require.NoError(t, err)
	require.Equal(t, "evt-scope-1", got.Event.EventID)
	require.Equal(t, "team-a", got.Snapshot.TeamID)
	require.Equal(t, "binding-a", got.ReviewTask.BindingID)
	require.Equal(t, "user-1", got.Publication.UserID)
	_, err = repo.GetMemoryProjection(ctx, types.MemoryProjectionKey{TenantID: 99, TeamID: key.TeamID, BindingID: key.BindingID, MemoryID: key.MemoryID, MemoryVersion: key.MemoryVersion})
	require.ErrorIs(t, err, ErrExternalMemoryNotFound)
}

func TestExternalMemoryRepositoryNewVersionCreatesNewReview(t *testing.T) {
	repo, db := newExternalMemoryTestRepository(t)
	ctx := context.Background()
	for version := uint64(1); version <= 2; version++ {
		event, snapshot, review, publication := externalMemoryProjectionFixture(fmt.Sprintf("evt-version-%d", version), 7, "team-a", "binding-a", "memory-a", version)
		_, duplicate, err := repo.CreateMaturedMemoryProjection(ctx, event, snapshot, review, publication)
		require.NoError(t, err)
		require.False(t, duplicate)
	}

	var reviews []*types.MemoryReviewTask
	require.NoError(t, db.Order("memory_version ASC").Find(&reviews).Error)
	require.Len(t, reviews, 2)
	require.Equal(t, uint64(1), reviews[0].MemoryVersion)
	require.Equal(t, uint64(2), reviews[1].MemoryVersion)
	require.Equal(t, types.MemoryReviewStatusPendingReview, reviews[1].Status)
}

func TestExternalMemoryRepositoryReviewTransitionUsesCASAndHistory(t *testing.T) {
	repo, db := newExternalMemoryTestRepository(t)
	ctx := context.Background()
	event, snapshot, review, publication := externalMemoryProjectionFixture("evt-review", 7, "team-a", "binding-a", "memory-a", 3)
	_, _, err := repo.CreateMaturedMemoryProjection(ctx, event, snapshot, review, publication)
	require.NoError(t, err)
	key := types.MemoryProjectionKey{TenantID: 7, TeamID: "team-a", BindingID: "binding-a", MemoryID: "memory-a", MemoryVersion: 3}

	approved, err := repo.TransitionMemoryReview(ctx, key, review.ID, types.MemoryReviewStatusPendingReview, types.MemoryReviewStatusApproved, "reviewer-1", "evidence checked")
	require.NoError(t, err)
	require.Equal(t, types.MemoryReviewStatusApproved, approved.Status)
	require.Equal(t, uint64(2), approved.LockVersion)
	require.Equal(t, "evidence checked", approved.ReviewComment)

	_, err = repo.TransitionMemoryReview(ctx, key, review.ID, types.MemoryReviewStatusPendingReview, types.MemoryReviewStatusRejected, "reviewer-2", "stale decision")
	require.ErrorIs(t, err, ErrExternalMemoryStateConflict)

	var history []types.MemoryReviewHistory
	require.NoError(t, db.Where("review_task_id = ?", review.ID).Find(&history).Error)
	require.Len(t, history, 1)
	require.Equal(t, types.MemoryReviewStatusPendingReview, history[0].FromStatus)
	require.Equal(t, types.MemoryReviewStatusApproved, history[0].ToStatus)
	require.Equal(t, "evidence checked", history[0].Comment)

	var storedPublication types.MemoryWikiPublication
	require.NoError(t, db.First(&storedPublication, "id = ?", publication.ID).Error)
	require.Equal(t, types.MemoryReviewStatusApproved, storedPublication.Status)
}

func TestExternalMemoryRepositoryChangesRequestedCanReturnToPendingReview(t *testing.T) {
	repo, db := newExternalMemoryTestRepository(t)
	ctx := context.Background()
	event, snapshot, review, publication := externalMemoryProjectionFixture("evt-changes-requested", 7, "team-a", "binding-a", "memory-a", 1)
	_, _, err := repo.CreateMaturedMemoryProjection(ctx, event, snapshot, review, publication)
	require.NoError(t, err)
	key := snapshot.ProjectionKey()

	changed, err := repo.TransitionMemoryReview(
		ctx, key, review.ID, types.MemoryReviewStatusPendingReview,
		types.MemoryReviewStatusChangesRequested, "reviewer-1", "clarify the recovery precondition",
	)
	require.NoError(t, err)
	require.Equal(t, types.MemoryReviewStatusChangesRequested, changed.Status)
	requireMemoryWorkflowStatus(t, db, review.ID, publication.ID, types.MemoryReviewStatusChangesRequested)

	pending, err := repo.TransitionMemoryReview(
		ctx, key, review.ID, types.MemoryReviewStatusChangesRequested,
		types.MemoryReviewStatusPendingReview, "memorycore:user-1", "revision received",
	)
	require.NoError(t, err)
	require.Equal(t, types.MemoryReviewStatusPendingReview, pending.Status)
	requireMemoryWorkflowStatus(t, db, review.ID, publication.ID, types.MemoryReviewStatusPendingReview)

	var history []types.MemoryReviewHistory
	require.NoError(t, db.Where("review_task_id = ?", review.ID).Order("created_at ASC").Find(&history).Error)
	require.Len(t, history, 2)
	require.Equal(t, types.MemoryReviewStatusChangesRequested, history[0].ToStatus)
	require.Equal(t, types.MemoryReviewStatusPendingReview, history[1].ToStatus)
}

func TestExternalMemoryRepositoryPublicationLifecycleStaysInSyncAndCanRevoke(t *testing.T) {
	repo, db := newExternalMemoryTestRepository(t)
	ctx := context.Background()
	event, snapshot, review, publication := externalMemoryProjectionFixture("evt-lifecycle", 7, "team-a", "binding-a", "memory-a", 4)
	_, _, err := repo.CreateMaturedMemoryProjection(ctx, event, snapshot, review, publication)
	require.NoError(t, err)
	key := snapshot.ProjectionKey()
	_, err = repo.TransitionMemoryReview(ctx, key, review.ID, types.MemoryReviewStatusPendingReview, types.MemoryReviewStatusApproved, "reviewer-1", "approved")
	require.NoError(t, err)

	_, err = repo.StartMemoryWikiPublishing(ctx, key, publication.ID, "kb-team-a")
	require.NoError(t, err)
	requireMemoryWorkflowStatus(t, db, review.ID, publication.ID, types.MemoryReviewStatusPublishing)

	require.NoError(t, repo.FailMemoryWikiPublishing(ctx, key, publication.ID, "publishing", "temporary failure"))
	requireMemoryWorkflowStatus(t, db, review.ID, publication.ID, types.MemoryReviewStatusApproved)

	_, err = repo.StartMemoryWikiPublishing(ctx, key, publication.ID, "kb-team-a")
	require.NoError(t, err)
	revision := externalMemoryRevisionFixture(
		"mwr_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"page-1", snapshot.ContentChecksum, "published body",
	)
	revision.TeamID = publication.TeamID
	revision.BindingID = publication.BindingID
	revision.UserID = publication.UserID
	revision.MemoryID = publication.MemoryID
	revision.MemoryVersion = publication.MemoryVersion
	revision.SourcePublicationID = publication.ID
	revision.SourceReviewTaskID = review.ID
	revision.WikiPageVersion = 1
	_, _, err = repo.CreateMemoryWikiRevision(ctx, revision)
	require.NoError(t, err)
	_, err = repo.CompleteMemoryWikiPublishing(ctx, key, publication.ID, types.MemoryWikiPublishResult{
		KnowledgeBaseID: "kb-team-a", WikiPageID: "page-1", WikiRevisionID: revision.ID,
		WikiPageVersion: 1, PublishedAt: time.Now().UTC(),
	})
	require.NoError(t, err)
	requireMemoryWorkflowStatus(t, db, review.ID, publication.ID, types.MemoryReviewStatusPublished)
	_, err = repo.CompleteMemoryWikiPublishing(ctx, key, publication.ID, types.MemoryWikiPublishResult{
		KnowledgeBaseID: "kb-other", WikiPageID: "page-1", WikiRevisionID: revision.ID,
		WikiPageVersion: 999, PublishedAt: time.Now().UTC(),
	})
	require.ErrorIs(t, err, ErrExternalMemoryStateConflict, "published checkpoint replay must match the complete durable result")

	_, err = repo.TransitionMemoryReview(ctx, key, review.ID, types.MemoryReviewStatusPublished, types.MemoryReviewStatusRevoked, "reviewer-2", "source revoked")
	require.NoError(t, err)
	requireMemoryWorkflowStatus(t, db, review.ID, publication.ID, types.MemoryReviewStatusRevoked)

	var historyCount int64
	require.NoError(t, db.Model(&types.MemoryReviewHistory{}).Where("review_task_id = ?", review.ID).Count(&historyCount).Error)
	require.Equal(t, int64(6), historyCount, "approve, start, fail, restart, complete, and revoke are all audited")
	var history types.MemoryReviewHistory
	require.NoError(t, db.Where("review_task_id = ?", review.ID).First(&history).Error)
	require.Equal(t, "user-1", history.UserID)
}

func TestExternalMemoryRepositoryRevocationRetriesStaleLifecycleCAS(t *testing.T) {
	repo, db := newExternalMemoryTestRepository(t)
	ctx := context.Background()
	event, snapshot, review, publication := externalMemoryProjectionFixture("evt-revoke-retry-source", 7, "team-a", "binding-a", "memory-revoke-retry", 1)
	_, _, err := repo.CreateMaturedMemoryProjection(ctx, event, snapshot, review, publication)
	require.NoError(t, err)

	injected := false
	callbackName := "test:external_memory_revoke_stale_cas"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if injected || tx.Statement == nil || tx.Statement.Table != "memory_review_tasks" {
			return
		}
		injected = true
		tx.Session(&gorm.Session{SkipHooks: true}).Exec(
			"UPDATE memory_review_tasks SET lock_version = lock_version + 1 WHERE id = ?", review.ID,
		)
	}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callbackName) })

	revocation := *event
	revocation.ID = uuid.NewString()
	revocation.EventID = "evt-revoke-retry"
	revocation.EventType = types.MemoryL3EventRevoked
	revocation.EventClass = types.MemoryIntegrationEventClassRevocation
	revocation.OccurredAt = event.OccurredAt.Add(time.Second)
	revocation.CreatedAt = time.Time{}
	revocation.UpdatedAt = time.Time{}
	projection, duplicate, err := repo.RevokeMemoryProjection(ctx, &revocation)
	require.NoError(t, err)
	require.False(t, duplicate)
	require.True(t, injected, "the test must force the first review CAS to lose")
	require.Equal(t, types.MemoryReviewStatusRevoked, projection.ReviewTask.Status)
	require.Equal(t, types.MemoryReviewStatusRevoked, projection.Publication.Status)

	var eventCount int64
	require.NoError(t, db.Model(&types.MemoryIntegrationEvent{}).Where("event_id = ?", revocation.EventID).Count(&eventCount).Error)
	require.Equal(t, int64(1), eventCount, "the rolled-back first attempt must not duplicate the durable event")
	var historyCount int64
	require.NoError(t, db.Model(&types.MemoryReviewHistory{}).
		Where("review_task_id = ? AND to_status = ?", review.ID, types.MemoryReviewStatusRevoked).
		Count(&historyCount).Error)
	require.Equal(t, int64(1), historyCount)
}

func TestExternalMemoryRepositoryCompleteRejectsMissingRevision(t *testing.T) {
	repo, _ := newExternalMemoryTestRepository(t)
	ctx := context.Background()
	event, snapshot, review, publication := externalMemoryProjectionFixture("evt-missing-revision", 7, "team-a", "binding-a", "memory-a", 1)
	_, _, err := repo.CreateMaturedMemoryProjection(ctx, event, snapshot, review, publication)
	require.NoError(t, err)
	key := snapshot.ProjectionKey()
	_, err = repo.TransitionMemoryReview(ctx, key, review.ID, types.MemoryReviewStatusPendingReview, types.MemoryReviewStatusApproved, "reviewer-1", "approved")
	require.NoError(t, err)
	_, err = repo.StartMemoryWikiPublishing(ctx, key, publication.ID, "kb-team-a")
	require.NoError(t, err)

	_, err = repo.CompleteMemoryWikiPublishing(ctx, key, publication.ID, types.MemoryWikiPublishResult{
		KnowledgeBaseID: "kb-team-a",
		WikiPageID:      "page-1",
		WikiRevisionID:  "mwr_missing",
		WikiPageVersion: 1,
		PublishedAt:     time.Now().UTC(),
	})
	require.ErrorIs(t, err, ErrMemoryWikiRevisionNotFound)
}

func TestExternalMemoryRepositoryCompleteRejectsRevisionFromDifferentLifecycle(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*types.MemoryWikiRevision)
	}{
		{name: "memory version", mutate: func(revision *types.MemoryWikiRevision) { revision.MemoryVersion++ }},
		{name: "publication", mutate: func(revision *types.MemoryWikiRevision) { revision.SourcePublicationID = "publication-other" }},
		{name: "review task", mutate: func(revision *types.MemoryWikiRevision) { revision.SourceReviewTaskID = "review-other" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo, _ := newExternalMemoryTestRepository(t)
			ctx := context.Background()
			event, snapshot, review, publication := externalMemoryProjectionFixture("evt-revision-scope-"+strings.ReplaceAll(test.name, " ", "-"), 7, "team-a", "binding-a", "memory-a", 4)
			_, _, err := repo.CreateMaturedMemoryProjection(ctx, event, snapshot, review, publication)
			require.NoError(t, err)
			key := snapshot.ProjectionKey()
			_, err = repo.TransitionMemoryReview(ctx, key, review.ID, types.MemoryReviewStatusPendingReview, types.MemoryReviewStatusApproved, "reviewer-1", "approved")
			require.NoError(t, err)
			_, err = repo.StartMemoryWikiPublishing(ctx, key, publication.ID, "kb-team-a")
			require.NoError(t, err)

			revision := externalMemoryRevisionFixture(uuid.NewString(), "page-1", snapshot.ContentChecksum, "published body")
			revision.TeamID = publication.TeamID
			revision.BindingID = publication.BindingID
			revision.UserID = publication.UserID
			revision.MemoryID = publication.MemoryID
			revision.MemoryVersion = publication.MemoryVersion
			revision.SourcePublicationID = publication.ID
			revision.SourceReviewTaskID = review.ID
			revision.WikiPageVersion = 1
			test.mutate(revision)
			_, _, err = repo.CreateMemoryWikiRevision(ctx, revision)
			require.NoError(t, err)

			_, err = repo.CompleteMemoryWikiPublishing(ctx, key, publication.ID, types.MemoryWikiPublishResult{
				KnowledgeBaseID: "kb-team-a", WikiPageID: "page-1", WikiRevisionID: revision.ID,
				WikiPageVersion: 1, PublishedAt: time.Now().UTC(),
			})
			require.ErrorIs(t, err, ErrMemoryWikiRevisionConflict)
		})
	}
}

func requireMemoryWorkflowStatus(t *testing.T, db *gorm.DB, reviewID, publicationID, status string) {
	t.Helper()
	var review types.MemoryReviewTask
	require.NoError(t, db.First(&review, "id = ?", reviewID).Error)
	require.Equal(t, status, review.Status)
	var publication types.MemoryWikiPublication
	require.NoError(t, db.First(&publication, "id = ?", publicationID).Error)
	require.Equal(t, status, publication.Status)
}
