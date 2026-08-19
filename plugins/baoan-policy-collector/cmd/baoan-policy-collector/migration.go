package main

// This file implements the one-time Baoan knowledge migration.  It deliberately
// updates only title, file_name and metadata/tag relations; chunks, vectors,
// physical storage paths and knowledge IDs are never rewritten.

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/canonical"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/model"
	"github.com/mmcdole/gofeed"
)

type migrationEntry struct {
	PolicyID     string             `json:"policy_id"`
	KnowledgeID  string             `json:"knowledge_id"`
	OldTitle     string             `json:"old_title"`
	OldFileName  string             `json:"old_file_name"`
	OldMetadata  json.RawMessage    `json:"old_metadata,omitempty"`
	OldTagIDs    []string           `json:"old_tag_ids,omitempty"`
	OldWikiPages []wikiPageRollback `json:"old_wiki_pages,omitempty"`
	NewTitle     string             `json:"new_title"`
	NewFileName  string             `json:"new_file_name"`
	ExternalID   string             `json:"external_id"`
	Tags         []string           `json:"tags,omitempty"`
	RejectedTags []string           `json:"rejected_official_tags,omitempty"`
}

type wikiPageRollback struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

type migrationRollback struct {
	GeneratedAt     time.Time        `json:"generated_at"`
	KnowledgeBaseID string           `json:"knowledge_base_id"`
	RollbackReady   bool             `json:"rollback_ready"`
	Entries         []migrationEntry `json:"entries"`
}

type knowledgeMigrationRecord struct {
	PolicyID    string
	KnowledgeID string
	Title       string
	FileName    string
	Metadata    json.RawMessage
	TagIDs      []string
	WikiPages   []wikiPageRollback
}

type migrationMatchSummary struct {
	Matched            int
	ArchiveMissingInKB int
}

type adoptionDataSource struct {
	ID             string
	Type           string
	Status         string
	Config         []byte
	LastSyncCursor []byte
}

type migrationChange struct {
	Knowledge bool
	Wiki      bool
	Tags      bool
}

// last_sync_cursor is json/jsonb in production. Cast it to text before
// COALESCE so PostgreSQL never attempts to parse the empty-string fallback as
// JSON (SQLSTATE 22P02).
const adoptionDataSourceQuery = `SELECT id,type,status,config,COALESCE(last_sync_cursor::text,'') FROM data_sources WHERE id=$1 AND knowledge_base_id=$2 AND deleted_at IS NULL`

func (c migrationChange) Any() bool {
	return c.Knowledge || c.Wiki || c.Tags
}

func runMigrate(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	kbID := fs.String("kb-id", "", "target knowledge base ID")
	feedURL := fs.String("feed-url", "", "stable RSS feed URL")
	dataDir := fs.String("data-dir", "", "canonical policy archive or exported raw directory")
	dryRun := fs.Bool("dry-run", true, "validate and print mappings without changing the database")
	apply := fs.Bool("apply", false, "apply the validated migration to PostgreSQL")
	rollbackApply := fs.Bool("rollback", false, "restore names and metadata from --rollback-file")
	rollback := fs.String("rollback-file", "baoan-policy-migration-rollback.json", "rollback manifest path")
	dbURL := fs.String("db-url", os.Getenv("DATABASE_URL"), "PostgreSQL URL (or DATABASE_URL)")
	datasourceID := fs.String("datasource-id", "", "existing FMind RSS data source ID; seed its cursor after apply")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *apply {
		*dryRun = false
		if strings.TrimSpace(*datasourceID) == "" {
			return errors.New("--apply requires --datasource-id so adopted documents retain RSS ownership")
		}
	}
	if *rollbackApply {
		data, err := os.ReadFile(*rollback)
		if err != nil {
			return fmt.Errorf("read rollback manifest: %w", err)
		}
		var manifest migrationRollback
		if err := json.Unmarshal(data, &manifest); err != nil {
			return fmt.Errorf("parse rollback manifest: %w", err)
		}
		if *dbURL == "" {
			return errors.New("--rollback requires --db-url or DATABASE_URL")
		}
		return rollbackMigration(context.Background(), *dbURL, manifest)
	}
	if *kbID == "" || *feedURL == "" || *dataDir == "" {
		return errors.New("--kb-id, --feed-url and --data-dir are required")
	}
	entries, err := buildMigrationEntries(*kbID, *feedURL, *dataDir)
	if err != nil {
		return err
	}
	summary := migrationMatchSummary{ArchiveMissingInKB: len(entries)}
	if *dbURL != "" {
		entries, summary, err = enrichEntriesFromDB(*dbURL, *kbID, entries)
		if err != nil {
			return err
		}
	} else if *apply {
		return errors.New("--apply requires --db-url or DATABASE_URL")
	}
	fmt.Printf("migration mapping: matched=%d archive_missing_in_kb=%d\n", summary.Matched, summary.ArchiveMissingInKB)
	manifest := migrationRollback{GeneratedAt: time.Now().UTC(), KnowledgeBaseID: *kbID, RollbackReady: *dbURL != "", Entries: entries}
	encoded, _ := json.MarshalIndent(manifest, "", "  ")
	if *apply {
		if existing, readErr := os.ReadFile(*rollback); readErr == nil {
			var prior migrationRollback
			if err := json.Unmarshal(existing, &prior); err != nil {
				return fmt.Errorf("existing rollback file is invalid: %w", err)
			}
			if prior.KnowledgeBaseID != *kbID {
				return fmt.Errorf("rollback file %s belongs to knowledge base %s", *rollback, prior.KnowledgeBaseID)
			}
			// Preserve the original pre-migration rollback on repeated idempotent
			// applies; never overwrite the only recovery point with new names. A
			// dry-run created without database access has no old tags/wiki state,
			// so replace that incomplete manifest before the first apply.
			if !prior.RollbackReady {
				if err := os.WriteFile(*rollback, append(encoded, '\n'), 0o600); err != nil {
					return fmt.Errorf("replace incomplete rollback manifest: %w", err)
				}
			}
		} else if !os.IsNotExist(readErr) {
			return fmt.Errorf("check rollback manifest: %w", readErr)
		} else if err := os.WriteFile(*rollback, append(encoded, '\n'), 0o600); err != nil {
			return fmt.Errorf("write rollback manifest: %w", err)
		}
	} else if err := os.WriteFile(*rollback, append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("write rollback manifest: %w", err)
	}
	for _, e := range entries {
		fmt.Printf("%s\t%s\t%s -> %s\t%s\n", e.PolicyID, e.KnowledgeID, e.OldFileName, e.NewFileName, strings.Join(e.Tags, ","))
		if len(e.RejectedTags) > 0 {
			fmt.Printf("%s\trejected_official_tags=%s\n", e.PolicyID, strings.Join(e.RejectedTags, ","))
		}
	}
	if *dryRun {
		return nil
	}
	return applyMigration(context.Background(), *dbURL, manifest, *datasourceID, *feedURL)
}

