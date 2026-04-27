package service

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/email"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

type AssignmentBoardResult struct {
	Publication *model.Publication
	Slots       []*AssignmentBoardSlotResult
	Employees   []*model.AssignmentBoardEmployee
}

type AssignmentBoardSlotResult struct {
	Slot      *model.TemplateSlot
	Positions []*AssignmentBoardPositionResult
}

type AssignmentBoardPositionResult struct {
	Position          *model.Position
	RequiredHeadcount int
	Assignments       []*model.AssignmentParticipant
}

type RosterResult struct {
	Publication *model.Publication
	WeekStart   time.Time
	Weekdays    []*RosterWeekdayResult
}

type RosterWeekdayResult struct {
	Weekday int
	Slots   []*RosterSlotResult
}

type RosterSlotResult struct {
	Slot           *model.TemplateSlot
	OccurrenceDate time.Time
	Positions      []*RosterPositionResult
}

type RosterPositionResult struct {
	Position          *model.Position
	RequiredHeadcount int
	Assignments       []*model.AssignmentParticipant
}

func (s *PublicationService) CreateAssignment(
	ctx context.Context,
	input CreateAssignmentInput,
) (*model.Assignment, error) {
	if input.PublicationID <= 0 ||
		input.UserID <= 0 ||
		input.SlotID <= 0 ||
		input.Weekday < 1 ||
		input.Weekday > 7 ||
		input.PositionID <= 0 {
		return nil, ErrInvalidInput
	}

	now := s.clock.Now()
	publication, err := s.publicationRepo.GetByID(ctx, input.PublicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if !isPublicationMutableForAssignments(model.ResolvePublicationState(publication, now)) {
		return nil, ErrPublicationNotMutable
	}

	slot, err := s.publicationRepo.GetSlot(ctx, publication.TemplateID, input.SlotID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if !slotHasWeekday(slot, input.Weekday) {
		return nil, ErrInvalidInput
	}

	slotPositions, err := s.publicationRepo.ListSlotPositions(ctx, input.SlotID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if !slotHasPosition(slotPositions, input.PositionID) {
		return nil, ErrTemplateSlotPositionNotFound
	}

	user, err := s.publicationRepo.GetUserByID(ctx, input.UserID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if user.Status == model.UserStatusDisabled {
		return nil, ErrUserDisabled
	}

	// The repository repeats the user-status check inside the insert transaction.
	assignment, err := s.publicationRepo.CreateAssignment(ctx, repository.CreateAssignmentParams{
		PublicationID: input.PublicationID,
		UserID:        input.UserID,
		SlotID:        input.SlotID,
		Weekday:       input.Weekday,
		PositionID:    input.PositionID,
		CreatedAt:     now,
	})
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	targetID := assignment.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionAssignmentCreate,
		TargetType: audit.TargetTypeAssignment,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"publication_id": assignment.PublicationID,
			"user_id":        assignment.UserID,
			"slot_id":        assignment.SlotID,
			"weekday":        assignment.Weekday,
			"position_id":    assignment.PositionID,
		},
	})

	return assignment, nil
}

func slotHasPosition(slotPositions []*model.TemplateSlotPosition, positionID int64) bool {
	for _, slotPosition := range slotPositions {
		if slotPosition.PositionID == positionID {
			return true
		}
	}

	return false
}

func (s *PublicationService) DeleteAssignment(
	ctx context.Context,
	input DeleteAssignmentInput,
) error {
	if input.PublicationID <= 0 || input.AssignmentID <= 0 {
		return ErrInvalidInput
	}

	now := s.clock.Now()
	publication, err := s.publicationRepo.GetByID(ctx, input.PublicationID)
	if err != nil {
		return mapPublicationRepositoryError(err)
	}
	if !isPublicationMutableForAssignments(model.ResolvePublicationState(publication, now)) {
		return ErrPublicationNotMutable
	}

	deletedAssignment := s.assignmentSnapshotForCascade(ctx, input.AssignmentID)
	assignmentOverridesRemoved, err := s.publicationRepo.CountAssignmentOverridesByAssignment(ctx, input.AssignmentID)
	if err != nil {
		s.logger.Warn("publication: count assignment overrides for delete", "assignment_id", input.AssignmentID, "error", err)
		assignmentOverridesRemoved = 0
	}

	var invalidatedRequests []*model.ShiftChangeRequest
	if err := s.publicationRepo.DeleteAssignment(ctx, repository.DeleteAssignmentParams{
		PublicationID: input.PublicationID,
		AssignmentID:  input.AssignmentID,
		AfterDeleteTx: func(ctx context.Context, tx *sql.Tx) error {
			if s.shiftChangeRepo == nil {
				return nil
			}
			requests, err := s.shiftChangeRepo.InvalidateRequestsForAssignmentTx(ctx, tx, input.AssignmentID, now)
			if err != nil {
				return err
			}
			for _, req := range requests {
				if err := s.enqueueShiftChangeRequestInvalidatedTx(ctx, tx, req, input.AssignmentID, deletedAssignment); err != nil {
					return err
				}
			}
			invalidatedRequests = requests
			return nil
		},
	}); err != nil {
		return mapPublicationRepositoryError(err)
	}

	targetID := input.AssignmentID
	metadata := map[string]any{
		"publication_id":               input.PublicationID,
		"assignment_overrides_removed": assignmentOverridesRemoved,
	}
	if deletedAssignment != nil {
		metadata["user_id"] = deletedAssignment.UserID
		metadata["slot_id"] = deletedAssignment.SlotID
		metadata["weekday"] = deletedAssignment.Weekday
		metadata["position_id"] = deletedAssignment.PositionID
	}
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionAssignmentDelete,
		TargetType: audit.TargetTypeAssignment,
		TargetID:   &targetID,
		Metadata:   metadata,
	})

	for _, req := range invalidatedRequests {
		targetID := req.ID
		audit.Record(ctx, audit.Event{
			Action:     audit.ActionShiftChangeInvalidateCascade,
			TargetType: audit.TargetTypeShiftChangeRequest,
			TargetID:   &targetID,
			Metadata: map[string]any{
				"request_id":    req.ID,
				"reason":        "assignment_deleted",
				"assignment_id": input.AssignmentID,
			},
		})
	}

	return nil
}

