//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

func TestAttendanceResponsiblePositionConstraints(t *testing.T) {
	ctx := context.Background()
	db := openIntegrationDB(t)
	repo := NewTemplateRepository(db)

	template := seedTemplate(t, db, templateSeed{})
	firstPosition := seedPosition(t, db, positionSeed{Name: "Lead"})
	secondPosition := seedPosition(t, db, positionSeed{Name: "Second Lead"})
	slot, err := repo.CreateSlot(ctx, CreateTemplateSlotParams{
		TemplateID: template.ID,
		Weekdays:   []int{1},
		StartTime:  "09:00",
		EndTime:    "12:00",
	})
	if err != nil {
		t.Fatalf("create slot: %v", err)
	}

	_, err = repo.CreateSlotPosition(ctx, CreateTemplateSlotPositionParams{
		TemplateID:            template.ID,
		SlotID:                slot.ID,
		PositionID:            firstPosition.ID,
		RequiredHeadcount:     2,
		AttendanceResponsible: true,
	})
	if !errors.Is(err, ErrAttendanceResponsibleRequired) {
		t.Fatalf("expected ErrAttendanceResponsibleRequired for responsible headcount > 1, got %v", err)
	}

	if _, err := repo.CreateSlotPosition(ctx, CreateTemplateSlotPositionParams{
		TemplateID:            template.ID,
		SlotID:                slot.ID,
		PositionID:            firstPosition.ID,
		RequiredHeadcount:     1,
		AttendanceResponsible: true,
	}); err != nil {
		t.Fatalf("create first responsible slot position: %v", err)
	}

	_, err = repo.CreateSlotPosition(ctx, CreateTemplateSlotPositionParams{
		TemplateID:            template.ID,
		SlotID:                slot.ID,
		PositionID:            secondPosition.ID,
		RequiredHeadcount:     1,
		AttendanceResponsible: true,
	})
	if !errors.Is(err, ErrAttendanceResponsibleRequired) {
		t.Fatalf("expected ErrAttendanceResponsibleRequired for second responsible row, got %v", err)
	}
}

