package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrExternalMemoryNotFound           = errors.New("external memory projection not found")
	ErrExternalMemoryEventConflict      = errors.New("external memory event id conflicts with stored event")
	ErrExternalMemoryStateConflict      = errors.New("external memory state conflict")
	ErrExternalMemoryConcurrentMutation = errors.New("external memory changed concurrently; retry the event")
	ErrMemoryWikiRevisionNotFound       = errors.New("memory Wiki revision not found")
	ErrMemoryWikiRevisionConflict       = errors.New("memory Wiki revision is immutable and conflicts with stored content")
	errExternalMemoryRetryableCAS       = errors.New("retryable external memory compare-and-swap conflict")
)

type externalMemoryRepository struct{ db *gorm.DB }

func NewExternalMemoryRepository(db *gorm.DB) interfaces.ExternalMemoryRepository {
	return &externalMemoryRepository{db: db}
}

func (r *externalMemoryRepository) CreateMaturedMemoryProjection(
	ctx context.Context,
	event *types.MemoryIntegrationEvent,
	snapshot *types.MemoryL3Snapshot,
	review *types.MemoryReviewTask,
	publication *types.MemoryWikiPublication,
) (projection *interfaces.ExternalMemoryProjection, duplicate bool, err error) {
	if event == nil || snapshot == nil || review == nil || publication == nil {
		return nil, false, errors.New("event, snapshot, review, and publication are required")
	}
	if event.EventClass == "" {
		event.EventClass = types.MemoryIntegrationEventClassProjection
	}
	if event.EventClass != types.MemoryIntegrationEventClassProjection {
		return nil, false, errors.New("matured memory event must use projection event class")
	}
	key := event.ProjectionKey()
	if snapshot.ProjectionKey() != key || review.ProjectionKey() != key || publicationProjectionKey(publication) != key ||
		!projectionScopesMatch(event, snapshot, review, publication) {
		return nil, false, errors.New("external memory projection scopes do not match")
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		storedEvent, found, loadErr := getIntegrationEventByEventID(tx, event.EventID)
		if loadErr != nil {
			return loadErr
		}
		if found {
			projection, loadErr = loadMatchingMemoryProjection(tx, storedEvent, event, snapshot, true)
			duplicate = true
			return loadErr
		}

		storedEvent, found, loadErr = getIntegrationEventByProjection(tx, key)
		if loadErr != nil {
			return loadErr
		}
		if found {
			projection, loadErr = loadMatchingMemoryProjection(tx, storedEvent, event, snapshot, false)
			duplicate = true
			return loadErr
		}

		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(event)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			storedEvent, found, loadErr = getIntegrationEventByEventID(tx, event.EventID)
			if loadErr != nil {
				return loadErr
			}
			if found {
				projection, loadErr = loadMatchingMemoryProjection(tx, storedEvent, event, snapshot, true)
				duplicate = true
				return loadErr
			}
			storedEvent, found, loadErr = getIntegrationEventByProjection(tx, key)
			if loadErr != nil {
				return loadErr
			}
			if !found {
				return ErrExternalMemoryEventConflict
			}
			projection, loadErr = loadMatchingMemoryProjection(tx, storedEvent, event, snapshot, false)
			duplicate = true
			return loadErr
		}

		if err := tx.Create(snapshot).Error; err != nil {
			return err
		}
		if err := tx.Create(review).Error; err != nil {
			return err
		}
		if err := tx.Create(publication).Error; err != nil {
			return err
		}
		projection = &interfaces.ExternalMemoryProjection{Event: event, Snapshot: snapshot, ReviewTask: review, Publication: publication}
		return nil
	})
	return projection, duplicate, err
}

