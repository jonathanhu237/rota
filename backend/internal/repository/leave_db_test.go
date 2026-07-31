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

func TestLeaveRepositoryIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("Insert and GetByID return leave with joined request", func(t *testing.T) {
		db := openIntegrationDB(t)
		leaveRepo := NewLeaveRepository(db)
		shiftRepo := NewShiftChangeRepository(db)
		publication, requester, assignment := seedLeavePrerequisites(t, db)
		now := testTime()

		var leave *model.Leave
		var request *model.ShiftChangeRequest
		err := leaveRepo.WithTx(ctx, func(tx *sql.Tx) error {
			var err error
			request, err = shiftRepo.CreateTx(ctx, tx, CreateShiftChangeRequestParams{
				PublicationID:         publication.ID,
				Type:                  model.ShiftChangeTypeGivePool,
				RequesterUserID:       requester.ID,
				RequesterAssignmentID: assignment.ID,
				OccurrenceDate:        now.AddDate(0, 0, 10),
				ExpiresAt:             now.Add(24 * time.Hour),
				CreatedAt:             now,
			})
			if err != nil {
				return err
			}
			leave, err = leaveRepo.Insert(ctx, tx, InsertLeaveParams{
				UserID:               requester.ID,
				PublicationID:        publication.ID,
				ShiftChangeRequestID: request.ID,
				Category:             model.LeaveCategoryPersonal,
				Reason:               "exam",
				CreatedAt:            now,
				UpdatedAt:            now,
			})
			if err != nil {
				return err
			}
			_, err = shiftRepo.SetLeaveIDTx(ctx, tx, request.ID, leave.ID)
			return err
		})
		if err != nil {
			t.Fatalf("create leave graph: %v", err)
		}

		found, joined, err := leaveRepo.GetByID(ctx, leave.ID)
		if err != nil {
			t.Fatalf("get leave: %v", err)
		}
		if found.ID != leave.ID || found.Category != model.LeaveCategoryPersonal || found.Reason != "exam" {
			t.Fatalf("unexpected leave: %+v", found)
		}
		if joined.ID != request.ID || joined.LeaveID == nil || *joined.LeaveID != leave.ID {
			t.Fatalf("unexpected joined request: %+v", joined)
		}
	})

	t.Run("GetByID maps missing row", func(t *testing.T) {
		db := openIntegrationDB(t)
		_, _, err := NewLeaveRepository(db).GetByID(ctx, 999)
		if !errors.Is(err, ErrLeaveNotFound) {
			t.Fatalf("expected ErrLeaveNotFound, got %v", err)
		}
	})

	t.Run("ListForUser and ListForPublication filter rows", func(t *testing.T) {
		db := openIntegrationDB(t)
		leaveRepo := NewLeaveRepository(db)
		leave := seedLeave(t, db, model.LeaveCategorySick)
		if _, err := db.ExecContext(ctx, `UPDATE publications SET state = 'ENDED' WHERE id = $1;`, leave.PublicationID); err != nil {
			t.Fatalf("end first publication: %v", err)
		}
		other := seedLeave(t, db, model.LeaveCategoryBereavement)

		userRows, err := leaveRepo.ListForUser(ctx, leave.UserID, 1, 25)
		if err != nil {
			t.Fatalf("list for user: %v", err)
		}
		if len(userRows) != 1 || userRows[0].Leave.ID != leave.ID {
			t.Fatalf("expected one user leave %d, got %+v", leave.ID, userRows)
		}

		publicationRows, err := leaveRepo.ListForPublication(ctx, other.PublicationID, 1, 25)
		if err != nil {
			t.Fatalf("list for publication: %v", err)
		}
		if len(publicationRows) != 1 || publicationRows[0].Leave.ID != other.ID {
			t.Fatalf("expected one publication leave %d, got %+v", other.ID, publicationRows)
		}
	})

	t.Run("database rejects unknown category and duplicate request", func(t *testing.T) {
		db := openIntegrationDB(t)
		leaveRepo := NewLeaveRepository(db)
		publication, requester, assignment := seedLeavePrerequisites(t, db)
		req := seedShiftChangeRequest(
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
			testTime().Add(time.Hour),
		)

		err := leaveRepo.WithTx(ctx, func(tx *sql.Tx) error {
			_, err := leaveRepo.Insert(ctx, tx, InsertLeaveParams{
				UserID:               requester.ID,
				PublicationID:        publication.ID,
				ShiftChangeRequestID: req.ID,
				Category:             "vacation",
				CreatedAt:            testTime(),
				UpdatedAt:            testTime(),
			})
			return err
		})
		if err == nil {
			t.Fatalf("expected unknown category insert to fail")
		}

		first := seedLeaveForRequest(t, db, req, model.LeaveCategorySick)
		err = leaveRepo.WithTx(ctx, func(tx *sql.Tx) error {
			_, err := leaveRepo.Insert(ctx, tx, InsertLeaveParams{
				UserID:               requester.ID,
				PublicationID:        publication.ID,
				ShiftChangeRequestID: first.ShiftChangeRequestID,
				Category:             model.LeaveCategoryPersonal,
				CreatedAt:            testTime(),
				UpdatedAt:            testTime(),
			})
			return err
		})
		if err == nil {
			t.Fatalf("expected duplicate shift_change_request_id insert to fail")
		}
	})

	t.Run("active occurrence uniqueness rejects sequential duplicate and exposes key", func(t *testing.T) {
		db := openIntegrationDB(t)
		leaveRepo := NewLeaveRepository(db)
		shiftRepo := NewShiftChangeRepository(db)
		publication, requester, assignment := seedLeavePrerequisites(t, db)
		occurrence := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)

		first, _, err := createLeaveWorkflow(
			ctx,
			leaveRepo,
			shiftRepo,
			publication.ID,
			requester.ID,
			assignment.ID,
			occurrence,
			testTime(),
		)
		if err != nil {
			t.Fatalf("create first workflow: %v", err)
		}

		if _, _, err := createLeaveWorkflow(
			ctx,
			leaveRepo,
			shiftRepo,
			publication.ID,
			requester.ID,
			assignment.ID,
			occurrence,
			testTime().Add(time.Minute),
		); !errors.Is(err, ErrLeaveAlreadyExists) {
			t.Fatalf("expected ErrLeaveAlreadyExists, got %v", err)
		}

		keys, err := leaveRepo.ListActiveOccurrenceKeys(ctx, requester.ID, publication.ID)
		if err != nil {
			t.Fatalf("list active occurrence keys: %v", err)
		}
		if len(keys) != 1 ||
			keys[0].AssignmentID != assignment.ID ||
			!keys[0].OccurrenceDate.Equal(occurrence) {
			t.Fatalf("unexpected active keys: %+v", keys)
		}

		var leaveCount, requestCount int
		if err := db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM leaves WHERE user_id = $1 AND publication_id = $2;`,
			requester.ID,
			publication.ID,
		).Scan(&leaveCount); err != nil {
			t.Fatalf("count leaves: %v", err)
		}
		if err := db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM shift_change_requests
			 WHERE requester_user_id = $1 AND requester_assignment_id = $2 AND occurrence_date = $3;`,
			requester.ID,
			assignment.ID,
			occurrence,
		).Scan(&requestCount); err != nil {
			t.Fatalf("count requests: %v", err)
		}
		if leaveCount != 1 || requestCount != 1 || first.ID == 0 {
			t.Fatalf("expected one committed workflow, leaves=%d requests=%d first=%+v", leaveCount, requestCount, first)
		}
	})

	t.Run("terminal workflow permits resubmission", func(t *testing.T) {
		db := openIntegrationDB(t)
		leaveRepo := NewLeaveRepository(db)
		shiftRepo := NewShiftChangeRepository(db)
		publication, requester, assignment := seedLeavePrerequisites(t, db)
		occurrence := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)

		_, firstRequest, err := createLeaveWorkflow(
			ctx,
			leaveRepo,
			shiftRepo,
			publication.ID,
			requester.ID,
			assignment.ID,
			occurrence,
			testTime(),
		)
		if err != nil {
			t.Fatalf("create first workflow: %v", err)
		}
		if _, err := db.ExecContext(
			ctx,
			`UPDATE shift_change_requests
			 SET state = 'cancelled', decided_at = $1
			 WHERE id = $2;`,
			testTime().Add(time.Minute),
			firstRequest.ID,
		); err != nil {
			t.Fatalf("cancel first workflow: %v", err)
		}

		second, _, err := createLeaveWorkflow(
			ctx,
			leaveRepo,
			shiftRepo,
			publication.ID,
			requester.ID,
			assignment.ID,
			occurrence,
			testTime().Add(2*time.Minute),
		)
		if err != nil {
			t.Fatalf("resubmit terminal workflow: %v", err)
		}
		if second.ID == 0 {
			t.Fatalf("expected resubmitted leave")
		}
	})

	t.Run("concurrent active occurrence creation commits exactly one workflow", func(t *testing.T) {
		db := openIntegrationDB(t)
		leaveRepo := NewLeaveRepository(db)
		shiftRepo := NewShiftChangeRepository(db)
		publication, requester, assignment := seedLeavePrerequisites(t, db)
		occurrence := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)

		ready := sync.WaitGroup{}
		ready.Add(2)
		release := make(chan struct{})
		results := make(chan error, 2)
		for i := 0; i < 2; i++ {
			createdAt := testTime().Add(time.Duration(i) * time.Second)
			go func() {
				err := leaveRepo.WithTx(ctx, func(tx *sql.Tx) error {
					request, err := shiftRepo.CreateTx(ctx, tx, CreateShiftChangeRequestParams{
						PublicationID:         publication.ID,
						Type:                  model.ShiftChangeTypeGivePool,
						RequesterUserID:       requester.ID,
						RequesterAssignmentID: assignment.ID,
						OccurrenceDate:        occurrence,
						ExpiresAt:             occurrence.Add(9 * time.Hour),
						CreatedAt:             createdAt,
					})
					if err != nil {
						return err
					}
					leave, err := leaveRepo.Insert(ctx, tx, InsertLeaveParams{
						UserID:               requester.ID,
						PublicationID:        publication.ID,
						ShiftChangeRequestID: request.ID,
						Category:             model.LeaveCategoryPersonal,
						CreatedAt:            createdAt,
						UpdatedAt:            createdAt,
					})
					if err != nil {
						return err
					}
					ready.Done()
					<-release
					_, err = shiftRepo.SetLeaveIDTx(ctx, tx, request.ID, leave.ID)
					return err
				})
				results <- err
			}()
		}

		ready.Wait()
		close(release)
		errs := []error{<-results, <-results}
		successes, duplicates := 0, 0
		for _, err := range errs {
			switch {
			case err == nil:
				successes++
			case errors.Is(err, ErrLeaveAlreadyExists):
				duplicates++
			default:
				t.Fatalf("unexpected concurrent error: %v", err)
			}
		}
		if successes != 1 || duplicates != 1 {
			t.Fatalf("expected one success and one duplicate, got errors=%v", errs)
		}

		var leaveCount, requestCount int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM leaves;`).Scan(&leaveCount); err != nil {
			t.Fatalf("count leaves: %v", err)
		}
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM shift_change_requests;`).Scan(&requestCount); err != nil {
			t.Fatalf("count requests: %v", err)
		}
		if leaveCount != 1 || requestCount != 1 {
			t.Fatalf("expected one committed graph, leaves=%d requests=%d", leaveCount, requestCount)
		}
	})
}

