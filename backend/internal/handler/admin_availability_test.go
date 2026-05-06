package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

func TestPublicationHandlerAdminAvailability(t *testing.T) {
	t.Run("ListAdminAvailability returns board response and passes pagination", func(t *testing.T) {
		t.Parallel()

		var gotInput service.ListAdminAvailabilityInput
		handler := NewPublicationHandler(&stubPublicationService{
			listAdminAvailabilityFunc: func(ctx context.Context, input service.ListAdminAvailabilityInput) (*service.AdminAvailabilityBoardResult, error) {
				gotInput = input
				return &service.AdminAvailabilityBoardResult{
					Publication: samplePublication(),
					Employees: []*service.AdminAvailabilityEmployee{
						{
							UserID:         7,
							Name:           "Alice",
							Email:          "alice@example.com",
							Positions:      []*model.Position{{ID: 101, Name: "Front Desk"}},
							SubmittedCount: 0,
						},
					},
					Page:       2,
					PageSize:   10,
					Total:      11,
					TotalPages: 2,
				}, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/1/availability-board?page=2&page_size=10&search=ali", nil), map[string]string{"id": "1"})

		handler.ListAdminAvailability(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		if gotInput.PublicationID != 1 || gotInput.Page != 2 || gotInput.PageSize != 10 || gotInput.Search != "ali" {
			t.Fatalf("unexpected input: %+v", gotInput)
		}
		response := decodeJSONResponse[adminAvailabilityBoardResponse](t, recorder)
		if len(response.Employees) != 1 || response.Employees[0].SubmittedCount != 0 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("GetAdminAvailabilityDetail returns employee detail", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			getAdminAvailabilityDetailFunc: func(ctx context.Context, input service.GetAdminAvailabilityDetailInput) (*service.AdminAvailabilityDetailResult, error) {
				if input.PublicationID != 1 || input.UserID != 7 {
					t.Fatalf("unexpected input: %+v", input)
				}
				return sampleAdminAvailabilityDetail(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/1/availability-submissions/7", nil), map[string]string{"id": "1", "user_id": "7"})

		handler.GetAdminAvailabilityDetail(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[adminAvailabilityDetailResponse](t, recorder)
		if response.User.ID != 7 || len(response.Cells) != 1 || !response.Cells[0].Eligible {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("ReplaceAdminAvailability accepts empty replacement", func(t *testing.T) {
		t.Parallel()

		var gotInput service.ReplaceAdminAvailabilityInput
		handler := NewPublicationHandler(&stubPublicationService{
			replaceAdminAvailabilityFunc: func(ctx context.Context, input service.ReplaceAdminAvailabilityInput) (*service.AdminAvailabilityDetailResult, error) {
				gotInput = input
				return sampleAdminAvailabilityDetail(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(jsonRequest(t, http.MethodPut, "/publications/1/availability-submissions/7", map[string]any{
			"submissions": []map[string]any{},
		}), map[string]string{"id": "1", "user_id": "7"})

		handler.ReplaceAdminAvailability(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		if gotInput.PublicationID != 1 || gotInput.UserID != 7 || len(gotInput.Submissions) != 0 {
			t.Fatalf("unexpected input: %+v", gotInput)
		}
	})

	t.Run("ReplaceAdminAvailability rejects missing submissions", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodPut, "/publications/1/availability-submissions/7", strings.NewReader(`{}`)), map[string]string{"id": "1", "user_id": "7"})
		req.Header.Set("Content-Type", "application/json")

		handler.ReplaceAdminAvailability(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("ReplaceAdminAvailability maps qualification errors", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			replaceAdminAvailabilityFunc: func(ctx context.Context, input service.ReplaceAdminAvailabilityInput) (*service.AdminAvailabilityDetailResult, error) {
				return nil, service.ErrNotQualified
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(jsonRequest(t, http.MethodPut, "/publications/1/availability-submissions/7", map[string]any{
			"submissions": []map[string]any{{"slot_id": 22, "weekday": 3}},
		}), map[string]string{"id": "1", "user_id": "7"})

		handler.ReplaceAdminAvailability(recorder, req)

		assertErrorResponse(t, recorder, http.StatusForbidden, "NOT_QUALIFIED")
	})
}

func sampleAdminAvailabilityDetail() *service.AdminAvailabilityDetailResult {
	return &service.AdminAvailabilityDetailResult{
		Publication: samplePublication(),
		User: &model.User{
			ID:     7,
			Email:  "alice@example.com",
			Name:   "Alice",
			Status: model.UserStatusActive,
		},
		Positions: []*model.Position{{ID: 101, Name: "Front Desk"}},
		Slots: []*service.AdminAvailabilitySlot{
			{
				Slot: &model.TemplateSlot{
					ID:        21,
					Weekdays:  []int{1},
					StartTime: "09:00",
					EndTime:   "10:00",
				},
				Positions: []service.AdminAvailabilitySlotPosition{
					{
						Position:          &model.Position{ID: 101, Name: "Front Desk"},
						RequiredHeadcount: 1,
					},
				},
			},
		},
		Submissions: []model.SlotRef{{SlotID: 21, Weekday: 1}},
		Cells: []service.AdminAvailabilityCell{
			{SlotID: 21, Weekday: 1, Eligible: true, Submitted: true},
		},
	}
}
