package service

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"reflect"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/justaboyhai-wq/fmind/internal/datasource"
	"github.com/justaboyhai-wq/fmind/internal/logger"
	"github.com/justaboyhai-wq/fmind/internal/tracing/langfuse"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	secutils "github.com/justaboyhai-wq/fmind/internal/utils"
)

// DataSourceService implements the DataSourceService interface
type DataSourceService struct {
	dsRepo            interfaces.DataSourceRepository
	syncLogRepo       interfaces.SyncLogRepository
	knowledgeService  interfaces.KnowledgeService
	kbService         interfaces.KnowledgeBaseService
	taskEnqueuer      interfaces.TaskEnqueuer
	connectorRegistry *datasource.ConnectorRegistry
	scheduler         *datasource.Scheduler
	tenantRepo        interfaces.TenantRepository
	tagService        interfaces.KnowledgeTagService
}

// NewDataSourceService creates a new data source service
func NewDataSourceService(
	dsRepo interfaces.DataSourceRepository,
	syncLogRepo interfaces.SyncLogRepository,
	knowledgeService interfaces.KnowledgeService,
	kbService interfaces.KnowledgeBaseService,
	taskEnqueuer interfaces.TaskEnqueuer,
	connectorRegistry *datasource.ConnectorRegistry,
	scheduler *datasource.Scheduler,
	tenantRepo interfaces.TenantRepository,
	tagService interfaces.KnowledgeTagService,
) interfaces.DataSourceService {
	return &DataSourceService{
		dsRepo:            dsRepo,
		syncLogRepo:       syncLogRepo,
		knowledgeService:  knowledgeService,
		kbService:         kbService,
		taskEnqueuer:      taskEnqueuer,
		connectorRegistry: connectorRegistry,
		scheduler:         scheduler,
		tenantRepo:        tenantRepo,
		tagService:        tagService,
	}
}

// CreateDataSource creates a new data source configuration
func (s *DataSourceService) CreateDataSource(ctx context.Context, ds *types.DataSource) (*types.DataSource, error) {
	if ds == nil {
		return nil, datasource.ErrDataSourceInvalid
	}

	// Validate knowledge base exists
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, ds.KnowledgeBaseID)
	if err != nil || kb == nil {
		return nil, datasource.ErrKnowledgeBaseNotFound
	}
	if kb.TenantID != ds.TenantID {
		return nil, datasource.ErrKnowledgeBaseNotFound
	}

	// Validate connector type
	_, err = s.connectorRegistry.Get(ds.Type)
	if err != nil {
		return nil, err
	}

	// Validate configuration
	if cfg, err := ds.ParseConfig(); err == nil && cfg != nil {
		cfg.StripNonSecretCredentials(ds.Type)
		if blob, err := cfg.ToJSON(); err == nil {
			ds.Config = blob
		}
	}
	if err := s.validateDataSourceConfig(ctx, ds); err != nil {
		return nil, err
	}

	// Create in database
	if err := s.dsRepo.Create(ctx, ds); err != nil {
		logger.Errorf(ctx, "failed to create data source: %v", err)
		return nil, err
	}

	// Register cron schedule if configured
	if ds.SyncSchedule != "" && ds.Status == types.DataSourceStatusActive {
		if err := s.scheduler.AddOrUpdate(ds); err != nil {
			logger.Warnf(ctx, "failed to register cron for ds=%s: %v", ds.ID, err)
		}
	}

	logger.Infof(ctx, "data source created: id=%s type=%s kb=%s", ds.ID, ds.Type, ds.KnowledgeBaseID)
	return ds, nil
}

// GetDataSource retrieves a data source by ID
func (s *DataSourceService) GetDataSource(ctx context.Context, id string) (*types.DataSource, error) {
	ds, err := s.dsRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return ds, nil
}

// ListDataSources lists all data sources for a knowledge base
func (s *DataSourceService) ListDataSources(ctx context.Context, kbID string) ([]*types.DataSource, error) {
	dataSources, err := s.dsRepo.FindByKnowledgeBase(ctx, kbID)
	if err != nil {
		logger.Errorf(ctx, "failed to list data sources: %v", err)
		return nil, err
	}

	// Attach latest sync log to each data source
	for _, ds := range dataSources {
		log, _ := s.syncLogRepo.FindLatest(ctx, ds.ID)
		if log != nil {
			ds.LatestSyncLog = log
		}
	}

	return dataSources, nil
}

