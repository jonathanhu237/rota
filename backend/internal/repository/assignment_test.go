//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
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
			Weekday:       1,
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
			Weekday:       1,
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

	t.Run("ListAssignmentBoardEmployees carries submitted slots for target publication only", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		template := seedTemplate(t, db, templateSeed{Name: "Submitted Slots Template"})
		position := seedPosition(t, db, positionSeed{Name: "Submitted Slots Position"})
		firstSlotID := seedTemplateSlot(t, db, template.ID, 1, "09:00", "12:00")
		secondSlotID := seedTemplateSlot(t, db, template.ID, 2, "13:00", "16:00")
		seedTemplateSlotPosition(t, db, firstSlotID, position.ID, 1)
		seedTemplateSlotPosition(t, db, secondSlotID, position.ID, 1)
		publication := seedPublication(t, db, publicationSeed{
			TemplateID:        template.ID,
			State:             model.PublicationStateAssigning,
			SubmissionStartAt: testTime().Add(-4 * time.Hour),
			SubmissionEndAt:   testTime().Add(-2 * time.Hour),
			PlannedActiveFrom: testTime().Add(1 * time.Hour),
			CreatedAt:         testTime().Add(-5 * time.Hour),
		})
		otherPublication := seedPublication(t, db, publicationSeed{
			TemplateID:        template.ID,
			State:             model.PublicationStateEnded,
			SubmissionStartAt: testTime().Add(-14 * 24 * time.Hour),
			SubmissionEndAt:   testTime().Add(-13 * 24 * time.Hour),
			PlannedActiveFrom: testTime().Add(-12 * 24 * time.Hour),
			CreatedAt:         testTime().Add(-15 * 24 * time.Hour),
		})

		submitter := seedUser(t, db, userSeed{Name: "Submitter", Email: "submitter@example.com"})
		nonSubmitter := seedUser(t, db, userSeed{Name: "Non Submitter", Email: "non-submitter@example.com"})
		otherSubmitter := seedUser(t, db, userSeed{Name: "Other Submitter", Email: "other-submitter@example.com"})
		for _, userID := range []int64{submitter.ID, nonSubmitter.ID, otherSubmitter.ID} {
			seedUserPosition(t, db, userID, position.ID)
		}

		seedSubmission(t, db, publication.ID, submitter.ID, firstSlotID, testTime())
		seedSubmission(t, db, publication.ID, submitter.ID, secondSlotID, testTime().Add(time.Minute))
		seedSubmission(t, db, otherPublication.ID, otherSubmitter.ID, firstSlotID, testTime().Add(2*time.Minute))

		employees, err := repo.ListAssignmentBoardEmployees(ctx, publication.ID)
		if err != nil {
			t.Fatalf("list assignment board employees: %v", err)
		}
		submittedSlotsByUser := make(map[int64][]model.SubmittedSlot, len(employees))
		for _, employee := range employees {
			submittedSlotsByUser[employee.UserID] = employee.SubmittedSlots
		}

		wantSubmitterSlots := []model.SubmittedSlot{
			{SlotID: firstSlotID, Weekday: 1},
			{SlotID: secondSlotID, Weekday: 2},
		}
		if !reflect.DeepEqual(submittedSlotsByUser[submitter.ID], wantSubmitterSlots) {
			t.Fatalf("unexpected submitted slots for target submitter: got %+v want %+v", submittedSlotsByUser[submitter.ID], wantSubmitterSlots)
		}
		if got := submittedSlotsByUser[nonSubmitter.ID]; len(got) != 0 {
			t.Fatalf("expected non-submitter to carry no submitted slots, got %+v", got)
		}
		if got := submittedSlotsByUser[otherSubmitter.ID]; len(got) != 0 {
			t.Fatalf("expected other-publication submissions to be excluded, got %+v", got)
		}
	})

	t.Run("DeleteAssignment is idempotent", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		publication, slotID, positionID, _, user := seedAssignmentPrerequisites(t, db)
		assignment := seedAssignment(t, db, publication.ID, user.ID, slotID, positionID, testTime())
		occurrence := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
		if _, err := repo.InsertAssignmentOverride(ctx, InsertAssignmentOverrideParams{
			AssignmentID:   assignment.ID,
			OccurrenceDate: occurrence,
			UserID:         user.ID,
			CreatedAt:      testTime(),
		}); err != nil {
			t.Fatalf("insert first assignment override: %v", err)
		}
		if _, err := repo.InsertAssignmentOverride(ctx, InsertAssignmentOverrideParams{
			AssignmentID:   assignment.ID,
			OccurrenceDate: occurrence.AddDate(0, 0, 7),
			UserID:         user.ID,
			CreatedAt:      testTime(),
		}); err != nil {
			t.Fatalf("insert second assignment override: %v", err)
		}
		count, err := repo.CountAssignmentOverridesByAssignment(ctx, assignment.ID)
		if err != nil {
			t.Fatalf("count assignment overrides: %v", err)
		}
		if count != 2 {
			t.Fatalf("expected 2 assignment overrides before delete, got %d", count)
		}

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
		count, err = repo.CountAssignmentOverridesByAssignment(ctx, assignment.ID)
		if err != nil {
			t.Fatalf("count assignment overrides after delete: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected assignment overrides to cascade on delete, got %d", count)
		}
	})

	t.Run("ReplaceAssignments swaps the full set inside one transaction", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		publication, slotID, positionID, _, firstUser := seedAssignmentPrerequisites(t, db)
		secondUser := seedUser(t, db, userSeed{})
		seedUserPosition(t, db, secondUser.ID, positionID)
		oldAssignment := seedAssignment(t, db, publication.ID, firstUser.ID, slotID, positionID, testTime())

		if err := repo.ReplaceAssignments(ctx, ReplaceAssignmentsParams{
			PublicationID: publication.ID,
			Assignments: []ReplaceAssignmentParams{
				{UserID: secondUser.ID, SlotID: slotID, Weekday: 1, PositionID: positionID},
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
		publication, slotID, positionID, _, user := seedAssignmentPrerequisites(t, db)
		original := seedAssignment(t, db, publication.ID, user.ID, slotID, positionID, testTime())

		err := repo.ReplaceAssignments(ctx, ReplaceAssignmentsParams{
			PublicationID: publication.ID,
			Assignments: []ReplaceAssignmentParams{
				{UserID: 999, SlotID: slotID, Weekday: 1, PositionID: positionID},
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

	t.Run("database rejects assignment when position does not belong to slot", func(t *testing.T) {
		db := openIntegrationDB(t)
		publication, slotID, _, _, user := seedAssignmentPrerequisites(t, db)
		otherPosition := seedPosition(t, db, positionSeed{Name: "Other Position"})

		_, err := db.ExecContext(
			ctx,
			`INSERT INTO assignments (publication_id, user_id, slot_id, weekday, position_id, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
			publication.ID,
			user.ID,
			slotID,
			1,
			otherPosition.ID,
			testTime(),
		)
		if err == nil {
			t.Fatal("expected position-belongs-to-slot rejection")
		}
	})

	t.Run("database rejects duplicate user assignment in the same slot", func(t *testing.T) {
		db := openIntegrationDB(t)
		publication, slotID, positionID, _, user := seedAssignmentPrerequisites(t, db)
		secondPosition := seedPosition(t, db, positionSeed{Name: "Second Assignment Position"})
		seedUserPosition(t, db, user.ID, secondPosition.ID)
		secondSlotPositionID := seedTemplateSlotPosition(t, db, slotID, secondPosition.ID, 1)

		first := seedAssignment(t, db, publication.ID, user.ID, slotID, positionID, testTime())
		if first == nil {
			t.Fatal("expected initial assignment")
		}

		_, err := db.ExecContext(
			ctx,
			`INSERT INTO assignments (publication_id, user_id, slot_id, weekday, position_id, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
			publication.ID,
			user.ID,
			slotID,
			1,
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
		publication, slotID, positionID, _, user := seedAssignmentPrerequisites(t, db)
		assignment := seedAssignment(t, db, publication.ID, user.ID, slotID, positionID, testTime())

		if _, err := db.ExecContext(ctx, `DELETE FROM publications WHERE id = $1`, publication.ID); err != nil {
			t.Fatalf("delete publication: %v", err)
		}
		if assignmentExists(t, db, assignment.ID) {
			t.Fatalf("expected assignment %d to cascade on publication delete", assignment.ID)
		}
	})

	t.Run("assignment cascades on user delete", func(t *testing.T) {
		db := openIntegrationDB(t)
		publication, slotID, positionID, _, user := seedAssignmentPrerequisites(t, db)
		assignment := seedAssignment(t, db, publication.ID, user.ID, slotID, positionID, testTime())

		if _, err := db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, user.ID); err != nil {
			t.Fatalf("delete user: %v", err)
		}
		if assignmentExists(t, db, assignment.ID) {
			t.Fatalf("expected assignment %d to cascade on user delete", assignment.ID)
		}
	})

	t.Run("assignment cascades on slot delete", func(t *testing.T) {
		db := openIntegrationDB(t)
		publication, slotID, positionID, _, user := seedAssignmentPrerequisites(t, db)
		assignment := seedAssignment(t, db, publication.ID, user.ID, slotID, positionID, testTime())

		if _, err := db.ExecContext(ctx, `DELETE FROM template_slots WHERE id = $1`, slotID); err != nil {
			t.Fatalf("delete slot: %v", err)
		}
		if assignmentExists(t, db, assignment.ID) {
			t.Fatalf("expected assignment %d to cascade on slot delete", assignment.ID)
		}
	})
}

func TestListAssignmentCandidatesFiltered(t *testing.T) {
	ctx := context.Background()
	db := openIntegrationDB(t)
	repo := NewPublicationRepository(db)
	publication, slotID, positionID, _, user := seedAssignmentPrerequisites(t, db)
	seedSubmission(t, db, publication.ID, user.ID, slotID, testTime())

	candidates, err := repo.ListAssignmentCandidates(ctx, publication.ID)
	if err != nil {
		t.Fatalf("ListAssignmentCandidates returned error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].UserID != user.ID || candidates[0].PositionID != positionID {
		t.Fatalf("expected active qualified candidate, got %+v", candidates)
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM user_positions WHERE user_id = $1 AND position_id = $2`, user.ID, positionID); err != nil {
		t.Fatalf("delete user position: %v", err)
	}
	candidates, err = repo.ListAssignmentCandidates(ctx, publication.ID)
	if err != nil {
		t.Fatalf("ListAssignmentCandidates after revoked qualification returned error: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected revoked qualification to be filtered, got %+v", candidates)
	}

	seedUserPosition(t, db, user.ID, positionID)
	if _, err := db.ExecContext(ctx, `UPDATE users SET status = 'disabled' WHERE id = $1`, user.ID); err != nil {
		t.Fatalf("disable user: %v", err)
	}
	candidates, err = repo.ListAssignmentCandidates(ctx, publication.ID)
	if err != nil {
		t.Fatalf("ListAssignmentCandidates after disable returned error: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected disabled user to be filtered, got %+v", candidates)
	}
}

func TestLockAndCheckUserStatus(t *testing.T) {
	ctx := context.Background()
	db := openIntegrationDB(t)
	publication, _, _, _, user := seedAssignmentPrerequisites(t, db)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := LockAndCheckUserStatus(ctx, tx, publication.ID, user.ID); err != nil {
		t.Fatalf("expected active user status check to pass, got %v", err)
	}
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		t.Fatalf("rollback active tx: %v", rollbackErr)
	}

	if _, err := db.ExecContext(ctx, `UPDATE users SET status = 'disabled' WHERE id = $1`, user.ID); err != nil {
		t.Fatalf("disable user: %v", err)
	}
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin disabled tx: %v", err)
	}
	err = LockAndCheckUserStatus(ctx, tx, publication.ID, user.ID)
	if !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("expected ErrUserDisabled, got %v", err)
	}
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		t.Fatalf("rollback disabled tx: %v", rollbackErr)
	}
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
