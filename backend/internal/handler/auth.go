package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

const sessionCookieName = "session_id"

type currentUserContextKey struct{}

type authService interface {
	Login(ctx context.Context, email, password string) (*service.LoginResult, error)
	RequestPasswordReset(ctx context.Context, emailAddress string) error
	PreviewSetupToken(ctx context.Context, rawToken string) (*service.SetupTokenPreview, error)
	SetupPassword(ctx context.Context, input service.SetupPasswordInput) error
	ChangeOwnPassword(ctx context.Context, viewerID int64, currentSessionID, currentPassword, newPassword string) (int, error)
	Authenticate(ctx context.Context, sessionID string) (*service.AuthenticateResult, error)
	Logout(ctx context.Context, sessionID string) error
}

type AuthHandler struct {
	authService authService
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type passwordResetRequest struct {
	Email string `json:"email"`
}

type setupPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type authUserResponse struct {
	User userResponse `json:"user"`
}

type passwordResetRequestResponse struct {
	Message string `json:"message"`
}

type setupTokenPreviewResponse struct {
	Email   string                  `json:"email"`
	Name    string                  `json:"name"`
	Purpose model.SetupTokenPurpose `json:"purpose"`
}

func NewAuthHandler(authService authService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Email and password are required")
		return
	}

	result, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password")
		case errors.Is(err, service.ErrUserPending):
			writeError(w, http.StatusForbidden, "USER_PENDING", "User has not finished setting a password")
		case errors.Is(err, service.ErrUserDisabled):
			writeError(w, http.StatusForbidden, "USER_DISABLED", "User is disabled")
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		}
		return
	}

	setSessionCookie(w, r, result.SessionID, result.ExpiresIn)

	writeData(w, http.StatusOK, authUserResponse{
		User: newUserResponse(result.User),
	})
}

func (h *AuthHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req passwordResetRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := h.authService.RequestPasswordReset(r.Context(), req.Email); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}

	writeData(w, http.StatusOK, passwordResetRequestResponse{
		Message: "If an account exists, a reset link has been sent",
	})
}

func (h *AuthHandler) PreviewSetupToken(w http.ResponseWriter, r *http.Request) {
	preview, err := h.authService.PreviewSetupToken(r.Context(), r.URL.Query().Get("token"))
	if err != nil {
		switch {
		case errors.Is(err, model.ErrInvalidToken):
			writeError(w, http.StatusBadRequest, "INVALID_TOKEN", "Invalid token")
		case errors.Is(err, model.ErrTokenNotFound):
			writeError(w, http.StatusNotFound, "TOKEN_NOT_FOUND", "Token not found")
		case errors.Is(err, model.ErrTokenExpired):
			writeError(w, http.StatusGone, "TOKEN_EXPIRED", "Token expired")
		case errors.Is(err, model.ErrTokenUsed):
			writeError(w, http.StatusGone, "TOKEN_USED", "Token already used")
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		}
		return
	}

	writeData(w, http.StatusOK, setupTokenPreviewResponse{
		Email:   preview.Email,
		Name:    preview.Name,
		Purpose: preview.Purpose,
	})
}

func (h *AuthHandler) SetupPassword(w http.ResponseWriter, r *http.Request) {
	var req setupPasswordRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	err := h.authService.SetupPassword(r.Context(), service.SetupPasswordInput{
		Token:    req.Token,
		Password: req.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, model.ErrPasswordTooShort):
			writeError(w, http.StatusBadRequest, "PASSWORD_TOO_SHORT", "Password must have at least 8 characters")
		case errors.Is(err, model.ErrInvalidToken):
			writeError(w, http.StatusBadRequest, "INVALID_TOKEN", "Invalid token")
		case errors.Is(err, model.ErrTokenNotFound):
			writeError(w, http.StatusNotFound, "TOKEN_NOT_FOUND", "Token not found")
		case errors.Is(err, model.ErrTokenExpired):
			writeError(w, http.StatusGone, "TOKEN_EXPIRED", "Token expired")
		case errors.Is(err, model.ErrTokenUsed):
			writeError(w, http.StatusGone, "TOKEN_USED", "Token already used")
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromRequest(r)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized")
		return
	}

	var req changePasswordRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	_, err = h.authService.ChangeOwnPassword(
		r.Context(),
		user.ID,
		cookie.Value,
		req.CurrentPassword,
		req.NewPassword,
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCurrentPassword):
			writeError(w, http.StatusUnauthorized, "INVALID_CURRENT_PASSWORD", "Current password is incorrect")
		case errors.Is(err, model.ErrPasswordTooShort):
			writeError(w, http.StatusBadRequest, "PASSWORD_TOO_SHORT", "Password must have at least 8 characters")
		case errors.Is(err, service.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil && cookie.Value != "" {
		if err := h.authService.Logout(r.Context(), cookie.Value); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
			return
		}
	}

	clearSessionCookie(w, r)
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
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized")
			return
		}

		result, err := h.authService.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrUnauthorized):
				clearSessionCookie(w, r)
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized")
			default:
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
			}
			return
		}

		// Refresh the cookie so the browser expiry stays in sync with the session row.
		setSessionCookie(w, r, cookie.Value, result.ExpiresIn)

		ctx := context.WithValue(r.Context(), currentUserContextKey{}, result.User)
		ctx = audit.WithActor(ctx, result.User.ID)
		next(w, r.WithContext(ctx))
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

func setSessionCookie(w http.ResponseWriter, r *http.Request, sessionID string, expiresIn int64) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		Expires:  time.Now().Add(time.Duration(expiresIn) * time.Second),
		MaxAge:   int(expiresIn),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}
