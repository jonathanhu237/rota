//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/lib/pq"
)

func TestAssignmentRepositoryIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateAssignment rejects same user in same slot", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		publication, slotID, positionID, _, user := seedAssignmentPrerequisites(t, db)
		secondPosition := seedPosition(t, db, positionSeed{Name: "Same Slot Alternate Position"})
		seedTemplateSlotPosition(t, db, slotID, secondPosition.ID, 1)

		first, err := repo.CreateAssignment(ctx, CreateAssignmentParams{
			PublicationID: publication.ID,
			UserID:        user.ID,
			SlotID:        slotID,
			PositionID:    positionID,
			CreatedAt:     testTime(),
		})
		if err != nil {
			t.Fatalf("create assignment: %v", err)
		}
		if first.SlotID != slotID || first.PositionID != positionID {
			t.Fatalf("unexpected assignment: %+v", first)
		}

		_, err = repo.CreateAssignment(ctx, CreateAssignmentParams{
			PublicationID: publication.ID,
			UserID:        user.ID,
			SlotID:        slotID,
			PositionID:    secondPosition.ID,
			CreatedAt:     testTime().AddDate(0, 0, 1),
		})
		if !errors.Is(err, ErrAssignmentUserAlreadyInSlot) {
			t.Fatalf("expected ErrAssignmentUserAlreadyInSlot, got %v", err)
		}
		if errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("expected mapped sentinel instead of sql.ErrNoRows")
		}
		assignments, err := repo.ListPublicationAssignments(ctx, publication.ID)
		if err != nil {
			t.Fatalf("list publication assignments: %v", err)
		}
		if len(assignments) != 1 || assignments[0].AssignmentID != first.ID {
			t.Fatalf("expected original assignment to remain untouched, got %+v", assignments)
		}
	})

	t.Run("GetUserByID tolerates NULL password hash", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		user := seedUser(t, db, userSeed{})

		if _, err := db.ExecContext(ctx, `UPDATE users SET password_hash = NULL WHERE id = $1`, user.ID); err != nil {
			t.Fatalf("clear password hash: %v", err)
		}

		got, err := repo.GetUserByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("get user by id: %v", err)
		}
		if got.PasswordHash != "" {
			t.Fatalf("expected empty password hash for NULL DB value, got %q", got.PasswordHash)
		}
		if got.ID != user.ID || got.Status != user.Status {
			t.Fatalf("unexpected user: %+v", got)
		}
	})

	t.Run("DeleteAssignment is idempotent", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		publication, _, _, slotPositionID, user := seedAssignmentPrerequisites(t, db)
		assignment := seedAssignment(t, db, publication.ID, user.ID, slotPositionID, testTime())

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
		publication, slotID, positionID, slotPositionID, firstUser := seedAssignmentPrerequisites(t, db)
		secondUser := seedUser(t, db, userSeed{})
		seedUserPosition(t, db, secondUser.ID, positionID)
		oldAssignment := seedAssignment(t, db, publication.ID, firstUser.ID, slotPositionID, testTime())

		if err := repo.ReplaceAssignments(ctx, ReplaceAssignmentsParams{
			PublicationID: publication.ID,
			Assignments: []ReplaceAssignmentParams{
				{UserID: secondUser.ID, SlotID: slotID, PositionID: positionID},
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
		if assignments[0].AssignmentID == oldAssignment.ID || assignments[0].UserID != secondUser.ID || assignments[0].SlotID != slotID || assignments[0].PositionID != positionID {
			t.Fatalf("unexpected assignments after replace: %+v", assignments)
		}
	})

	t.Run("ReplaceAssignments rolls back when one inserted row is invalid", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		publication, slotID, positionID, slotPositionID, user := seedAssignmentPrerequisites(t, db)
		original := seedAssignment(t, db, publication.ID, user.ID, slotPositionID, testTime())

		err := repo.ReplaceAssignments(ctx, ReplaceAssignmentsParams{
			PublicationID: publication.ID,
			Assignments: []ReplaceAssignmentParams{
				{UserID: 999, SlotID: slotID, PositionID: positionID},
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

	t.Run("ListUserAssignmentsOnWeekdayInPublication returns weekly slots", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		publication, slotID, positionID, slotPositionID, user := seedAssignmentPrerequisites(t, db)
		otherSlotID := seedTemplateSlot(t, db, publication.TemplateID, 2, "10:00", "12:00")
		otherSlotPositionID := seedTemplateSlotPosition(t, db, otherSlotID, positionID, 1)

		seedAssignment(t, db, publication.ID, user.ID, slotPositionID, testTime())
		seedAssignment(t, db, publication.ID, user.ID, otherSlotPositionID, testTime().Add(time.Minute))

		assignments, err := repo.ListUserAssignmentsOnWeekdayInPublication(ctx, publication.ID, user.ID, 1)
		if err != nil {
			t.Fatalf("list weekday assignments: %v", err)
		}
		if len(assignments) != 1 {
			t.Fatalf("expected 1 monday assignment, got %+v", assignments)
		}
		if assignments[0].SlotID != slotID || assignments[0].PositionID != positionID || assignments[0].StartTime != "09:00" || assignments[0].EndTime != "12:00" {
			t.Fatalf("unexpected monday assignment view: %+v", assignments[0])
		}
	})

	t.Run("database rejects assignment when position does not belong to slot", func(t *testing.T) {
		db := openIntegrationDB(t)
		publication, slotID, _, _, user := seedAssignmentPrerequisites(t, db)
		otherPosition := seedPosition(t, db, positionSeed{Name: "Other Position"})

		_, err := db.ExecContext(
			ctx,
			`INSERT INTO assignments (publication_id, user_id, slot_id, position_id, created_at) VALUES ($1, $2, $3, $4, $5)`,
			publication.ID,
			user.ID,
			slotID,
			otherPosition.ID,
			testTime(),
		)
		if err == nil {
			t.Fatal("expected position-belongs-to-slot rejection")
		}
	})

	t.Run("database rejects duplicate user assignment in the same slot", func(t *testing.T) {
		db := openIntegrationDB(t)
		publication, slotID, _, slotPositionID, user := seedAssignmentPrerequisites(t, db)
		secondPosition := seedPosition(t, db, positionSeed{Name: "Second Assignment Position"})
		seedUserPosition(t, db, user.ID, secondPosition.ID)
		secondSlotPositionID := seedTemplateSlotPosition(t, db, slotID, secondPosition.ID, 1)

		first := seedAssignment(t, db, publication.ID, user.ID, slotPositionID, testTime())
		if first == nil {
			t.Fatal("expected initial assignment")
		}

		_, err := db.ExecContext(
			ctx,
			`INSERT INTO assignments (publication_id, user_id, slot_id, position_id, created_at) VALUES ($1, $2, $3, $4, $5)`,
			publication.ID,
			user.ID,
			slotID,
			secondPosition.ID,
			testTime().Add(time.Minute),
		)
		if err == nil {
			t.Fatal("expected unique(publication_id, user_id, slot_id) rejection")
		}
		var pqErr *pq.Error
		if !errors.As(err, &pqErr) || pqErr.Code != "23505" {
			t.Fatalf("expected unique violation, got %v", err)
		}
		if secondSlotPositionID == 0 {
			t.Fatal("expected second slot position to be created")
		}
	})

	t.Run("assignment cascades on publication delete", func(t *testing.T) {
		db := openIntegrationDB(t)
		publication, _, _, slotPositionID, user := seedAssignmentPrerequisites(t, db)
		assignment := seedAssignment(t, db, publication.ID, user.ID, slotPositionID, testTime())

		if _, err := db.ExecContext(ctx, `DELETE FROM publications WHERE id = $1`, publication.ID); err != nil {
			t.Fatalf("delete publication: %v", err)
		}
		if assignmentExists(t, db, assignment.ID) {
			t.Fatalf("expected assignment %d to cascade on publication delete", assignment.ID)
		}
	})

	t.Run("assignment cascades on user delete", func(t *testing.T) {
		db := openIntegrationDB(t)
		publication, _, _, slotPositionID, user := seedAssignmentPrerequisites(t, db)
		assignment := seedAssignment(t, db, publication.ID, user.ID, slotPositionID, testTime())

		if _, err := db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, user.ID); err != nil {
			t.Fatalf("delete user: %v", err)
		}
		if assignmentExists(t, db, assignment.ID) {
			t.Fatalf("expected assignment %d to cascade on user delete", assignment.ID)
		}
	})

	t.Run("assignment cascades on slot delete", func(t *testing.T) {
		db := openIntegrationDB(t)
		publication, slotID, _, slotPositionID, user := seedAssignmentPrerequisites(t, db)
		assignment := seedAssignment(t, db, publication.ID, user.ID, slotPositionID, testTime())

		if _, err := db.ExecContext(ctx, `DELETE FROM template_slots WHERE id = $1`, slotID); err != nil {
			t.Fatalf("delete slot: %v", err)
		}
		if assignmentExists(t, db, assignment.ID) {
			t.Fatalf("expected assignment %d to cascade on slot delete", assignment.ID)
		}
	})
}

func seedAssignmentPrerequisites(
	t testing.TB,
	db *sql.DB,
) (*model.Publication, int64, int64, int64, *model.User) {
	t.Helper()

	template := seedTemplate(t, db, templateSeed{Name: "Assignment Template"})
	position := seedPosition(t, db, positionSeed{Name: "Assignment Position"})
	slotID := seedTemplateSlot(t, db, template.ID, 1, "09:00", "12:00")
	slotPositionID := seedTemplateSlotPosition(t, db, slotID, position.ID, 1)
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

	return publication, slotID, position.ID, slotPositionID, user
}

func assignmentExists(t testing.TB, db *sql.DB, assignmentID int64) bool {
	t.Helper()

	var exists bool
	if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM assignments WHERE id = $1)`, assignmentID).Scan(&exists); err != nil {
		t.Fatalf("query assignment exists: %v", err)
	}
	return exists
}
