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

type stubPublicationService struct {
	listPublicationsFunc               func(ctx context.Context, input service.ListPublicationsInput) (*service.ListPublicationsResult, error)
	createPublicationFunc              func(ctx context.Context, input service.CreatePublicationInput) (*model.Publication, error)
	updatePublicationFunc              func(ctx context.Context, input service.UpdatePublicationInput) (*model.Publication, error)
	getPublicationByIDFunc             func(ctx context.Context, id int64) (*model.Publication, error)
	deletePublicationFunc              func(ctx context.Context, id int64) error
	getCurrentPublicationFunc          func(ctx context.Context) (*model.Publication, error)
	listSubmissionSlotsFunc            func(ctx context.Context, publicationID, userID int64) ([]model.SlotRef, error)
	createAvailabilitySubmissionFunc   func(ctx context.Context, input service.CreateAvailabilitySubmissionInput) (*model.AvailabilitySubmission, error)
	deleteAvailabilitySubmissionFunc   func(ctx context.Context, input service.DeleteAvailabilitySubmissionInput) error
	listQualifiedPublicationShiftsFunc func(ctx context.Context, publicationID, userID int64) ([]*model.QualifiedShift, error)
	getAssignmentBoardFunc             func(ctx context.Context, publicationID int64) (*service.AssignmentBoardResult, error)
	autoAssignPublicationFunc          func(ctx context.Context, publicationID int64) (*service.AssignmentBoardResult, error)
	createAssignmentFunc               func(ctx context.Context, input service.CreateAssignmentInput) (*model.Assignment, error)
	deleteAssignmentFunc               func(ctx context.Context, input service.DeleteAssignmentInput) error
	activatePublicationFunc            func(ctx context.Context, publicationID int64) (*model.Publication, error)
	publishPublicationFunc             func(ctx context.Context, publicationID int64) (*model.Publication, error)
	endPublicationFunc                 func(ctx context.Context, publicationID int64) (*model.Publication, error)
	getPublicationRosterFunc           func(ctx context.Context, publicationID int64) (*service.RosterResult, error)
	getPublicationRosterWithWeekFunc   func(ctx context.Context, publicationID int64, weekStart *time.Time) (*service.RosterResult, error)
	getCurrentRosterFunc               func(ctx context.Context) (*service.RosterResult, error)
}

func (s *stubPublicationService) ListPublications(ctx context.Context, input service.ListPublicationsInput) (*service.ListPublicationsResult, error) {
	return s.listPublicationsFunc(ctx, input)
}

func (s *stubPublicationService) CreatePublication(ctx context.Context, input service.CreatePublicationInput) (*model.Publication, error) {
	return s.createPublicationFunc(ctx, input)
}

func (s *stubPublicationService) UpdatePublication(ctx context.Context, input service.UpdatePublicationInput) (*model.Publication, error) {
	return s.updatePublicationFunc(ctx, input)
}

func (s *stubPublicationService) GetPublicationByID(ctx context.Context, id int64) (*model.Publication, error) {
	return s.getPublicationByIDFunc(ctx, id)
}

func (s *stubPublicationService) DeletePublication(ctx context.Context, id int64) error {
	return s.deletePublicationFunc(ctx, id)
}

func (s *stubPublicationService) GetCurrentPublication(ctx context.Context) (*model.Publication, error) {
	return s.getCurrentPublicationFunc(ctx)
}

func (s *stubPublicationService) ListAvailabilitySubmissionSlots(ctx context.Context, publicationID, userID int64) ([]model.SlotRef, error) {
	return s.listSubmissionSlotsFunc(ctx, publicationID, userID)
}

func (s *stubPublicationService) CreateAvailabilitySubmission(ctx context.Context, input service.CreateAvailabilitySubmissionInput) (*model.AvailabilitySubmission, error) {
	return s.createAvailabilitySubmissionFunc(ctx, input)
}

func (s *stubPublicationService) DeleteAvailabilitySubmission(ctx context.Context, input service.DeleteAvailabilitySubmissionInput) error {
	return s.deleteAvailabilitySubmissionFunc(ctx, input)
}

func (s *stubPublicationService) ListQualifiedPublicationSlotPositions(ctx context.Context, publicationID, userID int64) ([]*model.QualifiedShift, error) {
	return s.listQualifiedPublicationShiftsFunc(ctx, publicationID, userID)
}

