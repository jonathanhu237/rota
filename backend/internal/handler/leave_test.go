package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

type stubLeaveService struct {
	createFunc             func(ctx context.Context, input service.CreateLeaveInput) (*service.LeaveDetail, error)
	cancelFunc             func(ctx context.Context, leaveID, userID int64) error
	getByIDFunc            func(ctx context.Context, leaveID int64) (*service.LeaveDetail, error)
	listPoolFunc           func(ctx context.Context, viewerUserID int64, viewerIsAdmin bool, input service.ListLeavePoolInput) (*service.LeavePoolResult, error)
	listForUserFunc        func(ctx context.Context, userID int64, input service.ListLeavesInput) ([]*service.LeaveDetail, error)
	listForPublicationFunc func(ctx context.Context, publicationID int64, input service.ListLeavesInput) ([]*service.LeaveDetail, error)
	previewFunc            func(ctx context.Context, userID int64, from time.Time, to time.Time) ([]*service.OccurrencePreview, error)
}

func (s *stubLeaveService) Create(ctx context.Context, input service.CreateLeaveInput) (*service.LeaveDetail, error) {
	return s.createFunc(ctx, input)
}

func (s *stubLeaveService) Cancel(ctx context.Context, leaveID, userID int64) error {
	return s.cancelFunc(ctx, leaveID, userID)
}

func (s *stubLeaveService) GetByID(ctx context.Context, leaveID int64, viewerUserID int64, viewerIsAdmin bool) (*service.LeaveDetail, error) {
	return s.getByIDFunc(ctx, leaveID)
}

func (s *stubLeaveService) ListPool(ctx context.Context, viewerUserID int64, viewerIsAdmin bool, input service.ListLeavePoolInput) (*service.LeavePoolResult, error) {
	return s.listPoolFunc(ctx, viewerUserID, viewerIsAdmin, input)
}

func (s *stubLeaveService) ListForUser(ctx context.Context, userID int64, input service.ListLeavesInput) ([]*service.LeaveDetail, error) {
	return s.listForUserFunc(ctx, userID, input)
}

func (s *stubLeaveService) ListForPublication(ctx context.Context, publicationID int64, input service.ListLeavesInput) ([]*service.LeaveDetail, error) {
	return s.listForPublicationFunc(ctx, publicationID, input)
}

func (s *stubLeaveService) PreviewOccurrences(ctx context.Context, userID int64, from time.Time, to time.Time) ([]*service.OccurrencePreview, error) {
	return s.previewFunc(ctx, userID, from, to)
}