var postIDPattern = regexp.MustCompile(`post_[0-9]+`)
var migratablePolicyFilenamePattern = regexp.MustCompile(`^(?:raw/)?(post_[0-9]+)\.md$`)
var migratedPolicyFilenamePattern = regexp.MustCompile(`^.+(?:\x{FF08}|\()(post_[0-9]+)(?:\x{FF09}|\))\.md$`)
var rssPolicyGUIDPattern = regexp.MustCompile(`^baoan-policy:(post_[0-9]+)$`)

func strictMigratablePolicyID(fileName string) string {
	fileName = strings.ReplaceAll(fileName, "\\", "/")
	matches := migratablePolicyFilenamePattern.FindStringSubmatch(fileName)
	if len(matches) == 2 {
		return matches[1]
	}
	// An already migrated policy uses a canonical safe display name ending in
	// “（post_N）.md”.  Accept that exact basename on re-runs so the migration
	// remains idempotent, while still rejecting arbitrary path fragments and
	// loose substring matches.
	if strings.Contains(fileName, "/") {
		return ""
	}
	matches = migratedPolicyFilenamePattern.FindStringSubmatch(fileName)
	if len(matches) == 2 {
		return matches[1]
	}
	return ""
}

func filterMigrationOfficialTags(tags []string, dimensions map[string][]string) ([]string, []string) {
	accepted, rejected := canonical.FilterOfficialTags(tags, dimensions)
	sort.Strings(accepted)
	sort.Strings(rejected)
	return accepted, rejected
}

var officialPolicyTagPrefixes = []string{"\u670d\u52a1\u5bf9\u8c61/", "\u53d1\u6587\u673a\u6784/", "\u4e3b\u9898/", "\u6587\u4ef6\u8f7d\u4f53/", "\u6587\u4ef6\u7c7b\u578b/", "\u5173\u8054\u5185\u5bb9/", "\u7533\u62a5\u72b6\u6001/"}

func isOfficialPolicyTagName(name string) bool {
	for _, prefix := range officialPolicyTagPrefixes {
		if strings.HasPrefix(name, prefix) && strings.TrimSpace(strings.TrimPrefix(name, prefix)) != "" {
			return true
		}
	}
	return false
}

func migrationMetadata(raw []byte, externalID, datasourceID, sourceResourceID string, tags []string, contentSignal ...string) (map[string]interface{}, error) {
	metadata := map[string]interface{}{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return nil, fmt.Errorf("parse existing knowledge metadata: %w", err)
		}
	}
	encodedTags, err := json.Marshal(tags)
	if err != nil {
		return nil, err
	}
	metadata["external_id"] = externalID
	metadata["datasource_id"] = datasourceID
	metadata["source_resource_id"] = sourceResourceID
	metadata["official_tag_names"] = string(encodedTags)
	metadata["official_tag_source"] = "website"
	metadata["guid"] = strings.TrimPrefix(externalID, strings.TrimRight(sourceResourceID, "/")+":")
	if len(contentSignal) > 0 && contentSignal[0] != "" {
		metadata["rss_content_signal"] = contentSignal[0]
	}
	return metadata, nil
}

