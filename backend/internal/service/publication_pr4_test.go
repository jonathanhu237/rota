package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/audit/audittest"
	"github.com/jonathanhu237/rota/backend/internal/model"
)

func TestPublicationServiceCreateAssignment(t *testing.T) {
	t.Run("creates an assignment during assigning without rechecking qualification", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		assignment, err := service.CreateAssignment(ctx, CreateAssignmentInput{
			PublicationID:   1,
			UserID:          8,
			TemplateShiftID: 11,
		})
		if err != nil {
			t.Fatalf("CreateAssignment returned error: %v", err)
		}
		if assignment.PublicationID != 1 || assignment.UserID != 8 || assignment.TemplateShiftID != 11 {
			t.Fatalf("unexpected assignment: %+v", assignment)
		}

		event := stub.FindByAction(audit.ActionAssignmentCreate)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionAssignmentCreate, stub.Actions())
		}
		if event.TargetType != audit.TargetTypeAssignment {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != assignment.ID {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if event.Metadata["publication_id"] != int64(1) {
			t.Fatalf("expected publication_id=1 in metadata, got %+v", event.Metadata)
		}
		if event.Metadata["user_id"] != int64(8) {
			t.Fatalf("expected user_id=8 in metadata, got %+v", event.Metadata)
		}
		if event.Metadata["template_shift_id"] != int64(11) {
			t.Fatalf("expected template_shift_id=11 in metadata, got %+v", event.Metadata)
		}
	})

	t.Run("duplicate assignment is idempotent", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		first, err := service.CreateAssignment(context.Background(), CreateAssignmentInput{
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
		})
		if err != nil {
			t.Fatalf("first CreateAssignment returned error: %v", err)
		}

		second, err := service.CreateAssignment(context.Background(), CreateAssignmentInput{
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
		})
		if err != nil {
			t.Fatalf("second CreateAssignment returned error: %v", err)
		}
		if first.ID != second.ID {
			t.Fatalf("expected idempotent assignment ids, got %d and %d", first.ID, second.ID)
		}
		if len(repo.assignments) != 1 {
			t.Fatalf("expected one stored assignment, got %d", len(repo.assignments))
		}
	})

	t.Run("rejects non-assigning effective states", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		tests := []struct {
			name        string
			publication *model.Publication
		}{
			{name: "draft", publication: draftPublication(now)},
			{name: "collecting", publication: collectingPublication(now)},
			{name: "active", publication: activePublication(now)},
			{name: "ended", publication: endedPublication(now)},
		}

		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				repo := newPublicationRepositoryStatefulMock()
				repo.publications[1] = tc.publication
				service := NewPublicationService(repo, fixedClock{now: now})

				_, err := service.CreateAssignment(context.Background(), CreateAssignmentInput{
					PublicationID:   1,
					UserID:          7,
					TemplateShiftID: 11,
				})
				if !errors.Is(err, ErrPublicationNotAssigning) {
					t.Fatalf("expected ErrPublicationNotAssigning, got %v", err)
				}
			})
		}
	})

	t.Run("rejects shift outside publication template", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		repo.templateShifts[99] = &model.TemplateShift{
			ID:                99,
			TemplateID:        2,
			Weekday:           5,
			StartTime:         "10:00",
			EndTime:           "12:00",
			PositionID:        101,
			RequiredHeadcount: 1,
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.CreateAssignment(context.Background(), CreateAssignmentInput{
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 99,
		})
		if !errors.Is(err, ErrTemplateShiftNotFound) {
			t.Fatalf("expected ErrTemplateShiftNotFound, got %v", err)
		}
	})

	t.Run("rejects missing user", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.CreateAssignment(context.Background(), CreateAssignmentInput{
			PublicationID:   1,
			UserID:          999,
			TemplateShiftID: 11,
		})
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("rejects disabled user", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		_, err := service.CreateAssignment(ctx, CreateAssignmentInput{
			PublicationID:   1,
			UserID:          9,
			TemplateShiftID: 11,
		})
		if !errors.Is(err, ErrUserDisabled) {
			t.Fatalf("expected ErrUserDisabled, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events, got %+v", stub.Events())
		}
	})

	t.Run("rejects missing publication", func(t *testing.T) {
		t.Parallel()

		repo := newPublicationRepositoryStatefulMock()
		delete(repo.publications, 1)
		service := NewPublicationService(repo, fixedClock{
			now: time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC),
		})

		_, err := service.CreateAssignment(context.Background(), CreateAssignmentInput{
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
		})
		if !errors.Is(err, ErrPublicationNotFound) {
			t.Fatalf("expected ErrPublicationNotFound, got %v", err)
		}
	})
}