// UpdateDataSource updates an existing data source
func (s *DataSourceService) UpdateDataSource(ctx context.Context, ds *types.DataSource) (*types.DataSource, error) {
	if ds == nil || ds.ID == "" {
		return nil, datasource.ErrDataSourceInvalid
	}

	// Verify data source exists
	existing, err := s.dsRepo.FindByID(ctx, ds.ID)
	if err != nil {
		return nil, err
	}

	if ds.KnowledgeBaseID == "" {
		ds.KnowledgeBaseID = existing.KnowledgeBaseID
	}
	if ds.KnowledgeBaseID != existing.KnowledgeBaseID {
		return nil, fmt.Errorf("changing knowledge base is not allowed")
	}

	if ds.TenantID == 0 {
		ds.TenantID = existing.TenantID
	}
	if ds.TenantID != existing.TenantID {
		return nil, datasource.ErrDataSourceInvalid
	}

	// Credentials NEVER flow through this endpoint — they live behind the
	// /credentials subresource. Force-preserve the stored credentials map
	// regardless of what the body says. Log a warning if a stale caller
	// passes one so we can spot them and migrate later. Non-credential
	// fields of Config (Type / ResourceIDs / Settings) flow through.
	var mergedCfg, existingParsedCfg *types.DataSourceConfig
	if len(ds.Config) > 0 {
		incomingCfg, parseIncErr := ds.ParseConfig()
		existingCfg, parseExErr := existing.ParseConfig()
		if parseIncErr == nil && parseExErr == nil && incomingCfg != nil {
			if incomingCfg.HasCredentials() {
				logger.Warnf(ctx,
					"deprecated: credentials in PUT /datasource/%s body are ignored; use PUT /credentials instead",
					secutils.SanitizeForLog(ds.ID))
			}
			merged := *incomingCfg
			if existingCfg != nil {
				merged.Credentials = existingCfg.Credentials
			} else {
				merged.Credentials = nil
			}
			merged.StripNonSecretCredentials(ds.Type)
			if blob, err := merged.ToJSON(); err == nil {
				ds.Config = blob
			}
			mergedCfg = &merged
			existingParsedCfg = existingCfg
		}
	}

	// Validate new configuration if non-credential fields changed. Skip
	// when there are no stored credentials yet (validators would fail with
	// no token to call the live API) and when the parsed config is
	// structurally identical.
	configActuallyChanged := true
	if mergedCfg != nil && existingParsedCfg != nil {
		configActuallyChanged = !reflect.DeepEqual(*mergedCfg, *existingParsedCfg)
	}
	hasCreds := mergedCfg != nil && mergedCfg.HasConfiguredCredentials(ds.Type)
	if hasCreds && (ds.Type != existing.Type || configActuallyChanged) {
		if err := s.validateDataSourceConfig(ctx, ds); err != nil {
			return nil, err
		}
	}

	if err := s.dsRepo.Update(ctx, ds); err != nil {
		logger.Errorf(ctx, "failed to update data source: %v", err)
		return nil, err
	}

	// Update cron schedule
	if err := s.scheduler.AddOrUpdate(ds); err != nil {
		logger.Warnf(ctx, "failed to update cron for ds=%s: %v", ds.ID, err)
	}

	logger.Infof(ctx, "data source updated: id=%s", ds.ID)
	return ds, nil
}

// UpdateDataSourceCredentials replaces the connector credential map. This is
// a single atomic write; the previous credential set is discarded entirely
// (callers cannot patch individual keys because half-configured connector
// auth is meaningless). After persisting, the live connection is validated
// so the caller learns immediately if the new credentials are wrong.
func (s *DataSourceService) UpdateDataSourceCredentials(
	ctx context.Context, id string, credentials map[string]interface{},
) (*types.DataSource, error) {
	if id == "" {
		return nil, datasource.ErrDataSourceInvalid
	}
	existing, err := s.dsRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	parsed, err := existing.ParseConfig()
	if err != nil {
		return nil, err
	}
	if parsed == nil {
		parsed = &types.DataSourceConfig{Type: existing.Type}
	}
	parsed.Credentials = credentials
	parsed.StripNonSecretCredentials(existing.Type)
	blob, err := parsed.ToJSON()
	if err != nil {
		return nil, err
	}
	existing.Config = blob

	// Run live validation now that the credentials are in place — surfaces
	// "wrong token" feedback immediately to the user instead of waiting for
	// the next scheduled sync.
	if err := s.validateDataSourceConfig(ctx, existing); err != nil {
		return nil, err
	}
	if err := s.dsRepo.Update(ctx, existing); err != nil {
		return nil, err
	}
	logger.Infof(ctx, "DataSource credentials updated: id=%s", secutils.SanitizeForLog(id))
	return existing, nil
}

// ClearDataSourceCredentials wipes the connector credential map without
// touching any other config field. Idempotent.
func (s *DataSourceService) ClearDataSourceCredentials(ctx context.Context, id string) error {
	if id == "" {
		return datasource.ErrDataSourceInvalid
	}
	existing, err := s.dsRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	parsed, err := existing.ParseConfig()
	if err != nil {
		return err
	}
	if parsed == nil {
		return nil
	}
	parsed.StripNonSecretCredentials(existing.Type)
	if !parsed.HasConfiguredCredentials(existing.Type) {
		blob, err := parsed.ToJSON()
		if err != nil {
			return err
		}
		existing.Config = blob
		return s.dsRepo.Update(ctx, existing)
	}
	parsed.Credentials = nil
	blob, err := parsed.ToJSON()
	if err != nil {
		return err
	}
	existing.Config = blob
	if err := s.dsRepo.Update(ctx, existing); err != nil {
		return err
	}
	logger.Infof(ctx, "DataSource credentials cleared by user: id=%s", secutils.SanitizeForLog(id))
	return nil
}

// DeleteDataSource deletes a data source (soft delete)
func (s *DataSourceService) DeleteDataSource(ctx context.Context, id string) error {
	// Verify data source exists
	_, err := s.dsRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.dsRepo.Delete(ctx, id); err != nil {
		logger.Errorf(ctx, "failed to delete data source: %v", err)
		return err
	}

	// Remove cron schedule
	s.scheduler.Remove(id)

	// Cancel any pending/running sync logs so queued asynq tasks won't retry
	if err := s.syncLogRepo.CancelPendingByDataSource(ctx, id); err != nil {
		logger.Warnf(ctx, "failed to cancel pending sync logs for ds=%s: %v", id, err)
	}

	logger.Infof(ctx, "data source deleted: id=%s", id)
	return nil
}

