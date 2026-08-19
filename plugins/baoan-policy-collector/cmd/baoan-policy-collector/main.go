package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/archive"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/canonical"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/collector"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/config"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/httpclient"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/rssserver"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/scheduler"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/state"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "usage: collect --full|--incremental | retry --failed | unlock | verify --all | export-raw | export-manifest --run ID | migrate | daemon | serve-rss")
		return 2
	}
	if args[0] == "migrate" {
		if err := runMigrate(args[1:]); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		return 0
	}
	cfg := config.Default()
	dataDir := cfg.DataDir
	maxItems := 0
	all := false
	runID := ""
	addr := ":18320"
	baseURL := ""
	outputDir := ""
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(errOut)
	full := args[0] == "collect"
	if args[0] == "collect" {
		fs.BoolVar(&full, "full", full, "full reconciliation")
		fs.BoolVar(&full, "incremental", !full, "incremental collection")
		fs.IntVar(&maxItems, "max-items", 0, "sample limit")
		fs.StringVar(&dataDir, "data-dir", dataDir, "data directory")
	}
	if args[0] == "verify" {
		fs.BoolVar(&all, "all", false, "verify all packages")
	}
	if args[0] == "retry" {
		fs.BoolVar(&all, "failed", false, "retry failed records")
	}
	if args[0] == "export-manifest" {
		fs.StringVar(&runID, "run", "", "run id")
	}
	if args[0] == "serve-rss" {
		fs.StringVar(&addr, "addr", addr, "HTTP listen address")
		fs.StringVar(&baseURL, "base-url", baseURL, "public base URL used in feed links")
	}
	if args[0] == "export-raw" {
		fs.StringVar(&outputDir, "output-dir", outputDir, "canonical Markdown output directory")
	}
	if args[0] == "daemon" {
		fs.StringVar(&cfg.IncrementalCron, "incremental-cron", cfg.IncrementalCron, "incremental collection cron (Asia/Shanghai)")
		fs.StringVar(&cfg.FullCron, "full-cron", cfg.FullCron, "full reconciliation cron (Asia/Shanghai)")
	}
	if args[0] != "collect" {
		fs.StringVar(&dataDir, "data-dir", dataDir, "data directory")
	}
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	cfg.DataDir = dataDir
	if args[0] == "daemon" {
		return daemon(cfg, errOut)
	}
	if args[0] == "serve-rss" {
		server := rssserver.New(rssserver.Config{DataDir: dataDir, BaseURL: baseURL})
		fmt.Fprintf(out, "RSS gateway listening on %s (feed: %s/feed.xml)\n", addr, strings.TrimRight(baseURL, "/"))
		if err := http.ListenAndServe(addr, server.Handler()); err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(errOut, err)
			return 1
		}
		return 0
	}
	if args[0] == "verify" {
		if err := archive.Verify(dataDir); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		if err := canonical.VerifyAll(dataDir); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		fmt.Fprintf(out, "verified %s\n", dataDir)
		return 0
	}
	if args[0] == "export-raw" {
		if outputDir == "" {
			fmt.Fprintln(errOut, "--output-dir is required")
			return 2
		}
		report, err := canonical.ExportAll(dataDir, outputDir)
		payload, _ := json.Marshal(report)
		fmt.Fprintln(out, string(payload))
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		if len(report.Failures) > 0 {
			return 3
		}
		return 0
	}
	if args[0] == "export-manifest" {
		if runID == "" {
			fmt.Fprintln(errOut, "--run is required")
			return 2
		}
		fmt.Fprintln(out, filepath.Join(dataDir, "runs", runID, "run-manifest.json"))
		return 0
	}
	if args[0] == "retry" {
		store, err := state.Open(filepath.Join(dataDir, "state", "collector.db"))
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		defer store.Close()
		client := httpclient.New(httpclient.Options{AllowedHosts: cfg.AllowedHosts, MaxBytes: cfg.HTMLMaxBytes, Interval: cfg.RequestInterval, Timeout: cfg.RequestTimeout, Retries: cfg.RetryCount})
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		summary, err := collector.New(cfg, client, store).Retry(ctx)
		payload, _ := json.Marshal(summary)
		fmt.Fprintln(out, string(payload))
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		if summary.Status == "partial" {
			return 3
		}
		return 0
	}
	if args[0] == "unlock" {
		store, err := state.Open(filepath.Join(dataDir, "state", "collector.db"))
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		defer store.Close()
		if err := store.ReleaseLock(context.Background(), "collector"); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		fmt.Fprintln(out, "collector lock released")
		return 0
	}
	if args[0] != "collect" {
		fmt.Fprintf(errOut, "unknown command %s\n", args[0])
		return 2
	}
	store, err := state.Open(filepath.Join(dataDir, "state", "collector.db"))
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	defer store.Close()
	client := httpclient.New(httpclient.Options{AllowedHosts: cfg.AllowedHosts, MaxBytes: cfg.HTMLMaxBytes, Interval: cfg.RequestInterval, Timeout: cfg.RequestTimeout, Retries: cfg.RetryCount})
	c := collector.New(cfg, client, store)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	summary, err := c.Collect(ctx, full, maxItems)
	payload, _ := json.Marshal(summary)
	fmt.Fprintln(out, string(payload))
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if summary.Status == "partial" || summary.Status == "sampled" {
		return 3
	}
	return 0
}

func daemon(cfg config.Config, errOut io.Writer) int {
	run := func(full bool) {
		store, err := state.Open(filepath.Join(cfg.DataDir, "state", "collector.db"))
		if err != nil {
			fmt.Fprintln(errOut, err)
			return
		}
		defer store.Close()
		client := httpclient.New(httpclient.Options{AllowedHosts: cfg.AllowedHosts, MaxBytes: cfg.HTMLMaxBytes, Interval: cfg.RequestInterval, Timeout: cfg.RequestTimeout, Retries: cfg.RetryCount})
		_, _ = collector.New(cfg, client, store).Collect(context.Background(), full, 0)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := scheduler.Run(ctx, cfg, func(_ context.Context, full bool) { run(full) }); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	return 0
}
func exportManifest(args []string, dataDir string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("export-manifest", flag.ContinueOnError)
	fs.SetOutput(errOut)
	runID := ""
	fs.StringVar(&runID, "run", "", "run id")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if runID == "" {
		fmt.Fprintln(errOut, "--run is required")
		return 2
	}
	fmt.Fprintln(out, filepath.Join(dataDir, "runs", runID, "run-manifest.json"))
	return 0
}