func TestPublicationServiceDeleteAssignment(t *testing.T) {
	t.Run("deletes an existing assignment", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		repo.assignments[assignmentKey(1, 7, 11)] = &model.Assignment{
			ID:              1,
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
			CreatedAt:       now.Add(-15 * time.Minute),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		err := service.DeleteAssignment(ctx, DeleteAssignmentInput{
			PublicationID: 1,
			AssignmentID:  1,
		})
		if err != nil {
			t.Fatalf("DeleteAssignment returned error: %v", err)
		}
		if len(repo.assignments) != 0 {
			t.Fatalf("expected assignments to be empty, got %d", len(repo.assignments))
		}

		event := stub.FindByAction(audit.ActionAssignmentDelete)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionAssignmentDelete, stub.Actions())
		}
		if event.TargetType != audit.TargetTypeAssignment {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != 1 {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if event.Metadata["publication_id"] != int64(1) {
			t.Fatalf("expected publication_id=1 in metadata, got %+v", event.Metadata)
		}
	})

	t.Run("delete is idempotent", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		err := service.DeleteAssignment(context.Background(), DeleteAssignmentInput{
			PublicationID: 1,
			AssignmentID:  999,
		})
		if err != nil {
			t.Fatalf("DeleteAssignment returned error: %v", err)
		}
	})

	t.Run("rejects non-assigning effective states", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		tests := []struct {
			name        string
			publication *model.Publication
		}{
			{name: "draft", publication: draftPublication(now)},
			{name: "collecting", publication: collectingPublication(now)},
			{name: "active", publication: activePublication(now)},
			{name: "ended", publication: endedPublication(now)},
		}

		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				repo := newPublicationRepositoryStatefulMock()
				repo.publications[1] = tc.publication
				service := NewPublicationService(repo, fixedClock{now: now})

				stub := audittest.New()
				ctx := stub.ContextWith(context.Background())

				err := service.DeleteAssignment(ctx, DeleteAssignmentInput{
					PublicationID: 1,
					AssignmentID:  1,
				})
				if !errors.Is(err, ErrPublicationNotAssigning) {
					t.Fatalf("expected ErrPublicationNotAssigning, got %v", err)
				}
				if len(stub.Events()) != 0 {
					t.Fatalf("expected no audit events, got %+v", stub.Events())
				}
			})
		}
	})
}

