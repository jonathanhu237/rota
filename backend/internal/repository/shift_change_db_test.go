//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

// seedShiftChangeRequest inserts a shift_change_requests row directly to
// keep the test isolated from the repo's own Create path.
func seedShiftChangeRequest(
	t testing.TB,
	db *sql.DB,
	publicationID int64,
	changeType model.ShiftChangeType,
	requesterUserID, requesterAssignmentID int64,
	counterpartUserID *int64,
	counterpartAssignmentID *int64,
	state model.ShiftChangeState,
	createdAt, expiresAt time.Time,
) *model.ShiftChangeRequest {
	t.Helper()

	const query = `
		INSERT INTO shift_change_requests (
			publication_id,
			type,
			requester_user_id,
			requester_assignment_id,
			occurrence_date,
			counterpart_user_id,
			counterpart_assignment_id,
			counterpart_occurrence_date,
			state,
			created_at,
			expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING
			id,
			publication_id,
			type,
			requester_user_id,
			requester_assignment_id,
			occurrence_date,
			counterpart_user_id,
			counterpart_assignment_id,
			counterpart_occurrence_date,
			state,
			leave_id,
			decided_by_user_id,
			created_at,
			decided_at,
			expires_at;
	`

	var cpUser sql.NullInt64
	if counterpartUserID != nil {
		cpUser = sql.NullInt64{Int64: *counterpartUserID, Valid: true}
	}
	var cpAssignment sql.NullInt64
	if counterpartAssignmentID != nil {
		cpAssignment = sql.NullInt64{Int64: *counterpartAssignmentID, Valid: true}
	}
	occurrenceDate := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	cpOccurrence := sql.NullTime{}
	if counterpartAssignmentID != nil {
		cpOccurrence = sql.NullTime{Time: occurrenceDate.AddDate(0, 0, 1), Valid: true}
	}

	req, err := scanShiftChangeRequest(db.QueryRowContext(
		context.Background(),
		query,
		publicationID,
		changeType,
		requesterUserID,
		requesterAssignmentID,
		occurrenceDate,
		cpUser,
		cpAssignment,
		cpOccurrence,
		state,
		createdAt,
		expiresAt,
	))
	if err != nil {
		t.Fatalf("seed shift change request: %v", err)
	}
	return req
}

// fetchRequestState returns the current persisted state of a request row.
func fetchRequestState(t testing.TB, db *sql.DB, id int64) model.ShiftChangeState {
	t.Helper()

	var state model.ShiftChangeState
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT state FROM shift_change_requests WHERE id = $1;`,
		id,
	).Scan(&state); err != nil {
		t.Fatalf("fetch request state: %v", err)
	}
	return state
}

// fetchAssignmentUserID returns the current baseline user_id on an assignment row.
func fetchAssignmentUserID(t testing.TB, db *sql.DB, id int64) int64 {
	t.Helper()

	var userID int64
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT user_id FROM assignments WHERE id = $1;`,
		id,
	).Scan(&userID); err != nil {
		t.Fatalf("fetch assignment user_id: %v", err)
	}
	return userID
}

func fetchAssignmentOverrideUserID(
	t testing.TB,
	db *sql.DB,
	assignmentID int64,
	occurrenceDate time.Time,
) int64 {
	t.Helper()

	var userID int64
	if err := db.QueryRowContext(
		context.Background(),
		`
			SELECT user_id
			FROM assignment_overrides
			WHERE assignment_id = $1 AND occurrence_date = $2;
		`,
		assignmentID,
		model.NormalizeOccurrenceDate(occurrenceDate),
	).Scan(&userID); err != nil {
		t.Fatalf("fetch assignment override user_id: %v", err)
	}
	return userID
}

