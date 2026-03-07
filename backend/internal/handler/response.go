package handler

import (
	"net/http"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

type dataResponse struct {
	Data any `json:"data"`
}

type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type userResponse struct {
	ID       int64            `json:"id"`
	Username string           `json:"username"`
	Name     string           `json:"name"`
	IsAdmin  bool             `json:"is_admin"`
	Status   model.UserStatus `json:"status"`
}

func newUserResponse(user *model.User) userResponse {
	return userResponse{
		ID:       user.ID,
		Username: user.Username,
		Name:     user.Name,
		IsAdmin:  user.IsAdmin,
		Status:   user.Status,
	}
}

func writeData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, dataResponse{Data: data})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{
		Error: errorDetail{
			Code:    code,
			Message: message,
		},
	})
}
