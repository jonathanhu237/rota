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
}

type AssignmentBoardSlotResult struct {
	Slot      *model.TemplateSlot
	Positions []*AssignmentBoardPositionResult
}

type AssignmentBoardPositionResult struct {
	Position              *model.Position
	RequiredHeadcount     int
	Candidates            []*model.AssignmentCandidate
	NonCandidateQualified []*model.AssignmentCandidate
	Assignments           []*model.AssignmentParticipant
}

type RosterResult struct {
	Publication *model.Publication
	Weekdays    []*RosterWeekdayResult
}

type RosterWeekdayResult struct {
	Weekday int
	Slots   []*RosterSlotResult
}

type RosterSlotResult struct {
	Slot      *model.TemplateSlot
	Positions []*RosterPositionResult
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
	if input.PublicationID <= 0 || input.UserID <= 0 || input.SlotID <= 0 || input.PositionID <= 0 {
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

	existingAssignments, err := s.publicationRepo.ListUserAssignmentsOnWeekdayInPublication(
		ctx,
		input.PublicationID,
		input.UserID,
		slot.Weekday,
	)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	for _, assignment := range existingAssignments {
		if assignment.SlotID == input.SlotID {
			return nil, ErrAssignmentUserAlreadyInSlot
		}
	}
	if hasAssignmentTimeConflict(existingAssignments, slot.StartTime, slot.EndTime) {
		return nil, ErrAssignmentTimeConflict
	}

	assignment, err := s.publicationRepo.CreateAssignment(ctx, repository.CreateAssignmentParams{
		PublicationID: input.PublicationID,
		UserID:        input.UserID,
		SlotID:        input.SlotID,
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

func hasAssignmentTimeConflict(
	assignments []*model.AssignmentSlotView,
	startTime, endTime string,
) bool {
	targetStartMinutes, err := parseClockMinutes(startTime)
	if err != nil {
		return false
	}
	targetEndMinutes, err := parseClockMinutes(endTime)
	if err != nil {
		return false
	}

	for _, assignment := range assignments {
		assignmentStartMinutes, err := parseClockMinutes(assignment.StartTime)
		if err != nil {
			continue
		}
		assignmentEndMinutes, err := parseClockMinutes(assignment.EndTime)
		if err != nil {
			continue
		}

		if assignmentStartMinutes < targetEndMinutes && targetStartMinutes < assignmentEndMinutes {
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

	if err := s.publicationRepo.DeleteAssignment(ctx, repository.DeleteAssignmentParams{
		PublicationID: input.PublicationID,
		AssignmentID:  input.AssignmentID,
	}); err != nil {
		return mapPublicationRepositoryError(err)
	}

	targetID := input.AssignmentID
	metadata := map[string]any{
		"publication_id": input.PublicationID,
	}
	if deletedAssignment != nil {
		metadata["user_id"] = deletedAssignment.UserID
		metadata["slot_id"] = deletedAssignment.SlotID
		metadata["position_id"] = deletedAssignment.PositionID
	}
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionAssignmentDelete,
		TargetType: audit.TargetTypeAssignment,
		TargetID:   &targetID,
		Metadata:   metadata,
	})

	s.invalidateShiftChangeRequestsForDeletedAssignment(ctx, input.AssignmentID, deletedAssignment, now)

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

	updated, err := s.publicationRepo.EndPublication(ctx, repository.EndPublicationParams{
		ID:  publicationID,
		Now: now,
	})
	if errors.Is(err, sql.ErrNoRows) {
		_, reloadErr := s.publicationRepo.GetByID(ctx, publicationID)
		if reloadErr != nil {
			return nil, mapPublicationRepositoryError(reloadErr)
		}
		return nil, ErrPublicationNotActive
	}
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
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

	result := &AssignmentBoardResult{
		Publication: publicationWithEffectiveState(publication, now),
		Slots:       make([]*AssignmentBoardSlotResult, 0),
	}

	slotIDs := make([]int64, 0, len(boardView))
	for slotID := range boardView {
		slotIDs = append(slotIDs, slotID)
	}
	sort.Slice(slotIDs, func(i, j int) bool {
		left := boardView[slotIDs[i]].Slot
		right := boardView[slotIDs[j]].Slot
		switch {
		case left.Weekday != right.Weekday:
			return left.Weekday < right.Weekday
		case left.StartTime != right.StartTime:
			return left.StartTime < right.StartTime
		case left.EndTime != right.EndTime:
			return left.EndTime < right.EndTime
		default:
			return left.ID < right.ID
		}
	})

	for _, slotID := range slotIDs {
		slotView := boardView[slotID]
		slotResult := &AssignmentBoardSlotResult{
			Slot: &model.TemplateSlot{
				ID:         slotView.Slot.ID,
				TemplateID: slotView.Slot.TemplateID,
				Weekday:    slotView.Slot.Weekday,
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
				RequiredHeadcount:     positionView.RequiredHeadcount,
				Candidates:            cloneAssignmentCandidates(positionView.Candidates),
				NonCandidateQualified: cloneAssignmentCandidates(positionView.NonCandidateQualified),
				Assignments:           cloneAssignmentParticipants(positionView.Assignments),
			})
		}

		result.Slots = append(result.Slots, slotResult)
	}

	return result, nil
}

func (s *PublicationService) GetPublicationRoster(
	ctx context.Context,
	publicationID int64,
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

	return s.buildRoster(ctx, publication, now)
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

func (s *PublicationService) invalidateShiftChangeRequestsForDeletedAssignment(
	ctx context.Context,
	assignmentID int64,
	deletedAssignment *model.Assignment,
	now time.Time,
) {
	if s.shiftChangeRepo == nil {
		return
	}

	requestIDs, err := s.shiftChangeRepo.InvalidateRequestsForAssignment(ctx, assignmentID, now)
	if err != nil {
		s.logger.Warn("publication: invalidate shift change requests for deleted assignment", "assignment_id", assignmentID, "error", err)
		return
	}

	for _, requestID := range requestIDs {
		targetID := requestID
		audit.Record(ctx, audit.Event{
			Action:     audit.ActionShiftChangeInvalidateCascade,
			TargetType: audit.TargetTypeShiftChangeRequest,
			TargetID:   &targetID,
			Metadata: map[string]any{
				"request_id":    requestID,
				"reason":        "assignment_deleted",
				"assignment_id": assignmentID,
			},
		})
		s.notifyShiftChangeRequestInvalidated(ctx, requestID, assignmentID, deletedAssignment)
	}
}

func (s *PublicationService) notifyShiftChangeRequestInvalidated(
	ctx context.Context,
	requestID int64,
	deletedAssignmentID int64,
	deletedAssignment *model.Assignment,
) {
	if s.emailer == nil || s.shiftChangeRepo == nil {
		return
	}

	req, err := s.shiftChangeRepo.GetByID(ctx, requestID)
	if err != nil {
		s.logger.Warn("publication: load shift change request for invalidation email", "request_id", requestID, "error", err)
		return
	}
	requester, err := s.publicationRepo.GetUserByID(ctx, req.RequesterUserID)
	if err != nil {
		s.logger.Warn("publication: load requester for invalidation email", "request_id", requestID, "error", err)
		return
	}
	publication, err := s.publicationRepo.GetByID(ctx, req.PublicationID)
	if err != nil {
		s.logger.Warn("publication: load publication for invalidation email", "request_id", requestID, "error", err)
		return
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
		s.logger.Warn("publication: load requester shift for invalidation email", "request_id", requestID, "error", err)
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
			s.logger.Warn("publication: load counterpart shift for invalidation email", "request_id", requestID, "error", err)
		} else if counterpartShift != nil {
			data.CounterpartShift = counterpartShift
		}
	}

	msg := email.BuildShiftChangeResolvedMessage(data)
	if err := s.emailer.Send(ctx, msg); err != nil {
		s.logger.Warn("publication: send invalidation email", "request_id", requestID, "error", err)
	}
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
			Weekdays:    make([]*RosterWeekdayResult, 0),
		}, nil
	}
	effective := model.ResolvePublicationState(publication, now)
	if effective != model.PublicationStatePublished && effective != model.PublicationStateActive {
		return &RosterResult{
			Publication: nil,
			Weekdays:    make([]*RosterWeekdayResult, 0),
		}, nil
	}

	return s.buildRoster(ctx, publication, now)
}

func (s *PublicationService) buildRoster(
	ctx context.Context,
	publication *model.Publication,
	now time.Time,
) (*RosterResult, error) {
	shifts, err := s.publicationRepo.ListPublicationShifts(ctx, publication.ID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	assignments, err := s.publicationRepo.ListPublicationAssignments(ctx, publication.ID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	assignmentsBySlotPosition := make(map[slotPositionKey][]*model.AssignmentParticipant)
	for _, assignment := range assignments {
		key := slotPositionKey{
			SlotID:     assignment.SlotID,
			PositionID: assignment.PositionID,
		}
		assignmentsBySlotPosition[key] = append(assignmentsBySlotPosition[key], assignment)
	}

	weekdays := make([]*RosterWeekdayResult, 0, 7)
	slotsByWeekday := make(map[int][]*RosterSlotResult)
	slotByWeekdayAndID := make(map[int]map[int64]*RosterSlotResult)
	for _, shift := range shifts {
		if slotByWeekdayAndID[shift.Weekday] == nil {
			slotByWeekdayAndID[shift.Weekday] = make(map[int64]*RosterSlotResult)
		}

		slotResult, ok := slotByWeekdayAndID[shift.Weekday][shift.SlotID]
		if !ok {
			slotResult = &RosterSlotResult{
				Slot:      publicationShiftSlot(shift),
				Positions: make([]*RosterPositionResult, 0),
			}
			slotByWeekdayAndID[shift.Weekday][shift.SlotID] = slotResult
			slotsByWeekday[shift.Weekday] = append(slotsByWeekday[shift.Weekday], slotResult)
		}

		key := slotPositionKey{
			SlotID:     shift.SlotID,
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
		Weekdays:    weekdays,
	}, nil
}

type slotPositionKey struct {
	SlotID     int64
	PositionID int64
}

func publicationShiftSlot(shift *model.PublicationShift) *model.TemplateSlot {
	if shift == nil {
		return nil
	}

	return &model.TemplateSlot{
		ID:         shift.SlotID,
		TemplateID: shift.TemplateID,
		Weekday:    shift.Weekday,
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

func cloneAssignmentCandidates(candidates []*model.AssignmentCandidate) []*model.AssignmentCandidate {
	if len(candidates) == 0 {
		return make([]*model.AssignmentCandidate, 0)
	}

	cloned := make([]*model.AssignmentCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		clonedCandidate := *candidate
		cloned = append(cloned, &clonedCandidate)
	}
	sort.Slice(cloned, func(i, j int) bool {
		return cloned[i].UserID < cloned[j].UserID
	})
	return cloned
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

func filterNonCandidateQualified(
	qualified []*model.AssignmentCandidate,
	candidates []*model.AssignmentCandidate,
	assignments []*model.AssignmentParticipant,
) []*model.AssignmentCandidate {
	if len(qualified) == 0 {
		return make([]*model.AssignmentCandidate, 0)
	}

	excludedUserIDs := make(map[int64]struct{}, len(candidates)+len(assignments))
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		excludedUserIDs[candidate.UserID] = struct{}{}
	}
	for _, assignment := range assignments {
		if assignment == nil {
			continue
		}
		excludedUserIDs[assignment.UserID] = struct{}{}
	}

	filtered := make([]*model.AssignmentCandidate, 0, len(qualified))
	for _, candidate := range qualified {
		if candidate == nil {
			continue
		}
		if _, ok := excludedUserIDs[candidate.UserID]; ok {
			continue
		}
		clonedCandidate := *candidate
		filtered = append(filtered, &clonedCandidate)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].UserID < filtered[j].UserID
	})

	return filtered
}