func (r *externalMemoryRepository) RevokeMemoryProjection(
	ctx context.Context,
	event *types.MemoryIntegrationEvent,
) (projection *interfaces.ExternalMemoryProjection, duplicate bool, err error) {
	if event == nil || event.EventID == "" || event.EventType != types.MemoryL3EventRevoked ||
		event.EventClass != types.MemoryIntegrationEventClassRevocation {
		return nil, false, errors.New("valid memory revocation event is required")
	}
	key := event.ProjectionKey()
	// Publishing has a finite status chain (approved -> publishing ->
	// published). If one of those commits after our read but before our CAS,
	// reload the newest durable state and apply the same source revocation.
	// Exhaustion remains retryable to the signed-event outbox (503), rather
	// than being misclassified as a permanent semantic conflict (409).
	for attempt := 0; attempt < 4; attempt++ {
		projection = nil
		duplicate = false
		err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			stored, found, loadErr := getIntegrationEventByEventID(tx, event.EventID)
			if loadErr != nil {
				return loadErr
			}
			if found {
				if !sameIntegrationEvent(stored, event) || stored.EventClass != types.MemoryIntegrationEventClassRevocation {
					return ErrExternalMemoryEventConflict
				}
				projection, loadErr = loadMemoryProjection(tx, key)
				if loadErr != nil {
					return loadErr
				}
				if projection.ReviewTask.Status != types.MemoryReviewStatusRevoked || projection.Publication.Status != types.MemoryReviewStatusRevoked {
					return ErrExternalMemoryStateConflict
				}
				duplicate = true
				return nil
			}

			stored, found, loadErr = getIntegrationEventByProjectionClass(tx, key, types.MemoryIntegrationEventClassRevocation)
			if loadErr != nil {
				return loadErr
			}
			if found {
				return ErrExternalMemoryEventConflict
			}

			projection, loadErr = loadMemoryProjection(tx, key)
			if loadErr != nil {
				return loadErr
			}
			// Refresh and lock the mutable pair in the same order used by the
			// transition writes. PostgreSQL therefore serializes completion and
			// revocation; SQLite still relies on the CAS retry below.
			var lockedReview types.MemoryReviewTask
			if loadErr = scopedProjectionQuery(tx.Clauses(clause.Locking{Strength: "UPDATE"}), key).
				Where("id = ?", projection.ReviewTask.ID).First(&lockedReview).Error; loadErr != nil {
				return loadErr
			}
			var lockedPublication types.MemoryWikiPublication
			if loadErr = scopedProjectionQuery(tx.Clauses(clause.Locking{Strength: "UPDATE"}), key).
				Where("id = ?", projection.Publication.ID).First(&lockedPublication).Error; loadErr != nil {
				return loadErr
			}
			projection.ReviewTask = &lockedReview
			projection.Publication = &lockedPublication
			if !projectionScopesMatch(projection.Event, projection.Snapshot, projection.ReviewTask, projection.Publication) {
				return ErrExternalMemoryStateConflict
			}
			if !revocationScopesMatchProjection(event, projection) {
				return ErrExternalMemoryEventConflict
			}
			if !legalMemoryReviewTransition(projection.ReviewTask.Status, types.MemoryReviewStatusRevoked) {
				return ErrExternalMemoryStateConflict
			}

			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(event)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrExternalMemoryEventConflict
			}

			now := time.Now().UTC()
			reviewResult := scopedProjectionQuery(tx.Model(&types.MemoryReviewTask{}), key).
				Where("id = ? AND status = ? AND lock_version = ?", projection.ReviewTask.ID, projection.ReviewTask.Status, projection.ReviewTask.LockVersion).
				Updates(map[string]any{
					"status": types.MemoryReviewStatusRevoked, "reviewer_id": "system:memory-core",
					"review_comment": "source memory revoked by " + event.EventID, "reviewed_at": now,
					"lock_version": gorm.Expr("lock_version + 1"), "updated_at": now,
				})
			if reviewResult.Error != nil {
				return reviewResult.Error
			}
			if reviewResult.RowsAffected != 1 {
				return errExternalMemoryRetryableCAS
			}
			publicationResult := scopedProjectionQuery(tx.Model(&types.MemoryWikiPublication{}), key).
				Where("id = ? AND status = ? AND lock_version = ?", projection.Publication.ID, projection.Publication.Status, projection.Publication.LockVersion).
				Updates(map[string]any{
					"status": types.MemoryReviewStatusRevoked, "reviewed_by": "system:memory-core",
					"review_comment": "source memory revoked by " + event.EventID, "reviewed_at": now,
					"failed_stage": "", "last_error": "",
					"lock_version": gorm.Expr("lock_version + 1"), "updated_at": now,
				})
			if publicationResult.Error != nil {
				return publicationResult.Error
			}
			if publicationResult.RowsAffected != 1 {
				return errExternalMemoryRetryableCAS
			}
			if createErr := tx.Create(&types.MemoryReviewHistory{
				ID: uuid.NewString(), ReviewTaskID: projection.ReviewTask.ID, TenantID: key.TenantID,
				TeamID: key.TeamID, BindingID: key.BindingID, UserID: projection.Snapshot.UserID,
				MemoryID: key.MemoryID, MemoryVersion: key.MemoryVersion, ContentChecksum: projection.Snapshot.ContentChecksum,
				FromStatus: projection.ReviewTask.Status, ToStatus: types.MemoryReviewStatusRevoked,
				ActorID: "system:memory-core", Comment: "source memory revoked by " + event.EventID, CreatedAt: now,
			}).Error; createErr != nil {
				return createErr
			}
			projection, loadErr = loadMemoryProjection(tx, key)
			return loadErr
		})
		if !errors.Is(err, errExternalMemoryRetryableCAS) {
			return projection, duplicate, err
		}
	}
	return nil, false, ErrExternalMemoryConcurrentMutation
}

