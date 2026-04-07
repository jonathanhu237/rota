package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

type positionRepositoryMock struct {
	listPaginatedFunc func(ctx context.Context, params repository.ListPositionsParams) ([]*model.Position, int, error)
	getByIDFunc       func(ctx context.Context, id int64) (*model.Position, error)
	createFunc        func(ctx context.Context, params repository.CreatePositionParams) (*model.Position, error)
	updateFunc        func(ctx context.Context, params repository.UpdatePositionParams) (*model.Position, error)
	deleteFunc        func(ctx context.Context, id int64) error
}

func (m *positionRepositoryMock) ListPaginated(ctx context.Context, params repository.ListPositionsParams) ([]*model.Position, int, error) {
	return m.listPaginatedFunc(ctx, params)
}

func (m *positionRepositoryMock) GetByID(ctx context.Context, id int64) (*model.Position, error) {
	return m.getByIDFunc(ctx, id)
}

func (m *positionRepositoryMock) Create(ctx context.Context, params repository.CreatePositionParams) (*model.Position, error) {
	return m.createFunc(ctx, params)
}

func (m *positionRepositoryMock) Update(ctx context.Context, params repository.UpdatePositionParams) (*model.Position, error) {
	return m.updateFunc(ctx, params)
}

func (m *positionRepositoryMock) Delete(ctx context.Context, id int64) error {
	return m.deleteFunc(ctx, id)
}

func TestPositionServiceListPositions(t *testing.T) {
	t.Run("success with pagination", func(t *testing.T) {
		t.Parallel()

		var receivedParams repository.ListPositionsParams
		service := NewPositionService(&positionRepositoryMock{
			listPaginatedFunc: func(ctx context.Context, params repository.ListPositionsParams) ([]*model.Position, int, error) {
				receivedParams = params
				return []*model.Position{{ID: 1}, {ID: 2}}, 21, nil
			},
		})

		result, err := service.ListPositions(context.Background(), ListPositionsInput{
			Page:     3,
			PageSize: 4,
		})
		if err != nil {
			t.Fatalf("ListPositions returned error: %v", err)
		}
		if receivedParams.Offset != 8 || receivedParams.Limit != 4 {
			t.Fatalf("expected offset=8 limit=4, got offset=%d limit=%d", receivedParams.Offset, receivedParams.Limit)
		}
		if result.Page != 3 || result.PageSize != 4 || result.Total != 21 || result.TotalPages != 6 {
			t.Fatalf("unexpected pagination result: %+v", result)
		}
	})

	t.Run("default pagination", func(t *testing.T) {
		t.Parallel()

		var receivedParams repository.ListPositionsParams
		service := NewPositionService(&positionRepositoryMock{
			listPaginatedFunc: func(ctx context.Context, params repository.ListPositionsParams) ([]*model.Position, int, error) {
				receivedParams = params
				return nil, 0, nil
			},
		})

		result, err := service.ListPositions(context.Background(), ListPositionsInput{})
		if err != nil {
			t.Fatalf("ListPositions returned error: %v", err)
		}
		if receivedParams.Offset != 0 || receivedParams.Limit != defaultUserListPageSize {
			t.Fatalf("expected default offset=0 limit=%d, got offset=%d limit=%d", defaultUserListPageSize, receivedParams.Offset, receivedParams.Limit)
		}
		if result.Page != defaultUserListPage || result.PageSize != defaultUserListPageSize {
			t.Fatalf("unexpected defaults: %+v", result)
		}
	})
}

