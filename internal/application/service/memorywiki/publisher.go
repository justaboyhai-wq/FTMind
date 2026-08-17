package memorywiki

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/justaboyhai-wq/fmind/internal/application/repository"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

func (s *Service) PublishApproved(ctx context.Context, tenantID uint64, id, knowledgeBaseID string) (*types.WikiPage, error) {
	if tenantID == 0 {
		return nil, ErrInvalidMemoryWikiTarget
	}
	if err := requireMemoryWikiReviewer(ctx, tenantID); err != nil {
		return nil, err
	}
	publication, err := s.repo.GetMemoryWikiPublication(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if publication.Status != types.MemoryReviewStatusApproved && publication.Status != types.MemoryReviewStatusPublishing && publication.Status != types.MemoryReviewStatusPublished {
		return nil, ErrMemoryReviewNotApproved
	}
	kb, err := s.resolveTeamMemoryWiki(ctx, tenantID, publication.TeamID, knowledgeBaseID)
	if err != nil || kb == nil || !kb.IsDedicatedMemoryWiki() || kb.TenantID != tenantID || kb.MemoryTeamID != publication.TeamID {
		return nil, ErrInvalidMemoryWikiTarget
	}
	knowledgeBaseID = kb.ID
	if publication.KnowledgeBaseID != "" && publication.KnowledgeBaseID != knowledgeBaseID {
		return nil, ErrInvalidMemoryWikiTarget
	}
	ctx = types.WithMemoryWikiMutation(ctx)
	key := publicationProjectionKey(publication)
	projection, err := s.repo.GetMemoryProjection(ctx, key)
	if err != nil {
		return nil, err
	}
	if err := validateClaimEvidenceCoverage(projection.Snapshot); err != nil {
		return nil, err
	}
	slug := StableMemoryWikiSlug(tenantID, publication.TeamID, publication.BindingID, publication.MemoryID)
	if publication.Status == types.MemoryReviewStatusPublished {
		return s.getPublishedMemoryWikiPage(ctx, publication, knowledgeBaseID, slug)
	}
	if _, err := s.repo.StartMemoryWikiPublishing(ctx, key, publication.ID, knowledgeBaseID); err != nil {
		if !errors.Is(err, repository.ErrExternalMemoryStateConflict) {
			return nil, err
		}
		current, reloadErr := s.repo.GetMemoryWikiPublication(ctx, tenantID, publication.ID)
		if reloadErr != nil {
			return nil, errors.Join(err, reloadErr)
		}
		if current.KnowledgeBaseID != knowledgeBaseID {
			return nil, err
		}
		switch current.Status {
		case types.MemoryReviewStatusPublishing:
			// Another worker owns or just completed the same transition. The
			// stable slug and idempotent Wiki write let us converge safely.
		case types.MemoryReviewStatusPublished:
			page, getErr := s.getPublishedMemoryWikiPage(ctx, current, knowledgeBaseID, slug)
			if getErr != nil {
				return nil, errors.Join(err, getErr)
			}
			return page, nil
		default:
			return nil, err
		}
	}

	desired, err := renderMemoryWikiPage(projection, knowledgeBaseID, slug)
	if err != nil {
		_ = s.repo.FailMemoryWikiPublishing(ctx, key, publication.ID, "rendering", err.Error())
		return nil, err
	}
	currentBeforeWrite, err := s.repo.GetMemoryWikiPublication(ctx, tenantID, publication.ID)
	if err != nil {
		return nil, err
	}
	if currentBeforeWrite.Status != types.MemoryReviewStatusPublishing {
		if currentBeforeWrite.Status == types.MemoryReviewStatusRevoked {
			return nil, ErrMemorySourceRevoked
		}
		return nil, repository.ErrExternalMemoryStateConflict
	}
	ctx = types.WithMemoryWikiPublicationGuard(ctx, tenantID, publication.ID)
	page, revision, writeErr := s.createOrUpdateMemoryWikiPage(ctx, desired, projection)
	if writeErr != nil {
		current, reloadErr := s.repo.GetMemoryWikiPublication(ctx, tenantID, publication.ID)
		if reloadErr == nil && current.Status == types.MemoryReviewStatusRevoked {
			return nil, ErrMemorySourceRevoked
		}
		if reloadErr != nil {
			return nil, errors.Join(writeErr, reloadErr)
		}
		resetErr := s.repo.FailMemoryWikiPublishing(ctx, key, publication.ID, "publishing", writeErr.Error())
		if resetErr != nil {
			return nil, errors.Join(writeErr, resetErr)
		}
		return nil, writeErr
	}

	publishedAt := time.Now().UTC()
	revisionID := revision.ID
	claims := make([]types.WikiClaimEvidence, 0, len(projection.Snapshot.Claims))
	for _, claim := range projection.Snapshot.Claims {
		claims = append(claims, types.WikiClaimEvidence{
			ClaimID: claim.ClaimID, ClaimText: claim.Text, Factual: claim.Factual,
			WikiLocator: claimWikiLocator(claim.ClaimID), EvidenceLocators: claim.Evidence, CreatedAt: publishedAt,
		})
	}
	_, err = s.repo.CompleteMemoryWikiPublishing(ctx, key, publication.ID, types.MemoryWikiPublishResult{
		KnowledgeBaseID: knowledgeBaseID, WikiPageID: page.ID, WikiRevisionID: revisionID,
		WikiPageVersion: revision.WikiPageVersion, PublishedAt: publishedAt, ClaimEvidence: claims,
	})
	if err != nil {
		if errors.Is(err, repository.ErrExternalMemoryStateConflict) {
			current, reloadErr := s.repo.GetMemoryWikiPublication(ctx, tenantID, publication.ID)
			if reloadErr == nil && current.Status == types.MemoryReviewStatusPublished &&
				current.PublishedPageID == page.ID && current.WikiRevisionID == revisionID {
				return page, nil
			}
			if reloadErr != nil {
				return nil, errors.Join(err, reloadErr)
			}
			if current.Status == types.MemoryReviewStatusRevoked {
				revokedProjection, projectionErr := s.repo.GetMemoryProjection(ctx, key)
				if projectionErr != nil {
					return nil, errors.Join(ErrMemorySourceRevoked, projectionErr)
				}
				if archiveErr := s.archiveRevokedMemoryWikiPage(ctx, revokedProjection, "publisher-compensation"); archiveErr != nil {
					return nil, errors.Join(ErrMemorySourceRevoked, archiveErr)
				}
				return nil, ErrMemorySourceRevoked
			}
		}
		// Wiki already succeeded. Keep the durable state at publishing so a
		// retry can find the stable slug and finish the checkpoint safely.
		return nil, err
	}
	return page, nil
}

func (s *Service) resolveTeamMemoryWiki(ctx context.Context, tenantID uint64, teamID, requestedID string) (*types.KnowledgeBase, error) {
	provisioningCtx := types.WithMemoryWikiProvisioning(ctx)
	kbs, err := s.kb.ListKnowledgeBasesByTenantID(provisioningCtx, tenantID)
	if err != nil {
		return nil, err
	}
	var matched *types.KnowledgeBase
	for _, candidate := range kbs {
		if candidate == nil || !candidate.HasMemoryWikiMarker() || candidate.MemoryTeamID != teamID {
			continue
		}
		// Persisted dedicated Wiki rows must already satisfy the zero-RAG
		// invariant. Do not normalize them in place: callers may share cached
		// model pointers, and mutating one here races concurrent publishers and
		// could silently turn a partially configured row into a valid target.
		if !candidate.IsDedicatedMemoryWiki() || matched != nil {
			return nil, ErrInvalidMemoryWikiTarget
		}
		matched = candidate
	}
	if matched != nil {
		if requestedID != "" && requestedID != matched.ID {
			return nil, ErrInvalidMemoryWikiTarget
		}
		return matched, nil
	}
	if requestedID != "" {
		return nil, ErrInvalidMemoryWikiTarget
	}
	created, createErr := s.kb.CreateKnowledgeBase(provisioningCtx, &types.KnowledgeBase{
		TenantID: tenantID, Name: "FMind Memory Wiki · " + teamID,
		Description: "Reviewed team L3 memory. Managed by FMind; zero RAG ingestion.",
		Type:        types.KnowledgeBaseTypeWiki, IsMemoryWiki: true, MemoryTeamID: teamID,
		WikiConfig:       &types.WikiConfig{IsMemoryWiki: true, MemoryTeamID: teamID},
		IndexingStrategy: types.IndexingStrategy{WikiEnabled: true},
	})
	if createErr == nil && created != nil && created.IsDedicatedMemoryWiki() {
		return created, nil
	}
	// A concurrent publisher may have won the partial unique index. Re-read
	// once and converge on that one team-owned KB.
	kbs, reloadErr := s.kb.ListKnowledgeBasesByTenantID(provisioningCtx, tenantID)
	if reloadErr != nil {
		return nil, errors.Join(createErr, reloadErr)
	}
	for _, candidate := range kbs {
		if candidate != nil && candidate.IsDedicatedMemoryWiki() && candidate.MemoryTeamID == teamID {
			return candidate, nil
		}
	}
	return nil, createErr
}

func (s *Service) getPublishedMemoryWikiPage(
	ctx context.Context,
	publication *types.MemoryWikiPublication,
	knowledgeBaseID string,
	slug string,
) (*types.WikiPage, error) {
	// The Wiki write happens before the publication checkpoint. A publisher
	// that loses the checkpoint CAS can therefore observe "published" while a
	// read replica (or a racing in-process gateway) still reports one miss.
	// Retry the read once; never create a replacement for an already-checkpointed
	// page because that could give the durable publication a different page ID.
	for attempt := 0; attempt < 2; attempt++ {
		page, err := s.wiki.GetPageBySlug(ctx, knowledgeBaseID, slug)
		if err == nil {
			if page == nil || page.ID != publication.PublishedPageID {
				return nil, repository.ErrWikiPageConflict
			}
			revision, revisionErr := s.repo.GetMemoryWikiRevision(ctx, publication.TenantID, publication.WikiRevisionID)
			if revisionErr != nil || revision.TenantID != publication.TenantID || revision.TeamID != publication.TeamID ||
				revision.BindingID != publication.BindingID || revision.UserID != publication.UserID ||
				revision.MemoryID != publication.MemoryID || revision.MemoryVersion != publication.MemoryVersion ||
				revision.SourcePublicationID != publication.ID || revision.SourceReviewTaskID != publication.ReviewTaskID ||
				revision.ContentChecksum != publication.ContentChecksum || revision.WikiPageID != page.ID || revision.WikiPageVersion != page.Version ||
				revision.Content != page.Content || revision.Title != page.Title || revision.Summary != page.Summary ||
				revision.PageStatus != page.Status {
				return nil, repository.ErrWikiPageConflict
			}
			projectionChecksum, checksumErr := memoryWikiPageProjectionChecksum(page)
			if checksumErr != nil || projectionChecksum != revision.ProjectionChecksum {
				return nil, repository.ErrWikiPageConflict
			}
			return page, nil
		}
		if !errors.Is(err, repository.ErrWikiPageNotFound) || attempt == 1 {
			return nil, err
		}
	}
	return nil, repository.ErrWikiPageNotFound
}

func (s *Service) createOrUpdateMemoryWikiPage(
	ctx context.Context,
	desired *types.WikiPage,
	projection *interfaces.ExternalMemoryProjection,
) (*types.WikiPage, *types.MemoryWikiRevision, error) {
	existing, err := s.wiki.GetPageBySlug(ctx, desired.KnowledgeBaseID, desired.Slug)
	if errors.Is(err, repository.ErrWikiPageNotFound) {
		created, createErr := s.wiki.CreatePage(ctx, desired)
		if createErr == nil {
			revision, revisionErr := s.persistMemoryWikiRevision(ctx, created, projection)
			return created, revision, revisionErr
		}
		// Another publisher may have won the unique (knowledge_base_id,
		// slug) race after our miss. Re-read and converge on that page before
		// treating the create error as a real publication failure.
		converged, getErr := s.wiki.GetPageBySlug(ctx, desired.KnowledgeBaseID, desired.Slug)
		if getErr != nil {
			return nil, nil, createErr
		}
		return s.convergeMemoryWikiPage(ctx, converged, desired, projection)
	}
	if err != nil {
		return nil, nil, err
	}
	return s.convergeMemoryWikiPage(ctx, existing, desired, projection)
}

func (s *Service) convergeMemoryWikiPage(
	ctx context.Context,
	existing, desired *types.WikiPage,
	projection *interfaces.ExternalMemoryProjection,
) (*types.WikiPage, *types.MemoryWikiRevision, error) {
	snapshot := projection.Snapshot
	version, _, memoryID, userID, metadataErr := memoryWikiPageProvenance(existing)
	if metadataErr != nil || memoryID != snapshot.MemoryID || userID != snapshot.UserID {
		return nil, nil, repository.ErrWikiPageConflict
	}
	currentProjectionChecksum, checksumErr := memoryWikiPageProjectionChecksum(existing)
	desiredProjectionChecksum := memoryWikiProjectionChecksum(snapshot)
	if checksumErr != nil {
		return nil, nil, repository.ErrWikiPageConflict
	}
	if version > snapshot.MemoryVersion {
		return nil, nil, ErrStaleMemoryWikiVersion
	}
	if currentProjectionChecksum == desiredProjectionChecksum {
		if version == snapshot.MemoryVersion {
			if !sameRenderedMemoryWikiPage(existing, desired) {
				return nil, nil, repository.ErrWikiPageConflict
			}
			revision, err := s.getOrCreateCurrentMemoryWikiRevision(ctx, existing, projection, version == snapshot.MemoryVersion)
			return existing, revision, err
		}
		// A distinct L3 version always advances the page lifecycle head, even
		// when its semantic projection is unchanged. This preserves the reviewed
		// version/review provenance and, critically, gives version-scoped revoke
		// a Wiki CAS fence. The stable page is reused and its knowledge semantics
		// remain unchanged; only exact replay of the same memory_version is a
		// physical zero-write no-op.
	}
	if version == snapshot.MemoryVersion {
		return nil, nil, repository.ErrWikiPageConflict
	}
	desired.ID = existing.ID
	desired.Version = existing.Version
	updated, updateErr := s.wiki.UpdatePage(ctx, desired)
	if updateErr != nil {
		current, getErr := s.wiki.GetPageBySlug(ctx, desired.KnowledgeBaseID, desired.Slug)
		if getErr == nil && current != nil {
			currentVersion, _, currentMemoryID, currentUserID, provenanceErr := memoryWikiPageProvenance(current)
			currentProjectionChecksum, projectionErr := memoryWikiPageProjectionChecksum(current)
			if provenanceErr == nil && projectionErr == nil && currentVersion == snapshot.MemoryVersion &&
				currentProjectionChecksum == desiredProjectionChecksum && currentMemoryID == snapshot.MemoryID &&
				currentUserID == snapshot.UserID && sameRenderedMemoryWikiPage(current, desired) {
				storedRevision, revisionErr := s.getOrCreateCurrentMemoryWikiRevision(ctx, current, projection, true)
				if revisionErr == nil {
					return current, storedRevision, nil
				}
			}
		}
		return nil, nil, updateErr
	}
	if updated == nil || updated.ID != existing.ID || updated.Version != existing.Version+1 || updated.Content != desired.Content {
		return nil, nil, repository.ErrWikiPageConflict
	}
	// Persist only the revision that actually won the Wiki optimistic lock.
	// A speculative pre-CAS revision can permanently poison retries when a
	// different L3 version wins first.
	revision, err := s.persistMemoryWikiRevision(ctx, updated, projection)
	if err != nil {
		return nil, nil, err
	}
	if revision.WikiPageID != updated.ID || revision.WikiPageVersion != updated.Version || revision.Content != updated.Content {
		return nil, nil, repository.ErrWikiPageConflict
	}
	return updated, revision, nil
}

func StableMemoryWikiSlug(tenantID uint64, teamID, bindingID, memoryID string) string {
	value := fmt.Sprintf("%d\x00%s\x00%s\x00%s", tenantID, teamID, bindingID, memoryID)
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("memory/%x", sum[:])
}

func StableMemoryWikiRevisionID(pageID, contentChecksum string, wikiPageVersion int) string {
	sum := sha256.Sum256([]byte(pageID + "\x00" + contentChecksum + "\x00" + strconv.Itoa(wikiPageVersion)))
	return fmt.Sprintf("mwr_%x", sum[:])
}

func (s *Service) persistMemoryWikiRevision(
	ctx context.Context,
	page *types.WikiPage,
	projection *interfaces.ExternalMemoryProjection,
) (*types.MemoryWikiRevision, error) {
	revision, err := buildMemoryWikiRevision(page, projection)
	if err != nil {
		return nil, err
	}
	stored, _, err := s.repo.CreateMemoryWikiRevision(ctx, revision)
	return stored, err
}

func buildMemoryWikiRevision(page *types.WikiPage, projection *interfaces.ExternalMemoryProjection) (*types.MemoryWikiRevision, error) {
	pageSnapshot, err := json.Marshal(page)
	if err != nil {
		return nil, err
	}
	snapshot := projection.Snapshot
	return &types.MemoryWikiRevision{
		ID:       StableMemoryWikiRevisionID(page.ID, memoryWikiProjectionChecksum(snapshot), page.Version),
		TenantID: snapshot.TenantID, TeamID: snapshot.TeamID, BindingID: snapshot.BindingID, UserID: snapshot.UserID,
		KnowledgeBaseID: page.KnowledgeBaseID, WikiPageID: page.ID, WikiPageVersion: page.Version, PageSlug: page.Slug,
		MemoryID: snapshot.MemoryID, MemoryVersion: snapshot.MemoryVersion,
		SourcePublicationID: projection.Publication.ID, SourceReviewTaskID: projection.ReviewTask.ID,
		ContentChecksum: snapshot.ContentChecksum, ProjectionChecksum: memoryWikiProjectionChecksum(snapshot),
		Title: page.Title, Summary: page.Summary, Content: page.Content,
		PageType: page.PageType, PageStatus: page.Status, SourceRefs: page.SourceRefs, ChunkRefs: page.ChunkRefs,
		PageMetadata: page.PageMetadata, PageSnapshot: types.JSON(pageSnapshot),
	}, nil
}

func (s *Service) getOrCreateCurrentMemoryWikiRevision(
	ctx context.Context,
	page *types.WikiPage,
	projection *interfaces.ExternalMemoryProjection,
	allowCreate bool,
) (*types.MemoryWikiRevision, error) {
	expected, err := buildMemoryWikiRevision(page, projection)
	if err != nil {
		return nil, err
	}
	revision, err := s.repo.GetMemoryWikiRevision(ctx, projection.Snapshot.TenantID, expected.ID)
	if errors.Is(err, repository.ErrMemoryWikiRevisionNotFound) && allowCreate {
		stored, _, createErr := s.repo.CreateMemoryWikiRevision(ctx, expected)
		return stored, createErr
	}
	if err != nil {
		return nil, err
	}
	if !sameExpectedMemoryWikiRevision(revision, expected) {
		return nil, repository.ErrMemoryWikiRevisionConflict
	}
	metadata, metadataErr := revision.PageMetadata.Map()
	if metadataErr != nil || metadata["claims_checksum"] != structuredClaimsChecksum(projection.Snapshot.Claims) {
		return nil, repository.ErrMemoryWikiRevisionConflict
	}
	return revision, nil
}

func sameExpectedMemoryWikiRevision(actual, expected *types.MemoryWikiRevision) bool {
	if actual == nil || expected == nil {
		return false
	}
	return actual.ID == expected.ID && actual.TenantID == expected.TenantID && actual.TeamID == expected.TeamID &&
		actual.BindingID == expected.BindingID && actual.UserID == expected.UserID &&
		actual.KnowledgeBaseID == expected.KnowledgeBaseID && actual.WikiPageID == expected.WikiPageID &&
		actual.WikiPageVersion == expected.WikiPageVersion && actual.PageSlug == expected.PageSlug &&
		actual.MemoryID == expected.MemoryID && actual.MemoryVersion == expected.MemoryVersion &&
		actual.SourcePublicationID == expected.SourcePublicationID && actual.SourceReviewTaskID == expected.SourceReviewTaskID &&
		actual.ContentChecksum == expected.ContentChecksum && actual.ProjectionChecksum == expected.ProjectionChecksum &&
		actual.Title == expected.Title && actual.Summary == expected.Summary && actual.Content == expected.Content &&
		actual.PageType == expected.PageType && actual.PageStatus == expected.PageStatus &&
		reflect.DeepEqual(actual.SourceRefs, expected.SourceRefs) && reflect.DeepEqual(actual.ChunkRefs, expected.ChunkRefs) &&
		sameSemanticJSON(actual.PageMetadata, expected.PageMetadata, false) &&
		sameSemanticJSON(actual.PageSnapshot, expected.PageSnapshot, true)
}

func sameSemanticJSON(leftRaw, rightRaw types.JSON, stripPageTimestamps bool) bool {
	var left any
	var right any
	if json.Unmarshal(leftRaw, &left) != nil || json.Unmarshal(rightRaw, &right) != nil {
		return string(leftRaw) == string(rightRaw)
	}
	if stripPageTimestamps {
		if value, ok := left.(map[string]any); ok {
			delete(value, "created_at")
			delete(value, "updated_at")
			delete(value, "deleted_at")
		}
		if value, ok := right.(map[string]any); ok {
			delete(value, "created_at")
			delete(value, "updated_at")
			delete(value, "deleted_at")
		}
	}
	return reflect.DeepEqual(left, right)
}

func validateClaimEvidenceCoverage(snapshot *types.MemoryL3Snapshot) error {
	seenClaims := make(map[string]struct{}, len(snapshot.Claims))
	available := make(map[string]struct{}, len(snapshot.EvidenceRefs))
	for _, reference := range snapshot.EvidenceRefs {
		if validateEvidenceReference(reference) == nil {
			available[evidenceReferenceKey(reference)] = struct{}{}
		}
	}
	for _, claim := range snapshot.Claims {
		if !claimIDPattern.MatchString(claim.ClaimID) || strings.TrimSpace(claim.Text) == "" ||
			!strings.Contains(snapshot.ContentMarkdown, claim.Text) {
			return ErrMemoryClaimSourceMismatch
		}
		if _, exists := seenClaims[claim.ClaimID]; exists {
			return ErrMemoryClaimSourceMismatch
		}
		seenClaims[claim.ClaimID] = struct{}{}
		if !claim.Factual {
			continue
		}
		covered := false
		for _, reference := range claim.Evidence {
			if validateEvidenceReference(reference) != nil {
				continue
			}
			if _, ok := available[evidenceReferenceKey(reference)]; ok {
				covered = true
				break
			}
		}
		if !covered {
			return ErrMemoryClaimEvidenceRequired
		}
	}
	return nil
}

func evidenceReferenceKey(reference types.EvidenceReference) string {
	return reference.Type + "\x00" + reference.ID + "\x00" + reference.Locator + "\x00" + reference.Checksum
}

func claimWikiLocator(claimID string) string { return "#" + claimID }

func renderStructuredClaims(claims types.ClaimEvidenceSet) string {
	var body strings.Builder
	body.WriteString("## Approved claims\n")
	for index, claim := range claims {
		fmt.Fprintf(&body, "\n<a id=%s></a>\n\n### Claim %d\n\n%s\n", strconv.Quote(claim.ClaimID), index+1, claim.Text)
	}
	return body.String()
}

func structuredClaimsChecksum(claims types.ClaimEvidenceSet) string {
	payload, _ := json.Marshal(claims)
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("sha256:%x", sum[:])
}

func renderMemoryWikiPage(projection *interfaces.ExternalMemoryProjection, knowledgeBaseID, slug string) (*types.WikiPage, error) {
	snapshot := projection.Snapshot
	review := projection.ReviewTask
	metadataBytes, err := json.Marshal(map[string]any{
		"schema": "fmind.cognition/v1", "source_type": "memory_l3",
		"source_memory_id": snapshot.MemoryID, "source_memory_version": snapshot.MemoryVersion,
		"source_team_id": snapshot.TeamID, "source_binding_id": snapshot.BindingID,
		"source_user_id": snapshot.UserID, "source_review_id": review.ID,
		"content_checksum": snapshot.ContentChecksum, "projection_checksum": memoryWikiProjectionChecksum(snapshot),
		"claims_checksum": structuredClaimsChecksum(snapshot.Claims),
	})
	if err != nil {
		return nil, err
	}
	reviewedAt := ""
	if review.ReviewedAt != nil {
		reviewedAt = review.ReviewedAt.UTC().Format(time.RFC3339Nano)
	}
	structuredClaims := renderStructuredClaims(snapshot.Claims)
	content := fmt.Sprintf(`---
schema: fmind.cognition/v1
title: %s
tenant_id: %d
team_id: %s
binding_id: %s
user_id: %s
source_type: memory_l3
source_memory_id: %s
source_memory_version: %d
confidence: %s
sensitivity: %s
review_status: approved
reviewed_at: %s
content_checksum: %s
claims_checksum: %s
---

# %s

## Mature memory document

%s

%s

## 来源说明

该页面由审核通过的团队 L3 记忆生成，原始证据保留在记忆系统中。
`, strconv.Quote(snapshot.Title), snapshot.TenantID, strconv.Quote(snapshot.TeamID),
		strconv.Quote(snapshot.BindingID), strconv.Quote(snapshot.UserID), strconv.Quote(snapshot.MemoryID), snapshot.MemoryVersion,
		strconv.FormatFloat(snapshot.Confidence, 'f', -1, 64), strconv.Quote(snapshot.Sensitivity),
		strconv.Quote(reviewedAt), strconv.Quote(snapshot.ContentChecksum), strconv.Quote(structuredClaimsChecksum(snapshot.Claims)),
		snapshot.Title, snapshot.ContentMarkdown, structuredClaims)
	return &types.WikiPage{
		TenantID: snapshot.TenantID, KnowledgeBaseID: knowledgeBaseID, Slug: slug,
		Title: snapshot.Title, PageType: types.WikiPageTypeSummary, Status: types.WikiPageStatusPublished,
		Content: content, Summary: snapshot.Claims[0].Text,
		SourceRefs: types.StringArray{
			fmt.Sprintf("memory:%s@%d", snapshot.MemoryID, snapshot.MemoryVersion),
			"binding:" + snapshot.BindingID, "user:" + snapshot.UserID, "review:" + review.ID,
		},
		ChunkRefs: nil, PageMetadata: types.JSON(metadataBytes),
	}, nil
}

func memoryWikiPageProvenance(page *types.WikiPage) (uint64, string, string, string, error) {
	metadata, err := page.PageMetadata.Map()
	if err != nil {
		return 0, "", "", "", err
	}
	memoryID, _ := metadata["source_memory_id"].(string)
	userID, _ := metadata["source_user_id"].(string)
	checksum, _ := metadata["content_checksum"].(string)
	var version uint64
	switch value := metadata["source_memory_version"].(type) {
	case float64:
		version = uint64(value)
	case json.Number:
		parsed, parseErr := strconv.ParseUint(value.String(), 10, 64)
		if parseErr != nil {
			return 0, "", "", "", parseErr
		}
		version = parsed
	}
	if metadata["schema"] != "fmind.cognition/v1" || metadata["source_type"] != "memory_l3" ||
		memoryID == "" || userID == "" || checksum == "" || version == 0 ||
		strings.TrimSpace(fmt.Sprint(metadata["source_team_id"])) == "" ||
		strings.TrimSpace(fmt.Sprint(metadata["source_binding_id"])) == "" ||
		strings.TrimSpace(fmt.Sprint(metadata["source_review_id"])) == "" ||
		!sha256ChecksumPattern.MatchString(fmt.Sprint(metadata["claims_checksum"])) ||
		!sha256ChecksumPattern.MatchString(fmt.Sprint(metadata["projection_checksum"])) {
		return 0, "", "", "", errors.New("memory Wiki provenance is incomplete")
	}
	return version, checksum, memoryID, userID, nil
}

func memoryWikiProjectionChecksum(snapshot *types.MemoryL3Snapshot) string {
	payload, _ := json.Marshal(struct {
		TenantID        uint64                 `json:"tenant_id"`
		TeamID          string                 `json:"team_id"`
		BindingID       string                 `json:"binding_id"`
		UserID          string                 `json:"user_id"`
		MemoryID        string                 `json:"memory_id"`
		Title           string                 `json:"title"`
		Confidence      float64                `json:"confidence"`
		Sensitivity     string                 `json:"sensitivity"`
		ContentChecksum string                 `json:"content_checksum"`
		Claims          types.ClaimEvidenceSet `json:"claims"`
	}{
		TenantID: snapshot.TenantID, TeamID: snapshot.TeamID, BindingID: snapshot.BindingID,
		UserID: snapshot.UserID, MemoryID: snapshot.MemoryID, Title: snapshot.Title,
		Confidence: snapshot.Confidence, Sensitivity: snapshot.Sensitivity,
		ContentChecksum: snapshot.ContentChecksum, Claims: snapshot.Claims,
	})
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("sha256:%x", sum[:])
}

func memoryWikiPageProjectionChecksum(page *types.WikiPage) (string, error) {
	metadata, err := page.PageMetadata.Map()
	if err != nil {
		return "", err
	}
	checksum, _ := metadata["projection_checksum"].(string)
	if !sha256ChecksumPattern.MatchString(checksum) {
		return "", errors.New("memory Wiki projection checksum is invalid")
	}
	return checksum, nil
}

func sameRenderedMemoryWikiPage(current, desired *types.WikiPage) bool {
	if !sameVisibleMemoryWikiPage(current, desired) {
		return false
	}
	currentMetadata, currentErr := current.PageMetadata.Map()
	desiredMetadata, desiredErr := desired.PageMetadata.Map()
	if currentErr != nil || desiredErr != nil {
		return false
	}
	currentJSON, _ := json.Marshal(currentMetadata)
	desiredJSON, _ := json.Marshal(desiredMetadata)
	return string(currentJSON) == string(desiredJSON)
}

func sameVisibleMemoryWikiPage(current, desired *types.WikiPage) bool {
	return current != nil && desired != nil &&
		current.Title == desired.Title && current.Content == desired.Content &&
		current.Summary == desired.Summary && current.PageType == desired.PageType && current.Status == desired.Status &&
		strings.Join(current.SourceRefs, "\x00") == strings.Join(desired.SourceRefs, "\x00") &&
		strings.Join(current.ChunkRefs, "\x00") == strings.Join(desired.ChunkRefs, "\x00")
}

func publicationProjectionKey(publication *types.MemoryWikiPublication) types.MemoryProjectionKey {
	return types.MemoryProjectionKey{
		TenantID: publication.TenantID, TeamID: publication.TeamID, BindingID: publication.BindingID,
		MemoryID: publication.MemoryID, MemoryVersion: publication.MemoryVersion,
	}
}
