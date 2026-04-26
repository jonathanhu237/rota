//go:build integration

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/email"
	"github.com/jonathanhu237/rota/backend/internal/repository"
	_ "github.com/lib/pq"
)

const outboxWorkerIntegrationDBLockKey int64 = 2026042301

func TestOutboxWorkerIntegrationRetryThenSucceed(t *testing.T) {
	ctx := context.Background()
	db := openOutboxWorkerIntegrationDB(t)
	repo := repository.NewOutboxRepository(db)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := repo.EnqueueTx(ctx, tx, email.Message{
		To:      "worker-cycle@example.com",
		Subject: "Worker cycle",
		Body:    "Body",
	}); err != nil {
		_ = tx.Rollback()
		t.Fatalf("enqueue: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	emailer := &workerEmailerMock{err: errors.New("temporary smtp failure")}
	processOutboxTick(ctx, repo, emailer, slog.New(slog.DiscardHandler))

	row := readOutboxWorkerIntegrationRow(t, db)
	if row.status != "pending" || row.retryCount != 1 || row.lastError != "temporary smtp failure" {
		t.Fatalf("unexpected retry state: %+v", row)
	}
	if !row.nextAttemptAt.After(time.Now()) {
		t.Fatalf("expected retry next_attempt_at in the future, got %s", row.nextAttemptAt)
	}

	if _, err := db.ExecContext(ctx, `
		UPDATE email_outbox
		SET next_attempt_at = NOW() - INTERVAL '1 minute'
		WHERE id = $1;
	`, row.id); err != nil {
		t.Fatalf("make row eligible again: %v", err)
	}

	emailer.err = nil
	processOutboxTick(ctx, repo, emailer, slog.New(slog.DiscardHandler))

	row = readOutboxWorkerIntegrationRow(t, db)
	if row.status != "sent" || row.retryCount != 1 || !row.sentAt.Valid {
		t.Fatalf("unexpected sent state: %+v", row)
	}
}

func TestOutboxWorkerIntegrationTerminalFailureAuditsInvitation(t *testing.T) {
	ctx := context.Background()
	db := openOutboxWorkerIntegrationDB(t)
	repo := repository.NewOutboxRepository(db)

	userID := insertOutboxWorkerIntegrationUser(t, db)
	msg := email.BuildInvitationMessage(email.TemplateData{
		To:         "invitee@example.com",
		Name:       "Invitee",
		BaseURL:    "https://rota.example.com",
		Token:      "raw-token",
		Language:   "en",
		Expiration: 72 * time.Hour,
	})
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := repo.EnqueueTx(ctx, tx, msg, repository.WithOutboxUserID(userID)); err != nil {
		_ = tx.Rollback()
		t.Fatalf("enqueue: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE email_outbox
		SET retry_count = 7, next_attempt_at = NOW() - INTERVAL '1 minute';
	`); err != nil {
		t.Fatalf("prepare terminal retry: %v", err)
	}

	auditCtx := audit.WithRecorder(ctx, repository.NewAuditRecorder(db, slog.New(slog.DiscardHandler)))
	emailer := &workerEmailerMock{err: errors.New("smtp rejected")}
	processOutboxTick(auditCtx, repo, emailer, slog.New(slog.DiscardHandler))

	row := readOutboxWorkerIntegrationRow(t, db)
	if row.status != "failed" || row.retryCount != 8 {
		t.Fatalf("unexpected failed state: %+v", row)
	}

	event := readOutboxWorkerIntegrationAuditEvent(t, db)
	if event.action != audit.ActionUserInvitationEmailFailed ||
		event.targetType != audit.TargetTypeUser ||
		event.targetID != userID ||
		event.email != "invitee@example.com" ||
		event.lastError != "smtp rejected" {
		t.Fatalf("unexpected audit event: %+v", event)
	}
}

type outboxWorkerIntegrationRow struct {
	id            int64
	status        string
	retryCount    int
	lastError     string
	nextAttemptAt time.Time
	sentAt        sql.NullTime
}

func readOutboxWorkerIntegrationRow(t testing.TB, db *sql.DB) outboxWorkerIntegrationRow {
	t.Helper()

	var row outboxWorkerIntegrationRow
	var lastError sql.NullString
	if err := db.QueryRowContext(context.Background(), `
		SELECT id, status, retry_count, last_error, next_attempt_at, sent_at
		FROM email_outbox
		ORDER BY id DESC
		LIMIT 1;
	`).Scan(
		&row.id,
		&row.status,
		&row.retryCount,
		&lastError,
		&row.nextAttemptAt,
		&row.sentAt,
	); err != nil {
		t.Fatalf("read outbox row: %v", err)
	}
	row.lastError = lastError.String
	return row
}

type outboxWorkerIntegrationAuditEvent struct {
	action     string
	targetType string
	targetID   int64
	email      string
	lastError  string
}

func insertOutboxWorkerIntegrationUser(t testing.TB, db *sql.DB) int64 {
	t.Helper()

	var id int64
	if err := db.QueryRowContext(context.Background(), `
		INSERT INTO users (email, password_hash, name, is_admin, status)
		VALUES ('invitee@example.com', NULL, 'Invitee', false, 'pending')
		RETURNING id;
	`).Scan(&id); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func readOutboxWorkerIntegrationAuditEvent(t testing.TB, db *sql.DB) outboxWorkerIntegrationAuditEvent {
	t.Helper()

	var event outboxWorkerIntegrationAuditEvent
	if err := db.QueryRowContext(context.Background(), `
		SELECT action, target_type, target_id, metadata->>'email', metadata->>'error'
		FROM audit_logs
		ORDER BY id DESC
		LIMIT 1;
	`).Scan(
		&event.action,
		&event.targetType,
		&event.targetID,
		&event.email,
		&event.lastError,
	); err != nil {
		t.Fatalf("read audit event: %v", err)
	}
	return event
}

func openOutboxWorkerIntegrationDB(t testing.TB) *sql.DB {
	t.Helper()

	db, err := sql.Open("postgres", outboxWorkerIntegrationDatabaseURL())
	if err != nil {
		t.Skipf("skipping integration test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Skipf("skipping integration test: %v", err)
	}

	lockConn, err := acquireOutboxWorkerIntegrationDBLock(ctx, db)
	if err != nil {
		_ = db.Close()
		t.Fatalf("acquire integration database lock: %v", err)
	}
	if err := resetOutboxWorkerIntegrationDB(ctx, db); err != nil {
		_ = releaseOutboxWorkerIntegrationDBLock(context.Background(), lockConn)
		_ = db.Close()
		t.Fatalf("reset integration database before test: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := resetOutboxWorkerIntegrationDB(ctx, db); err != nil {
			t.Fatalf("reset integration database after test: %v", err)
		}
		if err := releaseOutboxWorkerIntegrationDBLock(ctx, lockConn); err != nil {
			t.Fatalf("release integration database lock: %v", err)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("close integration database: %v", err)
		}
	})

	return db
}

func acquireOutboxWorkerIntegrationDBLock(ctx context.Context, db *sql.DB) (*sql.Conn, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}

	var locked bool
	if err := conn.QueryRowContext(
		ctx,
		`SELECT pg_advisory_lock($1) IS NOT NULL;`,
		outboxWorkerIntegrationDBLockKey,
	).Scan(&locked); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return conn, nil
}

func releaseOutboxWorkerIntegrationDBLock(ctx context.Context, conn *sql.Conn) error {
	if conn == nil {
		return nil
	}

	var unlocked bool
	err := conn.QueryRowContext(
		ctx,
		`SELECT pg_advisory_unlock($1);`,
		outboxWorkerIntegrationDBLockKey,
	).Scan(&unlocked)
	closeErr := conn.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func resetOutboxWorkerIntegrationDB(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		TRUNCATE TABLE email_outbox, audit_logs, users RESTART IDENTITY CASCADE;
	`)
	return err
}

func outboxWorkerIntegrationDatabaseURL() string {
	if value := strings.TrimSpace(os.Getenv("DATABASE_URL")); value != "" {
		return value
	}

	host := outboxWorkerIntegrationEnvOrDefault("POSTGRES_HOST", "localhost")
	port := outboxWorkerIntegrationEnvOrDefault("POSTGRES_PORT", "5432")
	user := outboxWorkerIntegrationEnvOrDefault("POSTGRES_USER", "rota")
	password := outboxWorkerIntegrationEnvOrDefault("POSTGRES_PASSWORD", "pa55word")
	database := outboxWorkerIntegrationEnvOrDefault("POSTGRES_DB", "rota")

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user,
		password,
		host,
		port,
		database,
	)
}

func outboxWorkerIntegrationEnvOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