func validateAdoptionDataSource(ds adoptionDataSource, datasourceID, feedURL string) error {
	if ds.ID != datasourceID {
		return fmt.Errorf("RSS data source %s was not found", datasourceID)
	}
	if ds.Type != "rss" {
		return fmt.Errorf("data source %s has type %q, want rss", datasourceID, ds.Type)
	}
	if ds.Status != "paused" {
		return fmt.Errorf("data source %s has status %q, want paused", datasourceID, ds.Status)
	}
	var config struct {
		Settings    map[string]interface{} `json:"settings"`
		Credentials map[string]interface{} `json:"credentials"`
	}
	if err := json.Unmarshal(ds.Config, &config); err != nil {
		return fmt.Errorf("parse RSS data source config: %w", err)
	}
	configured := ""
	if raw, ok := config.Settings["feed_urls"].(string); ok {
		configured = raw
	}
	if configured == "" {
		if raw, ok := config.Credentials["feed_urls"].(string); ok {
			configured = raw
		}
	}
	for _, value := range strings.FieldsFunc(configured, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
		if strings.TrimSpace(value) == strings.TrimSpace(feedURL) {
			return nil
		}
	}
	return fmt.Errorf("data source %s is not configured for RSS feed %s", datasourceID, feedURL)
}

func validateAdoptionDataSourceInDB(ctx context.Context, db *sql.DB, kbID, datasourceID, feedURL string) (adoptionDataSource, error) {
	var ds adoptionDataSource
	if err := db.QueryRowContext(ctx, adoptionDataSourceQuery, datasourceID, kbID).Scan(&ds.ID, &ds.Type, &ds.Status, &ds.Config, &ds.LastSyncCursor); err != nil {
		return adoptionDataSource{}, err
	}
	if err := validateAdoptionDataSource(ds, datasourceID, feedURL); err != nil {
		return adoptionDataSource{}, err
	}
	return ds, nil
}

func buildMigrationEntries(kbID, feedURL, dataDir string) ([]migrationEntry, error) {
	policies, err := loadPolicyIndex(dataDir)
	if err != nil {
		return nil, err
	}
	if len(policies) == 0 {
		return nil, fmt.Errorf("no policy packages found under %s", dataDir)
	}
	// The database is optionally inspected during dry-run (when DATABASE_URL is
	// supplied) and is mandatory for --apply. This keeps planning possible before
	// credentials are available while making a credentialed dry-run authoritative.
	entries := make([]migrationEntry, 0, len(policies))
	for id, p := range policies {
		entries = append(entries, migrationEntry{PolicyID: id, NewTitle: p.Title, NewFileName: p.ExportFilename(), ExternalID: strings.TrimRight(feedURL, "/") + ":baoan-policy:" + id, Tags: p.Tags, RejectedTags: p.RejectedTags})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].PolicyID < entries[j].PolicyID })
	_ = kbID // retained in the manifest and required by the public command contract
	return entries, nil
}

type indexedPolicy struct {
	ID, Title    string
	Tags         []string
	RejectedTags []string
	exportName   string
}

func (p indexedPolicy) ExportFilename() string { return p.exportName }

func loadPolicyIndex(root string) (map[string]indexedPolicy, error) {
	result := map[string]indexedPolicy{}
	canonicalRoot := filepath.Join(root, "policies")
	if entries, err := os.ReadDir(canonicalRoot); err == nil {
		dimensions, err := canonical.LoadLatestDictionary(root)
		if err != nil {
			return nil, fmt.Errorf("load official tag dictionary: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() {
				doc, err := canonical.LoadLatest(root, e.Name())
				if err != nil {
					return nil, fmt.Errorf("load %s: %w", e.Name(), err)
				}
				if strings.TrimSpace(doc.PackageID) == "" || strings.TrimSpace(doc.Structured.Title) == "" {
					return nil, fmt.Errorf("package %s has empty policy ID or title", e.Name())
				}
				if _, exists := result[doc.PackageID]; exists {
					return nil, fmt.Errorf("duplicate policy ID %s", doc.PackageID)
				}
				tags, rejected := filterMigrationOfficialTags(doc.OfficialTags(time.Now().UTC()), dimensions)
				result[doc.PackageID] = indexedPolicy{ID: doc.PackageID, Title: doc.Structured.Title, Tags: tags, RejectedTags: rejected, exportName: doc.ExportFilename()}
			}
		}
		return result, nil
	}
	// Fallback for an exported raw folder: title comes from the first Markdown
	// heading and the package ID is retained from the file name.
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		id := postIDPattern.FindString(filepath.Base(path))
		if id == "" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		title := id
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(strings.TrimLeft(line, "# "))
			if line != "" {
				title = line
				break
			}
		}
		if _, exists := result[id]; exists {
			return fmt.Errorf("duplicate policy ID %s", id)
		}
		result[id] = indexedPolicy{ID: id, Title: title, exportName: title + "（" + id + "）.md"}
		policy := result[id]
		policy.exportName = canonical.Document{PackageID: id, Structured: model.StructuredPolicy{Title: title}}.ExportFilename()
		result[id] = policy
		return nil
	})
	return result, err
}

