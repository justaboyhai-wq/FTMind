package service

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	apprepo "github.com/justaboyhai-wq/fmind/internal/application/repository"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessSyncCancelsWhenKnowledgeBaseDeleted(t *testing.T) {
	ds := &types.DataSource{
		ID:              "ds-1",
		TenantID:        1,
		KnowledgeBaseID: "kb-deleted",
		Type:            types.ConnectorTypeRSS,
		Status:          types.DataSourceStatusActive,
	}
	dsRepo := newKBDeleteDSRepo("kb-deleted", ds)
	syncLog := &types.SyncLog{
		ID:           "log-1",
		DataSourceID: ds.ID,
		TenantID:     ds.TenantID,
		Status:       types.SyncLogStatusRunning,
		StartedAt:    time.Now().UTC(),
	}
	syncLogRepo := &processSyncSyncLogRepo{logs: map[string]*types.SyncLog{syncLog.ID: syncLog}}

	svc := &DataSourceService{
		dsRepo:      dsRepo,
		syncLogRepo: syncLogRepo,
		kbService:   &processSyncKBService{getErr: apprepo.ErrKnowledgeBaseNotFound},
	}

	payload, err := json.Marshal(types.DataSourceSyncPayload{
		DataSourceID: ds.ID,
		TenantID:     ds.TenantID,
		SyncLogID:    syncLog.ID,
	})
	require.NoError(t, err)

	err = svc.ProcessSync(context.Background(), asynq.NewTask(types.TypeDataSourceSync, payload))
	require.NoError(t, err)

	updated := syncLogRepo.logs[syncLog.ID]
	require.NotNil(t, updated)
	assert.Equal(t, types.SyncLogStatusCanceled, updated.Status)
	assert.Equal(t, "knowledge base has been deleted", updated.ErrorMessage)
	require.NotNil(t, updated.FinishedAt)
}

type processSyncKBService struct {
	getErr error
}

func (s *processSyncKBService) CreateKnowledgeBase(context.Context, *types.KnowledgeBase) (*types.KnowledgeBase, error) {
	return nil, nil
}
func (s *processSyncKBService) GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error) {
	return nil, s.getErr
}
func (s *processSyncKBService) GetKnowledgeBaseByIDOnly(context.Context, string) (*types.KnowledgeBase, error) {
	return nil, s.getErr
}
func (s *processSyncKBService) GetKnowledgeBasesByIDsOnly(context.Context, []string) ([]*types.KnowledgeBase, error) {
	return nil, nil
}
func (s *processSyncKBService) FillKnowledgeBaseCounts(context.Context, *types.KnowledgeBase) error {
	return nil
}
func (s *processSyncKBService) ListKnowledgeBases(context.Context) ([]*types.KnowledgeBase, error) {
	return nil, nil
}
func (s *processSyncKBService) ListKnowledgeBasesByTenantID(context.Context, uint64) ([]*types.KnowledgeBase, error) {
	return nil, nil
}
func (s *processSyncKBService) UpdateKnowledgeBase(
	context.Context, string, string, string, *types.KnowledgeBaseConfig,
) (*types.KnowledgeBase, error) {
	return nil, nil
}
func (s *processSyncKBService) DeleteKnowledgeBase(context.Context, string) error { return nil }
func (s *processSyncKBService) TogglePinKnowledgeBase(context.Context, string) (*types.KnowledgeBase, error) {
	return nil, nil
}
func (s *processSyncKBService) HybridSearch(context.Context, string, types.SearchParams) ([]*types.SearchResult, error) {
	return nil, nil
}
func (s *processSyncKBService) GetQueryEmbedding(context.Context, string, string) ([]float32, error) {
	return nil, nil
}
func (s *processSyncKBService) ResolveEmbeddingModelKeys(context.Context, []*types.KnowledgeBase) map[string]string {
	return nil
}
func (s *processSyncKBService) CopyKnowledgeBase(context.Context, string, string) (*types.KnowledgeBase, *types.KnowledgeBase, error) {
	return nil, nil, nil
}
func (s *processSyncKBService) DuplicateKnowledgeBase(context.Context, string) (*types.KnowledgeBase, error) {
	return nil, nil
}
func (s *processSyncKBService) GetRepository() interfaces.KnowledgeBaseRepository { return nil }
func (s *processSyncKBService) ProcessKBDelete(context.Context, *asynq.Task) error {
	return nil
}

var _ interfaces.KnowledgeBaseService = (*processSyncKBService)(nil)

type processSyncSyncLogRepo struct {
	logs map[string]*types.SyncLog
}

