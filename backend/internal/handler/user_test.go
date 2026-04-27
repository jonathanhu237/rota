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

type stubUserService struct {
	listUsersFunc        func(ctx context.Context, input service.ListUsersInput) (*service.ListUsersResult, error)
	createUserFunc       func(ctx context.Context, input service.CreateUserInput) (*model.User, error)
	resendInvitationFunc func(ctx context.Context, userID int64) error
	getUserByIDFunc      func(ctx context.Context, id int64) (*model.User, error)
	updateUserFunc       func(ctx context.Context, input service.UpdateUserInput) (*model.User, error)
	updateOwnProfileFunc func(ctx context.Context, input service.UpdateOwnProfileInput) (*model.User, error)
	updateUserStatusFunc func(ctx context.Context, input service.UpdateUserStatusInput) (*model.User, error)
}

func (s *stubUserService) ListUsers(ctx context.Context, input service.ListUsersInput) (*service.ListUsersResult, error) {
	return s.listUsersFunc(ctx, input)
}

func (s *stubUserService) CreateUser(ctx context.Context, input service.CreateUserInput) (*model.User, error) {
	return s.createUserFunc(ctx, input)
}

func (s *stubUserService) ResendInvitation(ctx context.Context, userID int64) error {
	return s.resendInvitationFunc(ctx, userID)
}

func (s *stubUserService) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	return s.getUserByIDFunc(ctx, id)
}

func (s *stubUserService) UpdateUser(ctx context.Context, input service.UpdateUserInput) (*model.User, error) {
	return s.updateUserFunc(ctx, input)
}

func (s *stubUserService) UpdateOwnProfile(ctx context.Context, input service.UpdateOwnProfileInput) (*model.User, error) {
	return s.updateOwnProfileFunc(ctx, input)
}

func (s *stubUserService) UpdateUserStatus(ctx context.Context, input service.UpdateUserStatusInput) (*model.User, error) {
	return s.updateUserStatusFunc(ctx, input)
}

