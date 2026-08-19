package canonical

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/model"
	"github.com/yuin/goldmark"
)

const SchemaVersion = "baoan.canonical-md/v1"

type Relation struct {
	RelationType     string `json:"relation_type"`
	TargetURL        string `json:"target_url"`
	TargetTitle      string `json:"target_title"`
	SourceLabel      string `json:"source_label"`
	AssignmentSource string `json:"assignment_source"`
	RuleVersion      string `json:"rule_version"`
}

type Attachment struct {
	Name, URL, MIME, SHA256, StoragePath, Status string
	DeclaredSize, ActualSize                     int64
}

type Document struct {
	PackageID, SnapshotID, SnapshotSHA256 string
	FetchedAt                             time.Time
	Structured                            model.StructuredPolicy
	Relations                             []Relation
	Attachments                           []Attachment
	Body                                  string
}

type ExportReport struct {
	Exported int               `json:"exported"`
	Failures map[string]string `json:"failures,omitempty"`
}

// FilterOfficialTags applies the collector's captured official dictionary to
// dimension tags. Empty dictionaries are treated as unavailable (so a feed
// remains usable during the first discovery run); unknown values are returned
// separately for audit logging and are never emitted as tags.
func FilterOfficialTags(tags []string, dimensions map[string][]string) (accepted, rejected []string) {
	allowedKey := map[string]string{"服务对象/": "service_objects", "发文机构/": "authorities", "主题/": "themes", "文件载体/": "carriers", "文件类型/": "document_genres"}
	sets := make(map[string]map[string]struct{}, len(dimensions))
	for key, values := range dimensions {
		if len(values) == 0 {
			continue
		}
		set := map[string]struct{}{}
		for _, value := range values {
			set[strings.TrimSpace(value)] = struct{}{}
		}
		sets[key] = set
	}
	for _, tag := range tags {
		valid := true
		for prefix, key := range allowedKey {
			if strings.HasPrefix(tag, prefix) {
				if set := sets[key]; len(set) > 0 {
					_, valid = set[strings.TrimSpace(strings.TrimPrefix(tag, prefix))]
				}
				break
			}
		}
		if valid {
			accepted = append(accepted, tag)
		} else {
			rejected = append(rejected, tag)
		}
	}
	return accepted, rejected
}

// LoadLatestDictionary reads the latest captured website dimension snapshot.
func LoadLatestDictionary(root string) (map[string][]string, error) {
	pointer, err := os.ReadFile(filepath.Join(root, "dictionaries", "latest.json"))
	if err != nil {
		return nil, err
	}
	var meta struct {
		SnapshotID string `json:"snapshot_id"`
	}
	if err := json.Unmarshal(pointer, &meta); err != nil {
		return nil, err
	}
	if meta.SnapshotID == "" {
		return nil, fmt.Errorf("dictionary latest pointer has no snapshot_id")
	}
	body, err := os.ReadFile(filepath.Join(root, "dictionaries", "snapshots", meta.SnapshotID, "official-dimensions.json"))
	if err != nil {
		return nil, err
	}
	var snapshot struct {
		Dimensions map[string][]string `json:"dimensions"`
	}
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return nil, err
	}
	return snapshot.Dimensions, nil
}

