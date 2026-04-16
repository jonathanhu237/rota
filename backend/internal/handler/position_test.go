package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

type stubPositionService struct {
	listPositionsFunc   func(ctx context.Context, input service.ListPositionsInput) (*service.ListPositionsResult, error)
	getPositionByIDFunc func(ctx context.Context, id int64) (*model.Position, error)
	createPositionFunc  func(ctx context.Context, input service.CreatePositionInput) (*model.Position, error)
	updatePositionFunc  func(ctx context.Context, input service.UpdatePositionInput) (*model.Position, error)
	deletePositionFunc  func(ctx context.Context, id int64) error
}

func (s *stubPositionService) ListPositions(ctx context.Context, input service.ListPositionsInput) (*service.ListPositionsResult, error) {
	return s.listPositionsFunc(ctx, input)
}

func (s *stubPositionService) GetPositionByID(ctx context.Context, id int64) (*model.Position, error) {
	return s.getPositionByIDFunc(ctx, id)
}

func (s *stubPositionService) CreatePosition(ctx context.Context, input service.CreatePositionInput) (*model.Position, error) {
	return s.createPositionFunc(ctx, input)
}

func (s *stubPositionService) UpdatePosition(ctx context.Context, input service.UpdatePositionInput) (*model.Position, error) {
	return s.updatePositionFunc(ctx, input)
}

func (s *stubPositionService) DeletePosition(ctx context.Context, id int64) error {
	return s.deletePositionFunc(ctx, id)
}

func TestPositionHandler(t *testing.T) {
	t.Run("List returns positions", func(t *testing.T) {
		t.Parallel()

		handler := NewPositionHandler(&stubPositionService{
			listPositionsFunc: func(ctx context.Context, input service.ListPositionsInput) (*service.ListPositionsResult, error) {
				return &service.ListPositionsResult{
					Positions:  []*model.Position{samplePosition()},
					Page:       1,
					PageSize:   10,
					Total:      1,
					TotalPages: 1,
				}, nil
			},
		})
		recorder := httptest.NewRecorder()

		handler.List(recorder, httptest.NewRequest(http.MethodGet, "/positions", nil))

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[positionsResponse](t, recorder)
		if len(response.Positions) != 1 || response.Positions[0].ID != 1 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("Create returns created position", func(t *testing.T) {
		t.Parallel()

		handler := NewPositionHandler(&stubPositionService{
			createPositionFunc: func(ctx context.Context, input service.CreatePositionInput) (*model.Position, error) {
				return samplePosition(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPost, "/positions", map[string]any{
			"name":        "Front Desk",
			"description": "Receives visitors",
		})

		handler.Create(recorder, req)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", recorder.Code)
		}
	})

	t.Run("Create rejects invalid JSON", func(t *testing.T) {
		t.Parallel()

		handler := NewPositionHandler(&stubPositionService{})
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/positions", strings.NewReader("{"))

		handler.Create(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("GetByID returns position", func(t *testing.T) {
		t.Parallel()

		handler := NewPositionHandler(&stubPositionService{
			getPositionByIDFunc: func(ctx context.Context, id int64) (*model.Position, error) {
				return samplePosition(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/positions/1", nil), map[string]string{"id": "1"})

		handler.GetByID(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
	})

	t.Run("GetByID maps position not found", func(t *testing.T) {
		t.Parallel()

		handler := NewPositionHandler(&stubPositionService{
			getPositionByIDFunc: func(ctx context.Context, id int64) (*model.Position, error) {
				return nil, service.ErrPositionNotFound
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/positions/2", nil), map[string]string{"id": "2"})

		handler.GetByID(recorder, req)

		assertErrorResponse(t, recorder, http.StatusNotFound, "POSITION_NOT_FOUND")
	})

	t.Run("Update returns updated position", func(t *testing.T) {
		t.Parallel()

		handler := NewPositionHandler(&stubPositionService{
			updatePositionFunc: func(ctx context.Context, input service.UpdatePositionInput) (*model.Position, error) {
				return samplePosition(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(jsonRequest(t, http.MethodPut, "/positions/1", map[string]any{
			"name":        "Front Desk",
			"description": "Updated",
		}), map[string]string{"id": "1"})

		handler.Update(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
	})

	t.Run("Update maps position not found", func(t *testing.T) {
		t.Parallel()

		handler := NewPositionHandler(&stubPositionService{
			updatePositionFunc: func(ctx context.Context, input service.UpdatePositionInput) (*model.Position, error) {
				return nil, service.ErrPositionNotFound
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(jsonRequest(t, http.MethodPut, "/positions/1", map[string]any{
			"name":        "Front Desk",
			"description": "Updated",
		}), map[string]string{"id": "1"})

		handler.Update(recorder, req)

		assertErrorResponse(t, recorder, http.StatusNotFound, "POSITION_NOT_FOUND")
	})

	t.Run("Delete returns no content", func(t *testing.T) {
		t.Parallel()

		handler := NewPositionHandler(&stubPositionService{
			deletePositionFunc: func(ctx context.Context, id int64) error { return nil },
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodDelete, "/positions/1", nil), map[string]string{"id": "1"})

		handler.Delete(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", recorder.Code)
		}
	})

	t.Run("Delete maps position in use", func(t *testing.T) {
		t.Parallel()

		handler := NewPositionHandler(&stubPositionService{
			deletePositionFunc: func(ctx context.Context, id int64) error { return service.ErrPositionInUse },
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodDelete, "/positions/1", nil), map[string]string{"id": "1"})

		handler.Delete(recorder, req)

		assertErrorResponse(t, recorder, http.StatusConflict, "POSITION_IN_USE")
	})

	t.Run("Delete maps position not found", func(t *testing.T) {
		t.Parallel()

		handler := NewPositionHandler(&stubPositionService{
			deletePositionFunc: func(ctx context.Context, id int64) error { return service.ErrPositionNotFound },
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodDelete, "/positions/1", nil), map[string]string{"id": "1"})

		handler.Delete(recorder, req)

		assertErrorResponse(t, recorder, http.StatusNotFound, "POSITION_NOT_FOUND")
	})
}

func samplePosition() *model.Position {
	now := time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC)
	return &model.Position{
		ID:          1,
		Name:        "Front Desk",
		Description: "Receives visitors",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
