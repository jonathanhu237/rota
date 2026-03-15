package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

const accessTokenCookieName = "access_token"

type currentUserContextKey struct{}

type authService interface {
	Login(ctx context.Context, username, password string) (*service.LoginResult, error)
	Authenticate(ctx context.Context, accessToken string) (*model.User, error)
}

type AuthHandler struct {
	authService authService
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authUserResponse struct {
	User userResponse `json:"user"`
}

func NewAuthHandler(authService authService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Username and password are required")
		return
	}

	result, err := h.authService.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid username or password")
		case errors.Is(err, service.ErrUserDisabled):
			writeError(w, http.StatusForbidden, "USER_DISABLED", "User is disabled")
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		}
		return
	}

	expiresAt := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	http.SetCookie(w, &http.Cookie{
		Name:     accessTokenCookieName,
		Value:    result.AccessToken,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(result.ExpiresIn),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	writeData(w, http.StatusOK, authUserResponse{
		User: newUserResponse(result.User),
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	clearAccessTokenCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}

	writeData(w, http.StatusOK, authUserResponse{
		User: newUserResponse(user),
	})
}

func (h *AuthHandler) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(accessTokenCookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized")
			return
		}

		user, err := h.authService.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrUnauthorized):
				clearAccessTokenCookie(w, r)
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized")
			default:
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
			}
			return
		}

		next(w, r.WithContext(context.WithValue(r.Context(), currentUserContextKey{}, user)))
	}
}

func (h *AuthHandler) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return h.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		user, ok := currentUserFromRequest(r)
		if !ok {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
			return
		}
		if !user.IsAdmin {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "Forbidden")
			return
		}

		next(w, r)
	})
}

func currentUserFromRequest(r *http.Request) (*model.User, bool) {
	user, ok := r.Context().Value(currentUserContextKey{}).(*model.User)
	return user, ok
}

func clearAccessTokenCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     accessTokenCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}
