package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/jonathanhu237/rota/internal/domain/user"
	"github.com/jonathanhu237/rota/internal/validator"
)

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := h.readJSON(r, &req); err != nil {
		h.invalidRequestBody(w)
		return
	}

	v := validator.New()
	v.Check(req.Username != "", "username", "username is required")
	v.Check(req.Password != "", "password", "password is required")

	if !v.Valid() {
		h.validationError(w, v.Errors)
		return
	}

	token, expiresAt, err := h.userService.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrInvalidCredentials):
			h.errorResponse(w, http.StatusUnauthorized, err.Error())
		default:
			h.internalServerError(w, err)
		}
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "__rota_token",
		Value:    token,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		Expires:  expiresAt,
	})

	h.writeJSON(w, http.StatusNoContent, nil)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "__rota_token",
		Value:    "",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		Expires:  time.Now().Add(-1 * time.Hour),
	})

	h.writeJSON(w, http.StatusNoContent, nil)
}
