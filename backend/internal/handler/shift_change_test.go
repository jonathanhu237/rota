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

type stubShiftChangeService struct {
	createFunc       func(ctx context.Context, input service.CreateShiftChangeInput) (*model.ShiftChangeRequest, error)
	getFunc          func(ctx context.Context, requestID, viewerUserID int64, viewerIsAdmin bool) (*model.ShiftChangeRequest, error)
	listFunc         func(ctx context.Context, publicationID, viewerUserID int64, viewerIsAdmin bool) ([]*model.ShiftChangeRequest, error)
	countPendingFunc func(ctx context.Context, viewerUserID int64) (int, error)
	cancelFunc       func(ctx context.Context, requestID, viewerUserID int64) error
	rejectFunc       func(ctx context.Context, requestID, viewerUserID int64) error
	approveFunc      func(ctx context.Context, requestID, viewerUserID int64) error
	listMembersFunc  func(ctx context.Context, publicationID int64) ([]service.PublicationMember, error)
}

func (s *stubShiftChangeService) CreateShiftChangeRequest(ctx context.Context, input service.CreateShiftChangeInput) (*model.ShiftChangeRequest, error) {
	return s.createFunc(ctx, input)
}

func (s *stubShiftChangeService) GetShiftChangeRequest(ctx context.Context, requestID, viewerUserID int64, viewerIsAdmin bool) (*model.ShiftChangeRequest, error) {
	return s.getFunc(ctx, requestID, viewerUserID, viewerIsAdmin)
}

func (s *stubShiftChangeService) ListShiftChangeRequests(ctx context.Context, publicationID, viewerUserID int64, viewerIsAdmin bool) ([]*model.ShiftChangeRequest, error) {
	return s.listFunc(ctx, publicationID, viewerUserID, viewerIsAdmin)
}

func (s *stubShiftChangeService) CountPendingForViewer(ctx context.Context, viewerUserID int64) (int, error) {
	return s.countPendingFunc(ctx, viewerUserID)
}

func (s *stubShiftChangeService) CancelShiftChangeRequest(ctx context.Context, requestID, viewerUserID int64) error {
	return s.cancelFunc(ctx, requestID, viewerUserID)
}

func (s *stubShiftChangeService) RejectShiftChangeRequest(ctx context.Context, requestID, viewerUserID int64) error {
	return s.rejectFunc(ctx, requestID, viewerUserID)
}

func (s *stubShiftChangeService) ApproveShiftChangeRequest(ctx context.Context, requestID, viewerUserID int64) error {
	return s.approveFunc(ctx, requestID, viewerUserID)
}

func (s *stubShiftChangeService) ListPublicationMembers(ctx context.Context, publicationID int64) ([]service.PublicationMember, error) {
	return s.listMembersFunc(ctx, publicationID)
}

// sampleShiftChangeRequest returns a fully populated pending swap request.
func sampleShiftChangeRequest() *model.ShiftChangeRequest {
	counterpartUser := int64(2)
	counterpartAssignment := int64(20)
	now := time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC)
	counterpartOccurrence := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)
	return &model.ShiftChangeRequest{
		ID:                        42,
		PublicationID:             1,
		Type:                      model.ShiftChangeTypeSwap,
		RequesterUserID:           1,
		RequesterAssignmentID:     10,
		OccurrenceDate:            time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
		CounterpartUserID:         &counterpartUser,
		CounterpartAssignmentID:   &counterpartAssignment,
		CounterpartOccurrenceDate: &counterpartOccurrence,
		State:                     model.ShiftChangeStatePending,
		CreatedAt:                 now,
		ExpiresAt:                 now.Add(48 * time.Hour),
	}
}

func sampleCreateShiftChangePayload() map[string]any {
	return map[string]any{
		"type":                        "swap",
		"requester_assignment_id":     10,
		"occurrence_date":             "2026-04-20",
		"counterpart_user_id":         2,
		"counterpart_assignment_id":   20,
		"counterpart_occurrence_date": "2026-04-22",
	}
}

