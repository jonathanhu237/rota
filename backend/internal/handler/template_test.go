package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

type stubTemplateService struct {
	listTemplatesFunc       func(ctx context.Context, input service.ListTemplatesInput) (*service.ListTemplatesResult, error)
	createTemplateFunc      func(ctx context.Context, input service.CreateTemplateInput) (*model.Template, error)
	getTemplateByIDFunc     func(ctx context.Context, id int64) (*model.Template, error)
	updateTemplateFunc      func(ctx context.Context, input service.UpdateTemplateInput) (*model.Template, error)
	deleteTemplateFunc      func(ctx context.Context, id int64) error
	cloneTemplateFunc       func(ctx context.Context, id int64) (*model.Template, error)
	createTemplateShiftFunc func(ctx context.Context, input service.CreateTemplateShiftInput) (*model.TemplateShift, error)
	updateTemplateShiftFunc func(ctx context.Context, input service.UpdateTemplateShiftInput) (*model.TemplateShift, error)
	deleteTemplateShiftFunc func(ctx context.Context, templateID, shiftID int64) error
}

func (s *stubTemplateService) ListTemplates(ctx context.Context, input service.ListTemplatesInput) (*service.ListTemplatesResult, error) {
	return s.listTemplatesFunc(ctx, input)
}

func (s *stubTemplateService) CreateTemplate(ctx context.Context, input service.CreateTemplateInput) (*model.Template, error) {
	return s.createTemplateFunc(ctx, input)
}

func (s *stubTemplateService) GetTemplateByID(ctx context.Context, id int64) (*model.Template, error) {
	return s.getTemplateByIDFunc(ctx, id)
}

func (s *stubTemplateService) UpdateTemplate(ctx context.Context, input service.UpdateTemplateInput) (*model.Template, error) {
	return s.updateTemplateFunc(ctx, input)
}

func (s *stubTemplateService) DeleteTemplate(ctx context.Context, id int64) error {
	return s.deleteTemplateFunc(ctx, id)
}

func (s *stubTemplateService) CloneTemplate(ctx context.Context, id int64) (*model.Template, error) {
	return s.cloneTemplateFunc(ctx, id)
}

func (s *stubTemplateService) CreateTemplateShift(ctx context.Context, input service.CreateTemplateShiftInput) (*model.TemplateShift, error) {
	return s.createTemplateShiftFunc(ctx, input)
}

func (s *stubTemplateService) UpdateTemplateShift(ctx context.Context, input service.UpdateTemplateShiftInput) (*model.TemplateShift, error) {
	return s.updateTemplateShiftFunc(ctx, input)
}

func (s *stubTemplateService) DeleteTemplateShift(ctx context.Context, templateID, shiftID int64) error {
	return s.deleteTemplateShiftFunc(ctx, templateID, shiftID)
}

