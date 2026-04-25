//go:build integration

package service

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

func TestConcurrentApplyGive(t *testing.T) {
	ctx := context.Background()
	db := openServiceIntegrationDB(t)
	now := serviceIntegrationTestTime()
	pubID, templateID, positionID := seedPublishedSchedulingPublication(t, db, "Concurrent Give", now)
	slotOne := seedServiceSlot(t, db, templateID, 1, "09:00", "11:00")
	slotTwo := seedServiceSlot(t, db, templateID, 1, "10:00", "12:00")
	seedServiceSlotPosition(t, db, slotOne, positionID, 1)
	seedServiceSlotPosition(t, db, slotTwo, positionID, 1)

	requesterOne := seedServiceUser(t, db, "give-requester-one@example.com", "Give Requester One")
	requesterTwo := seedServiceUser(t, db, "give-requester-two@example.com", "Give Requester Two")
	receiver := seedServiceUser(t, db, "give-receiver@example.com", "Give Receiver")
	for _, userID := range []int64{requesterOne, requesterTwo, receiver} {
		seedServiceUserPosition(t, db, userID, positionID)
	}

	assignmentOne := seedServiceAssignment(t, db, pubID, requesterOne, slotOne, positionID, now)
	assignmentTwo := seedServiceAssignment(t, db, pubID, requesterTwo, slotTwo, positionID, now)
	shiftRepo := repository.NewShiftChangeRepository(db)
	requestOne := seedServiceShiftChange(t, shiftRepo, repository.CreateShiftChangeRequestParams{
		PublicationID:         pubID,
		Type:                  model.ShiftChangeTypeGiveDirect,
		RequesterUserID:       requesterOne,
		RequesterAssignmentID: assignmentOne.ID,
		CounterpartUserID:     &receiver,
		ExpiresAt:             now.Add(time.Hour),
		CreatedAt:             now,
	})
	requestTwo := seedServiceShiftChange(t, shiftRepo, repository.CreateShiftChangeRequestParams{
		PublicationID:         pubID,
		Type:                  model.ShiftChangeTypeGiveDirect,
		RequesterUserID:       requesterTwo,
		RequesterAssignmentID: assignmentTwo.ID,
		CounterpartUserID:     &receiver,
		ExpiresAt:             now.Add(time.Hour),
		CreatedAt:             now,
	})

	errs := runConcurrently(
		func() error {
			_, err := shiftRepo.ApplyGive(ctx, repository.ApplyGiveParams{
				RequestID:             requestOne.ID,
				RequesterAssignmentID: assignmentOne.ID,
				RequesterUserID:       requesterOne,
				ReceiverUserID:        receiver,
				DecidedByUserID:       receiver,
				Now:                   now,
			})
			return err
		},
		func() error {
			_, err := shiftRepo.ApplyGive(ctx, repository.ApplyGiveParams{
				RequestID:             requestTwo.ID,
				RequesterAssignmentID: assignmentTwo.ID,
				RequesterUserID:       requesterTwo,
				ReceiverUserID:        receiver,
				DecidedByUserID:       receiver,
				Now:                   now,
			})
			return err
		},
	)
	assertOneSuccessOneError(t, errs, model.ErrShiftChangeTimeConflict)
	assertUserAssignmentCount(t, db, pubID, receiver, 1)
}

