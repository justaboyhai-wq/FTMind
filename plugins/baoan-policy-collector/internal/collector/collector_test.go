package collector

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/archive"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/config"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/httpclient"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/state"
)

func testHost(raw string) string { u, _ := url.Parse(raw); return u.Hostname() }
func TestCollectDiscoversIndex(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/xxk/", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "../../testdata/seed.html") })
	mux.HandleFunc("/zcfg.js", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "../../testdata/zcfg.js") })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cfg := config.Default()
	cfg.SeedURL = srv.URL + "/xxk/"
	cfg.DataDir = t.TempDir()
	cfg.AllowedHosts = []string{testHost(srv.URL)}
	client := httpclient.New(httpclient.Options{AllowedHosts: cfg.AllowedHosts, AllowPrivateNetworks: true, MaxBytes: 10 << 20, Interval: time.Nanosecond})
	c := New(cfg, client, nil)
	s, err := c.Collect(context.Background(), true, 0)
	if err == nil && s.IndexCount != 3 {
		t.Fatalf("summary=%+v", s)
	}
	_ = filepath.Separator
}

func TestCollectEndToEndPublishesPackage(t *testing.T) {
	detailJSON, err := os.ReadFile("../../testdata/detail-12846556.json")
	if err != nil {
		t.Fatal(err)
	}
	policyHTML, err := os.ReadFile("../../testdata/policy-12846556.html")
	if err != nil {
		t.Fatal(err)
	}
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/xxk/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><script src=\"/zcfg.js\"></script></html>"))
	})
	mux.HandleFunc("/zcfg.js", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `var allData = [{"id":"12846556","title":"policy","url":%q}];`, srv.URL+"/policy.html")
	})
	mux.HandleFunc("/postmeta/p/12/12846/12846556.json", func(w http.ResponseWriter, r *http.Request) {
		body := strings.ReplaceAll(string(detailJSON), "https://www.baoan.gov.cn", srv.URL)
		body = strings.ReplaceAll(body, "/xxgk/fgk/qbmwj/content/post_12846556.html", "/policy.html")
		body = strings.ReplaceAll(body, `"size":"205216"`, `"size":"3"`)
		body = strings.ReplaceAll(body, `"size":"65536"`, `"size":"3"`)
		body = strings.ReplaceAll(body, `"size":"49152"`, `"size":"3"`)
		body = strings.ReplaceAll(body, `"size":"30208"`, `"size":"3"`)
		_, _ = w.Write([]byte(body))
	})
	mux.HandleFunc("/policy.html", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(policyHTML) })
	mux.HandleFunc("/attachment/", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("pdf")) })
	srv = httptest.NewServer(mux)
	defer srv.Close()

	cfg := config.Default()
	cfg.SeedURL = srv.URL + "/xxk/"
	cfg.SourceBaseURL = srv.URL
	cfg.DataDir = t.TempDir()
	cfg.AllowedHosts = []string{testHost(srv.URL)}
	client := httpclient.New(httpclient.Options{AllowedHosts: cfg.AllowedHosts, AllowPrivateNetworks: true, MaxBytes: cfg.HTMLMaxBytes, Interval: time.Nanosecond})
	store, err := state.Open(filepath.Join(cfg.DataDir, "state", "collector.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	summary, err := New(cfg, client, store).Collect(context.Background(), true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Status != "success" || summary.Created != 1 || summary.AttachmentsSaved != 4 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if err := archive.Verify(cfg.DataDir); err != nil {
		t.Fatal(err)
	}
}

func TestRetryProcessesOnlyFailedRecord(t *testing.T) {
	detailJSON, err := os.ReadFile("../../testdata/detail-12846556.json")
	if err != nil {
		t.Fatal(err)
	}
	policyHTML, err := os.ReadFile("../../testdata/policy-12846556.html")
	if err != nil {
		t.Fatal(err)
	}
	var srv *httptest.Server
	policyHits := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/xxk/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<script src=\"/zcfg.js\"></script>"))
	})
	mux.HandleFunc("/zcfg.js", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `var allData = [{"id":"12846556","title":"policy","url":%q}];`, srv.URL+"/policy.html")
	})
	mux.HandleFunc("/postmeta/p/12/12846/12846556.json", func(w http.ResponseWriter, r *http.Request) {
		body := strings.ReplaceAll(string(detailJSON), "https://www.baoan.gov.cn", srv.URL)
		body = strings.ReplaceAll(body, "/xxgk/fgk/qbmwj/content/post_12846556.html", "/policy.html")
		body = strings.ReplaceAll(body, `"size":"205216"`, `"size":"3"`)
		body = strings.ReplaceAll(body, `"size":"65536"`, `"size":"3"`)
		body = strings.ReplaceAll(body, `"size":"49152"`, `"size":"3"`)
		body = strings.ReplaceAll(body, `"size":"30208"`, `"size":"3"`)
		_, _ = w.Write([]byte(body))
	})
	mux.HandleFunc("/policy.html", func(w http.ResponseWriter, r *http.Request) {
		policyHits++
		if policyHits == 1 {
			http.Error(w, "temporary", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(policyHTML)
	})
	mux.HandleFunc("/attachment/", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("pdf")) })
	srv = httptest.NewServer(mux)
	defer srv.Close()
	cfg := config.Default()
	cfg.SeedURL, cfg.SourceBaseURL, cfg.DataDir = srv.URL+"/xxk/", srv.URL, t.TempDir()
	cfg.AllowedHosts, cfg.RetryCount = []string{testHost(srv.URL)}, 0
	client := httpclient.New(httpclient.Options{AllowedHosts: cfg.AllowedHosts, AllowPrivateNetworks: true, MaxBytes: cfg.HTMLMaxBytes, Interval: time.Nanosecond, Retries: 0})
	store, err := state.Open(filepath.Join(cfg.DataDir, "state", "collector.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	c := New(cfg, client, store)
	first, err := c.Collect(context.Background(), false, 0)
	if err != nil || first.Status != "partial" || first.Failed != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := c.Retry(context.Background())
	if err != nil || second.Failed != 0 || second.Created != 1 {
		t.Fatalf("retry=%+v err=%v", second, err)
	}
	items, err := store.ListRetryable(context.Background(), 10)
	if err != nil || len(items) != 0 {
		t.Fatalf("retry queue=%+v err=%v", items, err)
	}
}