func (r *externalMemoryRepository) GetMemoryProjection(ctx context.Context, key types.MemoryProjectionKey) (*interfaces.ExternalMemoryProjection, error) {
	return loadMemoryProjection(r.db.WithContext(ctx), key)
}

func (r *externalMemoryRepository) CreateMemoryWikiRevision(ctx context.Context, revision *types.MemoryWikiRevision) (*types.MemoryWikiRevision, bool, error) {
	if revision == nil || revision.ID == "" || revision.TenantID == 0 || revision.WikiPageID == "" ||
		revision.ContentChecksum == "" || revision.ProjectionChecksum == "" {
		return nil, false, errors.New("revision id, tenant, page, source checksum, and projection checksum are required")
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(revision)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 1 {
		return revision, false, nil
	}
	stored, err := r.GetMemoryWikiRevision(ctx, revision.TenantID, revision.ID)
	if err != nil {
		if errors.Is(err, ErrMemoryWikiRevisionNotFound) {
			return nil, false, ErrMemoryWikiRevisionConflict
		}
		return nil, false, err
	}
	if !sameMemoryWikiRevision(stored, revision) {
		return nil, false, ErrMemoryWikiRevisionConflict
	}
	return stored, true, nil
}

func (r *externalMemoryRepository) GetMemoryWikiRevision(ctx context.Context, tenantID uint64, revisionID string) (*types.MemoryWikiRevision, error) {
	var revision types.MemoryWikiRevision
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, revisionID).First(&revision).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMemoryWikiRevisionNotFound
	}
	return &revision, err
}

func (r *externalMemoryRepository) GetMemoryWikiRevisionByPageChecksum(ctx context.Context, tenantID uint64, pageID, checksum string) (*types.MemoryWikiRevision, error) {
	var revision types.MemoryWikiRevision
	err := r.db.WithContext(ctx).Where(
		"tenant_id = ? AND wiki_page_id = ? AND projection_checksum = ?", tenantID, pageID, checksum,
	).Order("wiki_page_version DESC").First(&revision).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMemoryWikiRevisionNotFound
	}
	return &revision, err
}

func (r *externalMemoryRepository) ListMemoryWikiPublicationsByRevision(ctx context.Context, tenantID uint64, revisionID string) ([]*types.MemoryWikiPublication, error) {
	var publications []*types.MemoryWikiPublication
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND wiki_revision_id = ?", tenantID, revisionID).
		Order("memory_version ASC").Find(&publications).Error
	return publications, err
}

func (r *externalMemoryRepository) TransitionMemoryReview(
	ctx context.Context,
	key types.MemoryProjectionKey,
	reviewTaskID string,
	expectedStatus string,
	targetStatus string,
	actorID string,
	comment string,
) (updated *types.MemoryReviewTask, err error) {
	if !legalMemoryReviewTransition(expectedStatus, targetStatus) {
		return nil, ErrExternalMemoryStateConflict
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task types.MemoryReviewTask
		query := scopedProjectionQuery(tx, key).Where("id = ?", reviewTaskID)
		if findErr := query.First(&task).Error; findErr != nil {
			if errors.Is(findErr, gorm.ErrRecordNotFound) {
				return ErrExternalMemoryNotFound
			}
			return findErr
		}
		if task.Status != expectedStatus {
			return ErrExternalMemoryStateConflict
		}

		now := time.Now().UTC()
		result := scopedProjectionQuery(tx.Model(&types.MemoryReviewTask{}), key).
			Where("id = ? AND status = ? AND lock_version = ?", task.ID, expectedStatus, task.LockVersion).
			Updates(map[string]any{
				"status": targetStatus, "reviewer_id": actorID, "review_comment": comment,
				"reviewed_at": now, "lock_version": gorm.Expr("lock_version + 1"), "updated_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrExternalMemoryStateConflict
		}

		publicationResult := scopedProjectionQuery(tx.Model(&types.MemoryWikiPublication{}), key).
			Where("review_task_id = ? AND status = ?", task.ID, expectedStatus).
			Updates(map[string]any{
				"status": targetStatus, "reviewed_by": actorID, "review_comment": comment,
				"reviewed_at": now, "lock_version": gorm.Expr("lock_version + 1"), "updated_at": now,
			})
		if publicationResult.Error != nil {
			return publicationResult.Error
		}
		if publicationResult.RowsAffected != 1 {
			return ErrExternalMemoryStateConflict
		}

		history := &types.MemoryReviewHistory{
			ID: uuid.NewString(), ReviewTaskID: task.ID, TenantID: task.TenantID,
			TeamID: task.TeamID, BindingID: task.BindingID, UserID: task.UserID, MemoryID: task.MemoryID,
			MemoryVersion: task.MemoryVersion, ContentChecksum: task.ContentChecksum,
			FromStatus: expectedStatus, ToStatus: targetStatus, ActorID: actorID,
			Comment: comment, CreatedAt: now,
		}
		if createErr := tx.Create(history).Error; createErr != nil {
			return createErr
		}
		if findErr := tx.First(&task, "id = ?", task.ID).Error; findErr != nil {
			return findErr
		}
		updated = &task
		return nil
	})
	return updated, err
}

