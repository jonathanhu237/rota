package service

import (
	"context"
	"errors"
	"strings"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

var (
	ErrPositionInUse    = errors.New("position in use")
	ErrPositionNotFound = errors.New("position not found")
)

type positionRepository interface {
	ListPaginated(ctx context.Context, params repository.ListPositionsParams) ([]*model.Position, int, error)
	GetByID(ctx context.Context, id int64) (*model.Position, error)
	Create(ctx context.Context, params repository.CreatePositionParams) (*model.Position, error)
	Update(ctx context.Context, params repository.UpdatePositionParams) (*model.Position, error)
	Delete(ctx context.Context, id int64) error
}

type PositionService struct {
	positionRepo positionRepository
}

type ListPositionsInput struct {
	Page     int
	PageSize int
}

type ListPositionsResult struct {
	Positions  []*model.Position
	Page       int
	PageSize   int
	Total      int
	TotalPages int
}

type CreatePositionInput struct {
	Name        string
	Description string
}

type UpdatePositionInput struct {
	ID          int64
	Name        string
	Description string
}

func NewPositionService(positionRepo positionRepository) *PositionService {
	return &PositionService{
		positionRepo: positionRepo,
	}
}

func (s *PositionService) ListPositions(ctx context.Context, input ListPositionsInput) (*ListPositionsResult, error) {
	page, pageSize, err := normalizePagination(input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	positions, total, err := s.positionRepo.ListPaginated(ctx, repository.ListPositionsParams{
		Offset: offset,
		Limit:  pageSize,
	})
	if err != nil {
		return nil, err
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	return &ListPositionsResult{
		Positions:  positions,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

func (s *PositionService) GetPositionByID(ctx context.Context, id int64) (*model.Position, error) {
	if id <= 0 {
		return nil, ErrInvalidInput
	}

	position, err := s.positionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, mapPositionRepositoryError(err)
	}

	return position, nil
}

func (s *PositionService) CreatePosition(ctx context.Context, input CreatePositionInput) (*model.Position, error) {
	name, description, err := normalizePositionInput(input.Name, input.Description)
	if err != nil {
		return nil, err
	}

	position, err := s.positionRepo.Create(ctx, repository.CreatePositionParams{
		Name:        name,
		Description: description,
	})
	if err != nil {
		return nil, err
	}

	return position, nil
}

func (s *PositionService) UpdatePosition(ctx context.Context, input UpdatePositionInput) (*model.Position, error) {
	if input.ID <= 0 {
		return nil, ErrInvalidInput
	}

	name, description, err := normalizePositionInput(input.Name, input.Description)
	if err != nil {
		return nil, err
	}

	position, err := s.positionRepo.Update(ctx, repository.UpdatePositionParams{
		ID:          input.ID,
		Name:        name,
		Description: description,
	})
	if err != nil {
		return nil, mapPositionRepositoryError(err)
	}

	return position, nil
}

func (s *PositionService) DeletePosition(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidInput
	}

	if err := s.positionRepo.Delete(ctx, id); err != nil {
		return mapPositionRepositoryError(err)
	}

	return nil
}

func normalizePositionInput(name, description string) (string, string, error) {
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		return "", "", ErrInvalidInput
	}

	return normalizedName, strings.TrimSpace(description), nil
}

func mapPositionRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrPositionInUse):
		return ErrPositionInUse
	case errors.Is(err, repository.ErrPositionNotFound):
		return ErrPositionNotFound
	default:
		return err
	}
}
