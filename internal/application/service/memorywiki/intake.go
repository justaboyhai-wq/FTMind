package memorywiki

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/justaboyhai-wq/fmind/internal/application/repository"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

const (
	MaxTrustedL3MarkdownBytes = 1 << 20
	MaxTrustedL3EvidenceBytes = 256 << 10
	maxTrustedL3SummaryBytes  = 64 << 10
	maxTrustedL3IdentifierLen = 128
)

var sha256ChecksumPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var claimIDPattern = regexp.MustCompile(`^claim-[a-z0-9](?:[a-z0-9_-]{0,120})$`)

func (s *Service) ReceiveTrustedL3Event(ctx context.Context, event types.TrustedL3Event) (*interfaces.ExternalMemoryProjection, bool, error) {
	if err := validateTrustedL3Event(event); err != nil {
		return nil, false, err
	}
	if event.EventType == types.MemoryL3EventRevoked {
		return s.receiveTrustedL3Revocation(ctx, event)
	}
	now := time.Now().UTC()
	eventRecord := &types.MemoryIntegrationEvent{
		ID: uuid.NewString(), EventID: event.EventID, EventType: event.EventType,
		SchemaVersion: event.SchemaVersion, OccurredAt: event.OccurredAt,
		TenantID: event.TenantID, DepartmentID: event.DepartmentID, WorkspaceID: event.WorkspaceID,
		ProjectID: event.ProjectID, TeamID: event.TeamID, BindingID: event.BindingID,
		UserID: event.UserID, AgentID: event.AgentID, TaskID: event.TaskID, MemoryID: event.MemoryID,
		MemoryVersion: event.MemoryVersion, ContentChecksum: event.ContentChecksum,
		Status: types.MemoryIntegrationEventProcessed, EventClass: types.MemoryIntegrationEventClassProjection,
		AttemptCount: 1, ProcessedAt: &now,
	}
	snapshot := &types.MemoryL3Snapshot{
		ID: uuid.NewString(), EventID: event.EventID, TenantID: event.TenantID,
		DepartmentID: event.DepartmentID, WorkspaceID: event.WorkspaceID, ProjectID: event.ProjectID,
		TeamID: event.TeamID, BindingID: event.BindingID, UserID: event.UserID, AgentID: event.AgentID, TaskID: event.TaskID,
		MemoryID: event.MemoryID, MemoryVersion: event.MemoryVersion,
		MemoryLevel: event.MemoryLevel, Maturity: event.Maturity, Title: event.Title,
		Summary: event.Summary, ContentMarkdown: event.ContentMarkdown,
		Confidence: event.Confidence, Sensitivity: event.Sensitivity,
		EvidenceRefs: event.EvidenceRefs, Claims: event.Claims, ContentChecksum: event.ContentChecksum,
	}
	review := &types.MemoryReviewTask{
		ID: uuid.NewString(), SnapshotID: snapshot.ID, EventID: event.EventID,
		TenantID: event.TenantID, DepartmentID: event.DepartmentID, WorkspaceID: event.WorkspaceID,
		ProjectID: event.ProjectID, TeamID: event.TeamID, BindingID: event.BindingID,
		UserID: event.UserID, AgentID: event.AgentID, TaskID: event.TaskID, MemoryID: event.MemoryID,
		MemoryVersion: event.MemoryVersion, TitleSnapshot: event.Title,
		ContentSnapshot: event.ContentMarkdown, EvidenceSnapshot: event.EvidenceRefs,
		ClaimsSnapshot: event.Claims, ContentChecksum: event.ContentChecksum,
		Status: types.MemoryReviewStatusPendingReview, LockVersion: 1,
	}
	publication := &types.MemoryWikiPublication{
		ID: uuid.NewString(), SnapshotID: snapshot.ID, ReviewTaskID: review.ID, EventID: event.EventID,
		TenantID: event.TenantID, DepartmentID: event.DepartmentID, WorkspaceID: event.WorkspaceID,
		ProjectID: event.ProjectID, TeamID: event.TeamID, BindingID: event.BindingID,
		UserID: event.UserID, AgentID: event.AgentID, TaskID: event.TaskID, MemoryID: event.MemoryID,
		MemoryVersion: event.MemoryVersion, Title: event.Title, Markdown: event.ContentMarkdown,
		Evidence: evidenceReferenceStrings(event.EvidenceRefs), ContentChecksum: event.ContentChecksum,
		Status: types.MemoryReviewStatusPendingReview, LockVersion: 1,
	}
	return s.repo.CreateMaturedMemoryProjection(ctx, eventRecord, snapshot, review, publication)
}