func TestLeaveHandler(t *testing.T) {
	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)

	t.Run("Create returns share URL", func(t *testing.T) {
		handler := NewLeaveHandler(&stubLeaveService{
			createFunc: func(ctx context.Context, input service.CreateLeaveInput) (*service.LeaveDetail, error) {
				if input.UserID != 1 || input.AssignmentID != 100 || input.Category != model.LeaveCategoryPersonal {
					t.Fatalf("unexpected input: %+v", input)
				}
				return sampleLeaveDetail(now), nil
			},
		})
		req := requestWithUser(jsonRequest(t, http.MethodPost, "/leaves", map[string]any{
			"assignment_id":   100,
			"occurrence_date": "2026-04-27",
			"type":            "give_pool",
			"category":        "personal",
			"reason":          "exam",
		}), sampleUser())
		rec := httptest.NewRecorder()

		handler.Create(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", rec.Code)
		}
		resp := decodeJSONResponse[leaveDetailResponse](t, rec)
		if resp.Leave.ShareURL != "/leaves/42" {
			t.Fatalf("expected share URL, got %+v", resp.Leave)
		}
	})

	t.Run("Create maps invalid type", func(t *testing.T) {
		handler := NewLeaveHandler(&stubLeaveService{
			createFunc: func(ctx context.Context, input service.CreateLeaveInput) (*service.LeaveDetail, error) {
				return nil, service.ErrShiftChangeInvalidType
			},
		})
		req := requestWithUser(jsonRequest(t, http.MethodPost, "/leaves", map[string]any{
			"assignment_id":   100,
			"occurrence_date": "2026-04-27",
			"type":            "swap",
			"category":        "personal",
		}), sampleUser())
		rec := httptest.NewRecorder()

		handler.Create(rec, req)

		assertErrorResponse(t, rec, http.StatusBadRequest, "SHIFT_CHANGE_INVALID_TYPE")
	})

	t.Run("GetByID maps missing leave", func(t *testing.T) {
		handler := NewLeaveHandler(&stubLeaveService{
			getByIDFunc: func(ctx context.Context, leaveID int64) (*service.LeaveDetail, error) {
				return nil, service.ErrLeaveNotFound
			},
		})
		rec := httptest.NewRecorder()
		req := requestWithUser(requestWithPathValues(httptest.NewRequest(http.MethodGet, "/leaves/99", nil), map[string]string{"id": "99"}), sampleUser())
		handler.GetByID(rec, req)

		assertErrorResponse(t, rec, http.StatusNotFound, "LEAVE_NOT_FOUND")
	})

	t.Run("Cancel maps non-owner", func(t *testing.T) {
		handler := NewLeaveHandler(&stubLeaveService{
			cancelFunc: func(ctx context.Context, leaveID, userID int64) error {
				return service.ErrLeaveNotOwner
			},
		})
		req := requestWithUser(requestWithPathValues(httptest.NewRequest(http.MethodPost, "/leaves/42/cancel", nil), map[string]string{"id": "42"}), sampleUser())
		rec := httptest.NewRecorder()

		handler.Cancel(rec, req)

		assertErrorResponse(t, rec, http.StatusForbidden, "LEAVE_NOT_OWNER")
	})

	t.Run("Preview maps inverted range", func(t *testing.T) {
		handler := NewLeaveHandler(&stubLeaveService{
			previewFunc: func(ctx context.Context, userID int64, from time.Time, to time.Time) ([]*service.OccurrencePreview, error) {
				return nil, service.ErrInvalidInput
			},
		})
		req := requestWithUser(httptest.NewRequest(http.MethodGet, "/users/me/leaves/preview?from=2026-05-10&to=2026-05-01", nil), sampleUser())
		rec := httptest.NewRecorder()

		handler.PreviewMine(rec, req)

		assertErrorResponse(t, rec, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("ListPool returns metadata and row actions", func(t *testing.T) {
		handler := NewLeaveHandler(&stubLeaveService{
			listPoolFunc: func(ctx context.Context, viewerUserID int64, viewerIsAdmin bool, input service.ListLeavePoolInput) (*service.LeavePoolResult, error) {
				if viewerUserID != 1 || input.State != "all" || input.Page != 2 || input.PageSize != 20 {
					t.Fatalf("unexpected pool input: viewer=%d input=%+v", viewerUserID, input)
				}
				detail := sampleLeaveDetail(now)
				detail.Actions = service.LeaveActions{CanClaim: true}
				return &service.LeavePoolResult{
					Leaves:     []*service.LeaveDetail{detail},
					Page:       2,
					PageSize:   20,
					TotalCount: 41,
				}, nil
			},
		})
		req := requestWithUser(httptest.NewRequest(http.MethodGet, "/leaves/pool?state=all&page=2&page_size=20", nil), sampleUser())
		rec := httptest.NewRecorder()

		handler.ListPool(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		resp := decodeJSONResponse[leavePoolResponse](t, rec)
		if resp.Page != 2 || resp.PageSize != 20 || resp.TotalCount != 41 {
			t.Fatalf("unexpected pool metadata: %+v", resp)
		}
		if len(resp.Leaves) != 1 || !resp.Leaves[0].Actions.CanClaim {
			t.Fatalf("unexpected pool leaves: %+v", resp.Leaves)
		}
	})

	t.Run("ListPool maps invalid state", func(t *testing.T) {
		handler := NewLeaveHandler(&stubLeaveService{
			listPoolFunc: func(ctx context.Context, viewerUserID int64, viewerIsAdmin bool, input service.ListLeavePoolInput) (*service.LeavePoolResult, error) {
				return nil, service.ErrInvalidInput
			},
		})
		req := requestWithUser(httptest.NewRequest(http.MethodGet, "/leaves/pool?state=unknown", nil), sampleUser())
		rec := httptest.NewRecorder()

		handler.ListPool(rec, req)

		assertErrorResponse(t, rec, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("ListForPublication succeeds", func(t *testing.T) {
		handler := NewLeaveHandler(&stubLeaveService{
			listForPublicationFunc: func(ctx context.Context, publicationID int64, input service.ListLeavesInput) ([]*service.LeaveDetail, error) {
				if publicationID != 1 {
					t.Fatalf("unexpected publication id %d", publicationID)
				}
				return []*service.LeaveDetail{sampleLeaveDetail(now)}, nil
			},
		})
		rec := httptest.NewRecorder()
		handler.ListForPublication(rec, requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/1/leaves", nil), map[string]string{"id": "1"}))

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		resp := decodeJSONResponse[leaveListResponse](t, rec)
		if len(resp.Leaves) != 1 || resp.Leaves[0].ID != 42 {
			t.Fatalf("unexpected leave list: %+v", resp)
		}
	})

	t.Run("Cancel bad path id is invalid request", func(t *testing.T) {
		handler := NewLeaveHandler(&stubLeaveService{
			cancelFunc: func(ctx context.Context, leaveID, userID int64) error {
				return errors.New("should not be called")
			},
		})
		req := requestWithUser(requestWithPathValues(httptest.NewRequest(http.MethodPost, "/leaves/bad/cancel", nil), map[string]string{"id": "bad"}), sampleUser())
		rec := httptest.NewRecorder()

		handler.Cancel(rec, req)

		assertErrorResponse(t, rec, http.StatusBadRequest, "INVALID_REQUEST")
	})
}

func TestLeavePoolHandler(t *testing.T) {
	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)

	t.Run("success serializes metadata and actions", func(t *testing.T) {
		handler := NewLeaveHandler(&stubLeaveService{
			listPoolFunc: func(ctx context.Context, viewerUserID int64, viewerIsAdmin bool, input service.ListLeavePoolInput) (*service.LeavePoolResult, error) {
				if viewerUserID != 1 || input.State != "all" || input.Page != 2 || input.PageSize != 20 {
					t.Fatalf("unexpected pool input: viewer=%d input=%+v", viewerUserID, input)
				}
				detail := sampleLeaveDetail(now)
				detail.Actions = service.LeaveActions{CanClaim: true}
				return &service.LeavePoolResult{
					Leaves:     []*service.LeaveDetail{detail},
					Page:       2,
					PageSize:   20,
					TotalCount: 41,
				}, nil
			},
		})
		req := requestWithUser(httptest.NewRequest(http.MethodGet, "/leaves/pool?state=all&page=2&page_size=20", nil), sampleUser())
		rec := httptest.NewRecorder()

		handler.ListPool(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		resp := decodeJSONResponse[leavePoolResponse](t, rec)
		if resp.Page != 2 || resp.PageSize != 20 || resp.TotalCount != 41 {
			t.Fatalf("unexpected pool metadata: %+v", resp)
		}
		if len(resp.Leaves) != 1 || !resp.Leaves[0].Actions.CanClaim || resp.Leaves[0].RequesterName != "Alice" {
			t.Fatalf("unexpected pool leaves: %+v", resp.Leaves)
		}
	})

	t.Run("invalid pagination is rejected", func(t *testing.T) {
		handler := NewLeaveHandler(&stubLeaveService{})
		req := requestWithUser(httptest.NewRequest(http.MethodGet, "/leaves/pool?page=abc", nil), sampleUser())
		rec := httptest.NewRecorder()

		handler.ListPool(rec, req)

		assertErrorResponse(t, rec, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("invalid state maps to invalid request", func(t *testing.T) {
		handler := NewLeaveHandler(&stubLeaveService{
			listPoolFunc: func(ctx context.Context, viewerUserID int64, viewerIsAdmin bool, input service.ListLeavePoolInput) (*service.LeavePoolResult, error) {
				return nil, service.ErrInvalidInput
			},
		})
		req := requestWithUser(httptest.NewRequest(http.MethodGet, "/leaves/pool?state=unknown", nil), sampleUser())
		rec := httptest.NewRecorder()

		handler.ListPool(rec, req)

		assertErrorResponse(t, rec, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("admin view-only action is serialized", func(t *testing.T) {
		admin := *sampleUser()
		admin.IsAdmin = true
		handler := NewLeaveHandler(&stubLeaveService{
			listPoolFunc: func(ctx context.Context, viewerUserID int64, viewerIsAdmin bool, input service.ListLeavePoolInput) (*service.LeavePoolResult, error) {
				if !viewerIsAdmin {
					t.Fatalf("expected admin viewer")
				}
				detail := sampleLeaveDetail(now)
				detail.Actions = service.LeaveActions{DisabledReason: model.LeaveActionDisabledAdminViewOnly}
				return &service.LeavePoolResult{
					Leaves:     []*service.LeaveDetail{detail},
					Page:       1,
					PageSize:   20,
					TotalCount: 1,
				}, nil
			},
		})
		req := requestWithUser(httptest.NewRequest(http.MethodGet, "/leaves/pool", nil), &admin)
		rec := httptest.NewRecorder()

		handler.ListPool(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		resp := decodeJSONResponse[leavePoolResponse](t, rec)
		if len(resp.Leaves) != 1 ||
			resp.Leaves[0].Actions.CanClaim ||
			resp.Leaves[0].Actions.DisabledReason != string(model.LeaveActionDisabledAdminViewOnly) {
			t.Fatalf("expected admin view-only actions, got %+v", resp.Leaves)
		}
	})
}

func TestLeavePreviewHandler(t *testing.T) {
	t.Run("serializes direct candidates", func(t *testing.T) {
		handler := NewLeaveHandler(&stubLeaveService{
			previewFunc: func(ctx context.Context, userID int64, from time.Time, to time.Time) ([]*service.OccurrencePreview, error) {
				if userID != 1 {
					t.Fatalf("unexpected user id %d", userID)
				}
				return []*service.OccurrencePreview{
					{
						AssignmentID:   100,
						OccurrenceDate: time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC),
						Slot: &model.TemplateSlot{
							ID:        21,
							Weekdays:  []int{1},
							StartTime: "09:00",
							EndTime:   "12:00",
						},
						Position: &model.Position{
							ID:   101,
							Name: "Front Desk",
						},
						OccurrenceStart: time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC),
						OccurrenceEnd:   time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC),
						DirectCandidates: []service.LeaveDirectCandidate{
							{UserID: 8, Name: "Bob"},
						},
					},
				}, nil
			},
		})
		req := requestWithUser(httptest.NewRequest(http.MethodGet, "/users/me/leaves/preview?from=2026-04-27&to=2026-04-27", nil), sampleUser())
		rec := httptest.NewRecorder()

		handler.PreviewMine(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		resp := decodeJSONResponse[leavePreviewResponse](t, rec)
		if len(resp.Occurrences) != 1 ||
			len(resp.Occurrences[0].DirectCandidates) != 1 ||
			resp.Occurrences[0].DirectCandidates[0].UserID != 8 ||
			resp.Occurrences[0].DirectCandidates[0].Name != "Bob" {
			t.Fatalf("unexpected preview response: %+v", resp)
		}
	})
}

func sampleLeaveDetail(now time.Time) *service.LeaveDetail {
	leaveID := int64(42)
	return &service.LeaveDetail{
		Leave: &model.Leave{
			ID:                   leaveID,
			UserID:               7,
			PublicationID:        1,
			ShiftChangeRequestID: 77,
			Category:             model.LeaveCategoryPersonal,
			Reason:               "exam",
			CreatedAt:            now,
			UpdatedAt:            now,
		},
		Request: &model.ShiftChangeRequest{
			ID:                    77,
			PublicationID:         1,
			Type:                  model.ShiftChangeTypeGivePool,
			RequesterUserID:       7,
			RequesterAssignmentID: 100,
			OccurrenceDate:        time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC),
			State:                 model.ShiftChangeStatePending,
			LeaveID:               &leaveID,
			CreatedAt:             now,
			ExpiresAt:             now.Add(24 * time.Hour),
		},
		State:         model.LeaveStatePending,
		RequesterName: "Alice",
		Shift: &service.LeaveShiftContext{
			AssignmentID:    100,
			SlotID:          21,
			Weekday:         1,
			StartTime:       "09:00",
			EndTime:         "12:00",
			PositionID:      101,
			PositionName:    "Front Desk",
			OccurrenceStart: now.Add(24 * time.Hour),
			OccurrenceEnd:   now.Add(27 * time.Hour),
		},
	}
}
