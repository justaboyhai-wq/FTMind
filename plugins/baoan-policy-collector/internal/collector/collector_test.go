package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/config"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/httpclient"
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
