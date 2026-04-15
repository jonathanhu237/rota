package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/service"
)

type userPositionService interface {
	ListUserPositions(ctx context.Context, userID int64) ([]*model.Position, error)
	ReplaceUserPositions(ctx context.Context, input service.ReplaceUserPositionsInput) error
}

type UserPositionHandler struct {
	userPositionService userPositionService
}

type userPositionsResponse struct {
	Positions []positionResponse `json:"positions"`
}

type replaceUserPositionsRequest struct {
	PositionIDs []int64 `json:"position_ids"`
}

func NewUserPositionHandler(userPositionService userPositionService) *UserPositionHandler {
	return &UserPositionHandler{userPositionService: userPositionService}
}

func (h *UserPositionHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid user id")
		return
	}

	positions, err := h.userPositionService.ListUserPositions(r.Context(), userID)
	if err != nil {
		h.writeUserPositionServiceError(w, err)
		return
	}

	response := make([]positionResponse, 0, len(positions))
	for _, position := range positions {
		response = append(response, newPositionResponse(position))
	}

	writeData(w, http.StatusOK, userPositionsResponse{
		Positions: response,
	})
}

func (h *UserPositionHandler) Replace(w http.ResponseWriter, r *http.Request) {
	userID, err := parsePathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid user id")
		return
	}

	var req replaceUserPositionsRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := h.userPositionService.ReplaceUserPositions(r.Context(), service.ReplaceUserPositionsInput{
		UserID:      userID,
		PositionIDs: req.PositionIDs,
	}); err != nil {
		h.writeUserPositionServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *UserPositionHandler) writeUserPositionServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request")
	case errors.Is(err, service.ErrUserNotFound):
		writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
	case errors.Is(err, service.ErrPositionNotFound):
		writeError(w, http.StatusNotFound, "POSITION_NOT_FOUND", "Position not found")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
}
