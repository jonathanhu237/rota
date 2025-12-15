package handler

import (
	"errors"
	"net/http"

	"github.com/jonathanhu237/rota/internal/domain/user"
	"github.com/jonathanhu237/rota/internal/validator"
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

func (h *Handler) updatePassword(w http.ResponseWriter, r *http.Request) {
	userID := h.getUserID(r)

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := h.readJSON(r, &req); err != nil {
		h.invalidRequestBody(w)
		return
	}

	v := validator.New()
	v.Check(req.CurrentPassword != "", "current_password", "current_password is required")
	v.Check(req.NewPassword != "", "new_password", "new_password is required")
	v.Check(len(req.NewPassword) >= 8, "new_password", "new_password must be at least 8 characters")

	if !v.Valid() {
		h.validationError(w, v.Errors)
		return
	}

	err := h.userService.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrUserNotFound):
			h.unauthorized(w)
		case errors.Is(err, user.ErrIncorrectPassword):
			h.errorResponse(w, http.StatusBadRequest, ErrCodeIncorrectPassword, err.Error())
		case errors.Is(err, user.ErrConcurrentUpdate):
			h.errorResponse(w, http.StatusConflict, ErrCodeConcurrentUpdate, err.Error())
		default:
			h.internalServerError(w, err)
		}
		return
	}

	h.writeJSON(w, http.StatusNoContent, nil)
}