func TestAttendanceRepositoryIntegration(t *testing.T) {
	t.Run("Roster follows occurrence override and stale arrival is rejected", func(t *testing.T) {
		ctx := context.Background()
		db := openIntegrationDB(t)
		fixture := seedAttendanceRepositoryFixture(t, db)
		repo := NewAttendanceRepository(db)
		publicationRepo := NewPublicationRepository(db)

		if _, err := publicationRepo.InsertAssignmentOverride(ctx, InsertAssignmentOverrideParams{
			AssignmentID:   fixture.workerAssignment.ID,
			OccurrenceDate: fixture.occurrenceDate,
			UserID:         fixture.replacement.ID,
			CreatedAt:      fixture.scheduledStart,
		}); err != nil {
			t.Fatalf("insert assignment override: %v", err)
		}

		if _, err := db.ExecContext(
			ctx,
			`
				INSERT INTO attendance_records (
					publication_id,
					assignment_id,
					occurrence_date,
					user_id,
					arrived_at,
					recorded_by_user_id,
					recorded_at,
					updated_by_user_id,
					updated_at
				)
				VALUES ($1, $2, $3, $4, $5, $6, $5, $6, $5);
			`,
			fixture.publication.ID,
			fixture.workerAssignment.ID,
			fixture.occurrenceDate,
			fixture.worker.ID,
			fixture.scheduledStart,
			fixture.leader.ID,
		); err != nil {
			t.Fatalf("seed orphan arrival: %v", err)
		}

		roster, err := repo.ListShiftRoster(
			ctx,
			fixture.publication.ID,
			fixture.slotID,
			1,
			fixture.occurrenceDate,
		)
		if err != nil {
			t.Fatalf("list shift roster: %v", err)
		}
		replacementRow := findAttendanceRosterRow(t, roster, fixture.workerAssignment.ID)
		if replacementRow.UserID != fixture.replacement.ID {
			t.Fatalf("expected override replacement user %d, got %+v", fixture.replacement.ID, replacementRow)
		}

		orphans, err := repo.ListOrphanArrivalRecords(
			ctx,
			fixture.publication.ID,
			fixture.slotID,
			1,
			fixture.occurrenceDate,
		)
		if err != nil {
			t.Fatalf("list orphan arrivals: %v", err)
		}
		if len(orphans) != 1 || orphans[0].UserID != fixture.worker.ID {
			t.Fatalf("expected one orphan arrival for original worker, got %+v", orphans)
		}

		_, err = repo.InsertLeaderArrival(ctx, UpsertAttendanceArrivalParams{
			PublicationID:    fixture.publication.ID,
			AssignmentID:     fixture.workerAssignment.ID,
			OccurrenceDate:   fixture.occurrenceDate,
			UserID:           fixture.worker.ID,
			ArrivedAt:        fixture.scheduledStart,
			RecordedByUserID: fixture.leader.ID,
			RecordedAt:       fixture.scheduledStart,
		})
		if !errors.Is(err, ErrAttendanceRosterStale) {
			t.Fatalf("expected ErrAttendanceRosterStale for stale user, got %v", err)
		}
	})

	t.Run("Leader arrival is unique and publication deletion cascades attendance facts", func(t *testing.T) {
		ctx := context.Background()
		db := openIntegrationDB(t)
		fixture := seedAttendanceRepositoryFixture(t, db)
		repo := NewAttendanceRepository(db)

		if _, err := repo.InsertLeaderArrival(ctx, UpsertAttendanceArrivalParams{
			PublicationID:    fixture.publication.ID,
			AssignmentID:     fixture.workerAssignment.ID,
			OccurrenceDate:   fixture.occurrenceDate,
			UserID:           fixture.worker.ID,
			ArrivedAt:        fixture.scheduledStart,
			RecordedByUserID: fixture.leader.ID,
			RecordedAt:       fixture.scheduledStart,
		}); err != nil {
			t.Fatalf("insert leader arrival: %v", err)
		}

		_, err := repo.InsertLeaderArrival(ctx, UpsertAttendanceArrivalParams{
			PublicationID:    fixture.publication.ID,
			AssignmentID:     fixture.workerAssignment.ID,
			OccurrenceDate:   fixture.occurrenceDate,
			UserID:           fixture.worker.ID,
			ArrivedAt:        fixture.scheduledStart,
			RecordedByUserID: fixture.leader.ID,
			RecordedAt:       fixture.scheduledStart,
		})
		if !errors.Is(err, ErrAttendanceAlreadyRecorded) {
			t.Fatalf("expected ErrAttendanceAlreadyRecorded, got %v", err)
		}

		overtime, err := repo.CreateOvertime(ctx, CreateOvertimeRecordParams{
			PublicationID:    fixture.publication.ID,
			SlotID:           fixture.slotID,
			Weekday:          1,
			OccurrenceDate:   fixture.occurrenceDate,
			UserID:           fixture.worker.ID,
			Hours:            1.5,
			Note:             "Inventory reconciliation",
			RecordedByUserID: fixture.leader.ID,
			RecordedAt:       fixture.scheduledStart,
		})
		if err != nil {
			t.Fatalf("create overtime: %v", err)
		}
		fetchedOvertime, err := repo.GetOvertime(ctx, fixture.publication.ID, overtime.ID)
		if err != nil {
			t.Fatalf("get overtime: %v", err)
		}
		if fetchedOvertime.Hours != 1.5 || fetchedOvertime.Note != "Inventory reconciliation" {
			t.Fatalf("unexpected fetched overtime: %+v", fetchedOvertime)
		}
		updatedOvertime, err := repo.UpdateOvertime(ctx, UpdateOvertimeRecordParams{
			PublicationID:   fixture.publication.ID,
			RecordID:        overtime.ID,
			Hours:           2.25,
			Note:            "Inventory follow-up",
			UpdatedByUserID: fixture.leader.ID,
			UpdatedAt:       fixture.scheduledStart.Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("update overtime: %v", err)
		}
		if updatedOvertime.Hours != 2.25 || updatedOvertime.Note != "Inventory follow-up" {
			t.Fatalf("unexpected updated overtime: %+v", updatedOvertime)
		}

		if _, err := db.ExecContext(ctx, `DELETE FROM publications WHERE id = $1;`, fixture.publication.ID); err != nil {
			t.Fatalf("delete publication: %v", err)
		}
		assertTableCount(t, db, `SELECT COUNT(*) FROM attendance_records WHERE publication_id = $1`, fixture.publication.ID, 0)
		assertTableCount(t, db, `SELECT COUNT(*) FROM attendance_overtime_records WHERE publication_id = $1`, fixture.publication.ID, 0)
	})

	t.Run("Overtime constraints reject invalid hours and notes", func(t *testing.T) {
		ctx := context.Background()
		db := openIntegrationDB(t)
		fixture := seedAttendanceRepositoryFixture(t, db)
		repo := NewAttendanceRepository(db)

		_, err := repo.CreateOvertime(ctx, CreateOvertimeRecordParams{
			PublicationID:    fixture.publication.ID,
			SlotID:           fixture.slotID,
			Weekday:          1,
			OccurrenceDate:   fixture.occurrenceDate,
			UserID:           fixture.worker.ID,
			Hours:            24.01,
			Note:             "Too much",
			RecordedByUserID: fixture.leader.ID,
			RecordedAt:       fixture.scheduledStart,
		})
		if err == nil {
			t.Fatal("expected overtime hours check to reject 24.01")
		}

		_, err = repo.CreateOvertime(ctx, CreateOvertimeRecordParams{
			PublicationID:    fixture.publication.ID,
			SlotID:           fixture.slotID,
			Weekday:          1,
			OccurrenceDate:   fixture.occurrenceDate,
			UserID:           fixture.worker.ID,
			Hours:            1,
			Note:             " needs trimming ",
			RecordedByUserID: fixture.leader.ID,
			RecordedAt:       fixture.scheduledStart,
		})
		if err == nil {
			t.Fatal("expected overtime note trim check to reject untrimmed note")
		}
	})
}

type attendanceRepositoryFixture struct {
	publication      *model.Publication
	leader           *model.User
	worker           *model.User
	replacement      *model.User
	slotID           int64
	leaderAssignment *model.Assignment
	workerAssignment *model.Assignment
	occurrenceDate   time.Time
	scheduledStart   time.Time
}

func seedAttendanceRepositoryFixture(t testing.TB, db *sql.DB) attendanceRepositoryFixture {
	t.Helper()

	ctx := context.Background()
	templateRepo := NewTemplateRepository(db)
	template := seedTemplate(t, db, templateSeed{Name: "Attendance Template"})
	leaderPosition := seedPosition(t, db, positionSeed{Name: "负责人"})
	workerPosition := seedPosition(t, db, positionSeed{Name: "Worker"})
	slot, err := templateRepo.CreateSlot(ctx, CreateTemplateSlotParams{
		TemplateID: template.ID,
		Weekdays:   []int{1},
		StartTime:  "09:00",
		EndTime:    "12:00",
	})
	if err != nil {
		t.Fatalf("create attendance slot: %v", err)
	}
	if _, err := templateRepo.CreateSlotPosition(ctx, CreateTemplateSlotPositionParams{
		TemplateID:            template.ID,
		SlotID:                slot.ID,
		PositionID:            leaderPosition.ID,
		RequiredHeadcount:     1,
		AttendanceResponsible: true,
	}); err != nil {
		t.Fatalf("create responsible slot position: %v", err)
	}
	if _, err := templateRepo.CreateSlotPosition(ctx, CreateTemplateSlotPositionParams{
		TemplateID:        template.ID,
		SlotID:            slot.ID,
		PositionID:        workerPosition.ID,
		RequiredHeadcount: 1,
	}); err != nil {
		t.Fatalf("create worker slot position: %v", err)
	}

	publication := seedPublication(t, db, publicationSeed{
		TemplateID:         template.ID,
		State:              model.PublicationStateActive,
		SubmissionStartAt:  time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC),
		SubmissionEndAt:    time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC),
		PlannedActiveFrom:  time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		PlannedActiveUntil: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		CreatedAt:          time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC),
	})
	leader := seedUser(t, db, userSeed{Name: "Lead"})
	worker := seedUser(t, db, userSeed{Name: "Alice"})
	replacement := seedUser(t, db, userSeed{Name: "Bob"})
	seedUserPosition(t, db, leader.ID, leaderPosition.ID)
	seedUserPosition(t, db, worker.ID, workerPosition.ID)
	seedUserPosition(t, db, replacement.ID, workerPosition.ID)

	scheduledStart := time.Date(2026, 5, 11, 9, 0, 0, 0, time.UTC)
	leaderAssignment := seedAssignment(t, db, publication.ID, leader.ID, slot.ID, leaderPosition.ID, scheduledStart)
	workerAssignment := seedAssignment(t, db, publication.ID, worker.ID, slot.ID, workerPosition.ID, scheduledStart)

	return attendanceRepositoryFixture{
		publication:      publication,
		leader:           leader,
		worker:           worker,
		replacement:      replacement,
		slotID:           slot.ID,
		leaderAssignment: leaderAssignment,
		workerAssignment: workerAssignment,
		occurrenceDate:   model.NormalizeOccurrenceDate(scheduledStart),
		scheduledStart:   scheduledStart,
	}
}

func findAttendanceRosterRow(
	t testing.TB,
	rows []*model.AttendanceRosterRow,
	assignmentID int64,
) *model.AttendanceRosterRow {
	t.Helper()

	for _, row := range rows {
		if row.AssignmentID == assignmentID {
			return row
		}
	}
	t.Fatalf("attendance roster row for assignment %d not found in %+v", assignmentID, rows)
	return nil
}
