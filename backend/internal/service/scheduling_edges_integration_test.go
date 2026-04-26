//go:build integration

package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

func TestApplyGiveDisabledReceiver(t *testing.T) {
	ctx := context.Background()
	db := openServiceIntegrationDB(t)
	now := serviceIntegrationTestTime()
	pubID, templateID, positionID := seedPublishedSchedulingPublication(t, db, "Disabled Give", now)
	slotID := seedServiceSlot(t, db, templateID, 1, "09:00", "11:00")
	seedServiceSlotPosition(t, db, slotID, positionID, 1)
	requester := seedServiceUser(t, db, "disabled-give-requester@example.com", "Disabled Give Requester")
	receiver := seedServiceUser(t, db, "disabled-give-receiver@example.com", "Disabled Give Receiver")
	for _, userID := range []int64{requester, receiver} {
		seedServiceUserPosition(t, db, userID, positionID)
	}
	assignment := seedServiceAssignment(t, db, pubID, requester, slotID, positionID, now)
	pubRepo := repository.NewPublicationRepository(db)
	shiftRepo := repository.NewShiftChangeRepository(db)
	shiftSvc := NewShiftChangeService(shiftRepo, pubRepo, nil, "", fixedClock{now: now}, nil)
	request := seedServiceShiftChange(t, shiftRepo, repository.CreateShiftChangeRequestParams{
		PublicationID:         pubID,
		Type:                  model.ShiftChangeTypeGiveDirect,
		RequesterUserID:       requester,
		RequesterAssignmentID: assignment.ID,
		OccurrenceDate:        time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC),
		CounterpartUserID:     &receiver,
		ExpiresAt:             now.Add(time.Hour),
		CreatedAt:             now,
	})
	disableServiceUser(t, db, receiver)

	err := shiftSvc.ApproveShiftChangeRequest(ctx, request.ID, receiver)
	if !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("expected ErrUserDisabled, got %v", err)
	}
	assertShiftChangeState(t, db, request.ID, model.ShiftChangeStatePending)
}

