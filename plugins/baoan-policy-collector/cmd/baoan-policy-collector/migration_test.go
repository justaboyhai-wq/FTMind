package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildMigrationEntriesUsesStableExternalIDAndTitleFilename(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "policy(post_123).md")
	if err := os.WriteFile(file, []byte("# Shenzhen policy\n\nBody"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := buildMigrationEntries("kb", "http://127.0.0.1:18320/feed.xml", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries", len(entries))
	}
	if entries[0].PolicyID != "post_123" {
		t.Fatalf("unexpected id: %s", entries[0].PolicyID)
	}
	if entries[0].ExternalID != "http://127.0.0.1:18320/feed.xml:baoan-policy:post_123" {
		t.Fatalf("unexpected external id: %s", entries[0].ExternalID)
	}
	if !strings.Contains(entries[0].NewFileName, "post_123") {
		t.Fatalf("filename must retain policy ID: %s", entries[0].NewFileName)
	}
}

func TestBuildMigrationEntriesIsIdempotent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "policy(post_456).md"), []byte("# Policy"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := buildMigrationEntries("kb", "http://rss/feed.xml", root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildMigrationEntries("kb", "http://rss/feed.xml", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != len(second) || first[0].NewFileName != second[0].NewFileName || first[0].ExternalID != second[0].ExternalID {
		t.Fatalf("migration plan is not stable: %#v %#v", first, second)
	}
}

func TestMatchMigrationEntriesAllowsArchivePoliciesNotYetInKnowledgeBase(t *testing.T) {
	entries := []migrationEntry{
		{PolicyID: "post_1", NewTitle: "Policy one"},
		{PolicyID: "post_2", NewTitle: "Policy two"},
	}
	matched, summary, err := matchMigrationEntries(entries, []knowledgeMigrationRecord{{PolicyID: "post_1", KnowledgeID: "knowledge-1", Title: "post_1", FileName: "raw/post_1.md"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(matched) != 1 || matched[0].PolicyID != "post_1" || matched[0].KnowledgeID != "knowledge-1" {
		t.Fatalf("matched entries = %#v", matched)
	}
	if summary.Matched != 1 || summary.ArchiveMissingInKB != 1 {
		t.Fatalf("summary = %#v, want 1 matched and 1 archive-only policy", summary)
	}
}

func TestMatchMigrationEntriesRejectsKnowledgePolicyMissingFromArchive(t *testing.T) {
	_, _, err := matchMigrationEntries(
		[]migrationEntry{{PolicyID: "post_1", NewTitle: "Policy one"}},
		[]knowledgeMigrationRecord{{PolicyID: "post_9", KnowledgeID: "knowledge-9", Title: "post_9", FileName: "raw/post_9.md"}},
	)
	if err == nil || !strings.Contains(err.Error(), "knowledge without archive") {
		t.Fatalf("expected archive mismatch error, got %v", err)
	}
}

func TestBuildAdoptionCursorSeedsOnlyAdoptedItemsWithRSSCompatibleSignal(t *testing.T) {
	const topic = "\u4e3b\u9898/\u7efc\u5408\u670d\u52a1"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<rss version="2.0"><channel><item><guid>baoan-policy:post_1</guid><link>https://example.test/1</link><title>Policy one</title><category>` + topic + `</category><pubDate>Mon, 02 Jan 2006 15:04:05 GMT</pubDate><description>Body summary</description></item><item><guid>baoan-policy:post_2</guid><link>https://example.test/2</link><title>Policy two</title><description>Not adopted</description></item></channel></rss>`))
	}))
	defer server.Close()
	body, err := buildAdoptionCursor(context.Background(), server.URL, map[string]struct{}{"post_1": {}})
	if err != nil {
		t.Fatal(err)
	}
	var cursor map[string]interface{}
	if err := json.Unmarshal(body, &cursor); err != nil {
		t.Fatal(err)
	}
	connector := cursor["connector_cursor"].(map[string]interface{})
	items := connector["feed_items"].(map[string]interface{})[server.URL].(map[string]interface{})
	if len(items) != 1 {
		t.Fatalf("cursor must only contain adopted policies, got %#v", items)
	}
	if got, ok := items["baoan-policy:post_1"].(string); !ok || got != expectedRSSContentFingerprint("Body summary") {
		t.Fatalf("cursor content fingerprint = %#v", items)
	}
	signals := connector["feed_signals"].(map[string]interface{})[server.URL].(map[string]interface{})
	if got, ok := signals["baoan-policy:post_1"].(string); !ok || got != expectedRSSFeedSignal("baoan-policy:post_1", "https://example.test/1", "Policy one", []string{topic}, time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC), "Body summary") {
		t.Fatalf("RSS signal = %#v, want connector-compatible signal", signals)
	}
	state, err := buildAdoptionState(context.Background(), server.URL, map[string]struct{}{"post_1": {}})
	if err != nil || !strings.HasPrefix(state.ContentSignals["post_1"], "c:") {
		t.Fatalf("migration must retain a connector-compatible no-category content signal: %#v err=%v", state.ContentSignals, err)
	}
}

func expectedRSSContentFingerprint(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "h:" + fmt.Sprintf("%x", sum[:])[:16]
}

func expectedRSSFeedSignal(guid, link, title string, categories []string, published time.Time, content string) string {
	var b strings.Builder
	b.WriteString(guid)
	b.WriteByte('\n')
	b.WriteString(link)
	b.WriteByte('\n')
	b.WriteString(title)
	b.WriteByte('\n')
	for _, category := range categories {
		b.WriteString(category)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(published.UTC().Format(time.RFC3339))
	b.WriteByte('\n')
	b.WriteString(content)
	sum := sha256.Sum256([]byte(b.String()))
	return "s:" + fmt.Sprintf("%x", sum[:])[:16]
}

func TestStrictMigratablePolicyIDAcceptsOnlyRawOrBarePolicyFilename(t *testing.T) {
	cases := map[string]string{
		"raw/post_12.md":               "post_12",
		"post_34.md":                   "post_34",
		"raw/post_12.md.bak":           "",
		"prefix-post_12.md":            "",
		"nested/raw/post_12.md":        "",
		"raw/post_12.txt":              "",
		"Policy\uff08post_56\uff09.md": "post_56",
		"Policy-post_56.md":            "",
	}
	for fileName, want := range cases {
		if got := strictMigratablePolicyID(fileName); got != want {
			t.Errorf("strictMigratablePolicyID(%q) = %q, want %q", fileName, got, want)
		}
	}
}

func TestEnsureAdoptionEntriesRejectsEmptyMigration(t *testing.T) {
	if err := ensureAdoptionEntries(nil); err == nil {
		t.Fatal("empty migration must not seed an empty RSS cursor")
	}
}

func TestMigrationSnapshotMatchesRejectsConcurrentKnowledgeEdits(t *testing.T) {
	entry := migrationEntry{OldTitle: "old title", OldFileName: "raw/post_1.md", OldMetadata: json.RawMessage(`{"keep":"yes"}`)}
	if err := ensureMigrationSnapshot(entry, "old title", "raw/post_1.md", []byte(`{"keep":"yes"}`)); err != nil {
		t.Fatalf("equal snapshot must be accepted: %v", err)
	}
	if err := ensureMigrationSnapshot(entry, "user edited", "raw/post_1.md", []byte(`{"keep":"yes"}`)); err == nil || !strings.Contains(err.Error(), "concurrent knowledge change") {
		t.Fatalf("concurrent title change must abort migration, got %v", err)
	}
	if err := ensureMigrationSnapshot(entry, "old title", "raw/post_1.md", []byte(`{"keep":"user edit"}`)); err == nil || !strings.Contains(err.Error(), "concurrent knowledge change") {
		t.Fatalf("concurrent metadata change must abort migration, got %v", err)
	}
}

func TestMigrationChangeSetSecondApplyIsNoOp(t *testing.T) {
	entry := migrationEntry{
		NewTitle:    "Policy",
		NewFileName: "Policy\uff08post_1\uff09.md",
		Tags:        []string{"\u4e3b\u9898/\u5b98\u7f51"},
	}
	metadata, err := migrationMetadata([]byte(`{}`), "feed:baoan-policy:post_1", "ds-1", "feed", entry.Tags, "c:content")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	changes := migrationChangeSet(entry, entry.NewTitle, entry.NewFileName, encoded, encoded, map[string]string{"\u4e3b\u9898/\u5b98\u7f51": "tag-1"})
	if changes.Any() {
		t.Fatalf("second apply of an already adopted policy must be a no-op: %#v", changes)
	}

	changes = migrationChangeSet(entry, entry.NewTitle, entry.NewFileName, encoded, encoded, map[string]string{})
	if !changes.Tags {
		t.Fatalf("missing managed relation must still be repaired: %#v", changes)
	}
}

func TestAdoptionCursorAlreadySeedsAllPolicies(t *testing.T) {
	seeded := []byte(`{"connector_cursor":{"feed_items":{"http://feed.test/feed.xml":{"baoan-policy:post_1":"h:1","baoan-policy:post_2":"h:2"}}}}`)
	if !adoptionCursorSeedsPolicies(seeded, "http://feed.test/feed.xml", map[string]struct{}{"post_1": {}, "post_2": {}}) {
		t.Fatal("existing adoption cursor must suppress a second cursor write")
	}
	if adoptionCursorSeedsPolicies(seeded, "http://feed.test/feed.xml", map[string]struct{}{"post_1": {}, "post_3": {}}) {
		t.Fatal("cursor missing an adopted policy must be reseeded")
	}
}

func TestAdoptionDataSourceQueryCastsJSONCursorBeforeEmptyFallback(t *testing.T) {
	if !strings.Contains(adoptionDataSourceQuery, "last_sync_cursor::text") {
		t.Fatalf("JSON cursor must be cast to text before COALESCE: %s", adoptionDataSourceQuery)
	}
	if strings.Contains(adoptionDataSourceQuery, "COALESCE(last_sync_cursor,'')") {
		t.Fatalf("raw JSON COALESCE fallback would raise SQLSTATE 22P02: %s", adoptionDataSourceQuery)
	}
}

func TestBuildAdoptionCursorFailsWhenAnAdoptedPolicyIsMissingOrDuplicated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<rss version="2.0"><channel><item><guid>baoan-policy:post_1</guid><title>One</title></item></channel></rss>`))
	}))
	defer server.Close()
	if _, err := buildAdoptionCursor(context.Background(), server.URL, map[string]struct{}{"post_1": {}, "post_2": {}}); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("expected missing adopted policy error, got %v", err)
	}

	duplicate := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<rss version="2.0"><channel><item><guid>baoan-policy:post_1</guid><title>One</title></item><item><guid>baoan-policy:post_1</guid><title>One again</title></item></channel></rss>`))
	}))
	defer duplicate.Close()
	if _, err := buildAdoptionCursor(context.Background(), duplicate.URL, map[string]struct{}{"post_1": {}}); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate adopted policy error, got %v", err)
	}
}