// ValidateConnection tests the connection to an external data source
func (s *DataSourceService) ValidateConnection(ctx context.Context, dsID string) error {
	ds, err := s.GetDataSource(ctx, dsID)
	if err != nil {
		return err
	}

	// Get connector
	connector, err := s.connectorRegistry.Get(ds.Type)
	if err != nil {
		return err
	}

	// Parse configuration
	config, err := ds.ParseConfig()
	if err != nil {
		return datasource.ErrInvalidConfig
	}

	// Validate connection
	if err := connector.Validate(ctx, config); err != nil {
		// Update data source with error
		ds.Status = types.DataSourceStatusError
		ds.ErrorMessage = err.Error()
		_ = s.dsRepo.Update(ctx, ds)
		return err
	}

	// Clear error if it was previously in error state
	if ds.Status == types.DataSourceStatusError {
		ds.Status = types.DataSourceStatusActive
		ds.ErrorMessage = ""
		_ = s.dsRepo.Update(ctx, ds)
	}

	return nil
}

// ListAvailableResources lists resources available for sync in the external system.
// parentID enables lazy (on-demand) loading of hierarchical resources: pass "" to
// list the top level, or a resource's ExternalID to list only its direct children.
func (s *DataSourceService) ListAvailableResources(
	ctx context.Context, dsID string, parentID string,
) ([]types.Resource, error) {
	ds, err := s.GetDataSource(ctx, dsID)
	if err != nil {
		return nil, err
	}

	// Get connector
	connector, err := s.connectorRegistry.Get(ds.Type)
	if err != nil {
		return nil, err
	}

	// Parse configuration
	config, err := ds.ParseConfig()
	if err != nil {
		return nil, datasource.ErrInvalidConfig
	}

	// List resources
	resources, err := connector.ListResources(ctx, config, parentID)
	if err != nil {
		logger.Errorf(ctx, "failed to list resources: %v", err)
		return nil, err
	}

	return resources, nil
}

// ResolveResourceAncestors resolves the ancestor ExternalIDs needed to reveal the
// given resources in a lazily-loaded picker (see the connector method for details).
func (s *DataSourceService) ResolveResourceAncestors(
	ctx context.Context, dsID string, resourceIDs []string,
) ([]string, error) {
	if len(resourceIDs) == 0 {
		return []string{}, nil
	}

	ds, err := s.GetDataSource(ctx, dsID)
	if err != nil {
		return nil, err
	}

	connector, err := s.connectorRegistry.Get(ds.Type)
	if err != nil {
		return nil, err
	}

	config, err := ds.ParseConfig()
	if err != nil {
		return nil, datasource.ErrInvalidConfig
	}

	ancestors, err := connector.ResolveResourceAncestors(ctx, config, resourceIDs)
	if err != nil {
		logger.Errorf(ctx, "failed to resolve resource ancestors: %v", err)
		return nil, err
	}

	return ancestors, nil
}

// ManualSync triggers an immediate sync for a data source
func (s *DataSourceService) ManualSync(ctx context.Context, dsID string) (*types.SyncLog, error) {
	ds, err := s.GetDataSource(ctx, dsID)
	if err != nil {
		return nil, err
	}

	if ds.Status != types.DataSourceStatusActive &&
		ds.Status != types.DataSourceStatusError &&
		ds.Status != types.DataSourceStatusPaused {
		return nil, datasource.ErrDataSourceNotActive
	}

	// Create sync log
	syncLog := &types.SyncLog{
		DataSourceID: dsID,
		TenantID:     ds.TenantID,
		Status:       types.SyncLogStatusRunning,
		StartedAt:    time.Now().UTC(),
	}

	if err := s.syncLogRepo.Create(ctx, syncLog); err != nil {
		logger.Errorf(ctx, "failed to create sync log: %v", err)
		return nil, err
	}

	// Enqueue sync task
	payload := &types.DataSourceSyncPayload{
		DataSourceID: dsID,
		TenantID:     ds.TenantID,
		SyncLogID:    syncLog.ID,
		ForceFull:    false,
	}
	langfuse.InjectTracing(ctx, payload)

	payloadJSON, _ := json.Marshal(payload)
	task := asynq.NewTask(types.TypeDataSourceSync, payloadJSON,
		asynq.Queue(types.QueueSync), asynq.MaxRetry(5), asynq.Timeout(2*time.Hour))

	_, err = s.taskEnqueuer.Enqueue(task)
	if err != nil {
		logger.Errorf(ctx, "failed to enqueue sync task: %v", err)
		syncLog.Status = types.SyncLogStatusFailed
		syncLog.FinishedAt = timePtr(time.Now().UTC())
		syncLog.ErrorMessage = err.Error()
		_ = s.syncLogRepo.Update(ctx, syncLog)
		if ds.Status != types.DataSourceStatusPaused {
			ds.Status = types.DataSourceStatusError
		}
		ds.ErrorMessage = fmt.Sprintf("Failed to enqueue sync: %v", err)
		_ = s.dsRepo.Update(ctx, ds)
		return nil, err
	}

	logger.Infof(ctx, "sync task enqueued: ds=%s syncLog=%s", dsID, syncLog.ID)
	return syncLog, nil
}

// PauseDataSource pauses a data source's scheduled syncs
func (s *DataSourceService) PauseDataSource(ctx context.Context, id string) error {
	ds, err := s.GetDataSource(ctx, id)
	if err != nil {
		return err
	}

	ds.Status = types.DataSourceStatusPaused
	if err := s.dsRepo.Update(ctx, ds); err != nil {
		logger.Errorf(ctx, "failed to pause data source: %v", err)
		return err
	}

	// Remove cron schedule
	s.scheduler.Remove(id)

	logger.Infof(ctx, "data source paused: id=%s", id)
	return nil
}