func VerifyAll(root string) error {
	entries, err := os.ReadDir(filepath.Join(root, "policies"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var failures []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := LoadLatest(root, entry.Name()); err != nil {
			failures = append(failures, entry.Name()+": "+err.Error())
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("canonical verification failed: %s", strings.Join(failures, "; "))
	}
	return nil
}

// ExportAll materializes one and only one knowledge document per latest raw
// package. Machine sidecars stay in the immutable raw store and are represented
// inside the canonical Markdown instead of becoming duplicate KB documents.
func ExportAll(root, outputDir string) (ExportReport, error) {
	entries, err := os.ReadDir(filepath.Join(root, "policies"))
	if err != nil {
		return ExportReport{}, err
	}
	if err := prepareOutputDir(outputDir); err != nil {
		return ExportReport{}, err
	}
	report := ExportReport{Failures: map[string]string{}}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		doc, err := LoadLatest(root, entry.Name())
		if err != nil {
			report.Failures[entry.Name()] = err.Error()
			continue
		}
		target := filepath.Join(outputDir, doc.ExportFilename())
		temporary := target + ".tmp"
		if err := os.WriteFile(temporary, []byte(doc.Markdown()), 0o644); err != nil {
			return report, err
		}
		if err := os.Rename(temporary, target); err != nil {
			return report, err
		}
		report.Exported++
	}
	if len(report.Failures) == 0 {
		report.Failures = nil
	}
	return report, nil
}

// ExportFilename keeps exported knowledge documents recognizable to people
// while retaining the immutable package ID needed to prevent collisions.
func (d Document) ExportFilename() string {
	const maxBytes = 180
	suffix := "（" + d.PackageID + "）.md"
	title := strings.TrimSpace(d.Structured.Title)
	if title == "" {
		title = "未命名政策"
	}
	var cleaned strings.Builder
	for _, r := range title {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			continue
		}
		cleaned.WriteRune(r)
	}
	title = strings.TrimRight(strings.TrimSpace(cleaned.String()), ". ")
	if title == "" {
		title = "未命名政策"
	}
	limit := maxBytes - len(suffix)
	var truncated strings.Builder
	for _, r := range title {
		if truncated.Len()+len(string(r)) > limit {
			break
		}
		truncated.WriteRune(r)
	}
	return truncated.String() + suffix
}

// OfficialTags returns only website-backed, dimension-prefixed tags. It never
// derives an application status when either boundary is missing or malformed.
func (d Document) OfficialTags(now time.Time) []string {
	seen := make(map[string]struct{})
	add := func(prefix, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		tag := prefix + value
		if _, ok := seen[tag]; ok {
			return
		}
		seen[tag] = struct{}{}
	}
	for _, value := range d.Structured.Official.ServiceObjects {
		add("服务对象/", value)
	}
	add("发文机构/", d.Structured.Official.IssuingAuthority)
	add("主题/", d.Structured.Official.Theme)
	add("文件载体/", d.Structured.Official.CarrierType)
	add("文件类型/", d.Structured.Official.DocumentGenre)
	for _, relation := range d.Relations {
		add("关联内容/", relation.SourceLabel)
	}
	start, startOK := parseOfficialTime(d.Structured.ApplicationStart)
	end, endOK := parseOfficialTime(d.Structured.ApplicationEnd)
	if startOK && endOK && !now.Before(start) && !now.After(end) {
		add("申报状态/", "当前可申报")
	}
	result := make([]string, 0, len(seen))
	for tag := range seen {
		result = append(result, tag)
	}
	sort.Strings(result)
	return result
}

func parseOfficialTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func prepareOutputDir(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return err
	}
	marker := filepath.Join(outputDir, ".baoan-canonical-export")
	if len(entries) > 0 {
		if _, err := os.Stat(marker); err != nil {
			return fmt.Errorf("output directory is not a managed canonical export: %s", outputDir)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if strings.HasSuffix(entry.Name(), ".md") || strings.HasSuffix(entry.Name(), ".tmp") {
				if err := os.Remove(filepath.Join(outputDir, entry.Name())); err != nil {
					return err
				}
			}
		}
	}
	return os.WriteFile(marker, []byte(SchemaVersion+"\n"), 0o644)
}

type latestPointer struct {
	SnapshotID     string `json:"snapshot_id"`
	SnapshotSHA256 string `json:"snapshot_sha256"`
}

