//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	_ "github.com/lib/pq"
)

var (
	integrationDBOnce     sync.Once
	integrationDB         *sql.DB
	integrationDBSkipErr  error
	integrationDBSetupErr error
	integrationSeedCount  atomic.Int64
)

const integrationDBLockKey int64 = 2026042301

func openIntegrationDB(t testing.TB) *sql.DB {
	t.Helper()

	integrationDBOnce.Do(func() {
		db, err := sql.Open("postgres", integrationDatabaseURL())
		if err != nil {
			integrationDBSkipErr = err
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			_ = db.Close()
			integrationDBSkipErr = err
			return
		}

		integrationDB = db
	})

	if integrationDBSkipErr != nil {
		t.Skipf("skipping integration test: %v", integrationDBSkipErr)
	}
	if integrationDBSetupErr != nil {
		t.Fatalf("integration database is reachable but not ready: %v", integrationDBSetupErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lockConn, err := acquireIntegrationDBLock(ctx, integrationDB)
	if err != nil {
		t.Fatalf("acquire integration database lock: %v", err)
	}

	if err := resetIntegrationDB(ctx, integrationDB); err != nil {
		_ = releaseIntegrationDBLock(context.Background(), lockConn)
		t.Fatalf("reset integration database before test: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := resetIntegrationDB(ctx, integrationDB); err != nil {
			t.Fatalf("reset integration database after test: %v", err)
		}
		if err := releaseIntegrationDBLock(ctx, lockConn); err != nil {
			t.Fatalf("release integration database lock: %v", err)
		}
	})

	return integrationDB
}

func acquireIntegrationDBLock(ctx context.Context, db *sql.DB) (*sql.Conn, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}

	var locked bool
	if err := conn.QueryRowContext(
		ctx,
		`SELECT pg_advisory_lock($1) IS NOT NULL;`,
		integrationDBLockKey,
	).Scan(&locked); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return conn, nil
}