func (s *stubPublicationService) GetAssignmentBoard(ctx context.Context, publicationID int64) (*service.AssignmentBoardResult, error) {
	return s.getAssignmentBoardFunc(ctx, publicationID)
}

func (s *stubPublicationService) AutoAssignPublication(ctx context.Context, publicationID int64) (*service.AssignmentBoardResult, error) {
	return s.autoAssignPublicationFunc(ctx, publicationID)
}

func (s *stubPublicationService) CreateAssignment(ctx context.Context, input service.CreateAssignmentInput) (*model.Assignment, error) {
	return s.createAssignmentFunc(ctx, input)
}

func (s *stubPublicationService) DeleteAssignment(ctx context.Context, input service.DeleteAssignmentInput) error {
	return s.deleteAssignmentFunc(ctx, input)
}

func (s *stubPublicationService) ActivatePublication(ctx context.Context, publicationID int64) (*model.Publication, error) {
	return s.activatePublicationFunc(ctx, publicationID)
}

func (s *stubPublicationService) EndPublication(ctx context.Context, publicationID int64) (*model.Publication, error) {
	return s.endPublicationFunc(ctx, publicationID)
}

func (s *stubPublicationService) PublishPublication(ctx context.Context, publicationID int64) (*model.Publication, error) {
	return s.publishPublicationFunc(ctx, publicationID)
}

func (s *stubPublicationService) GetPublicationRoster(ctx context.Context, publicationID int64, weekStart *time.Time) (*service.RosterResult, error) {
	if s.getPublicationRosterWithWeekFunc != nil {
		return s.getPublicationRosterWithWeekFunc(ctx, publicationID, weekStart)
	}
	return s.getPublicationRosterFunc(ctx, publicationID)
}

func (s *stubPublicationService) GetCurrentRoster(ctx context.Context) (*service.RosterResult, error) {
	return s.getCurrentRosterFunc(ctx)
}

