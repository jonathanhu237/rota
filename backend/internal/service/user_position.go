package service

import (
	"context"
	"errors"
	"sort"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

type userPositionRepository interface {
	ListPositionsByUserID(ctx context.Context, userID int64) ([]*model.Position, error)
	ReplacePositionsByUserID(ctx context.Context, userID int64, positionIDs []int64) error
}

type UserPositionService struct {
	userPositionRepo userPositionRepository
}

type ReplaceUserPositionsInput struct {
	UserID      int64
	PositionIDs []int64
}

func NewUserPositionService(userPositionRepo userPositionRepository) *UserPositionService {
	return &UserPositionService{
		userPositionRepo: userPositionRepo,
	}
}

func (s *UserPositionService) ListUserPositions(ctx context.Context, userID int64) ([]*model.Position, error) {
	if userID <= 0 {
		return nil, ErrInvalidInput
	}

	positions, err := s.userPositionRepo.ListPositionsByUserID(ctx, userID)
	if err != nil {
		return nil, mapUserPositionRepositoryError(err)
	}

	return positions, nil
}

func (s *UserPositionService) ReplaceUserPositions(ctx context.Context, input ReplaceUserPositionsInput) error {
	if input.UserID <= 0 {
		return ErrInvalidInput
	}

	positionIDs, err := normalizePositionIDs(input.PositionIDs)
	if err != nil {
		return err
	}

	if err := s.userPositionRepo.ReplacePositionsByUserID(ctx, input.UserID, positionIDs); err != nil {
		return mapUserPositionRepositoryError(err)
	}

	targetID := input.UserID
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionUserQualificationsReplace,
		TargetType: audit.TargetTypeUser,
		TargetID:   &targetID,
		Metadata: map[string]any{
			"position_ids": positionIDs,
		},
	})

	return nil
}

func normalizePositionIDs(positionIDs []int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(positionIDs))
	normalized := make([]int64, 0, len(positionIDs))
	for _, positionID := range positionIDs {
		if positionID <= 0 {
			return nil, ErrInvalidInput
		}
		if _, ok := seen[positionID]; ok {
			continue
		}
		seen[positionID] = struct{}{}
		normalized = append(normalized, positionID)
	}

	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i] < normalized[j]
	})

	return normalized, nil
}

func mapUserPositionRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrUserNotFound):
		return ErrUserNotFound
	case errors.Is(err, repository.ErrPositionNotFound):
		return ErrPositionNotFound
	default:
		return err
	}
}
