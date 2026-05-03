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

type stubBrandingService struct {
	getBrandingFunc    func(ctx context.Context) (*model.Branding, error)
	updateBrandingFunc func(ctx context.Context, input service.UpdateBrandingInput) (*model.Branding, error)
}

func (s *stubBrandingService) GetBranding(ctx context.Context) (*model.Branding, error) {
	return s.getBrandingFunc(ctx)
}

func (s *stubBrandingService) UpdateBranding(ctx context.Context, input service.UpdateBrandingInput) (*model.Branding, error) {
	return s.updateBrandingFunc(ctx, input)
}

func TestBrandingHandler(t *testing.T) {
	t.Run("Get returns public branding", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 4, 8, 0, 0, 0, time.UTC)
		handler := NewBrandingHandler(&stubBrandingService{
			getBrandingFunc: func(ctx context.Context) (*model.Branding, error) {
				return &model.Branding{
					ProductName:      "排班系统",
					OrganizationName: "Acme",
					Version:          2,
					CreatedAt:        now,
					UpdatedAt:        now,
				}, nil
			},
		})
		recorder := httptest.NewRecorder()

		handler.Get(recorder, httptest.NewRequest(http.MethodGet, "/branding", nil))

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[brandingResponse](t, recorder)
		if response.ProductName != "排班系统" ||
			response.OrganizationName != "Acme" ||
			response.Version != 2 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("Update sends input and returns branding", func(t *testing.T) {
		t.Parallel()

		handler := NewBrandingHandler(&stubBrandingService{
			updateBrandingFunc: func(ctx context.Context, input service.UpdateBrandingInput) (*model.Branding, error) {
				if input.ProductName != "排班系统" ||
					input.OrganizationName != "Acme" ||
					input.Version != 2 {
					t.Fatalf("unexpected input: %+v", input)
				}
				return &model.Branding{
					ProductName:      input.ProductName,
					OrganizationName: input.OrganizationName,
					Version:          3,
				}, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPut, "/branding", map[string]any{
			"product_name":      "排班系统",
			"organization_name": "Acme",
			"version":           2,
		})

		handler.Update(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		response := decodeJSONResponse[brandingResponse](t, recorder)
		if response.ProductName != "排班系统" || response.Version != 3 {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("Update rejects invalid JSON", func(t *testing.T) {
		t.Parallel()

		handler := NewBrandingHandler(&stubBrandingService{})
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/branding", strings.NewReader("{"))

		handler.Update(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("Update maps invalid input", func(t *testing.T) {
		t.Parallel()

		handler := NewBrandingHandler(&stubBrandingService{
			updateBrandingFunc: func(ctx context.Context, input service.UpdateBrandingInput) (*model.Branding, error) {
				return nil, service.ErrInvalidInput
			},
		})
		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPut, "/branding", map[string]any{
			"product_name": "",
			"version":      1,
		})

		handler.Update(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("Update maps version conflict", func(t *testing.T) {
		t.Parallel()

		handler := NewBrandingHandler(&stubBrandingService{
			updateBrandingFunc: func(ctx context.Context, input service.UpdateBrandingInput) (*model.Branding, error) {
				return nil, service.ErrVersionConflict
			},
		})
		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPut, "/branding", map[string]any{
			"product_name": "Rota",
			"version":      1,
		})

		handler.Update(recorder, req)

		assertErrorResponse(t, recorder, http.StatusConflict, "VERSION_CONFLICT")
	})

	t.Run("Update route rejects anonymous requests", func(t *testing.T) {
		t.Parallel()

		updateCalled := false
		brandingHandler := NewBrandingHandler(&stubBrandingService{
			updateBrandingFunc: func(ctx context.Context, input service.UpdateBrandingInput) (*model.Branding, error) {
				updateCalled = true
				return &model.Branding{}, nil
			},
		})
		authHandler := NewAuthHandler(&stubAuthService{})
		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPut, "/branding", map[string]any{
			"product_name": "排班系统",
			"version":      1,
		})

		authHandler.RequireAdmin(brandingHandler.Update)(recorder, req)

		assertErrorResponse(t, recorder, http.StatusUnauthorized, "UNAUTHORIZED")
		if updateCalled {
			t.Fatalf("branding update should not be called for anonymous requests")
		}
	})

	t.Run("Update route rejects non-admin requests", func(t *testing.T) {
		t.Parallel()

		updateCalled := false
		brandingHandler := NewBrandingHandler(&stubBrandingService{
			updateBrandingFunc: func(ctx context.Context, input service.UpdateBrandingInput) (*model.Branding, error) {
				updateCalled = true
				return &model.Branding{}, nil
			},
		})
		authHandler := NewAuthHandler(&stubAuthService{
			authenticateFunc: func(ctx context.Context, sessionID string) (*service.AuthenticateResult, error) {
				return &service.AuthenticateResult{
					User:      sampleUser(),
					ExpiresIn: 3600,
				}, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPut, "/branding", map[string]any{
			"product_name": "排班系统",
			"version":      1,
		})
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-123"})

		authHandler.RequireAdmin(brandingHandler.Update)(recorder, req)

		assertErrorResponse(t, recorder, http.StatusForbidden, "FORBIDDEN")
		if updateCalled {
			t.Fatalf("branding update should not be called for non-admin requests")
		}
	})
}