func LoadLatest(root, packageID string) (Document, error) {
	if packageID == "" || strings.ContainsAny(packageID, `/\\`) || packageID == "." || packageID == ".." {
		return Document{}, fmt.Errorf("invalid package id")
	}
	base := filepath.Join(root, "policies", packageID)
	var latest latestPointer
	if err := readJSON(filepath.Join(base, "latest.json"), &latest); err != nil {
		return Document{}, err
	}
	if latest.SnapshotID == "" || strings.ContainsAny(latest.SnapshotID, `/\\`) {
		return Document{}, fmt.Errorf("invalid snapshot id")
	}
	snapshot := filepath.Join(base, "snapshots", latest.SnapshotID)
	var structured model.StructuredPolicy
	if err := readJSON(filepath.Join(snapshot, "structured.json"), &structured); err != nil {
		return Document{}, err
	}
	body, err := os.ReadFile(filepath.Join(snapshot, "normalized.md"))
	if err != nil {
		return Document{}, err
	}
	if strings.TrimSpace(string(body)) != strings.TrimSpace(structured.Markdown) {
		return Document{}, fmt.Errorf("markdown mismatch for %s/%s", packageID, latest.SnapshotID)
	}
	var relations []Relation
	if err := readJSON(filepath.Join(snapshot, "relations.json"), &relations); err != nil {
		return Document{}, err
	}
	var manifest model.Manifest
	if err := readJSON(filepath.Join(snapshot, "manifest.json"), &manifest); err != nil {
		return Document{}, err
	}
	var detail struct {
		Attachments []model.Attachment `json:"attachment"`
	}
	if err := readJSON(filepath.Join(snapshot, "source-detail.json"), &detail); err != nil {
		return Document{}, err
	}

	attachments := mergeAttachments(snapshot, detail.Attachments, manifest.Attachments)
	sort.Slice(relations, func(i, j int) bool {
		return relations[i].RelationType+relations[i].TargetURL < relations[j].RelationType+relations[j].TargetURL
	})
	return Document{
		PackageID: packageID, SnapshotID: latest.SnapshotID,
		SnapshotSHA256: firstNonEmpty(latest.SnapshotSHA256, manifest.SnapshotSHA256),
		FetchedAt:      manifest.FetchedAt,
		Structured:     structured, Relations: relations, Attachments: attachments,
		Body: string(body),
	}, nil
}

func (d Document) Markdown() string {
	var b strings.Builder
	line := func(key, value string) {
		encoded, _ := json.Marshal(scalar(value))
		fmt.Fprintf(&b, "%s: %s\n", key, encoded)
	}
	listLine := func(key string, values []string) {
		if values == nil {
			values = []string{}
		}
		encoded, _ := json.Marshal(values)
		fmt.Fprintf(&b, "%s: %s\n", key, encoded)
	}
	b.WriteString("---\n")
	line("schema_version", SchemaVersion)
	line("raw_schema_version", "baoan.raw/v1")
	line("external_id", d.PackageID)
	line("snapshot_id", d.SnapshotID)
	line("snapshot_sha256", d.SnapshotSHA256)
	line("title", d.Structured.Title)
	line("document_number", d.Structured.DocumentNumber)
	line("source_url", d.Structured.SourceURL)
	line("final_url", d.Structured.FinalURL)
	line("published_at", d.Structured.PublishedAt)
	line("effective_at", d.Structured.EffectiveAt)
	line("expires_at", d.Structured.ExpiresAt)
	line("application_start", d.Structured.ApplicationStart)
	line("application_end", d.Structured.ApplicationEnd)
	fmt.Fprintf(&b, "official_listed: %t\n", d.Structured.OfficialListed)
	line("local_application_status", d.Structured.LocalApplicationStatus)
	listLine("service_objects", d.Structured.Official.ServiceObjects)
	line("issuing_authority", d.Structured.Official.IssuingAuthority)
	line("theme", d.Structured.Official.Theme)
	line("carrier_type", d.Structured.Official.CarrierType)
	line("document_genre", d.Structured.Official.DocumentGenre)
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", d.Structured.Title)
	if d.Structured.Abstract != "" {
		fmt.Fprintf(&b, "## 摘要\n\n%s\n\n", d.Structured.Abstract)
	}
	b.WriteString("## 官网关系\n\n")
	if len(d.Relations) == 0 {
		b.WriteString("- 无官网关系记录\n")
	}
	for _, relation := range d.Relations {
		label := firstNonEmpty(relation.TargetTitle, relation.SourceLabel, relation.TargetURL)
		fmt.Fprintf(&b, "- [%s](%s) — relation_type: `%s`; assignment_source: `%s`; rule_version: `%s`\n",
			label, relation.TargetURL, relation.RelationType, relation.AssignmentSource, relation.RuleVersion)
	}
	b.WriteString("\n## 附件\n\n")
	if len(d.Attachments) == 0 {
		b.WriteString("- 无附件记录\n")
	}
	for _, attachment := range d.Attachments {
		fmt.Fprintf(&b, "- [%s](%s) — status: `%s`; declared_size: `%d`; actual_size: `%d`; sha256: `%s`; storage_path: `%s`\n",
			attachment.Name, attachment.URL, attachment.Status, attachment.DeclaredSize, attachment.ActualSize, attachment.SHA256, attachment.StoragePath)
	}
	b.WriteString("\n## 政策原文\n\n")
	b.WriteString(strings.TrimSpace(d.Body))
	b.WriteString("\n\n## 审计信息\n\n")
	fmt.Fprintf(&b, "- 原始包：`policies/%s/snapshots/%s`\n", d.PackageID, d.SnapshotID)
	fmt.Fprintf(&b, "- 来源 URL：%s\n", firstNonEmpty(d.Structured.FinalURL, d.Structured.SourceURL))
	return b.String()
}

