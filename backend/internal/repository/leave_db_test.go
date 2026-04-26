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