func releaseIntegrationDBLock(ctx context.Context, conn *sql.Conn) error {
	if conn == nil {
		return nil
	}

	var unlocked bool
	err := conn.QueryRowContext(
		ctx,
		`SELECT pg_advisory_unlock($1);`,
		integrationDBLockKey,
	).Scan(&unlocked)
	closeErr := conn.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func resetIntegrationDB(ctx context.Context, db *sql.DB) error {
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
		"email_outbox",
		"sessions",
		"leaves",
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

	query := "TRUNCATE TABLE " + strings.Join(tables, ", ") + " RESTART IDENTITY CASCADE;"
	_, err = db.ExecContext(ctx, query)
	return err
}

func integrationDatabaseURL() string {
	if value := strings.TrimSpace(os.Getenv("DATABASE_URL")); value != "" {
		return value
	}

	host := envOrDefault("POSTGRES_HOST", "localhost")
	port := envOrDefault("POSTGRES_PORT", "5432")
	user := envOrDefault("POSTGRES_USER", "rota")
	password := envOrDefault("POSTGRES_PASSWORD", "pa55word")
	database := envOrDefault("POSTGRES_DB", "rota")

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user,
		password,
		host,
		port,
		database,
	)
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func testTime() time.Time {
	return time.Date(2026, 4, 17, 9, 0, 0, 0, time.UTC)
}

func uniqueSuffix() int64 {
	return integrationSeedCount.Add(1)
}

type userSeed struct {
	Email        string
	PasswordHash string
	Name         string
	IsAdmin      bool
	Status       model.UserStatus
	Version      int
}

func seedUser(t testing.TB, db *sql.DB, seed userSeed) *model.User {
	t.Helper()

	suffix := uniqueSuffix()
	if seed.Email == "" {
		seed.Email = fmt.Sprintf("worker-%d@example.com", suffix)
	}
	if seed.PasswordHash == "" {
		seed.PasswordHash = fmt.Sprintf("hash-%d", suffix)
	}
	if seed.Name == "" {
		seed.Name = fmt.Sprintf("Worker %d", suffix)
	}
	if seed.Status == "" {
		seed.Status = model.UserStatusActive
	}
	if seed.Version == 0 {
		seed.Version = 1
	}

	const query = `
		INSERT INTO users (email, password_hash, name, is_admin, status, version)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, email, password_hash, name, is_admin, status, version;
	`

	user := &model.User{}
	if err := db.QueryRowContext(
		context.Background(),
		query,
		seed.Email,
		seed.PasswordHash,
		seed.Name,
		seed.IsAdmin,
		seed.Status,
		seed.Version,
	).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
		&user.IsAdmin,
		&user.Status,
		&user.Version,
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	return user
}

type positionSeed struct {
	Name        string
	Description string
}

func seedPosition(t testing.TB, db *sql.DB, seed positionSeed) *model.Position {
	t.Helper()

	suffix := uniqueSuffix()
	if seed.Name == "" {
		seed.Name = fmt.Sprintf("Position %d", suffix)
	}
	if seed.Description == "" {
		seed.Description = fmt.Sprintf("Description %d", suffix)
	}

	const query = `
		INSERT INTO positions (name, description)
		VALUES ($1, $2)
		RETURNING id, name, description, created_at, updated_at;
	`

	position := &model.Position{}
	if err := db.QueryRowContext(
		context.Background(),
		query,
		seed.Name,
		seed.Description,
	).Scan(
		&position.ID,
		&position.Name,
		&position.Description,
		&position.CreatedAt,
		&position.UpdatedAt,
	); err != nil {
		t.Fatalf("seed position: %v", err)
	}

	return position
}

type templateSeed struct {
	Name        string
	Description string
	IsLocked    bool
}

func seedTemplate(t testing.TB, db *sql.DB, seed templateSeed) *model.Template {
	t.Helper()

	suffix := uniqueSuffix()
	if seed.Name == "" {
		seed.Name = fmt.Sprintf("Template %d", suffix)
	}
	if seed.Description == "" {
		seed.Description = fmt.Sprintf("Description %d", suffix)
	}

	const query = `
		INSERT INTO templates (name, description, is_locked, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		RETURNING id, name, description, is_locked, created_at, updated_at;
	`

	template := &model.Template{}
	if err := db.QueryRowContext(
		context.Background(),
		query,
		seed.Name,
		seed.Description,
		seed.IsLocked,
		testTime(),
	).Scan(
		&template.ID,
		&template.Name,
		&template.Description,
		&template.IsLocked,
		&template.CreatedAt,
		&template.UpdatedAt,
	); err != nil {
		t.Fatalf("seed template: %v", err)
	}

	return template
}

type qualifiedShiftSeed struct {
	TemplateID        int64
	Weekday           int
	StartTime         string
	EndTime           string
	PositionID        int64
	RequiredHeadcount int
}

type seededSlotPosition struct {
	EntryID           int64
	SlotID            int64
	TemplateID        int64
	Weekday           int
	StartTime         string
	EndTime           string
	PositionID        int64
	RequiredHeadcount int
}

func seedQualifiedShift(t testing.TB, db *sql.DB, seed qualifiedShiftSeed) *seededSlotPosition {
	t.Helper()

	if seed.Weekday == 0 {
		seed.Weekday = 1
	}
	if seed.StartTime == "" {
		seed.StartTime = "09:00"
	}
	if seed.EndTime == "" {
		seed.EndTime = "12:00"
	}
	if seed.RequiredHeadcount == 0 {
		seed.RequiredHeadcount = 1
	}

	slotID := seedTemplateSlot(t, db, seed.TemplateID, seed.Weekday, seed.StartTime, seed.EndTime)
	entryID := seedTemplateSlotPosition(t, db, slotID, seed.PositionID, seed.RequiredHeadcount)

	return &seededSlotPosition{
		EntryID:           entryID,
		SlotID:            slotID,
		TemplateID:        seed.TemplateID,
		Weekday:           seed.Weekday,
		StartTime:         seed.StartTime,
		EndTime:           seed.EndTime,
		PositionID:        seed.PositionID,
		RequiredHeadcount: seed.RequiredHeadcount,
	}
}

type publicationSeed struct {
	TemplateID         int64
	Name               string
	State              model.PublicationState
	SubmissionStartAt  time.Time
	SubmissionEndAt    time.Time
	PlannedActiveFrom  time.Time
	PlannedActiveUntil time.Time
	CreatedAt          time.Time
}

func seedPublication(t testing.TB, db *sql.DB, seed publicationSeed) *model.Publication {
	t.Helper()

	suffix := uniqueSuffix()
	base := testTime()
	if seed.Name == "" {
		seed.Name = fmt.Sprintf("Publication %d", suffix)
	}
	if seed.State == "" {
		seed.State = model.PublicationStateDraft
	}
	if seed.CreatedAt.IsZero() {
		seed.CreatedAt = base
	}
	if seed.SubmissionStartAt.IsZero() {
		seed.SubmissionStartAt = base.Add(1 * time.Hour)
	}
	if seed.SubmissionEndAt.IsZero() {
		seed.SubmissionEndAt = base.Add(2 * time.Hour)
	}
	if seed.PlannedActiveFrom.IsZero() {
		seed.PlannedActiveFrom = base.Add(3 * time.Hour)
	}
	if seed.PlannedActiveUntil.IsZero() {
		seed.PlannedActiveUntil = seed.PlannedActiveFrom.Add(8 * 7 * 24 * time.Hour)
	}

	const query = `
		INSERT INTO publications (
			template_id,
			name,
			description,
			state,
			submission_start_at,
			submission_end_at,
			planned_active_from,
			planned_active_until,
			created_at,
			updated_at
		)
		VALUES ($1, $2, '', $3, $4, $5, $6, $7, $8, $8)
		RETURNING
			id,
			template_id,
			$9 AS template_name,
			name,
			description,
			state,
			submission_start_at,
			submission_end_at,
			planned_active_from,
			planned_active_until,
			activated_at,
			created_at,
			updated_at;
	`

	var templateName string
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT name FROM templates WHERE id = $1;`,
		seed.TemplateID,
	).Scan(&templateName); err != nil {
		t.Fatalf("load template name for publication seed: %v", err)
	}

	publication := &model.Publication{}
	if err := db.QueryRowContext(
		context.Background(),
		query,
		seed.TemplateID,
		seed.Name,
		seed.State,
		seed.SubmissionStartAt,
		seed.SubmissionEndAt,
		seed.PlannedActiveFrom,
		seed.PlannedActiveUntil,
		seed.CreatedAt,
		templateName,
	).Scan(
		&publication.ID,
		&publication.TemplateID,
		&publication.TemplateName,
		&publication.Name,
		&publication.Description,
		&publication.State,
		&publication.SubmissionStartAt,
		&publication.SubmissionEndAt,
		&publication.PlannedActiveFrom,
		&publication.PlannedActiveUntil,
		&publication.ActivatedAt,
		&publication.CreatedAt,
		&publication.UpdatedAt,
	); err != nil {
		t.Fatalf("seed publication: %v", err)
	}

	return publication
}

func seedUserPosition(t testing.TB, db *sql.DB, userID, positionID int64) {
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

func seedSubmission(
	t testing.TB,
	db *sql.DB,
	publicationID, userID, slotID int64,
	createdAt time.Time,
) *model.AvailabilitySubmission {
	t.Helper()

	const query = `
		INSERT INTO availability_submissions (
			publication_id,
			user_id,
			slot_id,
			created_at
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id, publication_id, user_id, slot_id, created_at;
	`

	submission := &model.AvailabilitySubmission{}
	if err := db.QueryRowContext(
		context.Background(),
		query,
		publicationID,
		userID,
		slotID,
		createdAt,
	).Scan(
		&submission.ID,
		&submission.PublicationID,
		&submission.UserID,
		&submission.SlotID,
		&submission.CreatedAt,
	); err != nil {
		t.Fatalf("seed submission: %v", err)
	}

	return submission
}

func seedAssignment(
	t testing.TB,
	db *sql.DB,
	publicationID, userID, slotID, positionID int64,
	createdAt time.Time,
) *model.Assignment {
	t.Helper()

	const query = `
		INSERT INTO assignments (
			publication_id,
			user_id,
			slot_id,
			position_id,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, publication_id, user_id, slot_id, position_id, created_at;
	`

	assignment := &model.Assignment{}
	if err := db.QueryRowContext(
		context.Background(),
		query,
		publicationID,
		userID,
		slotID,
		positionID,
		createdAt,
	).Scan(
		&assignment.ID,
		&assignment.PublicationID,
		&assignment.UserID,
		&assignment.SlotID,
		&assignment.PositionID,
		&assignment.CreatedAt,
	); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}

	return assignment
}
