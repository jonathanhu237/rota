//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

func TestAssignmentRepositoryIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateAssignment is idempotent on duplicate keys", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		publication, shift, user := seedAssignmentPrerequisites(t, db)

		first, err := repo.CreateAssignment(ctx, CreateAssignmentParams{
			PublicationID:   publication.ID,
			UserID:          user.ID,
			TemplateShiftID: shift.ID,
			CreatedAt:       testTime(),
		})
		if err != nil {
			t.Fatalf("create assignment: %v", err)
		}

		second, err := repo.CreateAssignment(ctx, CreateAssignmentParams{
			PublicationID:   publication.ID,
			UserID:          user.ID,
			TemplateShiftID: shift.ID,
			CreatedAt:       testTime().AddDate(0, 0, 1),
		})
		if err != nil {
			t.Fatalf("create duplicate assignment: %v", err)
		}
		if second.ID != first.ID {
			t.Fatalf("expected idempotent assignment ID %d, got %d", first.ID, second.ID)
		}
	})

	t.Run("DeleteAssignment is idempotent", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		publication, shift, user := seedAssignmentPrerequisites(t, db)
		assignment := seedAssignment(t, db, publication.ID, user.ID, shift.ID, testTime())

		if err := repo.DeleteAssignment(ctx, DeleteAssignmentParams{
			PublicationID: publication.ID,
			AssignmentID:  assignment.ID,
		}); err != nil {
			t.Fatalf("delete assignment: %v", err)
		}

		if err := repo.DeleteAssignment(ctx, DeleteAssignmentParams{
			PublicationID: publication.ID,
			AssignmentID:  assignment.ID,
		}); err != nil {
			t.Fatalf("delete assignment second time: %v", err)
		}
	})

	t.Run("ReplaceAssignments swaps the full set inside one transaction", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		publication, shift, firstUser := seedAssignmentPrerequisites(t, db)
		secondUser := seedUser(t, db, userSeed{})
		seedUserPosition(t, db, secondUser.ID, shift.PositionID)
		oldAssignment := seedAssignment(t, db, publication.ID, firstUser.ID, shift.ID, testTime())

		if err := repo.ReplaceAssignments(ctx, ReplaceAssignmentsParams{
			PublicationID: publication.ID,
			Assignments: []ReplaceAssignmentParams{
				{UserID: secondUser.ID, TemplateShiftID: shift.ID},
			},
			CreatedAt: testTime().Add(5 * time.Minute),
		}); err != nil {
			t.Fatalf("replace assignments: %v", err)
		}

		assignments, err := repo.ListPublicationAssignments(ctx, publication.ID)
		if err != nil {
			t.Fatalf("list publication assignments: %v", err)
		}
		if len(assignments) != 1 {
			t.Fatalf("expected 1 assignment after replace, got %d", len(assignments))
		}
		if assignments[0].AssignmentID == oldAssignment.ID || assignments[0].UserID != secondUser.ID {
			t.Fatalf("unexpected assignments after replace: %+v", assignments)
		}
	})

	t.Run("ReplaceAssignments rolls back when one inserted row is invalid", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		publication, shift, user := seedAssignmentPrerequisites(t, db)
		original := seedAssignment(t, db, publication.ID, user.ID, shift.ID, testTime())

		err := repo.ReplaceAssignments(ctx, ReplaceAssignmentsParams{
			PublicationID: publication.ID,
			Assignments: []ReplaceAssignmentParams{
				{UserID: 999, TemplateShiftID: shift.ID},
			},
			CreatedAt: testTime(),
		})
		if err == nil {
			t.Fatalf("expected replace assignments to fail for missing user")
		}

		assignments, listErr := repo.ListPublicationAssignments(ctx, publication.ID)
		if listErr != nil {
			t.Fatalf("list publication assignments after rollback: %v", listErr)
		}
		if len(assignments) != 1 || assignments[0].AssignmentID != original.ID {
			t.Fatalf("expected rollback to preserve original assignment, got %+v", assignments)
		}
	})
}

func seedAssignmentPrerequisites(
	t testing.TB,
	db *sql.DB,
) (*model.Publication, *model.TemplateShift, *model.User) {
	t.Helper()

	template, shift, position := seedPublicationPrerequisites(t, db)
	publication := seedPublication(t, db, publicationSeed{
		TemplateID:        template.ID,
		State:             model.PublicationStateAssigning,
		SubmissionStartAt: testTime().Add(-4 * time.Hour),
		SubmissionEndAt:   testTime().Add(-2 * time.Hour),
		PlannedActiveFrom: testTime().Add(1 * time.Hour),
		CreatedAt:         testTime().Add(-5 * time.Hour),
	})
	user := seedUser(t, db, userSeed{})
	seedUserPosition(t, db, user.ID, position.ID)

	return publication, shift, user
}
