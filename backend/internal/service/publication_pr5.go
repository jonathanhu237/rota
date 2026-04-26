package service

import (
	"context"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

func (s *PublicationService) AutoAssignPublication(
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
	if model.ResolvePublicationState(publication, now) != model.PublicationStateAssigning {
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

	solverSlotPositions := make([]AutoAssignSlotPosition, 0, len(shifts))
	for _, shift := range shifts {
		if shift == nil {
			continue
		}
		solverSlotPositions = append(solverSlotPositions, AutoAssignSlotPosition{
			SlotID:            shift.SlotID,
			PositionID:        shift.PositionID,
			Weekday:           shift.Weekday,
			StartTime:         shift.StartTime,
			EndTime:           shift.EndTime,
			RequiredHeadcount: shift.RequiredHeadcount,
		})
	}

	solverCandidates := make([]AutoAssignCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		solverCandidates = append(solverCandidates, AutoAssignCandidate{
			UserID:     candidate.UserID,
			SlotID:     candidate.SlotID,
			Weekday:    candidate.Weekday,
			PositionID: candidate.PositionID,
		})
	}

	solvedAssignments, err := SolveAutoAssignments(solverSlotPositions, solverCandidates)
	if err != nil {
		return nil, err
	}

	replacementAssignments := make([]repository.ReplaceAssignmentParams, 0, len(solvedAssignments))
	for _, assignment := range solvedAssignments {
		replacementAssignments = append(replacementAssignments, repository.ReplaceAssignmentParams{
			UserID:     assignment.UserID,
			SlotID:     assignment.SlotID,
			Weekday:    assignment.Weekday,
			PositionID: assignment.PositionID,
		})
	}

	if err := s.publicationRepo.ReplaceAssignments(ctx, repository.ReplaceAssignmentsParams{
		PublicationID: publicationID,
		Assignments:   replacementAssignments,
		CreatedAt:     now,
	}); err != nil {
		return nil, mapPublicationRepositoryError(err)
	}

	targetID := publicationID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionPublicationAutoAssign,
		TargetType: audit.TargetTypePublication,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"assignments_created": len(solvedAssignments),
		},
	})

	return s.GetAssignmentBoard(ctx, publicationID)
}
