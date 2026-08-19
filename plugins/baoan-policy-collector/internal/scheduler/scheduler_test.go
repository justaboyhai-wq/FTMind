package scheduler

import (
	"context"
	"testing"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/config"
)

func TestNewRegistersBothSchedules(t *testing.T) {
	cfg := config.Default()
	c, err := New(cfg, func(context.Context, bool) {})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(c.Entries()); got != 2 {
		t.Fatalf("entries=%d", got)
	}
}

func TestNewRejectsInvalidSchedule(t *testing.T) {
	cfg := config.Default()
	cfg.FullCron = "not a cron"
	if _, err := New(cfg, func(context.Context, bool) {}); err == nil {
		t.Fatal("expected cron parse error")
	}
}