func (r *externalMemoryRepository) StartMemoryWikiPublishing(
	ctx context.Context,
	key types.MemoryProjectionKey,
	publicationID string,
	knowledgeBaseID string,
) (*types.MemoryWikiPublication, error) {
	var publication types.MemoryWikiPublication
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := scopedProjectionQuery(tx, key).Where("id = ?", publicationID).First(&publication).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrExternalMemoryNotFound
			}
			return err
		}
		if publication.KnowledgeBaseID != "" && publication.KnowledgeBaseID != knowledgeBaseID {
			return ErrExternalMemoryStateConflict
		}
		if publication.Status == types.MemoryReviewStatusPublishing {
			var task types.MemoryReviewTask
			if err := scopedProjectionQuery(tx, key).Where("id = ?", publication.ReviewTaskID).First(&task).Error; err != nil {
				return err
			}
			if task.Status != types.MemoryReviewStatusPublishing {
				return ErrExternalMemoryStateConflict
			}
			return nil
		}
		if publication.Status != types.MemoryReviewStatusApproved {
			return ErrExternalMemoryStateConflict
		}
		now := time.Now().UTC()
		result := scopedProjectionQuery(tx.Model(&types.MemoryWikiPublication{}), key).
			Where("id = ? AND status = ? AND lock_version = ?", publication.ID, types.MemoryReviewStatusApproved, publication.LockVersion).
			Updates(map[string]any{
				"status": types.MemoryReviewStatusPublishing, "knowledge_base_id": knowledgeBaseID,
				"failed_stage": "", "last_error": "", "publish_attempt_count": gorm.Expr("publish_attempt_count + 1"),
				"lock_version": gorm.Expr("lock_version + 1"), "updated_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrExternalMemoryStateConflict
		}
		if err := transitionPublicationReviewTask(
			tx, key, publication.ReviewTaskID,
			types.MemoryReviewStatusApproved, types.MemoryReviewStatusPublishing,
			"system:memory-wiki-publisher", "publishing to "+knowledgeBaseID, now,
		); err != nil {
			return err
		}
		if result := scopedProjectionQuery(tx.Model(&types.MemoryReviewTask{}), key).
			Where("id = ?", publication.ReviewTaskID).
			Update("target_knowledge_base_id", knowledgeBaseID); result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return result.Error
			}
			return ErrExternalMemoryStateConflict
		}
		return scopedProjectionQuery(tx, key).Where("id = ?", publication.ID).First(&publication).Error
	})
	return &publication, err
}

func (r *externalMemoryRepository) FailMemoryWikiPublishing(
	ctx context.Context,
	key types.MemoryProjectionKey,
	publicationID string,
	failedStage string,
	lastError string,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var publication types.MemoryWikiPublication
		if err := scopedProjectionQuery(tx, key).Where("id = ?", publicationID).First(&publication).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrExternalMemoryNotFound
			}
			return err
		}
		if publication.Status != types.MemoryReviewStatusPublishing {
			return ErrExternalMemoryStateConflict
		}
		now := time.Now().UTC()
		if err := transitionPublicationReviewTask(
			tx, key, publication.ReviewTaskID,
			types.MemoryReviewStatusPublishing, types.MemoryReviewStatusApproved,
			"system:memory-wiki-publisher", failedStage+": "+lastError, now,
		); err != nil {
			return err
		}
		result := scopedProjectionQuery(tx.Model(&types.MemoryWikiPublication{}), key).
			Where("id = ? AND status = ? AND lock_version = ?", publicationID, types.MemoryReviewStatusPublishing, publication.LockVersion).
			Updates(map[string]any{
				"status": types.MemoryReviewStatusApproved, "failed_stage": failedStage, "last_error": lastError,
				"lock_version": gorm.Expr("lock_version + 1"), "updated_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrExternalMemoryStateConflict
		}
		return nil
	})
}