func TestShiftChangeHandler_Create(t *testing.T) {
	t.Run("returns created request", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{
			createFunc: func(ctx context.Context, input service.CreateShiftChangeInput) (*model.ShiftChangeRequest, error) {
				return sampleShiftChangeRequest(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(jsonRequest(t, http.MethodPost, "/publications/1/shift-changes", sampleCreateShiftChangePayload()), map[string]string{"id": "1"}),
			sampleUser(),
		)

		handler.Create(recorder, req)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", recorder.Code)
		}
		response := decodeJSONResponse[shiftChangeRequestDetailResponse](t, recorder)
		if response.Request.ID != 42 || response.Request.Type != "swap" || response.Request.State != "pending" {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("rejects invalid path id", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(jsonRequest(t, http.MethodPost, "/publications/bad/shift-changes", sampleCreateShiftChangePayload()), map[string]string{"id": "bad"}),
			sampleUser(),
		)

		handler.Create(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{})
		recorder := httptest.NewRecorder()
		raw := httptest.NewRequest(http.MethodPost, "/publications/1/shift-changes", strings.NewReader("{"))
		raw.Header.Set("Content-Type", "application/json")
		req := requestWithUser(requestWithPathValues(raw, map[string]string{"id": "1"}), sampleUser())

		handler.Create(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("maps service errors", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name   string
			err    error
			status int
			code   string
		}{
			{name: "invalid type", err: service.ErrShiftChangeInvalidType, status: http.StatusBadRequest, code: "SHIFT_CHANGE_INVALID_TYPE"},
			{name: "self", err: service.ErrShiftChangeSelf, status: http.StatusBadRequest, code: "SHIFT_CHANGE_SELF"},
			{name: "not owner", err: service.ErrShiftChangeNotOwner, status: http.StatusForbidden, code: "SHIFT_CHANGE_NOT_OWNER"},
			{name: "not qualified", err: service.ErrShiftChangeNotQualified, status: http.StatusForbidden, code: "SHIFT_CHANGE_NOT_QUALIFIED"},
			{name: "user disabled", err: service.ErrUserDisabled, status: http.StatusConflict, code: "USER_DISABLED"},
			{name: "retryable scheduling", err: service.ErrSchedulingRetryable, status: http.StatusServiceUnavailable, code: "SCHEDULING_RETRYABLE"},
			{name: "publication not published", err: service.ErrPublicationNotPublished, status: http.StatusConflict, code: "PUBLICATION_NOT_PUBLISHED"},
			{name: "publication not found", err: service.ErrPublicationNotFound, status: http.StatusNotFound, code: "PUBLICATION_NOT_FOUND"},
			{name: "invalid input", err: service.ErrInvalidInput, status: http.StatusBadRequest, code: "INVALID_REQUEST"},
			{name: "unknown error falls through to internal", err: context.DeadlineExceeded, status: http.StatusInternalServerError, code: "INTERNAL_ERROR"},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				handler := NewShiftChangeHandler(&stubShiftChangeService{
					createFunc: func(ctx context.Context, input service.CreateShiftChangeInput) (*model.ShiftChangeRequest, error) {
						return nil, tc.err
					},
				})
				recorder := httptest.NewRecorder()
				req := requestWithUser(
					requestWithPathValues(jsonRequest(t, http.MethodPost, "/publications/1/shift-changes", sampleCreateShiftChangePayload()), map[string]string{"id": "1"}),
					sampleUser(),
				)

				handler.Create(recorder, req)

				assertErrorResponse(t, recorder, tc.status, tc.code)
			})
		}
	})
}