// ResumeDataSource resumes a paused data source
func (s *DataSourceService) ResumeDataSource(ctx context.Context, id string) error {
	ds, err := s.GetDataSource(ctx, id)
	if err != nil {
		return err
	}

	ds.Status = types.DataSourceStatusActive
	if err := s.dsRepo.Update(ctx, ds); err != nil {
		logger.Errorf(ctx, "failed to resume data source: %v", err)
		return err
	}

	// Re-register cron schedule
	if err := s.scheduler.AddOrUpdate(ds); err != nil {
		logger.Warnf(ctx, "failed to re-register cron for ds=%s: %v", ds.ID, err)
	}

	logger.Infof(ctx, "data source resumed: id=%s", id)
	return nil
}

// GetSyncLogs retrieves sync history for a data source
func (s *DataSourceService) GetSyncLogs(ctx context.Context, dsID string, limit int, offset int) ([]*types.SyncLog, error) {
	logs, err := s.syncLogRepo.FindByDataSource(ctx, dsID, limit, offset)
	if err != nil {
		logger.Errorf(ctx, "failed to get sync logs: %v", err)
		return nil, err
	}
	return logs, nil
}

// GetSyncLog retrieves a specific sync log entry
func (s *DataSourceService) GetSyncLog(ctx context.Context, syncLogID string) (*types.SyncLog, error) {
	log, err := s.syncLogRepo.FindByID(ctx, syncLogID)
	if err != nil {
		return nil, err
	}
	return log, nil
}