func enrichEntriesFromDB(dbURL, kbID string, entries []migrationEntry) ([]migrationEntry, migrationMatchSummary, error) {
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return nil, migrationMatchSummary{}, err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, migrationMatchSummary{}, fmt.Errorf("database ping: %w", err)
	}
	rows, err := db.QueryContext(ctx, `SELECT id,title,file_name,COALESCE(metadata,'{}'::jsonb) FROM knowledges WHERE knowledge_base_id=$1 AND deleted_at IS NULL`, kbID)
	if err != nil {
		return nil, migrationMatchSummary{}, fmt.Errorf("list knowledge: %w", err)
	}
	defer rows.Close()
	records := make([]knowledgeMigrationRecord, 0)
	for rows.Next() {
		var id, title, fileName string
		var metadata []byte
		if err := rows.Scan(&id, &title, &fileName, &metadata); err != nil {
			return nil, migrationMatchSummary{}, err
		}
		postID := strictMigratablePolicyID(fileName)
		if postID == "" {
			continue
		}
		record := knowledgeMigrationRecord{PolicyID: postID, KnowledgeID: id, Title: title, FileName: fileName, Metadata: append([]byte(nil), metadata...)}
		tagRows, tagErr := db.QueryContext(ctx, `SELECT tag_id FROM knowledge_tag_relations WHERE knowledge_id=$1`, id)
		if tagErr != nil {
			return nil, migrationMatchSummary{}, fmt.Errorf("list old tags for %s: %w", id, tagErr)
		}
		for tagRows.Next() {
			var tagID string
			if err := tagRows.Scan(&tagID); err != nil {
				tagRows.Close()
				return nil, migrationMatchSummary{}, fmt.Errorf("scan old tag for %s: %w", id, err)
			}
			record.TagIDs = append(record.TagIDs, tagID)
		}
		if err := tagRows.Err(); err != nil {
			tagRows.Close()
			return nil, migrationMatchSummary{}, fmt.Errorf("read old tags for %s: %w", id, err)
		}
		if err := tagRows.Close(); err != nil {
			return nil, migrationMatchSummary{}, fmt.Errorf("close old tags for %s: %w", id, err)
		}
		wikiPages, err := loadWikiPageSnapshots(ctx, db, kbID, fileName)
		if err != nil {
			return nil, migrationMatchSummary{}, fmt.Errorf("list wiki pages for %s: %w", id, err)
		}
		record.WikiPages = wikiPages
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, migrationMatchSummary{}, err
	}
	return matchMigrationEntries(entries, records)
}

func loadWikiPageSnapshots(ctx context.Context, db *sql.DB, kbID, oldFileName string) ([]wikiPageRollback, error) {
	rows, err := db.QueryContext(ctx, `SELECT id,content FROM wiki_pages WHERE knowledge_base_id=$1 AND content LIKE '%' || $2 || '%'`, kbID, oldFileName)
	if err != nil {
		return nil, err
	}
	pages := make([]wikiPageRollback, 0)
	for rows.Next() {
		var page wikiPageRollback
		if err := rows.Scan(&page.ID, &page.Content); err != nil {
			rows.Close()
			return nil, err
		}
		pages = append(pages, page)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return pages, nil
}

// matchMigrationEntries permits archive policies that have not been imported
// yet, but refuses to touch an existing post_* knowledge item without an
// authoritative archive record.
func matchMigrationEntries(entries []migrationEntry, records []knowledgeMigrationRecord) ([]migrationEntry, migrationMatchSummary, error) {
	byPolicy := make(map[string]migrationEntry, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.PolicyID) == "" || strings.TrimSpace(entry.NewTitle) == "" {
			return nil, migrationMatchSummary{}, fmt.Errorf("archive entry is incomplete: %s", entry.PolicyID)
		}
		if _, exists := byPolicy[entry.PolicyID]; exists {
			return nil, migrationMatchSummary{}, fmt.Errorf("duplicate archive policy: %s", entry.PolicyID)
		}
		byPolicy[entry.PolicyID] = entry
	}
	matchedByPolicy := make(map[string]migrationEntry, len(records))
	for _, record := range records {
		entry, found := byPolicy[record.PolicyID]
		if !found {
			return nil, migrationMatchSummary{}, fmt.Errorf("knowledge without archive: %s (%s)", record.PolicyID, record.KnowledgeID)
		}
		if _, duplicate := matchedByPolicy[record.PolicyID]; duplicate {
			return nil, migrationMatchSummary{}, fmt.Errorf("multiple knowledge documents for policy: %s", record.PolicyID)
		}
		entry.KnowledgeID = record.KnowledgeID
		entry.OldTitle = record.Title
		entry.OldFileName = record.FileName
		entry.OldMetadata = append([]byte(nil), record.Metadata...)
		entry.OldTagIDs = append([]string(nil), record.TagIDs...)
		entry.OldWikiPages = append([]wikiPageRollback(nil), record.WikiPages...)
		matchedByPolicy[record.PolicyID] = entry
	}
	matched := make([]migrationEntry, 0, len(matchedByPolicy))
	for _, entry := range entries {
		if mapped, found := matchedByPolicy[entry.PolicyID]; found {
			matched = append(matched, mapped)
		}
	}
	return matched, migrationMatchSummary{Matched: len(matched), ArchiveMissingInKB: len(entries) - len(matched)}, nil
}

