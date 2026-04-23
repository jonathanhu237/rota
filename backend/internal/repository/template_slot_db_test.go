//go:build integration

package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/lib/pq"
)

func TestTemplateSlotOverlapConstraint(t *testing.T) {
	ctx := context.Background()
	db := openIntegrationDB(t)
	repo := NewTemplateRepository(db)
	template := seedTemplate(t, db, templateSeed{})

	firstSlot, err := repo.CreateSlot(ctx, CreateTemplateSlotParams{
		TemplateID: template.ID,
		Weekday:    1,
		StartTime:  "09:00",
		EndTime:    "11:00",
	})
	if err != nil {
		t.Fatalf("create first slot: %v", err)
	}
	if firstSlot.ID <= 0 {
		t.Fatalf("expected first slot ID to be set")
	}

	_, err = repo.CreateSlot(ctx, CreateTemplateSlotParams{
		TemplateID: template.ID,
		Weekday:    1,
		StartTime:  "10:00",
		EndTime:    "12:00",
	})
	if !errors.Is(err, ErrTemplateSlotOverlap) {
		t.Fatalf("expected ErrTemplateSlotOverlap, got %v", err)
	}
}

func TestTemplateSlotUpdateOverlapConstraint(t *testing.T) {
	ctx := context.Background()
	db := openIntegrationDB(t)
	repo := NewTemplateRepository(db)
	template := seedTemplate(t, db, templateSeed{})

	firstSlot, err := repo.CreateSlot(ctx, CreateTemplateSlotParams{
		TemplateID: template.ID,
		Weekday:    1,
		StartTime:  "09:00",
		EndTime:    "11:00",
	})
	if err != nil {
		t.Fatalf("create first slot: %v", err)
	}
	secondSlot, err := repo.CreateSlot(ctx, CreateTemplateSlotParams{
		TemplateID: template.ID,
		Weekday:    1,
		StartTime:  "12:00",
		EndTime:    "14:00",
	})
	if err != nil {
		t.Fatalf("create second slot: %v", err)
	}

	_, err = repo.UpdateSlot(ctx, UpdateTemplateSlotParams{
		TemplateID: template.ID,
		SlotID:     secondSlot.ID,
		Weekday:    1,
		StartTime:  "10:30",
		EndTime:    "12:30",
	})
	if !errors.Is(err, ErrTemplateSlotOverlap) {
		t.Fatalf("expected ErrTemplateSlotOverlap, got %v", err)
	}
	if firstSlot.ID == secondSlot.ID {
		t.Fatalf("expected distinct slot IDs")
	}
}

func TestAssignmentPositionTriggerRejectsPositionOutsideSlot(t *testing.T) {
	db := openIntegrationDB(t)
	template := seedTemplate(t, db, templateSeed{})
	matchingPosition := seedPosition(t, db, positionSeed{Name: "Matching"})
	otherPosition := seedPosition(t, db, positionSeed{Name: "Other"})
	slotID := seedTemplateSlot(t, db, template.ID, 1, "09:00", "11:00")
	seedTemplateSlotPosition(t, db, slotID, matchingPosition.ID, 1)

	publication := seedPublication(t, db, publicationSeed{
		TemplateID:        template.ID,
		State:             model.PublicationStateAssigning,
		SubmissionStartAt: testTime().Add(-4 * time.Hour),
		SubmissionEndAt:   testTime().Add(-2 * time.Hour),
		PlannedActiveFrom: testTime().Add(1 * time.Hour),
		CreatedAt:         testTime().Add(-5 * time.Hour),
	})
	user := seedUser(t, db, userSeed{})

	_, err := db.ExecContext(
		context.Background(),
		`
			INSERT INTO assignments (
				publication_id,
				user_id,
				slot_id,
				position_id,
				created_at
			)
			VALUES ($1, $2, $3, $4, $5);
		`,
		publication.ID,
		user.ID,
		slotID,
		otherPosition.ID,
		testTime(),
	)
	if err == nil {
		t.Fatalf("expected assignment insert to fail")
	}

	if !strings.Contains(err.Error(), "is not part of slot") {
		t.Fatalf("expected trigger error, got %v", err)
	}
}

func TestTemplateSlotRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	db := openIntegrationDB(t)
	repo := NewTemplateRepository(db)

	template := seedTemplate(t, db, templateSeed{})
	firstPosition := seedPosition(t, db, positionSeed{Name: "Front Desk"})
	secondPosition := seedPosition(t, db, positionSeed{Name: "Runner"})

	slot, err := repo.CreateSlot(ctx, CreateTemplateSlotParams{
		TemplateID: template.ID,
		Weekday:    2,
		StartTime:  "9:05",
		EndTime:    "17:45",
	})
	if err != nil {
		t.Fatalf("create slot: %v", err)
	}
	assertTemplateSlotEqual(t, slot, template.ID, 2, "09:05", "17:45")

	slotPosition, err := repo.CreateSlotPosition(ctx, CreateTemplateSlotPositionParams{
		TemplateID:        template.ID,
		SlotID:            slot.ID,
		PositionID:        firstPosition.ID,
		RequiredHeadcount: 3,
	})
	if err != nil {
		t.Fatalf("create slot position: %v", err)
	}
	assertTemplateSlotPositionEqual(t, slotPosition, slot.ID, firstPosition.ID, 3)

	loadedSlot, err := repo.GetSlot(ctx, template.ID, slot.ID)
	if err != nil {
		t.Fatalf("get slot: %v", err)
	}
	assertTemplateSlotEqual(t, loadedSlot, template.ID, 2, "09:05", "17:45")

	listedSlots, err := repo.ListSlotsByTemplate(ctx, template.ID)
	if err != nil {
		t.Fatalf("list slots: %v", err)
	}
	if len(listedSlots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(listedSlots))
	}
	assertTemplateSlotEqual(t, listedSlots[0], template.ID, 2, "09:05", "17:45")

	listedPositions, err := repo.ListSlotPositions(ctx, slot.ID)
	if err != nil {
		t.Fatalf("list slot positions: %v", err)
	}
	if len(listedPositions) != 1 {
		t.Fatalf("expected 1 slot position, got %d", len(listedPositions))
	}
	assertTemplateSlotPositionEqual(t, listedPositions[0], slot.ID, firstPosition.ID, 3)

	updatedSlot, err := repo.UpdateSlot(ctx, UpdateTemplateSlotParams{
		TemplateID: template.ID,
		SlotID:     slot.ID,
		Weekday:    4,
		StartTime:  "10:07",
		EndTime:    "18:30",
	})
	if err != nil {
		t.Fatalf("update slot: %v", err)
	}
	assertTemplateSlotEqual(t, updatedSlot, template.ID, 4, "10:07", "18:30")

	updatedSlotPosition, err := repo.UpdateSlotPosition(ctx, UpdateTemplateSlotPositionParams{
		TemplateID:        template.ID,
		SlotID:            slot.ID,
		SlotPositionID:    slotPosition.ID,
		PositionID:        secondPosition.ID,
		RequiredHeadcount: 4,
	})
	if err != nil {
		t.Fatalf("update slot position: %v", err)
	}
	assertTemplateSlotPositionEqual(t, updatedSlotPosition, slot.ID, secondPosition.ID, 4)

	loadedSlotPosition, err := repo.GetSlotPosition(ctx, template.ID, slot.ID, slotPosition.ID)
	if err != nil {
		t.Fatalf("get slot position: %v", err)
	}
	assertTemplateSlotPositionEqual(t, loadedSlotPosition, slot.ID, secondPosition.ID, 4)

	if err := repo.DeleteSlotPosition(ctx, template.ID, slot.ID, slotPosition.ID); err != nil {
		t.Fatalf("delete slot position: %v", err)
	}

	listedPositions, err = repo.ListSlotPositions(ctx, slot.ID)
	if err != nil {
		t.Fatalf("list slot positions after delete: %v", err)
	}
	if len(listedPositions) != 0 {
		t.Fatalf("expected no slot positions after delete, got %d", len(listedPositions))
	}

	slotPosition, err = repo.CreateSlotPosition(ctx, CreateTemplateSlotPositionParams{
		TemplateID:        template.ID,
		SlotID:            slot.ID,
		PositionID:        firstPosition.ID,
		RequiredHeadcount: 1,
	})
	if err != nil {
		t.Fatalf("recreate slot position: %v", err)
	}

	if err := repo.DeleteSlot(ctx, template.ID, slot.ID); err != nil {
		t.Fatalf("delete slot: %v", err)
	}

	listedSlots, err = repo.ListSlotsByTemplate(ctx, template.ID)
	if err != nil {
		t.Fatalf("list slots after delete: %v", err)
	}
	if len(listedSlots) != 0 {
		t.Fatalf("expected no slots after delete, got %d", len(listedSlots))
	}

	if _, err := repo.GetSlotPosition(ctx, template.ID, slot.ID, slotPosition.ID); !errors.Is(err, ErrTemplateSlotPositionNotFound) {
		t.Fatalf("expected ErrTemplateSlotPositionNotFound after slot cascade delete, got %v", err)
	}
}

