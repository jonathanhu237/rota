package handler

import (
	"errors"
	"net/http"

	"github.com/jonathanhu237/rota/internal/domain/user"
)

func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	userID := h.getUserID(r)

	u, err := h.userService.GetByID(r.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrUserNotFound):
			h.unauthorized(w)
		default:
			h.internalServerError(w, err)
		}
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{"user": u})
}

func (h *Handler) updateMe(w http.ResponseWriter, r *http.Request) {
	userID := h.getUserID(r)

	var req struct {
		Name  *string `json:"name"`
		Email *string `json:"email"`
	}
	if err := h.readJSON(r, &req); err != nil {
		h.invalidRequestBody(w)
		return
	}

	u, err := h.userService.UpdateProfile(r.Context(), userID, req.Name, req.Email)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrUserNotFound):
			h.unauthorized(w)
		case errors.Is(err, user.ErrConcurrentUpdate):
			h.errorResponse(w, http.StatusConflict, ErrCodeConcurrentUpdate, err.Error())
		case errors.Is(err, user.ErrEmailAlreadyExists):
			h.errorResponse(w, http.StatusConflict, ErrCodeEmailExists, err.Error())
		default:
			h.internalServerError(w, err)
		}
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{"user": u})
}