func (r *externalMemoryRepository) CompleteMemoryWikiPublishing(
	ctx context.Context,
	key types.MemoryProjectionKey,
	publicationID string,
	publishResult types.MemoryWikiPublishResult,
) (published *types.MemoryWikiPublication, err error) {
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var publication types.MemoryWikiPublication
		if findErr := scopedProjectionQuery(tx, key).Where("id = ?", publicationID).First(&publication).Error; findErr != nil {
			if errors.Is(findErr, gorm.ErrRecordNotFound) {
				return ErrExternalMemoryNotFound
			}
			return findErr
		}
		if publication.Status == types.MemoryReviewStatusPublished {
			if publication.KnowledgeBaseID != publishResult.KnowledgeBaseID ||
				publication.PublishedPageID != publishResult.WikiPageID ||
				publication.WikiRevisionID != publishResult.WikiRevisionID ||
				publication.WikiPageVersion != publishResult.WikiPageVersion {
				return ErrExternalMemoryStateConflict
			}
			published = &publication
			return nil
		}
		if publication.Status != types.MemoryReviewStatusPublishing {
			return ErrExternalMemoryStateConflict
		}
		var revision types.MemoryWikiRevision
		if findErr := tx.Where("tenant_id = ? AND id = ?", publication.TenantID, publishResult.WikiRevisionID).First(&revision).Error; findErr != nil {
			if errors.Is(findErr, gorm.ErrRecordNotFound) {
				return ErrMemoryWikiRevisionNotFound
			}
			return findErr
		}
		if revision.TeamID != publication.TeamID ||
			revision.BindingID != publication.BindingID ||
			revision.UserID != publication.UserID ||
			revision.MemoryID != publication.MemoryID ||
			revision.MemoryVersion != publication.MemoryVersion ||
			revision.SourcePublicationID != publication.ID ||
			revision.SourceReviewTaskID != publication.ReviewTaskID ||
			revision.ContentChecksum != publication.ContentChecksum ||
			revision.KnowledgeBaseID != publishResult.KnowledgeBaseID ||
			revision.WikiPageID != publishResult.WikiPageID ||
			revision.WikiPageVersion != publishResult.WikiPageVersion {
			return ErrMemoryWikiRevisionConflict
		}

		for i := range publishResult.ClaimEvidence {
			claim := publishResult.ClaimEvidence[i]
			claim.PublicationID = publication.ID
			claim.TenantID = publication.TenantID
			claim.TeamID = publication.TeamID
			claim.BindingID = publication.BindingID
			claim.UserID = publication.UserID
			claim.MemoryID = publication.MemoryID
			claim.MemoryVersion = publication.MemoryVersion
			claim.WikiPageID = publishResult.WikiPageID
			claim.WikiRevisionID = publishResult.WikiRevisionID
			if claim.ID == "" {
				claim.ID = uuid.NewString()
			}
			if claim.CreatedAt.IsZero() {
				claim.CreatedAt = publishResult.PublishedAt
			}
			if createErr := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim).Error; createErr != nil {
				return createErr
			}
		}
		if err := transitionPublicationReviewTask(
			tx, key, publication.ReviewTaskID,
			types.MemoryReviewStatusPublishing, types.MemoryReviewStatusPublished,
			"system:memory-wiki-publisher", "published "+publishResult.WikiRevisionID, publishResult.PublishedAt,
		); err != nil {
			return err
		}

		result := scopedProjectionQuery(tx.Model(&types.MemoryWikiPublication{}), key).
			Where("id = ? AND status = ? AND lock_version = ?", publication.ID, types.MemoryReviewStatusPublishing, publication.LockVersion).
			Updates(map[string]any{
				"status":            types.MemoryReviewStatusPublished,
				"knowledge_base_id": publishResult.KnowledgeBaseID,
				"published_page_id": publishResult.WikiPageID,
				"wiki_revision_id":  publishResult.WikiRevisionID,
				"wiki_page_version": publishResult.WikiPageVersion,
				"published_at":      publishResult.PublishedAt,
				"failed_stage":      "", "last_error": "",
				"lock_version": gorm.Expr("lock_version + 1"), "updated_at": publishResult.PublishedAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrExternalMemoryStateConflict
		}
		if findErr := tx.First(&publication, "id = ?", publication.ID).Error; findErr != nil {
			return findErr
		}
		published = &publication
		return nil
	})
	return published, err
}

