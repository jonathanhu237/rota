//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jonathanhu237/rota/api/internal/rota/model"
)

const (
	schedulingIntegrationUserA = "019535d9-3df7-79fb-b466-fa907fa17f9c"
	schedulingIntegrationUserB = "019535d9-3df7-79fb-b466-fa907fa17f9d"
)

func TestRotaSchedulingRepositoriesCoverTransactionsConstraintsAndRaces(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(24)
	db.SetMaxIdleConns(24)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
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

	for _, user := range []struct{ id, email, name string }{
		{schedulingIntegrationUserA, "scheduling-a@example.com", "Scheduling A"},
		{schedulingIntegrationUserB, "scheduling-b@example.com", "Scheduling B"},
	} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO auth_users (id, name, email, email_canonical, password_hash)
			VALUES ($1::uuid, $2, $3, $3, 'test-hash')`, user.id, user.name, user.email); err != nil {
			t.Fatal(err)
		}
	}

	positions := NewPositionRepository(db)
	templates := NewTemplateRepository(db)
	publications := NewPublicationRepository(db)
	shifts := NewShiftChangeRepository(db)
	leaves := NewLeaveRepository(db)
	attendance := NewAttendanceRepository(db)
	overrides := NewAssignmentOverrideRepository(db)

	position, err := positions.Create(ctx, CreatePositionParams{Name: "Scheduling Position"})
	if err != nil {
		t.Fatalf("create position: %v", err)
	}
	template, err := templates.Create(ctx, CreateTemplateParams{Name: "Scheduling Template"})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	slot, err := templates.CreateSlot(ctx, CreateTemplateSlotParams{
		TemplateID: template.ID, Weekdays: []int{1}, StartTime: "09:00", EndTime: "11:00",
	})
	if err != nil {
		t.Fatalf("create slot: %v", err)
	}
	if _, err := templates.CreateSlotPosition(ctx, CreateTemplateSlotPositionParams{
		TemplateID: template.ID, SlotID: slot.ID, PositionID: position.ID, RequiredHeadcount: 1, AttendanceResponsible: true,
	}); err != nil {
		t.Fatalf("create slot position: %v", err)
	}
	if _, err := templates.CreateSlot(ctx, CreateTemplateSlotParams{
		TemplateID: template.ID, Weekdays: []int{1}, StartTime: "10:00", EndTime: "12:00",
	}); !errors.Is(err, ErrTemplateSlotOverlap) {
		t.Fatalf("overlapping slot error = %v, want ErrTemplateSlotOverlap", err)
	}

	now := time.Date(2026, 1, 5, 8, 0, 0, 0, time.UTC)
	publication, err := publications.CreatePublication(ctx, CreatePublicationParams{
		TemplateID:               template.ID,
		Name:                     "Scheduling Publication",
		State:                    model.PublicationStateDraft,
		SubmissionStartAt:        now.Add(-48 * time.Hour),
		SubmissionEndAt:          now.Add(-24 * time.Hour),
		PlannedActiveFrom:        now,
		PlannedActiveUntil:       now.Add(14 * 24 * time.Hour),
		OvertimeEntryWindowHours: 24,
		CreatedAt:                now,
	})
	if err != nil {
		t.Fatalf("create publication: %v", err)
	}

	// The schedule lock plus the unique key must make concurrent duplicate
	// assignment writes deterministic: one succeeds and the loser receives a
	// domain conflict rather than a duplicate row or a raw driver error.
	type assignmentResult struct {
		assignment *model.Assignment
		err        error
	}
	startGate := make(chan struct{})
	results := make(chan assignmentResult, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-startGate
			assignment, err := publications.CreateAssignment(ctx, CreateAssignmentParams{
				PublicationID: publication.ID,
				UserID:        schedulingIntegrationUserA,
				SlotID:        slot.ID,
				Weekday:       1,
				PositionID:    position.ID,
				CreatedAt:     now,
			})
			results <- assignmentResult{assignment: assignment, err: err}
		}()
	}
	close(startGate)
	wait.Wait()
	close(results)

	var assignment *model.Assignment
	var successes, conflicts int
	for result := range results {
		switch {
		case result.err == nil:
			successes++
			assignment = result.assignment
		case errors.Is(result.err, ErrAssignmentUserAlreadyInSlot):
			conflicts++
		default:
			t.Fatalf("concurrent assignment error = %v", result.err)
		}
	}
	if successes != 1 || conflicts != 1 || assignment == nil {
		t.Fatalf("concurrent assignments = successes %d conflicts %d, want 1/1", successes, conflicts)
	}
	if got, err := publications.GetAssignment(ctx, assignment.ID); err != nil || got.UserID != schedulingIntegrationUserA {
		t.Fatalf("GetAssignment() = %#v, %v", got, err)
	}
	if _, err := publications.CreateAssignment(ctx, CreateAssignmentParams{
		PublicationID: publication.ID, UserID: schedulingIntegrationUserB, SlotID: slot.ID, Weekday: 1, PositionID: position.ID, CreatedAt: now,
	}); err != nil {
		t.Fatalf("second user assignment: %v", err)
	}
	if _, err := publications.CreateAssignment(ctx, CreateAssignmentParams{
		PublicationID: publication.ID, UserID: schedulingIntegrationUserB, SlotID: slot.ID, Weekday: 1, PositionID: position.ID + 100, CreatedAt: now,
	}); !errors.Is(err, ErrTemplateSlotPositionNotFound) {
		t.Fatalf("invalid assignment position error = %v, want ErrTemplateSlotPositionNotFound", err)
	}

	// Occurrence overrides are a separate numeric-ID scheduling entity while
	// their replacement user remains a canonical auth UUID.
	weekTwo := now.AddDate(0, 0, 7)
	override, err := overrides.Insert(ctx, InsertAssignmentOverrideParams{
		AssignmentID: assignment.ID, OccurrenceDate: weekTwo, UserID: schedulingIntegrationUserB, CreatedAt: now,
	})
	if err != nil || override.UserID != schedulingIntegrationUserB {
		t.Fatalf("Insert override = %#v, %v", override, err)
	}
	if count, err := overrides.DeleteByAssignment(ctx, assignment.ID); err != nil || count != 1 {
		t.Fatalf("Delete override = %d, %v", count, err)
	}
	if _, err := overrides.Insert(ctx, InsertAssignmentOverrideParams{
		AssignmentID: assignment.ID, OccurrenceDate: weekTwo, UserID: schedulingIntegrationUserB, CreatedAt: now,
	}); err != nil {
		t.Fatalf("restore override: %v", err)
	}
	if count, err := publications.CountAssignmentOverridesByAssignment(ctx, assignment.ID); err != nil || count != 1 {
		t.Fatalf("CountAssignmentOverridesByAssignment() = %d, %v", count, err)
	}

	// Exercise the request transaction and the atomic give application. The
	// callback is deliberately observable only inside the transaction; a later
	// read verifies the request state and override committed together.
	request, err := shifts.Create(ctx, CreateShiftChangeRequestParams{
		PublicationID:         publication.ID,
		Type:                  model.ShiftChangeTypeGiveDirect,
		RequesterUserID:       schedulingIntegrationUserA,
		RequesterAssignmentID: assignment.ID,
		OccurrenceDate:        now,
		CounterpartUserID:     stringPtrForIntegration(schedulingIntegrationUserB),
		ExpiresAt:             now.Add(24 * time.Hour),
		CreatedAt:             now,
		AfterCreateTx: func(ctx context.Context, tx *sql.Tx, _ *model.ShiftChangeRequest) error {
			var one int
			return tx.QueryRowContext(ctx, `SELECT 1`).Scan(&one)
		},
	})
	if err != nil {
		t.Fatalf("Create shift-change request: %v", err)
	}
	if _, err := shifts.ApplyGive(ctx, ApplyGiveParams{
		RequestID:             request.ID,
		PublicationID:         publication.ID,
		RequesterAssignmentID: assignment.ID,
		RequesterUserID:       schedulingIntegrationUserA,
		OccurrenceDate:        now,
		ReceiverUserID:        schedulingIntegrationUserB,
		DecidedByUserID:       schedulingIntegrationUserB,
		Now:                   now,
		AfterApplyTx: func(ctx context.Context, tx *sql.Tx) error {
			var one int
			return tx.QueryRowContext(ctx, `SELECT 1`).Scan(&one)
		},
	}); err != nil {
		t.Fatalf("ApplyGive: %v", err)
	}
	loaded, err := shifts.GetByID(ctx, request.ID)
	if err != nil || loaded.State != model.ShiftChangeStateApproved {
		t.Fatalf("approved request = %#v, %v", loaded, err)
	}
	assignments, err := overrides.ListForPublicationWeek(ctx, publication.ID, now)
	if err != nil || len(assignments) == 0 {
		t.Fatalf("ListForPublicationWeek() = %#v, %v", assignments, err)
	}
	foundOverride := false
	for _, item := range assignments {
		if item.AssignmentID == assignment.ID && item.UserID == schedulingIntegrationUserB {
			foundOverride = true
		}
	}
	if !foundOverride {
		t.Fatalf("approved give did not appear in roster assignments: %#v", assignments)
	}

	// Leave rows are linked to the request in one transaction, then queried
	// through the historical display joins and active-occurrence index.
	leaveRequest, err := shifts.Create(ctx, CreateShiftChangeRequestParams{
		PublicationID:         publication.ID,
		Type:                  model.ShiftChangeTypeGivePool,
		RequesterUserID:       schedulingIntegrationUserA,
		RequesterAssignmentID: assignment.ID,
		OccurrenceDate:        now.AddDate(0, 0, 14),
		ExpiresAt:             now.Add(48 * time.Hour),
		CreatedAt:             now,
	})
	if err != nil {
		t.Fatalf("create leave request: %v", err)
	}
	var leave *model.Leave
	if err := leaves.WithTx(ctx, func(tx *sql.Tx) error {
		var insertErr error
		leave, insertErr = leaves.Insert(ctx, tx, InsertLeaveParams{
			UserID:               schedulingIntegrationUserA,
			PublicationID:        publication.ID,
			ShiftChangeRequestID: leaveRequest.ID,
			Category:             model.LeaveCategorySick,
			Reason:               "integration",
			CreatedAt:            now,
			UpdatedAt:            now,
		})
		if insertErr != nil {
			return insertErr
		}
		_, insertErr = shifts.SetLeaveIDTx(ctx, tx, leaveRequest.ID, leave.ID)
		return insertErr
	}); err != nil {
		t.Fatalf("insert leave transaction: %v", err)
	}
	leaveRow, err := leaves.GetWithRequestByID(ctx, leave.ID)
	if err != nil || leaveRow.Request.ID != leaveRequest.ID {
		t.Fatalf("GetWithRequestByID() = %#v, %v", leaveRow, err)
	}
	if keys, err := leaves.ListActiveOccurrenceKeys(ctx, schedulingIntegrationUserA, publication.ID); err != nil || len(keys) != 1 || !keys[0].OccurrenceDate.Equal(model.NormalizeOccurrenceDate(now.AddDate(0, 0, 14))) {
		t.Fatalf("active leave keys = %#v, %v", keys, err)
	}
	if rows, err := leaves.ListForUser(ctx, schedulingIntegrationUserA, 1, 10); err != nil || len(rows) != 1 {
		t.Fatalf("ListForUser() = %#v, %v", rows, err)
	}
	if rows, err := leaves.ListForPublication(ctx, publication.ID, 1, 10); err != nil || len(rows) != 1 {
		t.Fatalf("ListForPublication() = %#v, %v", rows, err)
	}
	if rows, total, err := leaves.ListPool(ctx, ListLeavePoolParams{ViewerUserID: schedulingIntegrationUserB, State: model.LeavePoolStateAll, Now: now, Limit: 10}); err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("ListPool() = rows %#v total %d err %v", rows, total, err)
	}

	// Attendance repository methods share the same UUID-backed assignment
	// rows. Verify the unique rejection, admin idempotent update, roster reads,
	// overtime CRUD, and the orphan/empty projections.
	occurrence := now.AddDate(0, 0, 21)
	arrival, err := attendance.InsertLeaderArrival(ctx, UpsertAttendanceArrivalParams{
		PublicationID: publication.ID, AssignmentID: assignment.ID, OccurrenceDate: occurrence, UserID: schedulingIntegrationUserA,
		ArrivedAt: occurrence.Add(9*time.Hour + 5*time.Minute), RecordedByUserID: schedulingIntegrationUserA, RecordedAt: now,
	})
	if err != nil || arrival.UserID != schedulingIntegrationUserA {
		t.Fatalf("InsertLeaderArrival() = %#v, %v", arrival, err)
	}
	if _, err := attendance.InsertLeaderArrival(ctx, UpsertAttendanceArrivalParams{
		PublicationID: publication.ID, AssignmentID: assignment.ID, OccurrenceDate: occurrence, UserID: schedulingIntegrationUserA,
		ArrivedAt: occurrence.Add(9*time.Hour + 6*time.Minute), RecordedByUserID: schedulingIntegrationUserA, RecordedAt: now,
	}); !errors.Is(err, ErrAttendanceAlreadyRecorded) {
		t.Fatalf("duplicate arrival error = %v, want ErrAttendanceAlreadyRecorded", err)
	}
	updatedArrival, err := attendance.UpsertAdminArrival(ctx, UpsertAttendanceArrivalParams{
		PublicationID: publication.ID, AssignmentID: assignment.ID, OccurrenceDate: occurrence, UserID: schedulingIntegrationUserA,
		ArrivedAt: occurrence.Add(9*time.Hour + 10*time.Minute), RecordedByUserID: schedulingIntegrationUserB, RecordedAt: now,
	})
	if err != nil || updatedArrival.ID != arrival.ID {
		t.Fatalf("UpsertAdminArrival() = %#v, %v", updatedArrival, err)
	}
	if roster, err := attendance.ListShiftRoster(ctx, publication.ID, slot.ID, 1, occurrence); err != nil || len(roster) != 2 {
		t.Fatalf("ListShiftRoster() = %#v, %v", roster, err)
	} else {
		recorded := 0
		for _, row := range roster {
			if row.Record != nil {
				recorded++
			}
		}
		if recorded != 1 {
			t.Fatalf("ListShiftRoster() recorded rows = %d, want 1: %#v", recorded, roster)
		}
	}
	if refs, err := attendance.ListLeaderCandidateShifts(ctx, publication.ID, schedulingIntegrationUserA, occurrence.Add(-time.Hour), occurrence.Add(24*time.Hour)); err != nil || len(refs) != 1 {
		t.Fatalf("ListLeaderCandidateShifts() = %#v, %v", refs, err)
	}
	if refs, err := attendance.ListPublicationShiftRefsForDate(ctx, publication.ID, occurrence); err != nil || len(refs) != 1 {
		t.Fatalf("ListPublicationShiftRefsForDate() = %#v, %v", refs, err)
	}
	if orphans, err := attendance.ListOrphanArrivalRecords(ctx, publication.ID, slot.ID, 1, occurrence); err != nil || len(orphans) != 0 {
		t.Fatalf("ListOrphanArrivalRecords() = %#v, %v", orphans, err)
	}
	overtime, err := attendance.CreateOvertime(ctx, CreateOvertimeRecordParams{
		PublicationID: publication.ID, SlotID: slot.ID, Weekday: 1, OccurrenceDate: occurrence, UserID: schedulingIntegrationUserA,
		Hours: 1.5, Note: "handover", RecordedByUserID: schedulingIntegrationUserB, RecordedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateOvertime() = %#v, %v", overtime, err)
	}
	if records, err := attendance.ListOvertimeRecords(ctx, publication.ID, slot.ID, 1, occurrence); err != nil || len(records) != 1 {
		t.Fatalf("ListOvertimeRecords() = %#v, %v", records, err)
	}
	if got, err := attendance.GetOvertime(ctx, publication.ID, overtime.ID); err != nil || got.ID != overtime.ID {
		t.Fatalf("GetOvertime() = %#v, %v", got, err)
	}
	if got, err := attendance.UpdateOvertime(ctx, UpdateOvertimeRecordParams{PublicationID: publication.ID, RecordID: overtime.ID, Hours: 2, Note: "extended", UpdatedByUserID: schedulingIntegrationUserB, UpdatedAt: now}); err != nil || got.Hours != 2 {
		t.Fatalf("UpdateOvertime() = %#v, %v", got, err)
	}
	if _, err := attendance.DeleteOvertime(ctx, publication.ID, overtime.ID); err != nil {
		t.Fatalf("DeleteOvertime() error = %v", err)
	}
	if _, err := attendance.DeleteArrival(ctx, publication.ID, arrival.ID); err != nil {
		t.Fatalf("DeleteArrival() error = %v", err)
	}
	if _, err := attendance.DeleteArrival(ctx, publication.ID, arrival.ID); !errors.Is(err, ErrAttendanceRecordNotFound) {
		t.Fatalf("missing arrival error = %v, want ErrAttendanceRecordNotFound", err)
	}

	// Keep a direct database constraint check in the integration suite too:
	// position rows referenced by a template slot cannot be silently removed.
	if err := positions.Delete(ctx, position.ID); err == nil {
		t.Fatal("deleting a referenced position unexpectedly succeeded")
	}
	if _, err := templates.CreateSlot(ctx, CreateTemplateSlotParams{
		TemplateID: template.ID, Weekdays: []int{1}, StartTime: "10:00", EndTime: "12:00",
	}); !errors.Is(err, ErrTemplateLocked) {
		t.Fatalf("locked template slot error = %v, want ErrTemplateLocked", err)
	}
}

func stringPtrForIntegration(value string) *string { return &value }