func TestShiftChangeRepositoryClaimRace(t *testing.T) {
	db := openIntegrationDB(t)
	repo := NewShiftChangeRepository(db)
	ctx := context.Background()

	admin := seedUser(t, db, userSeed{IsAdmin: true})
	position := seedPosition(t, db, positionSeed{})
	template := seedTemplate(t, db, templateSeed{})
	shift := seedQualifiedShift(t, db, qualifiedShiftSeed{
		TemplateID: template.ID,
		PositionID: position.ID,
	})
	publication := seedPublication(t, db, publicationSeed{
		TemplateID: template.ID,
		State:      model.PublicationStatePublished,
		CreatedAt:  testTime(),
	})

	requester := seedUser(t, db, userSeed{})
	claimerB := seedUser(t, db, userSeed{})
	claimerC := seedUser(t, db, userSeed{})
	// Both potential claimers are qualified for the position, per setup.
	seedUserPosition(t, db, claimerB.ID, position.ID)
	seedUserPosition(t, db, claimerC.ID, position.ID)

	assignment := seedAssignment(t, db, publication.ID, requester.ID, shift.SlotID, shift.PositionID, testTime())

	request := seedShiftChangeRequest(
		t,
		db,
		publication.ID,
		model.ShiftChangeTypeGivePool,
		requester.ID,
		assignment.ID,
		nil,
		nil,
		model.ShiftChangeStatePending,
		testTime(),
		testTime().Add(24*time.Hour),
	)

	type outcome struct {
		receiverID int64
		err        error
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []outcome
		// start gate lets both goroutines race as close to simultaneously as
		// possible; Postgres FOR UPDATE should still serialize them.
		start = make(chan struct{})
	)

	claim := func(receiver *model.User) {
		defer wg.Done()
		<-start
		_, err := repo.ApplyGive(ctx, ApplyGiveParams{
			RequestID:             request.ID,
			PublicationID:         publication.ID,
			RequesterAssignmentID: assignment.ID,
			RequesterUserID:       requester.ID,
			OccurrenceDate:        request.OccurrenceDate,
			ReceiverUserID:        receiver.ID,
			DecidedByUserID:       admin.ID,
			Now:                   testTime().Add(time.Hour),
		})
		mu.Lock()
		results = append(results, outcome{receiverID: receiver.ID, err: err})
		mu.Unlock()
	}

	wg.Add(2)
	go claim(claimerB)
	go claim(claimerC)
	close(start)
	wg.Wait()

	if len(results) != 2 {
		t.Fatalf("expected 2 outcomes, got %d", len(results))
	}

	var (
		winners int
		losers  int
		winner  outcome
	)
	for _, r := range results {
		switch {
		case r.err == nil:
			winners++
			winner = r
		case errors.Is(r.err, ErrShiftChangeNotPending):
			losers++
		default:
			t.Fatalf("unexpected error for receiver %d: %v", r.receiverID, r.err)
		}
	}

	if winners != 1 || losers != 1 {
		t.Fatalf("expected exactly one winner and one loser, got winners=%d losers=%d", winners, losers)
	}

	if got := fetchRequestState(t, db, request.ID); got != model.ShiftChangeStateApproved {
		t.Fatalf("expected request state approved, got %q", got)
	}
	if got := fetchAssignmentUserID(t, db, assignment.ID); got != requester.ID {
		t.Fatalf("expected baseline assignment user_id = requester %d, got %d", requester.ID, got)
	}
	if got := fetchAssignmentOverrideUserID(t, db, assignment.ID, request.OccurrenceDate); got != winner.receiverID {
		t.Fatalf("expected override user_id = winner %d, got %d", winner.receiverID, got)
	}
}