func TestTemplateSlotPositionUniqueConstraint(t *testing.T) {
	ctx := context.Background()
	db := openIntegrationDB(t)
	repo := NewTemplateRepository(db)

	template := seedTemplate(t, db, templateSeed{})
	position := seedPosition(t, db, positionSeed{Name: "Front Desk"})

	slot, err := repo.CreateSlot(ctx, CreateTemplateSlotParams{
		TemplateID: template.ID,
		Weekday:    1,
		StartTime:  "09:00",
		EndTime:    "11:00",
	})
	if err != nil {
		t.Fatalf("create slot: %v", err)
	}

	if _, err := repo.CreateSlotPosition(ctx, CreateTemplateSlotPositionParams{
		TemplateID:        template.ID,
		SlotID:            slot.ID,
		PositionID:        position.ID,
		RequiredHeadcount: 1,
	}); err != nil {
		t.Fatalf("create first slot position: %v", err)
	}

	_, err = repo.CreateSlotPosition(ctx, CreateTemplateSlotPositionParams{
		TemplateID:        template.ID,
		SlotID:            slot.ID,
		PositionID:        position.ID,
		RequiredHeadcount: 2,
	})
	if err == nil {
		t.Fatalf("expected duplicate slot-position insert to fail")
	}

	pqErr, ok := err.(*pq.Error)
	if !ok {
		t.Fatalf("expected pq.Error, got %T (%v)", err, err)
	}
	if pqErr.Code != "23505" {
		t.Fatalf("expected unique violation 23505, got %s", pqErr.Code)
	}
}

func TestAssignmentSlotUniqueConstraint(t *testing.T) {
	db := openIntegrationDB(t)
	template := seedTemplate(t, db, templateSeed{})
	firstPosition := seedPosition(t, db, positionSeed{Name: "Front Desk"})
	secondPosition := seedPosition(t, db, positionSeed{Name: "Runner"})
	slotID := seedTemplateSlot(t, db, template.ID, 1, "09:00", "11:00")
	seedTemplateSlotPosition(t, db, slotID, firstPosition.ID, 1)
	seedTemplateSlotPosition(t, db, slotID, secondPosition.ID, 1)

	publication := seedPublication(t, db, publicationSeed{
		TemplateID:        template.ID,
		State:             model.PublicationStateAssigning,
		SubmissionStartAt: testTime().Add(-4 * time.Hour),
		SubmissionEndAt:   testTime().Add(-2 * time.Hour),
		PlannedActiveFrom: testTime().Add(1 * time.Hour),
		CreatedAt:         testTime().Add(-5 * time.Hour),
	})
	user := seedUser(t, db, userSeed{})

	if _, err := db.ExecContext(
		context.Background(),
		`
			INSERT INTO assignments (
				publication_id,
				user_id,
				slot_id,
				position_id,
				created_at
			)
			VALUES ($1, $2, $3, $4, $5);
		`,
		publication.ID,
		user.ID,
		slotID,
		firstPosition.ID,
		testTime(),
	); err != nil {
		t.Fatalf("seed first assignment: %v", err)
	}

	_, err := db.ExecContext(
		context.Background(),
		`
			INSERT INTO assignments (
				publication_id,
				user_id,
				slot_id,
				position_id,
				created_at
			)
			VALUES ($1, $2, $3, $4, $5);
		`,
		publication.ID,
		user.ID,
		slotID,
		secondPosition.ID,
		testTime(),
	)
	if err == nil {
		t.Fatalf("expected duplicate slot assignment to fail")
	}

	pqErr, ok := err.(*pq.Error)
	if !ok {
		t.Fatalf("expected pq.Error, got %T (%v)", err, err)
	}
	if pqErr.Code != "23505" {
		t.Fatalf("expected unique violation 23505, got %s", pqErr.Code)
	}
}

