//go:build integration

package repository

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

func TestPreventDuplicateActiveLeavesMigrationIntegration(t *testing.T) {
	ctx := context.Background()
	db := openIntegrationDB(t)
	publication, requester, assignment := seedLeavePrerequisites(t, db)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin migration fixture transaction: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`DROP INDEX shift_change_requests_active_leave_occurrence_uidx;`,
	); err != nil {
		t.Fatalf("drop active-leave index for legacy fixture: %v", err)
	}

	insertWorkflow := func(
		state model.ShiftChangeState,
		createdAt time.Time,
	) int64 {
		t.Helper()

		var decidedAt any
		if state == model.ShiftChangeStateApproved {
			decidedAt = createdAt.Add(time.Minute)
		}

		var requestID int64
		if err := tx.QueryRowContext(
			ctx,
			`INSERT INTO shift_change_requests (
				publication_id,
				type,
				requester_user_id,
				requester_assignment_id,
				occurrence_date,
				state,
				created_at,
				decided_at,
				expires_at
			)
			VALUES ($1, 'give_pool', $2, $3, $4, $5, $6, $7, $8)
			RETURNING id;`,
			publication.ID,
			requester.ID,
			assignment.ID,
			time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC),
			state,
			createdAt,
			decidedAt,
			createdAt.AddDate(0, 0, 30),
		).Scan(&requestID); err != nil {
			t.Fatalf("insert legacy request: %v", err)
		}

		var leaveID int64
		if err := tx.QueryRowContext(
			ctx,
			`INSERT INTO leaves (
				user_id,
				publication_id,
				shift_change_request_id,
				category,
				created_at,
				updated_at
			)
			VALUES ($1, $2, $3, 'personal', $4, $4)
			RETURNING id;`,
			requester.ID,
			publication.ID,
			requestID,
			createdAt,
		).Scan(&leaveID); err != nil {
			t.Fatalf("insert legacy leave: %v", err)
		}
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE shift_change_requests SET leave_id = $1 WHERE id = $2;`,
			leaveID,
			requestID,
		); err != nil {
			t.Fatalf("link legacy leave: %v", err)
		}
		return requestID
	}

	pendingID := insertWorkflow(
		model.ShiftChangeStatePending,
		time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC),
	)
	approvedID := insertWorkflow(
		model.ShiftChangeStateApproved,
		time.Date(2026, 4, 2, 8, 0, 0, 0, time.UTC),
	)

	if _, err := tx.ExecContext(ctx, activeLeaveMigrationUpSQL(t)); err != nil {
		t.Fatalf("apply active-leave migration: %v", err)
	}

	var pendingState model.ShiftChangeState
	var pendingDecided bool
	if err := tx.QueryRowContext(
		ctx,
		`SELECT state, decided_at IS NOT NULL
		 FROM shift_change_requests
		 WHERE id = $1;`,
		pendingID,
	).Scan(&pendingState, &pendingDecided); err != nil {
		t.Fatalf("read normalized pending request: %v", err)
	}
	if pendingState != model.ShiftChangeStateInvalidated || !pendingDecided {
		t.Fatalf(
			"expected older pending request invalidated with decided_at, state=%s decided=%t",
			pendingState,
			pendingDecided,
		)
	}

	var approvedState model.ShiftChangeState
	if err := tx.QueryRowContext(
		ctx,
		`SELECT state FROM shift_change_requests WHERE id = $1;`,
		approvedID,
	).Scan(&approvedState); err != nil {
		t.Fatalf("read preserved approved request: %v", err)
	}
	if approvedState != model.ShiftChangeStateApproved {
		t.Fatalf("expected approved request preserved, got %s", approvedState)
	}

	var activeCount int
	if err := tx.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		 FROM shift_change_requests
		 WHERE requester_user_id = $1
		   AND requester_assignment_id = $2
		   AND occurrence_date = $3
		   AND leave_id IS NOT NULL
		   AND state IN ('pending', 'approved');`,
		requester.ID,
		assignment.ID,
		time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC),
	).Scan(&activeCount); err != nil {
		t.Fatalf("count normalized active requests: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected one active request after migration, got %d", activeCount)
	}

	var indexExists bool
	if err := tx.QueryRowContext(
		ctx,
		`SELECT EXISTS (
			SELECT 1
			FROM pg_indexes
			WHERE schemaname = 'public'
			  AND indexname = 'shift_change_requests_active_leave_occurrence_uidx'
		);`,
	).Scan(&indexExists); err != nil {
		t.Fatalf("inspect active-leave index: %v", err)
	}
	if !indexExists {
		t.Fatal("expected active-leave partial unique index")
	}
}

func activeLeaveMigrationUpSQL(t testing.TB) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration test path")
	}
	path := filepath.Join(
		filepath.Dir(filename),
		"..",
		"..",
		"..",
		"migrations",
		"00022_prevent_duplicate_active_leaves.sql",
	)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read active-leave migration: %v", err)
	}
	up, _, found := strings.Cut(string(content), "-- +goose Down")
	if !found {
		t.Fatal("active-leave migration is missing a Down section")
	}
	return up
}