func TestTemplateHandler(t *testing.T) {
	t.Run("List returns templates", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{
			listTemplatesFunc: func(ctx context.Context, input service.ListTemplatesInput) (*service.ListTemplatesResult, error) {
				return &service.ListTemplatesResult{
					Templates:  []*model.Template{sampleTemplate()},
					Page:       1,
					PageSize:   10,
					Total:      1,
					TotalPages: 1,
				}, nil
			},
		})

		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/templates?page=1&page_size=10", nil)

		handler.List(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[templatesResponse](t, recorder)
		if len(response.Templates) != 1 || response.Templates[0].ID != 1 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("List maps service errors to internal error", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{
			listTemplatesFunc: func(ctx context.Context, input service.ListTemplatesInput) (*service.ListTemplatesResult, error) {
				return nil, errors.New("boom")
			},
		})

		recorder := httptest.NewRecorder()
		handler.List(recorder, httptest.NewRequest(http.MethodGet, "/templates", nil))

		assertErrorResponse(t, recorder, http.StatusInternalServerError, "INTERNAL_ERROR")
	})

	t.Run("Create returns created template", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{
			createTemplateFunc: func(ctx context.Context, input service.CreateTemplateInput) (*model.Template, error) {
				return sampleTemplate(), nil
			},
		})

		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPost, "/templates", map[string]any{
			"name":        "Weekday",
			"description": "Core shifts",
		})

		handler.Create(recorder, req)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", recorder.Code)
		}
		response := decodeJSONResponse[templateDetailResponse](t, recorder)
		if response.Template.ID != 1 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("Create rejects invalid JSON", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{})
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/templates", stringsReader("{"))

		handler.Create(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("Create maps invalid input", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{
			createTemplateFunc: func(ctx context.Context, input service.CreateTemplateInput) (*model.Template, error) {
				return nil, service.ErrInvalidInput
			},
		})
		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPost, "/templates", map[string]any{"name": "", "description": ""})

		handler.Create(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("GetByID returns template", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{
			getTemplateByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
				return sampleTemplate(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/templates/1", nil), map[string]string{"id": "1"})

		handler.GetByID(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
	})

	t.Run("GetByID rejects invalid path id", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/templates/bad", nil), map[string]string{"id": "bad"})

		handler.GetByID(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("GetByID maps template not found", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{
			getTemplateByIDFunc: func(ctx context.Context, id int64) (*model.Template, error) {
				return nil, service.ErrTemplateNotFound
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/templates/2", nil), map[string]string{"id": "2"})

		handler.GetByID(recorder, req)

		assertErrorResponse(t, recorder, http.StatusNotFound, "TEMPLATE_NOT_FOUND")
	})

	t.Run("Update returns template", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{
			updateTemplateFunc: func(ctx context.Context, input service.UpdateTemplateInput) (*model.Template, error) {
				return sampleTemplate(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(jsonRequest(t, http.MethodPut, "/templates/1", map[string]any{
			"name":        "Weekday",
			"description": "Updated",
		}), map[string]string{"id": "1"})

		handler.Update(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
	})

	t.Run("Update maps template locked", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{
			updateTemplateFunc: func(ctx context.Context, input service.UpdateTemplateInput) (*model.Template, error) {
				return nil, service.ErrTemplateLocked
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(jsonRequest(t, http.MethodPut, "/templates/1", map[string]any{
			"name":        "Weekday",
			"description": "Locked",
		}), map[string]string{"id": "1"})

		handler.Update(recorder, req)

		assertErrorResponse(t, recorder, http.StatusConflict, "TEMPLATE_LOCKED")
	})

	t.Run("Delete returns no content", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{
			deleteTemplateFunc: func(ctx context.Context, id int64) error { return nil },
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodDelete, "/templates/1", nil), map[string]string{"id": "1"})

		handler.Delete(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", recorder.Code)
		}
	})

	t.Run("Delete maps template locked", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{
			deleteTemplateFunc: func(ctx context.Context, id int64) error { return service.ErrTemplateLocked },
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodDelete, "/templates/1", nil), map[string]string{"id": "1"})

		handler.Delete(recorder, req)

		assertErrorResponse(t, recorder, http.StatusConflict, "TEMPLATE_LOCKED")
	})

	t.Run("Clone returns created clone", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{
			cloneTemplateFunc: func(ctx context.Context, id int64) (*model.Template, error) {
				template := sampleTemplate()
				template.ID = 2
				return template, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodPost, "/templates/1/clone", nil), map[string]string{"id": "1"})

		handler.Clone(recorder, req)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", recorder.Code)
		}
	})

	t.Run("Clone maps template not found", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{
			cloneTemplateFunc: func(ctx context.Context, id int64) (*model.Template, error) {
				return nil, service.ErrTemplateNotFound
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodPost, "/templates/9/clone", nil), map[string]string{"id": "9"})

		handler.Clone(recorder, req)

		assertErrorResponse(t, recorder, http.StatusNotFound, "TEMPLATE_NOT_FOUND")
	})

	t.Run("CreateShift returns created shift", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{
			createTemplateShiftFunc: func(ctx context.Context, input service.CreateTemplateShiftInput) (*model.TemplateShift, error) {
				return sampleTemplateShift(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(jsonRequest(t, http.MethodPost, "/templates/1/shifts", map[string]any{
			"weekday": 1, "start_time": "09:00", "end_time": "12:00", "position_id": 7, "required_headcount": 2,
		}), map[string]string{"id": "1"})

		handler.CreateShift(recorder, req)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", recorder.Code)
		}
	})

	t.Run("CreateShift maps service errors", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name   string
			err    error
			status int
			code   string
		}{
			{name: "invalid weekday", err: service.ErrInvalidWeekday, status: http.StatusBadRequest, code: "INVALID_WEEKDAY"},
			{name: "invalid shift time", err: service.ErrInvalidShiftTime, status: http.StatusBadRequest, code: "INVALID_SHIFT_TIME"},
			{name: "invalid headcount", err: service.ErrInvalidHeadcount, status: http.StatusBadRequest, code: "INVALID_HEADCOUNT"},
			{name: "position not found", err: service.ErrPositionNotFound, status: http.StatusNotFound, code: "POSITION_NOT_FOUND"},
			{name: "template locked", err: service.ErrTemplateLocked, status: http.StatusConflict, code: "TEMPLATE_LOCKED"},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				handler := NewTemplateHandler(&stubTemplateService{
					createTemplateShiftFunc: func(ctx context.Context, input service.CreateTemplateShiftInput) (*model.TemplateShift, error) {
						return nil, tc.err
					},
				})
				recorder := httptest.NewRecorder()
				req := requestWithPathValues(jsonRequest(t, http.MethodPost, "/templates/1/shifts", map[string]any{
					"weekday": 1, "start_time": "09:00", "end_time": "12:00", "position_id": 7, "required_headcount": 2,
				}), map[string]string{"id": "1"})

				handler.CreateShift(recorder, req)

				assertErrorResponse(t, recorder, tc.status, tc.code)
			})
		}
	})

	t.Run("UpdateShift returns updated shift", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{
			updateTemplateShiftFunc: func(ctx context.Context, input service.UpdateTemplateShiftInput) (*model.TemplateShift, error) {
				return sampleTemplateShift(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(jsonRequest(t, http.MethodPut, "/templates/1/shifts/2", map[string]any{
			"weekday": 1, "start_time": "09:00", "end_time": "12:00", "position_id": 7, "required_headcount": 2,
		}), map[string]string{"id": "1", "shift_id": "2"})

		handler.UpdateShift(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
	})

	t.Run("UpdateShift maps shift not found", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{
			updateTemplateShiftFunc: func(ctx context.Context, input service.UpdateTemplateShiftInput) (*model.TemplateShift, error) {
				return nil, service.ErrTemplateShiftNotFound
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(jsonRequest(t, http.MethodPut, "/templates/1/shifts/2", map[string]any{
			"weekday": 1, "start_time": "09:00", "end_time": "12:00", "position_id": 7, "required_headcount": 2,
		}), map[string]string{"id": "1", "shift_id": "2"})

		handler.UpdateShift(recorder, req)

		assertErrorResponse(t, recorder, http.StatusNotFound, "TEMPLATE_SHIFT_NOT_FOUND")
	})

	t.Run("DeleteShift returns no content", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{
			deleteTemplateShiftFunc: func(ctx context.Context, templateID, shiftID int64) error { return nil },
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodDelete, "/templates/1/shifts/2", nil), map[string]string{"id": "1", "shift_id": "2"})

		handler.DeleteShift(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", recorder.Code)
		}
	})

	t.Run("DeleteShift maps template locked", func(t *testing.T) {
		t.Parallel()

		handler := NewTemplateHandler(&stubTemplateService{
			deleteTemplateShiftFunc: func(ctx context.Context, templateID, shiftID int64) error { return service.ErrTemplateLocked },
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodDelete, "/templates/1/shifts/2", nil), map[string]string{"id": "1", "shift_id": "2"})

		handler.DeleteShift(recorder, req)

		assertErrorResponse(t, recorder, http.StatusConflict, "TEMPLATE_LOCKED")
	})
}

func sampleTemplate() *model.Template {
	now := time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC)
	return &model.Template{
		ID:          1,
		Name:        "Weekday Template",
		Description: "Core shifts",
		CreatedAt:   now,
		UpdatedAt:   now,
		Shifts:      []*model.TemplateShift{},
	}
}

func sampleTemplateShift() *model.TemplateShift {
	now := time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC)
	return &model.TemplateShift{
		ID:                2,
		TemplateID:        1,
		Weekday:           1,
		StartTime:         "09:00",
		EndTime:           "12:00",
		PositionID:        7,
		RequiredHeadcount: 2,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func stringsReader(value string) *strings.Reader {
	return strings.NewReader(value)
}
