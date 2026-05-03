//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/email"
)

func TestOutboxRepositoryIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("Enqueue and Claim round-trip", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewOutboxRepository(db)
		user := seedUser(t, db, userSeed{})

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := repo.EnqueueTx(ctx, tx, email.Message{
			Kind:     email.KindInvitation,
			To:       "worker@example.com",
			Subject:  "Welcome",
			Body:     "Body",
			HTMLBody: "<p>Body</p>",
		}, WithOutboxUserID(user.ID)); err != nil {
			_ = tx.Rollback()
			t.Fatalf("enqueue: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}

		jobs, err := repo.Claim(ctx, 10)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if len(jobs) != 1 {
			t.Fatalf("expected one job, got %d", len(jobs))
		}
		job := jobs[0]
		if job.UserID == nil || *job.UserID != user.ID {
			t.Fatalf("job userID = %v, want %d", job.UserID, user.ID)
		}
		if job.Kind != email.KindInvitation ||
			job.Recipient != "worker@example.com" ||
			job.Subject != "Welcome" ||
			job.Body != "Body" ||
			job.HTMLBody != "<p>Body</p>" {
			t.Fatalf("unexpected job: %+v", job)
		}
		if !outboxRowHasFutureAttempt(t, db, job.ID) {
			t.Fatalf("expected claimed job %d to receive a visibility lease", job.ID)
		}
	})

	t.Run("Enqueue rolls back with transaction", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewOutboxRepository(db)

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := repo.EnqueueTx(ctx, tx, email.Message{
			To:      "rollback@example.com",
			Subject: "Rollback",
			Body:    "Body",
		}); err != nil {
			_ = tx.Rollback()
			t.Fatalf("enqueue: %v", err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatalf("rollback: %v", err)
		}
		if count := countOutboxRows(t, db); count != 0 {
			t.Fatalf("outbox row count after rollback = %d, want 0", count)
		}
	})

	t.Run("Claim skips future rows and terminal states", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewOutboxRepository(db)

		insertOutboxRow(t, db, outboxSeed{Recipient: "future@example.com", NextAttemptAt: time.Now().Add(time.Hour)})
		insertOutboxRow(t, db, outboxSeed{Recipient: "sent@example.com", Status: "sent"})
		insertOutboxRow(t, db, outboxSeed{Recipient: "failed@example.com", Status: "failed"})
		readyID := insertOutboxRow(t, db, outboxSeed{Recipient: "ready@example.com"})

		jobs, err := repo.Claim(ctx, 10)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if len(jobs) != 1 || jobs[0].ID != readyID {
			t.Fatalf("expected only ready job %d, got %+v", readyID, jobs)
		}
		if jobs[0].Kind != email.KindUnknown || jobs[0].HTMLBody != "" {
			t.Fatalf("legacy row should claim as unknown plain text, got %+v", jobs[0])
		}
	})

	t.Run("Concurrent claims do not overlap", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewOutboxRepository(db)
		for i := 0; i < 15; i++ {
			insertOutboxRow(t, db, outboxSeed{Recipient: fmt.Sprintf("worker-%d@example.com", i)})
		}

		var (
			wg      sync.WaitGroup
			mu      sync.Mutex
			claimed []int64
			errs    []error
		)
		wg.Add(2)
		for i := 0; i < 2; i++ {
			go func() {
				defer wg.Done()
				jobs, err := repo.Claim(ctx, 10)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					errs = append(errs, err)
					return
				}
				for _, job := range jobs {
					claimed = append(claimed, job.ID)
				}
			}()
		}
		wg.Wait()
		if len(errs) > 0 {
			t.Fatalf("claim errors: %v", errs)
		}

		sort.Slice(claimed, func(i, j int) bool { return claimed[i] < claimed[j] })
		seen := make(map[int64]struct{}, len(claimed))
		for _, id := range claimed {
			if _, ok := seen[id]; ok {
				t.Fatalf("duplicate claim for row %d in %v", id, claimed)
			}
			seen[id] = struct{}{}
		}
		if len(claimed) != 15 {
			t.Fatalf("claimed rows = %d, want 15", len(claimed))
		}
	})

	t.Run("Mark transitions", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewOutboxRepository(db)

		sentID := insertOutboxRow(t, db, outboxSeed{Recipient: "sent@example.com"})
		if err := repo.MarkSent(ctx, sentID); err != nil {
			t.Fatalf("mark sent: %v", err)
		}
		assertOutboxState(t, db, sentID, "sent", 0, "", true, false)

		retryID := insertOutboxRow(t, db, outboxSeed{Recipient: "retry@example.com"})
		nextAttempt := time.Now().Add(10 * time.Minute).UTC().Truncate(time.Microsecond)
		if err := repo.MarkRetryable(ctx, retryID, "temporary", nextAttempt); err != nil {
			t.Fatalf("mark retryable: %v", err)
		}
		assertOutboxState(t, db, retryID, "pending", 1, "temporary", false, false)

		failedID := insertOutboxRow(t, db, outboxSeed{Recipient: "failed@example.com", RetryCount: 7})
		if err := repo.MarkFailed(ctx, failedID, "permanent"); err != nil {
			t.Fatalf("mark failed: %v", err)
		}
		assertOutboxState(t, db, failedID, "failed", 8, "permanent", false, true)
	})
}