// ProcessSync handles the actual sync operation (called by asynq task)
func (s *DataSourceService) ProcessSync(ctx context.Context, task *asynq.Task) error {
	var payload types.DataSourceSyncPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Errorf(ctx, "failed to unmarshal sync payload: %v", err)
		return err
	}

	logger.Infof(ctx, "processing data source sync: ds=%s syncLog=%s", payload.DataSourceID, payload.SyncLogID)

	// Get data source
	ds, err := s.GetDataSource(ctx, payload.DataSourceID)
	if err != nil {
		logger.Warnf(ctx, "data source not found (likely deleted), cancelling sync: ds=%s err=%v", payload.DataSourceID, err)
		if syncLog, slErr := s.syncLogRepo.FindByID(ctx, payload.SyncLogID); slErr == nil && syncLog != nil {
			syncLog.Status = types.SyncLogStatusCanceled
			syncLog.FinishedAt = timePtr(time.Now().UTC())
			syncLog.ErrorMessage = "data source has been deleted"
			_ = s.syncLogRepo.Update(ctx, syncLog)
		}
		return nil
	}

	// Get sync log
	syncLog, err := s.syncLogRepo.FindByID(ctx, payload.SyncLogID)
	if err != nil {
		logger.Errorf(ctx, "failed to get sync log: %v", err)
		return nil
	}

	if _, err := s.kbService.GetKnowledgeBaseByID(ctx, ds.KnowledgeBaseID); err != nil {
		logger.Warnf(ctx, "knowledge base not found (likely deleted), cancelling sync: kb=%s ds=%s err=%v",
			ds.KnowledgeBaseID, payload.DataSourceID, err)
		syncLog.Status = types.SyncLogStatusCanceled
		syncLog.FinishedAt = timePtr(time.Now().UTC())
		syncLog.ErrorMessage = "knowledge base has been deleted"
		_ = s.syncLogRepo.Update(ctx, syncLog)
		return nil
	}

	wasPaused := ds.Status == types.DataSourceStatusPaused

	// Get connector
	connector, err := s.connectorRegistry.Get(ds.Type)
	if err != nil {
		logger.Errorf(ctx, "connector not found: type=%s", ds.Type)
		syncLog.Status = types.SyncLogStatusFailed
		syncLog.FinishedAt = timePtr(time.Now().UTC())
		syncLog.ErrorMessage = fmt.Sprintf("Connector not found: %s", ds.Type)
		_ = s.syncLogRepo.Update(ctx, syncLog)
		if !wasPaused {
			ds.Status = types.DataSourceStatusError
		}
		ds.ErrorMessage = syncLog.ErrorMessage
		_ = s.dsRepo.Update(ctx, ds)
		return err
	}

	// Parse configuration
	config, err := ds.ParseConfig()
	if err != nil {
		logger.Errorf(ctx, "failed to parse config: %v", err)
		syncLog.Status = types.SyncLogStatusFailed
		syncLog.FinishedAt = timePtr(time.Now().UTC())
		syncLog.ErrorMessage = fmt.Sprintf("Invalid configuration: %v", err)
		_ = s.syncLogRepo.Update(ctx, syncLog)
		if !wasPaused {
			ds.Status = types.DataSourceStatusError
		}
		ds.ErrorMessage = syncLog.ErrorMessage
		_ = s.dsRepo.Update(ctx, ds)
		return err
	}

	// Fetch items based on sync mode
	var items []types.FetchedItem
	var nextCursor *types.SyncCursor
	var fetchErr error

	if payload.ForceFull || ds.SyncMode == types.SyncModeFull {
		// Full sync
		items, fetchErr = connector.FetchAll(ctx, config, config.ResourceIDs)
		logger.Infof(ctx, "full sync fetched %d items", len(items))
	} else {
		// Incremental sync
		cursor, _ := ds.ParseSyncCursor()
		items, nextCursor, fetchErr = connector.FetchIncremental(ctx, config, cursor)
		logger.Infof(ctx, "incremental sync fetched %d items", len(items))
	}

	var fetchWarnings []string
	var tagWarnings []string
	var partialFetch *datasource.PartialFetchError
	if errors.As(fetchErr, &partialFetch) {
		fetchWarnings = partialFetch.Details
		fetchErr = nil
	}

	if fetchErr != nil {
		// Persist connector cursor even when fetch failed so transient outages
		// (e.g. RSS feed downtime) do not force a full re-ingest on recovery.
		if nextCursor != nil {
			if cursorJSON, cerr := nextCursor.ToJSON(); cerr == nil {
				ds.LastSyncCursor = cursorJSON
				if uerr := s.dsRepo.UpdateSyncState(ctx, ds); uerr != nil {
					logger.Warnf(ctx, "failed to persist sync cursor after fetch error: %v", uerr)
				}
			}
		}
		logger.Errorf(ctx, "fetch operation failed: %v", fetchErr)
		syncLog.Status = types.SyncLogStatusFailed
		syncLog.FinishedAt = timePtr(time.Now().UTC())
		syncLog.ErrorMessage = fmt.Sprintf("Fetch failed: %v", fetchErr)
		_ = s.syncLogRepo.Update(ctx, syncLog)
		if !wasPaused {
			ds.Status = types.DataSourceStatusError
		}
		ds.ErrorMessage = syncLog.ErrorMessage
		_ = s.dsRepo.Update(ctx, ds)
		return fetchErr
	}

	// Process fetched items and write to knowledge base
	var result = &types.SyncResult{
		Total: len(items),
	}

	// Set tenant context so KnowledgeService can resolve tenant info correctly
	ctx = context.WithValue(ctx, types.TenantIDContextKey, ds.TenantID)

	tenant, err := s.tenantRepo.GetTenantByID(ctx, ds.TenantID)
	if err != nil {
		logger.Errorf(ctx, "failed to get tenant info: %v", err)
		syncLog.Status = types.SyncLogStatusFailed
		syncLog.FinishedAt = timePtr(time.Now().UTC())
		syncLog.ErrorMessage = fmt.Sprintf("Failed to get tenant info: %v", err)
		_ = s.syncLogRepo.Update(ctx, syncLog)
		if !wasPaused {
			ds.Status = types.DataSourceStatusError
		}
		ds.ErrorMessage = syncLog.ErrorMessage
		_ = s.dsRepo.Update(ctx, ds)
		return err
	}
	ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenant)

	// Auto-tag: find or create a tag for this data source so synced items are easily identifiable
	autoTagIDs := []string{}
	autoTagName := ds.Name
	if autoTag, tagErr := s.tagService.FindOrCreateTagByName(ctx, ds.KnowledgeBaseID, autoTagName); tagErr != nil {
		logger.Warnf(ctx, "failed to find/create auto-tag %q: %v (proceeding without tag)", autoTagName, tagErr)
	} else if autoTag != nil {
		autoTagIDs = append(autoTagIDs, autoTag.ID)
		logger.Infof(ctx, "using auto-tag %q (id=%s) for data source sync", autoTagName, autoTag.ID)
	}

	hadItemFailure := false
	for _, item := range items {
		if item.IsDeleted {
			if ds.SyncDeletions {
				// Count only — actual KB deletion is intentionally not performed.
				// Users manage knowledge removal explicitly via the KB UI to avoid
				// accidental data loss from connector misdetection or reconfiguration.
				result.Deleted++
			}
			continue
		}

		if len(item.Content) == 0 && item.URL == "" {
			// Check if this is an error item from the connector (failed to fetch content)
			if errMsg, hasErr := item.Metadata["error"]; hasErr {
				logger.Warnf(ctx, "item %q (external_id=%s) fetch failed: %s", item.Title, item.ExternalID, errMsg)
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", item.Title, errMsg))
				hadItemFailure = true
			} else {
				logger.Infof(ctx, "skipping item %q (external_id=%s): no content or URL", item.Title, item.ExternalID)
				result.Skipped++
			}
			continue
		}
		if invalid := strings.TrimSpace(item.Metadata["invalid_official_tags"]); invalid != "" {
			logger.Warnf(ctx, "item %q contains rejected RSS tags: %s", item.Title, invalid)
			tagWarnings = append(tagWarnings, fmt.Sprintf("%s: rejected RSS tags: %s", item.Title, invalid))
		}

		itemTagIDs, tagErr := s.resolveFetchedItemTags(ctx, ds.KnowledgeBaseID, item.TagNames, autoTagIDs)
		if tagErr != nil {
			// Do not ingest a partial official-tag set.  On an existing RSS item
			// this would otherwise remove previously managed tags; on any failure
			// we retain the cursor so the full item is retried intact.
			logger.Warnf(ctx, "failed to resolve tags for item %q: %v", item.Title, tagErr)
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: resolve official tags: %v", item.Title, tagErr))
			hadItemFailure = true
			continue
		}
		isUpdate, err := s.ingestItem(ctx, ds, &item, itemTagIDs)
		if err != nil {
			// Duplicate file/URL is not a failure — count as skipped
			var dupErr *types.DuplicateKnowledgeError
			if errors.As(err, &dupErr) {
				logger.Infof(ctx, "item %q (external_id=%s) already exists, skipping", item.Title, item.ExternalID)
				result.Skipped++
			} else {
				logger.Warnf(ctx, "failed to ingest item %q (external_id=%s): %v", item.Title, item.ExternalID, err)
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", item.Title, err))
				hadItemFailure = true
			}
		} else if isUpdate {
			result.Updated++
		} else {
			result.Created++
		}
	}

	resultJSON, _ := result.ToJSON()
	if err := allFetchedItemsFailedError(result); err != nil {
		logger.Errorf(ctx, "data source sync failed while processing fetched items: %v", err)
		s.updateSyncRunResult(ctx, ds, syncLog, result, resultJSON, types.SyncLogStatusFailed, err.Error(), wasPaused)
		return err
	}

	// Update cursor for next incremental sync
	if shouldAdvanceRSSCursor(nextCursor != nil, hadItemFailure) {
		cursorJSON, _ := nextCursor.ToJSON()
		ds.LastSyncCursor = cursorJSON
	}

	ds.LastSyncAt = timePtr(time.Now().UTC())
	syncStatus := types.SyncLogStatusSuccess
	syncErrorMessage := ""
	if len(fetchWarnings) > 0 {
		syncStatus = types.SyncLogStatusPartial
		syncErrorMessage = fmt.Sprintf("Some feeds failed: %s", strings.Join(fetchWarnings, "; "))
		for _, w := range fetchWarnings {
			result.Errors = append(result.Errors, w)
		}
		resultJSON, _ = result.ToJSON()
	}
	if len(tagWarnings) > 0 {
		syncStatus = types.SyncLogStatusPartial
		if syncErrorMessage != "" {
			syncErrorMessage += "; "
		}
		syncErrorMessage += strings.Join(tagWarnings, "; ")
		result.Errors = append(result.Errors, tagWarnings...)
		resultJSON, _ = result.ToJSON()
	}
	s.updateSyncRunResult(ctx, ds, syncLog, result, resultJSON, syncStatus, syncErrorMessage, wasPaused)

	logger.Infof(ctx, "data source sync completed: ds=%s created=%d updated=%d deleted=%d",
		payload.DataSourceID, syncLog.ItemsCreated, syncLog.ItemsUpdated, syncLog.ItemsDeleted)

	return nil
}

