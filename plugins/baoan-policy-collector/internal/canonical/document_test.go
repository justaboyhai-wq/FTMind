package canonical

import (
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/model"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestLoadLatestRendersCompletePolicyDocument(t *testing.T) {
	root := t.TempDir()
	snapshot := filepath.Join(root, "policies", "post_42", "snapshots", "snap-1")
	if err := os.MkdirAll(filepath.Join(snapshot, "attachments"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(snapshot, path), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "policies", "post_42", "latest.json"), []byte(`{"snapshot_id":"snap-1","snapshot_sha256":"abc"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	write("normalized.md", "第一条 政策正文")
	write("attachments/a.pdf", "pdf")
	write("structured.json", `{"id":42,"title":"测试政策","document_number":"宝规〔2026〕1号","source_url":"https://www.baoan.gov.cn/post_42.html","final_url":"https://www.baoan.gov.cn/post_42.html","abstract":"摘要","markdown":"第一条 政策正文","official":{"service_objects":["企业政策"],"issuing_authority":"宝安区政府","theme":"综合政务","carrier_type":"区政府规范性文件","document_genre":"通知"},"published_at":"2026-08-01T00:00:00Z","effective_at":"2026-09-01T00:00:00Z","expires_at":"2027-09-01T00:00:00Z","application_start":"2026-08-01T00:00:00Z","application_end":"2026-12-01T00:00:00Z","official_listed":true,"local_application_status":"open"}`)
	write("relations.json", `[{"source_id":42,"relation_type":"text_interpretation","target_id":43,"target_url":"https://www.baoan.gov.cn/post_43.html","target_title":"文字解读","assignment_source":"website","rule_version":"baoan-related-v1"}]`)
	write("source-detail.json", `{"attachment":[{"name":"申报表.pdf","url":"https://www.baoan.gov.cn/attachment/form.pdf","size":"3"},{"name":"未下载.docx","url":"https://www.baoan.gov.cn/attachment/missing.docx","size":"5"},{"name":"丢失.pdf","url":"https://www.baoan.gov.cn/attachment/lost.pdf","size":"4"}]}`)
	write("manifest.json", `{"schema_version":"baoan.raw/v1","package_id":"post_42","external_id":"post_42","snapshot_id":"snap-1","fetched_at":"2026-08-19T00:00:00Z","snapshot_sha256":"abcdef","files":["normalized.md","structured.json","relations.json","source-detail.json","attachments/a.pdf","attachments/lost.pdf"],"attachments":[{"name":"申报表.pdf","url":"https://www.baoan.gov.cn/attachment/form.pdf","actual_size":3,"sha256":"1234","storage_path":"attachments/a.pdf"},{"name":"丢失.pdf","url":"https://www.baoan.gov.cn/attachment/lost.pdf","actual_size":4,"sha256":"5678","storage_path":"attachments/lost.pdf"}]}`)

	doc, err := LoadLatest(root, "post_42")
	if err != nil {
		t.Fatal(err)
	}
	md := doc.Markdown()
	for _, want := range []string{
		`schema_version: "baoan.canonical-md/v1"`, `raw_schema_version: "baoan.raw/v1"`,
		`snapshot_id: "snap-1"`, "测试政策", "宝规〔2026〕1号",
		"企业政策", "宝安区政府", "综合政务", "区政府规范性文件",
		"text_interpretation", "文字解读", "https://www.baoan.gov.cn/post_43.html",
		"申报表.pdf", "archived", "未下载.docx", "missing",
		"第一条 政策正文", "https://www.baoan.gov.cn/post_42.html",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("canonical markdown missing %q\n%s", want, md)
		}
	}
	if !strings.Contains(md, "[丢失.pdf](https://www.baoan.gov.cn/attachment/lost.pdf) — status: `missing`") {
		t.Fatalf("missing on-disk attachment was not marked missing:\n%s", md)
	}
}

func TestMarkdownFrontmatterQuotesScalarValues(t *testing.T) {
	doc := Document{PackageID: "post_1", SnapshotID: "snap", Structured: model.StructuredPolicy{Title: "标题: #一"}, Body: "正文"}
	if got := doc.Markdown(); !strings.Contains(got, `title: "标题: #一"`) {
		t.Fatalf("frontmatter scalar is not YAML safe:\n%s", got)
	}
}

func TestLoadLatestRejectsBodyDrift(t *testing.T) {
	root := t.TempDir()
	snapshot := filepath.Join(root, "policies", "post_1", "snapshots", "snap")
	if err := os.MkdirAll(snapshot, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(root, "policies", "post_1", "latest.json"), []byte(`{"snapshot_id":"snap"}`), 0o644)
	_ = os.WriteFile(filepath.Join(snapshot, "normalized.md"), []byte("正文 A"), 0o644)
	_ = os.WriteFile(filepath.Join(snapshot, "structured.json"), []byte(`{"id":1,"title":"政策","markdown":"正文 B","official":{}}`), 0o644)
	_ = os.WriteFile(filepath.Join(snapshot, "relations.json"), []byte(`[]`), 0o644)
	_ = os.WriteFile(filepath.Join(snapshot, "source-detail.json"), []byte(`{}`), 0o644)
	_ = os.WriteFile(filepath.Join(snapshot, "manifest.json"), []byte(`{"schema_version":"baoan.raw/v1","package_id":"post_1","external_id":"post_1","snapshot_id":"snap","fetched_at":"2026-08-19T00:00:00Z","snapshot_sha256":"abc","files":[]}`), 0o644)

	if _, err := LoadLatest(root, "post_1"); err == nil || !strings.Contains(err.Error(), "markdown mismatch") {
		t.Fatalf("expected markdown mismatch, got %v", err)
	}
}

func TestExportAllWritesCanonicalMarkdownOnly(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "policies", "post_9")
	snapshot := filepath.Join(base, "snapshots", "snap-9")
	if err := os.MkdirAll(snapshot, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(base, "latest.json"), `{"snapshot_id":"snap-9"}`)
	write(filepath.Join(snapshot, "normalized.md"), "正文九")
	write(filepath.Join(snapshot, "structured.json"), `{"id":9,"title":"政策九","markdown":"正文九","official":{}}`)
	write(filepath.Join(snapshot, "relations.json"), `[]`)
	write(filepath.Join(snapshot, "source-detail.json"), `{}`)
	write(filepath.Join(snapshot, "manifest.json"), `{"schema_version":"baoan.raw/v1","package_id":"post_9","external_id":"post_9","snapshot_id":"snap-9","fetched_at":"2026-08-19T00:00:00Z","snapshot_sha256":"hash-9","files":[]}`)
	out := filepath.Join(t.TempDir(), "raw")

	report, err := ExportAll(root, out)
	if err != nil {
		t.Fatal(err)
	}
	if report.Exported != 1 || len(report.Failures) != 0 {
		t.Fatalf("report=%+v", report)
	}
	doc, err := LoadLatest(root, "post_9")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(out, doc.ExportFilename()))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != doc.Markdown() {
		t.Fatalf("export differs from canonical document")
	}
	if _, err := os.Stat(filepath.Join(out, "post_9.structured.json")); !os.IsNotExist(err) {
		t.Fatalf("sidecar must not be exported as a second knowledge document")
	}
}

func TestExportFilenameUsesSanitizedTitleAndPackageID(t *testing.T) {
	doc := Document{
		PackageID:  "post_42",
		Structured: model.StructuredPolicy{Title: `  Education/Policy?*  `},
	}
	if got, want := doc.ExportFilename(), "EducationPolicy（post_42）.md"; got != want {
		t.Fatalf("ExportFilename() = %q, want %q", got, want)
	}
}

func TestOfficialTagsUsesWebsiteDimensionsAndDateWindow(t *testing.T) {
	doc := Document{
		PackageID: "post_42",
		Structured: model.StructuredPolicy{
			Official: model.OfficialFacts{
				ServiceObjects:   []string{"企业政策", "企业政策"},
				IssuingAuthority: "宝安区发展和改革局",
				Theme:            "国民经济管理",
				CarrierType:      "区政府规范性文件",
				DocumentGenre:    "通知",
			},
			ApplicationStart: "2026-08-01T00:00:00Z",
			ApplicationEnd:   "2026-08-31T23:59:59Z",
		},
		Relations: []Relation{{SourceLabel: "文字解读"}},
	}
	got := doc.OfficialTags(time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC))
	want := []string{
		"服务对象/企业政策", "发文机构/宝安区发展和改革局", "主题/国民经济管理",
		"文件载体/区政府规范性文件", "文件类型/通知", "关联内容/文字解读", "申报状态/当前可申报",
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("OfficialTags() = %#v, want %#v", got, want)
	}
}

func TestOfficialTagsDoesNotInferApplicationStatusWithoutBothDates(t *testing.T) {
	doc := Document{Structured: model.StructuredPolicy{ApplicationStart: "2026-08-01T00:00:00Z"}}
	for _, tag := range doc.OfficialTags(time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)) {
		if strings.HasPrefix(tag, "申报状态/") {
			t.Fatalf("unexpected inferred application tag %q", tag)
		}
	}
}

func TestFilterOfficialTagsUsesCapturedWebsiteDictionary(t *testing.T) {
	tags, rejected := FilterOfficialTags([]string{"服务对象/企业政策", "主题/未知", "文件类型/区规范性文件"}, map[string][]string{
		"service_objects": {"企业政策"},
		"themes":          {"综合服务"},
	})
	if len(rejected) != 1 || rejected[0] != "主题/未知" {
		t.Fatalf("rejected=%v", rejected)
	}
	if len(tags) != 2 {
		t.Fatalf("accepted=%v", tags)
	}
}

func TestExportAllRejectsUnmanagedNonEmptyDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "policies"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(out, "unrelated.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExportAll(root, out); err == nil || !strings.Contains(err.Error(), "not a managed canonical export") {
		t.Fatalf("expected unmanaged directory error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "unrelated.txt")); err != nil {
		t.Fatalf("unrelated file changed: %v", err)
	}
}