func TestPublicationServiceActivatePublication(t *testing.T) {
	t.Run("activates assigning publication and sets activated_at", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = &model.Publication{
			ID:                1,
			TemplateID:        1,
			TemplateName:      "Core Week",
			Name:              "April Coverage",
			State:             model.PublicationStateAssigning,
			SubmissionStartAt: now.Add(-72 * time.Hour),
			SubmissionEndAt:   now.Add(-48 * time.Hour),
			PlannedActiveFrom: now.Add(-24 * time.Hour),
			CreatedAt:         now.Add(-96 * time.Hour),
			UpdatedAt:         now.Add(-48 * time.Hour),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		publication, err := service.ActivatePublication(ctx, 1)
		if err != nil {
			t.Fatalf("ActivatePublication returned error: %v", err)
		}
		if publication.State != model.PublicationStateActive {
			t.Fatalf("expected active state, got %s", publication.State)
		}
		if publication.ActivatedAt == nil || !publication.ActivatedAt.Equal(now) {
			t.Fatalf("expected activated_at %v, got %+v", now, publication.ActivatedAt)
		}

		event := stub.FindByAction(audit.ActionPublicationActivate)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionPublicationActivate, stub.Actions())
		}
		if event.TargetType != audit.TargetTypePublication {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != 1 {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if event.Metadata["name"] != "April Coverage" {
			t.Fatalf("expected name=April Coverage in metadata, got %+v", event.Metadata)
		}
	})

	t.Run("works when stored state lags behind effective assigning", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		publication, err := service.ActivatePublication(context.Background(), 1)
		if err != nil {
			t.Fatalf("ActivatePublication returned error: %v", err)
		}
		if publication.State != model.PublicationStateActive {
			t.Fatalf("expected active state, got %s", publication.State)
		}
		if repo.publications[1].State != model.PublicationStateActive {
			t.Fatalf("expected stored state to become active, got %s", repo.publications[1].State)
		}
	})

	t.Run("rejects non-assigning effective states", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		tests := []struct {
			name        string
			publication *model.Publication
		}{
			{name: "draft", publication: draftPublication(now)},
			{name: "collecting", publication: collectingPublication(now)},
			{name: "active", publication: activePublication(now)},
			{name: "ended", publication: endedPublication(now)},
		}

		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				repo := newPublicationRepositoryStatefulMock()
				repo.publications[1] = tc.publication
				service := NewPublicationService(repo, fixedClock{now: now})

				_, err := service.ActivatePublication(context.Background(), 1)
				if !errors.Is(err, ErrPublicationNotAssigning) {
					t.Fatalf("expected ErrPublicationNotAssigning, got %v", err)
				}
			})
		}
	})

	t.Run("rejects missing publication", func(t *testing.T) {
		t.Parallel()

		repo := newPublicationRepositoryStatefulMock()
		delete(repo.publications, 1)
		service := NewPublicationService(repo, fixedClock{
			now: time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC),
		})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		_, err := service.ActivatePublication(ctx, 1)
		if !errors.Is(err, ErrPublicationNotFound) {
			t.Fatalf("expected ErrPublicationNotFound, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events, got %+v", stub.Events())
		}
	})
}

func TestPublicationServiceEndPublication(t *testing.T) {
	t.Run("ends active publication and sets ended_at", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = activePublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		publication, err := service.EndPublication(ctx, 1)
		if err != nil {
			t.Fatalf("EndPublication returned error: %v", err)
		}
		if publication.State != model.PublicationStateEnded {
			t.Fatalf("expected ended state, got %s", publication.State)
		}
		if publication.EndedAt == nil || !publication.EndedAt.Equal(now) {
			t.Fatalf("expected ended_at %v, got %+v", now, publication.EndedAt)
		}

		event := stub.FindByAction(audit.ActionPublicationEnd)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionPublicationEnd, stub.Actions())
		}
		if event.TargetType != audit.TargetTypePublication {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != 1 {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if event.Metadata["name"] != "Active" {
			t.Fatalf("expected name=Active in metadata, got %+v", event.Metadata)
		}
	})

	t.Run("rejects non-active effective states", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		tests := []struct {
			name        string
			publication *model.Publication
		}{
			{name: "draft", publication: draftPublication(now)},
			{name: "collecting", publication: collectingPublication(now)},
			{name: "assigning", publication: assigningPublication(now)},
			{name: "ended", publication: endedPublication(now)},
		}

		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				repo := newPublicationRepositoryStatefulMock()
				repo.publications[1] = tc.publication
				service := NewPublicationService(repo, fixedClock{now: now})

				_, err := service.EndPublication(context.Background(), 1)
				if !errors.Is(err, ErrPublicationNotActive) {
					t.Fatalf("expected ErrPublicationNotActive, got %v", err)
				}
			})
		}
	})
}

