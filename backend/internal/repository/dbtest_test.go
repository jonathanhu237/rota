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

		if err := resetIntegrationDB(ctx, db); err != nil {
			_ = db.Close()
			integrationDBSetupErr = fmt.Errorf("reset integration database: %w", err)
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

	if err := resetIntegrationDB(ctx, integrationDB); err != nil {
		t.Fatalf("reset integration database before test: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := resetIntegrationDB(ctx, integrationDB); err != nil {
			t.Fatalf("reset integration database after test: %v", err)
		}
	})

	return integrationDB
}

func resetIntegrationDB(ctx context.Context, db *sql.DB) error {
	const query = `
		TRUNCATE TABLE
			audit_logs,
			assignments,
			availability_submissions,
			publications,
			template_shifts,
			user_setup_tokens,
			user_positions,
			templates,
			positions,
			users
		RESTART IDENTITY CASCADE;
	`

	_, err := db.ExecContext(ctx, query)
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

type templateShiftSeed struct {
	TemplateID        int64
	Weekday           int
	StartTime         string
	EndTime           string
	PositionID        int64
	RequiredHeadcount int
}

func seedTemplateShift(t testing.TB, db *sql.DB, seed templateShiftSeed) *model.TemplateShift {
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

	const query = `
		INSERT INTO template_shifts (
			template_id,
			weekday,
			start_time,
			end_time,
			position_id,
			required_headcount,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		RETURNING
			id,
			template_id,
			weekday,
			TO_CHAR(start_time, 'HH24:MI'),
			TO_CHAR(end_time, 'HH24:MI'),
			position_id,
			required_headcount,
			created_at,
			updated_at;
	`

	shift := &model.TemplateShift{}
	if err := db.QueryRowContext(
		context.Background(),
		query,
		seed.TemplateID,
		seed.Weekday,
		seed.StartTime,
		seed.EndTime,
		seed.PositionID,
		seed.RequiredHeadcount,
		testTime(),
	).Scan(
		&shift.ID,
		&shift.TemplateID,
		&shift.Weekday,
		&shift.StartTime,
		&shift.EndTime,
		&shift.PositionID,
		&shift.RequiredHeadcount,
		&shift.CreatedAt,
		&shift.UpdatedAt,
	); err != nil {
		t.Fatalf("seed template shift: %v", err)
	}

	return shift
}

type publicationSeed struct {
	TemplateID        int64
	Name              string
	State             model.PublicationState
	SubmissionStartAt time.Time
	SubmissionEndAt   time.Time
	PlannedActiveFrom time.Time
	CreatedAt         time.Time
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

	const query = `
		INSERT INTO publications (
			template_id,
			name,
			state,
			submission_start_at,
			submission_end_at,
			planned_active_from,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		RETURNING
			id,
			template_id,
			$8 AS template_name,
			name,
			state,
			submission_start_at,
			submission_end_at,
			planned_active_from,
			activated_at,
			ended_at,
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
		seed.CreatedAt,
		templateName,
	).Scan(
		&publication.ID,
		&publication.TemplateID,
		&publication.TemplateName,
		&publication.Name,
		&publication.State,
		&publication.SubmissionStartAt,
		&publication.SubmissionEndAt,
		&publication.PlannedActiveFrom,
		&publication.ActivatedAt,
		&publication.EndedAt,
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
	publicationID, userID, templateShiftID int64,
	createdAt time.Time,
) *model.AvailabilitySubmission {
	t.Helper()

	const query = `
		INSERT INTO availability_submissions (
			publication_id,
			user_id,
			template_shift_id,
			created_at
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id, publication_id, user_id, template_shift_id, created_at;
	`

	submission := &model.AvailabilitySubmission{}
	if err := db.QueryRowContext(
		context.Background(),
		query,
		publicationID,
		userID,
		templateShiftID,
		createdAt,
	).Scan(
		&submission.ID,
		&submission.PublicationID,
		&submission.UserID,
		&submission.TemplateShiftID,
		&submission.CreatedAt,
	); err != nil {
		t.Fatalf("seed submission: %v", err)
	}

	return submission
}

func seedAssignment(
	t testing.TB,
	db *sql.DB,
	publicationID, userID, templateShiftID int64,
	createdAt time.Time,
) *model.Assignment {
	t.Helper()

	const query = `
		INSERT INTO assignments (
			publication_id,
			user_id,
			template_shift_id,
			created_at
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id, publication_id, user_id, template_shift_id, created_at;
	`

	assignment := &model.Assignment{}
	if err := db.QueryRowContext(
		context.Background(),
		query,
		publicationID,
		userID,
		templateShiftID,
		createdAt,
	).Scan(
		&assignment.ID,
		&assignment.PublicationID,
		&assignment.UserID,
		&assignment.TemplateShiftID,
		&assignment.CreatedAt,
	); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}

	return assignment
}
