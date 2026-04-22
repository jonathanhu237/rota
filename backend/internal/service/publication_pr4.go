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
	Shifts      []*AssignmentBoardShiftResult
}

type AssignmentBoardShiftResult struct {
	Shift                 *model.PublicationShift
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
	Shifts  []*RosterShiftResult
}

type RosterShiftResult struct {
	Shift       *model.PublicationShift
	Assignments []*model.AssignmentParticipant
}

func (s *PublicationService) CreateAssignment(
	ctx context.Context,
	input CreateAssignmentInput,
) (*model.Assignment, error) {
	if input.PublicationID <= 0 || input.UserID <= 0 || input.TemplateShiftID <= 0 {
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

	shift, err := s.publicationRepo.GetTemplateShift(ctx, publication.TemplateID, input.TemplateShiftID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if shift.TemplateID != publication.TemplateID {
		return nil, ErrTemplateShiftNotFound
	}

	user, err := s.publicationRepo.GetUserByID(ctx, input.UserID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if user.Status == model.UserStatusDisabled {
		return nil, ErrUserDisabled
	}

	assignment, err := s.publicationRepo.CreateAssignment(ctx, repository.CreateAssignmentParams{
		PublicationID:   input.PublicationID,
		UserID:          input.UserID,
		TemplateShiftID: input.TemplateShiftID,
		CreatedAt:       now,
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
			"publication_id":    assignment.PublicationID,
			"user_id":           assignment.UserID,
			"template_shift_id": assignment.TemplateShiftID,
		},
	})

	return assignment, nil
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
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionAssignmentDelete,
		TargetType: audit.TargetTypeAssignment,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"publication_id": input.PublicationID,
		},
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

	shifts, err := s.publicationRepo.ListPublicationShifts(ctx, publicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	candidates, err := s.publicationRepo.ListAssignmentCandidates(ctx, publicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	positionIDs := make([]int64, 0, len(shifts))
	seenPositionIDs := make(map[int64]struct{}, len(shifts))
	for _, shift := range shifts {
		if _, ok := seenPositionIDs[shift.PositionID]; ok {
			continue
		}
		seenPositionIDs[shift.PositionID] = struct{}{}
		positionIDs = append(positionIDs, shift.PositionID)
	}
	qualifiedByPosition, err := s.publicationRepo.ListQualifiedUsersForPositions(ctx, positionIDs)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	assignments, err := s.publicationRepo.ListPublicationAssignments(ctx, publicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	candidatesByShift := make(map[int64][]*model.AssignmentCandidate)
	for _, candidate := range candidates {
		candidatesByShift[candidate.TemplateShiftID] = append(candidatesByShift[candidate.TemplateShiftID], candidate)
	}
	assignmentsByShift := make(map[int64][]*model.AssignmentParticipant)
	for _, assignment := range assignments {
		assignmentsByShift[assignment.TemplateShiftID] = append(assignmentsByShift[assignment.TemplateShiftID], assignment)
	}

	result := &AssignmentBoardResult{
		Publication: publicationWithEffectiveState(publication, now),
		Shifts:      make([]*AssignmentBoardShiftResult, 0, len(shifts)),
	}
	for _, shift := range shifts {
		shiftCandidates := candidatesByShift[shift.ID]
		shiftAssignments := assignmentsByShift[shift.ID]

		result.Shifts = append(result.Shifts, &AssignmentBoardShiftResult{
			Shift:      shift,
			Candidates: cloneAssignmentCandidates(shiftCandidates),
			NonCandidateQualified: filterNonCandidateQualified(
				qualifiedByPosition[shift.PositionID],
				shiftCandidates,
				shiftAssignments,
			),
			Assignments: cloneAssignmentParticipants(shiftAssignments),
		})
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
		publication.TemplateID,
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
			publication.TemplateID,
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
	templateID int64,
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

	shift, err := s.publicationRepo.GetTemplateShift(ctx, templateID, assignment.TemplateShiftID)
	if err != nil {
		return nil, err
	}

	ref := toShiftRef(shift, nil)
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

	assignmentsByShift := make(map[int64][]*model.AssignmentParticipant)
	for _, assignment := range assignments {
		assignmentsByShift[assignment.TemplateShiftID] = append(assignmentsByShift[assignment.TemplateShiftID], assignment)
	}

	weekdays := make([]*RosterWeekdayResult, 0, 7)
	shiftsByWeekday := make(map[int][]*RosterShiftResult)
	for _, shift := range shifts {
		shiftsByWeekday[shift.Weekday] = append(shiftsByWeekday[shift.Weekday], &RosterShiftResult{
			Shift:       shift,
			Assignments: cloneAssignmentParticipants(assignmentsByShift[shift.ID]),
		})
	}

	for weekday := 1; weekday <= 7; weekday++ {
		weekdays = append(weekdays, &RosterWeekdayResult{
			Weekday: weekday,
			Shifts:  shiftsByWeekday[weekday],
		})
		if weekdays[len(weekdays)-1].Shifts == nil {
			weekdays[len(weekdays)-1].Shifts = make([]*RosterShiftResult, 0)
		}
	}

	return &RosterResult{
		Publication: publicationWithEffectiveState(publication, now),
		Weekdays:    weekdays,
	}, nil
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