func migrationManagedOfficialTagNames(metadata []byte) map[string]bool {
	managed := map[string]bool{}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(metadata, &fields); err != nil {
		return managed
	}
	raw := fields["official_tag_names"]
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		var encoded string
		if err := json.Unmarshal(raw, &encoded); err != nil || json.Unmarshal([]byte(encoded), &names) != nil {
			return managed
		}
	}
	for _, name := range names {
		if isOfficialPolicyTagName(name) {
			managed[name] = true
		}
	}
	return managed
}

func managedOfficialTagIDs(ctx context.Context, tx *sql.Tx, knowledgeID string, metadata []byte) ([]string, error) {
	managedNames, err := managedOfficialTagNamesForKnowledge(ctx, tx, knowledgeID, metadata)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(managedNames))
	for _, id := range managedNames {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func managedOfficialTagNamesForKnowledge(ctx context.Context, tx *sql.Tx, knowledgeID string, metadata []byte) (map[string]string, error) {
	managedNames := migrationManagedOfficialTagNames(metadata)
	rows, err := tx.QueryContext(ctx, `SELECT r.tag_id,t.name FROM knowledge_tag_relations r JOIN knowledge_tags t ON t.id=r.tag_id WHERE r.knowledge_id=$1 AND t.deleted_at IS NULL`, knowledgeID)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, err
		}
		if managedNames[name] {
			result[name] = id
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return result, nil
}

func applyMigration(ctx context.Context, dbURL string, manifest migrationRollback, datasourceID, feedURL string) error {
	if err := ensureAdoptionEntries(manifest.Entries); err != nil {
		return err
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}
	datasourceState, err := validateAdoptionDataSourceInDB(ctx, db, manifest.KnowledgeBaseID, datasourceID, feedURL)
	if err != nil {
		return fmt.Errorf("validate RSS adoption data source: %w", err)
	}
	adoption, err := buildAdoptionState(ctx, feedURL, policyIDSet(manifest.Entries))
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	changed := false
	for i := range manifest.Entries {
		e := &manifest.Entries[i]
		var tenantID uint64
		var oldTitle, oldFile string
		var metadata []byte
		if e.KnowledgeID == "" {
			return fmt.Errorf("missing knowledge ID for %s", e.PolicyID)
		}
		if err := tx.QueryRowContext(ctx, `SELECT tenant_id,title,file_name,COALESCE(metadata,'{}'::jsonb) FROM knowledges WHERE id=$1 AND knowledge_base_id=$2 AND deleted_at IS NULL FOR UPDATE`, e.KnowledgeID, manifest.KnowledgeBaseID).Scan(&tenantID, &oldTitle, &oldFile, &metadata); err != nil {
			return fmt.Errorf("match %s: %w", e.PolicyID, err)
		}
		if err := ensureMigrationSnapshot(*e, oldTitle, oldFile, metadata); err != nil {
			return fmt.Errorf("validate %s: %w", e.PolicyID, err)
		}
		meta, err := migrationMetadata(metadata, e.ExternalID, datasourceID, feedURL, e.Tags, adoption.ContentSignals[e.PolicyID])
		if err != nil {
			return fmt.Errorf("metadata for %s: %w", e.PolicyID, err)
		}
		m, _ := json.Marshal(meta)
		currentManagedTags, err := managedOfficialTagNamesForKnowledge(ctx, tx, e.KnowledgeID, metadata)
		if err != nil {
			return fmt.Errorf("list managed tag names for %s: %w", e.PolicyID, err)
		}
		change := migrationChangeSet(*e, oldTitle, oldFile, metadata, m, currentManagedTags)
		if change.Knowledge {
			result, err := tx.ExecContext(ctx, `UPDATE knowledges SET title=$1,file_name=$2,metadata=$3 WHERE id=$4 AND knowledge_base_id=$5 AND deleted_at IS NULL`, e.NewTitle, e.NewFileName, m, e.KnowledgeID, manifest.KnowledgeBaseID)
			if err != nil {
				return err
			}
			if err := requireOneAffected(result, "update knowledge "+e.PolicyID); err != nil {
				return err
			}
		}
		// Wiki source_refs are UUID based, but cached display names may appear in
		// page content. Only update the exact page snapshots captured before the
		// migration so rollback can restore their original content byte-for-byte.
		if change.Wiki {
			for _, page := range e.OldWikiPages {
				result, err := tx.ExecContext(ctx, `UPDATE wiki_pages SET content=REPLACE(content,$1,$2) WHERE id=$3 AND knowledge_base_id=$4 AND content=$5`, e.OldFileName, e.NewFileName, page.ID, manifest.KnowledgeBaseID, page.Content)
				if err != nil {
					return fmt.Errorf("update wiki page %s for %s: %w", page.ID, e.PolicyID, err)
				}
				if err := requireOneAffected(result, fmt.Sprintf("update wiki page %s for %s", page.ID, e.PolicyID)); err != nil {
					return err
				}
			}
		}
		if change.Tags {
			managedTagIDs, err := managedOfficialTagIDs(ctx, tx, e.KnowledgeID, metadata)
			if err != nil {
				return fmt.Errorf("list managed tags for %s: %w", e.PolicyID, err)
			}
			for _, tagID := range managedTagIDs {
				if _, err := tx.ExecContext(ctx, `DELETE FROM knowledge_tag_relations WHERE knowledge_id=$1 AND tag_id=$2`, e.KnowledgeID, tagID); err != nil {
					return fmt.Errorf("remove old managed tag for %s: %w", e.PolicyID, err)
				}
			}
			for _, name := range e.Tags {
				if !isOfficialPolicyTagName(name) {
					return fmt.Errorf("refusing non-official migration tag %q for %s", name, e.PolicyID)
				}
				var tagID string
				err := tx.QueryRowContext(ctx, `SELECT id FROM knowledge_tags WHERE knowledge_base_id=$1 AND name=$2 AND deleted_at IS NULL LIMIT 1`, manifest.KnowledgeBaseID, name).Scan(&tagID)
				if errors.Is(err, sql.ErrNoRows) {
					tagID = uuid.NewString()
					if _, err = tx.ExecContext(ctx, `INSERT INTO knowledge_tags(id,tenant_id,knowledge_base_id,name,color,sort_order,created_at,updated_at) VALUES($1,$2,$3,$4,'',0,NOW(),NOW())`, tagID, tenantID, manifest.KnowledgeBaseID, name); err != nil {
						return err
					}
				} else if err != nil {
					return err
				}
				if _, err = tx.ExecContext(ctx, `INSERT INTO knowledge_tag_relations(knowledge_id,tag_id,created_at) VALUES($1,$2,NOW()) ON CONFLICT DO NOTHING`, e.KnowledgeID, tagID); err != nil {
					return err
				}
			}
		}
		changed = changed || change.Any()
	}
	if datasourceID != "" && (changed || !adoptionCursorSeedsPolicies(datasourceState.LastSyncCursor, feedURL, policyIDSet(manifest.Entries))) {
		result, err := tx.ExecContext(ctx, `UPDATE data_sources SET last_sync_cursor=$1 WHERE id=$2 AND knowledge_base_id=$3 AND type='rss' AND status='paused' AND deleted_at IS NULL`, adoption.Cursor, datasourceID, manifest.KnowledgeBaseID)
		if err != nil {
			return fmt.Errorf("seed RSS cursor: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("count seeded RSS cursor: %w", err)
		}
		if affected != 1 {
			return fmt.Errorf("seed RSS cursor affected %d data sources", affected)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func adoptionCursorSeedsPolicies(cursor []byte, feedURL string, policyIDs map[string]struct{}) bool {
	if len(policyIDs) == 0 || len(cursor) == 0 {
		return false
	}
	var stored struct {
		ConnectorCursor struct {
			FeedItems map[string]map[string]string `json:"feed_items"`
		} `json:"connector_cursor"`
	}
	if err := json.Unmarshal(cursor, &stored); err != nil {
		return false
	}
	items := stored.ConnectorCursor.FeedItems[feedURL]
	if len(items) == 0 {
		return false
	}
	for policyID := range policyIDs {
		if _, ok := items["baoan-policy:"+policyID]; !ok {
			return false
		}
	}
	return true
}

// migrationChangeSet computes whether this entry needs any database write.
// A repeated --apply with an identical adopted state must leave both the
// knowledge base and the RSS cursor untouched.
func migrationChangeSet(entry migrationEntry, title, fileName string, metadata, desiredMetadata []byte, currentManagedTags map[string]string) migrationChange {
	tagsChanged := !sameStringSet(entry.Tags, mapKeys(currentManagedTags))
	return migrationChange{
		Knowledge: title != entry.NewTitle || fileName != entry.NewFileName || !jsonEqual(metadata, desiredMetadata),
		Wiki:      entry.OldFileName != entry.NewFileName && len(entry.OldWikiPages) > 0,
		Tags:      tagsChanged,
	}
}

func mapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[string]struct{}, len(left))
	for _, value := range left {
		seen[value] = struct{}{}
	}
	if len(seen) != len(right) {
		return false
	}
	for _, value := range right {
		if _, ok := seen[value]; !ok {
			return false
		}
	}
	return true
}

func ensureAdoptionEntries(entries []migrationEntry) error {
	if len(entries) == 0 {
		return errors.New("refusing to seed RSS cursor for an empty migration")
	}
	return nil
}

// ensureMigrationSnapshot detects edits made after the dry-run/enrichment
// snapshot.  The caller holds SELECT ... FOR UPDATE, so once this succeeds no
// concurrent edit can be overwritten before the transaction commits.
func ensureMigrationSnapshot(entry migrationEntry, title, fileName string, metadata []byte) error {
	if entry.OldTitle != title || entry.OldFileName != fileName || !jsonEqual(entry.OldMetadata, metadata) {
		return errors.New("concurrent knowledge change detected")
	}
	return nil
}

func jsonEqual(left, right []byte) bool {
	var a, b interface{}
	if len(left) == 0 {
		left = []byte(`{}`)
	}
	if len(right) == 0 {
		right = []byte(`{}`)
	}
	return json.Unmarshal(left, &a) == nil && json.Unmarshal(right, &b) == nil && reflect.DeepEqual(a, b)
}

func requireOneAffected(result sql.Result, action string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count %s: %w", action, err)
	}
	if affected != 1 {
		return fmt.Errorf("%s affected %d rows", action, affected)
	}
	return nil
}

type adoptionState struct {
	Cursor         []byte
	ContentSignals map[string]string
}

func buildAdoptionCursor(ctx context.Context, feedURL string, adoptedPolicyIDs map[string]struct{}) ([]byte, error) {
	state, err := buildAdoptionState(ctx, feedURL, adoptedPolicyIDs)
	if err != nil {
		return nil, err
	}
	return state.Cursor, nil
}

func buildAdoptionState(ctx context.Context, feedURL string, adoptedPolicyIDs map[string]struct{}) (adoptionState, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return adoptionState{}, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return adoptionState{}, fmt.Errorf("fetch RSS for cursor: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return adoptionState{}, fmt.Errorf("RSS cursor seed returned HTTP %d", resp.StatusCode)
	}
	feed, err := gofeed.NewParser().Parse(resp.Body)
	if err != nil {
		return adoptionState{}, fmt.Errorf("parse RSS for cursor: %w", err)
	}
	items := map[string]string{}
	signals := map[string]string{}
	contentSignals := map[string]string{}
	seenAdopted := make(map[string]int, len(adoptedPolicyIDs))
	for _, item := range feed.Items {
		if item == nil {
			continue
		}
		matches := rssPolicyGUIDPattern.FindStringSubmatch(item.GUID)
		if len(matches) != 2 {
			continue
		}
		policyID := matches[1]
		if _, adopted := adoptedPolicyIDs[policyID]; !adopted {
			continue
		}
		seenAdopted[policyID]++
		if seenAdopted[policyID] > 1 {
			return adoptionState{}, fmt.Errorf("RSS contains duplicate adopted policy %s", policyID)
		}
		id := firstNonEmptyRSS(item.GUID, item.Link, item.Title)
		feedContent := firstNonEmptyRSS(item.Content, item.Description)
		items[id] = rssContentFingerprint(feedContent)
		signals[id] = rssFeedSignalFingerprint(item, feedContent)
		contentSignals[policyID] = rssFeedContentSignal(item, feedContent)
	}
	missing := make([]string, 0)
	for policyID := range adoptedPolicyIDs {
		if seenAdopted[policyID] != 1 {
			missing = append(missing, policyID)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return adoptionState{}, fmt.Errorf("RSS missing adopted policies: %s", strings.Join(missing, ","))
	}
	now := time.Now().UTC()
	cursor, err := json.Marshal(map[string]interface{}{"last_sync_time": now, "connector_cursor": map[string]interface{}{"last_sync_time": now, "feed_items": map[string]map[string]string{feedURL: items}, "feed_signals": map[string]map[string]string{feedURL: signals}}})
	if err != nil {
		return adoptionState{}, err
	}
	return adoptionState{Cursor: cursor, ContentSignals: contentSignals}, nil
}

func policyIDSet(entries []migrationEntry) map[string]struct{} {
	result := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		result[entry.PolicyID] = struct{}{}
	}
	return result
}

func rssContentFingerprint(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "h:" + hex.EncodeToString(sum[:])[:16]
}

// rssFeedSignalFingerprint intentionally mirrors FMind's RSS connector. The
// signal lets its first incremental run skip an adopted policy before it
// fetches the linked canonical page, so existing knowledge IDs stay intact.
func rssFeedSignalFingerprint(item *gofeed.Item, feedContent string) string {
	var b strings.Builder
	b.WriteString(item.GUID)
	b.WriteByte('\n')
	b.WriteString(item.Link)
	b.WriteByte('\n')
	b.WriteString(item.Title)
	b.WriteByte('\n')
	for _, category := range normalizeRSSOfficialTags(item.Categories) {
		b.WriteString(category)
		b.WriteByte('\n')
	}
	if item.UpdatedParsed != nil && !item.UpdatedParsed.IsZero() {
		b.WriteString(item.UpdatedParsed.UTC().Format(time.RFC3339))
	}
	b.WriteByte('\n')
	if item.PublishedParsed != nil && !item.PublishedParsed.IsZero() {
		b.WriteString(item.PublishedParsed.UTC().Format(time.RFC3339))
	}
	b.WriteByte('\n')
	b.WriteString(feedContent)
	sum := sha256.Sum256([]byte(b.String()))
	return "s:" + hex.EncodeToString(sum[:])[:16]
}

// rssFeedContentSignal mirrors FMind's RSS connector but deliberately omits
// categories, allowing a policy whose only feed change is tagging to retain
// its existing knowledge ID and derived artifacts.
func rssFeedContentSignal(item *gofeed.Item, feedContent string) string {
	var b strings.Builder
	b.WriteString(item.GUID)
	b.WriteByte('\n')
	b.WriteString(item.Link)
	b.WriteByte('\n')
	b.WriteString(item.Title)
	b.WriteByte('\n')
	if item.UpdatedParsed != nil && !item.UpdatedParsed.IsZero() {
		b.WriteString(item.UpdatedParsed.UTC().Format(time.RFC3339))
	}
	b.WriteByte('\n')
	if item.PublishedParsed != nil && !item.PublishedParsed.IsZero() {
		b.WriteString(item.PublishedParsed.UTC().Format(time.RFC3339))
	}
	b.WriteByte('\n')
	b.WriteString(feedContent)
	sum := sha256.Sum256([]byte(b.String()))
	return "c:" + hex.EncodeToString(sum[:])[:16]
}

func normalizeRSSOfficialTags(values []string) []string {
	allowed := []string{"\u670d\u52a1\u5bf9\u8c61/", "\u53d1\u6587\u673a\u6784/", "\u4e3b\u9898/", "\u6587\u4ef6\u8f7d\u4f53/", "\u6587\u4ef6\u7c7b\u578b/", "\u5173\u8054\u5185\u5bb9/", "\u7533\u62a5\u72b6\u6001/"}
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		for _, prefix := range allowed {
			if strings.HasPrefix(value, prefix) && strings.TrimSpace(strings.TrimPrefix(value, prefix)) != "" {
				seen[value] = struct{}{}
				break
			}
		}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func firstNonEmptyRSS(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func rollbackMigration(ctx context.Context, dbURL string, manifest migrationRollback) error {
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, e := range manifest.Entries {
		if e.KnowledgeID == "" || e.OldTitle == "" || e.OldFileName == "" {
			return fmt.Errorf("rollback entry %s is incomplete", e.PolicyID)
		}
		metadata := e.OldMetadata
		if len(metadata) == 0 {
			metadata = []byte(`{}`)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE knowledges SET title=$1,file_name=$2,metadata=$3 WHERE id=$4 AND knowledge_base_id=$5`, e.OldTitle, e.OldFileName, metadata, e.KnowledgeID, manifest.KnowledgeBaseID); err != nil {
			return err
		}
		for _, page := range e.OldWikiPages {
			result, err := tx.ExecContext(ctx, `UPDATE wiki_pages SET content=$1 WHERE id=$2 AND knowledge_base_id=$3`, page.Content, page.ID, manifest.KnowledgeBaseID)
			if err != nil {
				return fmt.Errorf("restore wiki page %s for %s: %w", page.ID, e.PolicyID, err)
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("count restored wiki page %s for %s: %w", page.ID, e.PolicyID, err)
			}
			if affected != 1 {
				return fmt.Errorf("restore wiki page %s for %s affected %d rows", page.ID, e.PolicyID, affected)
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM knowledge_tag_relations WHERE knowledge_id=$1`, e.KnowledgeID); err != nil {
			return err
		}
		for _, tagID := range e.OldTagIDs {
			if _, err := tx.ExecContext(ctx, `INSERT INTO knowledge_tag_relations(knowledge_id,tag_id,created_at) VALUES($1,$2,NOW()) ON CONFLICT DO NOTHING`, e.KnowledgeID, tagID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