func TestPublicationServiceAssignmentBoardAndRoster(t *testing.T) {
	t.Run("assignment board returns shifts, candidates, assignments, and zero-candidate shifts", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		repo.submissions[submissionKey(1, 7, 11)] = &model.AvailabilitySubmission{
			ID:              1,
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
			CreatedAt:       now.Add(-2 * time.Hour),
		}
		repo.submissions[submissionKey(1, 8, 11)] = &model.AvailabilitySubmission{
			ID:              2,
			PublicationID:   1,
			UserID:          8,
			TemplateShiftID: 11,
			CreatedAt:       now.Add(-90 * time.Minute),
		}
		delete(repo.qualifiedByUser, 7)
		repo.assignments[assignmentKey(1, 8, 11)] = &model.Assignment{
			ID:              1,
			PublicationID:   1,
			UserID:          8,
			TemplateShiftID: 11,
			CreatedAt:       now.Add(-30 * time.Minute),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		board, err := service.GetAssignmentBoard(context.Background(), 1)
		if err != nil {
			t.Fatalf("GetAssignmentBoard returned error: %v", err)
		}
		if len(board.Shifts) != 2 {
			t.Fatalf("expected 2 board shifts, got %d", len(board.Shifts))
		}
		if board.Shifts[0].Shift.RequiredHeadcount != 2 {
			t.Fatalf("expected headcount 2, got %d", board.Shifts[0].Shift.RequiredHeadcount)
		}
		if len(board.Shifts[0].Candidates) != 2 {
			t.Fatalf("expected 2 candidates, got %d", len(board.Shifts[0].Candidates))
		}
		if board.Shifts[0].Candidates[0].UserID != 7 {
			t.Fatalf("expected revoked-but-submitted candidate user 7, got %d", board.Shifts[0].Candidates[0].UserID)
		}
		if len(board.Shifts[0].Assignments) != 1 || board.Shifts[0].Assignments[0].UserID != 8 {
			t.Fatalf("unexpected assignments: %+v", board.Shifts[0].Assignments)
		}
		if len(board.Shifts[1].Candidates) != 0 {
			t.Fatalf("expected zero candidates for second shift, got %d", len(board.Shifts[1].Candidates))
		}
	})

	t.Run("publication roster returns all weekdays and sorted assignments", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = activePublication(now)
		repo.templateShifts[13] = &model.TemplateShift{
			ID:                13,
			TemplateID:        1,
			Weekday:           1,
			StartTime:         "14:00",
			EndTime:           "18:00",
			PositionID:        103,
			RequiredHeadcount: 1,
		}
		repo.assignments[assignmentKey(1, 8, 11)] = &model.Assignment{
			ID:              2,
			PublicationID:   1,
			UserID:          8,
			TemplateShiftID: 11,
			CreatedAt:       now.Add(-20 * time.Minute),
		}
		repo.assignments[assignmentKey(1, 7, 11)] = &model.Assignment{
			ID:              1,
			PublicationID:   1,
			UserID:          7,
			TemplateShiftID: 11,
			CreatedAt:       now.Add(-30 * time.Minute),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		roster, err := service.GetPublicationRoster(context.Background(), 1)
		if err != nil {
			t.Fatalf("GetPublicationRoster returned error: %v", err)
		}
		if len(roster.Weekdays) != 7 {
			t.Fatalf("expected 7 weekdays, got %d", len(roster.Weekdays))
		}
		if len(roster.Weekdays[0].Shifts) != 2 {
			t.Fatalf("expected 2 monday shifts, got %d", len(roster.Weekdays[0].Shifts))
		}
		if roster.Weekdays[0].Shifts[0].Shift.ID != 11 || roster.Weekdays[0].Shifts[1].Shift.ID != 13 {
			t.Fatalf("unexpected monday shift order: %+v", roster.Weekdays[0].Shifts)
		}
		assignments := roster.Weekdays[0].Shifts[0].Assignments
		if len(assignments) != 2 || assignments[0].UserID != 7 || assignments[1].UserID != 8 {
			t.Fatalf("expected user_id ordering [7,8], got %+v", assignments)
		}
		if len(roster.Weekdays[1].Shifts) != 0 {
			t.Fatalf("expected empty tuesday shifts, got %d", len(roster.Weekdays[1].Shifts))
		}
	})

	t.Run("current roster is empty when no active publication or current is assigning", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		service := NewPublicationService(repo, fixedClock{now: now})

		current, err := service.GetCurrentRoster(context.Background())
		if err != nil {
			t.Fatalf("GetCurrentRoster returned error: %v", err)
		}
		if current.Publication != nil || len(current.Weekdays) != 0 {
			t.Fatalf("expected empty current roster, got %+v", current)
		}

		repo.publications[1] = assigningPublication(now)
		current, err = service.GetCurrentRoster(context.Background())
		if err != nil {
			t.Fatalf("GetCurrentRoster returned error: %v", err)
		}
		if current.Publication != nil || len(current.Weekdays) != 0 {
			t.Fatalf("expected empty assigning roster, got %+v", current)
		}
	})
}