type outboxSeed struct {
	Recipient     string
	Status        string
	RetryCount    int
	NextAttemptAt time.Time
}

func insertOutboxRow(t testing.TB, db *sql.DB, seed outboxSeed) int64 {
	t.Helper()

	if seed.Recipient == "" {
		seed.Recipient = fmt.Sprintf("outbox-%d@example.com", uniqueSuffix())
	}
	if seed.Status == "" {
		seed.Status = "pending"
	}
	if seed.NextAttemptAt.IsZero() {
		seed.NextAttemptAt = time.Now().Add(-time.Minute)
	}

	var id int64
	if err := db.QueryRowContext(
		context.Background(),
		`
			INSERT INTO email_outbox (
				recipient,
				subject,
				body,
				status,
				retry_count,
				next_attempt_at
			)
			VALUES ($1, 'Subject', 'Body', $2, $3, $4)
			RETURNING id;
		`,
		seed.Recipient,
		seed.Status,
		seed.RetryCount,
		seed.NextAttemptAt,
	).Scan(&id); err != nil {
		t.Fatalf("insert outbox row: %v", err)
	}
	return id
}

func countOutboxRows(t testing.TB, db *sql.DB) int {
	t.Helper()

	var count int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM email_outbox;`).Scan(&count); err != nil {
		t.Fatalf("count outbox rows: %v", err)
	}
	return count
}

func outboxRowHasFutureAttempt(t testing.TB, db *sql.DB, id int64) bool {
	t.Helper()

	var future bool
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT next_attempt_at > NOW() FROM email_outbox WHERE id = $1;`,
		id,
	).Scan(&future); err != nil {
		t.Fatalf("read outbox next_attempt_at: %v", err)
	}
	return future
}

func assertOutboxState(
	t testing.TB,
	db *sql.DB,
	id int64,
	wantStatus string,
	wantRetryCount int,
	wantLastError string,
	wantSentAt bool,
	wantFailedAt bool,
) {
	t.Helper()

	var (
		status     string
		retryCount int
		lastError  sql.NullString
		sentAt     sql.NullTime
		failedAt   sql.NullTime
	)
	if err := db.QueryRowContext(
		context.Background(),
		`
			SELECT status, retry_count, last_error, sent_at, failed_at
			FROM email_outbox
			WHERE id = $1;
		`,
		id,
	).Scan(&status, &retryCount, &lastError, &sentAt, &failedAt); err != nil {
		t.Fatalf("read outbox state: %v", err)
	}

	if status != wantStatus {
		t.Fatalf("status = %q, want %q", status, wantStatus)
	}
	if retryCount != wantRetryCount {
		t.Fatalf("retry_count = %d, want %d", retryCount, wantRetryCount)
	}
	if gotLastError := lastError.String; gotLastError != wantLastError {
		t.Fatalf("last_error = %q, want %q", gotLastError, wantLastError)
	}
	if sentAt.Valid != wantSentAt {
		t.Fatalf("sent_at valid = %v, want %v", sentAt.Valid, wantSentAt)
	}
	if failedAt.Valid != wantFailedAt {
		t.Fatalf("failed_at valid = %v, want %v", failedAt.Valid, wantFailedAt)
	}
}
