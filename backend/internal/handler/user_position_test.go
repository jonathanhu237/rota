package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

func TestUserPositionHandlerReplace(t *testing.T) {
	t.Run("accepts wrapped position_ids request body", func(t *testing.T) {
		t.Parallel()

		var receivedInput service.ReplaceUserPositionsInput
		handler := NewUserPositionHandler(&stubUserPositionService{
			replaceUserPositionsFunc: func(ctx context.Context, input service.ReplaceUserPositionsInput) error {
				receivedInput = input
				return nil
			},
		})

		req := httptest.NewRequest(http.MethodPut, "/users/7/positions", bytes.NewBufferString(`{"position_ids":[1,2]}`))
		req.SetPathValue("id", "7")
		recorder := httptest.NewRecorder()

		handler.Replace(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
		}
		if receivedInput.UserID != 7 {
			t.Fatalf("expected user id 7, got %d", receivedInput.UserID)
		}
		if len(receivedInput.PositionIDs) != 2 || receivedInput.PositionIDs[0] != 1 || receivedInput.PositionIDs[1] != 2 {
			t.Fatalf("unexpected position ids: %+v", receivedInput.PositionIDs)
		}
	})

	t.Run("rejects bare array request body", func(t *testing.T) {
		t.Parallel()

		called := false
		handler := NewUserPositionHandler(&stubUserPositionService{
			replaceUserPositionsFunc: func(ctx context.Context, input service.ReplaceUserPositionsInput) error {
				called = true
				return nil
			},
		})

		req := httptest.NewRequest(http.MethodPut, "/users/7/positions", bytes.NewBufferString(`[1,2]`))
		req.SetPathValue("id", "7")
		recorder := httptest.NewRecorder()

		handler.Replace(recorder, req)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
		}
		if called {
			t.Fatal("service should not be called for invalid request body")
		}
	})
}

type stubUserPositionService struct {
	listUserPositionsFunc    func(ctx context.Context, userID int64) ([]*model.Position, error)
	replaceUserPositionsFunc func(ctx context.Context, input service.ReplaceUserPositionsInput) error
}

func (s *stubUserPositionService) ListUserPositions(ctx context.Context, userID int64) ([]*model.Position, error) {
	if s.listUserPositionsFunc != nil {
		return s.listUserPositionsFunc(ctx, userID)
	}
	return nil, nil
}

func (s *stubUserPositionService) ReplaceUserPositions(ctx context.Context, input service.ReplaceUserPositionsInput) error {
	if s.replaceUserPositionsFunc != nil {
		return s.replaceUserPositionsFunc(ctx, input)
	}
	return nil
}