func shouldAdvanceRSSCursor(hasNextCursor, hadItemFailure bool) bool {
	return hasNextCursor && !hadItemFailure
}

// resolveFetchedItemTags creates/reuses source-provided tags and combines them
// with the data-source tag. Invalid tags are ignored by the RSS connector
// before this point; this guard also protects other connectors.
func (s *DataSourceService) resolveFetchedItemTags(ctx context.Context, kbID string, names []string, baseIDs []string) ([]string, error) {
	ids := append([]string(nil), baseIDs...)
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id != "" {
			seen[id] = struct{}{}
		}
	}
	var firstErr error
	for _, name := range names {
		if !isOfficialPolicyTagName(name) {
			continue
		}
		tag, err := s.tagService.FindOrCreateTagByName(ctx, kbID, name)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if tag != nil && tag.ID != "" {
			if _, ok := seen[tag.ID]; !ok {
				ids = append(ids, tag.ID)
				seen[tag.ID] = struct{}{}
			}
		}
	}
	return ids, firstErr
}

func isOfficialPolicyTagName(name string) bool {
	name = strings.TrimSpace(name)
	for _, prefix := range []string{"服务对象/", "发文机构/", "主题/", "文件载体/", "文件类型/", "关联内容/", "申报状态/"} {
		if strings.HasPrefix(name, prefix) && strings.TrimSpace(strings.TrimPrefix(name, prefix)) != "" {
			return true
		}
	}
	return false
}

func (s *DataSourceService) updateSyncRunResult(
	ctx context.Context,
	ds *types.DataSource,
	syncLog *types.SyncLog,
	result *types.SyncResult,
	resultJSON types.JSON,
	status string,
	errorMessage string,
	wasPaused bool,
) {
	syncLog.ItemsTotal = result.Total
	syncLog.ItemsCreated = result.Created
	syncLog.ItemsUpdated = result.Updated
	syncLog.ItemsDeleted = result.Deleted
	syncLog.ItemsSkipped = result.Skipped
	syncLog.ItemsFailed = result.Failed
	syncLog.Status = status
	syncLog.FinishedAt = timePtr(time.Now().UTC())
	syncLog.ErrorMessage = errorMessage
	syncLog.Result = resultJSON
	if err := s.syncLogRepo.UpdateResult(ctx, syncLog); err != nil {
		logger.Errorf(ctx, "failed to update sync log: %v", err)
	}

	if status == types.SyncLogStatusFailed {
		if !wasPaused {
			ds.Status = types.DataSourceStatusError
		}
	} else if wasPaused {
		ds.Status = types.DataSourceStatusPaused
	} else {
		ds.Status = types.DataSourceStatusActive
	}
	ds.ErrorMessage = errorMessage
	ds.LastSyncResult = resultJSON
	if err := s.dsRepo.UpdateSyncState(ctx, ds); err != nil {
		logger.Errorf(ctx, "failed to update data source: %v", err)
	}
}

func allFetchedItemsFailedError(result *types.SyncResult) error {
	if result == nil || result.Total == 0 {
		return nil
	}
	if result.Failed != result.Total || result.Created != 0 || result.Updated != 0 ||
		result.Deleted != 0 || result.Skipped != 0 {
		return nil
	}

	detail := ""
	if len(result.Errors) > 0 {
		detail = result.Errors[0]
		const maxDetailLen = 500
		if len(detail) > maxDetailLen {
			detail = detail[:maxDetailLen] + "..."
		}
	}
	if detail == "" {
		return fmt.Errorf("all fetched items failed during sync (%d/%d)", result.Failed, result.Total)
	}
	return fmt.Errorf("all fetched items failed during sync (%d/%d): %s", result.Failed, result.Total, detail)
}

// ValidateCredentials tests connectivity using raw credentials without persisting anything.
func (s *DataSourceService) ValidateCredentials(ctx context.Context, connectorType string, credentials map[string]interface{}) error {
	connector, err := s.connectorRegistry.Get(connectorType)
	if err != nil {
		return err
	}

	config := &types.DataSourceConfig{
		Type:        connectorType,
		Credentials: credentials,
	}

	if err := connector.Validate(ctx, config); err != nil {
		return err
	}

	return nil
}