func TestConcurrentApplySwap(t *testing.T) {
	ctx := context.Background()
	db := openServiceIntegrationDB(t)
	now := serviceIntegrationTestTime()
	pubID, templateID, positionID := seedPublishedSchedulingPublication(t, db, "Concurrent Swap", now)
	slotA := seedServiceSlot(t, db, templateID, 1, "08:00", "09:00")
	slotB := seedServiceSlot(t, db, templateID, 1, "09:00", "11:00")
	slotC := seedServiceSlot(t, db, templateID, 2, "08:00", "09:00")
	slotD := seedServiceSlot(t, db, templateID, 1, "10:00", "12:00")
	for _, slotID := range []int64{slotA, slotB, slotC, slotD} {
		seedServiceSlotPosition(t, db, slotID, positionID, 1)
	}

	requester := seedServiceUser(t, db, "swap-requester@example.com", "Swap Requester")
	counterpartOne := seedServiceUser(t, db, "swap-counterpart-one@example.com", "Swap Counterpart One")
	counterpartTwo := seedServiceUser(t, db, "swap-counterpart-two@example.com", "Swap Counterpart Two")
	for _, userID := range []int64{requester, counterpartOne, counterpartTwo} {
		seedServiceUserPosition(t, db, userID, positionID)
	}

	assignmentA := seedServiceAssignment(t, db, pubID, requester, slotA, positionID, now)
	assignmentC := seedServiceAssignment(t, db, pubID, requester, slotC, positionID, now)
	assignmentB := seedServiceAssignment(t, db, pubID, counterpartOne, slotB, positionID, now)
	assignmentD := seedServiceAssignment(t, db, pubID, counterpartTwo, slotD, positionID, now)
	shiftRepo := repository.NewShiftChangeRepository(db)
	requestOne := seedServiceShiftChange(t, shiftRepo, repository.CreateShiftChangeRequestParams{
		PublicationID:           pubID,
		Type:                    model.ShiftChangeTypeSwap,
		RequesterUserID:         requester,
		RequesterAssignmentID:   assignmentA.ID,
		CounterpartUserID:       &counterpartOne,
		CounterpartAssignmentID: &assignmentB.ID,
		ExpiresAt:               now.Add(time.Hour),
		CreatedAt:               now,
	})
	requestTwo := seedServiceShiftChange(t, shiftRepo, repository.CreateShiftChangeRequestParams{
		PublicationID:           pubID,
		Type:                    model.ShiftChangeTypeSwap,
		RequesterUserID:         requester,
		RequesterAssignmentID:   assignmentC.ID,
		CounterpartUserID:       &counterpartTwo,
		CounterpartAssignmentID: &assignmentD.ID,
		ExpiresAt:               now.Add(time.Hour),
		CreatedAt:               now,
	})

	errs := runConcurrently(
		func() error {
			_, err := shiftRepo.ApplySwap(ctx, repository.ApplySwapParams{
				RequestID:               requestOne.ID,
				RequesterAssignmentID:   assignmentA.ID,
				RequesterUserID:         requester,
				CounterpartAssignmentID: assignmentB.ID,
				CounterpartUserID:       counterpartOne,
				DecidedByUserID:         counterpartOne,
				Now:                     now,
			})
			return err
		},
		func() error {
			_, err := shiftRepo.ApplySwap(ctx, repository.ApplySwapParams{
				RequestID:               requestTwo.ID,
				RequesterAssignmentID:   assignmentC.ID,
				RequesterUserID:         requester,
				CounterpartAssignmentID: assignmentD.ID,
				CounterpartUserID:       counterpartTwo,
				DecidedByUserID:         counterpartTwo,
				Now:                     now,
			})
			return err
		},
	)
	assertOneSuccessOneError(t, errs, model.ErrShiftChangeTimeConflict)
	assertNoUserOverlap(t, db, pubID, requester)
}

func TestConcurrentCreateAssignment(t *testing.T) {
	ctx := context.Background()
	db := openServiceIntegrationDB(t)
	now := serviceIntegrationTestTime()
	pubID, templateID, positionID := seedPublishedSchedulingPublication(t, db, "Concurrent Create", now)
	slotGive := seedServiceSlot(t, db, templateID, 1, "09:00", "11:00")
	slotCreate := seedServiceSlot(t, db, templateID, 1, "10:00", "12:00")
	seedServiceSlotPosition(t, db, slotGive, positionID, 1)
	seedServiceSlotPosition(t, db, slotCreate, positionID, 1)

	requester := seedServiceUser(t, db, "create-race-requester@example.com", "Create Race Requester")
	receiver := seedServiceUser(t, db, "create-race-receiver@example.com", "Create Race Receiver")
	for _, userID := range []int64{requester, receiver} {
		seedServiceUserPosition(t, db, userID, positionID)
	}

	giveAssignment := seedServiceAssignment(t, db, pubID, requester, slotGive, positionID, now)
	pubRepo := repository.NewPublicationRepository(db)
	shiftRepo := repository.NewShiftChangeRepository(db)
	publicationSvc := NewPublicationService(pubRepo, fixedClock{now: now})
	request := seedServiceShiftChange(t, shiftRepo, repository.CreateShiftChangeRequestParams{
		PublicationID:         pubID,
		Type:                  model.ShiftChangeTypeGiveDirect,
		RequesterUserID:       requester,
		RequesterAssignmentID: giveAssignment.ID,
		CounterpartUserID:     &receiver,
		ExpiresAt:             now.Add(time.Hour),
		CreatedAt:             now,
	})

	errs := runConcurrently(
		func() error {
			_, err := publicationSvc.CreateAssignment(ctx, CreateAssignmentInput{
				PublicationID: pubID,
				UserID:        receiver,
				SlotID:        slotCreate,
				PositionID:    positionID,
			})
			return err
		},
		func() error {
			_, err := shiftRepo.ApplyGive(ctx, repository.ApplyGiveParams{
				RequestID:             request.ID,
				RequesterAssignmentID: giveAssignment.ID,
				RequesterUserID:       requester,
				ReceiverUserID:        receiver,
				DecidedByUserID:       receiver,
				Now:                   now,
			})
			return err
		},
	)
	if countNil(errs) != 1 {
		t.Fatalf("expected exactly one success, got errors %v", errs)
	}
	for _, err := range errs {
		if err == nil {
			continue
		}
		if !errors.Is(err, ErrAssignmentTimeConflict) && !errors.Is(err, model.ErrShiftChangeTimeConflict) {
			t.Fatalf("expected assignment or shift-change conflict, got %v", err)
		}
	}
	assertUserAssignmentCount(t, db, pubID, receiver, 1)
}

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

