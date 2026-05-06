package service

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/audit/audittest"
	"github.com/jonathanhu237/rota/backend/internal/model"
)

func TestPublicationServiceAdminAvailabilityRead(t *testing.T) {
	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)

	t.Run("board includes zero-submission relevant employees and supports search", func(t *testing.T) {
		repo := newAdminAvailabilityMock(now)
		repo.submissions[submissionKey(1, 7, 21, 1)] = &model.AvailabilitySubmission{
			ID:            1,
			PublicationID: 1,
			UserID:        7,
			SlotID:        21,
			Weekday:       1,
			CreatedAt:     now,
		}
		repo.users[10] = &model.User{ID: 10, Email: "admin@example.com", Name: "Admin", IsAdmin: true, Status: model.UserStatusActive}
		repo.qualifiedByUser[10] = map[int64]struct{}{101: {}}
		repo.users[11] = &model.User{ID: 11, Email: "outside@example.com", Name: "Outside", Status: model.UserStatusActive}
		repo.qualifiedByUser[11] = map[int64]struct{}{999: {}}

		service := NewPublicationService(repo, fixedClock{now: now})
		result, err := service.ListAdminAvailability(context.Background(), ListAdminAvailabilityInput{
			PublicationID: 1,
			Page:          1,
			PageSize:      10,
		})
		if err != nil {
			t.Fatalf("list admin availability: %v", err)
		}

		if result.Page != 1 || result.PageSize != 10 || result.Total != 2 || result.TotalPages != 1 {
			t.Fatalf("unexpected pagination: %+v", result)
		}
		if len(result.Employees) != 2 {
			t.Fatalf("expected Alice and Bob only, got %+v", result.Employees)
		}
		if result.Employees[0].UserID != 7 || result.Employees[0].SubmittedCount != 1 {
			t.Fatalf("unexpected Alice row: %+v", result.Employees[0])
		}
		if result.Employees[1].UserID != 8 || result.Employees[1].SubmittedCount != 0 {
			t.Fatalf("unexpected Bob zero-submission row: %+v", result.Employees[1])
		}

		filtered, err := service.ListAdminAvailability(context.Background(), ListAdminAvailabilityInput{
			PublicationID: 1,
			Page:          1,
			PageSize:      10,
			Search:        "bob",
		})
		if err != nil {
			t.Fatalf("search admin availability: %v", err)
		}
		if len(filtered.Employees) != 1 || filtered.Employees[0].UserID != 8 {
			t.Fatalf("expected only Bob after search, got %+v", filtered.Employees)
		}
	})

	t.Run("detail carries eligibility and ineligible submitted exceptions", func(t *testing.T) {
		repo := newAdminAvailabilityMock(now)
		repo.submissions[submissionKey(1, 7, 22, 3)] = &model.AvailabilitySubmission{
			ID:            1,
			PublicationID: 1,
			UserID:        7,
			SlotID:        22,
			Weekday:       3,
			CreatedAt:     now,
		}

		service := NewPublicationService(repo, fixedClock{now: now})
		result, err := service.GetAdminAvailabilityDetail(context.Background(), GetAdminAvailabilityDetailInput{
			PublicationID: 1,
			UserID:        7,
		})
		if err != nil {
			t.Fatalf("get admin availability detail: %v", err)
		}
		if result.User.ID != 7 || len(result.Slots) != 2 || len(result.Cells) != 2 {
			t.Fatalf("unexpected detail result: %+v", result)
		}

		cells := cellsByRef(result.Cells)
		if !cells[model.SlotRef{SlotID: 21, Weekday: 1}].Eligible {
			t.Fatalf("expected Alice to be eligible for slot 21")
		}
		exception := cells[model.SlotRef{SlotID: 22, Weekday: 3}]
		if exception.Eligible || !exception.Submitted {
			t.Fatalf("expected slot 22 to be an ineligible submitted exception, got %+v", exception)
		}
	})
}

