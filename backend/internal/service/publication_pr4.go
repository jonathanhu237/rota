package service

import (
	"context"
	"database/sql"
	"errors"
	"sort"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

type AssignmentBoardResult struct {
	Publication *model.Publication
	Shifts      []*AssignmentBoardShiftResult
}

type AssignmentBoardShiftResult struct {
	Shift       *model.PublicationShift
	Candidates  []*model.AssignmentCandidate
	Assignments []*model.AssignmentParticipant
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

	publication, err := s.publicationRepo.GetByID(ctx, input.PublicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if model.ResolvePublicationState(publication, s.clock.Now()) != model.PublicationStateAssigning {
		return nil, ErrPublicationNotAssigning
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
		CreatedAt:       s.clock.Now(),
	})
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	return assignment, nil
}

func (s *PublicationService) DeleteAssignment(
	ctx context.Context,
	input DeleteAssignmentInput,
) error {
	if input.PublicationID <= 0 || input.AssignmentID <= 0 {
		return ErrInvalidInput
	}

	publication, err := s.publicationRepo.GetByID(ctx, input.PublicationID)
	if err != nil {
		return mapPublicationRepositoryError(err)
	}
	if model.ResolvePublicationState(publication, s.clock.Now()) != model.PublicationStateAssigning {
		return ErrPublicationNotAssigning
	}

	if err := s.publicationRepo.DeleteAssignment(ctx, repository.DeleteAssignmentParams{
		PublicationID: input.PublicationID,
		AssignmentID:  input.AssignmentID,
	}); err != nil {
		return mapPublicationRepositoryError(err)
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

	publication, err := s.publicationRepo.GetByID(ctx, publicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if model.ResolvePublicationState(publication, s.clock.Now()) != model.PublicationStateAssigning {
		return nil, ErrPublicationNotAssigning
	}

	updated, err := s.publicationRepo.ActivatePublication(ctx, repository.ActivatePublicationParams{
		ID:  publicationID,
		Now: s.clock.Now(),
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

	return publicationWithEffectiveState(updated, s.clock.Now()), nil
}

func (s *PublicationService) EndPublication(
	ctx context.Context,
	publicationID int64,
) (*model.Publication, error) {
	if publicationID <= 0 {
		return nil, ErrInvalidInput
	}

	publication, err := s.publicationRepo.GetByID(ctx, publicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if model.ResolvePublicationState(publication, s.clock.Now()) != model.PublicationStateActive {
		return nil, ErrPublicationNotActive
	}

	updated, err := s.publicationRepo.EndPublication(ctx, repository.EndPublicationParams{
		ID:  publicationID,
		Now: s.clock.Now(),
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

	return publicationWithEffectiveState(updated, s.clock.Now()), nil
}

func (s *PublicationService) GetAssignmentBoard(
	ctx context.Context,
	publicationID int64,
) (*AssignmentBoardResult, error) {
	if publicationID <= 0 {
		return nil, ErrInvalidInput
	}

	publication, err := s.publicationRepo.GetByID(ctx, publicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	effectiveState := model.ResolvePublicationState(publication, s.clock.Now())
	if effectiveState != model.PublicationStateAssigning && effectiveState != model.PublicationStateActive {
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
		Publication: publicationWithEffectiveState(publication, s.clock.Now()),
		Shifts:      make([]*AssignmentBoardShiftResult, 0, len(shifts)),
	}
	for _, shift := range shifts {
		result.Shifts = append(result.Shifts, &AssignmentBoardShiftResult{
			Shift:       shift,
			Candidates:  cloneAssignmentCandidates(candidatesByShift[shift.ID]),
			Assignments: cloneAssignmentParticipants(assignmentsByShift[shift.ID]),
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

	publication, err := s.publicationRepo.GetByID(ctx, publicationID)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if model.ResolvePublicationState(publication, s.clock.Now()) != model.PublicationStateActive {
		return nil, ErrPublicationNotActive
	}

	return s.buildRoster(ctx, publication)
}

func (s *PublicationService) GetCurrentRoster(ctx context.Context) (*RosterResult, error) {
	publication, err := s.publicationRepo.GetCurrent(ctx)
	if err != nil {
		return nil, mapPublicationRepositoryError(err)
	}
	if publication == nil || model.ResolvePublicationState(publication, s.clock.Now()) != model.PublicationStateActive {
		return &RosterResult{
			Publication: nil,
			Weekdays:    make([]*RosterWeekdayResult, 0),
		}, nil
	}

	return s.buildRoster(ctx, publication)
}

func (s *PublicationService) buildRoster(
	ctx context.Context,
	publication *model.Publication,
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
		Publication: publicationWithEffectiveState(publication, s.clock.Now()),
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
		copy := *candidate
		cloned = append(cloned, &copy)
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
		copy := *participant
		cloned = append(cloned, &copy)
	}
	sort.Slice(cloned, func(i, j int) bool {
		return cloned[i].UserID < cloned[j].UserID
	})
	return cloned
}