func TestApplyGiveDisabledReceiverWhileScheduleLocked(t *testing.T) {
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
	existingAssignment := seedServiceAssignment(t, db, pubID, receiver, existingSlot, positionID, now)
	giveAssignment := seedServiceAssignment(t, db, pubID, requester, giveSlot, positionID, now)
	shiftRepo := repository.NewShiftChangeRepository(db)
	request := seedServiceShiftChange(t, shiftRepo, repository.CreateShiftChangeRequestParams{
		PublicationID:         pubID,
		Type:                  model.ShiftChangeTypeGiveDirect,
		RequesterUserID:       requester,
		RequesterAssignmentID: giveAssignment.ID,
		CounterpartUserID:     &receiver,
		ExpiresAt:             now.Add(time.Hour),
		CreatedAt:             now,
	})

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin lock tx: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT id FROM assignments WHERE id = $1 FOR UPDATE;`, existingAssignment.ID); err != nil {
		t.Fatalf("lock receiver assignment: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := shiftRepo.ApplyGive(ctx, repository.ApplyGiveParams{
			RequestID:             request.ID,
			RequesterAssignmentID: giveAssignment.ID,
			RequesterUserID:       requester,
			ReceiverUserID:        receiver,
			DecidedByUserID:       receiver,
			Now:                   now,
		})
		errCh <- err
	}()
	time.Sleep(100 * time.Millisecond)

	disableServiceUser(t, db, receiver)
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
	slotCounterpart := seedServiceSlot(t, db, templateID, 2, "09:00", "11:00")
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
		PublicationID:           pubID,
		Type:                    model.ShiftChangeTypeSwap,
		RequesterUserID:         requester,
		RequesterAssignmentID:   requesterAssignment.ID,
		CounterpartUserID:       &counterpart,
		CounterpartAssignmentID: &counterpartAssignment.ID,
		ExpiresAt:               now.Add(time.Hour),
		CreatedAt:               now,
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
			INSERT INTO assignments (publication_id, user_id, slot_id, position_id, created_at)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, publication_id, user_id, slot_id, position_id, created_at;
		`,
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

func runConcurrently(left func() error, right func() error) []error {
	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		errs[0] = left()
	}()
	go func() {
		defer wg.Done()
		<-start
		errs[1] = right()
	}()
	close(start)
	wg.Wait()
	return errs
}

func assertOneSuccessOneError(t testing.TB, errs []error, expected error) {
	t.Helper()

	if countNil(errs) != 1 {
		t.Fatalf("expected exactly one success, got errors %v", errs)
	}
	for _, err := range errs {
		if err == nil {
			continue
		}
		if !errors.Is(err, expected) {
			t.Fatalf("expected %v, got %v", expected, err)
		}
	}
}

func countNil(errs []error) int {
	count := 0
	for _, err := range errs {
		if err == nil {
			count++
		}
	}
	return count
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

func assertNoUserOverlap(t testing.TB, db *sql.DB, publicationID, userID int64) {
	t.Helper()

	rows, err := db.QueryContext(
		context.Background(),
		`
			SELECT ts.weekday, TO_CHAR(ts.start_time, 'HH24:MI'), TO_CHAR(ts.end_time, 'HH24:MI')
			FROM assignments a
			INNER JOIN template_slots ts ON ts.id = a.slot_id
			WHERE a.publication_id = $1 AND a.user_id = $2
			ORDER BY a.id;
		`,
		publicationID,
		userID,
	)
	if err != nil {
		t.Fatalf("query user assignments: %v", err)
	}
	defer rows.Close()

	windows := make([]repository.SlotTimeWindow, 0)
	for rows.Next() {
		var window repository.SlotTimeWindow
		if err := rows.Scan(&window.Weekday, &window.StartTime, &window.EndTime); err != nil {
			t.Fatalf("scan user assignment: %v", err)
		}
		windows = append(windows, window)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate user assignments: %v", err)
	}
	for i := 0; i < len(windows); i++ {
		for j := i + 1; j < len(windows); j++ {
			if windows[i].Weekday == windows[j].Weekday &&
				windows[i].StartTime < windows[j].EndTime &&
				windows[j].StartTime < windows[i].EndTime {
				t.Fatalf("expected no overlap for user %d, got %+v", userID, windows)
			}
		}
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
