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

type stubAuthService struct {
	loginFunc                func(ctx context.Context, email, password string) (*service.LoginResult, error)
	requestPasswordResetFunc func(ctx context.Context, email string) error
	previewSetupTokenFunc    func(ctx context.Context, rawToken string) (*service.SetupTokenPreview, error)
	setupPasswordFunc        func(ctx context.Context, input service.SetupPasswordInput) error
	changeOwnPasswordFunc    func(ctx context.Context, viewerID int64, currentSessionID, currentPassword, newPassword string) (int, error)
	confirmEmailChangeFunc   func(ctx context.Context, rawToken string) error
	authenticateFunc         func(ctx context.Context, sessionID string) (*service.AuthenticateResult, error)
	logoutFunc               func(ctx context.Context, sessionID string) error
}

func (s *stubAuthService) Login(ctx context.Context, email, password string) (*service.LoginResult, error) {
	return s.loginFunc(ctx, email, password)
}

func (s *stubAuthService) RequestPasswordReset(ctx context.Context, email string) error {
	return s.requestPasswordResetFunc(ctx, email)
}

func (s *stubAuthService) PreviewSetupToken(ctx context.Context, rawToken string) (*service.SetupTokenPreview, error) {
	return s.previewSetupTokenFunc(ctx, rawToken)
}

func (s *stubAuthService) SetupPassword(ctx context.Context, input service.SetupPasswordInput) error {
	return s.setupPasswordFunc(ctx, input)
}

func (s *stubAuthService) ChangeOwnPassword(ctx context.Context, viewerID int64, currentSessionID, currentPassword, newPassword string) (int, error) {
	return s.changeOwnPasswordFunc(ctx, viewerID, currentSessionID, currentPassword, newPassword)
}

func (s *stubAuthService) ConfirmEmailChange(ctx context.Context, rawToken string) error {
	return s.confirmEmailChangeFunc(ctx, rawToken)
}

func (s *stubAuthService) Authenticate(ctx context.Context, sessionID string) (*service.AuthenticateResult, error) {
	return s.authenticateFunc(ctx, sessionID)
}

func (s *stubAuthService) Logout(ctx context.Context, sessionID string) error {
	return s.logoutFunc(ctx, sessionID)
}

