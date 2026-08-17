package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestExternalMemorySQLiteMigrationUpAndDown(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	_, err = db.Exec(`CREATE TABLE tenants (id INTEGER PRIMARY KEY);`)
	require.NoError(t, err)

	migrationDir := filepath.Join("..", "..", "migrations", "sqlite")
	up, err := os.ReadFile(filepath.Join(migrationDir, "000002_external_memory_l3_projection.up.sql"))
	require.NoError(t, err)
	require.Contains(t, string(up), "changes_requested")
	_, err = db.Exec(string(up))
	require.NoError(t, err)

	for _, table := range []string{
		"memory_integration_events", "memory_l3_snapshots", "memory_review_tasks",
		"memory_review_histories", "memory_wiki_publications", "memory_wiki_revisions", "wiki_claim_evidences",
	} {
		var count int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count))
		require.Equal(t, 1, count, table)
	}
	exists, notNull := sqliteColumnConstraint(t, db, "wiki_claim_evidences", "binding_id")
	require.True(t, exists, "wiki_claim_evidences.binding_id")
	require.True(t, notNull, "wiki_claim_evidences.binding_id")
	_, err = db.Exec(`INSERT INTO memory_wiki_revisions(
		id, tenant_id, team_id, binding_id, user_id, knowledge_base_id,
		wiki_page_id, wiki_page_version, page_slug, memory_id, memory_version,
		source_publication_id, source_review_task_id, content_checksum,
		projection_checksum, title, summary, content, page_type, page_status,
		page_snapshot
	) VALUES (
		'rev-1', 7, 'team-a', 'binding-a', 'user-a', 'kb-a',
		'page-a', 1, 'memory/a', 'memory-a', 1,
		'publication-a', 'review-a', 'sha256:content',
		'sha256:projection', 'Title', 'Summary', 'Content', 'memory', 'published',
		'{}'
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE memory_wiki_revisions SET title = 'mutated' WHERE id = 'rev-1'`)
	require.ErrorContains(t, err, "revisions are immutable")
	_, err = db.Exec(`DELETE FROM memory_wiki_revisions WHERE id = 'rev-1'`)
	require.ErrorContains(t, err, "revisions are immutable")
	for _, index := range []string{
		"ux_memory_integration_event_id", "ux_memory_integration_projection", "ux_memory_snapshot_projection",
		"ux_memory_publication_projection", "ux_memory_wiki_revision_page_checksum",
	} {
		var count int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, index).Scan(&count))
		require.Equal(t, 1, count, index)
	}
	for _, table := range []string{
		"memory_integration_events", "memory_l3_snapshots", "memory_review_tasks",
		"memory_review_histories", "memory_wiki_publications", "memory_wiki_revisions", "wiki_claim_evidences",
	} {
		exists, notNull := sqliteColumnConstraint(t, db, table, "user_id")
		require.True(t, exists, "%s.user_id", table)
		require.True(t, notNull, "%s.user_id", table)
	}
	for _, table := range []string{
		"memory_integration_events", "memory_l3_snapshots", "memory_review_tasks", "memory_wiki_publications",
	} {
		for _, column := range []string{"department_id", "workspace_id", "project_id", "task_id"} {
			exists, notNull := sqliteColumnConstraint(t, db, table, column)
			require.True(t, exists, "%s.%s", table, column)
			require.False(t, notNull, "%s.%s must remain optional", table, column)
		}
	}

	down, err := os.ReadFile(filepath.Join(migrationDir, "000002_external_memory_l3_projection.down.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(down))
	require.NoError(t, err)
	var remaining int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name LIKE 'memory_%'`).Scan(&remaining))
	require.Zero(t, remaining)
}

func TestMemoryWikiIsolationSQLiteMigrationEnforcesOneTeamAndZeroRawIngest(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	_, err = db.Exec(`
		CREATE TABLE knowledge_bases (
			id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL, type TEXT NOT NULL,
			wiki_config TEXT, deleted_at DATETIME
		);
		CREATE TABLE knowledges (
			id TEXT PRIMARY KEY, knowledge_base_id TEXT NOT NULL, deleted_at DATETIME
		);
		CREATE TABLE chunks (
			id TEXT PRIMARY KEY, knowledge_base_id TEXT NOT NULL, deleted_at DATETIME
		);
		CREATE TABLE kb_shares (
			id TEXT PRIMARY KEY, knowledge_base_id TEXT NOT NULL,
			organization_id TEXT NOT NULL, deleted_at DATETIME
		);`)
	require.NoError(t, err)
	path := filepath.Join("..", "..", "migrations", "sqlite", "000003_memory_wiki_isolation.up.sql")
	up, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, execSQLiteStatements(db, string(up)))

	exists, notNull := sqliteColumnConstraint(t, db, "knowledge_bases", "is_memory_wiki")
	require.True(t, exists)
	require.True(t, notNull)
	exists, _ = sqliteColumnConstraint(t, db, "knowledge_bases", "memory_team_id")
	require.True(t, exists)

	_, err = db.Exec(`INSERT INTO knowledge_bases(id, tenant_id, type, is_memory_wiki, memory_team_id) VALUES ('kb-1', 7, 'wiki', 1, 'team-a')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO knowledge_bases(id, tenant_id, type, is_memory_wiki, memory_team_id) VALUES ('kb-2', 7, 'wiki', 1, 'team-a')`)
	require.Error(t, err)
	_, err = db.Exec(`INSERT INTO knowledges(id, knowledge_base_id) VALUES ('doc-1', 'kb-1')`)
	require.ErrorContains(t, err, "rejects document/RAG ingestion")
	_, err = db.Exec(`INSERT INTO chunks(id, knowledge_base_id) VALUES ('chunk-1', 'kb-1')`)
	require.ErrorContains(t, err, "rejects document/RAG ingestion")
	_, err = db.Exec(`INSERT INTO kb_shares(id, knowledge_base_id, organization_id) VALUES ('share-1', 'kb-1', 'org-1')`)
	require.ErrorContains(t, err, "cannot be organization-shared")

	_, err = db.Exec(`INSERT INTO knowledge_bases(id, tenant_id, type) VALUES ('kb-normal', 7, 'document')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO knowledges(id, knowledge_base_id) VALUES ('doc-normal', 'kb-normal')`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE knowledge_bases SET is_memory_wiki = 1, memory_team_id = 'team-b', type = 'wiki' WHERE id = 'kb-normal'`)
	require.ErrorContains(t, err, "populated knowledge base")
	_, err = db.Exec(`INSERT INTO knowledge_bases(id, tenant_id, type) VALUES ('kb-normal-chunk', 7, 'document')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO chunks(id, knowledge_base_id) VALUES ('chunk-normal', 'kb-normal-chunk')`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE knowledge_bases SET is_memory_wiki = 1, memory_team_id = 'team-c', type = 'wiki' WHERE id = 'kb-normal-chunk'`)
	require.ErrorContains(t, err, "populated knowledge base")

	down, err := os.ReadFile(filepath.Join("..", "..", "migrations", "sqlite", "000003_memory_wiki_isolation.down.sql"))
	require.NoError(t, err)
	require.NoError(t, execSQLiteStatements(db, string(down)))
}

func execSQLiteStatements(db *sql.DB, script string) error {
	_, err := db.Exec(script)
	return err
}

func sqliteColumnConstraint(t *testing.T, db *sql.DB, table, column string) (bool, bool) {
	t.Helper()
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%q)", table))
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue sql.NullString
		require.NoError(t, rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey))
		if name == column {
			return true, notNull == 1
		}
	}
	require.NoError(t, rows.Err())
	return false, false
}

func TestExternalMemoryPostgresMigrationWidensLegacyScopeColumns(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "versioned", "000072_external_memory_l3_projection.up.sql")
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	sql := strings.ToLower(string(contents))
	require.Contains(t, sql, "changes_requested")
	require.Contains(t, sql, "create table if not exists memory_wiki_revisions")
	require.Contains(t, sql, "ux_memory_wiki_revision_page_checksum")
	require.Contains(t, sql, "ux_memory_integration_projection")

	for _, clause := range []string{
		"alter column workspace_id type varchar(128)",
		"alter column project_id type varchar(128)",
		"alter column reviewed_by type varchar(128)",
	} {
		require.Contains(t, sql, clause)
	}
	require.GreaterOrEqual(t, strings.Count(sql, "user_id varchar(128) not null"), 6)
	for _, clause := range []string{
		"department_id varchar(128) not null",
		"workspace_id varchar(128) not null",
		"project_id varchar(128) not null",
		"task_id varchar(128) not null",
		"alter column workspace_id set not null",
		"alter column project_id set not null",
	} {
		require.NotContains(t, sql, clause)
	}
}

func TestMemoryWikiIsolationPostgresMigrationDeclaresDurableGuards(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "migrations", "versioned", "000073_memory_wiki_isolation.up.sql"))
	require.NoError(t, err)
	sql := strings.ToLower(string(contents))
	for _, clause := range []string{
		"add column if not exists is_memory_wiki",
		"add column if not exists memory_team_id",
		"ux_knowledge_bases_memory_team",
		"where is_memory_wiki = true and deleted_at is null",
		"trg_reject_memory_wiki_knowledge_ingest",
		"trg_reject_populated_memory_wiki_marker",
	} {
		require.Contains(t, sql, clause)
	}
}

func TestExternalMemoryPostgresMigrationUpAndDown(t *testing.T) {
	dsn := os.Getenv("FMIND_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set FMIND_TEST_POSTGRES_DSN to run the real PostgreSQL migration test")
	}
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.Ping())
	tx, err := db.Begin()
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	schema := fmt.Sprintf("memorywiki_migration_%d", time.Now().UnixNano())
	_, err = tx.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	_, err = tx.Exec(`SET LOCAL search_path TO ` + schema)
	require.NoError(t, err)
	_, err = tx.Exec(`CREATE TABLE tenants (id BIGINT PRIMARY KEY)`)
	require.NoError(t, err)

	migrationDir := filepath.Join("..", "..", "migrations", "versioned")
	legacy, err := os.ReadFile(filepath.Join(migrationDir, "000070_memory_wiki_publications.up.sql"))
	require.NoError(t, err)
	_, err = tx.Exec(string(legacy))
	require.NoError(t, err)
	up, err := os.ReadFile(filepath.Join(migrationDir, "000072_external_memory_l3_projection.up.sql"))
	require.NoError(t, err)
	_, err = tx.Exec(string(up))
	require.NoError(t, err)

	for _, table := range []string{
		"memory_integration_events", "memory_l3_snapshots", "memory_review_tasks",
		"memory_review_histories", "memory_wiki_revisions", "wiki_claim_evidences",
	} {
		var relation sql.NullString
		require.NoError(t, tx.QueryRow(`SELECT to_regclass($1)`, table).Scan(&relation))
		require.True(t, relation.Valid, table)
	}
	var indexDefinition string
	require.NoError(t, tx.QueryRow(`SELECT indexdef FROM pg_indexes WHERE schemaname = current_schema() AND indexname = 'ux_memory_integration_projection'`).Scan(&indexDefinition))
	require.Contains(t, strings.ToUpper(indexDefinition), "CREATE UNIQUE INDEX")
	var statusConstraint string
	require.NoError(t, tx.QueryRow(`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid = 'memory_review_tasks'::regclass AND conname = 'ck_memory_review_status'`).Scan(&statusConstraint))
	require.Contains(t, statusConstraint, "changes_requested")
	for _, column := range []string{"department_id", "workspace_id", "project_id", "task_id"} {
		var nullable string
		require.NoError(t, tx.QueryRow(`SELECT is_nullable FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'memory_review_tasks' AND column_name = $1`, column).Scan(&nullable))
		require.Equal(t, "YES", nullable, column)
	}

	_, err = tx.Exec(`
		CREATE TABLE knowledge_bases (
			id TEXT PRIMARY KEY, tenant_id BIGINT NOT NULL, type TEXT NOT NULL,
			wiki_config JSONB, deleted_at TIMESTAMP
		);
		CREATE TABLE knowledges (
			id TEXT PRIMARY KEY, knowledge_base_id TEXT NOT NULL, deleted_at TIMESTAMP
		);
		CREATE TABLE chunks (
			id TEXT PRIMARY KEY, knowledge_base_id TEXT NOT NULL, deleted_at TIMESTAMP
		);
		CREATE TABLE kb_shares (
			id TEXT PRIMARY KEY, knowledge_base_id TEXT NOT NULL, organization_id TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at TIMESTAMP
		);`)
	require.NoError(t, err)
	isolationUp, err := os.ReadFile(filepath.Join(migrationDir, "000073_memory_wiki_isolation.up.sql"))
	require.NoError(t, err)
	_, err = tx.Exec(string(isolationUp))
	require.NoError(t, err)

	for _, column := range []string{"is_memory_wiki", "memory_team_id"} {
		var exists bool
		require.NoError(t, tx.QueryRow(`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = 'knowledge_bases' AND column_name = $1
		)`, column).Scan(&exists))
		require.True(t, exists, column)
	}
	_, err = tx.Exec(`INSERT INTO knowledge_bases(id, tenant_id, type, is_memory_wiki, memory_team_id) VALUES ('kb-memory', 7, 'wiki', TRUE, 'team-a')`)
	require.NoError(t, err)
	expectPostgresGuard := func(statement string) {
		t.Helper()
		require.NoError(t, func() error { _, saveErr := tx.Exec(`SAVEPOINT memory_wiki_guard`); return saveErr }())
		_, guardedErr := tx.Exec(statement)
		require.Error(t, guardedErr, statement)
		require.NoError(t, func() error { _, rollbackErr := tx.Exec(`ROLLBACK TO SAVEPOINT memory_wiki_guard`); return rollbackErr }())
		require.NoError(t, func() error { _, releaseErr := tx.Exec(`RELEASE SAVEPOINT memory_wiki_guard`); return releaseErr }())
	}
	expectPostgresGuard(`INSERT INTO knowledge_bases(id, tenant_id, type, is_memory_wiki, memory_team_id) VALUES ('kb-memory-duplicate', 7, 'wiki', TRUE, 'team-a')`)
	expectPostgresGuard(`INSERT INTO knowledges(id, knowledge_base_id) VALUES ('doc-memory', 'kb-memory')`)
	expectPostgresGuard(`INSERT INTO chunks(id, knowledge_base_id) VALUES ('chunk-memory', 'kb-memory')`)
	expectPostgresGuard(`INSERT INTO kb_shares(id, knowledge_base_id, organization_id) VALUES ('share-memory', 'kb-memory', 'org-a')`)
	_, err = tx.Exec(`INSERT INTO knowledge_bases(id, tenant_id, type) VALUES ('kb-populated', 7, 'document')`)
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO chunks(id, knowledge_base_id) VALUES ('chunk-normal', 'kb-populated')`)
	require.NoError(t, err)
	expectPostgresGuard(`UPDATE knowledge_bases SET type='wiki', is_memory_wiki=TRUE, memory_team_id='team-b' WHERE id='kb-populated'`)

	isolationDown, err := os.ReadFile(filepath.Join(migrationDir, "000073_memory_wiki_isolation.down.sql"))
	require.NoError(t, err)
	_, err = tx.Exec(string(isolationDown))
	require.NoError(t, err)
	var isolationColumnCount int
	require.NoError(t, tx.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'knowledge_bases'
		AND column_name IN ('is_memory_wiki', 'memory_team_id')`).Scan(&isolationColumnCount))
	require.Zero(t, isolationColumnCount)

	down, err := os.ReadFile(filepath.Join(migrationDir, "000072_external_memory_l3_projection.down.sql"))
	require.NoError(t, err)
	_, err = tx.Exec(string(down))
	require.NoError(t, err)
	var relation sql.NullString
	require.NoError(t, tx.QueryRow(`SELECT to_regclass('memory_wiki_revisions')`).Scan(&relation))
	require.False(t, relation.Valid)
}