// Helper functions

func (s *DataSourceService) validateDataSourceConfig(ctx context.Context, ds *types.DataSource) error {
	connector, err := s.connectorRegistry.Get(ds.Type)
	if err != nil {
		return err
	}

	config, err := ds.ParseConfig()
	if err != nil {
		return datasource.ErrInvalidConfig
	}

	return connector.Validate(ctx, config)
}

// ingestItem writes a single FetchedItem into the knowledge base.
// If a knowledge item with the same external_id already exists, it is deleted first (update = delete + re-create).
//
// Routing logic:
//   - Has Content bytes → CreateKnowledgeFromFile (走完整的文档解析 pipeline)
//   - Has URL only      → CreateKnowledgeFromURL  (让 FTMind 下载并解析)
//
// Returns (isUpdate, error) — isUpdate is true when an existing item was replaced.
func (s *DataSourceService) ingestItem(ctx context.Context, ds *types.DataSource, item *types.FetchedItem, tagIDs []string) (bool, error) {
	channel := ds.Type // e.g. "feishu", "notion"

	metadata := map[string]string{
		"external_id":        item.ExternalID,
		"source_resource_id": item.SourceResourceID,
		"datasource_id":      ds.ID,
	}
	for k, v := range item.Metadata {
		metadata[k] = v
	}
	if len(item.TagNames) > 0 {
		if encoded, err := json.Marshal(item.TagNames); err == nil {
			metadata["official_tag_names"] = string(encoded)
			metadata["official_tag_source"] = "website"
		}
	}
	if len(item.Content) > 0 {
		metadata["rss_content_sha256"] = contentSHA256(item.Content)
	}

	// Check if a knowledge item with this external_id already exists → delete it first (update)
	isUpdate := false
	if item.ExternalID != "" {
		repo := s.knowledgeService.GetRepository()
		existing, err := repo.FindByMetadataKey(ctx, ds.TenantID, ds.KnowledgeBaseID, "external_id", item.ExternalID)
		if err != nil {
			logger.Warnf(ctx, "failed to check existing knowledge for external_id=%s: %v", item.ExternalID, err)
			// Non-fatal: proceed with creation (may produce duplicate)
		} else if existing != nil {
			// Preserve user/AI tags while replacing only tags managed by this
			// connector. Official dimension tags are intentionally refreshed from
			// the latest RSS item, so expired/removed website tags do not linger.
			existingTags, tagErr := loadExistingKnowledgeTags(ctx, s.knowledgeService, existing.ID)
			if tagErr != nil {
				// Failing open here would call SetKnowledgeTags with only the RSS
				// tags, deleting a user's manual/AI tags.  Report this item as a
				// partial failure instead and leave it entirely untouched.
				return false, tagErr
			}
			tagIDs = mergeExistingManualTagIDs(tagIDs, existingTags, existing.Metadata)
			// A category/metadata-only RSS change must not destroy the existing
			// knowledge ID, chunks, vectors or Wiki references. When the fetched
			// body is byte-identical, update only mutable display/metadata columns
			// and replace the managed tag set in place.
			if len(item.Content) > 0 && matchesExistingRSSContent(item, existing) {
				updates := map[string]interface{}{"metadata": metadata}
				if item.Title != "" {
					updates["title"] = item.Title
				}
				if item.FileName != "" {
					updates["file_name"] = item.FileName
				}
				if err := repo.UpdateKnowledgeColumns(ctx, existing.ID, updates); err != nil {
					return false, err
				}
				if err := s.knowledgeService.SetKnowledgeTags(ctx, existing.ID, tagIDs); err != nil {
					return false, rollbackRSSInPlaceUpdate(ctx, repo, s.knowledgeService, existing, existingTags, err)
				}
				return true, nil
			}
			logger.Infof(ctx, "found existing knowledge %s for external_id=%s, deleting for update", existing.ID, item.ExternalID)
			if err := s.knowledgeService.DeleteKnowledge(ctx, existing.ID); err != nil {
				logger.Warnf(ctx, "failed to delete existing knowledge %s: %v", existing.ID, err)
			} else {
				isUpdate = true
			}
		}
	}

	// Case 1: content already fetched → build a FileHeader from bytes and call CreateKnowledgeFromFile
	if len(item.Content) > 0 {
		fh, err := bytesToFileHeader(item.Content, item.FileName)
		if err != nil {
			return isUpdate, fmt.Errorf("build file header: %w", err)
		}
		_, err = s.knowledgeService.CreateKnowledgeFromFile(
			ctx,
			ds.KnowledgeBaseID,
			fh,
			metadata,
			nil,           // use KB default for multimodal
			item.FileName, // customFileName — must include extension for file-type validation
			tagIDs,        // auto-tag from data source
			channel,
			nil,
		)
		return isUpdate, err
	}

	// Case 2: only a remote URL — let FTMind handle downloading and parsing
	if item.URL != "" {
		_, err := s.knowledgeService.CreateKnowledgeFromURL(
			ctx,
			ds.KnowledgeBaseID,
			item.URL,
			item.FileName,
			"",  // auto-detect file type
			nil, // use KB default for multimodal
			item.Title,
			tagIDs, // auto-tag from data source
			channel,
			nil,
		)
		return isUpdate, err
	}

	return isUpdate, fmt.Errorf("item has neither content nor URL")
}

