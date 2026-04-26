//go:build integration

package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
	_ "github.com/lib/pq"
)

var (
	serviceIntegrationDBOnce sync.Once
	serviceIntegrationDB     *sql.DB
	serviceIntegrationDBErr  error
)

const serviceIntegrationDBLockKey int64 = 2026042301

func TestPublicationServiceAutoAssignPublicationIntegration(t *testing.T) {
	db := openServiceIntegrationDB(t)
	now := serviceIntegrationTestTime()

	templateID := seedServiceTemplate(t, db, "Auto Assign Integration Template")
	publicationID := seedServicePublication(t, db, templateID, now)
	positionA := seedServicePosition(t, db, "Front Desk")
	positionB := seedServicePosition(t, db, "Cashier")
	slotOneID := seedServiceSlot(t, db, templateID, 1, "09:00", "12:00")
	slotTwoID := seedServiceSlot(t, db, templateID, 1, "13:00", "17:00")
	seedServiceSlotPosition(t, db, slotOneID, positionA, 1)
	seedServiceSlotPosition(t, db, slotOneID, positionB, 1)
	seedServiceSlotPosition(t, db, slotTwoID, positionA, 1)

	flexibleUserID := seedServiceUser(t, db, "flex@example.com", "Flexible")
	overlapOnlyUserID := seedServiceUser(t, db, "overlap@example.com", "Overlap Only")
	slotOnlyUserID := seedServiceUser(t, db, "slot@example.com", "Slot Only")

	seedServiceUserPosition(t, db, flexibleUserID, positionA)
	seedServiceUserPosition(t, db, flexibleUserID, positionB)
	seedServiceUserPosition(t, db, overlapOnlyUserID, positionA)
	seedServiceUserPosition(t, db, slotOnlyUserID, positionA)

	seedServiceSubmission(t, db, publicationID, flexibleUserID, slotOneID, positionA, now.Add(-4*time.Minute))
	seedServiceSubmission(t, db, publicationID, flexibleUserID, slotOneID, positionB, now.Add(-3*time.Minute))
	seedServiceSubmission(t, db, publicationID, flexibleUserID, slotTwoID, positionA, now.Add(-2*time.Minute))
	seedServiceSubmission(t, db, publicationID, overlapOnlyUserID, slotTwoID, positionA, now.Add(-time.Minute))
	seedServiceSubmission(t, db, publicationID, slotOnlyUserID, slotOneID, positionA, now)

	repo := repository.NewPublicationRepository(db)
	service := NewPublicationService(repo, fixedClock{now: now})

	result, err := service.AutoAssignPublication(context.Background(), publicationID)
	if err != nil {
		t.Fatalf("AutoAssignPublication returned error: %v", err)
	}
	if len(result.Slots) != 2 {
		t.Fatalf("expected 2 board slots, got %+v", result.Slots)
	}

	assignments, err := repo.ListPublicationAssignments(context.Background(), publicationID)
	if err != nil {
		t.Fatalf("ListPublicationAssignments returned error: %v", err)
	}
	if len(assignments) != 3 {
		t.Fatalf("expected 3 persisted assignments, got %+v", assignments)
	}

	seenUserSlots := make(map[string]struct{}, len(assignments))
	slotPositionCounts := make(map[string]int, len(assignments))
	for _, assignment := range assignments {
		userSlotKey := fmt.Sprintf("%d:%d", assignment.UserID, assignment.SlotID)
		if _, exists := seenUserSlots[userSlotKey]; exists {
			t.Fatalf("expected one assignment per user-slot, got %+v", assignments)
		}
		seenUserSlots[userSlotKey] = struct{}{}

		slotPositionKey := fmt.Sprintf("%d:%d", assignment.SlotID, assignment.PositionID)
		slotPositionCounts[slotPositionKey]++
	}

	for _, key := range []string{
		fmt.Sprintf("%d:%d", slotOneID, positionA),
		fmt.Sprintf("%d:%d", slotOneID, positionB),
		fmt.Sprintf("%d:%d", slotTwoID, positionA),
	} {
		if slotPositionCounts[key] != 1 {
			t.Fatalf("expected slot-position %s to be staffed exactly once, got %+v", key, slotPositionCounts)
		}
	}

	for _, userID := range []int64{flexibleUserID, overlapOnlyUserID, slotOnlyUserID} {
		userAssignments, err := listServiceUserAssignmentWindows(db, publicationID, userID, 1)
		if err != nil {
			t.Fatalf("list user assignment windows returned error: %v", err)
		}

		for i := 0; i < len(userAssignments); i++ {
			for j := i + 1; j < len(userAssignments); j++ {
				left := userAssignments[i]
				right := userAssignments[j]
				if left.StartTime < right.EndTime && right.StartTime < left.EndTime {
					t.Fatalf("expected auto-assign result to avoid overlapping weekday assignments, got %+v", userAssignments)
				}
			}
		}
	}
}