func TestShiftChangeHandler_GetByID(t *testing.T) {
	t.Run("returns request", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{
			getFunc: func(ctx context.Context, requestID, viewerUserID int64, viewerIsAdmin bool) (*model.ShiftChangeRequest, error) {
				return sampleShiftChangeRequest(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/1/shift-changes/42", nil), map[string]string{"id": "1", "request_id": "42"}),
			sampleUser(),
		)

		handler.GetByID(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[shiftChangeRequestDetailResponse](t, recorder)
		if response.Request.ID != 42 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("rejects invalid path id", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/1/shift-changes/bad", nil), map[string]string{"id": "1", "request_id": "bad"}),
			sampleUser(),
		)

		handler.GetByID(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("maps not found", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{
			getFunc: func(ctx context.Context, requestID, viewerUserID int64, viewerIsAdmin bool) (*model.ShiftChangeRequest, error) {
				return nil, service.ErrShiftChangeNotFound
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/1/shift-changes/42", nil), map[string]string{"id": "1", "request_id": "42"}),
			sampleUser(),
		)

		handler.GetByID(recorder, req)

		assertErrorResponse(t, recorder, http.StatusNotFound, "SHIFT_CHANGE_NOT_FOUND")
	})
}

func TestShiftChangeHandler_List(t *testing.T) {
	t.Run("returns requests", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{
			listFunc: func(ctx context.Context, publicationID, viewerUserID int64, viewerIsAdmin bool) ([]*model.ShiftChangeRequest, error) {
				return []*model.ShiftChangeRequest{sampleShiftChangeRequest()}, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/1/shift-changes", nil), map[string]string{"id": "1"}),
			sampleUser(),
		)

		handler.List(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[shiftChangeRequestListResponse](t, recorder)
		if len(response.Requests) != 1 || response.Requests[0].ID != 42 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("rejects invalid path id", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/bad/shift-changes", nil), map[string]string{"id": "bad"}),
			sampleUser(),
		)

		handler.List(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})
}

func TestShiftChangeHandler_Approve(t *testing.T) {
	t.Run("returns no content", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{
			approveFunc: func(ctx context.Context, requestID, viewerUserID int64) error { return nil },
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(httptest.NewRequest(http.MethodPost, "/publications/1/shift-changes/42/approve", nil), map[string]string{"id": "1", "request_id": "42"}),
			sampleUser(),
		)

		handler.Approve(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", recorder.Code)
		}
	})

	t.Run("rejects invalid path id", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(httptest.NewRequest(http.MethodPost, "/publications/1/shift-changes/bad/approve", nil), map[string]string{"id": "1", "request_id": "bad"}),
			sampleUser(),
		)

		handler.Approve(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("maps service errors", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name   string
			err    error
			status int
			code   string
		}{
			{name: "not pending", err: service.ErrShiftChangeNotPending, status: http.StatusConflict, code: "SHIFT_CHANGE_NOT_PENDING"},
			{name: "expired", err: service.ErrShiftChangeExpired, status: http.StatusConflict, code: "SHIFT_CHANGE_EXPIRED"},
			{name: "invalidated", err: service.ErrShiftChangeInvalidated, status: http.StatusConflict, code: "SHIFT_CHANGE_INVALIDATED"},
			{name: "not qualified", err: service.ErrShiftChangeNotQualified, status: http.StatusForbidden, code: "SHIFT_CHANGE_NOT_QUALIFIED"},
			{name: "user disabled", err: service.ErrUserDisabled, status: http.StatusConflict, code: "USER_DISABLED"},
			{name: "retryable scheduling", err: service.ErrSchedulingRetryable, status: http.StatusServiceUnavailable, code: "SCHEDULING_RETRYABLE"},
			{name: "not owner", err: service.ErrShiftChangeNotOwner, status: http.StatusForbidden, code: "SHIFT_CHANGE_NOT_OWNER"},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				handler := NewShiftChangeHandler(&stubShiftChangeService{
					approveFunc: func(ctx context.Context, requestID, viewerUserID int64) error { return tc.err },
				})
				recorder := httptest.NewRecorder()
				req := requestWithUser(
					requestWithPathValues(httptest.NewRequest(http.MethodPost, "/publications/1/shift-changes/42/approve", nil), map[string]string{"id": "1", "request_id": "42"}),
					sampleUser(),
				)

				handler.Approve(recorder, req)

				assertErrorResponse(t, recorder, tc.status, tc.code)
			})
		}
	})
}

func TestShiftChangeHandler_Reject(t *testing.T) {
	t.Run("returns no content", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{
			rejectFunc: func(ctx context.Context, requestID, viewerUserID int64) error { return nil },
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(httptest.NewRequest(http.MethodPost, "/publications/1/shift-changes/42/reject", nil), map[string]string{"id": "1", "request_id": "42"}),
			sampleUser(),
		)

		handler.Reject(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", recorder.Code)
		}
	})

	t.Run("rejects invalid path id", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(httptest.NewRequest(http.MethodPost, "/publications/1/shift-changes/bad/reject", nil), map[string]string{"id": "1", "request_id": "bad"}),
			sampleUser(),
		)

		handler.Reject(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("maps service errors", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name   string
			err    error
			status int
			code   string
		}{
			{name: "invalid type", err: service.ErrShiftChangeInvalidType, status: http.StatusBadRequest, code: "SHIFT_CHANGE_INVALID_TYPE"},
			{name: "not owner", err: service.ErrShiftChangeNotOwner, status: http.StatusForbidden, code: "SHIFT_CHANGE_NOT_OWNER"},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				handler := NewShiftChangeHandler(&stubShiftChangeService{
					rejectFunc: func(ctx context.Context, requestID, viewerUserID int64) error { return tc.err },
				})
				recorder := httptest.NewRecorder()
				req := requestWithUser(
					requestWithPathValues(httptest.NewRequest(http.MethodPost, "/publications/1/shift-changes/42/reject", nil), map[string]string{"id": "1", "request_id": "42"}),
					sampleUser(),
				)

				handler.Reject(recorder, req)

				assertErrorResponse(t, recorder, tc.status, tc.code)
			})
		}
	})
}