func transitionPublicationReviewTask(
	tx *gorm.DB,
	key types.MemoryProjectionKey,
	reviewTaskID string,
	expectedStatus string,
	targetStatus string,
	actorID string,
	comment string,
	now time.Time,
) error {
	var task types.MemoryReviewTask
	if err := scopedProjectionQuery(tx, key).Where("id = ?", reviewTaskID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrExternalMemoryNotFound
		}
		return err
	}
	if task.Status != expectedStatus {
		return ErrExternalMemoryStateConflict
	}
	result := scopedProjectionQuery(tx.Model(&types.MemoryReviewTask{}), key).
		Where("id = ? AND status = ? AND lock_version = ?", task.ID, expectedStatus, task.LockVersion).
		Updates(map[string]any{
			"status": targetStatus, "lock_version": gorm.Expr("lock_version + 1"), "updated_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrExternalMemoryStateConflict
	}
	return tx.Create(&types.MemoryReviewHistory{
		ID: uuid.NewString(), ReviewTaskID: task.ID, TenantID: task.TenantID,
		TeamID: task.TeamID, BindingID: task.BindingID, UserID: task.UserID, MemoryID: task.MemoryID,
		MemoryVersion: task.MemoryVersion, ContentChecksum: task.ContentChecksum,
		FromStatus: expectedStatus, ToStatus: targetStatus, ActorID: actorID,
		Comment: comment, CreatedAt: now,
	}).Error
}

func publicationProjectionKey(p *types.MemoryWikiPublication) types.MemoryProjectionKey {
	return types.MemoryProjectionKey{TenantID: p.TenantID, TeamID: p.TeamID, BindingID: p.BindingID, MemoryID: p.MemoryID, MemoryVersion: p.MemoryVersion}
}

func scopedProjectionQuery(db *gorm.DB, key types.MemoryProjectionKey) *gorm.DB {
	return db.Where(
		"tenant_id = ? AND team_id = ? AND binding_id = ? AND memory_id = ? AND memory_version = ?",
		key.TenantID, key.TeamID, key.BindingID, key.MemoryID, key.MemoryVersion,
	)
}