type serviceAssignmentWindow struct {
	StartTime string
	EndTime   string
}

func listServiceUserAssignmentWindows(
	db *sql.DB,
	publicationID, userID int64,
	weekday int,
) ([]serviceAssignmentWindow, error) {
	rows, err := db.QueryContext(
		context.Background(),
		`
			SELECT TO_CHAR(ts.start_time, 'HH24:MI'), TO_CHAR(ts.end_time, 'HH24:MI')
			FROM assignments a
			INNER JOIN template_slots ts ON ts.id = a.slot_id
			WHERE a.publication_id = $1
				AND a.user_id = $2
				AND ts.weekday = $3
			ORDER BY ts.start_time ASC, ts.id ASC, a.position_id ASC;
		`,
		publicationID,
		userID,
		weekday,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	windows := make([]serviceAssignmentWindow, 0)
	for rows.Next() {
		var window serviceAssignmentWindow
		if err := rows.Scan(&window.StartTime, &window.EndTime); err != nil {
			return nil, err
		}
		windows = append(windows, window)
	}
	return windows, rows.Err()
}

func openServiceIntegrationDB(t testing.TB) *sql.DB {
	t.Helper()

	serviceIntegrationDBOnce.Do(func() {
		db, err := sql.Open("postgres", serviceIntegrationDatabaseURL())
		if err != nil {
			serviceIntegrationDBErr = err
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			_ = db.Close()
			serviceIntegrationDBErr = err
			return
		}

		serviceIntegrationDB = db
	})

	if serviceIntegrationDBErr != nil {
		t.Skipf("skipping integration test: %v", serviceIntegrationDBErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lockConn, err := acquireServiceIntegrationDBLock(ctx, serviceIntegrationDB)
	if err != nil {
		t.Fatalf("acquire integration database lock: %v", err)
	}

	if err := resetServiceIntegrationDB(ctx, serviceIntegrationDB); err != nil {
		_ = releaseServiceIntegrationDBLock(context.Background(), lockConn)
		t.Fatalf("reset integration database before test: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := resetServiceIntegrationDB(ctx, serviceIntegrationDB); err != nil {
			t.Fatalf("reset integration database after test: %v", err)
		}
		if err := releaseServiceIntegrationDBLock(ctx, lockConn); err != nil {
			t.Fatalf("release integration database lock: %v", err)
		}
	})

	return serviceIntegrationDB
}

func acquireServiceIntegrationDBLock(ctx context.Context, db *sql.DB) (*sql.Conn, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}

	var locked bool
	if err := conn.QueryRowContext(
		ctx,
		`SELECT pg_advisory_lock($1) IS NOT NULL;`,
		serviceIntegrationDBLockKey,
	).Scan(&locked); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return conn, nil
}

func releaseServiceIntegrationDBLock(ctx context.Context, conn *sql.Conn) error {
	if conn == nil {
		return nil
	}

	var unlocked bool
	err := conn.QueryRowContext(
		ctx,
		`SELECT pg_advisory_unlock($1);`,
		serviceIntegrationDBLockKey,
	).Scan(&unlocked)
	closeErr := conn.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func resetServiceIntegrationDB(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public';
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	existing := make(map[string]struct{})
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return err
		}
		existing[table] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	ordered := []string{
		"audit_logs",
		"shift_change_requests",
		"assignments",
		"availability_submissions",
		"template_slot_positions",
		"template_slots",
		"user_setup_tokens",
		"publications",
		"user_positions",
		"templates",
		"positions",
		"users",
	}

	tables := make([]string, 0, len(ordered))
	for _, table := range ordered {
		if _, ok := existing[table]; ok {
			tables = append(tables, table)
		}
	}
	if len(tables) == 0 {
		return nil
	}

	_, err = db.ExecContext(ctx, "TRUNCATE TABLE "+strings.Join(tables, ", ")+" RESTART IDENTITY CASCADE;")
	return err
}

func serviceIntegrationDatabaseURL() string {
	if value := strings.TrimSpace(os.Getenv("DATABASE_URL")); value != "" {
		return value
	}

	host := serviceEnvOrDefault("POSTGRES_HOST", "localhost")
	port := serviceEnvOrDefault("POSTGRES_PORT", "5432")
	user := serviceEnvOrDefault("POSTGRES_USER", "rota")
	password := serviceEnvOrDefault("POSTGRES_PASSWORD", "pa55word")
	database := serviceEnvOrDefault("POSTGRES_DB", "rota")

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user,
		password,
		host,
		port,
		database,
	)
}

func serviceEnvOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func serviceIntegrationTestTime() time.Time {
	return time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC)
}

func seedServiceUser(t testing.TB, db *sql.DB, email, name string) int64 {
	t.Helper()

	var id int64
	if err := db.QueryRowContext(
		context.Background(),
		`
			INSERT INTO users (email, password_hash, name, is_admin, status, version)
			VALUES ($1, $2, $3, false, $4, 1)
			RETURNING id;
		`,
		email,
		"hash",
		name,
		model.UserStatusActive,
	).Scan(&id); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func seedServicePosition(t testing.TB, db *sql.DB, name string) int64 {
	t.Helper()

	var id int64
	if err := db.QueryRowContext(
		context.Background(),
		`
			INSERT INTO positions (name, description)
			VALUES ($1, $2)
			RETURNING id;
		`,
		name,
		name+" description",
	).Scan(&id); err != nil {
		t.Fatalf("seed position: %v", err)
	}
	return id
}

func seedServiceTemplate(t testing.TB, db *sql.DB, name string) int64 {
	t.Helper()

	var id int64
	now := serviceIntegrationTestTime()
	if err := db.QueryRowContext(
		context.Background(),
		`
			INSERT INTO templates (name, description, is_locked, created_at, updated_at)
			VALUES ($1, $2, false, $3, $3)
			RETURNING id;
		`,
		name,
		name+" description",
		now,
	).Scan(&id); err != nil {
		t.Fatalf("seed template: %v", err)
	}
	return id
}

func seedServiceSlot(
	t testing.TB,
	db *sql.DB,
	templateID int64,
	weekday int,
	startTime string,
	endTime string,
) int64 {
	t.Helper()

	var id int64
	now := serviceIntegrationTestTime()
	if err := db.QueryRowContext(
		context.Background(),
		`
			INSERT INTO template_slots (template_id, weekday, start_time, end_time, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5)
			RETURNING id;
		`,
		templateID,
		weekday,
		startTime,
		endTime,
		now,
	).Scan(&id); err != nil {
		t.Fatalf("seed slot: %v", err)
	}
	return id
}

func seedServiceSlotPosition(
	t testing.TB,
	db *sql.DB,
	slotID, positionID int64,
	requiredHeadcount int,
) int64 {
	t.Helper()

	var id int64
	now := serviceIntegrationTestTime()
	if err := db.QueryRowContext(
		context.Background(),
		`
			INSERT INTO template_slot_positions (slot_id, position_id, required_headcount, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $4)
			RETURNING id;
		`,
		slotID,
		positionID,
		requiredHeadcount,
		now,
	).Scan(&id); err != nil {
		t.Fatalf("seed slot position: %v", err)
	}
	return id
}

func seedServicePublication(t testing.TB, db *sql.DB, templateID int64, now time.Time) int64 {
	t.Helper()

	var id int64
	if err := db.QueryRowContext(
		context.Background(),
		`
			INSERT INTO publications (
				template_id,
				name,
				state,
				submission_start_at,
				submission_end_at,
				planned_active_from,
				planned_active_until,
				created_at,
				updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
			RETURNING id;
		`,
		templateID,
		"Auto Assign Integration Publication",
		model.PublicationStateAssigning,
		now.Add(-2*time.Hour),
		now.Add(-time.Hour),
		now.Add(time.Hour),
		now.Add(8*7*24*time.Hour),
		now,
	).Scan(&id); err != nil {
		t.Fatalf("seed publication: %v", err)
	}
	return id
}

func seedServiceUserPosition(t testing.TB, db *sql.DB, userID, positionID int64) {
	t.Helper()

	if _, err := db.ExecContext(
		context.Background(),
		`INSERT INTO user_positions (user_id, position_id) VALUES ($1, $2);`,
		userID,
		positionID,
	); err != nil {
		t.Fatalf("seed user position: %v", err)
	}
}

func seedServiceSubmission(
	t testing.TB,
	db *sql.DB,
	publicationID, userID, slotID, positionID int64,
	createdAt time.Time,
) {
	t.Helper()

	if _, err := db.ExecContext(
		context.Background(),
		`
			INSERT INTO availability_submissions (
				publication_id,
				user_id,
				slot_id,
				position_id,
				created_at
			)
			VALUES ($1, $2, $3, $4, $5);
		`,
		publicationID,
		userID,
		slotID,
		positionID,
		createdAt,
	); err != nil {
		t.Fatalf("seed submission: %v", err)
	}
}