func TestFirstNonEmptyRSSPreservesRSSFieldWhitespace(t *testing.T) {
	if got, want := firstNonEmptyRSS("  guid  ", "fallback"), "  guid  "; got != want {
		t.Fatalf("firstNonEmptyRSS()=%q, want %q; RSS fields must not be trimmed", got, want)
	}
}

func TestMigrationMetadataMatchesRuntimeRSSFields(t *testing.T) {
	metadata, err := migrationMetadata([]byte(`{"preserve":"yes"}`), "https://feed.xml:baoan-policy:post_1", "ds-1", "https://feed.xml", []string{"\u4e3b\u9898/\u7efc\u5408"})
	if err != nil {
		t.Fatal(err)
	}
	if metadata["datasource_id"] != "ds-1" || metadata["source_resource_id"] != "https://feed.xml" || metadata["external_id"] != "https://feed.xml:baoan-policy:post_1" || metadata["preserve"] != "yes" {
		t.Fatalf("metadata=%#v", metadata)
	}
}

func TestOfficialPolicyTagNameOnlyRecognizesManagedPrefixes(t *testing.T) {
	if !isOfficialPolicyTagName("\u4e3b\u9898/\u7efc\u5408") {
		t.Fatal("expected official tag")
	}
	if isOfficialPolicyTagName("AI\u5206\u6790/\u7efc\u5408") || isOfficialPolicyTagName("\u4e3b\u9898/") {
		t.Fatal("unexpected managed tag")
	}
}