func TestAuthHandler(t *testing.T) {
	t.Run("Login returns authenticated user and session cookie", func(t *testing.T) {
		t.Parallel()

		var receivedEmail string
		var receivedPassword string
		handler := NewAuthHandler(&stubAuthService{
			loginFunc: func(ctx context.Context, email, password string) (*service.LoginResult, error) {
				receivedEmail = email
				receivedPassword = password
				return &service.LoginResult{
					SessionID: "session-123",
					ExpiresIn: 3600,
					User:      sampleUser(),
				}, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPost, "/login", map[string]any{
			"email":    "worker@example.com",
			"password": "pa55word",
		})
		before := time.Now()

		handler.Login(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		if receivedEmail != "worker@example.com" || receivedPassword != "pa55word" {
			t.Fatalf("unexpected login input: %q %q", receivedEmail, receivedPassword)
		}

		response := decodeJSONResponse[authUserResponse](t, recorder)
		if response.User.Email != "worker@example.com" {
			t.Fatalf("expected response user email %q, got %q", "worker@example.com", response.User.Email)
		}

		assertSessionCookie(t, recorder.Result().Cookies(), "session-123", 3600, before)
	})

	t.Run("Login rejects invalid JSON", func(t *testing.T) {
		t.Parallel()

		handler := NewAuthHandler(&stubAuthService{})
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("{"))

		handler.Login(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "INVALID_REQUEST")
	})

	t.Run("Login maps invalid credentials", func(t *testing.T) {
		t.Parallel()

		handler := NewAuthHandler(&stubAuthService{
			loginFunc: func(ctx context.Context, email, password string) (*service.LoginResult, error) {
				return nil, service.ErrInvalidCredentials
			},
		})
		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPost, "/login", map[string]any{
			"email":    "worker@example.com",
			"password": "wrong",
		})

		handler.Login(recorder, req)

		assertErrorResponse(t, recorder, http.StatusUnauthorized, "INVALID_CREDENTIALS")
	})

	t.Run("Login maps disabled user", func(t *testing.T) {
		t.Parallel()

		handler := NewAuthHandler(&stubAuthService{
			loginFunc: func(ctx context.Context, email, password string) (*service.LoginResult, error) {
				return nil, service.ErrUserDisabled
			},
		})
		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPost, "/login", map[string]any{
			"email":    "worker@example.com",
			"password": "pa55word",
		})

		handler.Login(recorder, req)

		assertErrorResponse(t, recorder, http.StatusForbidden, "USER_DISABLED")
	})

	t.Run("Login maps pending user", func(t *testing.T) {
		t.Parallel()

		handler := NewAuthHandler(&stubAuthService{
			loginFunc: func(ctx context.Context, email, password string) (*service.LoginResult, error) {
				return nil, service.ErrUserPending
			},
		})
		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPost, "/login", map[string]any{
			"email":    "worker@example.com",
			"password": "pa55word",
		})

		handler.Login(recorder, req)

		assertErrorResponse(t, recorder, http.StatusForbidden, "USER_PENDING")
	})

	t.Run("Login maps unexpected service error", func(t *testing.T) {
		t.Parallel()

		handler := NewAuthHandler(&stubAuthService{
			loginFunc: func(ctx context.Context, email, password string) (*service.LoginResult, error) {
				return nil, errors.New("boom")
			},
		})
		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPost, "/login", map[string]any{
			"email":    "worker@example.com",
			"password": "pa55word",
		})

		handler.Login(recorder, req)

		assertErrorResponse(t, recorder, http.StatusInternalServerError, "INTERNAL_ERROR")
	})

	t.Run("RequestPasswordReset returns a generic success response", func(t *testing.T) {
		t.Parallel()

		var receivedEmail string
		handler := NewAuthHandler(&stubAuthService{
			requestPasswordResetFunc: func(ctx context.Context, email string) error {
				receivedEmail = email
				return nil
			},
		})
		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPost, "/auth/password-reset-request", map[string]any{
			"email": "worker@example.com",
		})

		handler.RequestPasswordReset(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}
		if receivedEmail != "worker@example.com" {
			t.Fatalf("expected email %q, got %q", "worker@example.com", receivedEmail)
		}

		response := decodeJSONResponse[passwordResetRequestResponse](t, recorder)
		if response.Message == "" {
			t.Fatalf("expected non-empty success message")
		}
	})

	t.Run("PreviewSetupToken returns token preview", func(t *testing.T) {
		t.Parallel()

		handler := NewAuthHandler(&stubAuthService{
			previewSetupTokenFunc: func(ctx context.Context, rawToken string) (*service.SetupTokenPreview, error) {
				if rawToken != "opaque-token" {
					t.Fatalf("expected token %q, got %q", "opaque-token", rawToken)
				}
				return &service.SetupTokenPreview{
					Email:   "worker@example.com",
					Name:    "Worker",
					Purpose: model.SetupTokenPurposeInvitation,
				}, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/auth/setup-token?token=opaque-token", nil)

		handler.PreviewSetupToken(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}

		response := decodeJSONResponse[setupTokenPreviewResponse](t, recorder)
		if response.Email != "worker@example.com" || response.Name != "Worker" {
			t.Fatalf("unexpected preview response: %+v", response)
		}
		if response.Purpose != model.SetupTokenPurposeInvitation {
			t.Fatalf("expected invitation purpose, got %q", response.Purpose)
		}
	})

	t.Run("PreviewSetupToken maps token errors", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name   string
			err    error
			status int
			code   string
		}{
			{name: "invalid", err: model.ErrInvalidToken, status: http.StatusBadRequest, code: "INVALID_TOKEN"},
			{name: "not found", err: model.ErrTokenNotFound, status: http.StatusNotFound, code: "TOKEN_NOT_FOUND"},
			{name: "expired", err: model.ErrTokenExpired, status: http.StatusGone, code: "TOKEN_EXPIRED"},
			{name: "used", err: model.ErrTokenUsed, status: http.StatusGone, code: "TOKEN_USED"},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				handler := NewAuthHandler(&stubAuthService{
					previewSetupTokenFunc: func(ctx context.Context, rawToken string) (*service.SetupTokenPreview, error) {
						return nil, tc.err
					},
				})
				recorder := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/auth/setup-token?token=opaque-token", nil)

				handler.PreviewSetupToken(recorder, req)

				assertErrorResponse(t, recorder, tc.status, tc.code)
			})
		}
	})

	t.Run("SetupPassword returns no content", func(t *testing.T) {
		t.Parallel()

		var receivedInput service.SetupPasswordInput
		handler := NewAuthHandler(&stubAuthService{
			setupPasswordFunc: func(ctx context.Context, input service.SetupPasswordInput) error {
				receivedInput = input
				return nil
			},
		})
		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPost, "/auth/setup-password", map[string]any{
			"token":    "opaque-token",
			"password": "pa55word",
		})

		handler.SetupPassword(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", recorder.Code)
		}
		if receivedInput.Token != "opaque-token" || receivedInput.Password != "pa55word" {
			t.Fatalf("unexpected setup password input: %+v", receivedInput)
		}
	})

	t.Run("SetupPassword maps validation and token errors", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name   string
			err    error
			status int
			code   string
		}{
			{name: "short password", err: model.ErrPasswordTooShort, status: http.StatusBadRequest, code: "PASSWORD_TOO_SHORT"},
			{name: "invalid token", err: model.ErrInvalidToken, status: http.StatusBadRequest, code: "INVALID_TOKEN"},
			{name: "not found", err: model.ErrTokenNotFound, status: http.StatusNotFound, code: "TOKEN_NOT_FOUND"},
			{name: "expired", err: model.ErrTokenExpired, status: http.StatusGone, code: "TOKEN_EXPIRED"},
			{name: "used", err: model.ErrTokenUsed, status: http.StatusGone, code: "TOKEN_USED"},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				handler := NewAuthHandler(&stubAuthService{
					setupPasswordFunc: func(ctx context.Context, input service.SetupPasswordInput) error {
						return tc.err
					},
				})
				recorder := httptest.NewRecorder()
				req := jsonRequest(t, http.MethodPost, "/auth/setup-password", map[string]any{
					"token":    "opaque-token",
					"password": "pa55word",
				})

				handler.SetupPassword(recorder, req)

				assertErrorResponse(t, recorder, tc.status, tc.code)
			})
		}
	})

	t.Run("Logout clears the session cookie after successful logout", func(t *testing.T) {
		t.Parallel()

		var receivedSessionID string
		handler := NewAuthHandler(&stubAuthService{
			logoutFunc: func(ctx context.Context, sessionID string) error {
				receivedSessionID = sessionID
				return nil
			},
		})
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/logout", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-456"})

		handler.Logout(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", recorder.Code)
		}
		if receivedSessionID != "session-456" {
			t.Fatalf("expected logout to receive session ID %q, got %q", "session-456", receivedSessionID)
		}

		assertClearedSessionCookie(t, recorder.Result().Cookies())
	})

	t.Run("Logout is idempotent when no session cookie is present", func(t *testing.T) {
		t.Parallel()

		handler := NewAuthHandler(&stubAuthService{
			logoutFunc: func(ctx context.Context, sessionID string) error {
				t.Fatalf("expected logout service not to be called")
				return nil
			},
		})
		recorder := httptest.NewRecorder()

		handler.Logout(recorder, httptest.NewRequest(http.MethodPost, "/logout", nil))

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", recorder.Code)
		}
		assertClearedSessionCookie(t, recorder.Result().Cookies())
	})

	t.Run("Logout maps unexpected service error", func(t *testing.T) {
		t.Parallel()

		handler := NewAuthHandler(&stubAuthService{
			logoutFunc: func(ctx context.Context, sessionID string) error {
				return errors.New("boom")
			},
		})
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/logout", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-456"})

		handler.Logout(recorder, req)

		assertErrorResponse(t, recorder, http.StatusInternalServerError, "INTERNAL_ERROR")
	})

	t.Run("ChangePassword returns no content", func(t *testing.T) {
		t.Parallel()

		var receivedViewerID int64
		var receivedSessionID string
		handler := NewAuthHandler(&stubAuthService{
			changeOwnPasswordFunc: func(ctx context.Context, viewerID int64, currentSessionID, currentPassword, newPassword string) (int, error) {
				receivedViewerID = viewerID
				receivedSessionID = currentSessionID
				if currentPassword != "current-password" || newPassword != "new-password" {
					t.Fatalf("unexpected password payload")
				}
				return 1, nil
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(jsonRequest(t, http.MethodPost, "/auth/change-password", map[string]any{
			"current_password": "current-password",
			"new_password":     "new-password",
		}), sampleUser())
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-a"})

		handler.ChangePassword(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", recorder.Code)
		}
		if receivedViewerID != 1 || receivedSessionID != "session-a" {
			t.Fatalf("unexpected service inputs: viewer=%d session=%q", receivedViewerID, receivedSessionID)
		}
	})

	t.Run("ChangePassword maps invalid current password", func(t *testing.T) {
		t.Parallel()

		handler := NewAuthHandler(&stubAuthService{
			changeOwnPasswordFunc: func(ctx context.Context, viewerID int64, currentSessionID, currentPassword, newPassword string) (int, error) {
				return 0, service.ErrInvalidCurrentPassword
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(jsonRequest(t, http.MethodPost, "/auth/change-password", map[string]any{
			"current_password": "wrong",
			"new_password":     "new-password",
		}), sampleUser())
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-a"})

		handler.ChangePassword(recorder, req)

		assertErrorResponse(t, recorder, http.StatusUnauthorized, "INVALID_CURRENT_PASSWORD")
	})

	t.Run("ChangePassword maps short password", func(t *testing.T) {
		t.Parallel()

		handler := NewAuthHandler(&stubAuthService{
			changeOwnPasswordFunc: func(ctx context.Context, viewerID int64, currentSessionID, currentPassword, newPassword string) (int, error) {
				return 0, model.ErrPasswordTooShort
			},
		})
		recorder := httptest.NewRecorder()
		req := requestWithUser(jsonRequest(t, http.MethodPost, "/auth/change-password", map[string]any{
			"current_password": "current-password",
			"new_password":     "short",
		}), sampleUser())
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-a"})

		handler.ChangePassword(recorder, req)

		assertErrorResponse(t, recorder, http.StatusBadRequest, "PASSWORD_TOO_SHORT")
	})

	t.Run("ConfirmEmailChange returns no content without auth", func(t *testing.T) {
		t.Parallel()

		var receivedToken string
		handler := NewAuthHandler(&stubAuthService{
			confirmEmailChangeFunc: func(ctx context.Context, rawToken string) error {
				receivedToken = rawToken
				return nil
			},
		})
		recorder := httptest.NewRecorder()
		req := jsonRequest(t, http.MethodPost, "/auth/confirm-email-change", map[string]any{
			"token": "raw-token",
		})

		handler.ConfirmEmailChange(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204, got %d", recorder.Code)
		}
		if receivedToken != "raw-token" {
			t.Fatalf("expected token to be passed through, got %q", receivedToken)
		}
	})

	t.Run("ConfirmEmailChange maps token errors and collision", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			err    error
			status int
			code   string
		}{
			{name: "invalid", err: model.ErrInvalidToken, status: http.StatusBadRequest, code: "INVALID_TOKEN"},
			{name: "not found", err: model.ErrTokenNotFound, status: http.StatusNotFound, code: "TOKEN_NOT_FOUND"},
			{name: "used", err: model.ErrTokenUsed, status: http.StatusGone, code: "TOKEN_USED"},
			{name: "expired", err: model.ErrTokenExpired, status: http.StatusGone, code: "TOKEN_EXPIRED"},
			{name: "collision", err: service.ErrEmailAlreadyExists, status: http.StatusConflict, code: "EMAIL_ALREADY_EXISTS"},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				handler := NewAuthHandler(&stubAuthService{
					confirmEmailChangeFunc: func(ctx context.Context, rawToken string) error {
						return tt.err
					},
				})
				recorder := httptest.NewRecorder()
				req := jsonRequest(t, http.MethodPost, "/auth/confirm-email-change", map[string]any{
					"token": "raw-token",
				})

				handler.ConfirmEmailChange(recorder, req)

				assertErrorResponse(t, recorder, tt.status, tt.code)
			})
		}
	})

	t.Run("Me returns the current user from request context", func(t *testing.T) {
		t.Parallel()

		handler := NewAuthHandler(&stubAuthService{})
		recorder := httptest.NewRecorder()
		language := model.LanguagePreferenceEN
		theme := model.ThemePreferenceDark
		user := sampleUser()
		user.LanguagePreference = &language
		user.ThemePreference = &theme
		req := requestWithUser(httptest.NewRequest(http.MethodGet, "/me", nil), user)

		handler.Me(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}

		response := decodeJSONResponse[authUserResponse](t, recorder)
		if response.User.Email != "worker@example.com" {
			t.Fatalf("expected response user email %q, got %q", "worker@example.com", response.User.Email)
		}
		if response.User.LanguagePreference == nil || *response.User.LanguagePreference != model.LanguagePreferenceEN {
			t.Fatalf("expected language preference en, got %+v", response.User.LanguagePreference)
		}
		if response.User.ThemePreference == nil || *response.User.ThemePreference != model.ThemePreferenceDark {
			t.Fatalf("expected theme preference dark, got %+v", response.User.ThemePreference)
		}
	})

	t.Run("Me returns internal error when user is missing from context", func(t *testing.T) {
		t.Parallel()

		handler := NewAuthHandler(&stubAuthService{})
		recorder := httptest.NewRecorder()

		handler.Me(recorder, httptest.NewRequest(http.MethodGet, "/me", nil))

		assertErrorResponse(t, recorder, http.StatusInternalServerError, "INTERNAL_ERROR")
	})
}

func TestAuthHandlerSetupPasswordTokenUsed(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(&stubAuthService{
		setupPasswordFunc: func(ctx context.Context, input service.SetupPasswordInput) error {
			return model.ErrTokenUsed
		},
	})
	recorder := httptest.NewRecorder()
	req := jsonRequest(t, http.MethodPost, "/auth/setup-password", map[string]any{
		"token":    "opaque-token",
		"password": "pa55word",
	})

	handler.SetupPassword(recorder, req)

	assertErrorResponse(t, recorder, http.StatusGone, "TOKEN_USED")
}

func assertSessionCookie(
	t testing.TB,
	cookies []*http.Cookie,
	expectedValue string,
	expectedMaxAge int,
	before time.Time,
) {
	t.Helper()

	cookie := findCookie(t, cookies, sessionCookieName)
	if cookie.Value != expectedValue {
		t.Fatalf("expected cookie value %q, got %q", expectedValue, cookie.Value)
	}
	if !cookie.HttpOnly {
		t.Fatalf("expected session cookie to be HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("expected SameSite=Lax, got %v", cookie.SameSite)
	}
	if cookie.MaxAge != expectedMaxAge {
		t.Fatalf("expected MaxAge %d, got %d", expectedMaxAge, cookie.MaxAge)
	}

	minExpiry := before.Add(time.Duration(expectedMaxAge) * time.Second)
	maxExpiry := time.Now().Add(time.Duration(expectedMaxAge) * time.Second)
	if cookie.Expires.Before(minExpiry.Add(-2*time.Second)) || cookie.Expires.After(maxExpiry.Add(2*time.Second)) {
		t.Fatalf("expected cookie expiry between %s and %s, got %s", minExpiry, maxExpiry, cookie.Expires)
	}
}

func assertClearedSessionCookie(t testing.TB, cookies []*http.Cookie) {
	t.Helper()

	cookie := findCookie(t, cookies, sessionCookieName)
	if cookie.Value != "" {
		t.Fatalf("expected cleared cookie value to be empty, got %q", cookie.Value)
	}
	if cookie.MaxAge != -1 {
		t.Fatalf("expected cleared cookie MaxAge -1, got %d", cookie.MaxAge)
	}
}

func findCookie(t testing.TB, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}

	t.Fatalf("cookie %q not found", name)
	return nil
}

func sampleAuthenticatedUser() *model.User {
	return sampleUser()
}
