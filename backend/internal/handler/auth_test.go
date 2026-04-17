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
	loginFunc        func(ctx context.Context, email, password string) (*service.LoginResult, error)
	authenticateFunc func(ctx context.Context, sessionID string) (*service.AuthenticateResult, error)
	logoutFunc       func(ctx context.Context, sessionID string) error
}

func (s *stubAuthService) Login(ctx context.Context, email, password string) (*service.LoginResult, error) {
	return s.loginFunc(ctx, email, password)
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

	t.Run("Me returns the current user from request context", func(t *testing.T) {
		t.Parallel()

		handler := NewAuthHandler(&stubAuthService{})
		recorder := httptest.NewRecorder()
		req := requestWithUser(httptest.NewRequest(http.MethodGet, "/me", nil), sampleUser())

		handler.Me(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", recorder.Code)
		}

		response := decodeJSONResponse[authUserResponse](t, recorder)
		if response.User.Email != "worker@example.com" {
			t.Fatalf("expected response user email %q, got %q", "worker@example.com", response.User.Email)
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