func (s *PublicationService) ActivatePublication(
	ctx context.Context,
	publicationID int64,
) (*model.Publication, error) {
	if publicationID <= 0 {
		return nil, ErrInvalidInput
	}

	now := s.clock.Now()
	publication, err := s.publicationRepo.GetByID(ctx, publicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if model.ResolvePublicationState(publication, now) != model.PublicationStatePublished {
		return nil, ErrPublicationNotPublished
	}

	result, err := s.publicationRepo.ActivatePublication(ctx, repository.ActivatePublicationParams{
		ID:  publicationID,
		Now: now,
	})
	if errors.Is(err, sql.ErrNoRows) {
		_, reloadErr := s.publicationRepo.GetByID(ctx, publicationID)
		if reloadErr != nil {
			return nil, mapPublicationRepositoryError(reloadErr)
		}
		return nil, ErrPublicationNotPublished
	}
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	updated := result.Publication
	targetID := updated.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionPublicationActivate,
		TargetType: audit.TargetTypePublication,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"name": updated.Name,
		},
	})

	if len(result.ExpiredRequestIDs) > 0 {
		audit.Record(ctx, audit.Event{
			Action:     audit.ActionShiftChangeExpireBulk,
			TargetType: audit.TargetTypePublication,
			TargetID:   &targetID,
			Metadata: map[string]any{
				"expired_count":  len(result.ExpiredRequestIDs),
				"publication_id": targetID,
			},
		})
	}

	return publicationWithEffectiveState(updated, now), nil
}

// PublishPublication transitions an ASSIGNING publication to PUBLISHED.
// After publishing, employees can see the roster and create shift-change
// requests, but the assignments remain editable by the admin.
func (s *PublicationService) PublishPublication(
	ctx context.Context,
	publicationID int64,
) (*model.Publication, error) {
	if publicationID <= 0 {
		return nil, ErrInvalidInput
	}

	now := s.clock.Now()
	publication, err := s.publicationRepo.GetByID(ctx, publicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if model.ResolvePublicationState(publication, now) != model.PublicationStateAssigning {
		return nil, ErrPublicationNotAssigning
	}

	updated, err := s.publicationRepo.PublishPublication(ctx, repository.PublishPublicationParams{
		ID:  publicationID,
		Now: now,
	})
	if errors.Is(err, sql.ErrNoRows) {
		_, reloadErr := s.publicationRepo.GetByID(ctx, publicationID)
		if reloadErr != nil {
			return nil, mapPublicationRepositoryError(reloadErr)
		}
		return nil, ErrPublicationNotAssigning
	}
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	targetID := updated.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionPublicationPublish,
		TargetType: audit.TargetTypePublication,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"name": updated.Name,
		},
	})

	return publicationWithEffectiveState(updated, now), nil
}

