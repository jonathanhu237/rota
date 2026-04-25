package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/audit/audittest"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
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
			PublicationID: 1,
			UserID:        8,
			SlotID:        21,
			PositionID:    101,
		})
		if err != nil {
			t.Fatalf("CreateAssignment returned error: %v", err)
		}
		if assignment.PublicationID != 1 || assignment.UserID != 8 || assignment.SlotID != 21 || assignment.PositionID != 101 {
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
		if event.Metadata["slot_id"] != int64(21) || event.Metadata["position_id"] != int64(101) {
			t.Fatalf("expected slot/position metadata, got %+v", event.Metadata)
		}
	})

	t.Run("rejects same user in same slot with a different position", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		repo.slotPositions[21] = append(repo.slotPositions[21], &model.TemplateSlotPosition{
			ID:                13,
			SlotID:            21,
			PositionID:        102,
			RequiredHeadcount: 1,
		})
		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		first, err := service.CreateAssignment(ctx, CreateAssignmentInput{
			PublicationID: 1,
			UserID:        7,
			SlotID:        21,
			PositionID:    101,
		})
		if err != nil {
			t.Fatalf("first CreateAssignment returned error: %v", err)
		}

		_, err = service.CreateAssignment(ctx, CreateAssignmentInput{
			PublicationID: 1,
			UserID:        7,
			SlotID:        21,
			PositionID:    102,
		})
		if !errors.Is(err, ErrAssignmentUserAlreadyInSlot) {
			t.Fatalf("expected ErrAssignmentUserAlreadyInSlot, got %v", err)
		}
		if len(repo.assignments) != 1 {
			t.Fatalf("expected one stored assignment, got %d", len(repo.assignments))
		}
		if len(stub.Events()) != 1 {
			t.Fatalf("expected one audit event, got %+v", stub.Events())
		}
		if event := stub.FindByAction(audit.ActionAssignmentCreate); event == nil || event.TargetID == nil || *event.TargetID != first.ID {
			t.Fatalf("expected one assignment.create event for first assignment, got %+v", stub.Events())
		}
	})

	t.Run("allows all mutable effective states", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		tests := []struct {
			name        string
			publication *model.Publication
			wantErr     error
		}{
			{name: "draft", publication: draftPublication(now), wantErr: ErrPublicationNotMutable},
			{name: "collecting", publication: collectingPublication(now), wantErr: ErrPublicationNotMutable},
			{name: "published", publication: publishedPublication(now)},
			{name: "active", publication: activePublication(now)},
			{name: "ended", publication: endedPublication(now), wantErr: ErrPublicationNotMutable},
		}

		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				repo := newPublicationRepositoryStatefulMock()
				repo.publications[1] = tc.publication
				service := NewPublicationService(repo, fixedClock{now: now})

				assignment, err := service.CreateAssignment(context.Background(), CreateAssignmentInput{
					PublicationID: 1,
					UserID:        7,
					SlotID:        21,
					PositionID:    101,
				})
				if tc.wantErr != nil {
					if !errors.Is(err, tc.wantErr) {
						t.Fatalf("expected %v, got %v", tc.wantErr, err)
					}
					return
				}
				if err != nil {
					t.Fatalf("CreateAssignment returned error: %v", err)
				}
				if assignment == nil {
					t.Fatal("expected assignment for mutable state")
				}
			})
		}
	})

	t.Run("rejects slot outside publication template", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		repo.templateSlots[99] = &model.TemplateSlot{
			ID:         99,
			TemplateID: 2,
			Weekday:    5,
			StartTime:  "10:00",
			EndTime:    "12:00",
		}
		repo.slotPositions[99] = []*model.TemplateSlotPosition{{ID: 99, SlotID: 99, PositionID: 101, RequiredHeadcount: 1}}
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.CreateAssignment(context.Background(), CreateAssignmentInput{
			PublicationID: 1,
			UserID:        7,
			SlotID:        99,
			PositionID:    101,
		})
		if !errors.Is(err, ErrTemplateSlotNotFound) {
			t.Fatalf("expected ErrTemplateSlotNotFound, got %v", err)
		}
	})

	t.Run("rejects missing user", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		service := NewPublicationService(repo, fixedClock{now: now})

		_, err := service.CreateAssignment(context.Background(), CreateAssignmentInput{
			PublicationID: 1,
			UserID:        999,
			SlotID:        21,
			PositionID:    101,
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
			PublicationID: 1,
			UserID:        9,
			SlotID:        21,
			PositionID:    101,
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
			PublicationID: 1,
			UserID:        7,
			SlotID:        21,
			PositionID:    101,
		})
		if !errors.Is(err, ErrPublicationNotFound) {
			t.Fatalf("expected ErrPublicationNotFound, got %v", err)
		}
	})

	t.Run("allows assignment on different weekday", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		repo.templateSlots[24] = &model.TemplateSlot{
			ID:         24,
			TemplateID: 1,
			Weekday:    2,
			StartTime:  "11:00",
			EndTime:    "14:00",
		}
		repo.slotPositions[24] = []*model.TemplateSlotPosition{{ID: 14, SlotID: 24, PositionID: 102, RequiredHeadcount: 1}}
		repo.assignments[assignmentKey(1, 7, 21)] = &model.Assignment{
			ID:            1,
			PublicationID: 1,
			UserID:        7,
			SlotID:        21,
			PositionID:    101,
			CreatedAt:     now.Add(-15 * time.Minute),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		assignment, err := service.CreateAssignment(context.Background(), CreateAssignmentInput{
			PublicationID: 1,
			UserID:        7,
			SlotID:        24,
			PositionID:    102,
		})
		if err != nil {
			t.Fatalf("CreateAssignment returned error: %v", err)
		}
		if assignment.SlotID != 24 || assignment.PositionID != 102 {
			t.Fatalf("unexpected assignment: %+v", assignment)
		}
	})

	t.Run("allows assignment on touching boundary", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		repo.templateSlots[25] = &model.TemplateSlot{
			ID:         25,
			TemplateID: 1,
			Weekday:    1,
			StartTime:  "12:00",
			EndTime:    "15:00",
		}
		repo.slotPositions[25] = []*model.TemplateSlotPosition{{ID: 15, SlotID: 25, PositionID: 102, RequiredHeadcount: 1}}
		repo.assignments[assignmentKey(1, 7, 21)] = &model.Assignment{
			ID:            1,
			PublicationID: 1,
			UserID:        7,
			SlotID:        21,
			PositionID:    101,
			CreatedAt:     now.Add(-15 * time.Minute),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		assignment, err := service.CreateAssignment(context.Background(), CreateAssignmentInput{
			PublicationID: 1,
			UserID:        7,
			SlotID:        25,
			PositionID:    102,
		})
		if err != nil {
			t.Fatalf("CreateAssignment returned error: %v", err)
		}
		if assignment.SlotID != 25 {
			t.Fatalf("unexpected assignment: %+v", assignment)
		}
	})

	t.Run("invalidates pending requester-side shift change requests after delete", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = publishedPublication(now)
		repo.assignments[assignmentKey(1, 7, 21)] = &model.Assignment{
			ID:            1,
			PublicationID: 1,
			UserID:        7,
			SlotID:        21,
			PositionID:    101,
			CreatedAt:     now.Add(-15 * time.Minute),
		}
		shiftChangeRepo := newShiftChangeRepositoryStatefulMock(repo)
		shiftChangeRepo.requests[55] = &model.ShiftChangeRequest{
			ID:                    55,
			PublicationID:         1,
			Type:                  model.ShiftChangeTypeGivePool,
			RequesterUserID:       7,
			RequesterAssignmentID: 1,
			State:                 model.ShiftChangeStatePending,
			CreatedAt:             now.Add(-time.Hour),
			ExpiresAt:             now.Add(24 * time.Hour),
		}
		emailer := &emailStub{}
		service := NewPublicationService(
			repo,
			fixedClock{now: now},
			WithPublicationShiftChangeNotifications(shiftChangeRepo, emailer, "https://rota.example.com", nil),
		)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		err := service.DeleteAssignment(ctx, DeleteAssignmentInput{
			PublicationID: 1,
			AssignmentID:  1,
		})
		if err != nil {
			t.Fatalf("DeleteAssignment returned error: %v", err)
		}

		if got := shiftChangeRepo.requests[55].State; got != model.ShiftChangeStateInvalidated {
			t.Fatalf("expected request to be invalidated, got %q", got)
		}

		events := stub.Events()
		if len(events) != 2 {
			t.Fatalf("expected 2 audit events, got %+v", events)
		}

		cascadeEvent := stub.FindByAction(audit.ActionShiftChangeInvalidateCascade)
		if cascadeEvent == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionShiftChangeInvalidateCascade, stub.Actions())
		}
		if cascadeEvent.TargetType != audit.TargetTypeShiftChangeRequest {
			t.Fatalf("unexpected target type: %q", cascadeEvent.TargetType)
		}
		if cascadeEvent.TargetID == nil || *cascadeEvent.TargetID != 55 {
			t.Fatalf("unexpected target id: %v", cascadeEvent.TargetID)
		}
		if cascadeEvent.Metadata["reason"] != "assignment_deleted" {
			t.Fatalf("expected reason metadata, got %+v", cascadeEvent.Metadata)
		}
		if cascadeEvent.Metadata["assignment_id"] != int64(1) {
			t.Fatalf("expected assignment_id metadata, got %+v", cascadeEvent.Metadata)
		}

		messages := emailer.messages()
		if len(messages) != 1 {
			t.Fatalf("expected 1 email, got %d", len(messages))
		}
		if messages[0].To != "alice@example.com" {
			t.Fatalf("unexpected email recipient: %+v", messages[0])
		}
		if !strings.Contains(messages[0].Body, "administrator edited the referenced shift") {
			t.Fatalf("expected invalidation copy, got %q", messages[0].Body)
		}
	})

	t.Run("invalidates pending counterpart-side shift change requests after delete", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = activePublication(now)
		repo.assignments[assignmentKey(1, 7, 21)] = &model.Assignment{
			ID:            1,
			PublicationID: 1,
			UserID:        7,
			SlotID:        21,
			PositionID:    101,
			CreatedAt:     now.Add(-30 * time.Minute),
		}
		repo.assignments[assignmentKey(1, 8, 22)] = &model.Assignment{
			ID:            2,
			PublicationID: 1,
			UserID:        8,
			SlotID:        22,
			PositionID:    102,
			CreatedAt:     now.Add(-20 * time.Minute),
		}
		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(2)
		shiftChangeRepo := newShiftChangeRepositoryStatefulMock(repo)
		shiftChangeRepo.requests[56] = &model.ShiftChangeRequest{
			ID:                      56,
			PublicationID:           1,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterUserID:         7,
			RequesterAssignmentID:   1,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
			State:                   model.ShiftChangeStatePending,
			CreatedAt:               now.Add(-time.Hour),
			ExpiresAt:               now.Add(24 * time.Hour),
		}
		emailer := &emailStub{}
		service := NewPublicationService(
			repo,
			fixedClock{now: now},
			WithPublicationShiftChangeNotifications(shiftChangeRepo, emailer, "https://rota.example.com", nil),
		)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		err := service.DeleteAssignment(ctx, DeleteAssignmentInput{
			PublicationID: 1,
			AssignmentID:  2,
		})
		if err != nil {
			t.Fatalf("DeleteAssignment returned error: %v", err)
		}

		if got := shiftChangeRepo.requests[56].State; got != model.ShiftChangeStateInvalidated {
			t.Fatalf("expected request to be invalidated, got %q", got)
		}
		if len(emailer.messages()) != 1 {
			t.Fatalf("expected 1 email, got %d", len(emailer.messages()))
		}
		if stub.FindByAction(audit.ActionShiftChangeInvalidateCascade) == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionShiftChangeInvalidateCascade, stub.Actions())
		}
	})

	t.Run("does not touch non-pending shift change requests", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		repo.assignments[assignmentKey(1, 7, 21)] = &model.Assignment{
			ID:            1,
			PublicationID: 1,
			UserID:        7,
			SlotID:        21,
			PositionID:    101,
			CreatedAt:     now.Add(-15 * time.Minute),
		}
		shiftChangeRepo := newShiftChangeRepositoryStatefulMock(repo)
		shiftChangeRepo.requests[57] = &model.ShiftChangeRequest{
			ID:                    57,
			PublicationID:         1,
			Type:                  model.ShiftChangeTypeGivePool,
			RequesterUserID:       7,
			RequesterAssignmentID: 1,
			State:                 model.ShiftChangeStateApproved,
			CreatedAt:             now.Add(-time.Hour),
			ExpiresAt:             now.Add(24 * time.Hour),
		}
		emailer := &emailStub{}
		service := NewPublicationService(
			repo,
			fixedClock{now: now},
			WithPublicationShiftChangeNotifications(shiftChangeRepo, emailer, "https://rota.example.com", nil),
		)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		err := service.DeleteAssignment(ctx, DeleteAssignmentInput{
			PublicationID: 1,
			AssignmentID:  1,
		})
		if err != nil {
			t.Fatalf("DeleteAssignment returned error: %v", err)
		}

		if got := shiftChangeRepo.requests[57].State; got != model.ShiftChangeStateApproved {
			t.Fatalf("expected request state to remain approved, got %q", got)
		}
		if stub.FindByAction(audit.ActionShiftChangeInvalidateCascade) != nil {
			t.Fatalf("did not expect cascade audit event, got %v", stub.Actions())
		}
		if len(emailer.messages()) != 0 {
			t.Fatalf("expected no email, got %d", len(emailer.messages()))
		}
	})

	t.Run("skips cascade side effects when no pending request references the assignment", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		repo.assignments[assignmentKey(1, 7, 21)] = &model.Assignment{
			ID:            1,
			PublicationID: 1,
			UserID:        7,
			SlotID:        21,
			PositionID:    101,
			CreatedAt:     now.Add(-15 * time.Minute),
		}
		shiftChangeRepo := newShiftChangeRepositoryStatefulMock(repo)
		emailer := &emailStub{}
		service := NewPublicationService(
			repo,
			fixedClock{now: now},
			WithPublicationShiftChangeNotifications(shiftChangeRepo, emailer, "https://rota.example.com", nil),
		)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		err := service.DeleteAssignment(ctx, DeleteAssignmentInput{
			PublicationID: 1,
			AssignmentID:  1,
		})
		if err != nil {
			t.Fatalf("DeleteAssignment returned error: %v", err)
		}

		if len(stub.Events()) != 1 {
			t.Fatalf("expected only assignment delete audit event, got %+v", stub.Events())
		}
		if len(emailer.messages()) != 0 {
			t.Fatalf("expected no email, got %d", len(emailer.messages()))
		}
	})

	t.Run("delete still succeeds when cascade update fails", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = publishedPublication(now)
		repo.assignments[assignmentKey(1, 7, 21)] = &model.Assignment{
			ID:            1,
			PublicationID: 1,
			UserID:        7,
			SlotID:        21,
			PositionID:    101,
			CreatedAt:     now.Add(-15 * time.Minute),
		}
		shiftChangeRepo := newShiftChangeRepositoryStatefulMock(repo)
		shiftChangeRepo.invalidateRequestsForAssignmentErr = errors.New("db unavailable")
		shiftChangeRepo.requests[58] = &model.ShiftChangeRequest{
			ID:                    58,
			PublicationID:         1,
			Type:                  model.ShiftChangeTypeGivePool,
			RequesterUserID:       7,
			RequesterAssignmentID: 1,
			State:                 model.ShiftChangeStatePending,
			CreatedAt:             now.Add(-time.Hour),
			ExpiresAt:             now.Add(24 * time.Hour),
		}
		emailer := &emailStub{}
		service := NewPublicationService(
			repo,
			fixedClock{now: now},
			WithPublicationShiftChangeNotifications(shiftChangeRepo, emailer, "https://rota.example.com", nil),
		)

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
			t.Fatalf("expected assignment delete to succeed, got %d assignments", len(repo.assignments))
		}
		if got := shiftChangeRepo.requests[58].State; got != model.ShiftChangeStatePending {
			t.Fatalf("expected request to remain pending, got %q", got)
		}
		if len(stub.Events()) != 1 {
			t.Fatalf("expected only assignment delete audit event, got %+v", stub.Events())
		}
		if len(emailer.messages()) != 0 {
			t.Fatalf("expected no email, got %d", len(emailer.messages()))
		}
	})
}