func TestShiftChangeHandler_Cancel(t *testing.T) {
	t.Run("returns no content", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{
			cancelFunc: func(ctx context.Context, requestID, viewerUserID int64) error { return nil },
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(httptest.NewRequest(http.MethodPost, "/publications/1/shift-changes/42/cancel", nil), map[string]string{"id": "1", "request_id": "42"}),
			sampleUser(),
		)

		handler.Cancel(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", recorder.Code)
		}
	})

	t.Run("rejects invalid path id", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(httptest.NewRequest(http.MethodPost, "/publications/1/shift-changes/bad/cancel", nil), map[string]string{"id": "1", "request_id": "bad"}),
			sampleUser(),
		)

		handler.Cancel(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("maps not owner", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{
			cancelFunc: func(ctx context.Context, requestID, viewerUserID int64) error { return service.ErrShiftChangeNotOwner },
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(httptest.NewRequest(http.MethodPost, "/publications/1/shift-changes/42/cancel", nil), map[string]string{"id": "1", "request_id": "42"}),
			sampleUser(),
		)

		handler.Cancel(recorder, req)

		assertErrorResponse(t, recorder, http.StatusForbidden, "SHIFT_CHANGE_NOT_OWNER")
	})
}

func TestShiftChangeHandler_ListMembers(t *testing.T) {
	t.Run("returns members", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{
			listMembersFunc: func(ctx context.Context, publicationID int64) ([]service.PublicationMember, error) {
				return []service.PublicationMember{
					{UserID: 1, Name: "Worker"},
					{UserID: 2, Name: "Helper"},
				}, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/1/members", nil), map[string]string{"id": "1"})

		handler.ListMembers(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[publicationMembersResponse](t, recorder)
		if len(response.Members) != 2 || response.Members[0].UserID != 1 || response.Members[1].Name != "Helper" {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("returns empty array when none", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{
			listMembersFunc: func(ctx context.Context, publicationID int64) ([]service.PublicationMember, error) {
				return nil, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/1/members", nil), map[string]string{"id": "1"})

		handler.ListMembers(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[publicationMembersResponse](t, recorder)
		if response.Members == nil || len(response.Members) != 0 {
			t.Fatalf("expected empty members array, got %+v", response.Members)
		}
	})

	t.Run("rejects invalid path id", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/bad/members", nil), map[string]string{"id": "bad"})

		handler.ListMembers(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})
}

func TestShiftChangeHandler_UnreadCount(t *testing.T) {
	t.Run("returns count", func(t *testing.T) {
		t.Parallel()

		handler := NewShiftChangeHandler(&stubShiftChangeService{
			countPendingFunc: func(ctx context.Context, viewerUserID int64) (int, error) { return 3, nil },
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(httptest.NewRequest(http.MethodGet, "/users/me/notifications/unread-count", nil), sampleUser())

		handler.UnreadCount(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[unreadCountResponse](t, recorder)
		if response.Count != 3 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})
}