func TestApplyGiveDisabledReceiverWhileStatusLocked(t *testing.T) {
	ctx := context.Background()
	db := openServiceIntegrationDB(t)
	now := serviceIntegrationTestTime()
	pubID, templateID, positionID := seedPublishedSchedulingPublication(t, db, "Disabled Give Locked", now)
	existingSlot := seedServiceSlot(t, db, templateID, 1, "08:00", "09:00")
	giveSlot := seedServiceSlot(t, db, templateID, 1, "12:00", "13:00")
	seedServiceSlotPosition(t, db, existingSlot, positionID, 1)
	seedServiceSlotPosition(t, db, giveSlot, positionID, 1)
	requester := seedServiceUser(t, db, "disabled-give-locked-requester@example.com", "Disabled Give Locked Requester")
	receiver := seedServiceUser(t, db, "disabled-give-locked-receiver@example.com", "Disabled Give Locked Receiver")
	for _, userID := range []int64{requester, receiver} {
		seedServiceUserPosition(t, db, userID, positionID)
	}
	seedServiceAssignment(t, db, pubID, receiver, existingSlot, positionID, now)
	giveAssignment := seedServiceAssignment(t, db, pubID, requester, giveSlot, positionID, now)
	shiftRepo := repository.NewShiftChangeRepository(db)
	request := seedServiceShiftChange(t, shiftRepo, repository.CreateShiftChangeRequestParams{
		PublicationID:         pubID,
		Type:                  model.ShiftChangeTypeGiveDirect,
		RequesterUserID:       requester,
		RequesterAssignmentID: giveAssignment.ID,
		OccurrenceDate:        time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC),
		CounterpartUserID:     &receiver,
		ExpiresAt:             now.Add(time.Hour),
		CreatedAt:             now,
	})

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin lock tx: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT status FROM users WHERE id = $1 FOR UPDATE;`, receiver); err != nil {
		t.Fatalf("lock receiver status: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := shiftRepo.ApplyGive(ctx, repository.ApplyGiveParams{
			RequestID:             request.ID,
			PublicationID:         pubID,
			RequesterAssignmentID: giveAssignment.ID,
			RequesterUserID:       requester,
			OccurrenceDate:        request.OccurrenceDate,
			ReceiverUserID:        receiver,
			DecidedByUserID:       receiver,
			Now:                   now,
		})
		errCh <- err
	}()
	time.Sleep(100 * time.Millisecond)

	if _, err := tx.ExecContext(ctx, `UPDATE users SET status = 'disabled' WHERE id = $1`, receiver); err != nil {
		t.Fatalf("disable locked receiver: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit lock tx: %v", err)
	}
	if err := <-errCh; !errors.Is(err, repository.ErrUserDisabled) {
		t.Fatalf("expected repository ErrUserDisabled, got %v", err)
	}
	assertShiftChangeState(t, db, request.ID, model.ShiftChangeStatePending)
	assertUserAssignmentCount(t, db, pubID, receiver, 1)
}

func TestApplySwapDisabledCounterpart(t *testing.T) {
	ctx := context.Background()
	db := openServiceIntegrationDB(t)
	now := serviceIntegrationTestTime()
	pubID, templateID, positionID := seedPublishedSchedulingPublication(t, db, "Disabled Swap", now)
	slotRequester := seedServiceSlot(t, db, templateID, 1, "09:00", "11:00")
	slotCounterpart := seedServiceSlot(t, db, templateID, 2, "12:00", "14:00")
	seedServiceSlotPosition(t, db, slotRequester, positionID, 1)
	seedServiceSlotPosition(t, db, slotCounterpart, positionID, 1)
	requester := seedServiceUser(t, db, "disabled-swap-requester@example.com", "Disabled Swap Requester")
	counterpart := seedServiceUser(t, db, "disabled-swap-counterpart@example.com", "Disabled Swap Counterpart")
	for _, userID := range []int64{requester, counterpart} {
		seedServiceUserPosition(t, db, userID, positionID)
	}
	requesterAssignment := seedServiceAssignment(t, db, pubID, requester, slotRequester, positionID, now)
	counterpartAssignment := seedServiceAssignment(t, db, pubID, counterpart, slotCounterpart, positionID, now)
	pubRepo := repository.NewPublicationRepository(db)
	shiftRepo := repository.NewShiftChangeRepository(db)
	shiftSvc := NewShiftChangeService(shiftRepo, pubRepo, nil, "", fixedClock{now: now}, nil)
	request := seedServiceShiftChange(t, shiftRepo, repository.CreateShiftChangeRequestParams{
		PublicationID:             pubID,
		Type:                      model.ShiftChangeTypeSwap,
		RequesterUserID:           requester,
		RequesterAssignmentID:     requesterAssignment.ID,
		OccurrenceDate:            time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC),
		CounterpartUserID:         &counterpart,
		CounterpartAssignmentID:   &counterpartAssignment.ID,
		CounterpartOccurrenceDate: timePtr(time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)),
		ExpiresAt:                 now.Add(time.Hour),
		CreatedAt:                 now,
	})
	disableServiceUser(t, db, counterpart)

	err := shiftSvc.ApproveShiftChangeRequest(ctx, request.ID, counterpart)
	if !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("expected ErrUserDisabled, got %v", err)
	}
	assertShiftChangeState(t, db, request.ID, model.ShiftChangeStatePending)
}

func seedPublishedSchedulingPublication(t testing.TB, db *sql.DB, name string, now time.Time) (int64, int64, int64) {
	t.Helper()

	templateID := seedServiceTemplate(t, db, name+" Template")
	positionID := seedServicePosition(t, db, name+" Position")
	publicationID := seedServicePublication(t, db, templateID, now)
	if _, err := db.ExecContext(
		context.Background(),
		`UPDATE publications SET state = 'PUBLISHED' WHERE id = $1`,
		publicationID,
	); err != nil {
		t.Fatalf("publish seeded publication: %v", err)
	}
	return publicationID, templateID, positionID
}

func seedServiceAssignment(
	t testing.TB,
	db *sql.DB,
	publicationID, userID, slotID, positionID int64,
	createdAt time.Time,
) *model.Assignment {
	t.Helper()

	assignment := &model.Assignment{}
	if err := db.QueryRowContext(
		context.Background(),
		`
				INSERT INTO assignments (publication_id, user_id, slot_id, weekday, position_id, created_at)
				VALUES ($1, $2, $3, $4, $5, $6)
				RETURNING id, publication_id, user_id, slot_id, weekday, position_id, created_at;
			`,
		publicationID,
		userID,
		slotID,
		seedServiceSlotWeekday(t, db, slotID),
		positionID,
		createdAt,
	).Scan(
		&assignment.ID,
		&assignment.PublicationID,
		&assignment.UserID,
		&assignment.SlotID,
		&assignment.Weekday,
		&assignment.PositionID,
		&assignment.CreatedAt,
	); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}
	return assignment
}

func seedServiceShiftChange(
	t testing.TB,
	repo *repository.ShiftChangeRepository,
	params repository.CreateShiftChangeRequestParams,
) *model.ShiftChangeRequest {
	t.Helper()

	req, err := repo.Create(context.Background(), params)
	if err != nil {
		t.Fatalf("seed shift-change request: %v", err)
	}
	return req
}

func assertUserAssignmentCount(t testing.TB, db *sql.DB, publicationID, userID int64, expected int) {
	t.Helper()

	var count int
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT count(*) FROM assignments WHERE publication_id = $1 AND user_id = $2`,
		publicationID,
		userID,
	).Scan(&count); err != nil {
		t.Fatalf("count user assignments: %v", err)
	}
	if count != expected {
		t.Fatalf("expected %d assignments for user %d, got %d", expected, userID, count)
	}
}

func disableServiceUser(t testing.TB, db *sql.DB, userID int64) {
	t.Helper()

	if _, err := db.ExecContext(
		context.Background(),
		`UPDATE users SET status = 'disabled' WHERE id = $1`,
		userID,
	); err != nil {
		t.Fatalf("disable user: %v", err)
	}
}

func assertShiftChangeState(t testing.TB, db *sql.DB, requestID int64, expected model.ShiftChangeState) {
	t.Helper()

	var state model.ShiftChangeState
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT state FROM shift_change_requests WHERE id = $1`,
		requestID,
	).Scan(&state); err != nil {
		t.Fatalf("load shift-change state: %v", err)
	}
	if state != expected {
		t.Fatalf("expected request state %q, got %q", expected, state)
	}
}
