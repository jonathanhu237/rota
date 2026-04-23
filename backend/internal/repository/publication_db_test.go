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

func TestPublicationRepositoryIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("CreatePublication locks the template and persists the publication", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		template, _, _ := seedPublicationPrerequisites(t, db)
		createdAt := testTime()

		publication, err := repo.CreatePublication(ctx, CreatePublicationParams{
			TemplateID:        template.ID,
			Name:              "April Week 1",
			State:             model.PublicationStateDraft,
			SubmissionStartAt: createdAt.Add(1 * time.Hour),
			SubmissionEndAt:   createdAt.Add(2 * time.Hour),
			PlannedActiveFrom: createdAt.Add(3 * time.Hour),
			CreatedAt:         createdAt,
		})
		if err != nil {
			t.Fatalf("create publication: %v", err)
		}
		if publication.TemplateName != template.Name {
			t.Fatalf("expected template name %q, got %q", template.Name, publication.TemplateName)
		}

		lockedTemplate, err := NewTemplateRepository(db).GetByID(ctx, template.ID)
		if err != nil {
			t.Fatalf("load template after publication create: %v", err)
		}
		if !lockedTemplate.IsLocked {
			t.Fatalf("expected template to be locked after publication creation")
		}
	})

	t.Run("CreatePublication maps unique index conflicts and rolls back template locking", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		firstTemplate, _, _ := seedPublicationPrerequisites(t, db)
		secondTemplate, _, _ := seedPublicationPrerequisites(t, db)

		seedPublication(t, db, publicationSeed{
			TemplateID:        firstTemplate.ID,
			Name:              "Existing",
			State:             model.PublicationStateDraft,
			SubmissionStartAt: testTime().Add(1 * time.Hour),
			SubmissionEndAt:   testTime().Add(2 * time.Hour),
			PlannedActiveFrom: testTime().Add(3 * time.Hour),
			CreatedAt:         testTime(),
		})

		_, err := repo.CreatePublication(ctx, CreatePublicationParams{
			TemplateID:        secondTemplate.ID,
			Name:              "Conflicting",
			State:             model.PublicationStateCollecting,
			SubmissionStartAt: testTime().Add(4 * time.Hour),
			SubmissionEndAt:   testTime().Add(5 * time.Hour),
			PlannedActiveFrom: testTime().Add(6 * time.Hour),
			CreatedAt:         testTime().Add(30 * time.Minute),
		})
		if !errors.Is(err, ErrPublicationAlreadyExists) {
			t.Fatalf("expected ErrPublicationAlreadyExists, got %v", err)
		}

		templateAfter, err := NewTemplateRepository(db).GetByID(ctx, secondTemplate.ID)
		if err != nil {
			t.Fatalf("load template after conflict: %v", err)
		}
		if templateAfter.IsLocked {
			t.Fatalf("expected conflicting create to roll back template lock")
		}
	})

	t.Run("CreatePublication maps invalid publication windows", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		template, _, _ := seedPublicationPrerequisites(t, db)

		_, err := repo.CreatePublication(ctx, CreatePublicationParams{
			TemplateID:        template.ID,
			Name:              "Invalid Window",
			State:             model.PublicationStateDraft,
			SubmissionStartAt: testTime().Add(3 * time.Hour),
			SubmissionEndAt:   testTime().Add(2 * time.Hour),
			PlannedActiveFrom: testTime().Add(4 * time.Hour),
			CreatedAt:         testTime(),
		})
		if !errors.Is(err, ErrInvalidPublicationWindow) {
			t.Fatalf("expected ErrInvalidPublicationWindow, got %v", err)
		}
	})

	t.Run("ActivatePublication only succeeds from PUBLISHED", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		template, _, _ := seedPublicationPrerequisites(t, db)
		published := seedPublication(t, db, publicationSeed{
			TemplateID:        template.ID,
			State:             model.PublicationStatePublished,
			SubmissionStartAt: testTime().Add(1 * time.Hour),
			SubmissionEndAt:   testTime().Add(2 * time.Hour),
			PlannedActiveFrom: testTime().Add(3 * time.Hour),
			CreatedAt:         testTime(),
		})

		activated, err := repo.ActivatePublication(ctx, ActivatePublicationParams{
			ID:  published.ID,
			Now: testTime().Add(4 * time.Hour),
		})
		if err != nil {
			t.Fatalf("activate publication: %v", err)
		}
		if activated.Publication.State != model.PublicationStateActive {
			t.Fatalf("expected ACTIVE state, got %q", activated.Publication.State)
		}

		// End the now-active publication so we can seed a second one without
		// tripping the single-non-ENDED invariant.
		if _, err := db.ExecContext(ctx, `UPDATE publications SET state = 'ENDED', ended_at = NOW() WHERE id = $1;`, published.ID); err != nil {
			t.Fatalf("end first publication: %v", err)
		}

		assigningTemplate, _, _ := seedPublicationPrerequisites(t, db)
		assigning := seedPublication(t, db, publicationSeed{
			TemplateID:        assigningTemplate.ID,
			State:             model.PublicationStateAssigning,
			SubmissionStartAt: testTime().Add(-3 * time.Hour),
			SubmissionEndAt:   testTime().Add(-2 * time.Hour),
			PlannedActiveFrom: testTime().Add(-1 * time.Hour),
			CreatedAt:         testTime().Add(-4 * time.Hour),
		})

		_, err = repo.ActivatePublication(ctx, ActivatePublicationParams{
			ID:  assigning.ID,
			Now: testTime(),
		})
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("expected sql.ErrNoRows for assigning publication, got %v", err)
		}
	})

	t.Run("EndPublication only succeeds for active publications", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		template, _, _ := seedPublicationPrerequisites(t, db)
		activeAt := testTime().Add(-30 * time.Minute)
		active := seedPublication(t, db, publicationSeed{
			TemplateID:        template.ID,
			State:             model.PublicationStateActive,
			SubmissionStartAt: testTime().Add(-4 * time.Hour),
			SubmissionEndAt:   testTime().Add(-3 * time.Hour),
			PlannedActiveFrom: testTime().Add(-2 * time.Hour),
			CreatedAt:         testTime().Add(-5 * time.Hour),
		})
		if _, err := db.ExecContext(
			ctx,
			`UPDATE publications SET activated_at = $2 WHERE id = $1;`,
			active.ID,
			activeAt,
		); err != nil {
			t.Fatalf("set activated_at: %v", err)
		}

		ended, err := repo.EndPublication(ctx, EndPublicationParams{
			ID:  active.ID,
			Now: testTime(),
		})
		if err != nil {
			t.Fatalf("end publication: %v", err)
		}
		if ended.State != model.PublicationStateEnded || ended.EndedAt == nil {
			t.Fatalf("expected ended publication, got %+v", ended)
		}

		draftTemplate, _, _ := seedPublicationPrerequisites(t, db)
		draft := seedPublication(t, db, publicationSeed{
			TemplateID:        draftTemplate.ID,
			State:             model.PublicationStateDraft,
			SubmissionStartAt: testTime().Add(1 * time.Hour),
			SubmissionEndAt:   testTime().Add(2 * time.Hour),
			PlannedActiveFrom: testTime().Add(3 * time.Hour),
			CreatedAt:         testTime(),
		})

		_, err = repo.EndPublication(ctx, EndPublicationParams{
			ID:  draft.ID,
			Now: testTime(),
		})
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("expected sql.ErrNoRows for draft publication, got %v", err)
		}
	})

	t.Run("UpsertSubmission is idempotent and can update publication state", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		template, shift, position := seedPublicationPrerequisites(t, db)
		user := seedUser(t, db, userSeed{})
		seedUserPosition(t, db, user.ID, position.ID)
		publication := seedPublication(t, db, publicationSeed{
			TemplateID:        template.ID,
			State:             model.PublicationStateDraft,
			SubmissionStartAt: testTime().Add(1 * time.Hour),
			SubmissionEndAt:   testTime().Add(2 * time.Hour),
			PlannedActiveFrom: testTime().Add(3 * time.Hour),
			CreatedAt:         testTime(),
		})
		newState := model.PublicationStateCollecting

		first, err := repo.UpsertSubmission(ctx, UpsertAvailabilitySubmissionParams{
			PublicationID:    publication.ID,
			UserID:           user.ID,
			TemplateShiftID:  shift.ID,
			PublicationState: &newState,
			Now:              testTime(),
		})
		if err != nil {
			t.Fatalf("first upsert submission: %v", err)
		}

		second, err := repo.UpsertSubmission(ctx, UpsertAvailabilitySubmissionParams{
			PublicationID:   publication.ID,
			UserID:          user.ID,
			TemplateShiftID: shift.ID,
			Now:             testTime().Add(5 * time.Minute),
		})
		if err != nil {
			t.Fatalf("second upsert submission: %v", err)
		}
		if second.ID != first.ID {
			t.Fatalf("expected idempotent submission ID %d, got %d", first.ID, second.ID)
		}

		storedPublication, err := repo.GetByID(ctx, publication.ID)
		if err != nil {
			t.Fatalf("reload publication: %v", err)
		}
		if storedPublication.State != model.PublicationStateCollecting {
			t.Fatalf("expected publication state COLLECTING, got %q", storedPublication.State)
		}
	})

	t.Run("ListAssignmentCandidates and ListQualifiedShifts join related data correctly", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		template := seedTemplate(t, db, templateSeed{})
		matchingPosition := seedPosition(t, db, positionSeed{Name: "Front Desk"})
		otherPosition := seedPosition(t, db, positionSeed{Name: "Kitchen"})
		firstShift := seedTemplateShift(t, db, templateShiftSeed{
			TemplateID:        template.ID,
			Weekday:           1,
			StartTime:         "09:00",
			EndTime:           "12:00",
			PositionID:        matchingPosition.ID,
			RequiredHeadcount: 2,
		})
		secondShift := seedTemplateShift(t, db, templateShiftSeed{
			TemplateID:        template.ID,
			Weekday:           2,
			StartTime:         "13:00",
			EndTime:           "16:00",
			PositionID:        otherPosition.ID,
			RequiredHeadcount: 1,
		})
		publication := seedPublication(t, db, publicationSeed{
			TemplateID:        template.ID,
			State:             model.PublicationStateAssigning,
			SubmissionStartAt: testTime().Add(-4 * time.Hour),
			SubmissionEndAt:   testTime().Add(-2 * time.Hour),
			PlannedActiveFrom: testTime().Add(1 * time.Hour),
			CreatedAt:         testTime().Add(-5 * time.Hour),
		})
		firstUser := seedUser(t, db, userSeed{Name: "Alice", Email: "alice@example.com"})
		secondUser := seedUser(t, db, userSeed{Name: "Bob", Email: "bob@example.com"})
		seedUserPosition(t, db, firstUser.ID, matchingPosition.ID)
		seedUserPosition(t, db, secondUser.ID, otherPosition.ID)
		seedSubmission(t, db, publication.ID, firstUser.ID, firstShift.ID, testTime())
		seedSubmission(t, db, publication.ID, secondUser.ID, secondShift.ID, testTime().Add(1*time.Minute))
		firstSlotID := slotIDForEntry(t, db, firstShift.ID)
		secondSlotID := slotIDForEntry(t, db, secondShift.ID)

		candidates, err := repo.ListAssignmentCandidates(ctx, publication.ID)
		if err != nil {
			t.Fatalf("list assignment candidates: %v", err)
		}
		if len(candidates) != 2 {
			t.Fatalf("expected 2 candidates, got %d", len(candidates))
		}
		if candidates[0].SlotID != firstSlotID || candidates[0].PositionID != matchingPosition.ID || candidates[0].Name != "Alice" {
			t.Fatalf("unexpected first candidate: %+v", candidates[0])
		}
		if candidates[1].SlotID != secondSlotID || candidates[1].PositionID != otherPosition.ID || candidates[1].Name != "Bob" {
			t.Fatalf("unexpected second candidate: %+v", candidates[1])
		}

		qualified, err := repo.ListQualifiedShifts(ctx, publication.ID, firstUser.ID)
		if err != nil {
			t.Fatalf("list qualified shifts: %v", err)
		}
		if len(qualified) != 1 {
			t.Fatalf("expected 1 qualified shift, got %d", len(qualified))
		}
		if qualified[0].ID != firstShift.ID {
			t.Fatalf("expected qualified shift ID %d, got %d", firstShift.ID, qualified[0].ID)
		}
		if qualified[0].StartTime != "09:00" || qualified[0].EndTime != "12:00" {
			t.Fatalf("expected time formatting 09:00-12:00, got %s-%s", qualified[0].StartTime, qualified[0].EndTime)
		}
	})

	t.Run("GetAssignmentBoardView groups slot positions and filters non-candidate qualified users", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		template := seedTemplate(t, db, templateSeed{Name: "Board Template"})
		firstPosition := seedPosition(t, db, positionSeed{Name: "Front Desk"})
		secondPosition := seedPosition(t, db, positionSeed{Name: "Cashier"})
		firstSlotID := seedTemplateSlot(t, db, template.ID, 1, "09:00", "12:00")
		secondSlotID := seedTemplateSlot(t, db, template.ID, 2, "13:00", "16:00")
		firstEntryID := seedTemplateSlotPosition(t, db, firstSlotID, firstPosition.ID, 2)
		seedTemplateSlotPosition(t, db, secondSlotID, secondPosition.ID, 1)
		publication := seedPublication(t, db, publicationSeed{
			TemplateID:        template.ID,
			State:             model.PublicationStateAssigning,
			SubmissionStartAt: testTime().Add(-4 * time.Hour),
			SubmissionEndAt:   testTime().Add(-2 * time.Hour),
			PlannedActiveFrom: testTime().Add(1 * time.Hour),
			CreatedAt:         testTime().Add(-5 * time.Hour),
		})
		firstCandidate := seedUser(t, db, userSeed{Name: "Alice", Email: "alice@example.com"})
		secondCandidate := seedUser(t, db, userSeed{Name: "Bob", Email: "bob@example.com"})
		nonCandidateQualified := seedUser(t, db, userSeed{Name: "Dana", Email: "dana@example.com"})
		disabledQualified := seedUser(t, db, userSeed{
			Name:   "Disabled",
			Email:  "disabled@example.com",
			Status: model.UserStatusDisabled,
		})

		for _, userID := range []int64{
			firstCandidate.ID,
			secondCandidate.ID,
			nonCandidateQualified.ID,
			disabledQualified.ID,
		} {
			seedUserPosition(t, db, userID, firstPosition.ID)
		}

		seedSubmission(t, db, publication.ID, firstCandidate.ID, firstEntryID, testTime())
		seedSubmission(t, db, publication.ID, secondCandidate.ID, firstEntryID, testTime().Add(1*time.Minute))
		seedAssignment(t, db, publication.ID, secondCandidate.ID, firstEntryID, testTime().Add(2*time.Minute))

		board, err := repo.GetAssignmentBoardView(ctx, publication.ID)
		if err != nil {
			t.Fatalf("get assignment board view: %v", err)
		}
		if len(board) != 2 {
			t.Fatalf("expected 2 slots in board view, got %+v", board)
		}

		firstSlot := board[firstSlotID]
		if firstSlot == nil || firstSlot.Slot == nil {
			t.Fatalf("expected first slot view, got %+v", board)
		}
		firstPositionView := firstSlot.Positions[firstPosition.ID]
		if firstPositionView == nil {
			t.Fatalf("expected first slot position view, got %+v", firstSlot.Positions)
		}
		if firstPositionView.RequiredHeadcount != 2 {
			t.Fatalf("expected required_headcount=2, got %+v", firstPositionView)
		}
		if len(firstPositionView.Candidates) != 2 ||
			firstPositionView.Candidates[0].UserID != firstCandidate.ID ||
			firstPositionView.Candidates[1].UserID != secondCandidate.ID {
			t.Fatalf("unexpected candidates: %+v", firstPositionView.Candidates)
		}
		if len(firstPositionView.Assignments) != 1 || firstPositionView.Assignments[0].UserID != secondCandidate.ID {
			t.Fatalf("unexpected assignments: %+v", firstPositionView.Assignments)
		}
		if len(firstPositionView.NonCandidateQualified) != 1 || firstPositionView.NonCandidateQualified[0].UserID != nonCandidateQualified.ID {
			t.Fatalf("unexpected non-candidate qualified users: %+v", firstPositionView.NonCandidateQualified)
		}

		secondSlot := board[secondSlotID]
		if secondSlot == nil || secondSlot.Slot == nil {
			t.Fatalf("expected second slot view, got %+v", board)
		}
		secondPositionView := secondSlot.Positions[secondPosition.ID]
		if secondPositionView == nil {
			t.Fatalf("expected second slot position view, got %+v", secondSlot.Positions)
		}
		if len(secondPositionView.Candidates) != 0 || len(secondPositionView.Assignments) != 0 || len(secondPositionView.NonCandidateQualified) != 0 {
			t.Fatalf("expected empty second slot position view, got %+v", secondPositionView)
		}
	})

	t.Run("ListQualifiedUsersForPositions groups active qualified users by position", func(t *testing.T) {
		db := openIntegrationDB(t)
		repo := NewPublicationRepository(db)
		firstPosition := seedPosition(t, db, positionSeed{Name: "Front Desk"})
		secondPosition := seedPosition(t, db, positionSeed{Name: "Kitchen"})
		firstUser := seedUser(t, db, userSeed{Name: "Alice", Email: "alice@example.com"})
		secondUser := seedUser(t, db, userSeed{Name: "Bob", Email: "bob@example.com"})
		disabledUser := seedUser(t, db, userSeed{
			Name:   "Disabled",
			Email:  "disabled@example.com",
			Status: model.UserStatusDisabled,
		})

		seedUserPosition(t, db, firstUser.ID, firstPosition.ID)
		seedUserPosition(t, db, secondUser.ID, firstPosition.ID)
		seedUserPosition(t, db, secondUser.ID, secondPosition.ID)
		seedUserPosition(t, db, disabledUser.ID, firstPosition.ID)

		qualified, err := repo.ListQualifiedUsersForPositions(ctx, []int64{
			secondPosition.ID,
			firstPosition.ID,
			firstPosition.ID,
		})
		if err != nil {
			t.Fatalf("list qualified users for positions: %v", err)
		}

		if len(qualified[firstPosition.ID]) != 2 {
			t.Fatalf("expected 2 active qualified users for first position, got %+v", qualified[firstPosition.ID])
		}
		if qualified[firstPosition.ID][0].UserID != firstUser.ID || qualified[firstPosition.ID][1].UserID != secondUser.ID {
			t.Fatalf("unexpected first-position users: %+v", qualified[firstPosition.ID])
		}
		if len(qualified[secondPosition.ID]) != 1 || qualified[secondPosition.ID][0].UserID != secondUser.ID {
			t.Fatalf("unexpected second-position users: %+v", qualified[secondPosition.ID])
		}
	})
}

func slotIDForEntry(t testing.TB, db *sql.DB, entryID int64) int64 {
	t.Helper()

	var slotID int64
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT slot_id FROM template_slot_positions WHERE id = $1`,
		entryID,
	).Scan(&slotID); err != nil {
		t.Fatalf("lookup slot id for entry %d: %v", entryID, err)
	}
	return slotID
}

func seedPublicationPrerequisites(
	t testing.TB,
	db *sql.DB,
) (*model.Template, *model.TemplateShift, *model.Position) {
	t.Helper()

	position := seedPosition(t, db, positionSeed{})
	template := seedTemplate(t, db, templateSeed{})
	shift := seedTemplateShift(t, db, templateShiftSeed{
		TemplateID:        template.ID,
		PositionID:        position.ID,
		RequiredHeadcount: 2,
	})

	return template, shift, position
}