func TestPublicationServiceCreateAssignmentDisabledAfterPrecheck(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	base := newPublicationRepositoryStatefulMock()
	base.publications[1] = assigningPublication(now)
	repo := createAssignmentDisabledRaceRepo{publicationRepositoryStatefulMock: base}
	service := NewPublicationService(repo, fixedClock{now: now})

	stub := audittest.New()
	ctx := stub.ContextWith(context.Background())
	_, err := service.CreateAssignment(ctx, CreateAssignmentInput{
		PublicationID: 1,
		UserID:        7,
		SlotID:        21,
		PositionID:    101,
	})
	if !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("expected ErrUserDisabled, got %v", err)
	}
	if len(stub.Events()) != 0 {
		t.Fatalf("expected no audit events, got %+v", stub.Events())
	}
}

type createAssignmentDisabledRaceRepo struct {
	*publicationRepositoryStatefulMock
}

func (r createAssignmentDisabledRaceRepo) CreateAssignment(
	ctx context.Context,
	params repository.CreateAssignmentParams,
) (*model.Assignment, error) {
	return nil, repository.ErrUserDisabled
}

func TestPublicationServiceDeleteAssignment(t *testing.T) {
	t.Run("deletes an existing assignment", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = assigningPublication(now)
		repo.assignments[assignmentKey(1, 7, 21)] = &model.Assignment{
			ID:            1,
			PublicationID: 1,
			UserID:        7,
			SlotID:        21,
			PositionID:    101,
			CreatedAt:     now.Add(-15 * time.Minute),
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
		if event.Metadata["user_id"] != int64(7) || event.Metadata["slot_id"] != int64(21) || event.Metadata["position_id"] != int64(101) {
			t.Fatalf("expected slot-based metadata, got %+v", event.Metadata)
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

	t.Run("allows all mutable effective states", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		tests := []struct {
			name        string
			publication *model.Publication
			wantErr     error
		}{
			{name: "draft", publication: draftPublication(now), wantErr: ErrPublicationNotMutable},
			{name: "collecting", publication: collectingPublication(now), wantErr: ErrPublicationNotMutable},
			{name: "published", publication: publishedPublication(now)},
			{name: "active", publication: activePublication(now)},
			{name: "ended", publication: endedPublication(now), wantErr: ErrPublicationNotMutable},
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
				if tc.wantErr != nil {
					if !errors.Is(err, tc.wantErr) {
						t.Fatalf("expected %v, got %v", tc.wantErr, err)
					}
					if len(stub.Events()) != 0 {
						t.Fatalf("expected no audit events, got %+v", stub.Events())
					}
					return
				}
				if err != nil {
					t.Fatalf("DeleteAssignment returned error: %v", err)
				}
				if len(stub.Events()) != 1 {
					t.Fatalf("expected one audit event, got %+v", stub.Events())
				}
			})
		}
	})
}

