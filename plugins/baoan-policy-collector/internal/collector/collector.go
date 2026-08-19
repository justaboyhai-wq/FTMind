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
)

type Collector struct {
	Config config.Config
	Client *httpclient.Client
	Store  *state.Store
	Now    func() time.Time
}
type Summary struct {
	RunID, Status                                                                                     string
	IndexCount, UniqueIDs, Created, Updated, Unchanged, Failed, AttachmentsDeclared, AttachmentsSaved int
}

func New(cfg config.Config, client *httpclient.Client, store *state.Store) *Collector {
	return &Collector{Config: cfg, Client: client, Store: store, Now: time.Now}
}

func (c *Collector) Collect(ctx context.Context, full bool, maxItems int) (Summary, error) {
	runID := c.Now().UTC().Format("20060102T150405.000000000Z")
	run := Summary{RunID: runID, Status: "discovering"}
	if c.Store != nil {
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
		if err := c.collectRecord(ctx, runID, record, &run); err != nil {
			run.Failed++
			if c.Store != nil {
				_ = c.Store.RecordFailure(ctx, model.Failure{RunID: runID, URL: record.URL, Stage: "record", Reason: err.Error(), Attempts: 1})
			}
		}
	}
	return finish(nil)
}

func (c *Collector) collectRecord(ctx context.Context, runID string, record model.IndexRecord, summary *Summary) error {
	detailURL, err := detail.URLForID("https://www.baoan.gov.cn", record.ID)
	if err != nil {
		return err
	}
	rawResp, err := c.Client.Get(ctx, detailURL)
	if err != nil {
		return err
	}
	d, err := detail.Decode(rawResp.Body)
	if err != nil {
		return err
	}
	sourceURL := d.URL
	if sourceURL == "" {
		sourceURL = record.URL
	}
	htmlResp, err := c.Client.Get(ctx, sourceURL)
	if err != nil {
		return err
	}
	p := normalize.Normalized{}
	p, err = normalize.Policy(d, c.Now())
	if err != nil {
		return err
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
			return err
		}
		total += int64(len(r.Body))
		if total > c.Config.PolicyAttachmentsMaxBytes {
			return fmt.Errorf("policy attachments exceed %d bytes", c.Config.PolicyAttachmentsMaxBytes)
		}
		attachments = append(attachments, model.DownloadedAttachment{Attachment: a, ActualSize: int64(len(r.Body)), SHA256: r.SHA256, Body: r.Body})
		summary.AttachmentsSaved++
	}
	structured, _ := json.Marshal(p.Structured)
	relations, _ := json.Marshal(p.Relations)
	_, err = archive.Publish(c.Config.DataDir, archive.Package{ExternalID: "post_" + strconv.FormatInt(d.ID, 10), DetailRaw: rawResp.Body, SourceHTML: htmlResp.Body, Markdown: p.Markdown, Structured: structured, Relations: relations, Attachments: attachments})
	if err != nil {
		return err
	}
	if c.Store != nil {
		hash, _ := discovery.RecordHash(record)
		_ = c.Store.UpsertRecord(ctx, record.ID, hash, d.URL)
	}
	summary.Created++
	return nil
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
