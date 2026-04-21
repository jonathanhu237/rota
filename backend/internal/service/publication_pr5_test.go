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

func TestPublicationServiceAutoAssignPublication(t *testing.T) {
	t.Run("happy path replaces old assignments and returns updated board", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		repo.templateShifts[11].RequiredHeadcount = 1
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
			CreatedAt:       now.Add(-2 * time.Hour),
		}
		repo.submissions[submissionKey(1, 8, 12)] = &model.AvailabilitySubmission{
			ID:              3,
			PublicationID:   1,
			UserID:          8,
			TemplateShiftID: 12,
			CreatedAt:       now.Add(-2 * time.Hour),
		}
		repo.assignments[assignmentKey(1, 8, 11)] = &model.Assignment{
			ID:              1,
			PublicationID:   1,
			UserID:          8,
			TemplateShiftID: 11,
			CreatedAt:       now.Add(-time.Hour),
		}

		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		result, err := service.AutoAssignPublication(ctx, 1)
		if err != nil {
			t.Fatalf("AutoAssignPublication returned error: %v", err)
		}

		if len(repo.assignments) != 2 {
			t.Fatalf("expected 2 stored assignments, got %d", len(repo.assignments))
		}
		if _, ok := repo.assignments[assignmentKey(1, 8, 11)]; ok {
			t.Fatalf("expected old manual assignment to be replaced, got %+v", repo.assignments)
		}
		if _, ok := repo.assignments[assignmentKey(1, 7, 11)]; !ok {
			t.Fatalf("expected user 7 assigned to shift 11, got %+v", repo.assignments)
		}
		if _, ok := repo.assignments[assignmentKey(1, 8, 12)]; !ok {
			t.Fatalf("expected user 8 assigned to shift 12, got %+v", repo.assignments)
		}
		if len(result.Shifts) != 2 {
			t.Fatalf("expected 2 board shifts, got %d", len(result.Shifts))
		}

		event := stub.FindByAction(audit.ActionPublicationAutoAssign)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionPublicationAutoAssign, stub.Actions())
		}
		if event.TargetType != audit.TargetTypePublication {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != 1 {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		// The solver is expected to produce exactly the two stored assignments.
		if event.Metadata["assignments_created"] != len(repo.assignments) {
			t.Fatalf("expected assignments_created=%d in metadata, got %+v", len(repo.assignments), event.Metadata)
		}
	})

	t.Run("rejected outside assigning", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = activePublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		_, err := service.AutoAssignPublication(ctx, 1)
		if !errors.Is(err, ErrPublicationNotAssigning) {
			t.Fatalf("expected ErrPublicationNotAssigning, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events, got %+v", stub.Events())
		}
	})

	t.Run("empty candidates returns empty assignments without error", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		result, err := service.AutoAssignPublication(ctx, 1)
		if err != nil {
			t.Fatalf("AutoAssignPublication returned error: %v", err)
		}
		if len(repo.assignments) != 0 {
			t.Fatalf("expected no stored assignments, got %+v", repo.assignments)
		}
		for _, shift := range result.Shifts {
			if len(shift.Assignments) != 0 {
				t.Fatalf("expected empty assignments per shift, got %+v", result.Shifts)
			}
		}

		event := stub.FindByAction(audit.ActionPublicationAutoAssign)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionPublicationAutoAssign, stub.Actions())
		}
		if event.TargetID == nil || *event.TargetID != 1 {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}
		if event.Metadata["assignments_created"] != 0 {
			t.Fatalf("expected assignments_created=0 in metadata, got %+v", event.Metadata)
		}
	})
}