func TestPublicationServiceActivatePublication(t *testing.T) {
	t.Run("activates published publication and sets activated_at", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		pub := publishedPublication(now)
		pub.Name = "April Coverage"
		repo.publications[1] = pub
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

	t.Run("rejects non-published effective states", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		tests := []struct {
			name        string
			publication *model.Publication
		}{
			{name: "draft", publication: draftPublication(now)},
			{name: "collecting", publication: collectingPublication(now)},
			{name: "assigning", publication: assigningPublication(now)},
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
				if !errors.Is(err, ErrPublicationNotPublished) {
					t.Fatalf("expected ErrPublicationNotPublished, got %v", err)
				}
			})
		}
	})

	t.Run("expires pending shift-change requests and emits aggregate audit event", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = publishedPublication(now)
		decidedAt := now.Add(-time.Hour)
		repo.shiftChangeRequests[10] = &model.ShiftChangeRequest{
			ID:                    10,
			PublicationID:         1,
			Type:                  model.ShiftChangeTypeSwap,
			RequesterUserID:       7,
			RequesterAssignmentID: 11,
			State:                 model.ShiftChangeStatePending,
			ExpiresAt:             now.Add(24 * time.Hour),
		}
		repo.shiftChangeRequests[11] = &model.ShiftChangeRequest{
			ID:                    11,
			PublicationID:         1,
			Type:                  model.ShiftChangeTypeGivePool,
			RequesterUserID:       7,
			RequesterAssignmentID: 12,
			State:                 model.ShiftChangeStateApproved,
			ExpiresAt:             now.Add(24 * time.Hour),
			DecidedAt:             &decidedAt,
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		if _, err := service.ActivatePublication(ctx, 1); err != nil {
			t.Fatalf("ActivatePublication returned error: %v", err)
		}
		if repo.shiftChangeRequests[10].State != model.ShiftChangeStateExpired {
			t.Fatalf("expected pending request to expire, got %s", repo.shiftChangeRequests[10].State)
		}
		if repo.shiftChangeRequests[11].State != model.ShiftChangeStateApproved {
			t.Fatalf("expected already-decided request to stay approved, got %s", repo.shiftChangeRequests[11].State)
		}

		event := stub.FindByAction(audit.ActionShiftChangeExpireBulk)
		if event == nil {
			t.Fatalf("expected %q audit event, got %v", audit.ActionShiftChangeExpireBulk, stub.Actions())
		}
		if got := event.Metadata["expired_count"]; got != 1 {
			t.Fatalf("expected expired_count=1, got %v", got)
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
	t.Run("assignment board is available during all mutable publication states", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		tests := []struct {
			name        string
			publication *model.Publication
			wantErr     error
		}{
			{name: "draft", publication: draftPublication(now), wantErr: ErrPublicationNotAssigning},
			{name: "collecting", publication: collectingPublication(now), wantErr: ErrPublicationNotAssigning},
			{name: "assigning", publication: assigningPublication(now)},
			{name: "published", publication: publishedPublication(now)},
			{name: "active", publication: activePublication(now)},
			{name: "ended", publication: endedPublication(now), wantErr: ErrPublicationNotAssigning},
		}

		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				repo := newPublicationRepositoryStatefulMock()
				repo.publications[1] = tc.publication
				service := NewPublicationService(repo, fixedClock{now: now})

				_, err := service.GetAssignmentBoard(context.Background(), 1)
				if tc.wantErr != nil {
					if !errors.Is(err, tc.wantErr) {
						t.Fatalf("expected %v, got %v", tc.wantErr, err)
					}
					return
				}
				if err != nil {
					t.Fatalf("GetAssignmentBoard returned error: %v", err)
				}
			})
		}
	})

	t.Run("assignment board returns slots, positions, non-candidate qualified users, assignments, and zero-candidate slots", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = publishedPublication(now)
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
		repo.users[10] = &model.User{
			ID:     10,
			Email:  "dana@example.com",
			Name:   "Dana",
			Status: model.UserStatusActive,
		}
		repo.qualifiedByUser[8] = map[int64]struct{}{101: {}}
		repo.qualifiedByUser[10] = map[int64]struct{}{101: {}}
		repo.assignments[assignmentKey(1, 8, 21)] = &model.Assignment{
			ID:            1,
			PublicationID: 1,
			UserID:        8,
			SlotID:        21,
			PositionID:    101,
			CreatedAt:     now.Add(-30 * time.Minute),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		board, err := service.GetAssignmentBoard(context.Background(), 1)
		if err != nil {
			t.Fatalf("GetAssignmentBoard returned error: %v", err)
		}
		if len(board.Slots) != 2 {
			t.Fatalf("expected 2 board slots, got %d", len(board.Slots))
		}
		if len(board.Slots[0].Positions) != 1 {
			t.Fatalf("expected one position in first slot, got %d", len(board.Slots[0].Positions))
		}
		firstPosition := board.Slots[0].Positions[0]
		if firstPosition.RequiredHeadcount != 2 {
			t.Fatalf("expected headcount 2, got %d", firstPosition.RequiredHeadcount)
		}
		if len(firstPosition.Candidates) != 2 {
			t.Fatalf("expected 2 candidates, got %d", len(firstPosition.Candidates))
		}
		if firstPosition.Candidates[0].UserID != 7 {
			t.Fatalf("expected revoked-but-submitted candidate user 7, got %d", firstPosition.Candidates[0].UserID)
		}
		if len(firstPosition.Assignments) != 1 || firstPosition.Assignments[0].UserID != 8 {
			t.Fatalf("unexpected assignments: %+v", firstPosition.Assignments)
		}
		if len(firstPosition.NonCandidateQualified) != 1 || firstPosition.NonCandidateQualified[0].UserID != 10 {
			t.Fatalf("unexpected non-candidate qualified users: %+v", firstPosition.NonCandidateQualified)
		}
		secondPosition := board.Slots[1].Positions[0]
		if len(secondPosition.Candidates) != 0 {
			t.Fatalf("expected zero candidates for second slot position, got %d", len(secondPosition.Candidates))
		}
		if len(secondPosition.NonCandidateQualified) != 0 {
			t.Fatalf("expected zero non-candidate qualified users for second slot position, got %d", len(secondPosition.NonCandidateQualified))
		}
	})

	t.Run("publication roster returns all weekdays and sorted assignments", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		repo := newPublicationRepositoryStatefulMock()
		repo.publications[1] = activePublication(now)
		repo.templateSlots[23] = &model.TemplateSlot{
			ID:         23,
			TemplateID: 1,
			Weekday:    1,
			StartTime:  "14:00",
			EndTime:    "18:00",
		}
		repo.slotPositions[23] = []*model.TemplateSlotPosition{{ID: 13, SlotID: 23, PositionID: 103, RequiredHeadcount: 1}}
		repo.assignments[assignmentKey(1, 8, 21)] = &model.Assignment{
			ID:            2,
			PublicationID: 1,
			UserID:        8,
			SlotID:        21,
			PositionID:    101,
			CreatedAt:     now.Add(-20 * time.Minute),
		}
		repo.assignments[assignmentKey(1, 7, 21)] = &model.Assignment{
			ID:            1,
			PublicationID: 1,
			UserID:        7,
			SlotID:        21,
			PositionID:    101,
			CreatedAt:     now.Add(-30 * time.Minute),
		}
		service := NewPublicationService(repo, fixedClock{now: now})

		roster, err := service.GetPublicationRoster(context.Background(), 1)
		if err != nil {
			t.Fatalf("GetPublicationRoster returned error: %v", err)
		}
		if len(roster.Weekdays) != 7 {
			t.Fatalf("expected 7 weekdays, got %d", len(roster.Weekdays))
		}
		if len(roster.Weekdays[0].Slots) != 2 {
			t.Fatalf("expected 2 monday slots, got %d", len(roster.Weekdays[0].Slots))
		}
		if roster.Weekdays[0].Slots[0].Slot.ID != 21 || roster.Weekdays[0].Slots[1].Slot.ID != 23 {
			t.Fatalf("unexpected monday slot order: %+v", roster.Weekdays[0].Slots)
		}
		assignments := roster.Weekdays[0].Slots[0].Positions[0].Assignments
		if len(assignments) != 2 || assignments[0].UserID != 7 || assignments[1].UserID != 8 {
			t.Fatalf("expected user_id ordering [7,8], got %+v", assignments)
		}
		if len(roster.Weekdays[1].Slots) != 0 {
			t.Fatalf("expected empty tuesday slots, got %d", len(roster.Weekdays[1].Slots))
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

func publishedPublication(now time.Time) *model.Publication {
	return &model.Publication{
		ID:                1,
		TemplateID:        1,
		TemplateName:      "Core Week",
		Name:              "Published",
		State:             model.PublicationStatePublished,
		SubmissionStartAt: now.Add(-72 * time.Hour),
		SubmissionEndAt:   now.Add(-48 * time.Hour),
		PlannedActiveFrom: now.Add(24 * time.Hour),
		CreatedAt:         now.Add(-96 * time.Hour),
		UpdatedAt:         now.Add(-2 * time.Hour),
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