func TestShiftChangeRepositoryCreateAndLoad(t *testing.T) {
	db := openIntegrationDB(t)
	repo := NewShiftChangeRepository(db)
	ctx := context.Background()

	position := seedPosition(t, db, positionSeed{})
	template := seedTemplate(t, db, templateSeed{})
	shiftA := seedQualifiedShift(t, db, qualifiedShiftSeed{
		TemplateID: template.ID,
		PositionID: position.ID,
		Weekday:    1,
		StartTime:  "09:00",
		EndTime:    "12:00",
	})
	shiftB := seedQualifiedShift(t, db, qualifiedShiftSeed{
		TemplateID: template.ID,
		PositionID: position.ID,
		Weekday:    2,
		StartTime:  "13:00",
		EndTime:    "17:00",
	})
	publication := seedPublication(t, db, publicationSeed{
		TemplateID: template.ID,
		State:      model.PublicationStatePublished,
		CreatedAt:  testTime(),
	})

	requester := seedUser(t, db, userSeed{})
	counterpart := seedUser(t, db, userSeed{})

	requesterAssignment := seedAssignment(t, db, publication.ID, requester.ID, shiftA.SlotID, shiftA.PositionID, testTime())
	counterpartAssignment := seedAssignment(t, db, publication.ID, counterpart.ID, shiftB.SlotID, shiftB.PositionID, testTime())

	base := testTime()
	expires := base.Add(24 * time.Hour)
	monday := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	nextMonday := monday.AddDate(0, 0, 7)
	thirdMonday := monday.AddDate(0, 0, 14)
	tuesday := monday.AddDate(0, 0, 1)

	t.Run("swap round-trip", func(t *testing.T) {
		created, err := repo.Create(ctx, CreateShiftChangeRequestParams{
			PublicationID:             publication.ID,
			Type:                      model.ShiftChangeTypeSwap,
			RequesterUserID:           requester.ID,
			RequesterAssignmentID:     requesterAssignment.ID,
			OccurrenceDate:            monday,
			CounterpartUserID:         &counterpart.ID,
			CounterpartAssignmentID:   &counterpartAssignment.ID,
			CounterpartOccurrenceDate: &tuesday,
			ExpiresAt:                 expires,
			CreatedAt:                 base,
		})
		if err != nil {
			t.Fatalf("create swap request: %v", err)
		}

		got, err := repo.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("get swap request: %v", err)
		}
		if got.Type != model.ShiftChangeTypeSwap {
			t.Fatalf("expected type swap, got %q", got.Type)
		}
		if got.State != model.ShiftChangeStatePending {
			t.Fatalf("expected state pending, got %q", got.State)
		}
		if got.CounterpartUserID == nil || *got.CounterpartUserID != counterpart.ID {
			t.Fatalf("expected counterpart user id %d, got %+v", counterpart.ID, got.CounterpartUserID)
		}
		if got.CounterpartAssignmentID == nil || *got.CounterpartAssignmentID != counterpartAssignment.ID {
			t.Fatalf("expected counterpart assignment id %d, got %+v", counterpartAssignment.ID, got.CounterpartAssignmentID)
		}
		if !got.OccurrenceDate.Equal(monday) {
			t.Fatalf("expected occurrence date %v, got %v", monday, got.OccurrenceDate)
		}
		if got.CounterpartOccurrenceDate == nil || !got.CounterpartOccurrenceDate.Equal(tuesday) {
			t.Fatalf("expected counterpart occurrence date %v, got %+v", tuesday, got.CounterpartOccurrenceDate)
		}
		if got.DecidedByUserID != nil {
			t.Fatalf("expected decided_by_user_id NULL, got %+v", got.DecidedByUserID)
		}
		if got.DecidedAt != nil {
			t.Fatalf("expected decided_at NULL, got %+v", got.DecidedAt)
		}
	})

	t.Run("give_direct round-trip", func(t *testing.T) {
		created, err := repo.Create(ctx, CreateShiftChangeRequestParams{
			PublicationID:         publication.ID,
			Type:                  model.ShiftChangeTypeGiveDirect,
			RequesterUserID:       requester.ID,
			RequesterAssignmentID: requesterAssignment.ID,
			OccurrenceDate:        nextMonday,
			CounterpartUserID:     &counterpart.ID,
			// CounterpartAssignmentID nil: the counterpart has no shift yet.
			ExpiresAt: expires,
			CreatedAt: base,
		})
		if err != nil {
			t.Fatalf("create give_direct request: %v", err)
		}

		got, err := repo.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("get give_direct request: %v", err)
		}
		if got.Type != model.ShiftChangeTypeGiveDirect {
			t.Fatalf("expected type give_direct, got %q", got.Type)
		}
		if got.CounterpartUserID == nil || *got.CounterpartUserID != counterpart.ID {
			t.Fatalf("expected counterpart user id %d, got %+v", counterpart.ID, got.CounterpartUserID)
		}
		if got.CounterpartAssignmentID != nil {
			t.Fatalf("expected counterpart_assignment_id NULL, got %+v", got.CounterpartAssignmentID)
		}
	})

	t.Run("give_pool round-trip", func(t *testing.T) {
		created, err := repo.Create(ctx, CreateShiftChangeRequestParams{
			PublicationID:         publication.ID,
			Type:                  model.ShiftChangeTypeGivePool,
			RequesterUserID:       requester.ID,
			RequesterAssignmentID: requesterAssignment.ID,
			OccurrenceDate:        thirdMonday,
			// Both counterpart fields nil for pool.
			ExpiresAt: expires,
			CreatedAt: base,
		})
		if err != nil {
			t.Fatalf("create give_pool request: %v", err)
		}

		got, err := repo.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("get give_pool request: %v", err)
		}
		if got.Type != model.ShiftChangeTypeGivePool {
			t.Fatalf("expected type give_pool, got %q", got.Type)
		}
		if got.CounterpartUserID != nil {
			t.Fatalf("expected counterpart_user_id NULL, got %+v", got.CounterpartUserID)
		}
		if got.CounterpartAssignmentID != nil {
			t.Fatalf("expected counterpart_assignment_id NULL, got %+v", got.CounterpartAssignmentID)
		}
	})

	t.Run("GetByID returns ErrShiftChangeNotFound for unknown id", func(t *testing.T) {
		if _, err := repo.GetByID(ctx, 999_999); !errors.Is(err, ErrShiftChangeNotFound) {
			t.Fatalf("expected ErrShiftChangeNotFound, got %v", err)
		}
	})
}

