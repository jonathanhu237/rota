package handler

import (
	"context"
	"net/http"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

type userListRepository interface {
	List(ctx context.Context) ([]*model.User, error)
}

type UserHandler struct {
	userRepo userListRepository
}

type usersResponse struct {
	Users []userResponse `json:"users"`
}

func NewUserHandler(userRepo userListRepository) *UserHandler {
	return &UserHandler{userRepo: userRepo}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.userRepo.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}

	result := make([]userResponse, 0, len(users))
	for _, user := range users {
		result = append(result, newUserResponse(user))
	}

	writeData(w, http.StatusOK, usersResponse{
		Users: result,
	})
}