func (s *PublicationService) EndPublication(
	ctx context.Context,
	publicationID int64,
) (*model.Publication, error) {
	if publicationID <= 0 {
		return nil, ErrInvalidInput
	}

	now := s.clock.Now()
	publication, err := s.publicationRepo.GetByID(ctx, publicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if model.ResolvePublicationState(publication, now) != model.PublicationStateActive {
		return nil, ErrPublicationNotActive
	}

	updated, err := s.UpdatePublication(ctx, UpdatePublicationInput{
		ID:                 publicationID,
		PlannedActiveUntil: &now,
	})
	if err != nil {
		return nil, err
	}

	targetID := updated.ID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionPublicationEnd,
		TargetType: audit.TargetTypePublication,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"name": updated.Name,
		},
	})

	return publicationWithEffectiveState(updated, now), nil
}

func (s *PublicationService) GetAssignmentBoard(
	ctx context.Context,
	publicationID int64,
) (*AssignmentBoardResult, error) {
	if publicationID <= 0 {
		return nil, ErrInvalidInput
	}

	now := s.clock.Now()
	publication, err := s.publicationRepo.GetByID(ctx, publicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	effectiveState := model.ResolvePublicationState(publication, now)
	if !isPublicationMutableForAssignments(effectiveState) {
		return nil, ErrPublicationNotAssigning
	}

	boardView, err := s.publicationRepo.GetAssignmentBoardView(ctx, publicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	employees, err := s.publicationRepo.ListAssignmentBoardEmployees(ctx, publicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	result := &AssignmentBoardResult{
		Publication: publicationWithEffectiveState(publication, now),
		Slots:       make([]*AssignmentBoardSlotResult, 0),
		Employees:   cloneAssignmentBoardEmployees(employees),
	}

	slotKeys := make([]repository.AssignmentBoardSlotKey, 0, len(boardView))
	for key := range boardView {
		slotKeys = append(slotKeys, key)
	}
	sort.Slice(slotKeys, func(i, j int) bool {
		left := boardView[slotKeys[i]].Slot
		right := boardView[slotKeys[j]].Slot
		leftWeekday := slotKeys[i].Weekday
		rightWeekday := slotKeys[j].Weekday
		switch {
		case leftWeekday != rightWeekday:
			return leftWeekday < rightWeekday
		case left.StartTime != right.StartTime:
			return left.StartTime < right.StartTime
		case left.EndTime != right.EndTime:
			return left.EndTime < right.EndTime
		default:
			return left.ID < right.ID
		}
	})

	for _, key := range slotKeys {
		slotView := boardView[key]
		slotResult := &AssignmentBoardSlotResult{
			Slot: &model.TemplateSlot{
				ID:         slotView.Slot.ID,
				TemplateID: slotView.Slot.TemplateID,
				Weekdays:   []int{key.Weekday},
				StartTime:  slotView.Slot.StartTime,
				EndTime:    slotView.Slot.EndTime,
			},
			Positions: make([]*AssignmentBoardPositionResult, 0, len(slotView.Positions)),
		}

		positionIDs := make([]int64, 0, len(slotView.Positions))
		for positionID := range slotView.Positions {
			positionIDs = append(positionIDs, positionID)
		}
		sort.Slice(positionIDs, func(i, j int) bool {
			return positionIDs[i] < positionIDs[j]
		})

		for _, positionID := range positionIDs {
			positionView := slotView.Positions[positionID]
			slotResult.Positions = append(slotResult.Positions, &AssignmentBoardPositionResult{
				Position: &model.Position{
					ID:   positionView.Position.ID,
					Name: positionView.Position.Name,
				},
				RequiredHeadcount: positionView.RequiredHeadcount,
				Assignments:       cloneAssignmentParticipants(positionView.Assignments),
			})
		}

		result.Slots = append(result.Slots, slotResult)
	}

	return result, nil
}

func cloneAssignmentBoardEmployees(
	employees []*model.AssignmentBoardEmployee,
) []*model.AssignmentBoardEmployee {
	if len(employees) == 0 {
		return make([]*model.AssignmentBoardEmployee, 0)
	}

	cloned := make([]*model.AssignmentBoardEmployee, 0, len(employees))
	for _, employee := range employees {
		if employee == nil {
			continue
		}
		clonedEmployee := *employee
		clonedEmployee.PositionIDs = append([]int64(nil), employee.PositionIDs...)
		sort.Slice(clonedEmployee.PositionIDs, func(i, j int) bool {
			return clonedEmployee.PositionIDs[i] < clonedEmployee.PositionIDs[j]
		})
		cloned = append(cloned, &clonedEmployee)
	}
	sort.Slice(cloned, func(i, j int) bool {
		return cloned[i].UserID < cloned[j].UserID
	})
	return cloned
}

func (s *PublicationService) GetPublicationRoster(
	ctx context.Context,
	publicationID int64,
	weekStart *time.Time,
) (*RosterResult, error) {
	if publicationID <= 0 {
		return nil, ErrInvalidInput
	}

	now := s.clock.Now()
	publication, err := s.publicationRepo.GetByID(ctx, publicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	effective := model.ResolvePublicationState(publication, now)
	if effective != model.PublicationStatePublished && effective != model.PublicationStateActive {
		return nil, ErrPublicationNotActive
	}

	resolvedWeekStart, err := resolveRosterWeekStart(publication, weekStart, now)
	if err != nil {
		return nil, err
	}

	return s.buildRoster(ctx, publication, resolvedWeekStart, now)
}

func isPublicationMutableForAssignments(state model.PublicationState) bool {
	return state == model.PublicationStateAssigning ||
		state == model.PublicationStatePublished ||
		state == model.PublicationStateActive
}

func (s *PublicationService) assignmentSnapshotForCascade(
	ctx context.Context,
	assignmentID int64,
) *model.Assignment {
	assignment, err := s.publicationRepo.GetAssignment(ctx, assignmentID)
	switch {
	case err == nil:
		return assignment
	case errors.Is(err, repository.ErrAssignmentNotFound):
		return nil
	default:
		s.logger.Warn("publication: load assignment for cascade", "assignment_id", assignmentID, "error", err)
		return nil
	}
}

func (s *PublicationService) enqueueShiftChangeRequestInvalidatedTx(
	ctx context.Context,
	tx *sql.Tx,
	req *model.ShiftChangeRequest,
	deletedAssignmentID int64,
	deletedAssignment *model.Assignment,
) error {
	if s.outboxRepo == nil {
		return nil
	}
	requester, err := s.publicationRepo.GetUserByID(ctx, req.RequesterUserID)
	if err != nil {
		return err
	}
	publication, err := s.publicationRepo.GetByID(ctx, req.PublicationID)
	if err != nil {
		return err
	}

	data := email.ShiftChangeResolvedData{
		To:            requester.Email,
		RecipientName: requester.Name,
		Outcome:       email.ShiftChangeOutcomeInvalidated,
		Type:          email.ShiftChangeType(req.Type),
		BaseURL:       s.appBaseURL,
	}
	requesterShift, err := s.resolveShiftChangeEmailShift(
		ctx,
		publication.ID,
		req.RequesterAssignmentID,
		deletedAssignmentID,
		deletedAssignment,
	)
	if err != nil {
		s.logger.Warn("publication: load requester shift for invalidation email", "request_id", req.ID, "error", err)
	} else if requesterShift != nil {
		data.RequesterShift = *requesterShift
	}
	if req.CounterpartAssignmentID != nil {
		counterpartShift, err := s.resolveShiftChangeEmailShift(
			ctx,
			publication.ID,
			*req.CounterpartAssignmentID,
			deletedAssignmentID,
			deletedAssignment,
		)
		if err != nil {
			s.logger.Warn("publication: load counterpart shift for invalidation email", "request_id", req.ID, "error", err)
		} else if counterpartShift != nil {
			data.CounterpartShift = counterpartShift
		}
	}

	msg := email.BuildShiftChangeResolvedMessage(data)
	return s.outboxRepo.EnqueueTx(ctx, tx, msg, repository.WithOutboxUserID(requester.ID))
}

func (s *PublicationService) resolveShiftChangeEmailShift(
	ctx context.Context,
	publicationID int64,
	assignmentID int64,
	deletedAssignmentID int64,
	deletedAssignment *model.Assignment,
) (*email.ShiftRef, error) {
	var assignment *model.Assignment
	if deletedAssignment != nil && assignmentID == deletedAssignmentID {
		assignment = deletedAssignment
	} else {
		var err error
		assignment, err = s.publicationRepo.GetAssignment(ctx, assignmentID)
		if err != nil {
			if errors.Is(err, repository.ErrAssignmentNotFound) {
				return nil, nil
			}
			return nil, err
		}
	}

	shifts, err := s.publicationRepo.ListPublicationShifts(ctx, publicationID)
	if err != nil {
		return nil, err
	}
	shift := findPublicationShiftForAssignment(buildPublicationShiftIndex(shifts), assignment)
	if shift == nil {
		return nil, ErrTemplateSlotPositionNotFound
	}

	ref := toShiftRef(shift)
	return &ref, nil
}

func (s *PublicationService) GetCurrentRoster(ctx context.Context) (*RosterResult, error) {
	now := s.clock.Now()
	publication, err := s.publicationRepo.GetCurrent(ctx)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if publication == nil {
		return &RosterResult{
			Publication: nil,
			WeekStart:   time.Time{},
			Weekdays:    make([]*RosterWeekdayResult, 0),
		}, nil
	}
	effective := model.ResolvePublicationState(publication, now)
	if effective != model.PublicationStatePublished && effective != model.PublicationStateActive {
		return &RosterResult{
			Publication: nil,
			WeekStart:   time.Time{},
			Weekdays:    make([]*RosterWeekdayResult, 0),
		}, nil
	}

	weekStart, err := resolveRosterWeekStart(publication, nil, now)
	if err != nil {
		return nil, err
	}
	return s.buildRoster(ctx, publication, weekStart, now)
}

func (s *PublicationService) buildRoster(
	ctx context.Context,
	publication *model.Publication,
	weekStart time.Time,
	now time.Time,
) (*RosterResult, error) {
	shifts, err := s.publicationRepo.ListPublicationShifts(ctx, publication.ID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	assignments, err := s.publicationRepo.ListPublicationAssignmentsForWeek(ctx, publication.ID, weekStart)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	assignmentsBySlotPosition := make(map[slotPositionKey][]*model.AssignmentParticipant)
	for _, assignment := range assignments {
		key := slotPositionKey{
			SlotID:     assignment.SlotID,
			Weekday:    assignment.Weekday,
			PositionID: assignment.PositionID,
		}
		assignmentsBySlotPosition[key] = append(assignmentsBySlotPosition[key], assignment)
	}

	weekdays := make([]*RosterWeekdayResult, 0, 7)
	slotsByWeekday := make(map[int][]*RosterSlotResult)
	slotByWeekdayAndID := make(map[int]map[int64]*RosterSlotResult)
	for _, shift := range shifts {
		occurrenceDate := occurrenceDateForWeekday(weekStart, shift.Weekday)
		occurrenceStart, err := model.OccurrenceStart(publicationShiftSlot(shift), occurrenceDate)
		if err != nil {
			return nil, err
		}
		if occurrenceStart.Before(publication.PlannedActiveFrom) || !occurrenceStart.Before(publication.PlannedActiveUntil) {
			continue
		}

		if slotByWeekdayAndID[shift.Weekday] == nil {
			slotByWeekdayAndID[shift.Weekday] = make(map[int64]*RosterSlotResult)
		}

		slotResult, ok := slotByWeekdayAndID[shift.Weekday][shift.SlotID]
		if !ok {
			slotResult = &RosterSlotResult{
				Slot:           publicationShiftSlot(shift),
				OccurrenceDate: occurrenceDate,
				Positions:      make([]*RosterPositionResult, 0),
			}
			slotByWeekdayAndID[shift.Weekday][shift.SlotID] = slotResult
			slotsByWeekday[shift.Weekday] = append(slotsByWeekday[shift.Weekday], slotResult)
		}

		key := slotPositionKey{
			SlotID:     shift.SlotID,
			Weekday:    shift.Weekday,
			PositionID: shift.PositionID,
		}
		slotResult.Positions = append(slotResult.Positions, &RosterPositionResult{
			Position:          publicationShiftPosition(shift),
			RequiredHeadcount: shift.RequiredHeadcount,
			Assignments:       cloneAssignmentParticipants(assignmentsBySlotPosition[key]),
		})
	}

	for weekday := 1; weekday <= 7; weekday++ {
		weekdays = append(weekdays, &RosterWeekdayResult{
			Weekday: weekday,
			Slots:   slotsByWeekday[weekday],
		})
		if weekdays[len(weekdays)-1].Slots == nil {
			weekdays[len(weekdays)-1].Slots = make([]*RosterSlotResult, 0)
		}
	}

	return &RosterResult{
		Publication: publicationWithEffectiveState(publication, now),
		WeekStart:   weekStart,
		Weekdays:    weekdays,
	}, nil
}

func resolveRosterWeekStart(
	publication *model.Publication,
	requested *time.Time,
	now time.Time,
) (time.Time, error) {
	if publication == nil {
		return time.Time{}, ErrPublicationNotFound
	}

	if requested != nil {
		weekStart := model.NormalizeOccurrenceDate(*requested)
		if weekdayToSlotValue(weekStart.Weekday()) != 1 {
			return time.Time{}, ErrInvalidOccurrenceDate
		}
		if weekStart.Before(model.NormalizeOccurrenceDate(publication.PlannedActiveFrom)) ||
			!weekStart.Before(model.NormalizeOccurrenceDate(publication.PlannedActiveUntil)) {
			return time.Time{}, ErrInvalidOccurrenceDate
		}
		return weekStart, nil
	}

	if !now.Before(publication.PlannedActiveFrom) && now.Before(publication.PlannedActiveUntil) {
		return mondayOf(model.NormalizeOccurrenceDate(now)), nil
	}
	return mondayOf(model.NormalizeOccurrenceDate(publication.PlannedActiveFrom)), nil
}

func occurrenceDateForWeekday(weekStart time.Time, weekday int) time.Time {
	return model.NormalizeOccurrenceDate(weekStart).AddDate(0, 0, weekday-1)
}

func mondayOf(date time.Time) time.Time {
	weekday := weekdayToSlotValue(date.Weekday())
	return model.NormalizeOccurrenceDate(date).AddDate(0, 0, -(weekday - 1))
}

func weekdayToSlotValue(weekday time.Weekday) int {
	if weekday == time.Sunday {
		return 7
	}
	return int(weekday)
}

type slotPositionKey struct {
	SlotID     int64
	Weekday    int
	PositionID int64
}

type slotCellKey struct {
	SlotID  int64
	Weekday int
}

func publicationShiftSlot(shift *model.PublicationShift) *model.TemplateSlot {
	if shift == nil {
		return nil
	}

	return &model.TemplateSlot{
		ID:         shift.SlotID,
		TemplateID: shift.TemplateID,
		Weekdays:   []int{shift.Weekday},
		StartTime:  shift.StartTime,
		EndTime:    shift.EndTime,
		CreatedAt:  shift.CreatedAt,
		UpdatedAt:  shift.UpdatedAt,
		Positions:  make([]*model.TemplateSlotPosition, 0),
	}
}

func publicationShiftPosition(shift *model.PublicationShift) *model.Position {
	if shift == nil {
		return nil
	}

	return &model.Position{
		ID:   shift.PositionID,
		Name: shift.PositionName,
	}
}

func cloneAssignmentParticipants(participants []*model.AssignmentParticipant) []*model.AssignmentParticipant {
	if len(participants) == 0 {
		return make([]*model.AssignmentParticipant, 0)
	}

	cloned := make([]*model.AssignmentParticipant, 0, len(participants))
	for _, participant := range participants {
		if participant == nil {
			continue
		}
		clonedParticipant := *participant
		cloned = append(cloned, &clonedParticipant)
	}
	sort.Slice(cloned, func(i, j int) bool {
		return cloned[i].UserID < cloned[j].UserID
	})
	return cloned
}