func (r *processSyncSyncLogRepo) Create(_ context.Context, log *types.SyncLog) error {
	r.logs[log.ID] = log
	return nil
}
func (r *processSyncSyncLogRepo) FindByID(_ context.Context, id string) (*types.SyncLog, error) {
	log, ok := r.logs[id]
	if !ok {
		return nil, errors.New("sync log not found")
	}
	return log, nil
}
func (r *processSyncSyncLogRepo) FindByDataSource(context.Context, string, int, int) ([]*types.SyncLog, error) {
	return nil, nil
}
func (r *processSyncSyncLogRepo) FindLatest(context.Context, string) (*types.SyncLog, error) {
	return nil, nil
}
func (r *processSyncSyncLogRepo) HasRunningSync(context.Context, string) (bool, error) {
	return false, nil
}
func (r *processSyncSyncLogRepo) Update(_ context.Context, log *types.SyncLog) error {
	r.logs[log.ID] = log
	return nil
}
func (r *processSyncSyncLogRepo) UpdateResult(_ context.Context, log *types.SyncLog) error {
	return r.Update(context.Background(), log)
}
func (r *processSyncSyncLogRepo) CancelPendingByDataSource(context.Context, string) error {
	return nil
}
func (r *processSyncSyncLogRepo) CleanupOldLogs(context.Context, int) error { return nil }