func draftPublication(now time.Time) *model.Publication {
	return &model.Publication{
		ID:                1,
		TemplateID:        1,
		TemplateName:      "Core Week",
		Name:              "Draft",
		State:             model.PublicationStateDraft,
		SubmissionStartAt: now.Add(2 * time.Hour),
		SubmissionEndAt:   now.Add(24 * time.Hour),
		PlannedActiveFrom: now.Add(48 * time.Hour),
		CreatedAt:         now.Add(-24 * time.Hour),
		UpdatedAt:         now.Add(-24 * time.Hour),
	}
}

func assigningPublication(now time.Time) *model.Publication {
	return &model.Publication{
		ID:                1,
		TemplateID:        1,
		TemplateName:      "Core Week",
		Name:              "Assigning",
		State:             model.PublicationStateDraft,
		SubmissionStartAt: now.Add(-72 * time.Hour),
		SubmissionEndAt:   now.Add(-24 * time.Hour),
		PlannedActiveFrom: now.Add(24 * time.Hour),
		CreatedAt:         now.Add(-96 * time.Hour),
		UpdatedAt:         now.Add(-48 * time.Hour),
	}
}

func activePublication(now time.Time) *model.Publication {
	activatedAt := now.Add(-2 * time.Hour)

	return &model.Publication{
		ID:                1,
		TemplateID:        1,
		TemplateName:      "Core Week",
		Name:              "Active",
		State:             model.PublicationStateActive,
		SubmissionStartAt: now.Add(-72 * time.Hour),
		SubmissionEndAt:   now.Add(-48 * time.Hour),
		PlannedActiveFrom: now.Add(-24 * time.Hour),
		ActivatedAt:       &activatedAt,
		CreatedAt:         now.Add(-96 * time.Hour),
		UpdatedAt:         now.Add(-2 * time.Hour),
	}
}

func endedPublication(now time.Time) *model.Publication {
	activatedAt := now.Add(-48 * time.Hour)
	endedAt := now.Add(-2 * time.Hour)

	return &model.Publication{
		ID:                1,
		TemplateID:        1,
		TemplateName:      "Core Week",
		Name:              "Ended",
		State:             model.PublicationStateEnded,
		SubmissionStartAt: now.Add(-96 * time.Hour),
		SubmissionEndAt:   now.Add(-72 * time.Hour),
		PlannedActiveFrom: now.Add(-48 * time.Hour),
		ActivatedAt:       &activatedAt,
		EndedAt:           &endedAt,
		CreatedAt:         now.Add(-120 * time.Hour),
		UpdatedAt:         now.Add(-2 * time.Hour),
	}
}
