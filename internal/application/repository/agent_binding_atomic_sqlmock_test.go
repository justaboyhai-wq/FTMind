package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newAgentBindingSQLMock(t *testing.T) (*agentBindingRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, PreferSimpleProtocol: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &agentBindingRepository{db: db}, mock
}

func TestAgentBindingAtomicResolveRejectsOldKeyRevokedWhileWaitingForBindingLock(t *testing.T) {
	repo, mock := newAgentBindingSQLMock(t)
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	keyRows := []string{"id", "binding_id", "tenant_id", "key_hash", "revoked_at", "expires_at"}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "agent_binding_keys" WHERE .*key_hash = \$1.*revoked_at IS NULL.*expires_at > \$2.*LIMIT \$3`).
		WithArgs("old-hash", now, 1).
		WillReturnRows(sqlmock.NewRows(keyRows).AddRow("key-old", "binding-1", 42, "old-hash", nil, nil))
	mock.ExpectQuery(`SELECT .* FROM "agent_bindings" WHERE .*tenant_id = \$1.*id = \$2.*status = \$3.*expires_at > \$4.*LIMIT \$5 FOR UPDATE`).
		WithArgs(uint64(42), "binding-1", types.AgentBindingStatusActive, now, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "status", "policy_version"}).
			AddRow("binding-1", 42, types.AgentBindingStatusActive, 2))
	mock.ExpectQuery(`SELECT .* FROM "agent_binding_keys" WHERE .*id = \$1.*key_hash = \$2.*tenant_id = \$3.*binding_id = \$4.*revoked_at IS NULL.*expires_at > \$5.*LIMIT \$6 FOR UPDATE`).
		WithArgs("key-old", "old-hash", uint64(42), "binding-1", now, 1).
		WillReturnRows(sqlmock.NewRows(keyRows))
	mock.ExpectRollback()

	key, binding, err := repo.ResolveActiveKeyAndBinding(context.Background(), "old-hash", now)
	if err == nil || key != nil || binding != nil {
		t.Fatalf("old key resolved against new policy: key=%+v binding=%+v err=%v", key, binding, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentBindingReadsExcludeSoftDeletedRows(t *testing.T) {
	repo, mock := newAgentBindingSQLMock(t)
	mock.ExpectQuery(`SELECT .* FROM "agent_bindings" WHERE .*tenant_id = \$1 AND id = \$2.*"agent_bindings"\."deleted_at" IS NULL.*LIMIT \$3`).
		WithArgs(uint64(42), "binding-deleted", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "status"}))

	_, err := repo.GetAgentBinding(context.Background(), 42, "binding-deleted")
	if !errors.Is(err, ErrAgentBindingNotFound) {
		t.Fatalf("soft-deleted binding lookup must fail closed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentBindingAtomicCreateRollsBackWhenKeyInsertFails(t *testing.T) {
	repo, mock := newAgentBindingSQLMock(t)
	binding := &types.AgentBinding{ID: "binding-1", TenantID: 42, TeamID: "team-1", UserID: "user-1", AgentID: "agent-1", ExternalAgent: "external", ConnectorType: "generic_sdk", Status: types.AgentBindingStatusActive}
	key := &types.AgentBindingKey{ID: "key-1", BindingID: binding.ID, TenantID: 42, KeyPrefix: "fmind_prefix", KeyHash: "hash"}
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "agent_bindings"`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO "agent_binding_keys"`).WillReturnError(errors.New("key insert failed"))
	mock.ExpectRollback()
	if err := repo.CreateAgentBindingWithKey(context.Background(), binding, key); err == nil {
		t.Fatal("expected key insert error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentBindingAtomicRotationInsertsBeforeRevokeAndRollsBack(t *testing.T) {
	repo, mock := newAgentBindingSQLMock(t)
	key := &types.AgentBindingKey{ID: "key-new", BindingID: "binding-1", TenantID: 42, KeyPrefix: "fmind_prefix", KeyHash: "new-hash"}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "agent_bindings" WHERE .*tenant_id = \$1 AND id = \$2 AND status = \$3.*FOR UPDATE`).
		WithArgs(uint64(42), "binding-1", types.AgentBindingStatusActive, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "status"}).AddRow("binding-1", 42, "active"))
	mock.ExpectExec(`INSERT INTO "agent_binding_keys"`).WillReturnError(errors.New("replacement insert failed"))
	mock.ExpectRollback()
	if err := repo.RotateAgentBindingKey(context.Background(), 42, "binding-1", key); err == nil {
		t.Fatal("expected replacement insert error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentBindingAtomicRotationRollsBackWhenPolicyVersionUpdateMissesRow(t *testing.T) {
	repo, mock := newAgentBindingSQLMock(t)
	key := &types.AgentBindingKey{ID: "key-new", BindingID: "binding-1", TenantID: 42, KeyPrefix: "fmind_prefix", KeyHash: "new-hash"}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "agent_bindings" WHERE .*tenant_id = \$1 AND id = \$2 AND status = \$3.*FOR UPDATE`).
		WithArgs(uint64(42), "binding-1", types.AgentBindingStatusActive, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "status", "policy_version"}).AddRow("binding-1", 42, "active", 1))
	mock.ExpectExec(`INSERT INTO "agent_binding_keys"`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE "agent_binding_keys"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "agent_bindings"`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := repo.RotateAgentBindingKey(context.Background(), 42, "binding-1", key)
	if !errors.Is(err, ErrAgentBindingNotFound) {
		t.Fatalf("zero-row policy update was not rejected: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentBindingAtomicRotationRejectsPolicyVersionOverflowBeforeKeyInsert(t *testing.T) {
	repo, mock := newAgentBindingSQLMock(t)
	key := &types.AgentBindingKey{ID: "key-new", BindingID: "binding-1", TenantID: 42, KeyPrefix: "fmind_prefix", KeyHash: "new-hash"}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "agent_bindings" WHERE .*tenant_id = \$1 AND id = \$2 AND status = \$3.*FOR UPDATE`).
		WithArgs(uint64(42), "binding-1", types.AgentBindingStatusActive, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "status", "policy_version"}).
			AddRow("binding-1", 42, "active", int64(1<<63-1)))
	mock.ExpectRollback()

	err := repo.RotateAgentBindingKey(context.Background(), 42, "binding-1", key)
	if !errors.Is(err, ErrAgentBindingPolicyVersionOverflow) {
		t.Fatalf("overflowing policy version was not rejected: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentBindingAtomicRevokeRollsBackWhenKeyUpdateFails(t *testing.T) {
	repo, mock := newAgentBindingSQLMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "agent_bindings" WHERE .*tenant_id = \$1 AND id = \$2.*FOR UPDATE`).
		WithArgs(uint64(42), "binding-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "status"}).AddRow("binding-1", 42, "active"))
	mock.ExpectExec(`UPDATE "agent_bindings"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "agent_binding_keys"`).WillReturnError(errors.New("key revoke failed"))
	mock.ExpectRollback()
	if err := repo.RevokeAgentBindingWithKeys(context.Background(), 42, "binding-1"); err == nil {
		t.Fatal("expected key revoke error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
