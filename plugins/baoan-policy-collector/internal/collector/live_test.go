//go:build live

package collector

import (
	"context"
	"testing"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/config"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/discovery"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/httpclient"
)

// TestLiveBaoanIndex is opt-in because it accesses the public Baoan website.
func TestLiveBaoanIndex(t *testing.T) {
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
}