func loadExistingKnowledgeTags(
	ctx context.Context,
	knowledgeService interfaces.KnowledgeService,
	knowledgeID string,
) ([]*types.KnowledgeTag, error) {
	existingTags, err := knowledgeService.GetKnowledgeTags(ctx, []string{knowledgeID})
	if err != nil {
		return nil, fmt.Errorf("read existing knowledge tags: %w", err)
	}
	return existingTags[knowledgeID], nil
}

// rollbackRSSInPlaceUpdate compensates a failed tag replacement.  The service
// interfaces do not expose a shared transaction, so restore both mutable row
// columns and the complete prior relationship set before returning the item
// failure; ProcessSync consequently leaves the incremental cursor unchanged.
func rollbackRSSInPlaceUpdate(
	ctx context.Context,
	repo interfaces.KnowledgeRepository,
	knowledgeService interfaces.KnowledgeService,
	existing *types.Knowledge,
	previousTags []*types.KnowledgeTag,
	setErr error,
) error {
	previousIDs := make([]string, 0, len(previousTags))
	for _, tag := range previousTags {
		if tag != nil && tag.ID != "" {
			previousIDs = append(previousIDs, tag.ID)
		}
	}
	columns := map[string]interface{}{"metadata": existing.Metadata, "title": existing.Title, "file_name": existing.FileName}
	columnErr := repo.UpdateKnowledgeColumns(ctx, existing.ID, columns)
	tagErr := knowledgeService.SetKnowledgeTags(ctx, existing.ID, previousIDs)
	if columnErr != nil || tagErr != nil {
		return fmt.Errorf("set RSS tags: %w; compensation failed: restore columns=%v restore tags=%v", setErr, columnErr, tagErr)
	}
	return fmt.Errorf("set RSS tags: %w; original metadata and tags restored", setErr)
}

func matchesStoredFileHash(content []byte, stored string) bool {
	stored = strings.TrimSpace(stored)
	md5Digest := md5.Sum(content)
	if fmt.Sprintf("%x", md5Digest[:]) == stored {
		return true
	}
	shaDigest := sha256.Sum256(content)
	return fmt.Sprintf("%x", shaDigest[:]) == stored
}

func contentSHA256(content []byte) string {
	digest := sha256.Sum256(content)
	return fmt.Sprintf("%x", digest[:])
}

func matchesStoredRSSContent(content []byte, existing *types.Knowledge) bool {
	if existing == nil {
		return false
	}
	if matchesStoredFileHash(content, existing.FileHash) {
		return true
	}
	metadata := existing.GetMetadata()
	return metadata["rss_content_sha256"] == contentSHA256(content)
}

func matchesExistingRSSContent(item *types.FetchedItem, existing *types.Knowledge) bool {
	if item == nil || existing == nil {
		return false
	}
	if matchesStoredRSSContent(item.Content, existing) {
		return true
	}
	existingMetadata := existing.GetMetadata()
	guid := item.Metadata["guid"]
	return isBaoanPolicyGUID(guid) && guid == existingMetadata["guid"] &&
		item.Metadata["rss_content_signal"] != "" &&
		item.Metadata["rss_content_signal"] == existingMetadata["rss_content_signal"]
}

func isBaoanPolicyGUID(guid string) bool {
	const prefix = "baoan-policy:post_"
	if !strings.HasPrefix(guid, prefix) || len(guid) == len(prefix) {
		return false
	}
	for _, r := range guid[len(prefix):] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// mergeExistingManualTagIDs retains every existing tag except a tag explicitly
// recorded as managed by the RSS policy integration. Prefixes alone are not
// ownership: a user may legitimately create a similarly-prefixed tag.
func mergeExistingManualTagIDs(tagIDs []string, existingTags []*types.KnowledgeTag, metadata ...types.JSON) []string {
	merged := append([]string(nil), tagIDs...)
	managedNames := map[string]struct{}{}
	if len(metadata) > 0 {
		managedNames = recordedOfficialTagNames(metadata[0])
	}
	for _, existingTag := range existingTags {
		if existingTag != nil {
			_, managed := managedNames[existingTag.Name]
			if managed && isOfficialPolicyTagName(existingTag.Name) {
				continue
			}
			merged = appendUniqueString(merged, existingTag.ID)
		}
	}
	return merged
}

func recordedOfficialTagNames(metadata types.JSON) map[string]struct{} {
	result := map[string]struct{}{}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(metadata, &fields); err != nil {
		return result
	}
	raw := fields["official_tag_names"]
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		var encoded string
		if err := json.Unmarshal(raw, &encoded); err != nil || json.Unmarshal([]byte(encoded), &names) != nil {
			return result
		}
	}
	for _, name := range names {
		if isOfficialPolicyTagName(name) {
			result[name] = struct{}{}
		}
	}
	return result
}

func appendUniqueString(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, v := range values {
		if v == value {
			return values
		}
	}
	return append(values, value)
}

// bytesToFileHeader wraps a []byte into a *multipart.FileHeader so it can be
// consumed by KnowledgeService.CreateKnowledgeFromFile.
func bytesToFileHeader(data []byte, filename string) (*multipart.FileHeader, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Create a form file part
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	partHeader.Set("Content-Type", "application/octet-stream")

	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return nil, fmt.Errorf("create multipart part: %w", err)
	}

	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("write data to part: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	// Parse the multipart data to get a FileHeader
	reader := multipart.NewReader(&buf, writer.Boundary())
	form, err := reader.ReadForm(int64(len(data)) + 1024)
	if err != nil {
		return nil, fmt.Errorf("read multipart form: %w", err)
	}

	files := form.File["file"]
	if len(files) == 0 {
		return nil, fmt.Errorf("no file in multipart form")
	}

	return files[0], nil
}

func timePtr(t time.Time) *time.Time {
	utc := t.UTC()
	return &utc
}