func TestLeavePoolRepositoryIntegration(t *testing.T) {
	ctx := context.Background()
	db := openIntegrationDB(t)
	leaveRepo := NewLeaveRepository(db)
	publication, alice, assignment := seedLeavePrerequisites(t, db)
	bob := seedUser(t, db, userSeed{Name: "Bob"})
	carol := seedUser(t, db, userSeed{Name: "Carol"})
	admin := seedUser(t, db, userSeed{Name: "Admin", IsAdmin: true})
	seedUserPosition(t, db, bob.ID, assignment.PositionID)
	now := testTime()

	publicReq := seedShiftChangeRequest(
		t,
		db,
		publication.ID,
		model.ShiftChangeTypeGivePool,
		alice.ID,
		assignment.ID,
		nil,
		nil,
		model.ShiftChangeStatePending,
		now.Add(-2*time.Hour),
		now.Add(48*time.Hour),
	)
	setShiftChangeOccurrenceDate(t, db, publicReq, time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC))
	publicLeave := seedLeaveForRequest(t, db, publicReq, model.LeaveCategorySick)

	directReq := seedShiftChangeRequest(
		t,
		db,
		publication.ID,
		model.ShiftChangeTypeGiveDirect,
		alice.ID,
		assignment.ID,
		&bob.ID,
		nil,
		model.ShiftChangeStatePending,
		now.Add(-time.Hour),
		now.Add(24*time.Hour),
	)
	directLeave := seedLeaveForRequest(t, db, directReq, model.LeaveCategoryPersonal)

	rows, total, err := leaveRepo.ListPool(ctx, ListLeavePoolParams{
		ViewerUserID: bob.ID,
		State:        model.LeavePoolStatePending,
		Offset:       0,
		Limit:        20,
	})
	if err != nil {
		t.Fatalf("list pool for counterpart: %v", err)
	}
	if total != 2 || len(rows) != 2 {
		t.Fatalf("expected Bob to see public and direct leaves, total=%d rows=%+v", total, rows)
	}
	if rows[0].Leave.ID != directLeave.ID || rows[1].Leave.ID != publicLeave.ID {
		t.Fatalf("expected pending rows sorted by occurrence start, got %+v", rows)
	}
	if rows[0].RequesterName != alice.Name || rows[0].CounterpartName == nil || *rows[0].CounterpartName != bob.Name {
		t.Fatalf("expected display names on direct row, got %+v", rows[0])
	}
	if rows[0].Shift == nil || rows[0].Shift.PositionID != assignment.PositionID {
		t.Fatalf("expected shift context, got %+v", rows[0].Shift)
	}
	if rows[0].Shift.StartTime != "09:00" || rows[0].Shift.EndTime != "12:00" {
		t.Fatalf("expected formatted shift time, got %s-%s", rows[0].Shift.StartTime, rows[0].Shift.EndTime)
	}
	wantOccurrenceEnd := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	if !rows[0].Shift.OccurrenceEnd.Equal(wantOccurrenceEnd) {
		t.Fatalf("expected occurrence end %s, got %s", wantOccurrenceEnd, rows[0].Shift.OccurrenceEnd)
	}

	rows, total, err = leaveRepo.ListPool(ctx, ListLeavePoolParams{
		ViewerUserID: carol.ID,
		State:        model.LeavePoolStatePending,
		Offset:       0,
		Limit:        20,
	})
	if err != nil {
		t.Fatalf("list pool for unrelated employee: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].Leave.ID != publicLeave.ID {
		t.Fatalf("expected Carol to see public leave only, total=%d rows=%+v", total, rows)
	}

	_, total, err = leaveRepo.ListPool(ctx, ListLeavePoolParams{
		ViewerUserID:  admin.ID,
		ViewerIsAdmin: true,
		State:         model.LeavePoolStatePending,
		Offset:        0,
		Limit:         20,
	})
	if err != nil {
		t.Fatalf("list pool for admin: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected admin to see all pending leaves, got %d", total)
	}

	approvedReq := seedShiftChangeRequest(
		t,
		db,
		publication.ID,
		model.ShiftChangeTypeGivePool,
		alice.ID,
		assignment.ID,
		nil,
		nil,
		model.ShiftChangeStateApproved,
		now.Add(-3*time.Hour),
		now.Add(72*time.Hour),
	)
	setShiftChangeOccurrenceDate(t, db, approvedReq, time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC))
	approvedLeave := seedLeaveForRequest(t, db, approvedReq, model.LeaveCategoryBereavement)
	if _, err := db.ExecContext(
		ctx,
		`UPDATE shift_change_requests SET decided_by_user_id = $1, decided_at = $2 WHERE id = $3;`,
		bob.ID,
		now,
		approvedReq.ID,
	); err != nil {
		t.Fatalf("mark approved decided_by: %v", err)
	}
	cancelledReq := seedShiftChangeRequest(
		t,
		db,
		publication.ID,
		model.ShiftChangeTypeGivePool,
		alice.ID,
		assignment.ID,
		nil,
		nil,
		model.ShiftChangeStateCancelled,
		now.Add(-4*time.Hour),
		now.Add(96*time.Hour),
	)
	cancelledLeave := seedLeaveForRequest(t, db, cancelledReq, model.LeaveCategorySick)
	rejectedReq := seedShiftChangeRequest(
		t,
		db,
		publication.ID,
		model.ShiftChangeTypeGivePool,
		alice.ID,
		assignment.ID,
		nil,
		nil,
		model.ShiftChangeStateRejected,
		now.Add(-5*time.Hour),
		now.Add(120*time.Hour),
	)
	rejectedLeave := seedLeaveForRequest(t, db, rejectedReq, model.LeaveCategoryPersonal)
	expiredPendingReq := seedShiftChangeRequest(
		t,
		db,
		publication.ID,
		model.ShiftChangeTypeGivePool,
		alice.ID,
		assignment.ID,
		nil,
		nil,
		model.ShiftChangeStatePending,
		now.Add(-6*time.Hour),
		now.Add(-time.Hour),
	)
	setShiftChangeOccurrenceDate(t, db, expiredPendingReq, time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC))
	expiredPendingLeave := seedLeaveForRequest(t, db, expiredPendingReq, model.LeaveCategoryPersonal)

	stateCases := []struct {
		name  string
		state model.LeavePoolStateFilter
		want  map[int64]struct{}
	}{
		{name: "completed", state: model.LeavePoolStateCompleted, want: map[int64]struct{}{approvedLeave.ID: {}}},
		{name: "cancelled", state: model.LeavePoolStateCancelled, want: map[int64]struct{}{cancelledLeave.ID: {}}},
		{name: "failed", state: model.LeavePoolStateFailed, want: map[int64]struct{}{
			rejectedLeave.ID:       {},
			expiredPendingLeave.ID: {},
		}},
	}
	for _, tc := range stateCases {
		t.Run(tc.name, func(t *testing.T) {
			rows, total, err := leaveRepo.ListPool(ctx, ListLeavePoolParams{
				ViewerUserID: bob.ID,
				State:        tc.state,
				Now:          now,
				Offset:       0,
				Limit:        20,
			})
			if err != nil {
				t.Fatalf("list state %s: %v", tc.state, err)
			}
			if total != len(tc.want) || len(rows) != len(tc.want) {
				t.Fatalf("state %s total=%d rows=%+v", tc.state, total, rows)
			}
			for _, row := range rows {
				if _, ok := tc.want[row.Leave.ID]; !ok {
					t.Fatalf("unexpected leave %d for state %s", row.Leave.ID, tc.state)
				}
			}
		})
	}

	allRows, total, err := leaveRepo.ListPool(ctx, ListLeavePoolParams{
		ViewerUserID: bob.ID,
		State:        model.LeavePoolStateAll,
		Now:          now,
		Offset:       0,
		Limit:        20,
	})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if total != 6 || len(allRows) != 6 {
		t.Fatalf("expected all visible leaves, total=%d rows=%+v", total, allRows)
	}
	if allRows[0].Leave.ID != directLeave.ID || allRows[1].Leave.ID != publicLeave.ID {
		t.Fatalf("expected all sort to place pending rows first by urgency, got %+v", allRows[:2])
	}
}