func assertTemplateSlotEqual(
	t testing.TB,
	got *model.TemplateSlot,
	wantTemplateID int64,
	wantWeekday int,
	wantStart string,
	wantEnd string,
) {
	t.Helper()

	if got.TemplateID != wantTemplateID {
		t.Fatalf("expected template ID %d, got %d", wantTemplateID, got.TemplateID)
	}
	if got.Weekday != wantWeekday {
		t.Fatalf("expected weekday %d, got %d", wantWeekday, got.Weekday)
	}
	if got.StartTime != wantStart {
		t.Fatalf("expected start time %q, got %q", wantStart, got.StartTime)
	}
	if got.EndTime != wantEnd {
		t.Fatalf("expected end time %q, got %q", wantEnd, got.EndTime)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("expected slot timestamps to be populated: %+v", got)
	}
}

func assertTemplateSlotPositionEqual(
	t testing.TB,
	got *model.TemplateSlotPosition,
	wantSlotID, wantPositionID int64,
	wantRequiredHeadcount int,
) {
	t.Helper()

	if got.SlotID != wantSlotID {
		t.Fatalf("expected slot ID %d, got %d", wantSlotID, got.SlotID)
	}
	if got.PositionID != wantPositionID {
		t.Fatalf("expected position ID %d, got %d", wantPositionID, got.PositionID)
	}
	if got.RequiredHeadcount != wantRequiredHeadcount {
		t.Fatalf("expected headcount %d, got %d", wantRequiredHeadcount, got.RequiredHeadcount)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("expected slot position timestamps to be populated: %+v", got)
	}
}

func seedTemplateSlot(
	t testing.TB,
	db queryer,
	templateID int64,
	weekday int,
	startTime string,
	endTime string,
) int64 {
	t.Helper()

	var id int64
	if err := db.QueryRowContext(
		context.Background(),
		`
			INSERT INTO template_slots (
				template_id,
				weekday,
				start_time,
				end_time,
				created_at,
				updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $5)
			RETURNING id;
		`,
		templateID,
		weekday,
		startTime,
		endTime,
		testTime(),
	).Scan(&id); err != nil {
		t.Fatalf("seed template slot: %v", err)
	}

	return id
}

func seedTemplateSlotPosition(
	t testing.TB,
	db queryer,
	slotID int64,
	positionID int64,
	requiredHeadcount int,
) int64 {
	t.Helper()

	var id int64
	if err := db.QueryRowContext(
		context.Background(),
		`
			INSERT INTO template_slot_positions (
				slot_id,
				position_id,
				required_headcount,
				created_at,
				updated_at
			)
			VALUES ($1, $2, $3, $4, $4)
			RETURNING id;
		`,
		slotID,
		positionID,
		requiredHeadcount,
		testTime(),
	).Scan(&id); err != nil {
		t.Fatalf("seed template slot position: %v", err)
	}

	return id
}
