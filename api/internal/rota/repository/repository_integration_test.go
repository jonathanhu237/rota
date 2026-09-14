//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jonathanhu237/rota/api/internal/rota/model"
)

const integrationUserID = "019535d9-3df7-79fb-b466-fa907fa17f91"

func TestRotaRepositoriesAgainstPostgresAndConcurrentWrites(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(16)
	db.SetMaxIdleConns(16)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	resetRotaIntegrationState(t, ctx, db)
	t.Cleanup(func() {
		resetRotaIntegrationState(t, context.Background(), db)
		_ = db.Close()
	})

	if _, err := db.ExecContext(ctx, `
		INSERT INTO auth_users (id, name, email, email_canonical, password_hash)
		VALUES ($1::uuid, 'Rota Integration User', 'rota-integration@example.com', 'rota-integration@example.com', 'test-hash')`, integrationUserID); err != nil {
		t.Fatal(err)
	}

	positions := NewPositionRepository(db)
	templates := NewTemplateRepository(db)
	publications := NewPublicationRepository(db)

	position, err := positions.Create(ctx, CreatePositionParams{Name: "Integration Position", Description: ""})
	if err != nil {
		t.Fatalf("Create position: %v", err)
	}
	template, err := templates.Create(ctx, CreateTemplateParams{Name: "Integration Template", Description: ""})
	if err != nil {
		t.Fatalf("Create template: %v", err)
	}
	slot, err := templates.CreateSlot(ctx, CreateTemplateSlotParams{
		TemplateID: template.ID,
		Weekdays:   []int{1},
		StartTime:  "09:00",
		EndTime:    "10:00",
	})
	if err != nil {
		t.Fatalf("Create slot: %v", err)
	}
	if _, err := templates.CreateSlotPosition(ctx, CreateTemplateSlotPositionParams{
		TemplateID:            template.ID,
		SlotID:                slot.ID,
		PositionID:            position.ID,
		RequiredHeadcount:     1,
		AttendanceResponsible: true,
	}); err != nil {
		t.Fatalf("Create slot position: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO user_positions (user_id, position_id) VALUES ($1::uuid, $2)`, integrationUserID, position.ID); err != nil {
		t.Fatal(err)
	}

	// The partial unique index is the database-level single-publication guard.
	// Two independent transactions must produce exactly one publication and a
	// domain-level conflict for the loser, rather than duplicate active work.
	now := time.Date(2026, 1, 5, 8, 0, 0, 0, time.UTC)
	start := now.Add(time.Hour)
	end := start.Add(time.Hour)
	activeFrom := end
	activeUntil := activeFrom.Add(24 * time.Hour)
	startGate := make(chan struct{})
	publicationErrors := make(chan error, 2)
	var publicationWait sync.WaitGroup
	for i := 0; i < 2; i++ {
		publicationWait.Add(1)
		go func(index int) {
			defer publicationWait.Done()
			<-startGate
			_, createErr := publications.CreatePublication(ctx, CreatePublicationParams{
				TemplateID:               template.ID,
				Name:                     fmt.Sprintf("Concurrent publication %d", index),
				State:                    model.PublicationStateDraft,
				SubmissionStartAt:        start,
				SubmissionEndAt:          end,
				PlannedActiveFrom:        activeFrom,
				PlannedActiveUntil:       activeUntil,
				OvertimeEntryWindowHours: 24,
				CreatedAt:                now,
			})
			publicationErrors <- createErr
		}(i)
	}
	close(startGate)
	publicationWait.Wait()
	close(publicationErrors)

	var publicationSuccesses, publicationConflicts int
	for createErr := range publicationErrors {
		switch {
		case createErr == nil:
			publicationSuccesses++
		case errors.Is(createErr, ErrPublicationAlreadyExists):
			publicationConflicts++
		default:
			t.Fatalf("concurrent CreatePublication() error = %v", createErr)
		}
	}
	if publicationSuccesses != 1 || publicationConflicts != 1 {
		t.Fatalf("concurrent publications = successes %d conflicts %d, want 1/1", publicationSuccesses, publicationConflicts)
	}

	publication, err := publications.GetCurrent(ctx)
	if err != nil {
		t.Fatalf("GetCurrent publication: %v", err)
	}
	if publication == nil {
		t.Fatal("GetCurrent publication returned nil")
	}
	if _, err := db.ExecContext(ctx, `UPDATE publications SET state = 'COLLECTING' WHERE id = $1`, publication.ID); err != nil {
		t.Fatal(err)
	}
	collecting := model.PublicationStateCollecting

	// Upsert is intentionally idempotent and guarded by the same advisory
	// schedule lock used by service writes. Concurrent callers must all observe
	// one row, not unique-constraint failures or duplicate submissions.
	const submissionAttempts = 8
	submissionErrors := make(chan error, submissionAttempts)
	var submissionWait sync.WaitGroup
	for range submissionAttempts {
		submissionWait.Add(1)
		go func() {
			defer submissionWait.Done()
			_, upsertErr := publications.UpsertSubmission(ctx, UpsertAvailabilitySubmissionParams{
				PublicationID:    publication.ID,
				UserID:           integrationUserID,
				SlotID:           slot.ID,
				Weekday:          1,
				PublicationState: &collecting,
				Now:              now,
			})
			submissionErrors <- upsertErr
		}()
	}
	submissionWait.Wait()
	close(submissionErrors)
	for upsertErr := range submissionErrors {
		if upsertErr != nil {
			t.Fatalf("concurrent UpsertSubmission() error = %v", upsertErr)
		}
	}

	submissions, err := publications.ListSubmissionSlots(ctx, publication.ID, integrationUserID)
	if err != nil {
		t.Fatalf("ListSubmissionSlots: %v", err)
	}
	if len(submissions) != 1 || submissions[0] != (model.SlotRef{SlotID: slot.ID, Weekday: 1}) {
		t.Fatalf("submissions = %#v, want one UUID-owned slot", submissions)
	}
	qualified, err := publications.ListQualifiedPublicationSlotPositions(ctx, publication.ID, integrationUserID)
	if err != nil {
		t.Fatalf("ListQualifiedPublicationSlotPositions: %v", err)
	}
	if len(qualified) != 1 || len(qualified[0].Composition) != 1 || qualified[0].Composition[0].PositionID != position.ID {
		t.Fatalf("qualified shifts = %#v, want one qualified position", qualified)
	}

	if _, err := db.ExecContext(ctx, `UPDATE auth_users SET disabled_at = $2 WHERE id = $1::uuid`, integrationUserID, now); err != nil {
		t.Fatal(err)
	}
	_, err = publications.UpsertSubmission(ctx, UpsertAvailabilitySubmissionParams{
		PublicationID:    publication.ID,
		UserID:           integrationUserID,
		SlotID:           slot.ID,
		Weekday:          1,
		PublicationState: &collecting,
		Now:              now,
	})
	if !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("UpsertSubmission(disabled user) error = %v, want ErrUserDisabled", err)
	}
}

func resetRotaIntegrationState(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	if db == nil {
		return
	}
	if _, err := db.ExecContext(ctx, `
		TRUNCATE TABLE
			attendance_overtime_records,
			attendance_records,
			leaves,
			shift_change_requests,
			assignment_overrides,
			assignments,
			availability_submissions,
			publications,
			template_slot_positions,
			template_slot_weekdays,
			template_slots,
			templates,
			user_positions,
			positions
		RESTART IDENTITY CASCADE`); err != nil {
		t.Errorf("reset Rota state: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM auth_users WHERE id IN ($1::uuid, $2::uuid, $3::uuid)`, integrationUserID, schedulingIntegrationUserA, schedulingIntegrationUserB); err != nil {
		t.Errorf("delete integration auth users: %v", err)
	}
}