func TestPublicationHandler(t *testing.T) {
	t.Run("List returns publications", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			listPublicationsFunc: func(ctx context.Context, input service.ListPublicationsInput) (*service.ListPublicationsResult, error) {
				return &service.ListPublicationsResult{
					Publications: []*model.Publication{samplePublication()},
					Page:         1,
					PageSize:     10,
					Total:        1,
					TotalPages:   1,
				}, nil
			},
		})
		recorder := httptest.NewRecorder()

		handler.List(recorder, httptest.NewRequest(http.MethodGet, "/publications?page=1&page_size=10", nil))

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[publicationsResponse](t, recorder)
		if len(response.Publications) != 1 || response.Publications[0].ID != 1 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("Create returns created publication", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			createPublicationFunc: func(ctx context.Context, input service.CreatePublicationInput) (*model.Publication, error) {
				return samplePublication(), nil
			},
		})
		recorder := httptest.NewRecorder()

		handler.Create(recorder, jsonRequest(t, http.MethodPost, "/publications", sampleCreatePublicationPayload()))

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", recorder.Code)
		}
		response := decodeJSONResponse[publicationDetailResponse](t, recorder)
		if response.Publication == nil || response.Publication.ID != 1 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("Create rejects invalid JSON", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{})
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/publications", strings.NewReader("{"))

		handler.Create(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("Create maps service errors", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name   string
			err    error
			status int
			code   string
		}{
			{name: "publication already exists", err: service.ErrPublicationAlreadyExists, status: http.StatusConflict, code: "PUBLICATION_ALREADY_EXISTS"},
			{name: "invalid publication window", err: service.ErrInvalidPublicationWindow, status: http.StatusBadRequest, code: "INVALID_PUBLICATION_WINDOW"},
			{name: "template not found", err: service.ErrTemplateNotFound, status: http.StatusNotFound, code: "TEMPLATE_NOT_FOUND"},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				handler := NewPublicationHandler(&stubPublicationService{
					createPublicationFunc: func(ctx context.Context, input service.CreatePublicationInput) (*model.Publication, error) {
						return nil, tc.err
					},
				})
				recorder := httptest.NewRecorder()

				handler.Create(recorder, jsonRequest(t, http.MethodPost, "/publications", sampleCreatePublicationPayload()))

				assertErrorResponse(t, recorder, tc.status, tc.code)
			})
		}
	})

	t.Run("GetByID returns publication", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			getPublicationByIDFunc: func(ctx context.Context, id int64) (*model.Publication, error) {
				return samplePublication(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/1", nil), map[string]string{"id": "1"})

		handler.GetByID(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[publicationDetailResponse](t, recorder)
		if response.Publication == nil || response.Publication.ID != 1 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("GetByID rejects invalid path id", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/bad", nil), map[string]string{"id": "bad"})

		handler.GetByID(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("GetByID maps publication not found", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			getPublicationByIDFunc: func(ctx context.Context, id int64) (*model.Publication, error) {
				return nil, service.ErrPublicationNotFound
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/2", nil), map[string]string{"id": "2"})

		handler.GetByID(recorder, req)

		assertErrorResponse(t, recorder, http.StatusNotFound, "PUBLICATION_NOT_FOUND")
	})

	t.Run("Delete returns no content", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			deletePublicationFunc: func(ctx context.Context, id int64) error { return nil },
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodDelete, "/publications/1", nil), map[string]string{"id": "1"})

		handler.Delete(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", recorder.Code)
		}
	})

	t.Run("Delete maps publication not deletable", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			deletePublicationFunc: func(ctx context.Context, id int64) error { return service.ErrPublicationNotDeletable },
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodDelete, "/publications/1", nil), map[string]string{"id": "1"})

		handler.Delete(recorder, req)

		assertErrorResponse(t, recorder, http.StatusConflict, "PUBLICATION_NOT_DELETABLE")
	})

	t.Run("GetCurrent returns publication", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			getCurrentPublicationFunc: func(ctx context.Context) (*model.Publication, error) {
				return samplePublication(), nil
			},
		})
		recorder := httptest.NewRecorder()

		handler.GetCurrent(recorder, httptest.NewRequest(http.MethodGet, "/publications/current", nil))

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[currentPublicationResponse](t, recorder)
		if response.Publication == nil || response.Publication.ID != 1 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("GetCurrent returns null when no publication", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			getCurrentPublicationFunc: func(ctx context.Context) (*model.Publication, error) {
				return nil, nil
			},
		})
		recorder := httptest.NewRecorder()

		handler.GetCurrent(recorder, httptest.NewRequest(http.MethodGet, "/publications/current", nil))

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[currentPublicationResponse](t, recorder)
		if response.Publication != nil {
			t.Fatalf("expected nil publication, got %+v", response.Publication)
		}
	})

	t.Run("ListMySubmissionSlots returns slots", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			listSubmissionSlotsFunc: func(ctx context.Context, publicationID, userID int64) ([]model.SlotRef, error) {
				return []model.SlotRef{
					{SlotID: 21},
					{SlotID: 22},
				}, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/1/submissions/me", nil), map[string]string{"id": "1"}),
			sampleUser(),
		)

		handler.ListMySubmissionSlots(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[submissionsMeResponse](t, recorder)
		if len(response.Submissions) != 2 ||
			response.Submissions[0].SlotID != 21 ||
			response.Submissions[1].SlotID != 22 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("CreateSubmission returns created status", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			createAvailabilitySubmissionFunc: func(ctx context.Context, input service.CreateAvailabilitySubmissionInput) (*model.AvailabilitySubmission, error) {
				return &model.AvailabilitySubmission{
					ID:            1,
					PublicationID: input.PublicationID,
					UserID:        input.UserID,
					SlotID:        input.SlotID,
					Weekday:       input.Weekday,
					CreatedAt:     samplePublicationTime(),
				}, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(jsonRequest(t, http.MethodPost, "/publications/1/submissions", map[string]any{
				"slot_id": 21, "weekday": 1,
			}), map[string]string{"id": "1"}),
			sampleUser(),
		)

		handler.CreateSubmission(recorder, req)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", recorder.Code)
		}
	})

	t.Run("CreateSubmission ignores stray position id", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			createAvailabilitySubmissionFunc: func(ctx context.Context, input service.CreateAvailabilitySubmissionInput) (*model.AvailabilitySubmission, error) {
				if input.SlotID != 21 {
					t.Fatalf("expected slot_id=21, got %+v", input)
				}
				return &model.AvailabilitySubmission{
					ID:            1,
					PublicationID: input.PublicationID,
					UserID:        input.UserID,
					SlotID:        input.SlotID,
					Weekday:       input.Weekday,
					CreatedAt:     samplePublicationTime(),
				}, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(jsonRequest(t, http.MethodPost, "/publications/1/submissions", map[string]any{
				"slot_id": 21, "weekday": 1, "position_id": 101,
			}), map[string]string{"id": "1"}),
			sampleUser(),
		)

		handler.CreateSubmission(recorder, req)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", recorder.Code)
		}
	})

	t.Run("CreateSubmission maps service errors", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name   string
			err    error
			status int
			code   string
		}{
			{name: "publication not collecting", err: service.ErrPublicationNotCollecting, status: http.StatusConflict, code: "PUBLICATION_NOT_COLLECTING"},
			{name: "not qualified", err: service.ErrNotQualified, status: http.StatusForbidden, code: "NOT_QUALIFIED"},
			{name: "slot not found", err: service.ErrTemplateSlotNotFound, status: http.StatusNotFound, code: "TEMPLATE_SLOT_NOT_FOUND"},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				handler := NewPublicationHandler(&stubPublicationService{
					createAvailabilitySubmissionFunc: func(ctx context.Context, input service.CreateAvailabilitySubmissionInput) (*model.AvailabilitySubmission, error) {
						return nil, tc.err
					},
				})
				recorder := httptest.NewRecorder()
				req := requestWithUser(
					requestWithPathValues(jsonRequest(t, http.MethodPost, "/publications/1/submissions", map[string]any{
						"slot_id": 21, "weekday": 1,
					}), map[string]string{"id": "1"}),
					sampleUser(),
				)

				handler.CreateSubmission(recorder, req)

				assertErrorResponse(t, recorder, tc.status, tc.code)
			})
		}
	})

	t.Run("CreateSubmission rejects legacy shift id body", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(jsonRequest(t, http.MethodPost, "/publications/1/submissions", map[string]any{
				"template" + "_shift_id": 2,
			}), map[string]string{"id": "1"}),
			sampleUser(),
		)

		handler.CreateSubmission(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("DeleteSubmission returns no content", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			deleteAvailabilitySubmissionFunc: func(ctx context.Context, input service.DeleteAvailabilitySubmissionInput) error { return nil },
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(httptest.NewRequest(http.MethodDelete, "/publications/1/submissions/21/1", nil), map[string]string{
				"id": "1", "slot_id": "21", "weekday": "1",
			}),
			sampleUser(),
		)

		handler.DeleteSubmission(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", recorder.Code)
		}
	})

	t.Run("DeleteSubmission maps publication not collecting", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			deleteAvailabilitySubmissionFunc: func(ctx context.Context, input service.DeleteAvailabilitySubmissionInput) error {
				return service.ErrPublicationNotCollecting
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(httptest.NewRequest(http.MethodDelete, "/publications/1/submissions/21/1", nil), map[string]string{
				"id": "1", "slot_id": "21", "weekday": "1",
			}),
			sampleUser(),
		)

		handler.DeleteSubmission(recorder, req)

		assertErrorResponse(t, recorder, http.StatusConflict, "PUBLICATION_NOT_COLLECTING")
	})

	t.Run("ListMyQualifiedShifts returns shifts", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			listQualifiedPublicationShiftsFunc: func(ctx context.Context, publicationID, userID int64) ([]*model.QualifiedShift, error) {
				return []*model.QualifiedShift{sampleQualifiedShift()}, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/1/shifts/me", nil), map[string]string{"id": "1"}),
			sampleUser(),
		)

		handler.ListMyQualifiedShifts(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[shiftsMeResponse](t, recorder)
		if len(response.Shifts) != 1 ||
			response.Shifts[0].SlotID != 21 ||
			len(response.Shifts[0].Composition) != 1 ||
			response.Shifts[0].Composition[0].PositionID != 101 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("ListMyQualifiedShifts maps publication not collecting", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			listQualifiedPublicationShiftsFunc: func(ctx context.Context, publicationID, userID int64) ([]*model.QualifiedShift, error) {
				return nil, service.ErrPublicationNotCollecting
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(
			requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/1/shifts/me", nil), map[string]string{"id": "1"}),
			sampleUser(),
		)

		handler.ListMyQualifiedShifts(recorder, req)

		assertErrorResponse(t, recorder, http.StatusConflict, "PUBLICATION_NOT_COLLECTING")
	})

	t.Run("GetAssignmentBoard returns board", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			getAssignmentBoardFunc: func(ctx context.Context, publicationID int64) (*service.AssignmentBoardResult, error) {
				return sampleAssignmentBoardResult(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/1/assignment-board", nil), map[string]string{"id": "1"})

		handler.GetAssignmentBoard(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		body := recorder.Body.String()
		if strings.Contains(body, `"candidates"`) || strings.Contains(body, `"non_candidate_qualified"`) {
			t.Fatalf("assignment-board response still carries removed per-position fields: %s", body)
		}
		response := decodeJSONResponse[assignmentBoardResponse](t, recorder)
		if response.Publication == nil ||
			len(response.Slots) != 1 ||
			len(response.Slots[0].Positions) != 1 ||
			len(response.Employees) != 2 ||
			len(response.Employees[0].PositionIDs) != 1 ||
			len(response.Slots[0].Positions[0].Assignments) != 1 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("GetAssignmentBoard maps publication not assigning", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			getAssignmentBoardFunc: func(ctx context.Context, publicationID int64) (*service.AssignmentBoardResult, error) {
				return nil, service.ErrPublicationNotAssigning
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/1/assignment-board", nil), map[string]string{"id": "1"})

		handler.GetAssignmentBoard(recorder, req)

		assertErrorResponse(t, recorder, http.StatusConflict, "PUBLICATION_NOT_ASSIGNING")
	})

	t.Run("CreateAssignment returns created status", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			createAssignmentFunc: func(ctx context.Context, input service.CreateAssignmentInput) (*model.Assignment, error) {
				return &model.Assignment{
					ID:            1,
					PublicationID: input.PublicationID,
					UserID:        input.UserID,
					SlotID:        input.SlotID,
					Weekday:       input.Weekday,
					PositionID:    input.PositionID,
					CreatedAt:     samplePublicationTime(),
				}, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(jsonRequest(t, http.MethodPost, "/publications/1/assignments", map[string]any{
			"user_id": 1, "slot_id": 21, "weekday": 1, "position_id": 101,
		}), map[string]string{"id": "1"})

		handler.CreateAssignment(recorder, req)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", recorder.Code)
		}
	})

	t.Run("CreateAssignment maps service errors", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name   string
			err    error
			status int
			code   string
		}{
			{name: "publication not assigning", err: service.ErrPublicationNotAssigning, status: http.StatusConflict, code: "PUBLICATION_NOT_ASSIGNING"},
			{name: "user not found", err: service.ErrUserNotFound, status: http.StatusNotFound, code: "USER_NOT_FOUND"},
			{name: "user disabled", err: service.ErrUserDisabled, status: http.StatusConflict, code: "USER_DISABLED"},
			{name: "user already in slot", err: service.ErrAssignmentUserAlreadyInSlot, status: http.StatusConflict, code: "ASSIGNMENT_USER_ALREADY_IN_SLOT"},
			{name: "retryable scheduling", err: service.ErrSchedulingRetryable, status: http.StatusServiceUnavailable, code: "SCHEDULING_RETRYABLE"},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				handler := NewPublicationHandler(&stubPublicationService{
					createAssignmentFunc: func(ctx context.Context, input service.CreateAssignmentInput) (*model.Assignment, error) {
						return nil, tc.err
					},
				})
				recorder := httptest.NewRecorder()
				req := requestWithPathValues(jsonRequest(t, http.MethodPost, "/publications/1/assignments", map[string]any{
					"user_id": 1, "slot_id": 21, "weekday": 1, "position_id": 101,
				}), map[string]string{"id": "1"})

				handler.CreateAssignment(recorder, req)

				assertErrorResponse(t, recorder, tc.status, tc.code)
			})
		}
	})

	t.Run("CreateAssignment rejects legacy shift id body", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(jsonRequest(t, http.MethodPost, "/publications/1/assignments", map[string]any{
			"user_id": 1, "template" + "_shift_id": 2,
		}), map[string]string{"id": "1"})

		handler.CreateAssignment(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("DeleteAssignment returns no content", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			deleteAssignmentFunc: func(ctx context.Context, input service.DeleteAssignmentInput) error { return nil },
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodDelete, "/publications/1/assignments/3", nil), map[string]string{"id": "1", "assignment_id": "3"})

		handler.DeleteAssignment(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", recorder.Code)
		}
	})

	t.Run("DeleteAssignment maps publication not assigning", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			deleteAssignmentFunc: func(ctx context.Context, input service.DeleteAssignmentInput) error {
				return service.ErrPublicationNotAssigning
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodDelete, "/publications/1/assignments/3", nil), map[string]string{"id": "1", "assignment_id": "3"})

		handler.DeleteAssignment(recorder, req)

		assertErrorResponse(t, recorder, http.StatusConflict, "PUBLICATION_NOT_ASSIGNING")
	})

	t.Run("Activate returns publication", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			activatePublicationFunc: func(ctx context.Context, publicationID int64) (*model.Publication, error) {
				publication := samplePublication()
				publication.State = model.PublicationStateActive
				activatedAt := samplePublicationTime()
				publication.ActivatedAt = &activatedAt
				return publication, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodPost, "/publications/1/activate", nil), map[string]string{"id": "1"})

		handler.Activate(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[publicationDetailResponse](t, recorder)
		if response.Publication == nil || response.Publication.State != model.PublicationStateActive {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("Activate maps publication not assigning", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			activatePublicationFunc: func(ctx context.Context, publicationID int64) (*model.Publication, error) {
				return nil, service.ErrPublicationNotAssigning
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodPost, "/publications/1/activate", nil), map[string]string{"id": "1"})

		handler.Activate(recorder, req)

		assertErrorResponse(t, recorder, http.StatusConflict, "PUBLICATION_NOT_ASSIGNING")
	})

	t.Run("End returns publication", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			endPublicationFunc: func(ctx context.Context, publicationID int64) (*model.Publication, error) {
				publication := samplePublication()
				publication.State = model.PublicationStateEnded
				publication.PlannedActiveUntil = samplePublicationTime()
				return publication, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodPost, "/publications/1/end", nil), map[string]string{"id": "1"})

		handler.End(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[publicationDetailResponse](t, recorder)
		if response.Publication == nil || response.Publication.State != model.PublicationStateEnded {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("End maps publication not active", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			endPublicationFunc: func(ctx context.Context, publicationID int64) (*model.Publication, error) {
				return nil, service.ErrPublicationNotActive
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodPost, "/publications/1/end", nil), map[string]string{"id": "1"})

		handler.End(recorder, req)

		assertErrorResponse(t, recorder, http.StatusConflict, "PUBLICATION_NOT_ACTIVE")
	})

	t.Run("GetRoster returns roster", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			getPublicationRosterFunc: func(ctx context.Context, publicationID int64) (*service.RosterResult, error) {
				return sampleRosterResult(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/1/roster", nil), map[string]string{"id": "1"})

		handler.GetRoster(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[rosterResponse](t, recorder)
		if response.Publication == nil || len(response.Weekdays) != 1 || len(response.Weekdays[0].Slots) != 1 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("GetRoster maps publication not active", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			getPublicationRosterFunc: func(ctx context.Context, publicationID int64) (*service.RosterResult, error) {
				return nil, service.ErrPublicationNotActive
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/publications/1/roster", nil), map[string]string{"id": "1"})

		handler.GetRoster(recorder, req)

		assertErrorResponse(t, recorder, http.StatusConflict, "PUBLICATION_NOT_ACTIVE")
	})

	t.Run("GetCurrentRoster returns roster", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			getCurrentRosterFunc: func(ctx context.Context) (*service.RosterResult, error) {
				return sampleRosterResult(), nil
			},
		})
		recorder := httptest.NewRecorder()

		handler.GetCurrentRoster(recorder, httptest.NewRequest(http.MethodGet, "/publications/current/roster", nil))

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[rosterResponse](t, recorder)
		if response.Publication == nil || response.Publication.ID != 1 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("AutoAssign returns assignment board", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			autoAssignPublicationFunc: func(ctx context.Context, publicationID int64) (*service.AssignmentBoardResult, error) {
				return sampleAssignmentBoardResult(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodPost, "/publications/1/auto-assign", nil), map[string]string{"id": "1"})

		handler.AutoAssign(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[assignmentBoardResponse](t, recorder)
		if response.Publication == nil || len(response.Slots) != 1 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("AutoAssign maps publication not assigning", func(t *testing.T) {
		t.Parallel()

		handler := NewPublicationHandler(&stubPublicationService{
			autoAssignPublicationFunc: func(ctx context.Context, publicationID int64) (*service.AssignmentBoardResult, error) {
				return nil, service.ErrPublicationNotAssigning
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodPost, "/publications/1/auto-assign", nil), map[string]string{"id": "1"})

		handler.AutoAssign(recorder, req)

		assertErrorResponse(t, recorder, http.StatusConflict, "PUBLICATION_NOT_ASSIGNING")
	})
}

func samplePublication() *model.Publication {
	now := samplePublicationTime()
	return &model.Publication{
		ID:                 1,
		TemplateID:         1,
		TemplateName:       "Weekday Template",
		Name:               "Week 16",
		State:              model.PublicationStateCollecting,
		SubmissionStartAt:  now.Add(-24 * time.Hour),
		SubmissionEndAt:    now.Add(24 * time.Hour),
		PlannedActiveFrom:  now.Add(48 * time.Hour),
		PlannedActiveUntil: now.Add(8 * 7 * 24 * time.Hour),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func samplePublicationSlot() *model.TemplateSlot {
	now := samplePublicationTime()
	return &model.TemplateSlot{
		ID:         21,
		TemplateID: 1,
		Weekdays:   []int{1},
		StartTime:  "09:00",
		EndTime:    "12:00",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func samplePublicationPosition() *model.Position {
	return &model.Position{
		ID:   7,
		Name: "Front Desk",
	}
}

func sampleAssignmentBoardResult() *service.AssignmentBoardResult {
	return &service.AssignmentBoardResult{
		Publication: samplePublication(),
		Employees: []*model.AssignmentBoardEmployee{
			{
				UserID:      1,
				Name:        "Worker",
				Email:       "worker@example.com",
				PositionIDs: []int64{7},
			},
			{
				UserID:      2,
				Name:        "Available",
				Email:       "available@example.com",
				PositionIDs: []int64{7},
			},
		},
		Slots: []*service.AssignmentBoardSlotResult{
			{
				Slot: samplePublicationSlot(),
				Positions: []*service.AssignmentBoardPositionResult{
					{
						Position:          samplePublicationPosition(),
						RequiredHeadcount: 2,
						Assignments: []*model.AssignmentParticipant{
							{
								AssignmentID: 3,
								SlotID:       21,
								PositionID:   7,
								UserID:       1,
								Name:         "Worker",
								Email:        "worker@example.com",
								CreatedAt:    samplePublicationTime(),
							},
						},
					},
				},
			},
		},
	}
}

func sampleRosterResult() *service.RosterResult {
	return &service.RosterResult{
		Publication: samplePublication(),
		WeekStart:   time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
		Weekdays: []*service.RosterWeekdayResult{
			{
				Weekday: 1,
				Slots: []*service.RosterSlotResult{
					{
						Slot:           samplePublicationSlot(),
						OccurrenceDate: time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
						Positions: []*service.RosterPositionResult{
							{
								Position:          samplePublicationPosition(),
								RequiredHeadcount: 2,
								Assignments: []*model.AssignmentParticipant{
									{
										AssignmentID: 3,
										SlotID:       21,
										PositionID:   7,
										UserID:       1,
										Name:         "Worker",
										Email:        "worker@example.com",
										CreatedAt:    samplePublicationTime(),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func sampleCreatePublicationPayload() map[string]any {
	now := samplePublicationTime()
	return map[string]any{
		"template_id":          1,
		"name":                 "Week 16",
		"submission_start_at":  now,
		"submission_end_at":    now.Add(24 * time.Hour),
		"planned_active_from":  now.Add(48 * time.Hour),
		"planned_active_until": now.Add(8 * 7 * 24 * time.Hour),
	}
}

func samplePublicationTime() time.Time {
	return time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC)
}