func (s *Service) archiveRevokedMemoryWikiPage(ctx context.Context, projection *interfaces.ExternalMemoryProjection, eventID string) error {
	if projection == nil || projection.Snapshot == nil || projection.Publication == nil || projection.Publication.KnowledgeBaseID == "" {
		return nil
	}
	publication := projection.Publication
	ctx = types.WithMemoryWikiMutation(ctx)
	slug := StableMemoryWikiSlug(publication.TenantID, publication.TeamID, publication.BindingID, publication.MemoryID)
	page, err := s.wiki.GetPageBySlug(ctx, publication.KnowledgeBaseID, slug)
	if errors.Is(err, repository.ErrWikiPageNotFound) && publication.PublishedPageID == "" {
		// A first-version publisher may already have passed its durable status
		// check and be paused immediately before CreatePage. Claim the stable
		// slug with an archived tombstone so that stale create cannot make the
		// revoked memory visible after this event has been acknowledged.
		tombstone, renderErr := renderMemoryWikiPage(projection, publication.KnowledgeBaseID, slug)
		if renderErr != nil {
			return renderErr
		}
		tombstone.Title = "Revoked memory"
		tombstone.Summary = "This reviewed memory version has been revoked."
		tombstone.Content = "# Revoked memory\n\nThis reviewed memory version is no longer available."
		tombstone.SourceRefs = nil
		tombstone.ChunkRefs = nil
		if applyErr := applyMemoryWikiRevocation(tombstone, publication.MemoryVersion, eventID); applyErr != nil {
			return applyErr
		}
		if _, createErr := s.wiki.CreatePage(ctx, tombstone); createErr == nil {
			return nil
		}
		// If the publisher won the slug race, load its page and archive it with
		// the same expected-version fence below.
		page, err = s.wiki.GetPageBySlug(ctx, publication.KnowledgeBaseID, slug)
	}
	if err != nil {
		return err
	}
	if page == nil || (publication.PublishedPageID != "" && page.ID != publication.PublishedPageID) {
		return repository.ErrWikiPageConflict
	}
	version, _, memoryID, userID, provenanceErr := memoryWikiPageProvenance(page)
	if provenanceErr != nil || memoryID != publication.MemoryID || userID != publication.UserID {
		return repository.ErrWikiPageConflict
	}
	if version != publication.MemoryVersion {
		// Revocation is strictly version-scoped. A delayed v1 revoke cannot hide
		// v2, and a v2 revoke cannot hide the still-visible v1 while v2 is only
		// in the publishing saga and has not won the Wiki CAS.
		return nil
	}
	metadata := map[string]any{}
	if len(page.PageMetadata) > 0 {
		if err := json.Unmarshal(page.PageMetadata, &metadata); err != nil {
			return repository.ErrWikiPageConflict
		}
	}
	if page.Status == types.WikiPageStatusArchived && metadata["memory_status"] == types.MemoryReviewStatusRevoked &&
		metadata["memory_revoked_version"] == strconv.FormatUint(publication.MemoryVersion, 10) {
		// The source revocation event is authoritative. A later publisher
		// compensation for the same version must converge without replacing its
		// provenance with the synthetic "publisher-compensation" marker.
		return nil
	}
	if err := applyMemoryWikiRevocation(page, publication.MemoryVersion, eventID); err != nil {
		return err
	}
	_, err = s.wiki.UpdatePage(ctx, page)
	return err
}