func TestAllFetchedItemsFailedError(t *testing.T) {
	err := allFetchedItemsFailedError(&types.SyncResult{
		Total:  2,
		Failed: 2,
		Errors: []string{"doc one: export failed"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all fetched items failed during sync (2/2)")
	assert.Contains(t, err.Error(), "doc one: export failed")
}

func TestAllFetchedItemsFailedErrorIgnoresPartialFailure(t *testing.T) {
	err := allFetchedItemsFailedError(&types.SyncResult{
		Total:   3,
		Created: 1,
		Failed:  2,
	})
	require.NoError(t, err)
}

func TestAllFetchedItemsFailedErrorIgnoresSkippedItems(t *testing.T) {
	err := allFetchedItemsFailedError(&types.SyncResult{
		Total:   3,
		Skipped: 3,
	})
	require.NoError(t, err)
}

func TestAllFetchedItemsFailedErrorTruncatesLongDetail(t *testing.T) {
	err := allFetchedItemsFailedError(&types.SyncResult{
		Total:  1,
		Failed: 1,
		Errors: []string{strings.Repeat("x", 600)},
	})
	require.Error(t, err)
	assert.LessOrEqual(t, len(err.Error()), 560)
	assert.Contains(t, err.Error(), "...")
}

func TestIsOfficialPolicyTagNameRejectsInferenceAndEmptyValues(t *testing.T) {
	if !isOfficialPolicyTagName("主题/综合服务") {
		t.Fatal("expected official tag")
	}
	if isOfficialPolicyTagName("综合服务") {
		t.Fatal("unprefixed tag must be rejected")
	}
	if isOfficialPolicyTagName("AI分析/产业扶持") {
		t.Fatal("inferred tag must be rejected")
	}
	if isOfficialPolicyTagName("主题/") {
		t.Fatal("empty official value must be rejected")
	}
}

func TestAppendUniqueStringPreservesOrder(t *testing.T) {
	got := appendUniqueString([]string{"a", "b"}, "b")
	got = appendUniqueString(got, "c")
	if strings.Join(got, ",") != "a,b,c" {
		t.Fatalf("unexpected tags: %v", got)
	}
}

func TestMergeExistingManualTagIDsPreservesManualAndDropsManaged(t *testing.T) {
	got := mergeExistingManualTagIDs([]string{"source"}, []*types.KnowledgeTag{
		{ID: "manual", Name: "重点政策"},
		{ID: "managed", Name: "主题/综合服务"},
	}, types.JSON(`{"official_tag_names":"[\"\u4e3b\u9898/\u7efc\u5408\u670d\u52a1\"]"}`))
	if strings.Join(got, ",") != "source,manual" {
		t.Fatalf("unexpected merged tags: %v", got)
	}
}

func TestMatchesStoredFileHashUsesUploadMD5(t *testing.T) {
	content := []byte("RSS canonical markdown")
	digest := md5.Sum(content)
	if !matchesStoredFileHash(content, fmt.Sprintf("%x", digest[:])) {
		t.Fatal("RSS content must compare with the MD5 stored by file ingestion")
	}
}

func TestMergeExistingManualTagIDsUsesRecordedManagedTagsInsteadOfPrefix(t *testing.T) {
	metadata := types.JSON(`{"official_tag_names":"[\"\u4e3b\u9898/\u5b98\u7f51\"]"}`)
	got := mergeExistingManualTagIDs([]string{"new-official"}, []*types.KnowledgeTag{
		{ID: "manual-same-prefix", Name: "\u4e3b\u9898/\u4eba\u5de5\u4fdd\u7559"},
		{ID: "old-managed", Name: "\u4e3b\u9898/\u5b98\u7f51"},
	}, metadata)
	if strings.Join(got, ",") != "new-official,manual-same-prefix" {
		t.Fatalf("managed tags must come from metadata only, got %v", got)
	}
}

func TestLoadExistingKnowledgeTagsFailsClosedWhenTagReadFails(t *testing.T) {
	svc := &tagReadFailureKnowledgeService{}
	_, err := loadExistingKnowledgeTags(context.Background(), svc, "knowledge-1")
	if err == nil || !strings.Contains(err.Error(), "read existing knowledge tags") {
		t.Fatalf("tag read failure must stop the in-place update, got %v", err)
	}
}

func TestShouldAdvanceRSSCursorRequiresEveryItemToSucceed(t *testing.T) {
	if !shouldAdvanceRSSCursor(true, false) {
		t.Fatal("a successful RSS batch must advance its cursor")
	}
	if shouldAdvanceRSSCursor(true, true) {
		t.Fatal("a partial item failure must retain the old cursor for retry")
	}
	if shouldAdvanceRSSCursor(false, false) {
		t.Fatal("no incremental cursor must not be advanced")
	}
}

func TestRollbackRSSInPlaceUpdateRestoresColumnsAndTagsAfterSetFailure(t *testing.T) {
	repo := &rssInPlaceRollbackRepo{}
	service := &rssInPlaceRollbackKnowledgeService{}
	existing := &types.Knowledge{ID: "knowledge-1", Title: "before", FileName: "before.md", Metadata: types.JSON(`{"old":"metadata"}`)}
	err := rollbackRSSInPlaceUpdate(context.Background(), repo, service, existing, []*types.KnowledgeTag{{ID: "manual-1"}, {ID: "official-1"}}, errors.New("set failed"))
	if err == nil || !strings.Contains(err.Error(), "original metadata and tags restored") {
		t.Fatalf("expected compensated tag failure, got %v", err)
	}
	if repo.updateCalls != 1 || repo.values["title"] != "before" || repo.values["file_name"] != "before.md" {
		t.Fatalf("original knowledge columns were not restored: %#v", repo)
	}
	if len(service.setCalls) != 1 || strings.Join(service.setCalls[0], ",") != "manual-1,official-1" {
		t.Fatalf("original tag relationships were not restored: %#v", service.setCalls)
	}
}

type rssInPlaceRollbackRepo struct {
	interfaces.KnowledgeRepository
	updateCalls int
	values      map[string]interface{}
}

func (r *rssInPlaceRollbackRepo) UpdateKnowledgeColumns(_ context.Context, _ string, values map[string]interface{}) error {
	r.updateCalls++
	r.values = values
	return nil
}

type rssInPlaceRollbackKnowledgeService struct {
	interfaces.KnowledgeService
	setCalls [][]string
}

func (s *rssInPlaceRollbackKnowledgeService) SetKnowledgeTags(_ context.Context, _ string, tags []string) error {
	s.setCalls = append(s.setCalls, append([]string(nil), tags...))
	return nil
}

// Embedding the full service interface keeps this test focused on the only
// method the tag-preservation guard is allowed to call.
type tagReadFailureKnowledgeService struct {
	interfaces.KnowledgeService
}

func (s *tagReadFailureKnowledgeService) GetKnowledgeTags(context.Context, []string) (map[string][]*types.KnowledgeTag, error) {
	return nil, errors.New("tag storage unavailable")
}

func TestMatchesExistingRSSContentAllowsCategoryOnlyBaoanPolicyUpdate(t *testing.T) {
	existing := &types.Knowledge{Metadata: types.JSON(`{"guid":"baoan-policy:post_42","rss_content_signal":"stable-content-signal"}`), FileHash: "unrelated-postprocess-hash"}
	item := &types.FetchedItem{ExternalID: "http://feed.xml:baoan-policy:post_42", Content: []byte("parser output changed"), Metadata: map[string]string{"guid": "baoan-policy:post_42", "rss_content_signal": "stable-content-signal"}}
	if !matchesExistingRSSContent(item, existing) {
		t.Fatal("category-only Baoan RSS update must update tags in place")
	}
}