func getIntegrationEventByEventID(db *gorm.DB, eventID string) (*types.MemoryIntegrationEvent, bool, error) {
	var event types.MemoryIntegrationEvent
	err := db.Where("event_id = ?", eventID).First(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return &event, err == nil, err
}

func getIntegrationEventByProjection(db *gorm.DB, key types.MemoryProjectionKey) (*types.MemoryIntegrationEvent, bool, error) {
	return getIntegrationEventByProjectionClass(db, key, types.MemoryIntegrationEventClassProjection)
}

func getIntegrationEventByProjectionClass(db *gorm.DB, key types.MemoryProjectionKey, eventClass string) (*types.MemoryIntegrationEvent, bool, error) {
	var event types.MemoryIntegrationEvent
	err := scopedProjectionQuery(db, key).Where("event_class = ?", eventClass).First(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return &event, err == nil, err
}

func getSnapshotByProjection(db *gorm.DB, key types.MemoryProjectionKey) (*types.MemoryL3Snapshot, bool, error) {
	var snapshot types.MemoryL3Snapshot
	err := scopedProjectionQuery(db, key).First(&snapshot).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return &snapshot, err == nil, err
}

func loadMemoryProjection(db *gorm.DB, key types.MemoryProjectionKey) (*interfaces.ExternalMemoryProjection, error) {
	snapshot, found, err := getSnapshotByProjection(db, key)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrExternalMemoryNotFound
	}
	var event types.MemoryIntegrationEvent
	if err := db.Where("event_id = ?", snapshot.EventID).First(&event).Error; err != nil {
		return nil, fmt.Errorf("load memory integration event: %w", err)
	}
	var review types.MemoryReviewTask
	if err := scopedProjectionQuery(db, key).First(&review).Error; err != nil {
		return nil, fmt.Errorf("load memory review task: %w", err)
	}
	var publication types.MemoryWikiPublication
	if err := scopedProjectionQuery(db, key).First(&publication).Error; err != nil {
		return nil, fmt.Errorf("load memory wiki publication: %w", err)
	}
	if !projectionScopesMatch(&event, snapshot, &review, &publication) {
		return nil, ErrExternalMemoryStateConflict
	}
	return &interfaces.ExternalMemoryProjection{Event: &event, Snapshot: snapshot, ReviewTask: &review, Publication: &publication}, nil
}

func projectionScopesMatch(
	event *types.MemoryIntegrationEvent,
	snapshot *types.MemoryL3Snapshot,
	review *types.MemoryReviewTask,
	publication *types.MemoryWikiPublication,
) bool {
	if event == nil || snapshot == nil || review == nil || publication == nil {
		return false
	}
	key := event.ProjectionKey()
	return snapshot.ProjectionKey() == key && review.ProjectionKey() == key && publicationProjectionKey(publication) == key &&
		snapshot.EventID == event.EventID && review.EventID == event.EventID && publication.EventID == event.EventID &&
		review.SnapshotID == snapshot.ID && publication.SnapshotID == snapshot.ID && publication.ReviewTaskID == review.ID &&
		snapshot.DepartmentID == event.DepartmentID && review.DepartmentID == event.DepartmentID && publication.DepartmentID == event.DepartmentID &&
		snapshot.WorkspaceID == event.WorkspaceID && review.WorkspaceID == event.WorkspaceID && publication.WorkspaceID == event.WorkspaceID &&
		snapshot.ProjectID == event.ProjectID && review.ProjectID == event.ProjectID && publication.ProjectID == event.ProjectID &&
		snapshot.UserID == event.UserID && review.UserID == event.UserID && publication.UserID == event.UserID &&
		snapshot.AgentID == event.AgentID && review.AgentID == event.AgentID && publication.AgentID == event.AgentID &&
		snapshot.TaskID == event.TaskID && review.TaskID == event.TaskID && publication.TaskID == event.TaskID &&
		snapshot.ContentChecksum == event.ContentChecksum && review.ContentChecksum == event.ContentChecksum && publication.ContentChecksum == event.ContentChecksum
}

func sameIntegrationEvent(a, b *types.MemoryIntegrationEvent) bool {
	return a.EventID == b.EventID && sameIntegrationEventPayload(a, b)
}

func sameIntegrationEventPayload(a, b *types.MemoryIntegrationEvent) bool {
	return a.EventType == b.EventType && a.EventClass == b.EventClass && a.SchemaVersion == b.SchemaVersion &&
		a.OccurredAt.Equal(b.OccurredAt) && a.ProjectionKey() == b.ProjectionKey() && a.DepartmentID == b.DepartmentID &&
		a.WorkspaceID == b.WorkspaceID && a.ProjectID == b.ProjectID && a.UserID == b.UserID && a.AgentID == b.AgentID &&
		a.TaskID == b.TaskID && a.ContentChecksum == b.ContentChecksum
}

func revocationScopesMatchProjection(event *types.MemoryIntegrationEvent, projection *interfaces.ExternalMemoryProjection) bool {
	if event == nil || projection == nil || projection.Snapshot == nil {
		return false
	}
	snapshot := projection.Snapshot
	return event.ProjectionKey() == snapshot.ProjectionKey() && event.DepartmentID == snapshot.DepartmentID &&
		event.WorkspaceID == snapshot.WorkspaceID && event.ProjectID == snapshot.ProjectID &&
		event.UserID == snapshot.UserID && event.AgentID == snapshot.AgentID && event.TaskID == snapshot.TaskID &&
		event.ContentChecksum == snapshot.ContentChecksum
}

func loadMatchingMemoryProjection(
	db *gorm.DB,
	storedEvent *types.MemoryIntegrationEvent,
	incomingEvent *types.MemoryIntegrationEvent,
	incomingSnapshot *types.MemoryL3Snapshot,
	requireSameEventID bool,
) (*interfaces.ExternalMemoryProjection, error) {
	matchesEvent := sameIntegrationEventPayload(storedEvent, incomingEvent)
	if requireSameEventID {
		matchesEvent = sameIntegrationEvent(storedEvent, incomingEvent)
	}
	if !matchesEvent {
		return nil, ErrExternalMemoryEventConflict
	}
	projection, err := loadMemoryProjection(db, storedEvent.ProjectionKey())
	if err != nil {
		return nil, err
	}
	if !sameMemorySnapshotPayload(projection.Snapshot, incomingSnapshot) {
		return nil, ErrExternalMemoryEventConflict
	}
	return projection, nil
}

func sameMemorySnapshotPayload(a, b *types.MemoryL3Snapshot) bool {
	return a.ProjectionKey() == b.ProjectionKey() && a.DepartmentID == b.DepartmentID &&
		a.WorkspaceID == b.WorkspaceID && a.ProjectID == b.ProjectID && a.UserID == b.UserID && a.AgentID == b.AgentID &&
		a.TaskID == b.TaskID && a.MemoryLevel == b.MemoryLevel && a.Maturity == b.Maturity &&
		a.Title == b.Title && a.Summary == b.Summary && a.ContentMarkdown == b.ContentMarkdown &&
		a.Confidence == b.Confidence && a.Sensitivity == b.Sensitivity &&
		a.ContentChecksum == b.ContentChecksum && reflect.DeepEqual(a.EvidenceRefs, b.EvidenceRefs) &&
		reflect.DeepEqual(a.Claims, b.Claims)
}

func sameMemoryWikiRevision(a, b *types.MemoryWikiRevision) bool {
	return a.ID == b.ID && a.TenantID == b.TenantID && a.TeamID == b.TeamID &&
		a.BindingID == b.BindingID && a.UserID == b.UserID && a.KnowledgeBaseID == b.KnowledgeBaseID &&
		a.WikiPageID == b.WikiPageID && a.WikiPageVersion == b.WikiPageVersion && a.PageSlug == b.PageSlug &&
		a.MemoryID == b.MemoryID && a.MemoryVersion == b.MemoryVersion &&
		a.SourcePublicationID == b.SourcePublicationID && a.SourceReviewTaskID == b.SourceReviewTaskID &&
		a.ContentChecksum == b.ContentChecksum && a.ProjectionChecksum == b.ProjectionChecksum && a.Title == b.Title && a.Summary == b.Summary &&
		a.Content == b.Content && a.PageType == b.PageType && a.PageStatus == b.PageStatus &&
		reflect.DeepEqual(a.SourceRefs, b.SourceRefs) && reflect.DeepEqual(a.ChunkRefs, b.ChunkRefs) &&
		sameJSON(a.PageMetadata, b.PageMetadata) && sameWikiPageSnapshot(a.PageSnapshot, b.PageSnapshot)
}

func sameWikiPageSnapshot(a, b types.JSON) bool {
	var left types.WikiPage
	var right types.WikiPage
	if json.Unmarshal(a, &left) != nil || json.Unmarshal(b, &right) != nil {
		return sameJSON(a, b)
	}
	// PostgreSQL persists timestamps at microsecond precision while the page
	// returned by Create can still carry Go nanoseconds. They are transport
	// metadata, not part of the immutable Wiki revision identity; all durable
	// content/provenance fields remain compared below and by the outer record.
	left.CreatedAt, right.CreatedAt = time.Time{}, time.Time{}
	left.UpdatedAt, right.UpdatedAt = time.Time{}, time.Time{}
	left.DeletedAt, right.DeletedAt = gorm.DeletedAt{}, gorm.DeletedAt{}
	if !sameJSON(left.PageMetadata, right.PageMetadata) {
		return false
	}
	left.PageMetadata, right.PageMetadata = nil, nil
	return reflect.DeepEqual(left, right)
}

func sameJSON(a, b types.JSON) bool {
	var left any
	var right any
	if json.Unmarshal(a, &left) != nil || json.Unmarshal(b, &right) != nil {
		return string(a) == string(b)
	}
	return reflect.DeepEqual(left, right)
}

func legalMemoryReviewTransition(from, to string) bool {
	switch from {
	case types.MemoryReviewStatusPendingReview:
		return to == types.MemoryReviewStatusApproved || to == types.MemoryReviewStatusChangesRequested ||
			to == types.MemoryReviewStatusRejected || to == types.MemoryReviewStatusRevoked
	case types.MemoryReviewStatusChangesRequested:
		return to == types.MemoryReviewStatusPendingReview || to == types.MemoryReviewStatusRevoked
	case types.MemoryReviewStatusApproved, types.MemoryReviewStatusPublishing, types.MemoryReviewStatusPublished:
		return to == types.MemoryReviewStatusRevoked
	case types.MemoryReviewStatusRejected:
		return to == types.MemoryReviewStatusRevoked
	default:
		return false
	}
}
