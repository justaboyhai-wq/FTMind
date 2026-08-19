package state

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/model"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "collector.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func TestFailedDetailRemainsRetryable(t *testing.T) {
	s := openTestStore(t)
	if err := s.RecordFailure(context.Background(), model.Failure{RunID: "r1", URL: "https://www.baoan.gov.cn/x", Stage: "detail", Reason: "503"}); err != nil {
		t.Fatal(err)
	}
	x, err := s.ListRetryable(context.Background(), 10)
	if err != nil || len(x) != 1 || x[0].RunID != "r1" {
		t.Fatalf("items=%+v err=%v", x, err)
	}
}

func TestCollectorLockSerializesRuns(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	ok, err := s.AcquireLock(ctx, "collector", time.Hour)
	if err != nil || !ok {
		t.Fatalf("first lock: ok=%v err=%v", ok, err)
	}
	ok, err = s.AcquireLock(ctx, "collector", time.Hour)
	if err != nil || ok {
		t.Fatalf("second lock: ok=%v err=%v", ok, err)
	}
	if err := s.ReleaseLock(ctx, "collector"); err != nil {
		t.Fatal(err)
	}
	ok, err = s.AcquireLock(ctx, "collector", time.Hour)
	if err != nil || !ok {
		t.Fatalf("lock after release: ok=%v err=%v", ok, err)
	}
}

func TestMarkFailureDoneRemovesRetryCandidate(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := model.Failure{RunID: "r1", ExternalID: "1", URL: "https://example.com/1", Stage: "detail", Reason: "temporary"}
	if err := s.RecordFailure(ctx, f); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkFailureDone(ctx, f); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListRetryable(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("retryable=%+v", items)
	}
}
func TestRunLifecycle(t *testing.T) {
	s := openTestStore(t)
	r, err := s.StartRun(context.Background(), "r1", true)
	if err != nil {
		t.Fatal(err)
	}
	r.Status = "success"
	r.IndexCount = 3
	r.UniqueIDs = 3
	if err := s.FinishRun(context.Background(), r); err != nil {
		t.Fatal(err)
	}
}
