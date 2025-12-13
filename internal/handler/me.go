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
