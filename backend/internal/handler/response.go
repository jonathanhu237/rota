package handler

import (
	"net/http"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type userResponse struct {
	ID      int64            `json:"id"`
	Email   string           `json:"email"`
	Name    string           `json:"name"`
	IsAdmin bool             `json:"is_admin"`
	Status  model.UserStatus `json:"status"`
	Version int              `json:"version"`
}

type paginationResponse struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type positionResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func newUserResponse(user *model.User) userResponse {
	return userResponse{
		ID:      user.ID,
		Email:   user.Email,
		Name:    user.Name,
		IsAdmin: user.IsAdmin,
		Status:  user.Status,
		Version: user.Version,
	}
}

func newPositionResponse(position *model.Position) positionResponse {
	return positionResponse{
		ID:          position.ID,
		Name:        position.Name,
		Description: position.Description,
		CreatedAt:   position.CreatedAt,
		UpdatedAt:   position.UpdatedAt,
	}
}

func writeData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, data)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{
		Error: errorDetail{
			Code:    code,
			Message: message,
		},
	})
}