func seedLeavePrerequisites(
	t testing.TB,
	db *sql.DB,
) (*model.Publication, *model.User, *model.Assignment) {
	t.Helper()

	template, shift, position := seedPublicationPrerequisites(t, db)
	user := seedUser(t, db, userSeed{})
	seedUserPosition(t, db, user.ID, position.ID)
	publication := seedPublication(t, db, publicationSeed{
		TemplateID:         template.ID,
		State:              model.PublicationStateActive,
		SubmissionStartAt:  testTime().Add(-5 * time.Hour),
		SubmissionEndAt:    testTime().Add(-4 * time.Hour),
		PlannedActiveFrom:  testTime().Add(-3 * time.Hour),
		PlannedActiveUntil: testTime().AddDate(0, 2, 0),
		CreatedAt:          testTime().Add(-6 * time.Hour),
	})
	assignment := seedAssignment(t, db, publication.ID, user.ID, shift.SlotID, shift.PositionID, testTime())
	return publication, user, assignment
}

func seedLeave(t testing.TB, db *sql.DB, category model.LeaveCategory) *model.Leave {
	t.Helper()

	publication, requester, assignment := seedLeavePrerequisites(t, db)
	req := seedShiftChangeRequest(
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
		testTime().Add(time.Hour),
	)
	return seedLeaveForRequest(t, db, req, category)
}

