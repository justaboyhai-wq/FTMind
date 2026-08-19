package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/collector"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/config"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/httpclient"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/state"
	"github.com/robfig/cron/v3"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "usage: collect --full|--incremental | retry --failed | verify --all | export-manifest --run ID | daemon")
		return 2
	}
	cfg := config.Default()
	dataDir := cfg.DataDir
	maxItems := 0
	all := false
	runID := ""
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
	if args[0] == "verify" {
		fmt.Fprintf(out, "verification requested for %s\n", dataDir)
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
		fmt.Fprintln(out, "retry queue is handled by the next collection run")
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
		return 1
	}
	if summary.Status == "partial" || summary.Status == "sampled" {
		return 3
	}
	return 0
}

func daemon(cfg config.Config, errOut io.Writer) int {
	c := cron.New(cron.WithLocation(time.FixedZone("Asia/Shanghai", 8*60*60)))
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
	if _, err := c.AddFunc(cfg.IncrementalCron, func() { run(false) }); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if _, err := c.AddFunc(cfg.FullCron, func() { run(true) }); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	c.Start()
	select {}
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
