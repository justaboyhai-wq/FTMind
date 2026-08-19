//go:build live

package collector

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/config"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/detail"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/discovery"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/httpclient"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/normalize"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/schema"
)

// TestLiveProtocol is opt-in because it accesses the public Baoan website.
func TestLiveProtocol(t *testing.T) {
	cfg := config.Default()
	client := httpclient.New(httpclient.Options{
		AllowedHosts: []string{"www.baoan.gov.cn"},
		MaxBytes:     cfg.HTMLMaxBytes,
		Interval:     cfg.RequestInterval,
		Timeout:      cfg.RequestTimeout,
		Retries:      cfg.RetryCount,
	})
	ctx := context.Background()
	seed, err := client.Get(ctx, cfg.SeedURL)
	if err != nil {
		t.Fatal(err)
	}
	indexURL, err := discovery.DiscoverIndexScript(cfg.SeedURL, seed.Body)
	if err != nil {
		t.Fatal(err)
	}
	index, err := client.Get(ctx, indexURL)
	if err != nil {
		t.Fatal(err)
	}
	records, err := discovery.ParseAllData(index.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) == 0 {
		t.Fatal("live index returned no policy records")
	}
	t.Logf("live index: %d records, script=%s", len(records), indexURL)
	positions := []int{0, len(records) / 2, len(records) - 1}
	seen := map[int]bool{}
	for _, pos := range positions {
		if seen[pos] {
			continue
		}
		seen[pos] = true
		r := records[pos]
		detailURL, err := detail.URLForID(cfg.SourceBaseURL, r.ID)
		if err != nil {
			t.Fatalf("id %s: %v", r.ID, err)
		}
		resp, err := client.Get(ctx, detailURL)
		if err != nil {
			t.Fatalf("detail id=%s url=%s: %v", r.ID, detailURL, err)
		}
		d, err := detail.Decode(resp.Body)
		if err != nil {
			t.Fatalf("decode id=%s: %v", r.ID, err)
		}
		if d.ID == 0 || strconv.FormatInt(d.ID, 10) != r.ID {
			t.Fatalf("detail id mismatch: index=%s detail=%d", r.ID, d.ID)
		}
		if d.ContentHTML == "" || d.URL == "" {
			t.Fatalf("detail id=%s missing content/source URL", r.ID)
		}
		for _, a := range d.Attachments {
			u, err := url.Parse(a.URL)
			if err != nil || u.Hostname() != "www.baoan.gov.cn" {
				t.Fatalf("attachment host id=%s url=%q", r.ID, a.URL)
			}
		}
		p, err := normalize.Policy(d, time.Now())
		if err != nil {
			t.Fatalf("normalize id=%s: %v", r.ID, err)
		}
		structured, _ := json.Marshal(p.Structured)
		if err := schema.Validate("structured.schema.json", structured); err != nil {
			t.Fatalf("schema id=%s: %v", r.ID, err)
		}
	}
}
