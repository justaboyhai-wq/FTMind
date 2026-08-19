package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/archive"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/config"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/detail"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/discovery"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/httpclient"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/model"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/normalize"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/state"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/schema"
)

type Collector struct {
	Config config.Config
	Client *httpclient.Client
	Store  *state.Store
	Now    func() time.Time
}
type Summary struct {
	RunID               string `json:"run_id"`
	Status              string `json:"status"`
	IndexCount          int    `json:"index_count"`
	UniqueIDs           int    `json:"unique_ids"`
	Created             int    `json:"created"`
	Updated             int    `json:"updated"`
	Unchanged           int    `json:"unchanged"`
	Failed              int    `json:"failed"`
	AttachmentsDeclared int    `json:"attachments_declared"`
	AttachmentsSaved    int    `json:"attachments_saved"`
	DataDir             string `json:"data_dir"`
}

type RecordResult struct {
	ExternalID       string `json:"external_id"`
	SourceURL        string `json:"source_url"`
	SnapshotID       string `json:"snapshot_id"`
	SnapshotSHA256   string `json:"snapshot_sha256"`
	AttachmentsSaved int    `json:"attachments_saved"`
	Created          bool   `json:"created"`
}

func New(cfg config.Config, client *httpclient.Client, store *state.Store) *Collector {
	return &Collector{Config: cfg, Client: client, Store: store, Now: time.Now}
}

func (c *Collector) Retry(ctx context.Context) (Summary, error) {
	if c.Store == nil {
		return c.Collect(ctx, false, 0)
	}
	failures, err := c.Store.ListRetryable(ctx, 1000)
	if err != nil {
		return Summary{Status: "failed", DataDir: c.Config.DataDir}, err
	}
	if len(failures) == 0 {
		return Summary{Status: "success", DataDir: c.Config.DataDir}, nil
	}
	filter := make(map[string]bool, len(failures))
	byID := make(map[string][]model.Failure)
	for _, f := range failures {
		key := f.ExternalID
		if key == "" {
			key = f.URL
		}
		filter[key] = true
		byID[key] = append(byID[key], f)
	}
	return c.collect(ctx, false, 0, filter, byID)
}

func (c *Collector) Collect(ctx context.Context, full bool, maxItems int) (Summary, error) {
	return c.collect(ctx, full, maxItems, nil, nil)
}

func (c *Collector) collect(ctx context.Context, full bool, maxItems int, filter map[string]bool, retryFailures map[string][]model.Failure) (Summary, error) {
	runID := c.Now().UTC().Format("20060102T150405.000000000Z")
	run := Summary{RunID: runID, Status: "discovering", DataDir: c.Config.DataDir}
	if c.Store != nil {
		locked, lockErr := c.Store.AcquireLock(ctx, "collector", 2*time.Hour)
		if lockErr != nil {
			return run, lockErr
		}
		if !locked {
			return run, state.ErrLocked
		}
		defer func() { _ = c.Store.ReleaseLock(context.Background(), "collector") }()
		if _, err := c.Store.StartRun(ctx, runID, full); err != nil {
			return run, err
		}
	}
	finish := func(err error) (Summary, error) {
		if err != nil {
			run.Status = "failed"
		} else if run.Failed > 0 {
			run.Status = "partial"
		} else if maxItems > 0 {
			run.Status = "sampled"
		} else {
			run.Status = "success"
		}
		if c.Store != nil {
			r := state.Run{ID: runID, Status: run.Status, Full: full, IndexCount: run.IndexCount, UniqueIDs: run.UniqueIDs, Created: run.Created, Updated: run.Updated, Unchanged: run.Unchanged, Failed: run.Failed}
			_ = c.Store.FinishRun(ctx, r)
		}
		_ = writeRunManifest(c.Config.DataDir, run, full, err)
		return run, err
	}
	seed, err := c.Client.Get(ctx, c.Config.SeedURL)
	if err != nil {
		return finish(err)
	}
	indexURL, err := discovery.DiscoverIndexScript(c.Config.SeedURL, seed.Body)
	if err != nil {
		return finish(err)
	}
	index, err := c.Client.Get(ctx, indexURL)
	if err != nil {
		return finish(err)
	}
	records, err := discovery.ParseAllData(index.Body)
	if err != nil {
		return finish(err)
	}
	if maxItems > 0 && len(records) > maxItems {
		records = records[:maxItems]
	}
	run.IndexCount = len(records)
	run.UniqueIDs = len(records)
	if err := writeDiscovery(c.Config.DataDir, runID, seed.Body, index.Body, records); err != nil {
		return finish(err)
	}
	for _, record := range records {
		if filter != nil && !filter[record.ID] && !filter[record.URL] {
			continue
		}
		result, err := c.collectRecord(ctx, runID, record, &run)
		if err != nil {
			run.Failed++
			if c.Store != nil {
				_ = c.Store.RecordFailure(ctx, model.Failure{RunID: runID, ExternalID: record.ID, URL: record.URL, Stage: "record", Reason: err.Error(), Attempts: 1})
			}
			_ = appendRunEvent(c.Config.DataDir, runID, "failures.ndjson", model.Failure{RunID: runID, ExternalID: record.ID, URL: record.URL, Stage: "record", Reason: err.Error(), Attempts: 1})
		} else {
			_ = appendRunEvent(c.Config.DataDir, runID, "policies.ndjson", result)
			if result.Created {
				_ = appendRunEvent(c.Config.DataDir, runID, "changes.ndjson", result)
			}
			if retryFailures != nil {
				for _, failure := range append(retryFailures[record.ID], retryFailures[record.URL]...) {
					_ = c.Store.MarkFailureDone(ctx, failure)
				}
			}
		}
	}
	if full && maxItems == 0 && run.Failed == 0 && c.Store != nil {
		seen := make([]string, 0, len(records))
		for _, r := range records {
			seen = append(seen, r.ID)
		}
		if err := c.Store.ReconcileMissing(ctx, seen); err != nil {
			return finish(err)
		}
	}
	return finish(nil)
}