func TestUserHandler(t *testing.T) {
	t.Run("List returns users", func(t *testing.T) {
		t.Parallel()

		handler := NewUserHandler(&stubUserService{
			listUsersFunc: func(ctx context.Context, input service.ListUsersInput) (*service.ListUsersResult, error) {
				return &service.ListUsersResult{
					Users:      []*model.User{sampleUser()},
					Page:       1,
					PageSize:   10,
					Total:      1,
					TotalPages: 1,
				}, nil
			},
		})
		recorder := httptest.NewRecorder()

		handler.List(recorder, httptest.NewRequest(http.MethodGet, "/users", nil))

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
	})

	t.Run("Create returns created user", func(t *testing.T) {
		t.Parallel()

		handler := NewUserHandler(&stubUserService{
			createUserFunc: func(ctx context.Context, input service.CreateUserInput) (*model.User, error) {
				return sampleUser(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPost, "/users", map[string]any{
			"email":    "worker@example.com",
			"name":     "Worker",
			"is_admin": false,
		})

		handler.Create(recorder, req)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", recorder.Code)
		}
	})

	t.Run("Create rejects invalid JSON", func(t *testing.T) {
		t.Parallel()

		handler := NewUserHandler(&stubUserService{})
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader("{"))

		handler.Create(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("Create maps duplicate email", func(t *testing.T) {
		t.Parallel()

		handler := NewUserHandler(&stubUserService{
			createUserFunc: func(ctx context.Context, input service.CreateUserInput) (*model.User, error) {
				return nil, service.ErrEmailAlreadyExists
			},
		})
		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPost, "/users", map[string]any{
			"email":    "worker@example.com",
			"name":     "Worker",
			"is_admin": false,
		})

		handler.Create(recorder, req)

		assertErrorResponse(t, recorder, http.StatusConflict, "EMAIL_ALREADY_EXISTS")
	})

	t.Run("GetByID returns user", func(t *testing.T) {
		t.Parallel()

		handler := NewUserHandler(&stubUserService{
			getUserByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return sampleUser(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/users/1", nil), map[string]string{"id": "1"})

		handler.GetByID(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
	})

	t.Run("GetByID maps user not found", func(t *testing.T) {
		t.Parallel()

		handler := NewUserHandler(&stubUserService{
			getUserByIDFunc: func(ctx context.Context, id int64) (*model.User, error) {
				return nil, service.ErrUserNotFound
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodGet, "/users/2", nil), map[string]string{"id": "2"})

		handler.GetByID(recorder, req)

		assertErrorResponse(t, recorder, http.StatusNotFound, "USER_NOT_FOUND")
	})

	t.Run("Update returns updated user", func(t *testing.T) {
		t.Parallel()

		handler := NewUserHandler(&stubUserService{
			updateUserFunc: func(ctx context.Context, input service.UpdateUserInput) (*model.User, error) {
				return sampleUser(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(jsonRequest(t, http.MethodPut, "/users/1", map[string]any{
			"email": "worker@example.com", "name": "Worker", "is_admin": false, "version": 1,
		}), map[string]string{"id": "1"})

		handler.Update(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
	})

	t.Run("Update maps version conflict", func(t *testing.T) {
		t.Parallel()

		handler := NewUserHandler(&stubUserService{
			updateUserFunc: func(ctx context.Context, input service.UpdateUserInput) (*model.User, error) {
				return nil, service.ErrVersionConflict
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(jsonRequest(t, http.MethodPut, "/users/1", map[string]any{
			"email": "worker@example.com", "name": "Worker", "is_admin": false, "version": 1,
		}), map[string]string{"id": "1"})

		handler.Update(recorder, req)

		assertErrorResponse(t, recorder, http.StatusConflict, "VERSION_CONFLICT")
	})

	t.Run("Update maps user not found", func(t *testing.T) {
		t.Parallel()

		handler := NewUserHandler(&stubUserService{
			updateUserFunc: func(ctx context.Context, input service.UpdateUserInput) (*model.User, error) {
				return nil, service.ErrUserNotFound
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(jsonRequest(t, http.MethodPut, "/users/1", map[string]any{
			"email": "worker@example.com", "name": "Worker", "is_admin": false, "version": 1,
		}), map[string]string{"id": "1"})

		handler.Update(recorder, req)

		assertErrorResponse(t, recorder, http.StatusNotFound, "USER_NOT_FOUND")
	})

	t.Run("UpdateMe returns updated user", func(t *testing.T) {
		t.Parallel()

		var received service.UpdateOwnProfileInput
		handler := NewUserHandler(&stubUserService{
			updateOwnProfileFunc: func(ctx context.Context, input service.UpdateOwnProfileInput) (*model.User, error) {
				received = input
				language := model.LanguagePreferenceEN
				theme := model.ThemePreferenceDark
				return &model.User{
					ID:                 input.ID,
					Email:              "worker@example.com",
					Name:               "Alice",
					Status:             model.UserStatusActive,
					Version:            2,
					LanguagePreference: &language,
					ThemePreference:    &theme,
				}, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(jsonRequest(t, http.MethodPut, "/users/me", map[string]any{
			"name":                "Alice",
			"language_preference": "en",
			"theme_preference":    "dark",
		}), sampleUser())

		handler.UpdateMe(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		if received.ID != 1 ||
			!received.Name.Set ||
			received.Name.Value == nil ||
			*received.Name.Value != "Alice" ||
			!received.LanguagePreference.Set ||
			received.LanguagePreference.Value == nil ||
			*received.LanguagePreference.Value != "en" ||
			!received.ThemePreference.Set ||
			received.ThemePreference.Value == nil ||
			*received.ThemePreference.Value != "dark" {
			t.Fatalf("unexpected service input: %+v", received)
		}
		response := decodeJSONResponse[userDetailResponse](t, recorder)
		if response.User.LanguagePreference == nil || *response.User.LanguagePreference != model.LanguagePreferenceEN {
			t.Fatalf("unexpected language preference: %+v", response.User.LanguagePreference)
		}
	})

	t.Run("UpdateMe rejects unknown fields", func(t *testing.T) {
		t.Parallel()

		handler := NewUserHandler(&stubUserService{
			updateOwnProfileFunc: func(ctx context.Context, input service.UpdateOwnProfileInput) (*model.User, error) {
				t.Fatalf("service should not be called for unknown field")
				return nil, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(jsonRequest(t, http.MethodPut, "/users/me", map[string]any{
			"is_admin": true,
		}), sampleUser())

		handler.UpdateMe(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("ResendInvitation returns no content", func(t *testing.T) {
		t.Parallel()

		handler := NewUserHandler(&stubUserService{
			resendInvitationFunc: func(ctx context.Context, userID int64) error {
				if userID != 1 {
					t.Fatalf("expected user ID 1, got %d", userID)
				}
				return nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodPost, "/users/1/resend-invitation", nil), map[string]string{"id": "1"})

		handler.ResendInvitation(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", recorder.Code)
		}
	})

	t.Run("ResendInvitation maps ErrUserNotPending", func(t *testing.T) {
		t.Parallel()

		handler := NewUserHandler(&stubUserService{
			resendInvitationFunc: func(ctx context.Context, userID int64) error {
				return model.ErrUserNotPending
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(httptest.NewRequest(http.MethodPost, "/users/1/resend-invitation", nil), map[string]string{"id": "1"})

		handler.ResendInvitation(recorder, req)

		assertErrorResponse(t, recorder, http.StatusConflict, "USER_NOT_PENDING")
	})

	t.Run("UpdateStatus returns updated user", func(t *testing.T) {
		t.Parallel()

		handler := NewUserHandler(&stubUserService{
			updateUserStatusFunc: func(ctx context.Context, input service.UpdateUserStatusInput) (*model.User, error) {
				return sampleUser(), nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(jsonRequest(t, http.MethodPatch, "/users/1/status", map[string]any{
			"status": "active", "version": 1,
		}), map[string]string{"id": "1"})

		handler.UpdateStatus(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
	})

	t.Run("UpdateStatus maps user not found", func(t *testing.T) {
		t.Parallel()

		handler := NewUserHandler(&stubUserService{
			updateUserStatusFunc: func(ctx context.Context, input service.UpdateUserStatusInput) (*model.User, error) {
				return nil, service.ErrUserNotFound
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithPathValues(jsonRequest(t, http.MethodPatch, "/users/1/status", map[string]any{
			"status": "active", "version": 1,
		}), map[string]string{"id": "1"})

		handler.UpdateStatus(recorder, req)

		assertErrorResponse(t, recorder, http.StatusNotFound, "USER_NOT_FOUND")
	})
}

func sampleUser() *model.User {
	return &model.User{
		ID:      1,
		Email:   "worker@example.com",
		Name:    "Worker",
		IsAdmin: false,
		Status:  model.UserStatusActive,
		Version: 1,
	}
}
