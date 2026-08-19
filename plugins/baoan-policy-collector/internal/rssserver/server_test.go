package rssserver

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackagePageAndFeedUseCanonicalSnapshot(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "policies", "post_7")
	snapshot := filepath.Join(base, "snapshots", "snap-7")
	if err := os.MkdirAll(snapshot, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(base, "latest.json"), `{"snapshot_id":"snap-7","snapshot_sha256":"hash-7"}`)
	write(filepath.Join(snapshot, "normalized.md"), "政策正文")
	write(filepath.Join(snapshot, "structured.json"), `{"id":7,"title":"政策七","source_url":"https://www.baoan.gov.cn/post_7.html","final_url":"https://www.baoan.gov.cn/post_7.html","abstract":"摘要七","markdown":"政策正文","official":{"service_objects":["企业政策"],"issuing_authority":"宝安区政府","theme":"综合政务","carrier_type":"其他文件"}}`)
	write(filepath.Join(snapshot, "relations.json"), `[{"source_id":7,"relation_type":"graphic_interpretation","target_url":"https://www.baoan.gov.cn/post_8.html","target_title":"图文解读"}]`)
	write(filepath.Join(snapshot, "source-detail.json"), `{"attachment":[]}`)
	write(filepath.Join(snapshot, "manifest.json"), `{"schema_version":"baoan.raw/v1","package_id":"post_7","external_id":"post_7","snapshot_id":"snap-7","fetched_at":"2026-08-19T00:00:00Z","snapshot_sha256":"hash-7","files":[]}`)

	server := New(Config{DataDir: root, BaseURL: "https://collector.example"})
	page := httptest.NewRecorder()
	server.Handler().ServeHTTP(page, httptest.NewRequest("GET", "/packages/post_7", nil))
	if page.Code != 200 {
		t.Fatalf("page status=%d body=%s", page.Code, page.Body.String())
	}
	for _, want := range []string{"baoan.canonical-md/v1", "官网关系", "graphic_interpretation", "图文解读", "政策原文", "政策正文"} {
		if !strings.Contains(page.Body.String(), want) {
			t.Errorf("page missing %q", want)
		}
	}
	if strings.Contains(page.Body.String(), "<pre>") || !strings.Contains(page.Body.String(), "<h2>官网关系</h2>") {
		t.Fatalf("RSS article must be semantic HTML, not one preformatted code block: %s", page.Body.String())
	}

	feed := httptest.NewRecorder()
	server.Handler().ServeHTTP(feed, httptest.NewRequest("GET", "/feed.xml", nil))
	if feed.Code != 200 {
		t.Fatalf("feed status=%d body=%s", feed.Code, feed.Body.String())
	}
	for _, want := range []string{"baoan-policy:post_7:snap-7", "https://collector.example/packages/post_7", "政策七"} {
		if !strings.Contains(feed.Body.String(), want) {
			t.Errorf("feed missing %q", want)
		}
	}
}

func TestFeedFailsClosedWhenAnyPackageCannotBeAssembled(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "policies", "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	server := New(Config{DataDir: root})
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/feed.xml", nil))
	if response.Code != 500 || !strings.Contains(response.Body.String(), "broken") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestHealthReportsCanonicalAssemblyFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "policies", "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	server := New(Config{DataDir: root})
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/healthz", nil))
	if response.Code != 503 || !strings.Contains(response.Body.String(), `"status":"degraded"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
