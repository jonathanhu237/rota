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
		repo.slotPositions[21][0].RequiredHeadcount = 1
		repo.submissions[submissionKey(1, 7, 21)] = &model.AvailabilitySubmission{
			ID:            1,
			PublicationID: 1,
			UserID:        7,
			SlotID:        21,
			Weekday:       1,
			CreatedAt:     now.Add(-2 * time.Hour),
		}
		repo.submissions[submissionKey(1, 8, 21)] = &model.AvailabilitySubmission{
			ID:            2,
			PublicationID: 1,
			UserID:        8,
			SlotID:        21,
			Weekday:       1,
			CreatedAt:     now.Add(-2 * time.Hour),
		}
		repo.submissions[submissionKey(1, 8, 22)] = &model.AvailabilitySubmission{
			ID:            3,
			PublicationID: 1,
			UserID:        8,
			SlotID:        22,
			Weekday:       3,
			CreatedAt:     now.Add(-2 * time.Hour),
		}
		repo.assignments[assignmentKey(1, 8, 21)] = &model.Assignment{
			ID:            1,
			PublicationID: 1,
			UserID:        8,
			SlotID:        21,
			Weekday:       1,
			PositionID:    101,
			CreatedAt:     now.Add(-time.Hour),
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
		if _, ok := repo.assignments[assignmentKey(1, 8, 21)]; ok {
			t.Fatalf("expected old manual assignment to be replaced, got %+v", repo.assignments)
		}
		if _, ok := repo.assignments[assignmentKey(1, 7, 21)]; !ok {
			t.Fatalf("expected user 7 assigned to slot 21, got %+v", repo.assignments)
		}
		if _, ok := repo.assignments[assignmentKey(1, 8, 22, 3)]; !ok {
			t.Fatalf("expected user 8 assigned to slot 22, got %+v", repo.assignments)
		}
		if len(result.Slots) != 2 {
			t.Fatalf("expected 2 board slots, got %d", len(result.Slots))
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
		for _, slot := range result.Slots {
			for _, position := range slot.Positions {
				if len(position.Assignments) != 0 {
					t.Fatalf("expected empty assignments per slot position, got %+v", result.Slots)
				}
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

func TestPublicationServiceAutoAssignSkipsRevokedQualification(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)
	repo := newPublicationRepositoryStatefulMock()
	repo.publications[1] = assigningPublication(now)
	repo.slotPositions[21][0].RequiredHeadcount = 1
	repo.submissions[submissionKey(1, 7, 21)] = &model.AvailabilitySubmission{
		ID:            1,
		PublicationID: 1,
		UserID:        7,
		SlotID:        21,
		CreatedAt:     now.Add(-time.Hour),
	}
	delete(repo.qualifiedByUser[7], 101)

	service := NewPublicationService(repo, fixedClock{now: now})
	if _, err := service.AutoAssignPublication(context.Background(), 1); err != nil {
		t.Fatalf("AutoAssignPublication returned error: %v", err)
	}
	if _, ok := repo.assignments[assignmentKey(1, 7, 21)]; ok {
		t.Fatalf("expected revoked qualification to be excluded, got assignments %+v", repo.assignments)
	}
	if len(repo.submissions) != 1 {
		t.Fatalf("expected stale submission row to remain, got %+v", repo.submissions)
	}
}

func TestPublicationServiceAutoAssignSkipsDisabled(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)
	repo := newPublicationRepositoryStatefulMock()
	repo.publications[1] = assigningPublication(now)
	repo.slotPositions[21][0].RequiredHeadcount = 1
	repo.users[7].Status = model.UserStatusDisabled
	repo.submissions[submissionKey(1, 7, 21)] = &model.AvailabilitySubmission{
		ID:            1,
		PublicationID: 1,
		UserID:        7,
		SlotID:        21,
		CreatedAt:     now.Add(-time.Hour),
	}

	service := NewPublicationService(repo, fixedClock{now: now})
	if _, err := service.AutoAssignPublication(context.Background(), 1); err != nil {
		t.Fatalf("AutoAssignPublication returned error: %v", err)
	}
	if _, ok := repo.assignments[assignmentKey(1, 7, 21)]; ok {
		t.Fatalf("expected disabled user to be excluded, got assignments %+v", repo.assignments)
	}
}