func (d Document) HTML() string {
	var rendered bytes.Buffer
	if err := goldmark.Convert([]byte(d.Markdown()), &rendered); err != nil {
		return "<!doctype html><html><head><meta charset=\"utf-8\"><title>" + html.EscapeString(d.Structured.Title) + "</title></head><body><article><p>canonical rendering failed</p></article></body></html>"
	}
	return "<!doctype html><html><head><meta charset=\"utf-8\"><title>" + html.EscapeString(d.Structured.Title) +
		"</title></head><body><article>" + rendered.String() + "</article></body></html>"
}

func mergeAttachments(snapshotDir string, declared []model.Attachment, archived []model.AttachmentManifest) []Attachment {
	byURL := make(map[string]model.AttachmentManifest, len(archived))
	for _, item := range archived {
		byURL[item.URL] = item
	}
	seen := make(map[string]bool)
	out := make([]Attachment, 0, len(declared)+len(archived))
	for _, item := range declared {
		stored, ok := byURL[item.URL]
		status := "missing"
		if ok && attachmentExists(snapshotDir, stored.StoragePath, stored.ActualSize) {
			status = "archived"
		}
		out = append(out, Attachment{Name: item.Name, URL: item.URL, MIME: firstNonEmpty(stored.MIME, item.MIME), DeclaredSize: item.Size, ActualSize: stored.ActualSize, SHA256: stored.SHA256, StoragePath: stored.StoragePath, Status: status})
		seen[item.URL] = true
	}
	for _, item := range archived {
		if seen[item.URL] {
			continue
		}
		status := "missing"
		if attachmentExists(snapshotDir, item.StoragePath, item.ActualSize) {
			status = "archived"
		}
		out = append(out, Attachment{Name: item.Name, URL: item.URL, MIME: item.MIME, DeclaredSize: item.DeclaredSize, ActualSize: item.ActualSize, SHA256: item.SHA256, StoragePath: item.StoragePath, Status: status})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].URL < out[j].URL })
	return out
}

func attachmentExists(snapshotDir, storagePath string, actualSize int64) bool {
	if strings.TrimSpace(storagePath) == "" {
		return false
	}
	normalized := strings.ReplaceAll(storagePath, "\\", string(filepath.Separator))
	cleaned := filepath.Clean(normalized)
	if filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return false
	}
	info, err := os.Stat(filepath.Join(snapshotDir, cleaned))
	if err != nil || info.IsDir() {
		return false
	}
	return actualSize <= 0 || info.Size() == actualSize
}

func readJSON(path string, target any) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func scalar(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(value), "\r", " "), "\n", " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