func TestFilterMigrationOfficialTagsRecordsRejectedDictionaryValues(t *testing.T) {
	accepted, rejected := filterMigrationOfficialTags([]string{"\u4e3b\u9898/\u5b98\u7f51", "\u4e3b\u9898/\u672a\u77e5"}, map[string][]string{"themes": {"\u5b98\u7f51"}})
	if strings.Join(accepted, ",") != "\u4e3b\u9898/\u5b98\u7f51" || strings.Join(rejected, ",") != "\u4e3b\u9898/\u672a\u77e5" {
		t.Fatalf("accepted=%v rejected=%v", accepted, rejected)
	}
}

func TestValidateAdoptionDataSourceRequiresPausedMatchingRSS(t *testing.T) {
	config := []byte(`{"settings":{"feed_urls":"http://collector/feed.xml"}}`)
	if err := validateAdoptionDataSource(adoptionDataSource{ID: "ds", Type: "rss", Status: "paused", Config: config}, "ds", "http://collector/feed.xml"); err != nil {
		t.Fatal(err)
	}
	if err := validateAdoptionDataSource(adoptionDataSource{ID: "ds", Type: "rss", Status: "active", Config: config}, "ds", "http://collector/feed.xml"); err == nil {
		t.Fatal("active data source must be rejected")
	}
}

func TestMigrationManagedOfficialTagNamesUseMetadataExactSet(t *testing.T) {
	managed := migrationManagedOfficialTagNames([]byte(`{"official_tag_names":"[\"\u4e3b\u9898/\u5b98\u7f51\"]"}`))
	if !managed["\u4e3b\u9898/\u5b98\u7f51"] || managed["\u4e3b\u9898/\u4eba\u5de5\u4fdd\u7559"] {
		t.Fatalf("managed=%#v", managed)
	}
}