func writeRunManifest(root string, summary Summary, full bool, runErr error) error {
	dir := filepath.Join(root, "runs", summary.RunID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	status := ""
	if runErr != nil {
		status = runErr.Error()
	}
	payload := map[string]any{
		"run_id": summary.RunID, "full": full, "status": summary.Status,
		"data_dir":    summary.DataDir,
		"index_count": summary.IndexCount, "unique_ids": summary.UniqueIDs,
		"created": summary.Created, "updated": summary.Updated, "unchanged": summary.Unchanged,
		"failed": summary.Failed, "attachments_declared": summary.AttachmentsDeclared,
		"attachments_saved": summary.AttachmentsSaved, "error": status,
		"completed_at": time.Now().UTC(),
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := schema.Validate("run-manifest.schema.json", b); err != nil {
		return err
	}
	tmp := filepath.Join(dir, "run-manifest.json.tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "run-manifest.json"))
}

func (c *Collector) collectRecord(ctx context.Context, runID string, record model.IndexRecord, summary *Summary) (RecordResult, error) {
	detailURL, err := detail.URLForID(c.Config.SourceBaseURL, record.ID)
	if err != nil {
		return RecordResult{}, err
	}
	rawResp, err := c.Client.Get(ctx, detailURL)
	if err != nil {
		return RecordResult{}, err
	}
	d, err := detail.Decode(rawResp.Body)
	if err != nil {
		return RecordResult{}, err
	}
	sourceURL := d.URL
	if sourceURL == "" {
		sourceURL = record.URL
	}
	htmlResp, err := c.Client.Get(ctx, sourceURL)
	if err != nil {
		return RecordResult{}, err
	}
	p := normalize.Normalized{}
	p, err = normalize.Policy(d, c.Now())
	if err != nil {
		return RecordResult{}, err
	}
	summary.AttachmentsDeclared += len(d.Attachments)
	var attachments []model.DownloadedAttachment
	var total int64
	for _, a := range d.Attachments {
		if a.URL == "" {
			continue
		}
		r, err := c.Client.GetWithMaxBytes(ctx, a.URL, c.Config.AttachmentMaxBytes)
		if err != nil {
			return RecordResult{}, err
		}
		total += int64(len(r.Body))
		if total > c.Config.PolicyAttachmentsMaxBytes {
			return RecordResult{}, fmt.Errorf("policy attachments exceed %d bytes", c.Config.PolicyAttachmentsMaxBytes)
		}
		attachments = append(attachments, model.DownloadedAttachment{Attachment: a, ActualSize: int64(len(r.Body)), SHA256: r.SHA256, Body: r.Body})
		summary.AttachmentsSaved++
	}
	structured, _ := json.Marshal(p.Structured)
	relations, _ := json.Marshal(p.Relations)
	externalID := "post_" + strconv.FormatInt(d.ID, 10)
	existed := false
	if c.Store != nil {
		existed, _ = c.Store.HasRecord(ctx, record.ID)
	}
	published, err := archive.Publish(c.Config.DataDir, archive.Package{ExternalID: externalID, DetailRaw: rawResp.Body, SourceHTML: htmlResp.Body, Markdown: p.Markdown, Structured: structured, Relations: relations, Attachments: attachments})
	if err != nil {
		return RecordResult{}, err
	}
	if c.Store != nil {
		hash, _ := discovery.RecordHash(record)
		if err := c.Store.UpsertRecord(ctx, record.ID, hash, d.URL); err != nil {
			return RecordResult{}, err
		}
	}
	if !published.Created {
		summary.Unchanged++
	} else if existed {
		summary.Updated++
	} else {
		summary.Created++
	}
	return RecordResult{ExternalID: externalID, SourceURL: d.URL, SnapshotID: published.SnapshotID, SnapshotSHA256: published.SnapshotSHA256, AttachmentsSaved: len(attachments), Created: published.Created}, nil
}

func appendRunEvent(root, runID, name string, value any) error {
	f, err := os.OpenFile(filepath.Join(root, "runs", runID, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(value)
}

func writeDiscovery(root, runID string, seed, index []byte, records []model.IndexRecord) error {
	dir := filepath.Join(root, "runs", runID, "discovery")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "seed.html"), seed, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "index-script.js"), index, 0o644); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, "index-records.ndjson"))
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			_ = f.Close()
			return err
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	ids := make([]string, 0, len(records))
	for _, r := range records {
		ids = append(ids, r.ID)
	}
	b, _ := json.MarshalIndent(map[string]any{"count": len(records), "ids": ids, "captured_at": time.Now().UTC()}, "", "  ")
	return os.WriteFile(filepath.Join(dir, "ids.json"), b, 0o644)
}