func seedLeaveForRequest(
	t testing.TB,
	db *sql.DB,
	req *model.ShiftChangeRequest,
	category model.LeaveCategory,
) *model.Leave {
	t.Helper()

	leaveRepo := NewLeaveRepository(db)
	shiftRepo := NewShiftChangeRepository(db)
	var leave *model.Leave
	err := leaveRepo.WithTx(context.Background(), func(tx *sql.Tx) error {
		var err error
		leave, err = leaveRepo.Insert(context.Background(), tx, InsertLeaveParams{
			UserID:               req.RequesterUserID,
			PublicationID:        req.PublicationID,
			ShiftChangeRequestID: req.ID,
			Category:             category,
			Reason:               "",
			CreatedAt:            req.CreatedAt,
			UpdatedAt:            req.CreatedAt,
		})
		if err != nil {
			return err
		}
		_, err = shiftRepo.SetLeaveIDTx(context.Background(), tx, req.ID, leave.ID)
		return err
	})
	if err != nil {
		t.Fatalf("seed leave: %v", err)
	}
	return leave
}

func createLeaveWorkflow(
	ctx context.Context,
	leaveRepo *LeaveRepository,
	shiftRepo *ShiftChangeRepository,
	publicationID int64,
	requesterUserID int64,
	requesterAssignmentID int64,
	occurrenceDate time.Time,
	createdAt time.Time,
) (*model.Leave, *model.ShiftChangeRequest, error) {
	var leave *model.Leave
	var request *model.ShiftChangeRequest
	err := leaveRepo.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		request, err = shiftRepo.CreateTx(ctx, tx, CreateShiftChangeRequestParams{
			PublicationID:         publicationID,
			Type:                  model.ShiftChangeTypeGivePool,
			RequesterUserID:       requesterUserID,
			RequesterAssignmentID: requesterAssignmentID,
			OccurrenceDate:        occurrenceDate,
			ExpiresAt:             occurrenceDate.Add(9 * time.Hour),
			CreatedAt:             createdAt,
		})
		if err != nil {
			return err
		}
		leave, err = leaveRepo.Insert(ctx, tx, InsertLeaveParams{
			UserID:               requesterUserID,
			PublicationID:        publicationID,
			ShiftChangeRequestID: request.ID,
			Category:             model.LeaveCategoryPersonal,
			CreatedAt:            createdAt,
			UpdatedAt:            createdAt,
		})
		if err != nil {
			return err
		}
		request, err = shiftRepo.SetLeaveIDTx(ctx, tx, request.ID, leave.ID)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return leave, request, nil
}

func setShiftChangeOccurrenceDate(
	t testing.TB,
	db *sql.DB,
	request *model.ShiftChangeRequest,
	occurrenceDate time.Time,
) {
	t.Helper()
	if _, err := db.ExecContext(
		context.Background(),
		`UPDATE shift_change_requests SET occurrence_date = $1 WHERE id = $2;`,
		occurrenceDate,
		request.ID,
	); err != nil {
		t.Fatalf("set shift-change occurrence date: %v", err)
	}
	request.OccurrenceDate = occurrenceDate
}