func TestPositionServiceCreatePosition(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		service := NewPositionService(&positionRepositoryMock{
			createFunc: func(ctx context.Context, params repository.CreatePositionParams) (*model.Position, error) {
				if params.Name != "Front Desk" {
					t.Fatalf("expected trimmed name, got %q", params.Name)
				}
				if params.Description != "Handles arrivals" {
					t.Fatalf("expected trimmed description, got %q", params.Description)
				}
				return &model.Position{
					ID:          1,
					Name:        params.Name,
					Description: params.Description,
					CreatedAt:   now,
					UpdatedAt:   now,
				}, nil
			},
		})

		position, err := service.CreatePosition(context.Background(), CreatePositionInput{
			Name:        " Front Desk ",
			Description: " Handles arrivals ",
		})
		if err != nil {
			t.Fatalf("CreatePosition returned error: %v", err)
		}
		if position.Name != "Front Desk" || position.Description != "Handles arrivals" {
			t.Fatalf("unexpected created position: %+v", position)
		}
	})

	t.Run("empty name returns ErrInvalidInput", func(t *testing.T) {
		t.Parallel()

		service := NewPositionService(&positionRepositoryMock{})

		_, err := service.CreatePosition(context.Background(), CreatePositionInput{
			Name:        "",
			Description: "Desc",
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("whitespace only name returns ErrInvalidInput", func(t *testing.T) {
		t.Parallel()

		service := NewPositionService(&positionRepositoryMock{})

		_, err := service.CreatePosition(context.Background(), CreatePositionInput{
			Name:        "   ",
			Description: "Desc",
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})
}

func TestPositionServiceGetPositionByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		service := NewPositionService(&positionRepositoryMock{
			getByIDFunc: func(ctx context.Context, id int64) (*model.Position, error) {
				return &model.Position{ID: id, Name: "Front Desk"}, nil
			},
		})

		position, err := service.GetPositionByID(context.Background(), 3)
		if err != nil {
			t.Fatalf("GetPositionByID returned error: %v", err)
		}
		if position.ID != 3 {
			t.Fatalf("expected position ID 3, got %d", position.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		service := NewPositionService(&positionRepositoryMock{
			getByIDFunc: func(ctx context.Context, id int64) (*model.Position, error) {
				return nil, repository.ErrPositionNotFound
			},
		})

		_, err := service.GetPositionByID(context.Background(), 3)
		if !errors.Is(err, ErrPositionNotFound) {
			t.Fatalf("expected ErrPositionNotFound, got %v", err)
		}
	})

	t.Run("invalid ID", func(t *testing.T) {
		t.Parallel()

		service := NewPositionService(&positionRepositoryMock{})

		_, err := service.GetPositionByID(context.Background(), 0)
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})
}

func TestPositionServiceUpdatePosition(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		service := NewPositionService(&positionRepositoryMock{
			updateFunc: func(ctx context.Context, params repository.UpdatePositionParams) (*model.Position, error) {
				if params.ID != 5 || params.Name != "Warehouse" || params.Description != "Loads inventory" {
					t.Fatalf("unexpected update params: %+v", params)
				}
				return &model.Position{
					ID:          params.ID,
					Name:        params.Name,
					Description: params.Description,
				}, nil
			},
		})

		position, err := service.UpdatePosition(context.Background(), UpdatePositionInput{
			ID:          5,
			Name:        " Warehouse ",
			Description: " Loads inventory ",
		})
		if err != nil {
			t.Fatalf("UpdatePosition returned error: %v", err)
		}
		if position.Name != "Warehouse" || position.Description != "Loads inventory" {
			t.Fatalf("unexpected position: %+v", position)
		}
	})

	t.Run("empty name returns ErrInvalidInput", func(t *testing.T) {
		t.Parallel()

		service := NewPositionService(&positionRepositoryMock{})

		_, err := service.UpdatePosition(context.Background(), UpdatePositionInput{
			ID:          5,
			Name:        "   ",
			Description: "Desc",
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		service := NewPositionService(&positionRepositoryMock{
			updateFunc: func(ctx context.Context, params repository.UpdatePositionParams) (*model.Position, error) {
				return nil, repository.ErrPositionNotFound
			},
		})

		_, err := service.UpdatePosition(context.Background(), UpdatePositionInput{
			ID:          5,
			Name:        "Warehouse",
			Description: "Desc",
		})
		if !errors.Is(err, ErrPositionNotFound) {
			t.Fatalf("expected ErrPositionNotFound, got %v", err)
		}
	})
}

func TestPositionServiceDeletePosition(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var deletedID int64
		service := NewPositionService(&positionRepositoryMock{
			deleteFunc: func(ctx context.Context, id int64) error {
				deletedID = id
				return nil
			},
		})

		if err := service.DeletePosition(context.Background(), 4); err != nil {
			t.Fatalf("DeletePosition returned error: %v", err)
		}
		if deletedID != 4 {
			t.Fatalf("expected delete ID 4, got %d", deletedID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		service := NewPositionService(&positionRepositoryMock{
			deleteFunc: func(ctx context.Context, id int64) error {
				return repository.ErrPositionNotFound
			},
		})

		err := service.DeletePosition(context.Background(), 4)
		if !errors.Is(err, ErrPositionNotFound) {
			t.Fatalf("expected ErrPositionNotFound, got %v", err)
		}
	})

	t.Run("invalid ID", func(t *testing.T) {
		t.Parallel()

		service := NewPositionService(&positionRepositoryMock{})

		err := service.DeletePosition(context.Background(), 0)
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})
}