func applyMemoryWikiRevocation(page *types.WikiPage, memoryVersion uint64, eventID string) error {
	if page == nil {
		return repository.ErrWikiPageConflict
	}
	metadata := map[string]any{}
	if len(page.PageMetadata) > 0 {
		if err := json.Unmarshal(page.PageMetadata, &metadata); err != nil {
			return repository.ErrWikiPageConflict
		}
	}
	metadata["memory_status"] = types.MemoryReviewStatusRevoked
	metadata["memory_revoked_event_id"] = eventID
	metadata["memory_revoked_version"] = strconv.FormatUint(memoryVersion, 10)
	metadata["memory_revoked_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	page.Status = types.WikiPageStatusArchived
	page.PageMetadata = types.JSON(encoded)
	return nil
}

func (s *Service) receiveTrustedL3Revocation(ctx context.Context, event types.TrustedL3Event) (*interfaces.ExternalMemoryProjection, bool, error) {
	key := event.ProjectionKey()
	current, err := s.repo.GetMemoryProjection(ctx, key)
	if err != nil {
		return nil, false, err
	}
	if current.Snapshot == nil || current.Snapshot.DepartmentID != event.DepartmentID ||
		current.Snapshot.WorkspaceID != event.WorkspaceID || current.Snapshot.ProjectID != event.ProjectID ||
		current.Snapshot.UserID != event.UserID || current.Snapshot.AgentID != event.AgentID || current.Snapshot.TaskID != event.TaskID {
		return nil, false, errors.New("memory revocation scope does not match stored projection")
	}
	now := time.Now().UTC()
	record := &types.MemoryIntegrationEvent{
		ID: uuid.NewString(), EventID: event.EventID, EventType: event.EventType,
		EventClass:    types.MemoryIntegrationEventClassRevocation,
		SchemaVersion: event.SchemaVersion, OccurredAt: event.OccurredAt,
		TenantID: event.TenantID, DepartmentID: event.DepartmentID, WorkspaceID: event.WorkspaceID,
		ProjectID: event.ProjectID, TeamID: event.TeamID, BindingID: event.BindingID,
		UserID: event.UserID, AgentID: event.AgentID, TaskID: event.TaskID, MemoryID: event.MemoryID,
		MemoryVersion: event.MemoryVersion, ContentChecksum: current.Snapshot.ContentChecksum,
		Status: types.MemoryIntegrationEventProcessed, AttemptCount: 1, ProcessedAt: &now,
	}
	projection, duplicate, err := s.repo.RevokeMemoryProjection(ctx, record)
	if err != nil {
		return nil, false, err
	}
	if err := s.archiveRevokedMemoryWikiPage(ctx, projection, event.EventID); err != nil {
		return projection, duplicate, err
	}
	return projection, duplicate, nil
}

// ValidateTrustedL3Event exposes the frozen intake contract to the signed HTTP
// adapter so permanent schema failures can return 422 while repository outages
// remain retryable 503 responses.
func ValidateTrustedL3Event(event types.TrustedL3Event) error {
	return validateTrustedL3Event(event)
}

func validateTrustedL3Event(event types.TrustedL3Event) error {
	if event.EventType != types.MemoryL3EventMatured && event.EventType != types.MemoryL3EventUpdated && event.EventType != types.MemoryL3EventRevoked {
		return errors.New("unsupported L3 lifecycle event")
	}
	if event.SchemaVersion != "1.0" {
		return errors.New("unsupported L3 event schema version")
	}
	if event.OccurredAt.IsZero() {
		return errors.New("occurred_at is required")
	}
	if event.MemoryLevel != "L3" {
		return errors.New("only L3 memory may enter review")
	}
	if event.TenantID == 0 || event.MemoryVersion == 0 {
		return errors.New("tenant and positive memory version are required")
	}
	requiredIdentifiers := map[string]string{
		"event_id": event.EventID, "team_id": event.TeamID, "binding_id": event.BindingID,
		"user_id": event.UserID, "agent_id": event.AgentID, "memory_id": event.MemoryID,
	}
	for name, value := range requiredIdentifiers {
		if strings.TrimSpace(value) == "" || len(value) > maxTrustedL3IdentifierLen {
			return fmt.Errorf("%s is required and must be at most %d bytes", name, maxTrustedL3IdentifierLen)
		}
	}
	optionalIdentifiers := map[string]string{
		"department_id": event.DepartmentID, "workspace_id": event.WorkspaceID,
		"project_id": event.ProjectID, "task_id": event.TaskID,
	}
	for name, value := range optionalIdentifiers {
		if value == "" {
			continue
		}
		if strings.TrimSpace(value) == "" || len(value) > maxTrustedL3IdentifierLen {
			return fmt.Errorf("%s, when present, must be non-blank and at most %d bytes", name, maxTrustedL3IdentifierLen)
		}
	}
	if event.EventType == types.MemoryL3EventRevoked {
		if event.Maturity != "revoked" {
			return errors.New("revoked L3 event must declare revoked maturity")
		}
		return nil
	}
	if event.Maturity != "matured" {
		return errors.New("only mature L3 memory may enter review")
	}
	if strings.TrimSpace(event.Title) == "" || len(event.Title) > 512 {
		return errors.New("title is required and must be at most 512 bytes")
	}
	if strings.TrimSpace(event.Summary) == "" || len(event.Summary) > maxTrustedL3SummaryBytes {
		return errors.New("summary is required and too large")
	}
	if strings.TrimSpace(event.ContentMarkdown) == "" || len(event.ContentMarkdown) > MaxTrustedL3MarkdownBytes {
		return errors.New("content_markdown is required and must be at most 1 MiB")
	}
	if math.IsNaN(event.Confidence) || math.IsInf(event.Confidence, 0) || event.Confidence < 0 || event.Confidence > 1 {
		return errors.New("confidence must be between zero and one")
	}
	switch event.Sensitivity {
	case "public", "internal", "confidential", "restricted":
	default:
		return errors.New("unsupported sensitivity")
	}
	expectedChecksum := checksumMemoryMarkdown(event.ContentMarkdown)
	if !sha256ChecksumPattern.MatchString(event.ContentChecksum) || event.ContentChecksum != expectedChecksum {
		return errors.New("content checksum does not match content_markdown")
	}
	if len(event.EvidenceRefs) == 0 {
		return errors.New("at least one evidence reference is required")
	}
	for _, reference := range event.EvidenceRefs {
		if err := validateEvidenceReference(reference); err != nil {
			return err
		}
	}
	if len(event.Claims) == 0 {
		return errors.New("at least one structured claim is required")
	}
	seenClaims := make(map[string]struct{}, len(event.Claims))
	for _, claim := range event.Claims {
		if strings.TrimSpace(claim.ClaimID) == "" || strings.TrimSpace(claim.Text) == "" {
			return errors.New("claim id and text are required")
		}
		if !claimIDPattern.MatchString(claim.ClaimID) {
			return errors.New("claim id must use the safe claim-<lowercase-id> form")
		}
		if _, exists := seenClaims[claim.ClaimID]; exists {
			return errors.New("claim ids must be unique")
		}
		seenClaims[claim.ClaimID] = struct{}{}
		if !strings.Contains(event.ContentMarkdown, claim.Text) {
			return errors.New("claim text must occur verbatim in content_markdown")
		}
		for _, reference := range claim.Evidence {
			if err := validateEvidenceReference(reference); err != nil {
				return err
			}
		}
	}
	evidenceJSON, err := json.Marshal(struct {
		Evidence types.EvidenceReferences `json:"evidence_refs"`
		Claims   types.ClaimEvidenceSet   `json:"claims"`
	}{event.EvidenceRefs, event.Claims})
	if err != nil {
		return fmt.Errorf("marshal evidence: %w", err)
	}
	if len(evidenceJSON) > MaxTrustedL3EvidenceBytes {
		return errors.New("evidence metadata must be at most 256 KiB")
	}
	return nil
}

func validateEvidenceReference(reference types.EvidenceReference) error {
	if strings.TrimSpace(reference.Type) == "" || strings.TrimSpace(reference.ID) == "" || strings.TrimSpace(reference.Locator) == "" {
		return errors.New("evidence type, id, and locator are required")
	}
	if len(reference.Type) > 64 || len(reference.ID) > maxTrustedL3IdentifierLen || len(reference.Locator) > MaxTrustedL3EvidenceBytes {
		return errors.New("evidence locator is too large")
	}
	if reference.Checksum != "" && !sha256ChecksumPattern.MatchString(reference.Checksum) {
		return errors.New("evidence checksum is invalid")
	}
	return nil
}

func checksumMemoryMarkdown(content string) string {
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("sha256:%x", sum[:])
}

func evidenceReferenceStrings(references types.EvidenceReferences) types.StringArray {
	result := make(types.StringArray, 0, len(references))
	for _, reference := range references {
		result = append(result, fmt.Sprintf("%s:%s#%s", reference.Type, reference.ID, reference.Locator))
	}
	return result
}
