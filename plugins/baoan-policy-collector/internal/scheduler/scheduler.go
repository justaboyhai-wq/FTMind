package scheduler

import (
	"context"
	"time"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/config"
	"github.com/robfig/cron/v3"
)

type Job func(context.Context, bool)

// New creates the Asia/Shanghai scheduler. SkipIfStillRunning prevents two
// daemon processes/jobs from downloading the same source concurrently.
func New(cfg config.Config, job Job) (*cron.Cron, error) {
	c := cron.New(
		cron.WithLocation(time.FixedZone("Asia/Shanghai", 8*60*60)),
		cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)),
	)
	if _, err := c.AddFunc(cfg.IncrementalCron, func() { job(context.Background(), false) }); err != nil {
		return nil, err
	}
	if _, err := c.AddFunc(cfg.FullCron, func() { job(context.Background(), true) }); err != nil {
		return nil, err
	}
	return c, nil
}

func Run(ctx context.Context, cfg config.Config, job Job) error {
	c, err := New(cfg, job)
	if err != nil {
		return err
	}
	c.Start()
	<-ctx.Done()
	<-c.Stop().Done()
	return nil
}
