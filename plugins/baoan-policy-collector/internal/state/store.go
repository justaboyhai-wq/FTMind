package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/model"
)

type Store struct{ db *sql.DB }
type Run struct {
	ID, Status                                                 string
	Full                                                       bool
	StartedAt, CompletedAt                                     time.Time
	IndexCount, UniqueIDs, Created, Updated, Unchanged, Failed int
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) migrate() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS runs(id TEXT PRIMARY KEY,status TEXT NOT NULL,full_run INTEGER NOT NULL,started_at TEXT NOT NULL,completed_at TEXT,index_count INTEGER NOT NULL DEFAULT 0,unique_ids INTEGER NOT NULL DEFAULT 0,created_count INTEGER NOT NULL DEFAULT 0,updated_count INTEGER NOT NULL DEFAULT 0,unchanged_count INTEGER NOT NULL DEFAULT 0,failed_count INTEGER NOT NULL DEFAULT 0);CREATE TABLE IF NOT EXISTS failures(id INTEGER PRIMARY KEY AUTOINCREMENT,run_id TEXT NOT NULL,url TEXT NOT NULL,stage TEXT NOT NULL,reason TEXT NOT NULL,attempts INTEGER NOT NULL DEFAULT 0,next_retry_at TEXT,done INTEGER NOT NULL DEFAULT 0);CREATE TABLE IF NOT EXISTS records(external_id TEXT PRIMARY KEY,index_hash TEXT NOT NULL,last_snapshot TEXT,missing_runs INTEGER NOT NULL DEFAULT 0,source_state TEXT NOT NULL DEFAULT 'active')`)
	return err
}
func (s *Store) StartRun(ctx context.Context, id string, full bool) (Run, error) {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `INSERT INTO runs(id,status,full_run,started_at) VALUES(?,?,?,?)`, id, "discovering", boolInt(full), now.Format(time.RFC3339Nano))
	return Run{ID: id, Status: "discovering", Full: full, StartedAt: now}, err
}
func (s *Store) FinishRun(ctx context.Context, r Run) error {
	_, err := s.db.ExecContext(ctx, `UPDATE runs SET status=?,completed_at=?,index_count=?,unique_ids=?,created_count=?,updated_count=?,unchanged_count=?,failed_count=? WHERE id=?`, r.Status, time.Now().UTC().Format(time.RFC3339Nano), r.IndexCount, r.UniqueIDs, r.Created, r.Updated, r.Unchanged, r.Failed, r.ID)
	return err
}
func (s *Store) RecordFailure(ctx context.Context, f model.Failure) error {
	var next any
	if !f.NextRetryAt.IsZero() {
		next = f.NextRetryAt.UTC().Format(time.RFC3339Nano)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO failures(run_id,url,stage,reason,attempts,next_retry_at) VALUES(?,?,?,?,?,?)`, f.RunID, f.URL, f.Stage, f.Reason, f.Attempts, next)
	return err
}
func (s *Store) ListRetryable(ctx context.Context, limit int) ([]model.Failure, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT run_id,url,stage,reason,attempts,next_retry_at FROM failures WHERE done=0 AND (next_retry_at IS NULL OR next_retry_at<=?) ORDER BY id LIMIT ?`, time.Now().UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Failure
	for rows.Next() {
		var f model.Failure
		var next sql.NullString
		if err := rows.Scan(&f.RunID, &f.URL, &f.Stage, &f.Reason, &f.Attempts, &next); err != nil {
			return nil, err
		}
		if next.Valid {
			f.NextRetryAt, _ = time.Parse(time.RFC3339Nano, next.String)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
func (s *Store) UpsertRecord(ctx context.Context, id, hash, snapshot string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO records(external_id,index_hash,last_snapshot,source_state) VALUES(?,?,?,'active') ON CONFLICT(external_id) DO UPDATE SET index_hash=excluded.index_hash,last_snapshot=excluded.last_snapshot,missing_runs=0,source_state='active'`, id, hash, snapshot)
	return err
}
func (s *Store) MarkMissing(ctx context.Context, ids []string) error {
	b, _ := json.Marshal(ids)
	_ = b
	return nil
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func (s *Store) String() string { return fmt.Sprintf("%p", s) }