func TestShiftChangeRepositoryApplySwap(t *testing.T) {
	db := openIntegrationDB(t)
	repo := NewShiftChangeRepository(db)
	ctx := context.Background()

	admin := seedUser(t, db, userSeed{IsAdmin: true})
	position := seedPosition(t, db, positionSeed{})
	template := seedTemplate(t, db, templateSeed{})
	shiftA := seedQualifiedShift(t, db, qualifiedShiftSeed{
		TemplateID: template.ID,
		PositionID: position.ID,
		Weekday:    1,
		StartTime:  "09:00",
		EndTime:    "12:00",
	})
	shiftB := seedQualifiedShift(t, db, qualifiedShiftSeed{
		TemplateID: template.ID,
		PositionID: position.ID,
		Weekday:    2,
		StartTime:  "13:00",
		EndTime:    "17:00",
	})
	publication := seedPublication(t, db, publicationSeed{
		TemplateID: template.ID,
		State:      model.PublicationStatePublished,
		CreatedAt:  testTime(),
	})

	userX := seedUser(t, db, userSeed{})
	userY := seedUser(t, db, userSeed{})

	assignmentX := seedAssignment(t, db, publication.ID, userX.ID, shiftA.SlotID, shiftA.PositionID, testTime())
	assignmentY := seedAssignment(t, db, publication.ID, userY.ID, shiftB.SlotID, shiftB.PositionID, testTime())

	request := seedShiftChangeRequest(
		t,
		db,
		publication.ID,
		model.ShiftChangeTypeSwap,
		userX.ID,
		assignmentX.ID,
		&userY.ID,
		&assignmentY.ID,
		model.ShiftChangeStatePending,
		testTime(),
		testTime().Add(24*time.Hour),
	)

	result, err := repo.ApplySwap(ctx, ApplySwapParams{
		RequestID:                 request.ID,
		PublicationID:             publication.ID,
		RequesterAssignmentID:     assignmentX.ID,
		RequesterUserID:           userX.ID,
		OccurrenceDate:            request.OccurrenceDate,
		CounterpartAssignmentID:   assignmentY.ID,
		CounterpartUserID:         userY.ID,
		CounterpartOccurrenceDate: *request.CounterpartOccurrenceDate,
		DecidedByUserID:           admin.ID,
		Now:                       testTime().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("apply swap: %v", err)
	}
	if result == nil || result.RequesterAssignment == nil || result.CounterpartAssignment == nil {
		t.Fatalf("expected non-nil swap result, got %+v", result)
	}

	if got := fetchAssignmentUserID(t, db, assignmentX.ID); got != userX.ID {
		t.Fatalf("expected assignmentX baseline user_id = %d, got %d", userX.ID, got)
	}
	if got := fetchAssignmentUserID(t, db, assignmentY.ID); got != userY.ID {
		t.Fatalf("expected assignmentY baseline user_id = %d, got %d", userY.ID, got)
	}
	if got := fetchAssignmentOverrideUserID(t, db, assignmentX.ID, request.OccurrenceDate); got != userY.ID {
		t.Fatalf("expected requester override user_id = %d, got %d", userY.ID, got)
	}
	if got := fetchAssignmentOverrideUserID(t, db, assignmentY.ID, *request.CounterpartOccurrenceDate); got != userX.ID {
		t.Fatalf("expected counterpart override user_id = %d, got %d", userX.ID, got)
	}
	if result.RequesterAssignment.UserID != userX.ID {
		t.Fatalf("expected requester baseline user %d in result, got %d", userX.ID, result.RequesterAssignment.UserID)
	}
	if result.CounterpartAssignment.UserID != userY.ID {
		t.Fatalf("expected counterpart baseline user %d in result, got %d", userY.ID, result.CounterpartAssignment.UserID)
	}

	if got := fetchRequestState(t, db, request.ID); got != model.ShiftChangeStateApproved {
		t.Fatalf("expected request state approved, got %q", got)
	}
}

func TestShiftChangeRepositoryApplySwapInvalidated(t *testing.T) {
	db := openIntegrationDB(t)
	repo := NewShiftChangeRepository(db)
	ctx := context.Background()

	admin := seedUser(t, db, userSeed{IsAdmin: true})
	position := seedPosition(t, db, positionSeed{})
	template := seedTemplate(t, db, templateSeed{})
	shiftA := seedQualifiedShift(t, db, qualifiedShiftSeed{
		TemplateID: template.ID,
		PositionID: position.ID,
		Weekday:    1,
		StartTime:  "09:00",
		EndTime:    "12:00",
	})
	shiftB := seedQualifiedShift(t, db, qualifiedShiftSeed{
		TemplateID: template.ID,
		PositionID: position.ID,
		Weekday:    2,
		StartTime:  "13:00",
		EndTime:    "17:00",
	})
	publication := seedPublication(t, db, publicationSeed{
		TemplateID: template.ID,
		State:      model.PublicationStatePublished,
		CreatedAt:  testTime(),
	})

	userX := seedUser(t, db, userSeed{})
	userY := seedUser(t, db, userSeed{})
	intruder := seedUser(t, db, userSeed{})

	assignmentX := seedAssignment(t, db, publication.ID, userX.ID, shiftA.SlotID, shiftA.PositionID, testTime())
	assignmentY := seedAssignment(t, db, publication.ID, userY.ID, shiftB.SlotID, shiftB.PositionID, testTime())

	request := seedShiftChangeRequest(
		t,
		db,
		publication.ID,
		model.ShiftChangeTypeSwap,
		userX.ID,
		assignmentX.ID,
		&userY.ID,
		&assignmentY.ID,
		model.ShiftChangeStatePending,
		testTime(),
		testTime().Add(24*time.Hour),
	)

	// Simulate admin intervention: reassign one side to a different user.
	if _, err := db.ExecContext(
		ctx,
		`UPDATE assignments SET user_id = $2 WHERE id = $1;`,
		assignmentY.ID, intruder.ID,
	); err != nil {
		t.Fatalf("simulate admin intervention: %v", err)
	}

	result, err := repo.ApplySwap(ctx, ApplySwapParams{
		RequestID:                 request.ID,
		PublicationID:             publication.ID,
		RequesterAssignmentID:     assignmentX.ID,
		RequesterUserID:           userX.ID,
		OccurrenceDate:            request.OccurrenceDate,
		CounterpartAssignmentID:   assignmentY.ID,
		CounterpartUserID:         userY.ID,
		CounterpartOccurrenceDate: *request.CounterpartOccurrenceDate,
		DecidedByUserID:           admin.ID,
		Now:                       testTime().Add(time.Hour),
	})
	if !errors.Is(err, ErrShiftChangeAssignmentMiss) {
		t.Fatalf("expected ErrShiftChangeAssignmentMiss, got err=%v result=%+v", err, result)
	}
	if result != nil {
		t.Fatalf("expected nil result on invalidation, got %+v", result)
	}

	// Ensure the transaction rolled back: the request remains pending, and
	// assignmentX still belongs to userX.
	if got := fetchRequestState(t, db, request.ID); got != model.ShiftChangeStatePending {
		t.Fatalf("expected request state still pending, got %q", got)
	}
	if got := fetchAssignmentUserID(t, db, assignmentX.ID); got != userX.ID {
		t.Fatalf("expected assignmentX user_id unchanged (%d), got %d", userX.ID, got)
	}
}

func TestShiftChangeRepositoryMarkExpiredAndInvalidated(t *testing.T) {
	db := openIntegrationDB(t)
	repo := NewShiftChangeRepository(db)
	ctx := context.Background()

	position := seedPosition(t, db, positionSeed{})
	template := seedTemplate(t, db, templateSeed{})
	shift := seedQualifiedShift(t, db, qualifiedShiftSeed{
		TemplateID: template.ID,
		PositionID: position.ID,
	})
	publication := seedPublication(t, db, publicationSeed{
		TemplateID: template.ID,
		State:      model.PublicationStatePublished,
		CreatedAt:  testTime(),
	})
	requester := seedUser(t, db, userSeed{})
	assignment := seedAssignment(t, db, publication.ID, requester.ID, shift.SlotID, shift.PositionID, testTime())

	expireReq := seedShiftChangeRequest(
		t,
		db,
		publication.ID,
		model.ShiftChangeTypeGivePool,
		requester.ID,
		assignment.ID,
		nil,
		nil,
		model.ShiftChangeStatePending,
		testTime(),
		testTime().Add(1*time.Hour),
	)

	if err := repo.MarkExpired(ctx, expireReq.ID, testTime().Add(2*time.Hour)); err != nil {
		t.Fatalf("mark expired: %v", err)
	}
	if got := fetchRequestState(t, db, expireReq.ID); got != model.ShiftChangeStateExpired {
		t.Fatalf("expected state expired, got %q", got)
	}

	invReq := seedShiftChangeRequest(
		t,
		db,
		publication.ID,
		model.ShiftChangeTypeGivePool,
		requester.ID,
		assignment.ID,
		nil,
		nil,
		model.ShiftChangeStatePending,
		testTime(),
		testTime().Add(24*time.Hour),
	)

	if err := repo.MarkInvalidated(ctx, invReq.ID, testTime().Add(3*time.Hour)); err != nil {
		t.Fatalf("mark invalidated: %v", err)
	}
	if got := fetchRequestState(t, db, invReq.ID); got != model.ShiftChangeStateInvalidated {
		t.Fatalf("expected state invalidated, got %q", got)
	}
}

func TestShiftChangeRepositoryInvalidateRequestsForAssignment(t *testing.T) {
	db := openIntegrationDB(t)
	repo := NewShiftChangeRepository(db)
	ctx := context.Background()

	position := seedPosition(t, db, positionSeed{})
	template := seedTemplate(t, db, templateSeed{})
	shiftA := seedQualifiedShift(t, db, qualifiedShiftSeed{
		TemplateID: template.ID,
		PositionID: position.ID,
		Weekday:    1,
	})
	shiftB := seedQualifiedShift(t, db, qualifiedShiftSeed{
		TemplateID: template.ID,
		PositionID: position.ID,
		Weekday:    2,
		StartTime:  "13:00",
		EndTime:    "16:00",
	})
	publication := seedPublication(t, db, publicationSeed{
		TemplateID: template.ID,
		State:      model.PublicationStatePublished,
		CreatedAt:  testTime(),
	})

	requester := seedUser(t, db, userSeed{})
	counterpart := seedUser(t, db, userSeed{})
	otherUser := seedUser(t, db, userSeed{})

	requesterAssignment := seedAssignment(t, db, publication.ID, requester.ID, shiftA.SlotID, shiftA.PositionID, testTime())
	counterpartAssignment := seedAssignment(t, db, publication.ID, counterpart.ID, shiftB.SlotID, shiftB.PositionID, testTime())
	otherAssignment := seedAssignment(t, db, publication.ID, otherUser.ID, shiftA.SlotID, shiftA.PositionID, testTime().Add(time.Minute))

	counterpartUserID := counterpart.ID
	counterpartAssignmentID := counterpartAssignment.ID
	requesterRef := seedShiftChangeRequest(
		t,
		db,
		publication.ID,
		model.ShiftChangeTypeSwap,
		requester.ID,
		requesterAssignment.ID,
		&counterpartUserID,
		&counterpartAssignmentID,
		model.ShiftChangeStatePending,
		testTime(),
		testTime().Add(24*time.Hour),
	)

	requesterUserID := requester.ID
	requesterAssignmentID := requesterAssignment.ID
	counterpartRef := seedShiftChangeRequest(
		t,
		db,
		publication.ID,
		model.ShiftChangeTypeSwap,
		otherUser.ID,
		otherAssignment.ID,
		&requesterUserID,
		&requesterAssignmentID,
		model.ShiftChangeStatePending,
		testTime(),
		testTime().Add(24*time.Hour),
	)

	approvedRef := seedShiftChangeRequest(
		t,
		db,
		publication.ID,
		model.ShiftChangeTypeGivePool,
		requester.ID,
		requesterAssignment.ID,
		nil,
		nil,
		model.ShiftChangeStateApproved,
		testTime(),
		testTime().Add(24*time.Hour),
	)

	unrelatedRef := seedShiftChangeRequest(
		t,
		db,
		publication.ID,
		model.ShiftChangeTypeGivePool,
		otherUser.ID,
		otherAssignment.ID,
		nil,
		nil,
		model.ShiftChangeStatePending,
		testTime(),
		testTime().Add(24*time.Hour),
	)

	ids, err := repo.InvalidateRequestsForAssignment(ctx, requesterAssignment.ID, testTime().Add(time.Hour))
	if err != nil {
		t.Fatalf("invalidate requests for assignment: %v", err)
	}

	if len(ids) != 2 || ids[0] != requesterRef.ID || ids[1] != counterpartRef.ID {
		t.Fatalf("unexpected invalidated ids: %+v", ids)
	}
	if got := fetchRequestState(t, db, requesterRef.ID); got != model.ShiftChangeStateInvalidated {
		t.Fatalf("expected requester-side ref invalidated, got %q", got)
	}
	if got := fetchRequestState(t, db, counterpartRef.ID); got != model.ShiftChangeStateInvalidated {
		t.Fatalf("expected counterpart-side ref invalidated, got %q", got)
	}
	if got := fetchRequestState(t, db, approvedRef.ID); got != model.ShiftChangeStateApproved {
		t.Fatalf("expected approved request unchanged, got %q", got)
	}
	if got := fetchRequestState(t, db, unrelatedRef.ID); got != model.ShiftChangeStatePending {
		t.Fatalf("expected unrelated request unchanged, got %q", got)
	}
}
