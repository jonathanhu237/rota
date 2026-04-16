package service

import (
	"context"
	"errors"
	"testing"
	"time"

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

		result, err := service.AutoAssignPublication(context.Background(), 1)
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
	})

	t.Run("rejected outside assigning", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = activePublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.AutoAssignPublication(context.Background(), 1)
		if !errors.Is(err, ErrPublicationNotAssigning) {
			t.Fatalf("expected ErrPublicationNotAssigning, got %v", err)
		}
	})

	t.Run("empty candidates returns empty assignments without error", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		result, err := service.AutoAssignPublication(context.Background(), 1)
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
	})
}
