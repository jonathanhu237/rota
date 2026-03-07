package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/service"
)

const accessTokenCookieName = "access_token"

type loginService interface {
	Login(ctx context.Context, username, password string) (*service.LoginResult, error)
}

type AuthHandler struct {
	authService loginService
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	User userResponse `json:"user"`
}

func NewAuthHandler(authService loginService) *AuthHandler {
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

	writeAccessTokenCookie(w, r, result.AccessToken, result.ExpiresIn)
	writeData(w, http.StatusOK, loginResponse{
		User: newUserResponse(result.User),
	})
}

func writeAccessTokenCookie(w http.ResponseWriter, r *http.Request, accessToken string, expiresIn int64) {
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
	http.SetCookie(w, &http.Cookie{
		Name:     accessTokenCookieName,
		Value:    accessToken,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(expiresIn),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}