func TestPublicationServiceAdminAvailabilityReplace(t *testing.T) {
	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)

	t.Run("replaces one employee in assigning", func(t *testing.T) {
		repo := newAdminAvailabilityMock(now)
		repo.publications[1] = assigningPublication(now)
		repo.submissions[submissionKey(1, 8, 21, 1)] = &model.AvailabilitySubmission{
			ID:            1,
			PublicationID: 1,
			UserID:        8,
			SlotID:        21,
			Weekday:       1,
			CreatedAt:     now,
		}

		service := NewPublicationService(repo, fixedClock{now: now})
		result, err := service.ReplaceAdminAvailability(context.Background(), ReplaceAdminAvailabilityInput{
			PublicationID: 1,
			UserID:        8,
			Submissions:   []model.SlotRef{{SlotID: 22, Weekday: 3}},
		})
		if err != nil {
			t.Fatalf("replace admin availability: %v", err)
		}

		want := []model.SlotRef{{SlotID: 22, Weekday: 3}}
		if !reflect.DeepEqual(result.Submissions, want) {
			t.Fatalf("unexpected submissions: got %+v want %+v", result.Submissions, want)
		}
		if _, ok := repo.submissions[submissionKey(1, 8, 21, 1)]; ok {
			t.Fatalf("expected old submission to be removed")
		}
		if _, ok := repo.submissions[submissionKey(1, 8, 22, 3)]; !ok {
			t.Fatalf("expected new submission to be inserted")
		}
	})

	t.Run("empty replacement clears submissions", func(t *testing.T) {
		repo := newAdminAvailabilityMock(now)
		repo.submissions[submissionKey(1, 8, 21, 1)] = &model.AvailabilitySubmission{ID: 1, PublicationID: 1, UserID: 8, SlotID: 21, Weekday: 1, CreatedAt: now}
		repo.submissions[submissionKey(1, 8, 22, 3)] = &model.AvailabilitySubmission{ID: 2, PublicationID: 1, UserID: 8, SlotID: 22, Weekday: 3, CreatedAt: now}

		service := NewPublicationService(repo, fixedClock{now: now})
		result, err := service.ReplaceAdminAvailability(context.Background(), ReplaceAdminAvailabilityInput{
			PublicationID: 1,
			UserID:        8,
			Submissions:   []model.SlotRef{},
		})
		if err != nil {
			t.Fatalf("replace admin availability: %v", err)
		}
		if len(result.Submissions) != 0 || repo.submissionCount(1, 8) != 0 {
			t.Fatalf("expected submissions to be cleared, got detail=%+v repo=%d", result.Submissions, repo.submissionCount(1, 8))
		}
	})

	t.Run("normalizes duplicate target cells", func(t *testing.T) {
		repo := newAdminAvailabilityMock(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		result, err := service.ReplaceAdminAvailability(context.Background(), ReplaceAdminAvailabilityInput{
			PublicationID: 1,
			UserID:        8,
			Submissions: []model.SlotRef{
				{SlotID: 22, Weekday: 3},
				{SlotID: 22, Weekday: 3},
			},
		})
		if err != nil {
			t.Fatalf("replace admin availability: %v", err)
		}
		want := []model.SlotRef{{SlotID: 22, Weekday: 3}}
		if !reflect.DeepEqual(result.Submissions, want) || repo.submissionCount(1, 8) != 1 {
			t.Fatalf("expected one normalized submission, got detail=%+v repo=%d", result.Submissions, repo.submissionCount(1, 8))
		}
	})

	t.Run("rejects ineligible final cell and rolls back", func(t *testing.T) {
		repo := newAdminAvailabilityMock(now)
		repo.submissions[submissionKey(1, 7, 21, 1)] = &model.AvailabilitySubmission{ID: 1, PublicationID: 1, UserID: 7, SlotID: 21, Weekday: 1, CreatedAt: now}

		service := NewPublicationService(repo, fixedClock{now: now})
		_, err := service.ReplaceAdminAvailability(context.Background(), ReplaceAdminAvailabilityInput{
			PublicationID: 1,
			UserID:        7,
			Submissions:   []model.SlotRef{{SlotID: 22, Weekday: 3}},
		})
		if !errors.Is(err, ErrNotQualified) {
			t.Fatalf("expected ErrNotQualified, got %v", err)
		}
		if _, ok := repo.submissions[submissionKey(1, 7, 21, 1)]; !ok {
			t.Fatalf("expected original submission to remain")
		}
	})

	t.Run("rejects invalid template cell and rolls back", func(t *testing.T) {
		repo := newAdminAvailabilityMock(now)
		repo.submissions[submissionKey(1, 8, 21, 1)] = &model.AvailabilitySubmission{ID: 1, PublicationID: 1, UserID: 8, SlotID: 21, Weekday: 1, CreatedAt: now}

		service := NewPublicationService(repo, fixedClock{now: now})
		_, err := service.ReplaceAdminAvailability(context.Background(), ReplaceAdminAvailabilityInput{
			PublicationID: 1,
			UserID:        8,
			Submissions:   []model.SlotRef{{SlotID: 999, Weekday: 1}},
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
		if _, ok := repo.submissions[submissionKey(1, 8, 21, 1)]; !ok {
			t.Fatalf("expected original submission to remain")
		}
	})

	t.Run("rejects non-mutable publication state", func(t *testing.T) {
		repo := newAdminAvailabilityMock(now)
		publication := collectingPublication(now)
		publication.State = model.PublicationStatePublished
		repo.publications[1] = publication

		service := NewPublicationService(repo, fixedClock{now: now})
		_, err := service.ReplaceAdminAvailability(context.Background(), ReplaceAdminAvailabilityInput{
			PublicationID: 1,
			UserID:        8,
			Submissions:   []model.SlotRef{{SlotID: 22, Weekday: 3}},
		})
		if !errors.Is(err, ErrPublicationNotMutable) {
			t.Fatalf("expected ErrPublicationNotMutable, got %v", err)
		}
	})

	t.Run("rejects disabled and irrelevant users", func(t *testing.T) {
		repo := newAdminAvailabilityMock(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.ReplaceAdminAvailability(context.Background(), ReplaceAdminAvailabilityInput{
			PublicationID: 1,
			UserID:        9,
			Submissions:   []model.SlotRef{},
		})
		if !errors.Is(err, ErrUserDisabled) {
			t.Fatalf("expected ErrUserDisabled, got %v", err)
		}

		repo.users[11] = &model.User{ID: 11, Email: "outside@example.com", Name: "Outside", Status: model.UserStatusActive}
		repo.qualifiedByUser[11] = map[int64]struct{}{999: {}}
		_, err = service.ReplaceAdminAvailability(context.Background(), ReplaceAdminAvailabilityInput{
			PublicationID: 1,
			UserID:        11,
			Submissions:   []model.SlotRef{},
		})
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestPublicationServiceAdminAvailabilityAudit(t *testing.T) {
	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)

	t.Run("records per-cell events after successful replacement", func(t *testing.T) {
		repo := newAdminAvailabilityMock(now)
		repo.submissions[submissionKey(1, 8, 21, 1)] = &model.AvailabilitySubmission{ID: 1, PublicationID: 1, UserID: 8, SlotID: 21, Weekday: 1, CreatedAt: now}
		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		service := NewPublicationService(repo, fixedClock{now: now})
		_, err := service.ReplaceAdminAvailability(ctx, ReplaceAdminAvailabilityInput{
			PublicationID: 1,
			UserID:        8,
			Submissions:   []model.SlotRef{{SlotID: 22, Weekday: 3}},
		})
		if err != nil {
			t.Fatalf("replace admin availability: %v", err)
		}

		actions := stub.Actions()
		wantActions := []string{audit.ActionAvailabilityAdminCreate, audit.ActionAvailabilityAdminDelete}
		sortStrings(actions)
		sortStrings(wantActions)
		if !reflect.DeepEqual(actions, wantActions) {
			t.Fatalf("unexpected audit actions: got %+v want %+v", actions, wantActions)
		}
		for _, event := range stub.Events() {
			if event.TargetType != audit.TargetTypeAvailabilitySubmission {
				t.Fatalf("unexpected target type: %+v", event)
			}
			if event.Metadata["publication_id"] != int64(1) || event.Metadata["user_id"] != int64(8) {
				t.Fatalf("unexpected metadata: %+v", event.Metadata)
			}
		}
	})

	t.Run("does not audit failed or no-op replacements", func(t *testing.T) {
		repo := newAdminAvailabilityMock(now)
		repo.submissions[submissionKey(1, 7, 21, 1)] = &model.AvailabilitySubmission{ID: 1, PublicationID: 1, UserID: 7, SlotID: 21, Weekday: 1, CreatedAt: now}
		stub := audittest.New()
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.ReplaceAdminAvailability(stub.ContextWith(context.Background()), ReplaceAdminAvailabilityInput{
			PublicationID: 1,
			UserID:        7,
			Submissions:   []model.SlotRef{{SlotID: 22, Weekday: 3}},
		})
		if !errors.Is(err, ErrNotQualified) {
			t.Fatalf("expected ErrNotQualified, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events after failed replacement, got %+v", stub.Events())
		}

		_, err = service.ReplaceAdminAvailability(stub.ContextWith(context.Background()), ReplaceAdminAvailabilityInput{
			PublicationID: 1,
			UserID:        7,
			Submissions:   []model.SlotRef{{SlotID: 21, Weekday: 1}},
		})
		if err != nil {
			t.Fatalf("no-op replacement: %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events after no-op replacement, got %+v", stub.Events())
		}
	})
}

func newAdminAvailabilityMock(now time.Time) *publicationRepositoryStatefulMock {
	repo := newPublicationRepositoryStatefulMock()
	publication := collectingPublication(now)
	publication.PlannedActiveUntil = now.Add(72 * time.Hour)
	repo.publications[1] = publication
	return repo
}

func cellsByRef(cells []AdminAvailabilityCell) map[model.SlotRef]AdminAvailabilityCell {
	result := make(map[model.SlotRef]AdminAvailabilityCell, len(cells))
	for _, cell := range cells {
		result[model.SlotRef{SlotID: cell.SlotID, Weekday: cell.Weekday}] = cell
	}
	return result
}

func sortStrings(values []string) {
	sort.Slice(values, func(i, j int) bool {
		return values[i] < values[j]
	})
}
